// Package guard implements the trackfw guard subcommands in Go.
// It is a faithful port of scripts/trackfw-git-branch-guard.sh (756 lines).
// The Go version replaces both the jq and awk extraction paths with
// encoding/json + map[string]json.RawMessage (case-sensitive, last-wins for
// duplicate keys), per ADR-2026-10-02 D1–D3 and ADR-2026-10-04 D9.
package guard

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// ErrStdinTruncated is returned by DrainStdin when a 2-second idle window
// received no bytes (matching the bash loop behavior in .sh lines 86–100).
var ErrStdinTruncated = errors.New("stdin truncated: idle 2s window returned no bytes")

// ErrNULInCommand is returned when the decoded command string contains NUL.
// ADR-2026-10-02 D2-bis.
var ErrNULInCommand = errors.New("command contains NUL (\\u0000)")

// ErrIndecodeable is returned when a command key is present but the value
// cannot be decoded as a JSON string. ADR-2026-10-02 D2.
var ErrIndecodeable = errors.New("command key present but cannot be decoded")

// DrainStdin reads from r using fixed idle windows of idleTimeout (2s in
// production). The semantics match bash's "read -r -t 2 -d ''" loop (lines
// 86–100 of .sh): each window collects ALL bytes that arrive; the timer is NOT
// reset when data arrives within a window. At the end of a window:
//
//   - EOF/ErrClosedPipe seen → return (data, false)   — not truncated
//   - data received but no EOF → start new window
//   - no bytes in the entire window → return (data, true) — truncated
//
// A single goroutine owns the read buffer (no data race). On truncation the
// goroutine is leaked, but the hook process exits immediately after
// DrainStdin returns, so this is acceptable in production.
func DrainStdin(r io.Reader, idleTimeout time.Duration) ([]byte, bool) {
	type chunk struct {
		data []byte
		err  error
	}

	ch := make(chan chunk, 64)

	// Single goroutine — owns buf, copies before sending so there are no races.
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf)
			c := chunk{err: err}
			if n > 0 {
				c.data = make([]byte, n)
				copy(c.data, buf[:n])
			}
			ch <- c
			if err != nil {
				return
			}
		}
	}()

	var result bytes.Buffer

	for {
		// Fixed window — timer starts once per window and is not reset on data.
		window := time.NewTimer(idleTimeout)
		gotData := false
		gotEOF := false

	windowLoop:
		for {
			select {
			case c := <-ch:
				if len(c.data) > 0 {
					result.Write(c.data)
					gotData = true
				}
				if c.err == io.EOF || errors.Is(c.err, io.ErrClosedPipe) {
					gotEOF = true
					break windowLoop
				}
				if c.err != nil {
					// Other errors treated as EOF (not truncated).
					gotEOF = true
					break windowLoop
				}
				// Data with no error — keep collecting within this window.

			case <-window.C:
				// Window closed — drain any item that arrived at the same instant.
				select {
				case c := <-ch:
					if len(c.data) > 0 {
						result.Write(c.data)
						gotData = true
					}
					if c.err != nil {
						// Any error (including non-EOF) treated as EOF, matching main path.
						gotEOF = true
					}
				default:
				}
				break windowLoop
			}
		}
		window.Stop()

		if gotEOF {
			return result.Bytes(), false
		}
		if !gotData {
			return result.Bytes(), true // idle window → truncated
		}
		// Data received, no EOF → start new window.
	}
}

// ExtractCommand parses the guard payload and returns the command string.
//
// If data starts with '{' (after stripping BOM and whitespace), it is parsed
// as JSON. Key priority (identical to the jq expression in the .sh):
//
//  1. tool_input.command
//  2. command (root-level)
//  3. tool_info.command_line
//  4. hook_input.command
//
// Rules (ADR-2026-10-02 D2, D2-bis, D2-ter):
//   - map[string]json.RawMessage is used for ALL levels so key matching is
//     case-sensitive and exact (e.g. "Command" never overwrites "command").
//   - Duplicate keys: last value wins (Go map semantics; matches jq).
//   - Key present with value null → treated as absent (try next priority).
//   - Key present, value not a JSON string (number, array, object, bool) →
//     treated as absent (matches awk extractor, which only captures string-quoted
//     values; try next priority key).
//   - Decoded string contains NUL → ErrNULInCommand (fail-closed).
//   - UTF-8 BOM stripped before parsing (PowerShell may emit one).
//
// Non-JSON input (does not start with '{'): the raw bytes are returned as
// the command string, same as the .sh `case *) CMD_RAW="$INPUT"` branch.
//
// Returns ("", nil) when no command key is found (guard is a no-op).
func ExtractCommand(data []byte) (string, error) {
	// Strip UTF-8 BOM (PowerShell can emit 0xEF 0xBB 0xBF).
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		// Non-JSON payload: treated as raw command (bash `case *` branch).
		return string(data), nil
	}

	// Parse root level into a case-sensitive map.
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return "", fmt.Errorf("payload JSON inválido: %w", err)
	}

	// Priority 1: tool_input.command
	if cmd, found, err := extractNested(root, "tool_input", "command"); err != nil {
		return "", err
	} else if found {
		return checkNUL(cmd)
	}

	// Priority 2: command (root-level)
	if raw, ok := root["command"]; ok && !isJSONNull(raw) {
		// Non-string value → treat as absent (matches awk extractor behavior).
		trimmedRaw := bytes.TrimLeft(raw, " \t\r\n")
		if len(trimmedRaw) > 0 && trimmedRaw[0] == '"' {
			var cmd string
			if err := json.Unmarshal(raw, &cmd); err != nil {
				return "", fmt.Errorf("%w: %s", ErrIndecodeable, err)
			}
			return checkNUL(cmd)
		}
	}

	// Priority 3: tool_info.command_line
	if cmd, found, err := extractNested(root, "tool_info", "command_line"); err != nil {
		return "", err
	} else if found {
		return checkNUL(cmd)
	}

	// Priority 4: hook_input.command
	if cmd, found, err := extractNested(root, "hook_input", "command"); err != nil {
		return "", err
	} else if found {
		return checkNUL(cmd)
	}

	return "", nil // no command key found
}

// extractNested looks up objKey in root, parses it as a JSON object, then
// extracts fieldKey from that object.
// Returns ("", false, nil) when either key is absent or null.
// Returns ("", false, err) when the field is present but not a JSON string.
func extractNested(root map[string]json.RawMessage, objKey, fieldKey string) (string, bool, error) {
	rawObj, ok := root[objKey]
	if !ok || isJSONNull(rawObj) {
		return "", false, nil
	}
	var nested map[string]json.RawMessage
	if err := json.Unmarshal(rawObj, &nested); err != nil {
		// Cannot parse the nested object; skip gracefully.
		return "", false, nil
	}
	rawField, ok := nested[fieldKey]
	if !ok || isJSONNull(rawField) {
		return "", false, nil
	}
	// Non-string JSON value (number, array, object, bool) → treat as absent.
	// Matches the bash awk extractor which only captures string-quoted values.
	trimmedRaw := bytes.TrimLeft(rawField, " \t\r\n")
	if len(trimmedRaw) == 0 || trimmedRaw[0] != '"' {
		return "", false, nil
	}
	var val string
	if err := json.Unmarshal(rawField, &val); err != nil {
		return "", false, fmt.Errorf("%w: %s.%s: %s", ErrIndecodeable, objKey, fieldKey, err)
	}
	return val, true, nil
}

// isJSONNull reports whether raw is the JSON null literal.
func isJSONNull(raw json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(raw), []byte("null"))
}

// checkNUL returns an error if s contains a NUL byte (ADR-2026-10-02 D2-bis).
func checkNUL(s string) (string, error) {
	if strings.ContainsRune(s, '\x00') {
		return "", ErrNULInCommand
	}
	return s, nil
}

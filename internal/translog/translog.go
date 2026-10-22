// Package translog is the single reader and writer for trackfw transition logs.
//
// # Format
//
// Every transition is stored as a single line of the form:
//
//	2006-01-02 15:04  ROADMAP-....md  backlog → wip  -0700  (user@example.com)
//
// # Storage
//
// New writes land in <dir>/.trackfw-log.d/<YYYYMMDDTHHMMSSZ>-<h8>.entry, where
// <h8> are the first 8 lowercase hex digits of the SHA-256 of the line (WITHOUT
// the trailing \n). Two parallel branches that record the same transition produce
// the same filename and the same content — a git merge of the two directories
// produces no conflict. The trailing .entry extension is deliberately different
// from .log to avoid being silenced by "*.log" patterns that many projects carry
// in their .gitignore.
//
// The legacy <dir>/.trackfw-log file (one line per entry, append-only) is still
// read for backward compatibility but is never written. Binaries older than the
// release that introduced this package continue appending to .trackfw-log; the
// unified reader merges both sources on every read.
//
// The hash input is the line WITHOUT trailing \n, and that is the contract shared
// between writers and cli-parity.md. Changing the hash input is a breaking change.
package translog

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Transition represents a single state-change entry from the log.
type Transition struct {
	Timestamp time.Time
	Basename  string
	From, To  string
}

// lineRe matches lines in the format produced by the writer:
//
//	2026-06-12 14:30  ROADMAP-....md  backlog → wip  -0300  (email)
//
// The trailing suffix (timezone offset, email) is not anchored; the regex stops
// at the fourth field. This is intentional: the validator and serve tests pin the
// decision that (a) a suffix at the END does not affect parsing, and (b) an offset
// inserted RIGHT AFTER the timestamp DOES break the line — see
// internal/generators/transition_log_suffix_test.go.
var lineRe = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2})\s{2,}(\S+)\s{2,}(\S+)\s+→\s+(\S+)`,
)

// entryNameRe matches the expected entry filename format so the reader can extract
// the UTC sort key. Files that do not match are silently skipped.
var entryNameRe = regexp.MustCompile(`^(\d{8}T\d{6}Z)-[0-9a-f]{8}\.entry$`)

const (
	entryTimeFormat = "20060102T150405Z"
	entryExt        = ".entry"
	dotDSuffix      = "/.trackfw-log.d"
	legacySuffix    = "/.trackfw-log"
)

// parseLine parses one log line. Returns a zero Transition and false if the line
// does not match the expected format. The Timestamp is parsed in time.Local for
// compatibility with the existing metrics.go and validator.go parsers.
func parseLine(line string) (Transition, bool) {
	m := lineRe.FindStringSubmatch(line)
	if m == nil {
		return Transition{}, false
	}
	ts, err := time.ParseInLocation("2006-01-02 15:04", m[1], time.Local)
	if err != nil {
		return Transition{}, false
	}
	return Transition{
		Timestamp: ts,
		Basename:  strings.TrimSpace(m[2]),
		From:      strings.TrimSpace(m[3]),
		To:        strings.TrimSpace(m[4]),
	}, true
}

// ReadAll reads all transitions from dir.
//
// It combines:
//   - the legacy file <dir>/.trackfw-log (if it exists), read FIFO-safely
//   - all *.entry regular files in <dir>/.trackfw-log.d/ (sorted by filename)
//
// Results are sorted by the entry's effective sort key:
//   - legacy lines: their parsed Timestamp (time.Local, minute precision)
//   - .d/ entries: the UTC timestamp embedded in the filename (second precision)
//
// Tie-breaking within the same second: lexicographic order of the source name,
// which means ".trackfw-log" (legacy) < "20261022T100000Z-…" (entry files).
//
// Files inside .d/ that are not regular files or do not end in .entry are silently
// ignored — they may be temp files from crashed writes (*.tmp from os.CreateTemp),
// OS metadata files (.DS_Store), editor swap files, or future-format extensions.
//
// An error is returned only for hard failures: symlink in place of .d/, unreadable
// directory. Individual unreadable entry files are silently skipped.
func ReadAll(dir string) ([]Transition, error) {
	type weighted struct {
		t       Transition
		sortKey time.Time
		name    string
	}
	var ws []weighted

	// 1. Legacy file — read FIFO-safely.
	legacyPath := dir + legacySuffix
	data, err := readRegularFileBytes(legacyPath)
	switch {
	case err == nil:
		for _, raw := range strings.Split(string(data), "\n") {
			line := strings.TrimRight(raw, "\r")
			if strings.TrimSpace(line) == "" {
				continue
			}
			t, ok := parseLine(line)
			if !ok {
				continue
			}
			ws = append(ws, weighted{t: t, sortKey: t.Timestamp, name: legacySuffix[1:]})
		}
	case os.IsNotExist(err), err == errNotRegularFile:
		// No legacy file — that is fine.
	default:
		return nil, fmt.Errorf("translog: read legacy log %s: %w", legacyPath, err)
	}

	// 2. .d/ entries.
	dPath := dir + dotDSuffix
	lstat, lstatErr := os.Lstat(dPath)
	switch {
	case lstatErr != nil && os.IsNotExist(lstatErr):
		// .d/ not created yet — normal for projects that have only ever run older binaries.
	case lstatErr != nil:
		return nil, fmt.Errorf("translog: stat %s: %w", dPath, lstatErr)
	case lstat.Mode()&os.ModeSymlink != 0:
		// A symlink where the directory should be is a hostile configuration — refuse.
		return nil, fmt.Errorf("translog: %s is a symlink — refusing to read", dPath)
	case !lstat.IsDir():
		return nil, fmt.Errorf("translog: %s is not a directory", dPath)
	default:
		des, rdErr := os.ReadDir(dPath)
		if rdErr != nil {
			return nil, fmt.Errorf("translog: read dir %s: %w", dPath, rdErr)
		}
		for _, de := range des {
			name := de.Name()
			if !strings.HasSuffix(name, entryExt) {
				// Silently ignored: .tmp crash remnants, .DS_Store, future-format files.
				continue
			}
			if de.Type()&os.ModeSymlink != 0 || !de.Type().IsRegular() {
				// Silently ignore symlinks and non-regular files inside .d/.
				continue
			}
			fullPath := filepath.Join(dPath, name)
			content, readErr := readRegularFileBytes(fullPath)
			if readErr != nil {
				// Silently skip unreadable entries (permission issues, torn writes).
				continue
			}

			// Extract sort key from filename.
			var sortKey time.Time
			if m := entryNameRe.FindStringSubmatch(name); m != nil {
				if ts, parseErr := time.Parse(entryTimeFormat, m[1]); parseErr == nil {
					sortKey = ts
				}
			}

			for _, raw := range strings.Split(strings.TrimRight(string(content), "\r\n"), "\n") {
				line := strings.TrimRight(raw, "\r")
				if strings.TrimSpace(line) == "" {
					continue
				}
				t, ok := parseLine(line)
				if !ok {
					continue
				}
				sk := sortKey
				if sk.IsZero() {
					sk = t.Timestamp // fallback for non-standard filenames
				}
				ws = append(ws, weighted{t: t, sortKey: sk, name: name})
			}
		}
	}

	sort.SliceStable(ws, func(i, j int) bool {
		if !ws[i].sortKey.Equal(ws[j].sortKey) {
			return ws[i].sortKey.Before(ws[j].sortKey)
		}
		return ws[i].name < ws[j].name
	})

	result := make([]Transition, len(ws))
	for i, w := range ws {
		result[i] = w.t
	}
	return result, nil
}

// EntryFilename returns the filename for an entry given a line and a timestamp.
// line must NOT include a trailing \n. The filename embeds now in UTC and the
// first 8 hex digits of SHA-256(line).
//
// The hash input is the line WITHOUT trailing \n. This is the contract documented
// in cli-parity.md and shared with any cross-language implementation.
func EntryFilename(line string, now time.Time) string {
	h := sha256.Sum256([]byte(line))
	h8 := fmt.Sprintf("%x", h[:4])
	ts := now.UTC().Format(entryTimeFormat)
	return ts + "-" + h8 + entryExt
}

// WriteEntry writes one entry to <dir>/.trackfw-log.d/<name>.entry where name is
// EntryFilename(line, now). line must NOT include a trailing \n.
//
// Idempotency: if the target file already exists with content equal to line+"\n"
// (after CRLF normalisation), WriteEntry is a no-op and returns nil. If the file
// exists with DIFFERENT content, WriteEntry returns an error — this can only happen
// if a SHA-256 collision occurs (astronomically unlikely) or via direct filesystem
// manipulation.
//
// The write is atomic: a temp file is written to the same directory (so the rename
// is within the same filesystem) and then renamed into place. Temp files use the
// pattern "tmp-*" (NOT "*.entry"), so a crashed write leaves a file that the reader
// silently ignores. The remaining TOCTOU race on Windows is narrow and documented.
//
// WriteEntry does NOT call pathguard — the caller must call pathguard.RejectAndReport
// on dir+"/.trackfw-log.d" before invoking this function.
func WriteEntry(dir string, line string, now time.Time) (string, error) {
	dPath := dir + dotDSuffix
	name := EntryFilename(line, now)
	entryPath := filepath.Join(dPath, name)
	content := line + "\n"

	// Safety: refuse to create .d/ if it is already a symlink.
	if lstat, err := os.Lstat(dPath); err == nil {
		if lstat.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("translog: %s is a symlink — refusing to write", dPath)
		}
		if !lstat.IsDir() {
			return "", fmt.Errorf("translog: %s exists but is not a directory", dPath)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("translog: stat %s: %w", dPath, err)
	}

	// Callers guard dir+"/.trackfw-log.d" with pathguard.RejectAndReport before invoking WriteEntry;
	// temp files use a non-.entry name so they are skipped by ReadAll.
	// write-containment-allowed: caller (e.g. discover.go) calls rejectDiscoverPath/pathguard before WriteEntry
	if err := os.MkdirAll(dPath, 0755); err != nil {
		return "", fmt.Errorf("translog: mkdir %s: %w", dPath, err)
	}

	// Idempotency check: read existing content before committing the temp file.
	if existing, readErr := os.ReadFile(entryPath); readErr == nil {
		// Normalise CRLF so a Windows checkout of an existing entry is idempotent.
		existingNorm := string(bytes.ReplaceAll(existing, []byte("\r\n"), []byte("\n")))
		if existingNorm == content {
			return name, nil
		}
		// TOCTOU note: the check and the rename below are not atomic. A collision here
		// (same hash, different content) requires a SHA-256 preimage attack or direct
		// filesystem manipulation and is treated as an error rather than silently clobbering.
		return "", fmt.Errorf("translog: entry %s already exists with different content", name)
	}

	// write-containment-allowed: MkdirAll passed above; CreateTemp is in the same dir.
	tmp, createErr := os.CreateTemp(dPath, "tmp-")
	if createErr != nil {
		return "", fmt.Errorf("translog: create temp in %s: %w", dPath, createErr)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after successful rename

	if _, err := io.WriteString(tmp, content); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("translog: write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("translog: close temp: %w", err)
	}

	// Rename is atomic on POSIX. On Windows, if entryPath already exists the rename
	// may fail; the idempotency check above reduces (but does not eliminate) the race.
	// write-containment-allowed: same dir as CreateTemp above; guarded by caller pathguard before WriteEntry
	if err := os.Rename(tmpName, entryPath); err != nil {
		return "", fmt.Errorf("translog: rename to %s: %w", entryPath, err)
	}
	return name, nil
}

// HasLog reports whether dir has any log at all — either a legacy .trackfw-log file
// or a .trackfw-log.d/ directory with at least one .entry file.
func HasLog(dir string) bool {
	if _, err := os.Stat(dir + legacySuffix); err == nil {
		return true
	}
	dPath := dir + dotDSuffix
	info, err := os.Stat(dPath)
	if err != nil || !info.IsDir() {
		return false
	}
	des, err := os.ReadDir(dPath)
	if err != nil {
		return false
	}
	for _, de := range des {
		if strings.HasSuffix(de.Name(), entryExt) && de.Type().IsRegular() {
			return true
		}
	}
	return false
}

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
// New writes land in <dir>/.trackfw-log-entries/<YYYYMMDDTHHMMSSZ>-<h8>.entry, where
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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/pathguard"
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
// Group 5 captures the optional timezone offset (e.g. "-0300", "+0530") that
// transitionLogSuffix appends after two or more spaces following the To field.
// parseLine uses the offset — when present — to build a time.FixedZone location
// so that sort keys are correct regardless of the machine's TZ setting.
//
// The trailing email field is not captured; it is decoration only.
//
// The invariant that (a) a suffix at the END does not affect the first four
// fields, and (b) an offset inserted RIGHT AFTER the timestamp DOES break the
// line, is pinned by internal/generators/transition_log_suffix_test.go.
var lineRe = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2})\s{2,}(\S+)\s{2,}(\S+)\s+→\s+(\S+)(?:\s{2,}([+-]\d{4}))?`,
)

// entryNameRe matches the expected entry filename format so the reader can extract
// the UTC sort key. Files that do not match are silently skipped.
var entryNameRe = regexp.MustCompile(`^(\d{8}T\d{6}Z)-[0-9a-f]{8}\.entry$`)

// EntriesDirName is the bare directory name where per-entry log files land.
// It is exported so callers can reference the name without embedding a
// literal, keeping the name in a single place.
//
// The old name was ".trackfw-log.d"; it was renamed because "*.d" is a
// common .gitignore pattern (e.g. the GitHub C++ .gitignore opens with it),
// which would silently prevent the entries from being committed.
//
// EntriesDirSuffix is derived from EntriesDirName: "/" + EntriesDirName.
// Use EntriesDirName when you need only the bare directory name;
// use EntriesDirSuffix when you need the slash-prefixed form for path
// concatenation with a parent dir.
const (
	EntriesDirName   = ".trackfw-log-entries"
	EntriesDirSuffix = "/" + EntriesDirName
	entryTimeFormat  = "20060102T150405Z"
	entryExt         = ".entry"
	legacySuffix     = "/.trackfw-log"
	// maxEntrySize is the per-.entry file size limit enforced by ReadAll (R2).
	// A single transition line is under 200 bytes; 1 MiB is several thousand
	// entries in one file and is reached only via manual or malicious writes.
	maxEntrySize = int64(1 << 20)
)

// parseLine parses one log line. Returns a zero Transition and false if the line
// does not match the expected format.
//
// If the line carries a timezone offset in group 5 (e.g. "-0300"), Timestamp is
// placed in a time.FixedZone for that offset — this preserves the wall-clock
// value written by the client and ensures that sort comparisons against .entry
// filenames (whose timestamps are always UTC) are correct regardless of the
// reader's TZ setting.
//
// Lines without an offset fall back to time.Local, preserving backward
// compatibility with legacy log lines that were written without a suffix.
func parseLine(line string) (Transition, bool) {
	m := lineRe.FindStringSubmatch(line)
	if m == nil {
		return Transition{}, false
	}
	loc := time.Local
	if len(m) > 5 && m[5] != "" {
		sign := 1
		if m[5][0] == '-' {
			sign = -1
		}
		h := int(m[5][1]-'0')*10 + int(m[5][2]-'0')
		min := int(m[5][3]-'0')*10 + int(m[5][4]-'0')
		loc = time.FixedZone("", sign*(h*3600+min*60))
	}
	ts, err := time.ParseInLocation("2006-01-02 15:04", m[1], loc)
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
//   - all *.entry regular files in <dir>/.trackfw-log-entries/ (sorted by filename)
//
// Results are sorted by the entry's effective sort key:
//   - legacy lines: their parsed Timestamp (time.Local or FixedZone from line offset)
//   - .trackfw-log-entries/ entries: the UTC timestamp embedded in the filename (second precision)
//
// Tie-breaking within the same second: lexicographic order of the source name,
// which means ".trackfw-log" (legacy) < "20261022T100000Z-…" (entry files).
//
// Files inside .trackfw-log-entries/ are accepted only when their name matches
// entryNameRe (the canonical format YYYYMMDDTHHMMSSZ-hhhhhhhh.entry). All other
// files are silently ignored — they may be temp files from crashed writes
// (tmp-* from os.CreateTemp), OS metadata files (.DS_Store), editor swap files,
// or future-format extensions.
//
// An error is returned for hard failures: symlink in place of the entries directory,
// unreadable directory, or any .entry file exceeding maxEntrySize (1 MiB).
// Individual unreadable entry files are silently skipped.
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

	// 2. entries directory.
	dPath := dir + EntriesDirSuffix
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
			// R3: accept only files matching the canonical entry filename format.
			// Silently ignored: tmp-* crash remnants, .DS_Store, x.entry (wrong format),
			// future-format extensions, and any other non-canonical names.
			if !entryNameRe.MatchString(name) {
				continue
			}
			if de.Type()&os.ModeSymlink != 0 || !de.Type().IsRegular() {
				// Silently ignore symlinks and non-regular files inside .trackfw-log-entries/.
				continue
			}
			fullPath := filepath.Join(dPath, name)
			// R2: enforce 1 MiB per-file limit. Oversize is a hard error (named with the
			// file); other read failures are silently skipped (permission issues, torn writes).
			content, readErr := readRegularFileBytesLimited(fullPath, maxEntrySize)
			if readErr != nil {
				if errors.Is(readErr, errEntryTooLarge) {
					return nil, fmt.Errorf("translog: %w", readErr)
				}
				// Silently skip unreadable entries (permission issues, torn writes).
				continue
			}

			// Extract sort key from filename.
			// After the entryNameRe check above, m[1] always parses without error.
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
				ws = append(ws, weighted{t: t, sortKey: sortKey, name: name})
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

// WriteEntry writes one entry to the log-entries directory derived from resolvedRoot
// and dir. The entry file is named EntryFilename(line, now). line must NOT include a
// trailing \n.
//
// resolvedRoot must be the resolved project root (from pathguard.ResolveRoot or
// equivalent). WriteEntry calls pathguard.RejectAndReport on the entry path before
// creating any file or directory, so a symlink placed in any ancestor of the entries
// directory is refused. An empty resolvedRoot is refused via
// pathguard.RefuseUnverifiableRoot.
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
func WriteEntry(resolvedRoot string, dir string, line string, now time.Time) (string, error) {
	if resolvedRoot == "" {
		return "", pathguard.RefuseUnverifiableRoot(dir+EntriesDirSuffix, fmt.Errorf("guard root is empty"))
	}
	// dPath is the absolute entries directory; entryPath is the leaf file to guard.
	// In production, dir is a relative path (e.g. "docs/roadmaps") and resolvedRoot
	// is the project root, so filepath.Join produces the correct absolute path.
	// In tests, dir is an absolute temp path. Go's filepath.Join does NOT drop
	// elements before an absolute component (unlike Python) — filepath.Join("/a",
	// "/a", "c") = "/a/a/c". So when dir is already absolute, use it directly.
	dPath := filepath.Join(resolvedRoot, dir, EntriesDirName)
	if filepath.IsAbs(dir) {
		dPath = filepath.Join(dir, EntriesDirName)
	}
	name := EntryFilename(line, now)
	entryPath := filepath.Join(dPath, name)
	content := line + "\n"

	// Guard entryPath against ancestor symlinks. RejectSymlinks walks every ancestor
	// up to resolvedRoot, so MkdirAll (writes dPath = filepath.Dir(entryPath)) and
	// CreateTemp (also dPath) are covered by the ancestor walk; Rename (writes
	// entryPath) is guarded directly.
	if guardErr := pathguard.RejectAndReport(resolvedRoot, entryPath); guardErr != nil {
		return "", guardErr
	}

	// Safety: refuse to create the entries directory if it is already a symlink.
	// The guard above catches symlinks in ancestors; this check catches a race where
	// the directory was replaced with a symlink after the guard ran.
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

	// write-containment-allowed: pathguard.RejectAndReport(resolvedRoot, entryPath) above
	if err := os.MkdirAll(dPath, 0755); err != nil {
		return "", fmt.Errorf("translog: mkdir %s: %w", dPath, err)
	}

	// write-containment-allowed: pathguard.RejectAndReport(resolvedRoot, entryPath) above
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
	// write-containment-allowed: pathguard.RejectAndReport(resolvedRoot, entryPath) above
	if err := os.Rename(tmpName, entryPath); err != nil {
		return "", fmt.Errorf("translog: rename to %s: %w", entryPath, err)
	}
	return name, nil
}

// HasLog reports whether dir has any log at all — either a legacy .trackfw-log file
// or a .trackfw-log-entries/ directory with at least one canonical .entry file.
//
// M5: uses os.Lstat (does not follow symlinks) for both paths, consistent with
// ReadAll which also uses os.Lstat on the entries directory — a symlink where
// .trackfw-log-entries/ should be is refused by ReadAll, so HasLog would mislead
// callers if it reported true for such a configuration.
//
// Uses entryNameRe (same filter as ReadAll) so that a directory containing only
// non-canonical files (tmp-*, .DS_Store) is not reported as having a log.
func HasLog(dir string) bool {
	if _, err := os.Lstat(dir + legacySuffix); err == nil {
		return true
	}
	dPath := dir + EntriesDirSuffix
	info, err := os.Lstat(dPath)
	if err != nil || !info.IsDir() {
		return false
	}
	des, err := os.ReadDir(dPath)
	if err != nil {
		return false
	}
	for _, de := range des {
		if entryNameRe.MatchString(de.Name()) && de.Type().IsRegular() {
			return true
		}
	}
	return false
}

// Package homedir resolves the user's home directory consistently across
// platforms. Canonical source of truth for npm/src/homedir.js (Node.js) and
// pypi/trackfw/homedir.py (Python).
//
// Why it exists: os.UserHomeDir() reads $HOME on Linux and macOS, but
// %USERPROFILE% on Windows. The trackfw test suites isolate the home directory
// with t.Setenv("HOME", t.TempDir()) — and the Node.js and Python suites do the
// equivalent — which on Windows isolates nothing: production keeps reading and
// writing the developer's real home. A single `go test ./...` run on a Windows
// machine created ADR files, an integrations manifest and two guard scripts
// inside the real ~/.trackfw.
//
// On a CI runner the same gap is a race, not just a mess: `go test` parallelizes
// packages by default, so several packages write to the one real home at once,
// and the failure surfaces as "flaky Windows test" rather than as the isolation
// defect it is.
//
// Dir() makes Windows behave like the other platforms: $HOME first,
// os.UserHomeDir() as the fallback. Where $HOME is unset nothing changes, and
// where it is set (Git Bash sets it) it points at the same place as
// %USERPROFILE%.
//
// The empty string does NOT count as set: HOME="" would resolve to "" and every
// derived path would silently become relative.
package homedir

import "os"

// Dir returns the user's home directory, preferring $HOME when it is set and
// non-empty. It replaces os.UserHomeDir() throughout trackfw's production code.
func Dir() (string, error) {
	if h := os.Getenv("HOME"); h != "" {
		return h, nil
	}
	return os.UserHomeDir()
}

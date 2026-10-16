// Package fsperm provides file-system permission helpers that go beyond POSIX
// mode bits. On Windows, os.Chmod(0o600) only toggles the READONLY attribute
// and leaves the NTFS DACL untouched. This package fills that gap by creating
// temporary files with an already-restricted DACL so that no race window exists
// between file creation and permission application.
//
// The package is build-tagged per platform:
//   - fsperm_windows.go  – real NTFS ACL implementation
//   - fsperm_other.go    – no-ops; delegates to os.CreateTemp (POSIX mode bits suffice)
//
// ADR: docs/adr/ADR-2026-10-16-dacl-explicita-para-arquivos-de-governanca-no-windows.md
package fsperm

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrNoACLSupport is a sentinel that signals the filesystem does not support
// NTFS ACLs (e.g. FAT, exFAT, SMB without ACLs). By ADR ajuste D4, this error
// is handled silently inside CreateRestrictedFile — callers receive a valid
// *os.File and a nil error. The sentinel is exported so tests can reference it.
var ErrNoACLSupport = errors.New("fsperm: filesystem does not support ACLs")

// IsTrackfwOwned reports whether path is the .trackfw directory itself or lies
// under a .trackfw ancestor directory. Such paths are considered property of the
// trackfw process and are re-protected even when they pre-existed (ADR D3 ajuste).
func IsTrackfwOwned(path string) bool {
	clean := filepath.Clean(path)
	for p := clean; ; p = filepath.Dir(p) {
		if strings.EqualFold(filepath.Base(p), ".trackfw") {
			return true
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
	}
	return false
}

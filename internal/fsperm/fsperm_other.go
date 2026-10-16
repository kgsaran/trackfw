//go:build !windows

package fsperm

import "os"

// CreateRestrictedFile is a no-op shim for non-Windows platforms. On POSIX
// systems, mode bits enforced by os.Chmod(0o600) are sufficient; no DACL
// manipulation is needed.
func CreateRestrictedFile(dir, pattern string) (*os.File, error) {
	// write-containment-allowed: non-Windows no-op; POSIX mode bits suffice; ACL protection not available on this platform
	return os.CreateTemp(dir, pattern)
}

// SetRestrictiveDirACL is a no-op on non-Windows platforms.
func SetRestrictiveDirACL(_ string) error { return nil }

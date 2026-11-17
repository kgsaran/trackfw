//go:build windows

package fileutil

import "os"

// openRegularFileNonblock is the Windows counterpart of fileutil_unix.go.
// Windows named pipes live in the separate \\.\pipe\ namespace and are not
// addressable as ordinary filesystem paths — a path under $HOME or a project
// directory on an NTFS/ReFS volume cannot resolve to a named pipe via mkfifo(1).
// The unix trick (open with O_NONBLOCK, fstat the fd) therefore has no equivalent
// on Windows that is both portable and low-effort.
//
// This function uses a stat-then-open sequence: check the file type at the path,
// then open it normally. This is a TOCTOU REDUCTION, not an elimination — the
// path can be swapped between Stat and Open in principle — but it is offered
// honestly as a reduction rather than a guarantee, consistent with other
// platform-gated behaviour in this repository.
//
// The residual race requires local write access to the exact path during the
// narrow window between the two syscalls, on a platform where the specific attack
// this package was written to close (a FIFO) is not constructible via ordinary
// filesystem operations.
func openRegularFileNonblock(path string) (*os.File, error) {
	info, statErr := os.Stat(path)
	if statErr != nil {
		return nil, statErr
	}
	if !info.Mode().IsRegular() {
		return nil, ErrNotRegularFile
	}
	return os.Open(path)
}

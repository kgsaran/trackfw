//go:build windows

package translog

import "os"

// openRegularFileNonblock is the Windows counterpart of regularfile_unix.go.
// Windows named pipes live in the \\.\pipe\ namespace and are not addressable
// through ordinary project-relative paths, so the FIFO attack that O_NONBLOCK
// closes on Unix does not apply here.
//
// This is a copy of internal/validator.openRegularFileNonblock (Windows version).
// The duplication is declared rather than hidden: translog cannot import validator.
// Any change to the algorithm in validator's copy must be mirrored here.
//
// This is a TOCTOU reduction, not an elimination: the path can be swapped between
// the Stat and the Open calls in principle, but the residual race requires local
// write access to the exact path during a narrow window on a platform where the
// specific attack this guards against does not exist.
func openRegularFileNonblock(path string) (*os.File, error) {
	info, statErr := os.Stat(path)
	if statErr != nil {
		return nil, statErr
	}
	if !info.Mode().IsRegular() {
		return nil, errNotRegularFile
	}
	return os.Open(path)
}

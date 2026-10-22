//go:build !windows

package translog

import (
	"os"
	"syscall"
)

// openRegularFileNonblock opens path for reading using O_NONBLOCK so that a FIFO
// with no writer on the other end never blocks the open() call itself, then
// fstats the resulting file descriptor — not the path — to confirm what was
// actually opened is a regular file.
//
// This is a copy of internal/validator.openRegularFileNonblock. The duplication is
// declared rather than hidden: translog cannot import validator (it would introduce
// a cycle), and the semantics must be identical. Any change to the algorithm in
// validator's copy must be mirrored here.
//
// Returns errNotRegularFile when the opened fd is not a regular file.
// Any other error is the raw *os.PathError from the platform open() call.
func openRegularFileNonblock(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	f := os.NewFile(uintptr(fd), path)
	info, statErr := f.Stat()
	if statErr != nil {
		f.Close()
		return nil, statErr
	}
	if !info.Mode().IsRegular() {
		f.Close()
		return nil, errNotRegularFile
	}
	return f, nil
}

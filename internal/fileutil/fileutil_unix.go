//go:build !windows

package fileutil

import (
	"os"
	"syscall"
)

// openRegularFileNonblock opens path for reading with O_NONBLOCK so that a FIFO
// with no writer on the other end never blocks the open() call (POSIX: a blocking
// open() of a FIFO's read end waits for a writer; O_NONBLOCK makes it return
// immediately), then fstats the resulting FILE DESCRIPTOR — not the path — to
// confirm the opened object is a regular file.
//
// Fstat-on-fd is what makes the type check immune to TOCTOU: the fd is bound to
// the inode (or pipe, or device) at open() time; whatever happens to the path
// after open() returns cannot change what this fd refers to.
//
// O_NONBLOCK has no effect on reads/writes of regular files (POSIX), so once this
// function confirms the fd is a regular file, its Read behaves identically to a
// fd opened without O_NONBLOCK — no fcntl(F_SETFL) call is needed.
//
// Returns ErrNotRegularFile when the fd is not a regular file. Any other error is
// the raw error from open() (e.g. *PathError wrapping ENOENT/EACCES), unchanged.
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
		return nil, ErrNotRegularFile
	}

	return f, nil
}

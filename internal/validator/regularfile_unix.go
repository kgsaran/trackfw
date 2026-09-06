//go:build !windows

package validator

import (
	"os"
	"syscall"
)

// openRegularFileNonblock opens path for reading using O_NONBLOCK so that a FIFO with no writer on
// the other end never blocks the open() call itself (POSIX: a blocking open() of a FIFO's read end
// waits for a writer to open the other end; O_NONBLOCK makes it return immediately instead), then
// fstats the resulting FILE DESCRIPTOR — not the path — to confirm what was actually opened is a
// regular file.
//
// Fstat-on-fd is what makes the type check immune to TOCTOU: a file descriptor is bound to the
// inode (or pipe, or device) it was opened against, not to the path string. Whatever happens to the
// PATH after this open() call returns — replaced, deleted, swapped for a FIFO by a racing writer —
// cannot change what this fd refers to, because the kernel already resolved the path to a concrete
// object at open() time. A stat()-then-open() sequence (stat the path, THEN open it) does NOT have
// this property — the path can be swapped between the two syscalls — which is why this function
// opens first and classifies second, not the other way around.
//
// O_NONBLOCK is not cleared before returning: POSIX defines O_NONBLOCK as having no effect on
// reads/writes of regular files (only pipes, FIFOs, sockets, and some devices honor it), so once
// this function has confirmed the fd is a regular file, its Read behaves identically to a fd opened
// without O_NONBLOCK — no fcntl(F_SETFL) call is needed to make that true.
//
// Returns errNotRegularFile (regularfile.go) when the opened fd is not a regular file. Any other
// error is the raw error open() returned (e.g. *PathError wrapping ENOENT/EACCES), unchanged from
// what os.ReadFile would have returned for the same failure — callers' existing os.IsNotExist(err)
// branches keep working exactly as before.
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

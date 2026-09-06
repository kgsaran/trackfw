//go:build windows

package validator

import "os"

// openRegularFileNonblock is the windows counterpart of regularfile_unix.go's function of the same
// name. Windows has no portable, low-effort equivalent of the unix trick (open with O_NONBLOCK,
// then fstat the FILE DESCRIPTOR to classify what was actually opened, immune to TOCTOU because the
// fd is bound to the object, not the path) — Windows named pipes are not addressed through ordinary
// filesystem paths like ".claude\\settings.json" at all; they live in the separate \\.\pipe\
// namespace and are created/opened via CreateNamedPipe/ConnectNamedPipe, a different API family
// entirely, not a drop-in replacement for os.Open. A path under a project or $HOME directory on an
// NTFS/ReFS volume cannot, in practice, resolve to a named pipe the way a POSIX path can resolve to
// a FIFO via mkfifo(1).
//
// This function therefore keeps a STAT-then-OPEN sequence: check the type at the path, then open it
// normally. This is a TOCTOU REDUCTION, not an elimination — the path can still be swapped between
// the Stat and the Open call, in principle — but it is offered honestly as a reduction, consistent
// with every other CurrentGOOS-gated behavior in this package, rather than claimed as closed. The
// residual race requires local write access to the exact path during the narrow window between
// these two syscalls, on a platform where the specific attack this ML was written to close (a FIFO)
// does not apply to begin with.
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

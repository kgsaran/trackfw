package translog

import (
	"errors"
	"io"
)

// errNotRegularFile is returned by readRegularFileBytes when the path resolves to
// something other than a regular file (FIFO, socket, device, directory).
//
// This is a duplicate of internal/validator's errNotRegularFile sentinel. The
// duplication is intentional: translog is a leaf package that cannot import
// validator, and both need the same FIFO-safe open primitive to protect against
// the indefinite-block attack documented in
// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio.
// Any behavioural change to the sentinel must be mirrored in both packages.
var errNotRegularFile = errors.New("not a regular file")

// readRegularFileBytes opens path for reading using openRegularFileNonblock (defined
// in regularfile_unix.go / regularfile_windows.go), confirms the fd refers to a
// regular file, and returns the contents.
//
// Returns errNotRegularFile when the path resolves to a non-regular file.
// Returns an os.IsNotExist error when the file does not exist.
// Any other error is the raw error from the platform open() call.
func readRegularFileBytes(path string) ([]byte, error) {
	f, err := openRegularFileNonblock(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

package translog

import (
	"errors"
	"fmt"
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

// errEntryTooLarge is returned by readRegularFileBytesLimited when the file
// exceeds the caller-supplied size limit.
var errEntryTooLarge = errors.New("entry exceeds size limit")

// readRegularFileBytesLimited is like readRegularFileBytes but returns
// errEntryTooLarge (wrapped with the path and limit) when the file content
// exceeds maxBytes. This lets callers distinguish a hard oversize error from a
// transient read failure (permissions, torn write) without relying on error
// string inspection.
//
// Uses io.LimitReader to avoid reading more than maxBytes+1 into memory, so a
// 1 GiB sparse file does not allocate 1 GiB of heap.
func readRegularFileBytesLimited(path string, maxBytes int64) ([]byte, error) {
	f, err := openRegularFileNonblock(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	lr := io.LimitReader(f, maxBytes+1)
	data, readErr := io.ReadAll(lr)
	if readErr != nil {
		return nil, readErr
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%s: %w (%d bytes)", path, errEntryTooLarge, maxBytes)
	}
	return data, nil
}

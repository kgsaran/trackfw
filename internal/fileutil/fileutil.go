// Package fileutil provides a fail-safe file reader that refuses to block on
// non-regular files (FIFOs, sockets, devices). It is a leaf package — it does
// not import any other internal package.
//
// Motivation (ML-FA2, REQ-2026-11-15): os.ReadFile on a POSIX named pipe (FIFO)
// with no writer on the other end blocks indefinitely. InertClaudeGuardPath and
// InertCodexGuardPath in internal/agentconfig both use os.ReadFile to probe the
// old-home remainder; a FIFO planted at ~/.codex/hooks.json or
// ~/.claude/settings.json hangs `update harness --install-missing`. This package
// provides ReadRegularFile which does not hang under those conditions.
//
// The unix implementation opens with O_NONBLOCK (so a FIFO with no writer does
// not block the open call) and then fstats the resulting file descriptor — not
// the path — to confirm that the opened object is a regular file. The Windows
// implementation uses a stat-then-open sequence (TOCTOU reduction, not
// elimination) because Windows named pipes are not reachable via ordinary
// filesystem paths and the platform does not need O_NONBLOCK for the same
// protection.
package fileutil

import (
	"errors"
	"io"
)

// ErrNotRegularFile is returned by ReadRegularFile when the path resolves to
// something other than a regular file: a FIFO, socket, character/block device,
// or directory. It is an exported sentinel so callers can distinguish it from
// ordinary absence errors (os.IsNotExist) using errors.Is.
//
// The check is a positive allowlist (Mode().IsRegular()), so every current and
// future special file type is excluded by construction — no branch is needed for
// each individual type.
var ErrNotRegularFile = errors.New("not a regular file")

// ReadRegularFile reads and returns the contents of path, refusing to block on
// non-regular files. If path resolves — after following symlinks — to anything
// other than a regular file, it returns ErrNotRegularFile without blocking.
//
// The returned error is never os.IsNotExist(err) except when the underlying
// open() itself failed with ENOENT (ordinary absence). ErrNotRegularFile is a
// distinct sentinel that callers' existing "was it absent?" branches already
// treat correctly without any new code path, because it falls through to the
// non-ENOENT branch by construction.
func ReadRegularFile(path string) ([]byte, error) {
	f, err := openRegularFileNonblock(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

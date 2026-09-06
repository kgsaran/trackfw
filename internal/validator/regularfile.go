package validator

import (
	"errors"
	"io"
)

// errNotRegularFile is returned by openRegularFileNonblock (regularfile_unix.go /
// regularfile_windows.go) when the path resolves — after following symlinks, exactly like an
// ordinary open() would — to something other than a regular file: FIFO, socket, character/block
// device, or (on unix) a directory that somehow slipped past open()'s own EISDIR.
//
// Checked, not exhaustively enumerated by type: "must be a regular file" is a POSITIVE allowlist
// (Mode().IsRegular()), so every current AND future special file type is excluded by construction —
// nothing here needs to name FIFO/socket/device individually, unlike a denylist that would need a
// new branch every time a new special type is found (ROADMAP-2026-09-06-fecha-o-fail-open-do-
// guard-config-ilegivel-deixa-de-ser-silencio, ML-1C: hades-tf's ML-1B barrier found FIFO by
// attack; a denylist would only ever cover what has already been found).
var errNotRegularFile = errors.New("not a regular file")

// readRegularFile is the fail-safe replacement for os.ReadFile used by every guard config/script
// read in this package: validateGuardHookResolvable and validateGuardGlobalHookResolvable
// (validator_credential_guard.go / validator_git_branch_guard.go), and the *_script_integrity
// family — validateCredentialGuardScriptIntegrity (validator_credential_guard_integrity.go),
// validateGitBranchGuardScriptIntegrity and validateGuardGlobalScriptIntegrity
// (validator_git_branch_guard.go). ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-
// deixa-de-ser-silencio, ML-1C.
//
// Why this exists: hades-tf's ML-1B barrier found that a FIFO in place of any of these files makes
// os.ReadFile block INDEFINITELY (`mkfifo .claude/settings.json` — `trackfw validate --json` never
// returns), reproduced live in all 3 runtimes, pre-existing behavior (not introduced by ML-1B) that
// this ML closes. See openRegularFileNonblock's unix implementation (regularfile_unix.go) for why
// fstat-ON-THE-OPEN-FILE-DESCRIPTOR — not stat-on-the-path — is what makes the type check immune to
// TOCTOU there. The windows implementation (regularfile_windows.go) is a reduction of the race
// window, not an elimination — see its doc comment for why a portable, low-effort windows
// equivalent of the unix trick does not exist.
//
// err from this function is NEVER os.IsNotExist(err) except when the underlying open() itself
// failed with ENOENT (ordinary absence) — errNotRegularFile is a distinct sentinel that every
// caller's existing "was it ENOENT?" branch already treats as "no, so accuse" without any new code
// path, because it already falls through to the non-ENOENT (accusing) branch by construction.
func readRegularFile(path string) ([]byte, error) {
	f, err := openRegularFileNonblock(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

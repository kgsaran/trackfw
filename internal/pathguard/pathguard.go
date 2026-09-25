// Package pathguard provides the containment predicate used by every write
// site in the trackfw CLI that derives a destination path from a user-supplied
// root (project root, $HOME, or a config-specified directory).
//
// # The problem
//
// os.Lstat(path) does NOT follow the last component of the path — but it DOES
// follow every ancestor component. A write guard that calls Lstat only on the
// leaf therefore misses a symlink placed in any ancestor directory, allowing the
// write to escape the intended tree silently. This is the vulnerability described
// in REQ-2026-08-31-guarda-de-folha-faz-lstat-so-no-ultimo-componente… and
// reproduced against the main-branch binary on 2026-09-18.
//
// # The predicates
//
// RejectSymlinks walks every ancestor of filename up to (and including) root,
// calling Lstat at each step. If any component is a symlink the call returns an
// error. Beneath asserts that filename is strictly inside root (not equal to
// root, not a sibling, not a parent) using filepath.Rel.
//
// Together they close both the "symlink in ancestor" and the "path escapes via
// .." attack surfaces.
//
// # Why this is not in internal/pathanchor
//
// internal/pathanchor's package doc draws an explicit boundary: "This package
// never governs an actual filesystem traversal, syscall, or path-building step —
// filepath.Clean, filepath.Join, filepath.Rel, os.Stat/os.Lstat … keep using
// path/filepath exactly as before." RejectSymlinks calls os.Lstat and
// filepath.Dir in a loop — that is exactly the other side of that boundary.
//
// # Extraction, not rewrite
//
// The implementation below is an exact extraction of the private functions
// rejectSymlinks and beneath from internal/integrations/manager.go (the class (c)
// reference — the only site that already protected writes correctly). Semantics
// are unchanged; internal/integrations/manager.go now delegates to this package
// via thin unexported forwarding functions so existing tests require zero edits.
//
// 🔴 Do NOT import internal/commands or internal/validator from this package —
// those packages import internal/generators which would introduce a cycle.
package pathguard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Beneath reports whether filename is strictly inside root: it must be
// reachable from root by a relative path that is neither "." nor ".." nor a
// path that starts with ".."+separator, and must not be an absolute path in its
// own right (which would mean filepath.Rel returned a result but the OS would
// not interpret it as root-relative).
//
// This is a pure path-string predicate: it does not touch the filesystem. Its
// job is to catch path traversal via ".." components; symlink traversal is
// handled by RejectSymlinks.
func Beneath(root, filename string) bool {
	relative, err := filepath.Rel(root, filename)
	return err == nil &&
		relative != "." &&
		relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator)) &&
		!filepath.IsAbs(relative)
}

// RejectSymlinks walks every path component of filename up to (and including)
// root, calling os.Lstat at each step. It returns a non-nil error if:
//
//   - any component along the walk is a symlink ("refusing symlink path"), or
//   - any component escapes root (path equal to or outside root before reaching
//     the root stop-condition).
//
// A non-existent component is not an error: the typical use case for this guard
// is writes that CREATE a new file, so the leaf (and sometimes its parent
// directory) legitimately does not exist yet.
//
// root must be the real (fully resolved) project root. filename must be an
// absolute path derived from root via filepath.Join or equivalent — passing a
// relative path will cause the guard to fail with an "escapes root" error
// immediately because filepath.Rel cannot establish containment.
func RejectSymlinks(root, filename string) error {
	current := filename
	for {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink path %q", current)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if current == root {
			return nil
		}
		parent := filepath.Dir(current)
		if parent == current || !Beneath(root, current) {
			return fmt.Errorf("path %q escapes root", filename)
		}
		current = parent
	}
}

// GuardedWrite applies RejectSymlinks(root, filename) and, if the check
// passes, writes data to filename atomically: it creates a temporary file in
// the same directory, writes and syncs it, then renames it into place.
//
// This consolidates the private atomicWrite functions that previously lived in
// internal/identity and internal/thirdparty/quarantine — both packages
// declared those copies as mirrors of internal/integrations/manager.go's
// atomicWrite. Centralising here removes three divergent implementations of
// the same pattern and ensures every caller inherits the containment check
// for free.
//
// root must be the real (fully resolved) parent boundary — the same constraint
// as RejectSymlinks, which GuardedWrite calls first so that no directory is
// created before the guard fires.
//
// The guard runs through RejectAndReport, not RejectSymlinks: all three callers
// (internal/identity, internal/thirdparty/quarantine, internal/thirdparty/
// provenance) wrapped the error and printed nothing, so a refusal here used to
// be silent — the same defect as the three mute call sites ML-7B fixed. Same
// mechanism, same fix.
func GuardedWrite(root, filename string, data []byte, mode os.FileMode) error {
	if err := RejectAndReport(root, filename); err != nil {
		return err
	}
	directory := filepath.Dir(filename)
	// write-containment-allowed: GuardedWrite implements the containment helper itself; MkdirAll is always called after RejectSymlinks guard passes
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	// write-containment-allowed: GuardedWrite implements the containment helper itself; CreateTemp is always called after RejectSymlinks guard passes
	temporary, err := os.CreateTemp(directory, ".trackfw-tmp-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName) //nolint:errcheck
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close() //nolint:errcheck
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close() //nolint:errcheck
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close() //nolint:errcheck
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	// write-containment-allowed: GuardedWrite implements the containment helper itself; Rename is always called after RejectSymlinks guard passes
	return os.Rename(temporaryName, filename)
}

// refusalGrammar is the ONE grammar of the containment refusal. Both the
// user-facing stderr line and the returned error are built from this single
// literal, so they cannot drift apart: identical by construction, not by
// textual coincidence. This is what AC5 of REQ-2026-08-31 demands.
//
// The trailing verb is appended by the two call forms below ("%v" for the
// stderr line, "%w" for the returned error) because the language forces that
// difference — fmt.Fprintf does not accept %w and fmt.Errorf must wrap. The
// grammar itself is not duplicated.
//
// 🔴 Do not add a second grammar here, and do not give RejectAndReport a
// "display path" parameter: a per-caller knob over the message re-creates the
// divergence (5 grammars, measured by ML-6A on 2026-09-25) that this constant
// removes.
const refusalGrammar = "refusing write to %s: "

// stderrPrefix identifies the binary and belongs only to the stderr line — the
// returned error is wrapped further up by callers and must not carry it.
const stderrPrefix = "trackfw: "

// RejectAndReport is the ONLY site in the binary that emits the containment
// refusal. Every write site that guards a destination path derived from a
// user-supplied root calls this instead of pairing RejectSymlinks with its own
// fmt.Fprintf(os.Stderr, …).
//
// Before this function existed the pair was implemented 53 times (4 named
// helpers + 49 inline copies) across 5 distinct message grammars, and 3 of the
// sites refused silently — measured by ML-6A on 2026-09-25.
//
// resolvedRoot SHOULD come from an approved resolver (projectRoot /
// scaffoldRoot / resolveRoot / filepath.EvalSymlinks). The parameter is named
// so that an AST provenance check can key on it (ML-7C).
//
// 🔴 This function deliberately does NOT validate or normalise resolvedRoot.
// Several call sites still pass filepath.Clean(cwd); fixing that ARGUMENT is
// Wave 8, and silently resolving it here would erase what Wave 8 must measure.
//
// It returns nil when the path is contained. The returned error already names
// the refused path — callers must NOT re-wrap it with a second
// "refusing write to …" or the message doubles.
func RejectAndReport(resolvedRoot, absTarget string) error {
	guardErr := RejectSymlinks(resolvedRoot, absTarget)
	if guardErr == nil {
		return nil
	}
	fmt.Fprintf(os.Stderr, stderrPrefix+refusalGrammar+"%v\n", absTarget, guardErr)
	return fmt.Errorf(refusalGrammar+"%w", absTarget, guardErr)
}

// unverifiableReason is the ONE reason clause used when the root itself could
// not be established, so RejectSymlinks could never run. It composes with
// refusalGrammar rather than restating it.
const unverifiableReason = "cannot verify containment: "

// RefuseUnverifiableRoot is the fail-closed sibling of RejectAndReport, and the
// ONLY other site in the binary that emits a containment refusal to the user.
//
// It is called when the root could not be resolved at all (os.Getwd failed, the
// caller passed an empty project root, …) — the guard cannot run, so the write
// is refused rather than proceeding unguarded. Keeping the emission here is what
// makes "no containment refusal is printed outside package pathguard" a
// mechanical property instead of a convention.
//
// cause must be non-nil: it is what the user needs in order to diagnose the
// refusal.
func RefuseUnverifiableRoot(target string, cause error) error {
	fmt.Fprintf(os.Stderr, stderrPrefix+refusalGrammar+unverifiableReason+"%v\n", target, cause)
	return fmt.Errorf(refusalGrammar+unverifiableReason+"%w", target, cause)
}

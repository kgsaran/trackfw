package generators

// TestScripts_LiteralMatchesVersionedCopy — ML-3D (REQ-2026-09-23, Wave 3)
//
// Regra Dura de Reconciliação:
//
//   - credential-guard (t.Run "credential-guard"): this test asserts that
//     trackfw-credential-guard.sh and trackfw-git-branch-guard.sh are literal/copy
//     pairs of the same nature as the attention pair: byte-identical today, same
//     generator structure, same absent guard — and therefore subject to the same
//     silent revert that #414 produced (literal fixed, copy not, or copy fixed and
//     a later `trackfw init` regenerated the copy from the unfixed literal).
//
//   - git-branch-guard (t.Run "git-branch-guard"): same conclusion as
//     credential-guard (see above).
//
//   - validate (t.Run "validate"): this test asserts that the content
//     buildValidateScript(Config{}) produces is byte-identical to the versioned copy
//     scripts/trackfw-validate.sh, which was generated from an empty Config.
//     generateValidateScript(cfg) is not called directly because it writes to a
//     CWD-relative path (os.MkdirAll("scripts")/os.WriteFile("scripts/...")) and
//     is unsafe in tests (Chdir trap documented in
//     vault/notes/copia-versionada-do-attention-signal-esta-obsoleta-e-sem-guarda-2026-09-02.md).
//     The direct buildValidateScript call is semantically equivalent for the empty-Config case.
//
// Population covered: 5 of 5 pairs (signal, cleanup, credential-guard, git-branch-guard, validate).
//
// Background (attention pair):
//
//	- attentionSignalScript (the literal in scaffold.go) and
//	  scripts/trackfw-attention-signal.sh (the versioned copy) are two natures of
//	  the same artefact.  Nothing compared them.
//	- In #414 the correction was applied to the versioned copy only; the literal
//	  stayed defective for weeks.  Later, a `trackfw init` run regenerated the
//	  versioned copy from the defective literal, silently reverting the fix.
//	- vault/notes/copia-versionada-do-attention-signal-esta-obsoleta-e-sem-guarda-2026-09-02.md
//	  documented the absent guard since 2026-09-02; nobody created it.
//
// How the cwd-relative path trap (present in getGoScripts) is avoided here:
//   - GenerateAttentionScripts(genDir), GenerateCredentialGuardScript(genDir), and
//     GenerateGitBranchGuardScript(genDir) each receive an explicit absolute path, so
//     no os.Chdir call is needed.  The versioned copies are read via findRepoRoot,
//     which walks up from os.Getwd() looking for go.mod — never from a TempDir.
//   - buildValidateScript(Config{}) is a pure function returning a string; no path is
//     involved.

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScripts_LiteralMatchesVersionedCopy(t *testing.T) {
	root := findRepoRoot(t) // defined in scaffold_parity_test.go; walks up via go.mod, never cwd-dependent after Chdir
	scriptsDir := filepath.Join(root, "scripts")

	if _, err := os.Stat(scriptsDir); os.IsNotExist(err) {
		t.Skip("scripts/ directory absent — not the maintainer tree; guard not applicable here")
	}

	// cases covers the four generator-based pairs.  Each generator receives the same
	// genDir so all four scripts land under genDir/scripts/ before the comparison loop.
	cases := []struct {
		name          string
		versionedFile string
		fixLiteral    string // name(s) of the Go constant / function to regenerate from
	}{
		{
			"signal",
			"trackfw-attention-signal.sh",
			"attentionSignalScript in internal/generators/scaffold.go",
		},
		{
			"cleanup",
			"trackfw-attention-cleanup.sh",
			"attentionCleanupScript in internal/generators/scaffold.go",
		},
		{
			"credential-guard",
			"trackfw-credential-guard.sh",
			"credentialGuardScript (= credentialGuardHeader + credentialGuardProjectGuardBlock + " +
				"credentialGuardDetectionCore + credentialGuardProjectTail) in internal/generators/scaffold.go",
		},
		{
			"git-branch-guard",
			"trackfw-git-branch-guard.sh",
			"gitBranchGuardScript in internal/generators/scaffold.go",
		},
	}

	// Generate all four scripts into a single fresh temp dir.
	// No os.Chdir — each path is absolute, so the cwd-relative trap cannot occur.
	genDir := t.TempDir()
	if err := GenerateAttentionScripts(genDir); err != nil {
		t.Fatalf("GenerateAttentionScripts(%q) failed: %v", genDir, err)
	}
	if err := GenerateCredentialGuardScript(genDir); err != nil {
		t.Fatalf("GenerateCredentialGuardScript(%q) failed: %v", genDir, err)
	}
	if err := GenerateGitBranchGuardScript(genDir); err != nil {
		t.Fatalf("GenerateGitBranchGuardScript(%q) failed: %v", genDir, err)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			versionedPath := filepath.Join(scriptsDir, tc.versionedFile)
			generatedPath := filepath.Join(genDir, "scripts", tc.versionedFile)

			versionedBytes, err := os.ReadFile(versionedPath)
			if err != nil {
				if os.IsNotExist(err) {
					t.Errorf("versioned copy scripts/%s is missing from the repo — was it deleted? "+
						"Fix: regenerate it from the literal %s; "+
						"the copy is derived, never the source.",
						tc.versionedFile, tc.fixLiteral)
					return
				}
				t.Fatalf("reading versioned copy %s: %v", versionedPath, err)
			}

			generatedBytes, err := os.ReadFile(generatedPath)
			if err != nil {
				t.Fatalf("reading generated file %s: %v", generatedPath, err)
			}

			if string(versionedBytes) == string(generatedBytes) {
				return // pass
			}

			divergenceError(t, tc.versionedFile, tc.fixLiteral, versionedBytes, generatedBytes)
		})
	}

	// validate — special case: generateValidateScript(cfg Config) writes to a CWD-relative
	// path and is unsafe in tests (see file-level comment).  buildValidateScript(Config{})
	// is the pure function it calls internally; scripts/trackfw-validate.sh was generated
	// from an empty Config (no Backend, no Frontend), so the comparison is equivalent.
	t.Run("validate", func(t *testing.T) {
		versionedPath := filepath.Join(scriptsDir, "trackfw-validate.sh")

		versionedBytes, err := os.ReadFile(versionedPath)
		if err != nil {
			if os.IsNotExist(err) {
				t.Errorf("versioned copy scripts/trackfw-validate.sh is missing from the repo — was it deleted? " +
					"Fix: regenerate it by calling buildValidateScript(Config{}) in " +
					"internal/generators/scaffold.go and writing the result to scripts/trackfw-validate.sh; " +
					"the copy is derived, never the source.")
				return
			}
			t.Fatalf("reading versioned copy scripts/trackfw-validate.sh: %v", err)
		}

		generated := buildValidateScript(Config{})

		if string(versionedBytes) == generated {
			return // pass
		}

		divergenceError(t,
			"trackfw-validate.sh",
			"buildValidateScript(Config{}) in internal/generators/scaffold.go",
			versionedBytes, []byte(generated))
	})
}

// divergenceError emits a legible test failure that names the fix direction and
// locates the first differing byte.  Shared by all sub-tests in this file.
func divergenceError(t *testing.T, versionedFile, fixLiteral string, versionedBytes, generatedBytes []byte) {
	t.Helper()

	minLen := len(versionedBytes)
	if len(generatedBytes) < minLen {
		minLen = len(generatedBytes)
	}
	diffOffset := minLen // default: lengths differ, point past the common prefix
	for i := 0; i < minLen; i++ {
		if versionedBytes[i] != generatedBytes[i] {
			diffOffset = i
			break
		}
	}

	const contextRadius = 60
	start := diffOffset - contextRadius
	if start < 0 {
		start = 0
	}
	endV := diffOffset + contextRadius
	if endV > len(versionedBytes) {
		endV = len(versionedBytes)
	}
	endG := diffOffset + contextRadius
	if endG > len(generatedBytes) {
		endG = len(generatedBytes)
	}

	t.Errorf(
		"scripts/%s (versioned copy) diverges from what the generator produces.\n\n"+
			"FIX DIRECTION: regenerate the versioned copy from\n"+
			"  %s.\n"+
			"The versioned copy is DERIVED from the literal — NEVER edit the copy and\n"+
			"back-port it into the literal; that is what caused this guard to be necessary.\n\n"+
			"First differing byte at offset %d.\n"+
			"  versioned: %q\n"+
			"  generated: %q\n"+
			"Lengths: versioned=%d  generated=%d",
		versionedFile,
		fixLiteral,
		diffOffset,
		versionedBytes[start:endV],
		generatedBytes[start:endG],
		len(versionedBytes), len(generatedBytes),
	)
}

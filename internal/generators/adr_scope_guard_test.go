package generators

// adr_scope_guard_test.go — ML-7A: closes the LIVE escape in `trackfw adr new`.
//
// The defect (§R-1 of docs/seguranca/2026-09-25-ponto-unico-de-contencao-...):
// adr.go chose its guard root with Beneath(projectRoot(), filepath.Abs(adrDir)).
// projectRoot() is EvalSymlinks-resolved; filepath.Abs inherits the LOGICAL cwd
// (Go's os.Getwd honours $PWD when it stat-matches "."). With a plain `cd /tmp/p`
// the two operands live in different namespaces, Beneath returns false, and the
// code DEGRADED from project scope to global scope — which by design never
// inspects the ancestors above adrDir. `trackfw adr new` then wrote outside the
// project through a symlinked `docs`, rc=0.
//
// Reconciliation — one sentence per new test, stating which conclusion of this ML
// the test asserts:
//
//   - TestNewADR_SymlinkAncestorRefused_BothPWDArms
//     Affirms: with `docs` symlinked outside the project, NewADR/NewADRDraft refuse
//     in BOTH $PWD arms (resolved and unresolved) and write nothing into the victim,
//     while the control (NewREQ, which already derived root and target from the same
//     projectRoot()) keeps refusing with the same "refusing symlink path" message —
//     so a green result cannot mean "nothing writes anymore".
//   - TestAdrGuardPaths_RelativeAdrDirNeverDegradesToGlobalScope
//     Affirms: for a RELATIVE adrDir the scope is project UNCONDITIONALLY — guardRoot
//     is projectRoot() and the target is Join(projectRoot(), adrDir) — which is the
//     falsification arm against the obvious-but-wrong fix: applying
//     EvalSymlinks(absAdrDir) would make both of them point at the VICTIM and this
//     test would fail.
//   - TestAdrGuardPaths_AbsoluteAdrDirKeepsGlobalScope
//     Affirms: the global branch is untouched by this ML — an absolute adrDir outside
//     the project keeps root = adrDir (the ADR decision-3 named exception), and an
//     absolute adrDir genuinely inside the project is UPGRADED to project-root scope,
//     so Beneath survives only where it can raise strictness, never lower it.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// adrEscapeFixture builds the R-1 reproduction on disk:
//
//	base/proj          — the project (real directory), the test's cwd
//	base/proj/docs     — SYMLINK pointing at base/victim (the ancestor attack)
//	base/alias         — SYMLINK pointing at base/proj (used to forge an
//	                     unresolved $PWD deterministically on every platform,
//	                     instead of depending on /tmp being a symlink)
//	base/victim        — the directory outside the project that must stay empty
type adrEscapeFixture struct {
	project string // resolved (EvalSymlinks) path of the project directory
	alias   string // symlink whose target is the project directory
	victim  string // resolved path of the victim directory
}

func newADREscapeFixture(t *testing.T) adrEscapeFixture {
	t.Helper()
	base := t.TempDir()
	project := filepath.Join(base, "proj")
	victim := filepath.Join(base, "victim")
	// victim/adr is PRE-CREATED on purpose: it is the state left by any earlier
	// successful escape (or by the victim's own layout), and it is what makes
	// filepath.EvalSymlinks(<proj>/docs/adr) SUCCEED and land inside the victim.
	// Without it EvalSymlinks fails on the missing leaf and the obvious-but-wrong
	// fix would be falsified only by accident, for the wrong reason.
	for _, d := range []string{project, filepath.Join(victim, "adr")} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	// docs -> victim: the symlink ANCESTOR of docs/adr. Note this is NOT the
	// adrDir leaf — the pre-existing TestNewADR_SymlinkAdrDirRefused covers the
	// leaf, and the leaf was caught even under the degraded global scope, which
	// is exactly why this defect survived.
	// check-symlink-privilege-guard: symlinkOrSkip is the creation site below
	symlinkOrSkip(t, victim, filepath.Join(project, "docs"))
	alias := filepath.Join(base, "alias")
	// check-symlink-privilege-guard: symlinkOrSkip is the creation site below
	symlinkOrSkip(t, project, alias)

	resolvedProject, err := filepath.EvalSymlinks(project)
	if err != nil {
		t.Fatalf("EvalSymlinks(project): %v", err)
	}
	resolvedVictim, err := filepath.EvalSymlinks(victim)
	if err != nil {
		t.Fatalf("EvalSymlinks(victim): %v", err)
	}
	return adrEscapeFixture{project: resolvedProject, alias: alias, victim: resolvedVictim}
}

// enterResolvedArm chdirs so that os.Getwd() == projectRoot() (the arm that
// already refused before this ML).
func (f adrEscapeFixture) enterResolvedArm(t *testing.T) {
	t.Helper()
	chdirADR(t, f.project)
	t.Setenv("PWD", f.project)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := projectRoot()
	if err != nil {
		t.Fatalf("projectRoot: %v", err)
	}
	if cwd != root {
		t.Fatalf("fixture broken: resolved arm requires os.Getwd() == projectRoot(), got %q vs %q", cwd, root)
	}
}

// enterUnresolvedArm chdirs through the alias symlink and forges $PWD so that
// os.Getwd() returns the UNRESOLVED path while projectRoot() returns the
// resolved one — the namespace mismatch that produced the escape.
//
// On Windows os.Getwd() ignores $PWD (it returns syscall.Getwd() directly), so
// the two arms would collapse into one and this arm would pass while measuring
// nothing. In that case it skips, naming what was not exercised.
func (f adrEscapeFixture) enterUnresolvedArm(t *testing.T) {
	t.Helper()
	chdirADR(t, f.alias)
	t.Setenv("PWD", f.alias)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := projectRoot()
	if err != nil {
		t.Fatalf("projectRoot: %v", err)
	}
	if cwd == root {
		t.Skipf("arm $PWD-nao-resolvido nao exercitada: os.Getwd() nao honra $PWD nesta plataforma (Getwd=%q == projectRoot=%q)", cwd, root)
	}
}

// assertVictimUntouched fails if any .md file reached the directory outside the
// project — the containment property itself, independent of the error message.
func assertVictimUntouched(t *testing.T, fixture adrEscapeFixture, label string) {
	t.Helper()
	entries, err := os.ReadDir(fixture.victim)
	if err != nil {
		t.Fatalf("%s: reading victim dir: %v", label, err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".md") {
			t.Errorf("%s: containment violated — %s was written OUTSIDE the project", label, e.Name())
		}
	}
	// docs/adr is reached through the symlink; check the nested form too.
	nested, err := os.ReadDir(filepath.Join(fixture.victim, "adr"))
	if err != nil {
		return // directory was never created — the strongest form of "untouched"
	}
	for _, e := range nested {
		if strings.HasSuffix(e.Name(), ".md") {
			t.Errorf("%s: containment violated — adr/%s was written OUTSIDE the project", label, e.Name())
		}
	}
}

// assertRefusedBySymlink fails unless err is non-nil AND names the symlink
// refusal. Asserting only err != nil would accept an unrelated failure (a
// config error, an agent-ambiguity error) as if it were containment.
func assertRefusedBySymlink(t *testing.T, err error, label string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected refusal, got nil", label)
	}
	if !strings.Contains(err.Error(), "refusing symlink path") {
		t.Errorf("%s: refused for the WRONG reason — want a %q error, got: %v", label, "refusing symlink path", err)
	}
}

// TestNewADR_SymlinkAncestorRefused_BothPWDArms is the R-1 reproduction turned
// into a test: both $PWD arms must refuse, and the control must keep refusing.
func TestNewADR_SymlinkAncestorRefused_BothPWDArms(t *testing.T) {
	arms := []struct {
		name  string
		enter func(adrEscapeFixture, *testing.T)
	}{
		{"pwd-resolved", func(f adrEscapeFixture, t *testing.T) { f.enterResolvedArm(t) }},
		{"pwd-unresolved", func(f adrEscapeFixture, t *testing.T) { f.enterUnresolvedArm(t) }},
	}
	for _, arm := range arms {
		t.Run(arm.name, func(t *testing.T) {
			fixture := newADREscapeFixture(t)
			arm.enter(fixture, t)
			config.Reset()
			t.Cleanup(config.Reset)

			assertRefusedBySymlink(t, NewADR(ADRContent{Title: "escape probe"}, "docs/adr"), "NewADR")
			assertVictimUntouched(t, fixture, "NewADR")

			_, draftErr := NewADRDraft("escape-probe-draft", "docs/adr")
			assertRefusedBySymlink(t, draftErr, "NewADRDraft")
			assertVictimUntouched(t, fixture, "NewADRDraft")

			// Control arm: NewREQ already derived root and target from the same
			// projectRoot() and refused in both arms before this ML. If it stopped
			// refusing (or started failing for another reason) the arms above would
			// no longer discriminate "fixed" from "nothing writes anymore".
			assertRefusedBySymlink(t, NewREQ(REQContent{Title: "control probe"}), "NewREQ (control)")
			assertVictimUntouched(t, fixture, "NewREQ (control)")
		})
	}
}

// TestAdrGuardPaths_RelativeAdrDirNeverDegradesToGlobalScope is the
// falsification arm against the obvious-but-wrong fix.
func TestAdrGuardPaths_RelativeAdrDirNeverDegradesToGlobalScope(t *testing.T) {
	fixture := newADREscapeFixture(t)
	fixture.enterUnresolvedArm(t)

	guardRoot, absAdrDir, err := adrGuardPaths("docs/adr")
	if err != nil {
		t.Fatalf("adrGuardPaths: %v", err)
	}
	if guardRoot != fixture.project {
		t.Errorf("relative adrDir must take project scope unconditionally: guardRoot = %q, want %q", guardRoot, fixture.project)
	}
	if want := filepath.Join(fixture.project, "docs", "adr"); absAdrDir != want {
		t.Errorf("relative adrDir target must be Join(projectRoot(), adrDir): absAdrDir = %q, want %q", absAdrDir, want)
	}
	// The wrong fix — EvalSymlinks(absAdrDir) — lands both operands inside the
	// victim, where RejectSymlinks finds nothing to reject and the write escapes.
	if strings.HasPrefix(guardRoot, fixture.victim) || strings.HasPrefix(absAdrDir, fixture.victim) {
		t.Errorf("guard operands resolved into the VICTIM tree (the EvalSymlinks-before-guard anti-pattern): guardRoot=%q absAdrDir=%q victim=%q", guardRoot, absAdrDir, fixture.victim)
	}
}

// TestAdrGuardPaths_AbsoluteAdrDirKeepsGlobalScope pins the branch this ML does
// NOT target: absolute adrDir.
func TestAdrGuardPaths_AbsoluteAdrDirKeepsGlobalScope(t *testing.T) {
	fixture := newADREscapeFixture(t)
	fixture.enterResolvedArm(t)

	outside := filepath.Join(fixture.victim, "global-adr")
	guardRoot, absAdrDir, err := adrGuardPaths(outside)
	if err != nil {
		t.Fatalf("adrGuardPaths(absolute outside): %v", err)
	}
	if guardRoot != outside || absAdrDir != outside {
		t.Errorf("absolute adrDir outside the project must keep global scope (root = adrDir): guardRoot=%q absAdrDir=%q, want both %q", guardRoot, absAdrDir, outside)
	}

	// Absolute AND genuinely inside the project: Beneath upgrades the scope to the
	// full project root. It can only raise strictness, never lower it.
	inside := filepath.Join(fixture.project, "docs-real", "adr")
	insideRoot, insideAbs, err := adrGuardPaths(inside)
	if err != nil {
		t.Fatalf("adrGuardPaths(absolute inside): %v", err)
	}
	if insideRoot != fixture.project {
		t.Errorf("absolute adrDir inside the project must be upgraded to project scope: guardRoot=%q, want %q", insideRoot, fixture.project)
	}
	if insideAbs != inside {
		t.Errorf("absolute adrDir must not be rewritten: absAdrDir=%q, want %q", insideAbs, inside)
	}
}

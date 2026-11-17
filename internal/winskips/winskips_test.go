// Package winskips — gate tests.
//
// TestGate_RealTree: asserts that every GOOS-conditional test exit in the
// module is documented in .github/windows-skips.json (gate against the
// live tree).
//
// TestGate_Arm1_UnregisteredSkip: a new Windows-conditional skip with no
// registry entry produces a ViolMissing violation naming that function.
//
// TestGate_Arm2_LogReturnAsSkip: t.Log(…) + return under a Windows branch
// (a functional skip that emits no SKIP line) also produces ViolMissing.
//
// TestGate_Arm3_NoEvidence: a registry entry with an empty evidence field
// produces ViolNoEvidence even when the function is otherwise correct.
//
// TestGate_Arm4_ObsoleteEntry: a registry entry whose test key matches no
// marked function produces ViolObsolete.
//
// TestGate_Arm5_Control: a correctly registered Windows-conditional skip
// produces zero violations.
package winskips

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// fixtureModRoot creates a minimal module in t.TempDir() and returns its path.
// pkgName is the sub-directory / package name, src is written as pkgName/fix_test.go.
func fixtureModRoot(t *testing.T, pkgName, src string) string {
	t.Helper()
	root := t.TempDir()
	modFile := "module example.com/fix\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(modFile), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	pkgDir := filepath.Join(root, pkgName)
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "fix_test.go"), []byte(src), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return root
}

func violKinds(vs []Violation) []ViolKind {
	ks := make([]ViolKind, len(vs))
	for i, v := range vs {
		ks[i] = v.Kind
	}
	return ks
}

func violKeys(vs []Violation) []string {
	ks := make([]string, len(vs))
	for i, v := range vs {
		ks[i] = v.Key
	}
	sort.Strings(ks)
	return ks
}

func hasKind(vs []Violation, k ViolKind) bool {
	for _, v := range vs {
		if v.Kind == k {
			return true
		}
	}
	return false
}

func hasKey(vs []Violation, substr string) bool {
	for _, v := range vs {
		if strings.Contains(v.Key, substr) {
			return true
		}
	}
	return false
}

// ── TestGate_RealTree ─────────────────────────────────────────────────────────

// TestGate_RealTree asserts that the real module tree produces zero violations
// when checked against .github/windows-skips.json.
// Reconciliation: this test asserts that the live registry is complete and
// correct — every GOOS-conditional test exit is classified, every entry is
// non-obsolete, and every entry has evidence.
func TestGate_RealTree(t *testing.T) {
	root, err := ModuleRoot(".")
	if err != nil {
		t.Fatalf("ModuleRoot: %v", err)
	}
	reg, err := LoadRegistry(root)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	vs, err := Check(root, reg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) == 0 {
		return
	}

	// Separate by kind for a clear report.
	var missing, obsolete, noEvid, badClass []string
	for _, v := range vs {
		switch v.Kind {
		case ViolMissing:
			missing = append(missing, v.Key)
		case ViolObsolete:
			obsolete = append(obsolete, v.Key)
		case ViolNoEvidence:
			noEvid = append(noEvid, v.Key)
		case ViolBadClassification:
			badClass = append(badClass, v.Key)
		}
	}
	sort.Strings(missing)
	sort.Strings(obsolete)
	sort.Strings(noEvid)
	sort.Strings(badClass)

	msg := fmt.Sprintf("gate violations (%d total):", len(vs))
	if len(missing) > 0 {
		msg += fmt.Sprintf("\n  MISSING entries (%d) — add to .github/windows-skips.json:\n    %s",
			len(missing), strings.Join(missing, "\n    "))
	}
	if len(obsolete) > 0 {
		msg += fmt.Sprintf("\n  OBSOLETE entries (%d) — remove from .github/windows-skips.json:\n    %s",
			len(obsolete), strings.Join(obsolete, "\n    "))
	}
	if len(noEvid) > 0 {
		msg += fmt.Sprintf("\n  NO EVIDENCE (%d) — add evidence field:\n    %s",
			len(noEvid), strings.Join(noEvid, "\n    "))
	}
	if len(badClass) > 0 {
		msg += fmt.Sprintf("\n  BAD CLASSIFICATION (%d) — fix classification to a, b, or c:\n    %s",
			len(badClass), strings.Join(badClass, "\n    "))
	}
	t.Fatal(msg)
}

// ── Arm 1: unregistered skip ──────────────────────────────────────────────────

// TestGate_Arm1_UnregisteredSkip asserts that a new Windows-conditional
// t.Skip with no registry entry causes a ViolMissing violation that names
// the specific function.
// Reconciliation: this test asserts that the gate detects a new unclassified
// GOOS skip — the core threat from REQ-IMP-1.
func TestGate_Arm1_UnregisteredSkip(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm1_NewWindowsSkip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("arm1: unregistered windows skip")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	vs, err := Check(root, nil) // empty registry
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasKind(vs, ViolMissing) {
		t.Fatalf("expected ViolMissing, got %v", violKinds(vs))
	}
	if !hasKey(vs, "TestArm1_NewWindowsSkip") {
		t.Fatalf("ViolMissing should name TestArm1_NewWindowsSkip; keys: %v", violKeys(vs))
	}
}

// ── Arm 2: t.Log + return as functional skip ──────────────────────────────────

// TestGate_Arm2_LogReturnAsSkip asserts that a Windows branch containing
// t.Log(…) followed by return (which exits without assertion, like t.Skip
// but without emitting a SKIP line) also causes a ViolMissing violation.
// Reconciliation: this test asserts that the HIGH-severity threat from
// docs/seguranca/2026-11-15-wave0-windows-threat-model.md (surface F,
// t.Log+return pattern) is caught by the gate.
func TestGate_Arm2_LogReturnAsSkip(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm2_LogReturn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Log("arm2: functional skip via return, not t.Skip")
		return
	}
	// non-windows assertion
	if 1 != 1 {
		t.Fatal("unreachable")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	vs, err := Check(root, nil)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasKind(vs, ViolMissing) {
		t.Fatalf("expected ViolMissing for t.Log+return pattern, got %v", violKinds(vs))
	}
	if !hasKey(vs, "TestArm2_LogReturn") {
		t.Fatalf("ViolMissing should name TestArm2_LogReturn; keys: %v", violKeys(vs))
	}
}

// ── Arm 3: entry without evidence ────────────────────────────────────────────

// TestGate_Arm3_NoEvidence asserts that a registry entry with an empty
// evidence field produces ViolNoEvidence even when the function key is correct.
// Reconciliation: this test asserts that classification "c" without a
// mechanism citation is rejected — closing the reclassification threat T1
// from the wave-0 threat model.
func TestGate_Arm3_NoEvidence(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm3_NoEvidence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("arm3: skip with entry but no evidence")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	reg := []Entry{{
		Test:           "mypkg/fix_test.go:TestArm3_NoEvidence",
		Classification: "c",
		Reason:         "some reason",
		Evidence:       "", // ← deliberately empty
	}}
	vs, err := Check(root, reg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasKind(vs, ViolNoEvidence) {
		t.Fatalf("expected ViolNoEvidence, got %v", violKinds(vs))
	}
	if !hasKey(vs, "TestArm3_NoEvidence") {
		t.Fatalf("ViolNoEvidence should name TestArm3_NoEvidence; keys: %v", violKeys(vs))
	}
}

// ── Arm 4: obsolete entry ────────────────────────────────────────────────────

// TestGate_Arm4_ObsoleteEntry asserts that a registry entry whose test key
// matches no marked function in the tree produces ViolObsolete.
// Reconciliation: this test asserts that stale entries (pointing to deleted
// or refactored tests) are caught, keeping the registry accurate.
func TestGate_Arm4_ObsoleteEntry(t *testing.T) {
	src := `package mypkg

import "testing"

func TestArm4_NoSkip(t *testing.T) {
	// No GOOS branch at all — should not be marked.
	if 1 != 1 {
		t.Fatal("unreachable")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	reg := []Entry{{
		Test:           "mypkg/fix_test.go:TestArm4_DeletedSkip",
		Classification: "c",
		Reason:         "this function was removed",
		Evidence:       "proof",
	}}
	vs, err := Check(root, reg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasKind(vs, ViolObsolete) {
		t.Fatalf("expected ViolObsolete, got %v", violKinds(vs))
	}
	if !hasKey(vs, "TestArm4_DeletedSkip") {
		t.Fatalf("ViolObsolete should name TestArm4_DeletedSkip; keys: %v", violKeys(vs))
	}
}

// ── Arm 5: control — correctly registered ────────────────────────────────────

// TestGate_Arm5_Control asserts that a correctly registered Windows-conditional
// skip (== "windows" branch with t.Skip) with non-empty evidence produces
// zero violations.
// Reconciliation: this test asserts that the gate does not report false
// positives for correctly classified, fully-evidenced entries.
// Note: the previous Arm5 fixture used `!= "windows"` (skips on non-Windows),
// which encoded the inverted-polarity defect this ML corrects. The corrected
// gate would have reported ViolObsolete for that fixture. The fixture is now
// `== "windows"` to match a true Windows-skip pattern.
func TestGate_Arm5_Control(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm5_RegisteredSkip(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("arm5: chmod not effective on Windows")
	}
	// POSIX-only assertions here.
}
`
	root := fixtureModRoot(t, "mypkg", src)
	reg := []Entry{{
		Test:           "mypkg/fix_test.go:TestArm5_RegisteredSkip",
		Classification: "c",
		Reason:         "POSIX-only test: chmod 0000 is not effective on Windows",
		Evidence:       "runtime.GOOS == windows branch with t.Skip; os.Chmod does not block reads/traversal on NTFS without ACL manipulation",
	}}
	vs, err := Check(root, reg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Fatalf("expected 0 violations for correctly registered skip, got %d: %v", len(vs), vs)
	}
}

// ── Arm 6: != "windows" skip is NOT marked ───────────────────────────────────

// TestGate_Arm6_NonWindowsSkipNotMarked asserts that a test with
// `if runtime.GOOS != "windows" { t.Skip() }` is NOT marked by the gate.
// Such a test skips on non-Windows and RUNS on Windows — it is not a
// Windows-skip and must not appear in the registry.
// Reconciliation: this test asserts that the polarity fix correctly ignores
// !=-windows branches; the old gate (ML-3A) marked these as (a) violations.
func TestGate_Arm6_NonWindowsSkipNotMarked(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm6_NonWindowsSkip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("arm6: Windows-only integration test; skips on non-Windows")
	}
	// Windows-only assertions here — this test RUNS on Windows.
}
`
	root := fixtureModRoot(t, "mypkg", src)
	vs, err := Check(root, nil) // empty registry
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// No violation expected: the gate does not mark this test, so the empty
	// registry is consistent (no ViolMissing, no ViolObsolete).
	if hasKind(vs, ViolMissing) && hasKey(vs, "TestArm6_NonWindowsSkip") {
		t.Fatalf("gate incorrectly marked a != windows skip; expected not marked, got ViolMissing for TestArm6_NonWindowsSkip")
	}
}

// ── Arm 7: //go:build windows file is NOT marked ─────────────────────────────

// TestGate_Arm7_WindowsBuildTagNotMarked asserts that tests in a file with
// //go:build windows are NOT marked by the gate. Such tests run ONLY on
// Windows and are not Windows-skips.
// Reconciliation: this test asserts that winOnly files are excluded from
// marking; the old gate (ML-3A) marked all platform-specific files including
// winOnly, producing false (c) entries.
func TestGate_Arm7_WindowsBuildTagNotMarked(t *testing.T) {
	root := t.TempDir()
	modFile := "module example.com/fix\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(modFile), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	pkgDir := filepath.Join(root, "mypkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	// File with //go:build windows: only compiles on Windows.
	src := "//go:build windows\n\npackage mypkg\n\nimport \"testing\"\n\nfunc TestArm7_WindowsOnly(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "fix_windows_test.go"), []byte(src), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	vs, err := Check(root, nil) // empty registry
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// No violation expected: //go:build windows tests run on Windows and are
	// not Windows-skips; the gate must not mark them.
	if hasKind(vs, ViolMissing) && hasKey(vs, "TestArm7_WindowsOnly") {
		t.Fatalf("gate incorrectly marked a //go:build windows test; expected not marked, got ViolMissing for TestArm7_WindowsOnly")
	}
}

// ── Arm 8: _linux_test.go IS marked ──────────────────────────────────────────

// TestGate_Arm8_LinuxFileIsMarked asserts that tests in a file excluded from
// Windows (e.g. named _linux_test.go or carrying //go:build !windows) ARE
// marked by the gate. Such tests are skipped on Windows because the file does
// not compile on Windows.
// Note: _unix_test.go WITHOUT a build tag DOES compile on Windows (verified
// with GOOS=windows go list); it is not marked. Only files whose build
// constraints exclude Windows are marked.
// Reconciliation: this test asserts that nonWinOnly detection (the unchanged
// half of the polarity fix) still marks these files; the old gate already
// marked them correctly.
func TestGate_Arm8_LinuxFileIsMarked(t *testing.T) {
	root := t.TempDir()
	modFile := "module example.com/fix\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(modFile), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	pkgDir := filepath.Join(root, "mypkg")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	// _linux_test.go is excluded on Windows by filename convention.
	src := "package mypkg\n\nimport \"testing\"\n\nfunc TestArm8_LinuxOnly(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "fix_linux_test.go"), []byte(src), 0644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	vs, err := Check(root, nil) // empty registry
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// ViolMissing expected: _linux_test.go is excluded on Windows (nonWinOnly),
	// so the gate marks every Test* in it. The empty registry has no entry
	// for it, producing ViolMissing.
	if !hasKind(vs, ViolMissing) {
		t.Fatalf("expected ViolMissing for _linux_test.go test, got %v", violKinds(vs))
	}
	if !hasKey(vs, "TestArm8_LinuxOnly") {
		t.Fatalf("ViolMissing should name TestArm8_LinuxOnly; keys: %v", violKeys(vs))
	}
}

// ── Arm 9: tagless switch with Windows skip ───────────────────────────────────

// TestGate_Arm9_TaglessSwitchWindowsSkip asserts that a tagless switch
// (switch { case runtime.GOOS == "windows": t.Skip() }) is detected as a
// Windows-conditional skip and produces ViolMissing when unregistered.
// Reconciliation: this test asserts that achado 4-A1 from the red team
// (tagless switch invisible to the gate) is closed — the gate must detect
// the form `switch { case runtime.GOOS == "windows": t.Skip() }`.
func TestGate_Arm9_TaglessSwitchWindowsSkip(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm9_TaglessSwitch(t *testing.T) {
	switch {
	case runtime.GOOS == "windows":
		t.Skip("arm9: tagless switch Windows skip")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	vs, err := Check(root, nil) // empty registry
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasKind(vs, ViolMissing) {
		t.Fatalf("expected ViolMissing for tagless switch Windows skip, got %v", violKinds(vs))
	}
	if !hasKey(vs, "TestArm9_TaglessSwitch") {
		t.Fatalf("ViolMissing should name TestArm9_TaglessSwitch; keys: %v", violKeys(vs))
	}
}

// TestGate_Arm9_ContraArm_TaglessSwitchNonWindows asserts that a tagless switch
// whose case condition is definitely false on Windows (runtime.GOOS != "windows")
// does NOT produce a ViolMissing — such a test runs on Windows.
// Reconciliation: this test asserts that the fix for 4-A1 does not over-mark
// non-Windows tagless switch cases as Windows skips.
func TestGate_Arm9_ContraArm_TaglessSwitchNonWindows(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm9CA_TaglessSwitchNotWindows(t *testing.T) {
	switch {
	case runtime.GOOS != "windows":
		t.Skip("arm9 counter: skips on non-Windows, runs on Windows")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	vs, err := Check(root, nil) // empty registry
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// Not marked: the skip fires when GOOS != windows (i.e., on non-Windows),
	// so this test RUNS on Windows — it must not appear in the registry.
	if hasKind(vs, ViolMissing) && hasKey(vs, "TestArm9CA_TaglessSwitchNotWindows") {
		t.Fatalf("gate incorrectly marked a non-Windows tagless switch skip; keys: %v", violKeys(vs))
	}
}

// ── Arm 10: local variable assigned from runtime.GOOS ────────────────────────

// TestGate_Arm10_LocalVarGOOS asserts that `goos := runtime.GOOS; if goos == "windows" { t.Skip() }`
// is detected as a Windows-conditional skip and produces ViolMissing when unregistered.
// Reconciliation: this test asserts that achado 4-A2 from the red team
// (local-variable alias for runtime.GOOS invisible to the gate) is closed.
func TestGate_Arm10_LocalVarGOOS(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm10_LocalVarGOOS(t *testing.T) {
	goos := runtime.GOOS
	if goos == "windows" {
		t.Skip("arm10: local var alias for runtime.GOOS")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	vs, err := Check(root, nil) // empty registry
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasKind(vs, ViolMissing) {
		t.Fatalf("expected ViolMissing for local-var GOOS skip, got %v", violKinds(vs))
	}
	if !hasKey(vs, "TestArm10_LocalVarGOOS") {
		t.Fatalf("ViolMissing should name TestArm10_LocalVarGOOS; keys: %v", violKeys(vs))
	}
}

// TestGate_Arm10_ContraArm_LocalVarNonGOOS asserts that a local variable NOT
// assigned from runtime.GOOS does not cause a ViolMissing even when compared
// to "windows".
// Reconciliation: this test asserts that the alias detection is scoped to
// `runtime.GOOS` assignments only; os.Getenv("GOOS") is a dead signal (ML-3A2
// decision) and must not be recognised as a Windows-skip.
func TestGate_Arm10_ContraArm_LocalVarNonGOOS(t *testing.T) {
	src := `package mypkg

import (
	"os"
	"testing"
)

func TestArm10CA_LocalVarNotRuntime(t *testing.T) {
	goos := os.Getenv("GOOS")
	if goos == "windows" {
		t.Skip("arm10 counter: goos from os.Getenv, not runtime.GOOS")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	vs, err := Check(root, nil) // empty registry
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// Not marked: goos comes from os.Getenv("GOOS"), not runtime.GOOS.
	if hasKind(vs, ViolMissing) && hasKey(vs, "TestArm10CA_LocalVarNotRuntime") {
		t.Fatalf("gate incorrectly marked a local var from os.Getenv; keys: %v", violKeys(vs))
	}
}

// ── Arm 11: continue in Windows branch ───────────────────────────────────────

// TestGate_Arm11_ContinueInWindowsBranch asserts that `continue` inside a
// Windows branch of a loop is detected as a Windows-conditional skip and
// produces ViolMissing when unregistered.
// Reconciliation: this test asserts that achado 4-A3 from the red team
// (continue in a Windows branch invisible to the gate) is closed.
func TestGate_Arm11_ContinueInWindowsBranch(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm11_ContinueInWindowsBranch(t *testing.T) {
	for i := 0; i < 1; i++ {
		if runtime.GOOS == "windows" {
			continue
		}
		if i < 0 {
			t.Fatal("unreachable")
		}
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	vs, err := Check(root, nil) // empty registry
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasKind(vs, ViolMissing) {
		t.Fatalf("expected ViolMissing for continue-in-Windows-branch, got %v", violKinds(vs))
	}
	if !hasKey(vs, "TestArm11_ContinueInWindowsBranch") {
		t.Fatalf("ViolMissing should name TestArm11_ContinueInWindowsBranch; keys: %v", violKeys(vs))
	}
}

// TestGate_Arm11_ContraArm_ContinueInNonWindowsBranch asserts that `continue`
// inside a branch that is definitely false on Windows (runtime.GOOS != "windows")
// does NOT produce a ViolMissing — such a loop iteration runs on Windows.
// Reconciliation: this test asserts that the fix for 4-A3 uses polarity
// correctly: continue in a non-Windows branch does not mark the test.
func TestGate_Arm11_ContraArm_ContinueInNonWindowsBranch(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm11CA_ContinueInNonWindowsBranch(t *testing.T) {
	for i := 0; i < 1; i++ {
		if runtime.GOOS != "windows" {
			continue
		}
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	vs, err := Check(root, nil) // empty registry
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// Not marked: the continue fires on non-Windows — the loop body runs on Windows.
	if hasKind(vs, ViolMissing) && hasKey(vs, "TestArm11CA_ContinueInNonWindowsBranch") {
		t.Fatalf("gate incorrectly marked a non-Windows continue; keys: %v", violKeys(vs))
	}
}

// ── Arm 12: placeholder evidence ─────────────────────────────────────────────

// TestGate_Arm12_PendingEvidence asserts that an entry with evidence "  pending  "
// (whitespace and mixed-case) is rejected with ViolNoEvidence.
// Reconciliation: this test asserts that achado 4-A4 from the red team
// (placeholder evidence accepted by the gate) is closed — both trim and
// case-insensitive match must work.
func TestGate_Arm12_PendingEvidence(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm12_PendingEvidence(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("arm12: placeholder evidence")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	reg := []Entry{{
		Test:           "mypkg/fix_test.go:TestArm12_PendingEvidence",
		Classification: "c",
		Reason:         "some reason",
		Evidence:       "  pending  ", // whitespace + mixed-case — must be rejected
	}}
	vs, err := Check(root, reg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasKind(vs, ViolNoEvidence) {
		t.Fatalf("expected ViolNoEvidence for placeholder evidence, got %v", violKinds(vs))
	}
	if !hasKey(vs, "TestArm12_PendingEvidence") {
		t.Fatalf("ViolNoEvidence should name TestArm12_PendingEvidence; keys: %v", violKeys(vs))
	}
}

// TestGate_Arm12_ContraArm_PendingSubstring asserts that an evidence field that
// merely CONTAINS "PENDING" as a substring (not the whole field) is NOT rejected.
// Reconciliation: this test asserts that the placeholder check uses anchored
// matching (^...$), not substring matching — a real evidence string like
// "PENDING até job 134276469418; agora PASS" must be accepted.
func TestGate_Arm12_ContraArm_PendingSubstring(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm12CA_PendingSubstring(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("arm12 counter: evidence contains PENDING as substring")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	reg := []Entry{{
		Test:           "mypkg/fix_test.go:TestArm12CA_PendingSubstring",
		Classification: "c",
		Reason:         "some reason",
		Evidence:       "PENDING até job 134276469418; agora PASS",
	}}
	vs, err := Check(root, reg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Fatalf("expected 0 violations for evidence with PENDING as substring, got %d: %v", len(vs), vs)
	}
}

// ── Arm 13: empty classification ─────────────────────────────────────────────

// TestGate_Arm13_EmptyClassification asserts that an entry with an empty
// classification field produces ViolBadClassification.
// Reconciliation: this test asserts that achado 4-A5 from the red team
// (empty classification accepted by the gate) is closed — classification must
// be one of "a", "b", or "c".
func TestGate_Arm13_EmptyClassification(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm13_EmptyClassification(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("arm13: empty classification")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	reg := []Entry{{
		Test:           "mypkg/fix_test.go:TestArm13_EmptyClassification",
		Classification: "", // ← deliberately empty
		Reason:         "some reason",
		Evidence:       "real evidence here",
	}}
	vs, err := Check(root, reg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !hasKind(vs, ViolBadClassification) {
		t.Fatalf("expected ViolBadClassification for empty classification, got %v", violKinds(vs))
	}
	if !hasKey(vs, "TestArm13_EmptyClassification") {
		t.Fatalf("ViolBadClassification should name TestArm13_EmptyClassification; keys: %v", violKeys(vs))
	}
}

// TestGate_Arm13_ContraArm_ValidClassification asserts that an entry with
// classification "c" and real evidence produces zero violations.
// Reconciliation: this test asserts that the classification validator does not
// reject valid values — the fix for 4-A5 must not cause false positives.
func TestGate_Arm13_ContraArm_ValidClassification(t *testing.T) {
	src := `package mypkg

import (
	"runtime"
	"testing"
)

func TestArm13CA_ValidClassification(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("arm13 counter: valid classification")
	}
}
`
	root := fixtureModRoot(t, "mypkg", src)
	reg := []Entry{{
		Test:           "mypkg/fix_test.go:TestArm13CA_ValidClassification",
		Classification: "c",
		Reason:         "POSIX-only: chmod semantics differ on NTFS",
		Evidence:       "job 134276469418 PASS on windows-latest",
	}}
	vs, err := Check(root, reg)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(vs) != 0 {
		t.Fatalf("expected 0 violations for valid classification, got %d: %v", len(vs), vs)
	}
}

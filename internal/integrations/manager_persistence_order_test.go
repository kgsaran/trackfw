package integrations

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// This file proves ADR-2026-08-18 (manifest before bytes for install/update,
// unchanged bytes-before-manifest for uninstall) with executed evidence, not
// by reading the code:
//
//  1. TestInstallInterruptedAfterManifestPersistSelfHeals — genuinely
//     interrupts a subprocess at the ADR seam (afterManifestPersist) and
//     proves the resulting disk state is manifest-ahead-of-disk
//     (StateNotInstalled) and that a plain Install (no --force) repairs it.
//     This is also the P4 falsification scenario for ML-1A: baseline is the
//     interrupted/inconsistent state, detection is inspectResolved resolving
//     it to StateNotInstalled instead of unmanaged/StateModified.
//  2. TestUpdateMidWriteFailureRollsBackManifestAndBytes — proves the
//     rollback defer still restores both manifest and artifact bytes to
//     their pre-batch baseline when a *normal* error (not a crash) happens
//     in the write phase, which now runs after the manifest has already
//     been persisted.

// TRACKFW_MANAGER_CRASH_HELPER, when set to "1", turns this test binary into
// a throwaway subprocess that installs one artifact with afterManifestPersist
// wired to os.Exit — simulating SIGKILL/power-loss: the defer never runs.
const crashHelperEnv = "TRACKFW_MANAGER_CRASH_HELPER"
const crashProjectEnv = "TRACKFW_MANAGER_CRASH_PROJECT"

func TestInstallInterruptedAfterManifestPersistSelfHeals(t *testing.T) {
	if os.Getenv(crashHelperEnv) == "1" {
		runInterruptedInstallHelper()
		return
	}

	project := t.TempDir()
	home := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestInstallInterruptedAfterManifestPersistSelfHeals$")
	cmd.Env = append(os.Environ(),
		crashHelperEnv+"=1",
		crashProjectEnv+"="+project,
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected the helper subprocess to be interrupted (os.Exit) before completing Install; it returned normally. output=%s", output)
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 7 {
		t.Fatalf("helper subprocess exited unexpectedly: err=%v output=%s", err, output)
	}

	// Baseline captured, mid-batch, executing: the manifest was persisted
	// (declares the artifact) but the artifact's bytes were never written —
	// the ADR-2026-08-18 window, reproduced for real.
	destination := filepath.Join(project, "agents", "interrupted.md")
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("artifact bytes should be absent after the interrupted write, statErr=%v", statErr)
	}
	manifest, loadErr := loadManifest(manifestPath(project))
	if loadErr != nil {
		t.Fatalf("load manifest: %v", loadErr)
	}
	entry, declared := manifest.Artifacts[destination]
	if !declared {
		t.Fatalf("manifest should already declare %q after the interrupted install (manifest-first ordering)", destination)
	}
	if entry.Hash == "" {
		t.Fatalf("manifest entry for %q has no Hash even though it should describe the target content", destination)
	}

	manager := Manager{ProjectRoot: project, HomeDir: home}
	plan := crashHelperPlan()

	// Detection: inspectResolved must resolve the window to StateNotInstalled
	// (self-repairable), never StateModified/unmanaged (which would require
	// manual `install --force`).
	assertState(t, manager, plan, StateNotInstalled)

	// Self-repair: a later Install, with NO --force, must succeed and reach
	// StateCurrent — this is the concrete win ADR-2026-08-18 claims.
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatalf("self-repair Install() failed: %v", err)
	}
	assertState(t, manager, plan, StateCurrent)
}

func crashHelperPlan() PlannedArtifact {
	return testPlan("project", "agents/interrupted.md", "v1", "interrupted-content")
}

// runInterruptedInstallHelper runs inside the subprocess spawned by
// TestInstallInterruptedAfterManifestPersistSelfHeals. It wires
// afterManifestPersist to os.Exit(7) — bypassing every deferred rollback,
// exactly like SIGKILL or power loss would — after the manifest has been
// persisted but before the artifact's bytes are written.
func runInterruptedInstallHelper() {
	project := os.Getenv(crashProjectEnv)
	afterManifestPersist = func() {
		os.Exit(7)
	}
	manager := Manager{ProjectRoot: project, HomeDir: project}
	_ = manager.Install([]PlannedArtifact{crashHelperPlan()}, false)
	// Unreachable: afterManifestPersist always fires for a non-empty active
	// batch before Install can return. If we get here, the ordering broke.
	os.Exit(0)
}

// TestUpdateMidWriteFailureRollsBackManifestAndBytes proves risk 2 from the
// handoff: with manifest persisted before artifact bytes, a *normal* error
// (not a crash — the defer runs) during the now-later write phase must still
// restore both the manifest and the file bytes to their pre-batch baseline.
// TestManagerPreflightRollsBackBatch only exercises a preflight failure,
// which aborts before manifests are ever persisted; this test exercises the
// failure the inversion newly makes possible: an error *after* the manifest
// write has already happened.
func TestUpdateMidWriteFailureRollsBackManifestAndBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits behave differently on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission enforcement is not applicable")
	}

	manager, project, _ := testManager(t)
	plan := testPlan("project", "agents/backend.md", "v1", "v1content")
	if err := manager.Install([]PlannedArtifact{plan}, false); err != nil {
		t.Fatal(err)
	}

	filename := filepath.Join(project, plan.Destination)
	baselineBytes := readFile(t, filename)
	baselineManifest, err := loadManifest(manifestPath(project))
	if err != nil {
		t.Fatal(err)
	}
	baselineHash := baselineManifest.Artifacts[filename].Hash
	baselineCatalogVersion := baselineManifest.Artifacts[filename].CatalogVersion

	updated := plan
	updated.Content = []byte("v2content")
	updated.CatalogVersion = "v2"

	// Force the write phase (which now runs after the manifest has already
	// been persisted) to fail for this destination: make its directory
	// read-only so atomicWrite's os.CreateTemp cannot create the temp file.
	// MkdirAll on an already-existing directory does not change its mode, so
	// this reliably forces only the byte write to fail — preflight, planning
	// and the manifest persist all still succeed.
	directory := filepath.Dir(filename)
	if err := os.Chmod(directory, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })

	err = manager.Update([]PlannedArtifact{updated}, false)
	if err == nil {
		t.Fatal("expected the write-phase failure to propagate as an error")
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}

	gotBytes := readFile(t, filename)
	if gotBytes != baselineBytes {
		t.Fatalf("rollback did not restore artifact bytes: got %q, want %q", gotBytes, baselineBytes)
	}
	manifest, err := loadManifest(manifestPath(project))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := manifest.Artifacts[filename]
	if !ok {
		t.Fatalf("rollback removed the manifest entry for %q entirely", filename)
	}
	if entry.Hash != baselineHash {
		t.Fatalf("rollback did not restore manifest Hash: got %q, want %q", entry.Hash, baselineHash)
	}
	if entry.CatalogVersion != baselineCatalogVersion {
		t.Fatalf("rollback did not restore manifest CatalogVersion: got %q, want %q", entry.CatalogVersion, baselineCatalogVersion)
	}

	// Non-regression: the artifact is still reported as current v1, exactly
	// as it was before the failed Update was attempted.
	assertState(t, manager, plan, StateCurrent)
}

// TestUpdateBatchRollbackRestoresAlreadyWrittenArtifactBytes closes the gap
// left by TestUpdateMidWriteFailureRollsBackManifestAndBytes: there, the
// artifact write never lands in the first place (permission denied before
// any byte moves), so "bytes equal baseline" holds trivially — nothing was
// ever overwritten. This test uses two artifacts in a batch: the first
// artifact's write succeeds (its bytes really do change to v2 before the
// batch fails), the second artifact's write fails. The defer/rollback must
// then genuinely revert the first artifact's bytes back to v1 — proving the
// restore actually restores, not just that a write never happened.
func TestUpdateBatchRollbackRestoresAlreadyWrittenArtifactBytes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits behave differently on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: permission enforcement is not applicable")
	}

	manager, project, _ := testManager(t)
	planA := testPlan("project", ".claude/agents/a.md", "v1", "a-v1content")
	planB := testPlan("project", ".gemini/agents/b.md", "v1", "b-v1content")
	if err := manager.Install([]PlannedArtifact{planA, planB}, false); err != nil {
		t.Fatal(err)
	}

	fileA := filepath.Join(project, planA.Destination)
	fileB := filepath.Join(project, planB.Destination)
	baselineA := readFile(t, fileA)
	baselineB := readFile(t, fileB)
	baselineManifest, err := loadManifest(manifestPath(project))
	if err != nil {
		t.Fatal(err)
	}

	updatedA := planA
	updatedA.Content = []byte("a-v2content")
	updatedA.CatalogVersion = "v2"
	updatedB := planB
	updatedB.Content = []byte("b-v2content")
	updatedB.CatalogVersion = "v2"

	// Only b's directory is made read-only: a's write (first in resolved
	// order) succeeds and genuinely overwrites disk with v2 content; b's
	// write then fails, forcing rollback of the whole batch.
	directoryB := filepath.Dir(fileB)
	if err := os.Chmod(directoryB, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(directoryB, 0o700) })

	err = manager.Update([]PlannedArtifact{updatedA, updatedB}, false)
	if err == nil {
		t.Fatal("expected the second artifact's write-phase failure to propagate as an error")
	}
	if err := os.Chmod(directoryB, 0o700); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, fileA); got != baselineA {
		t.Fatalf("rollback did not restore already-written artifact A bytes: got %q, want %q", got, baselineA)
	}
	if got := readFile(t, fileB); got != baselineB {
		t.Fatalf("artifact B bytes changed even though its write failed: got %q, want %q", got, baselineB)
	}
	manifest, err := loadManifest(manifestPath(project))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Artifacts[fileA].Hash != baselineManifest.Artifacts[fileA].Hash {
		t.Fatalf("rollback did not restore manifest entry for A: got %q, want %q", manifest.Artifacts[fileA].Hash, baselineManifest.Artifacts[fileA].Hash)
	}
	if manifest.Artifacts[fileB].Hash != baselineManifest.Artifacts[fileB].Hash {
		t.Fatalf("rollback did not restore manifest entry for B: got %q, want %q", manifest.Artifacts[fileB].Hash, baselineManifest.Artifacts[fileB].Hash)
	}
	assertState(t, manager, planA, StateCurrent)
	assertState(t, manager, planB, StateCurrent)
}

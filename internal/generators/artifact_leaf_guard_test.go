package generators

// artifact_leaf_guard_test.go — ML-4B: proves that artifact generators and
// syncREQReferences guard the LEAF FILE and not only the parent directory.
//
// Defects closed:
//
//	B (gap de folha): NewNote, NewREQ, NewRoadmapFromContent, NewADR, NewADRDraft
//	  called RejectSymlinks on the DIRECTORY, not the file to be written. A file
//	  that is itself a symlink inside a clean directory was not caught.
//	D (false marker): syncREQReferences had a write-containment-allowed marker
//	  with no corresponding pathguard.RejectSymlinks call. A REQ file replaced
//	  by a symlink pointing outside root would have been overwritten.
//
// Tests:
//   - TestNewNote_SymlinkLeafRefused — arm (a): note file is a symlink →
//     write refused, victim unchanged.
//     Affirms: the new leaf guard in note.go fires when the note FILE (not the
//     directory) is a symlink pointing outside root.
//   - TestSyncREQReferences_SymlinkLeafRefused — arm (a): REQ file is a symlink
//     → syncREQReferences refuses the write, victim unchanged.
//     Affirms: the new per-file guard in syncREQReferences fires when the REQ
//     file is a symlink pointing outside root, so roadmap move cannot clobber
//     an attacker-controlled victim via a paired REQ symlink.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestNewNote_SymlinkLeafRefused asserts that the ML-4B leaf-level guard in
// note.go rejects NewNote when the target note FILE (inside a clean vault/notes/
// directory) is itself a symlink pointing outside the project root.
//
// Affirms: the new pathguard.RejectSymlinks(noteRoot, absNotePath) guard fires
// when the note file leaf is a symlink, and the victim file outside root remains
// byte-identical after the refused call.
func TestNewNote_SymlinkLeafRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()
	chdirNote(t, dir)

	// victimPath is NOT created before the call — os.Stat(notePath) follows the
	// symlink and gets ENOENT, so the idempotency check passes and the code
	// reaches the WriteFile site. Without the ML-4B guard the write creates
	// victimPath (containment violated); with the guard it is refused.
	victimPath := filepath.Join(outside, "victim.txt")

	// Create vault/notes/ as a real directory (clean, NOT a symlink).
	if err := os.MkdirAll(filepath.Join(dir, "vault", "notes"), 0755); err != nil {
		t.Fatalf("creating vault/notes: %v", err)
	}

	// Place the leaf file — the exact filename NewNote("leaf guard test") would
	// generate — as a symlink pointing to the victim. NewNote produces
	// slug-date.md where slug = toSlug(title).
	date := time.Now().Format("2006-01-02")
	noteFilename := "leaf-guard-test-" + date + ".md"
	leafPath := filepath.Join(dir, "vault", "notes", noteFilename)
	// check-symlink-privilege-guard: symlinkOrSkip is on the line immediately above the t.Fatalf below
	symlinkOrSkip(t, victimPath, leafPath)

	err := NewNote("leaf guard test")
	if err == nil {
		t.Fatal("NewNote() should refuse when the note file leaf is a symlink, got nil")
	}

	// Victim must NOT have been created — containment guard must have fired
	// before the WriteFile call.
	if _, statErr := os.Stat(victimPath); statErr == nil {
		t.Error("containment violated: victim file was created at outside path through symlink leaf")
	}
}

// TestSyncREQReferences_SymlinkLeafRefused asserts that the ML-4B guard in
// syncREQReferences rejects writing to a REQ file that is itself a symlink
// pointing outside the project root.
//
// Setup: a real roadmap in wip/ with a REQ whose file is a symlink to victim.txt
// outside root. The REQ frontmatter's roadmap: field matches the moved roadmap,
// so syncREQReferences would try to rewrite it.
//
// Affirms: the new pathguard.RejectSymlinks(syncRoot, absReqPath) guard fires
// when the REQ file is a symlink, and the victim file outside root remains
// byte-identical after the refused write.
func TestSyncREQReferences_SymlinkLeafRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()
	chdirRoadmap(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	const roadmapFilename = "ROADMAP-2026-09-15-ml4b-sync-test.md"
	const roadmapContent = "---\nstatus: wip\nreq: \"\"\nsquad: \"\"\ndate: 2026-09-15\n---\n\n# Roadmap: ml4b sync test\n\n"

	// Set up the roadmap in docs/roadmaps/wip/.
	if err := os.MkdirAll(filepath.Join(dir, "docs", "roadmaps", "wip"), 0755); err != nil {
		t.Fatalf("mkdir roadmaps/wip: %v", err)
	}
	rmPath := filepath.Join(dir, "docs", "roadmaps", "wip", roadmapFilename)
	if err := os.WriteFile(rmPath, []byte(roadmapContent), 0644); err != nil {
		t.Fatalf("writing roadmap: %v", err)
	}

	// Create docs/req/ as a real directory (clean, NOT a symlink).
	if err := os.MkdirAll(filepath.Join(dir, "docs", "req"), 0755); err != nil {
		t.Fatalf("mkdir docs/req: %v", err)
	}

	// Victim file: valid REQ content with frontmatter roadmap: field referencing
	// the roadmap above. syncREQReferences reads this via the symlink and would
	// write updated content back through the symlink without the ML-4B guard.
	victimContent := []byte("---\nstatus: Open\ndate: 2026-09-15\nauthor: \"\"\nadr: \"\"\nroadmap: \"docs/roadmaps/wip/ROADMAP-2026-09-15-ml4b-sync-test.md\"\n---\n\n# REQ: ml4b sync test\n\n> Date: 2026-09-15 | Status: Open\n")
	victimPath := filepath.Join(outside, "victim.txt")
	if err := os.WriteFile(victimPath, victimContent, 0644); err != nil {
		t.Fatalf("creating victim: %v", err)
	}

	// Place the REQ file as a symlink pointing to the victim.
	const reqFilename = "REQ-2026-09-15-ml4b-sync-test.md"
	leafPath := filepath.Join(dir, "docs", "req", reqFilename)
	// check-symlink-privilege-guard: symlinkOrSkip is on the line immediately above the t.Fatalf below
	symlinkOrSkip(t, victimPath, leafPath)

	// MoveRoadmap triggers syncREQReferences, which must refuse the symlink REQ.
	// Use "blocked" to avoid the Wave 0 gate that only fires for "done".
	err := MoveRoadmap("ROADMAP-2026-09-15-ml4b-sync-test", "blocked")
	if err == nil {
		t.Fatal("MoveRoadmap() should return error when syncREQReferences encounters a symlink REQ, got nil")
	}

	// Victim must remain byte-identical — write must not have gone through the symlink.
	got, readErr := os.ReadFile(victimPath)
	if readErr != nil {
		t.Fatalf("reading victim after refused call: %v", readErr)
	}
	if string(got) != string(victimContent) {
		t.Errorf("containment violated: victim was overwritten\ngot:  %q\nwant: %q", got, victimContent)
	}
}

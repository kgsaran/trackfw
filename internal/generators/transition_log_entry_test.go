package generators

// transition_log_entry_test.go — ML-1A acceptance tests.
//
// What each test affirms (Regra Dura de Reconciliação, one sentence each):
//
//   - TestTransitionLog_ParallelBranchesMergeWithoutConflict affirms the ML-1A
//     conclusion that two branches each writing to .trackfw-log-entries/ produce no merge
//     conflict, because the per-entry filenames are distinct (AC7).
//
//   - TestTransitionLog_LegacyAppendConflicts affirms the contra-arm (ADR D3): the
//     old append-to-single-file strategy produces a merge conflict, proving the
//     per-entry approach solves a real problem rather than an imaginary one.
//
//   - TestTransitionLog_EntriesInDotD_VisibleViaReadAll affirms that entries written
//     by MoveRoadmap (which calls appendTransitionLogEntry → translog.WriteEntry) are
//     returned by translog.ReadAll on the same directory — the only way callers read
//     the log (AC6).

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/translog"
)

// AFIRMA: dois branches que chamam MoveRoadmap sobre roadmaps diferentes gravam em
// arquivos distintos em .trackfw-log-entries/ — `git merge` sai 0, sem marcadores de conflito
// (AC7, D1 do ADR).
func TestTransitionLog_ParallelBranchesMergeWithoutConflict(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repoDir := t.TempDir()

	// Helper: run a git command in repoDir.
	git := func(args ...string) *exec.Cmd {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_CONFIG_COUNT=4",
			"GIT_CONFIG_KEY_0=core.autocrlf",
			"GIT_CONFIG_VALUE_0=false",
			"GIT_CONFIG_KEY_1=commit.gpgsign",
			"GIT_CONFIG_VALUE_1=false",
			"GIT_CONFIG_KEY_2=core.attributesFile",
			"GIT_CONFIG_VALUE_2=/dev/null",
			"GIT_CONFIG_KEY_3=user.email",
			"GIT_CONFIG_VALUE_3=test@example.com",
		)
		return cmd
	}

	mustGit := func(t *testing.T, args ...string) {
		t.Helper()
		out, err := git(args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Init repo and configure user.
	mustGit(t, "init", "-b", "main")
	mustGit(t, "config", "user.name", "Test")
	mustGit(t, "config", "user.email", "test@example.com")

	// Prepare fixture: trackfw.yaml and two roadmaps in wip/.
	writeFixture := func(path, content string) {
		t.Helper()
		full := filepath.Join(repoDir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	writeFixture("trackfw.yaml", "roadmap_namespacing: flat\n")
	roadmapBody := "---\nstatus: wip\ndate: 2026-10-22\n---\n\n# Roadmap: Test\n\n> Created: 2026-10-22 | Status: wip\n\n## Wave 0 — Threat Model\n\n```bash\nexit 0\n```\n"
	writeFixture("docs/roadmaps/wip/ROADMAP-alpha.md", roadmapBody)
	writeFixture("docs/roadmaps/wip/ROADMAP-beta.md", roadmapBody)
	for _, d := range []string{"docs/roadmaps/backlog", "docs/roadmaps/analyzing",
		"docs/roadmaps/blocked", "docs/roadmaps/done", "docs/roadmaps/abandoned"} {
		if err := os.MkdirAll(filepath.Join(repoDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Initial commit on main.
	mustGit(t, "add", "-A")
	mustGit(t, "commit", "-m", "initial")

	// Branch A: move ROADMAP-alpha wip → done.
	mustGit(t, "checkout", "-b", "branch-a")
	t.Chdir(repoDir)
	config.Reset()
	t.Cleanup(config.Reset)
	if err := MoveRoadmap("ROADMAP-alpha", "done"); err != nil {
		t.Fatalf("branch-a MoveRoadmap: %v", err)
	}
	mustGit(t, "add", "-A")
	mustGit(t, "commit", "-m", "branch-a: alpha done")

	// Branch B: move ROADMAP-beta wip → done.
	mustGit(t, "checkout", "main")
	mustGit(t, "checkout", "-b", "branch-b")
	config.Reset()
	if err := MoveRoadmap("ROADMAP-beta", "done"); err != nil {
		t.Fatalf("branch-b MoveRoadmap: %v", err)
	}
	mustGit(t, "add", "-A")
	mustGit(t, "commit", "-m", "branch-b: beta done")

	// Merge branch-a into branch-b — must exit 0 with no conflict markers.
	mustGit(t, "merge", "branch-a", "--no-edit")

	// Verify no conflict markers anywhere.
	entriesDir := filepath.Join(repoDir, "docs/roadmaps", translog.EntriesDirSuffix[1:])
	entries, _ := os.ReadDir(entriesDir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".entry") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(entriesDir, e.Name()))
		if strings.Contains(string(data), "<<<<<<<") || strings.Contains(string(data), ">>>>>>>") {
			t.Errorf(".d entry %s contains conflict markers", e.Name())
		}
	}

	// Both transitions must be readable via translog.ReadAll (AC6).
	config.Reset()
	all, err := translog.ReadAll(filepath.Join(repoDir, "docs/roadmaps"))
	if err != nil {
		t.Fatalf("ReadAll after merge: %v", err)
	}
	alphaFound, betaFound := false, false
	for _, tr := range all {
		if strings.Contains(tr.Basename, "ROADMAP-alpha.md") && tr.To == "done" {
			alphaFound = true
		}
		if strings.Contains(tr.Basename, "ROADMAP-beta.md") && tr.To == "done" {
			betaFound = true
		}
	}
	if !alphaFound || !betaFound {
		t.Errorf("after merge: alphaFound=%v betaFound=%v (got %d entries)", alphaFound, betaFound, len(all))
	}
}

// AFIRMA: o contra-braço (ADR D3) — dois branches que APPENDAM ao mesmo arquivo
// .trackfw-log legado produzem um conflito de merge — prova que o problema que ML-1A
// resolve é real.
func TestTransitionLog_LegacyAppendConflicts(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	repoDir := t.TempDir()

	git := func(args ...string) *exec.Cmd {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL=/dev/null",
			"GIT_CONFIG_NOSYSTEM=1",
			"GIT_CONFIG_COUNT=3",
			"GIT_CONFIG_KEY_0=commit.gpgsign",
			"GIT_CONFIG_VALUE_0=false",
			"GIT_CONFIG_KEY_1=core.attributesFile",
			"GIT_CONFIG_VALUE_1=/dev/null",
			"GIT_CONFIG_KEY_2=user.email",
			"GIT_CONFIG_VALUE_2=test@example.com",
		)
		return cmd
	}
	mustGit := func(t *testing.T, args ...string) {
		t.Helper()
		out, err := git(args...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	mustGit(t, "init", "-b", "main")
	mustGit(t, "config", "user.name", "Test")
	mustGit(t, "config", "user.email", "test@example.com")

	logDir := filepath.Join(repoDir, "docs", "roadmaps")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatal(err)
	}
	legacyLog := filepath.Join(logDir, ".trackfw-log")
	if err := os.WriteFile(legacyLog, []byte("# initial\n"), 0644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, "add", "-A")
	mustGit(t, "commit", "-m", "initial")

	// Branch A appends one line.
	mustGit(t, "checkout", "-b", "legacy-a")
	appendLegacy := func(t *testing.T, line string) {
		t.Helper()
		f, err := os.OpenFile(legacyLog, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(line + "\n"); err != nil {
			t.Fatal(err)
		}
	}
	appendLegacy(t, "2026-10-22 10:00  ROADMAP-alpha.md  wip → done")
	mustGit(t, "add", "-A")
	mustGit(t, "commit", "-m", "legacy-a appends")

	// Branch B appends a DIFFERENT line.
	mustGit(t, "checkout", "main")
	mustGit(t, "checkout", "-b", "legacy-b")
	appendLegacy(t, "2026-10-22 10:01  ROADMAP-beta.md   wip → done")
	mustGit(t, "add", "-A")
	mustGit(t, "commit", "-m", "legacy-b appends")

	// Merge legacy-a into legacy-b WITHOUT merge=union — this MUST conflict.
	// Both branches appended a distinct line at the same position (after the base
	// "# initial\n"), so this is a deterministic 3-way conflict every time.
	out, mergeErr := git("merge", "legacy-a", "--no-edit").CombinedOutput()
	if mergeErr == nil {
		t.Fatalf("expected merge to conflict but it succeeded cleanly — the D3 contra-arm is no longer valid\nmerge output:\n%s", out)
	}
	if !strings.Contains(string(out), "CONFLICT") {
		t.Errorf("merge failed but output does not contain CONFLICT — unexpected failure mode\noutput:\n%s", out)
	}
	data, err := os.ReadFile(legacyLog)
	if err != nil {
		t.Fatalf("read legacy log after failed merge: %v", err)
	}
	if !strings.Contains(string(data), "<<<<<<<") {
		t.Errorf("merge failed but legacy log has no conflict markers (<<<<<<< absent) — unexpected failure mode\nfile contents:\n%s", data)
	}
}

// AFIRMA: entradas gravadas por appendTransitionLogEntry são visíveis via
// translog.ReadAll no mesmo diretório (AC6).
func TestTransitionLog_EntriesInDotD_VisibleViaReadAll(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	config.Reset()
	t.Cleanup(config.Reset)

	for _, d := range []string{
		"docs/roadmaps/backlog", "docs/roadmaps/wip", "docs/roadmaps/done",
		"docs/roadmaps/analyzing", "docs/roadmaps/blocked", "docs/roadmaps/abandoned",
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a roadmap in wip/ and move it to done/.
	roadmapBody := "---\nstatus: wip\ndate: 2026-10-22\n---\n\n# Roadmap: Visibility Test\n\n> Created: 2026-10-22 | Status: wip\n\n## Wave 0 — Threat Model\n\n```bash\nexit 0\n```\n"
	if err := os.WriteFile("docs/roadmaps/wip/ROADMAP-visibility.md", []byte(roadmapBody), 0644); err != nil {
		t.Fatal(err)
	}
	if err := MoveRoadmap("ROADMAP-visibility", "done"); err != nil {
		t.Fatalf("MoveRoadmap: %v", err)
	}

	transitions, err := translog.ReadAll("docs/roadmaps")
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	found := false
	for _, tr := range transitions {
		if strings.Contains(tr.Basename, "ROADMAP-visibility.md") && tr.To == "done" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("translog.ReadAll did not return the entry written by MoveRoadmap; got %d transitions", len(transitions))
	}

	// Legacy .trackfw-log must NOT have been created (ML-1A: new writes go to .d/).
	if _, err := os.Stat("docs/roadmaps/.trackfw-log"); err == nil {
		t.Errorf("legacy .trackfw-log was created — ML-1A requires writes go to .trackfw-log-entries/")
	}
}

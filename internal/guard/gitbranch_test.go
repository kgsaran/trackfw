package guard

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testIdleTimeout = 50 * time.Millisecond

// makeProjectDir creates a temp directory with a trackfw.yaml file.
func makeProjectDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "trackfw.yaml"))
	if err != nil {
		t.Fatalf("create trackfw.yaml: %v", err)
	}
	f.Close()
	return dir
}

// runWith invokes RunGitBranch with a string payload from stdin.
func runWith(t *testing.T, dir, payload, command string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	var stdin io.Reader
	if payload != "" {
		stdin = bytes.NewReader([]byte(payload))
	} else {
		stdin = bytes.NewReader(nil)
	}
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %q: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	code := RunGitBranch(stdin, testIdleTimeout, command, os.Getenv, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// --- MatchSubcommand tests ---
// Each test asserts one conclusion from this ML: the corresponding blocking
// rule from scripts/trackfw-git-branch-guard.sh is faithfully ported to Go.

// TestMatchSubcommand_Commit asserts that "git commit" is blocked.
func TestMatchSubcommand_Commit(t *testing.T) {
	if got := MatchSubcommand("git commit -m 'msg'"); got != "commit" {
		t.Fatalf("got %q want %q", got, "commit")
	}
}

// TestMatchSubcommand_Push asserts that "git push" is blocked.
func TestMatchSubcommand_Push(t *testing.T) {
	if got := MatchSubcommand("git push origin main"); got != "push" {
		t.Fatalf("got %q want %q", got, "push")
	}
}

// TestMatchSubcommand_CheckoutB asserts that "git checkout -b" is blocked.
func TestMatchSubcommand_CheckoutB(t *testing.T) {
	if got := MatchSubcommand("git checkout -b feat/x"); got != "checkout-b" {
		t.Fatalf("got %q want %q", got, "checkout-b")
	}
}

// TestMatchSubcommand_CheckoutBigB asserts that "git checkout -B" is blocked.
func TestMatchSubcommand_CheckoutBigB(t *testing.T) {
	if got := MatchSubcommand("git checkout -B feat/x"); got != "checkout-b" {
		t.Fatalf("got %q want %q", got, "checkout-b")
	}
}

// TestMatchSubcommand_CheckoutOrphan asserts that "git checkout --orphan" is blocked.
func TestMatchSubcommand_CheckoutOrphan(t *testing.T) {
	if got := MatchSubcommand("git checkout --orphan gh-pages"); got != "checkout-b" {
		t.Fatalf("got %q want %q", got, "checkout-b")
	}
}

// TestMatchSubcommand_CheckoutBranchAllowed asserts "git checkout <branch>" is allowed.
func TestMatchSubcommand_CheckoutBranchAllowed(t *testing.T) {
	if got := MatchSubcommand("git checkout main"); got != "" {
		t.Fatalf("git checkout <branch> should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_CheckoutPathDoubleDash asserts "git checkout -- ." is blocked.
func TestMatchSubcommand_CheckoutPathDoubleDash(t *testing.T) {
	if got := MatchSubcommand("git checkout -- ."); got != "checkout-path" {
		t.Fatalf("got %q want %q", got, "checkout-path")
	}
}

// TestMatchSubcommand_CheckoutDot asserts "git checkout ." is blocked.
func TestMatchSubcommand_CheckoutDot(t *testing.T) {
	if got := MatchSubcommand("git checkout ."); got != "checkout-path" {
		t.Fatalf("got %q want %q", got, "checkout-path")
	}
}

// TestMatchSubcommand_SwitchC asserts "git switch -c" is blocked.
func TestMatchSubcommand_SwitchC(t *testing.T) {
	if got := MatchSubcommand("git switch -c feat/x"); got != "switch-c" {
		t.Fatalf("got %q want %q", got, "switch-c")
	}
}

// TestMatchSubcommand_SwitchCreate asserts "git switch --create" is blocked.
func TestMatchSubcommand_SwitchCreate(t *testing.T) {
	if got := MatchSubcommand("git switch --create feat/x"); got != "switch-c" {
		t.Fatalf("got %q want %q", got, "switch-c")
	}
}

// TestMatchSubcommand_SwitchAllowed asserts "git switch <branch>" is allowed.
func TestMatchSubcommand_SwitchAllowed(t *testing.T) {
	if got := MatchSubcommand("git switch main"); got != "" {
		t.Fatalf("git switch <branch> should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_BranchCreate asserts "git branch <name>" (positional) is blocked.
func TestMatchSubcommand_BranchCreate(t *testing.T) {
	if got := MatchSubcommand("git branch feat/x"); got != "branch-create" {
		t.Fatalf("got %q want %q", got, "branch-create")
	}
}

// TestMatchSubcommand_BranchDelete asserts "git branch -d" is allowed.
func TestMatchSubcommand_BranchDelete(t *testing.T) {
	if got := MatchSubcommand("git branch -d feat/x"); got != "" {
		t.Fatalf("git branch -d should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_BranchList asserts "git branch -a" is allowed.
func TestMatchSubcommand_BranchList(t *testing.T) {
	if got := MatchSubcommand("git branch -a"); got != "" {
		t.Fatalf("git branch -a should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_BranchMove asserts "git branch -m old new" is blocked.
func TestMatchSubcommand_BranchMove(t *testing.T) {
	if got := MatchSubcommand("git branch -m old new"); got != "branch-create" {
		t.Fatalf("got %q want %q", got, "branch-create")
	}
}

// TestMatchSubcommand_WorktreeAddB asserts "git worktree add -b" is blocked.
func TestMatchSubcommand_WorktreeAddB(t *testing.T) {
	if got := MatchSubcommand("git worktree add -b feat/x ../x"); got != "worktree-add-b" {
		t.Fatalf("got %q want %q", got, "worktree-add-b")
	}
}

// TestMatchSubcommand_WorktreeAddNoBranch asserts "git worktree add" without -b is allowed.
func TestMatchSubcommand_WorktreeAddNoBranch(t *testing.T) {
	if got := MatchSubcommand("git worktree add ../x main"); got != "" {
		t.Fatalf("git worktree add without -b should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_WorktreeRemoveForce asserts "git worktree remove -f" is blocked.
func TestMatchSubcommand_WorktreeRemoveForce(t *testing.T) {
	if got := MatchSubcommand("git worktree remove -f ../x"); got != "worktree-remove-force" {
		t.Fatalf("got %q want %q", got, "worktree-remove-force")
	}
}

// TestMatchSubcommand_Stash asserts bare "git stash" is blocked.
func TestMatchSubcommand_Stash(t *testing.T) {
	if got := MatchSubcommand("git stash"); got != "stash" {
		t.Fatalf("got %q want %q", got, "stash")
	}
}

// TestMatchSubcommand_StashPush asserts "git stash push" is blocked.
func TestMatchSubcommand_StashPush(t *testing.T) {
	if got := MatchSubcommand("git stash push"); got != "stash" {
		t.Fatalf("got %q want %q", got, "stash")
	}
}

// TestMatchSubcommand_StashList asserts "git stash list" is allowed.
func TestMatchSubcommand_StashList(t *testing.T) {
	if got := MatchSubcommand("git stash list"); got != "" {
		t.Fatalf("git stash list should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_StashShow asserts "git stash show" is allowed.
func TestMatchSubcommand_StashShow(t *testing.T) {
	if got := MatchSubcommand("git stash show"); got != "" {
		t.Fatalf("git stash show should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_ResetHard asserts "git reset --hard" is blocked.
func TestMatchSubcommand_ResetHard(t *testing.T) {
	if got := MatchSubcommand("git reset --hard HEAD~1"); got != "reset-hard" {
		t.Fatalf("got %q want %q", got, "reset-hard")
	}
}

// TestMatchSubcommand_ResetSoft asserts "git reset --soft" is allowed.
func TestMatchSubcommand_ResetSoft(t *testing.T) {
	if got := MatchSubcommand("git reset --soft HEAD~1"); got != "" {
		t.Fatalf("git reset --soft should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_ResetMixed asserts "git reset" (implicit --mixed) is allowed.
func TestMatchSubcommand_ResetMixed(t *testing.T) {
	if got := MatchSubcommand("git reset HEAD~1"); got != "" {
		t.Fatalf("git reset (--mixed) should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_CleanForce asserts "git clean -f" is blocked.
func TestMatchSubcommand_CleanForce(t *testing.T) {
	if got := MatchSubcommand("git clean -f"); got != "clean-force" {
		t.Fatalf("got %q want %q", got, "clean-force")
	}
}

// TestMatchSubcommand_CleanDryRun asserts "git clean -n" is allowed.
func TestMatchSubcommand_CleanDryRun(t *testing.T) {
	if got := MatchSubcommand("git clean -n"); got != "" {
		t.Fatalf("git clean -n should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_CleanForceDryRun asserts "git clean -f -n" is allowed (dry-run wins).
func TestMatchSubcommand_CleanForceDryRun(t *testing.T) {
	if got := MatchSubcommand("git clean -f -n"); got != "" {
		t.Fatalf("git clean -f -n should be allowed (dry-run present), got %q", got)
	}
}

// TestMatchSubcommand_RestorePath asserts "git restore <file>" is blocked.
func TestMatchSubcommand_RestorePath(t *testing.T) {
	if got := MatchSubcommand("git restore src/main.go"); got != "restore-path" {
		t.Fatalf("got %q want %q", got, "restore-path")
	}
}

// TestMatchSubcommand_RestoreStaged asserts "git restore --staged <file>" is allowed.
func TestMatchSubcommand_RestoreStaged(t *testing.T) {
	if got := MatchSubcommand("git restore --staged src/main.go"); got != "" {
		t.Fatalf("git restore --staged should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_RestoreWorktree asserts "git restore --worktree <file>" is blocked.
func TestMatchSubcommand_RestoreWorktree(t *testing.T) {
	if got := MatchSubcommand("git restore --worktree src/main.go"); got != "restore-path" {
		t.Fatalf("got %q want %q", got, "restore-path")
	}
}

// TestMatchSubcommand_UpdateRef asserts "git update-ref" is always blocked.
func TestMatchSubcommand_UpdateRef(t *testing.T) {
	if got := MatchSubcommand("git update-ref refs/heads/main HEAD"); got != "update-ref" {
		t.Fatalf("got %q want %q", got, "update-ref")
	}
}

// TestMatchSubcommand_RmForce asserts "git rm -f" is blocked.
func TestMatchSubcommand_RmForce(t *testing.T) {
	if got := MatchSubcommand("git rm -f file.go"); got != "rm-force" {
		t.Fatalf("got %q want %q", got, "rm-force")
	}
}

// TestMatchSubcommand_RmAllowed asserts "git rm --cached" (no -f) is allowed.
func TestMatchSubcommand_RmAllowed(t *testing.T) {
	if got := MatchSubcommand("git rm --cached file.go"); got != "" {
		t.Fatalf("git rm --cached should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_StatusAllowed asserts "git status" is allowed.
func TestMatchSubcommand_StatusAllowed(t *testing.T) {
	if got := MatchSubcommand("git status"); got != "" {
		t.Fatalf("git status should be allowed, got %q", got)
	}
}

// TestMatchSubcommand_EnvPrefix asserts "env git push" is blocked (env prefix stripped).
func TestMatchSubcommand_EnvPrefix(t *testing.T) {
	if got := MatchSubcommand("env git push origin main"); got != "push" {
		t.Fatalf("got %q want %q", got, "push")
	}
}

// TestMatchSubcommand_CommandPrefix asserts "command git push" is blocked.
func TestMatchSubcommand_CommandPrefix(t *testing.T) {
	if got := MatchSubcommand("command git push origin main"); got != "push" {
		t.Fatalf("got %q want %q", got, "push")
	}
}

// TestMatchSubcommand_EnvAssignment asserts "env FOO=bar git push" is blocked.
func TestMatchSubcommand_EnvAssignment(t *testing.T) {
	if got := MatchSubcommand("env FOO=bar git push origin main"); got != "push" {
		t.Fatalf("got %q want %q", got, "push")
	}
}

// TestMatchSubcommand_AbsPath asserts "/usr/bin/git push" is blocked.
func TestMatchSubcommand_AbsPath(t *testing.T) {
	if got := MatchSubcommand("/usr/bin/git push origin main"); got != "push" {
		t.Fatalf("got %q want %q", got, "push")
	}
}

// TestMatchSubcommand_CompoundBlocked asserts "echo ok; git push" is blocked.
func TestMatchSubcommand_CompoundBlocked(t *testing.T) {
	if got := MatchSubcommand("echo ok; git push origin main"); got != "push" {
		t.Fatalf("compound blocked: got %q want %q", got, "push")
	}
}

// TestMatchSubcommand_PipeBlocked asserts "ls | git push" blocks on the push segment.
func TestMatchSubcommand_PipeBlocked(t *testing.T) {
	if got := MatchSubcommand("ls | git push origin main"); got != "push" {
		t.Fatalf("got %q want %q", got, "push")
	}
}

// TestMatchSubcommand_MultilineNewlineBlocksSecondSegment asserts that a newline outside
// quotes creates a segment boundary, so git push in the second segment is blocked.
// Assertion: Group A — Go analyzes multiline commands segment-by-segment; a bare LF
// outside quotes is equivalent to `;` (bash quote_aware_split awk behavior).
func TestMatchSubcommand_MultilineNewlineBlocksSecondSegment(t *testing.T) {
	if got := MatchSubcommand("echo oi\ngit push origin main"); got != "push" {
		t.Fatalf("got %q want %q (newline outside quotes must create segment boundary)", got, "push")
	}
}

// --- RunGitBranch integration tests ---

// TestRunGitBranch_DenyPush asserts that a push payload from stdin → exit 2.
// Assertion: end-to-end deny for git push via stdin.
func TestRunGitBranch_DenyPush(t *testing.T) {
	dir := makeProjectDir(t)
	code, out, _ := runWith(t, dir, `{"tool_input":{"command":"git push origin main"}}`, "")
	if code != 2 {
		t.Fatalf("expected exit 2, got %d", code)
	}
	if out == "" {
		t.Fatal("expected hookSpecificOutput on stdout, got empty")
	}
}

// TestRunGitBranch_AllowStatus asserts that git status payload → exit 0.
// Assertion: allowed commands are not blocked.
func TestRunGitBranch_AllowStatus(t *testing.T) {
	dir := makeProjectDir(t)
	code, _, _ := runWith(t, dir, `{"tool_input":{"command":"git status"}}`, "")
	if code != 0 {
		t.Fatalf("expected exit 0 for git status, got %d", code)
	}
}

// TestRunGitBranch_NoOpOutsideProject asserts guard is a no-op outside a trackfw project.
// Assertion: exit 0 when trackfw.yaml not found up the directory tree.
func TestRunGitBranch_NoOpOutsideProject(t *testing.T) {
	dir := t.TempDir() // no trackfw.yaml here or in any parent within TempDir
	code, _, _ := runWith(t, dir, `{"tool_input":{"command":"git push origin main"}}`, "")
	if code != 0 {
		t.Fatalf("expected exit 0 outside project, got %d", code)
	}
}

// TestRunGitBranch_CommandFlag asserts --command flag bypasses stdin (D8).
// Assertion: --command "git push" → deny even with empty stdin.
func TestRunGitBranch_CommandFlag(t *testing.T) {
	dir := makeProjectDir(t)
	code, out, _ := runWith(t, dir, "", "git push origin main")
	if code != 2 {
		t.Fatalf("expected exit 2 with --command, got %d", code)
	}
	if out == "" {
		t.Fatal("expected hookSpecificOutput on stdout")
	}
}

// TestRunGitBranch_EnvFallback asserts TRACKFW_GIT_COMMAND is used when stdin is empty.
// Assertion: env var fallback channel (third priority after argv and stdin).
func TestRunGitBranch_EnvFallback(t *testing.T) {
	dir := makeProjectDir(t)
	var stdout, stderr bytes.Buffer
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	stdin := bytes.NewReader(nil) // empty
	getEnv := func(key string) string {
		if key == "TRACKFW_GIT_COMMAND" {
			return "git push origin main"
		}
		return ""
	}
	code := RunGitBranch(stdin, testIdleTimeout, "", getEnv, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit 2 via TRACKFW_GIT_COMMAND, got %d", code)
	}
}

// TestRunGitBranch_HookSpecificOutputFormat asserts the exact JSON format on stdout
// matches the .sh output byte-for-byte.
// Assertion: hookSpecificOutput JSON format is compatible with the .sh.
func TestRunGitBranch_HookSpecificOutputFormat(t *testing.T) {
	dir := makeProjectDir(t)
	_, out, _ := runWith(t, dir, `{"command":"git push origin main"}`, "")
	expected := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"` + reasonPush + `"}}` + "\n"
	if out != expected {
		t.Fatalf("stdout format mismatch:\ngot:  %q\nwant: %q", out, expected)
	}
}

// TestRunGitBranch_ReasonOnStderr asserts the REASON is written to stderr byte-for-byte.
// Assertion: stderr text equals the REASON constant (porta fiel to .sh).
func TestRunGitBranch_ReasonOnStderr(t *testing.T) {
	dir := makeProjectDir(t)
	_, _, errOut := runWith(t, dir, `{"command":"git push origin main"}`, "")
	want := reasonPush + "\n"
	if errOut != want {
		t.Fatalf("stderr mismatch:\ngot:  %q\nwant: %q", errOut, want)
	}
}

// TestRunGitBranch_CaseSensitivePayload is gate (ii): "Command" must not override "command".
// Assertion: {"tool_input":{"command":"git push","Command":"echo ok"}} → deny (fail-closed).
func TestRunGitBranch_CaseSensitivePayload(t *testing.T) {
	dir := makeProjectDir(t)
	payload := `{"tool_input":{"command":"git push origin main","Command":"echo ok"}}`
	code, _, _ := runWith(t, dir, payload, "")
	if code != 2 {
		t.Fatalf("case-sensitive gate (ii): expected exit 2, got %d", code)
	}
}

// TestRunGitBranch_NULDeny asserts NUL in command → deny (D2-bis).
// Assertion: command with \u0000 → exit 2 (fail-closed).
func TestRunGitBranch_NULDeny(t *testing.T) {
	dir := makeProjectDir(t)
	payload := `{"command":"git push\u0000origin main"}`
	code, _, _ := runWith(t, dir, payload, "")
	if code != 2 {
		t.Fatalf("NUL check: expected exit 2, got %d", code)
	}
}

// TestRunGitBranch_NonStringCommandAbsent asserts that a non-string command value is
// treated as absent (no-op), matching the bash awk extractor behavior.
// Assertion: Group B — {"command":99} → absent → guard is a no-op → rc=0.
func TestRunGitBranch_NonStringCommandAbsent(t *testing.T) {
	dir := makeProjectDir(t)
	payload := `{"command":99}`
	code, _, _ := runWith(t, dir, payload, "")
	if code != 0 {
		t.Fatalf("non-string command: expected exit 0 (no-op), got %d", code)
	}
}

// TestRunGitBranch_TruncatedDeny asserts truncated stdin → deny (fail-closed, section 0c).
// Uses testTimingWindow so elapsed bound distinguishes fixed-window from rolling-timer.
// Assertion: (payload; sleep 6) | guard → deny because window 2 fires empty at 2W.
// elapsed ≥ 1.5W proves window 2 was started (data received in window 1); the old
// rolling-timer code truncated at 1W so this test would fail there.
func TestRunGitBranch_TruncatedDeny(t *testing.T) {
	dir := makeProjectDir(t)
	var stdout, stderr bytes.Buffer
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// delayReader sends payload then blocks indefinitely — simulates "sleep 6" after payload.
	// Using testTimingWindow (200ms) for a measurable elapsed lower bound.
	r, _ := newDelayReader([]byte(`{"tool_input":{"command":"echo ok"}}`))

	start := time.Now()
	code := RunGitBranch(r, testTimingWindow, "", os.Getenv, &stdout, &stderr)
	elapsed := time.Since(start)

	if code != 2 {
		t.Fatalf("truncated stdin: expected exit 2, got %d (stdout=%q stderr=%q)", code, stdout.String(), stderr.String())
	}
	minElapsed := testTimingWindow * 3 / 2
	if elapsed < minElapsed {
		t.Fatalf("elapsed %v < %v: fixed-window semantics require 2 windows for this case", elapsed, minElapsed)
	}
}

// TestRunGitBranch_EarlyEOFAllow asserts that payload + EOF-in-second-window → allow for benign command.
// Uses testTimingWindow (200ms) so EOF at 1.5×window gives ≥100ms margin under -race.
// Assertion: (payload; sleep 3) | guard → allow: payload in window 1, EOF arrives at 1.5×window
// (mid-second-window), so no idle window ever fires and guard reads a complete payload.
func TestRunGitBranch_EarlyEOFAllow(t *testing.T) {
	dir := makeProjectDir(t)
	var stdout, stderr bytes.Buffer
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// timedEOFReader: sends payload immediately, then sleeps 1.5×testTimingWindow before EOF.
	// This simulates "(payload; sleep 3) | guard" with a 2s window.
	r := &timedEOFReader{
		data:     []byte(`{"tool_input":{"command":"echo ok"}}`),
		eofDelay: testTimingWindow * 3 / 2,
	}

	code := RunGitBranch(r, testTimingWindow, "", os.Getenv, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("sleep-3 analog (EOF in second window): expected exit 0, got %d (stdout=%q)", code, stdout.String())
	}
}

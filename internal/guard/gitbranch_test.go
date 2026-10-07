package guard

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
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

// --- M1: findSubcommand coverage ---

// TestMatchSubcommand_GlobalFlagAsLastToken_NoPanic asserts that a -C flag as the
// final token (no argument following) does not panic and returns no subcommand (no-op).
// Assertion: the fallback branch tokens=tokens[1:] in findSubcommand is reachable and
// safe; a truncated git command line does not cause a slice-out-of-bounds panic.
func TestMatchSubcommand_GlobalFlagAsLastToken_NoPanic(t *testing.T) {
	got := MatchSubcommand("git -C")
	if got != "" {
		t.Fatalf("git -C (flag as last token): expected allow, got %q", got)
	}
}

// TestMatchSubcommand_GlobalFlagWithArg_BlocksPush asserts that "git -C dir push"
// skips the -C flag and its value and correctly identifies push as the subcommand.
// Assertion: findSubcommand correctly skips the -C flag+value pair, leaving push as
// the subcommand, which applyRule then maps to the "push" block tag.
func TestMatchSubcommand_GlobalFlagWithArg_BlocksPush(t *testing.T) {
	got := MatchSubcommand("git -C /some/dir push origin main")
	if got != "push" {
		t.Fatalf("git -C dir push: expected block (push), got %q", got)
	}
}

// --- M2: parseHeredocDelim and stripHeredocBodies coverage ---

// TestParseHeredocDelim_IndentedHeredoc asserts that <<-EOF (indented heredoc) is
// recognized: the '-' is stripped and the delimiter "EOF" is returned.
// Assertion: the <<- branch in parseHeredocDelim is reachable and correctly strips
// the dash, allowing stripHeredocBodies to recognize and remove the heredoc body.
func TestParseHeredocDelim_IndentedHeredoc(t *testing.T) {
	got := parseHeredocDelim("cat <<-EOF")
	if got != "EOF" {
		t.Fatalf("parseHeredocDelim(<<-EOF): want %q, got %q", "EOF", got)
	}
}

// TestParseHeredocDelim_SingleQuotedDelim asserts that <<'EOF' strips the single
// quotes from the delimiter and returns "EOF".
// Assertion: the single-quote stripping in parseHeredocDelim is reachable; without it
// the stored delimiter would be "'EOF'" and the terminator "EOF" would never match.
func TestParseHeredocDelim_SingleQuotedDelim(t *testing.T) {
	got := parseHeredocDelim("cat <<'EOF'")
	if got != "EOF" {
		t.Fatalf("parseHeredocDelim(<<'EOF'): want %q, got %q", "EOF", got)
	}
}

// TestParseHeredocDelim_DoubleQuotedDelim asserts that <<"EOF" strips double quotes.
// Assertion: same mechanism as single-quote stripping, double-quote variant.
func TestParseHeredocDelim_DoubleQuotedDelim(t *testing.T) {
	got := parseHeredocDelim(`cat <<"EOF"`)
	if got != "EOF" {
		t.Fatalf("parseHeredocDelim(<<\"EOF\"): want %q, got %q", "EOF", got)
	}
}

// TestMatchSubcommand_HeredocIndented_BodyStripped asserts that cat <<-EOF with a
// tab-indented body containing "git push" is allowed: the body is stripped before
// segment analysis (verified against the .sh fixture: exit 0).
// Assertion: stripHeredocBodies recognizes <<- and removes the tab-indented body so
// the cat opener is analyzed as a harmless command and MatchSubcommand returns "".
func TestMatchSubcommand_HeredocIndented_BodyStripped(t *testing.T) {
	cmd := "cat <<-EOF\n\tgit push\nEOF"
	got := MatchSubcommand(cmd)
	if got != "" {
		t.Fatalf("<<-EOF with tab-indented body: expected allow, got %q", got)
	}
}

// TestMatchSubcommand_HeredocSingleQuotedDelim_BodyStripped asserts that cat <<'EOF'
// with a body containing "git push" is allowed after body stripping (verified against
// the .sh fixture: exit 0).
// Assertion: parseHeredocDelim strips single quotes from "'EOF'" so the stored
// delimiter is "EOF", the body is stripped, and the opener is harmless.
func TestMatchSubcommand_HeredocSingleQuotedDelim_BodyStripped(t *testing.T) {
	cmd := "cat <<'EOF'\ngit push\nEOF"
	got := MatchSubcommand(cmd)
	if got != "" {
		t.Fatalf("<<'EOF' with body: expected allow, got %q", got)
	}
}

// TestMatchSubcommand_HeredocDoubleQuotedDelim_BodyStripped asserts that cat <<"EOF"
// with a body containing "git push" is allowed after body stripping (verified against
// the .sh fixture: exit 0).
// Assertion: parseHeredocDelim strips double quotes from the delimiter; same mechanism
// as single-quote variant.
func TestMatchSubcommand_HeredocDoubleQuotedDelim_BodyStripped(t *testing.T) {
	cmd := "cat <<\"EOF\"\ngit push\nEOF"
	got := MatchSubcommand(cmd)
	if got != "" {
		t.Fatalf("<<\"EOF\" with body: expected allow, got %q", got)
	}
}

// TestMatchSubcommand_UnterminatedHeredoc_ReturnsOriginal asserts that an unclosed
// heredoc causes stripHeredocBodies to return the original string, so a "git push"
// appearing after the unclosed opener is not hidden (verified against the .sh
// fixture: exit 2, push reason).
// Assertion: the never-closed branch in stripHeredocBodies returns the original
// string, preserving "git push origin main" so the guard correctly blocks it.
func TestMatchSubcommand_UnterminatedHeredoc_ReturnsOriginal(t *testing.T) {
	cmd := "cat <<'EOF'\ngit push\nNOTEOF\ngit push origin main"
	got := MatchSubcommand(cmd)
	if got != "push" {
		t.Fatalf("unclosed heredoc before git push: expected block (push), got %q", got)
	}
}

// --- C2: single & operator and git.exe ---
// NOTE: These cases intentionally diverge from the .sh fixture (which treats bare
// & as opaque / allow). The .sh was written for bash semantics and never covered
// cmd.exe or PowerShell. The Go guard adds these as deliberate hardening for
// Windows platforms. The parity corpus contains no bare-& cases so parity tests
// remain green.

// TestMatchSubcommand_AmpersandSeparator_Blocks asserts that "echo ok & git push"
// is blocked: the bare `&` splits the command into two segments and the second
// segment contains "git push" which is blocked.
// Assertion: C2 fix — `&` outside quotes and not adjacent to `>/<` is a segment
// separator; cmd.exe uses `&` to chain commands, so `echo ok & git push` chains
// two commands and the guard must see both segments.
func TestMatchSubcommand_AmpersandSeparator_Blocks(t *testing.T) {
	got := MatchSubcommand("echo ok & git push origin main")
	if got != "push" {
		t.Fatalf("echo ok & git push: expected block (push), got %q", got)
	}
}

// TestMatchSubcommand_PSCallOperator_Blocks asserts that "& git push origin main"
// is blocked. In PowerShell, `&` is the call operator and `& git push` invokes
// git push. The leading `&` splits into an empty first segment and a second segment
// " git push origin main" which is blocked.
// Assertion: C2 fix — leading `&` as PS call operator is handled by the separator
// logic; the resulting segment contains "git push" and is blocked.
func TestMatchSubcommand_PSCallOperator_Blocks(t *testing.T) {
	got := MatchSubcommand("& git push origin main")
	if got != "push" {
		t.Fatalf("& git push: expected block (push), got %q", got)
	}
}

// TestMatchSubcommand_GitExe_Blocks asserts that "git.exe push origin main" is
// blocked. On Windows, `git.exe` is the idiomatic form of the git binary name.
// Assertion: C2 fix — .exe suffix is stripped and lowercased before the "git"
// basename check so `git.exe push` is treated identically to `git push`.
func TestMatchSubcommand_GitExe_Blocks(t *testing.T) {
	got := MatchSubcommand("git.exe push origin main")
	if got != "push" {
		t.Fatalf("git.exe push: expected block (push), got %q", got)
	}
}

// TestMatchSubcommand_GitExeUppercase_Blocks asserts that "GIT.EXE push" is blocked.
// Assertion: case-insensitive normalization treats GIT.EXE as git; same mechanism as
// git.exe.
func TestMatchSubcommand_GitExeUppercase_Blocks(t *testing.T) {
	got := MatchSubcommand("GIT.EXE push origin main")
	if got != "push" {
		t.Fatalf("GIT.EXE push: expected block (push), got %q", got)
	}
}

// TestMatchSubcommand_RedirectAmpersand_Allow asserts that "git status 2>&1" is
// allowed: the `&` is preceded by `>` and not a separator, so the command is
// analyzed as a single segment containing "git status" which is not blocked.
// Assertion: no false positive end-to-end — git status with 2>&1 redirect is not
// blocked. (Discriminating proof is TestQuoteAwareSplit_RedirectPreservesSegment below.)
func TestMatchSubcommand_RedirectAmpersand_Allow(t *testing.T) {
	got := MatchSubcommand("git status 2>&1")
	if got != "" {
		t.Fatalf("git status 2>&1: expected allow, got %q", got)
	}
}

// TestMatchSubcommand_AmpersandGtFile_Allow asserts that "ls &> /tmp/log" is
// allowed: `&>` is bash's stdout+stderr redirect; no blocked git command is present.
// Assertion: no false positive end-to-end. (Discriminating proof is
// TestQuoteAwareSplit_AmpGtPreservesSegment below.)
func TestMatchSubcommand_AmpersandGtFile_Allow(t *testing.T) {
	got := MatchSubcommand("ls &> /tmp/log")
	if got != "" {
		t.Fatalf("ls &> /tmp/log: expected allow, got %q", got)
	}
}

// --- C2: |& pipe operator ---

// TestMatchSubcommand_PipeAmpersand_Blocks asserts that "ls |& git push origin main"
// is blocked. In bash, `|&` pipes both stdout and stderr into the next command.
// The `|` splits into a first segment "ls " and a second segment "& git push ...".
// The leading `&` in the second segment is stripped (MatchSubcommand's segment-level
// strip), leaving "git push origin main" which is blocked.
// Assertion: `|&` does not create a fail-open path; the leading `&` left by `|`
// is stripped from the segment before analysis.
func TestMatchSubcommand_PipeAmpersand_Blocks(t *testing.T) {
	got := MatchSubcommand("ls |& git push origin main")
	if got != "push" {
		t.Fatalf("ls |& git push: expected block (push), got %q", got)
	}
}

// TestMatchSubcommand_PipeAmpersandNoSpace_Blocks asserts that "ls |&git push" is
// blocked: the `&g` has no space so the leading-& strip must work on characters,
// not tokens.
// Assertion: character-level leading-`&` strip in MatchSubcommand handles the
// no-space form as well as the spaced form.
func TestMatchSubcommand_PipeAmpersandNoSpace_Blocks(t *testing.T) {
	got := MatchSubcommand("ls |&git push origin main")
	if got != "push" {
		t.Fatalf("ls |&git push (no space): expected block (push), got %q", got)
	}
}

// --- quoteAwareSplit segment tests for redirect exception ---
// These tests are the discriminating proofs for TestMatchSubcommand_RedirectAmpersand_Allow
// and TestMatchSubcommand_AmpersandGtFile_Allow. They fail if the redirect exception
// is removed from quoteAwareSplit.

// TestQuoteAwareSplit_RedirectPreservesSegment asserts that "git status 2>&1" is
// one segment. If the redirect exception were removed, `&` preceded by `>` would
// become a separator: ["git status 2>", "1"].
// Assertion: C2 redirect exception — `&` preceded by `>` is not a separator;
// without it, `2>&1` is split and the test produces 2 segments, not 1.
func TestQuoteAwareSplit_RedirectPreservesSegment(t *testing.T) {
	parts := quoteAwareSplit("git status 2>&1")
	if len(parts) != 1 {
		t.Fatalf("git status 2>&1: expected 1 segment, got %d: %q", len(parts), parts)
	}
	if parts[0] != "git status 2>&1" {
		t.Fatalf("segment mismatch: got %q, want %q", parts[0], "git status 2>&1")
	}
}

// TestQuoteAwareSplit_AmpGtPreservesSegment asserts that "make &> log" is one segment.
// If the redirect exception were removed, `&` followed by `>` would become a separator.
// Assertion: C2 redirect exception — `&` followed by `>` is not a separator.
func TestQuoteAwareSplit_AmpGtPreservesSegment(t *testing.T) {
	parts := quoteAwareSplit("make &> log")
	if len(parts) != 1 {
		t.Fatalf("make &> log: expected 1 segment, got %d: %q", len(parts), parts)
	}
	if parts[0] != "make &> log" {
		t.Fatalf("segment mismatch: got %q, want %q", parts[0], "make &> log")
	}
}

// TestQuoteAwareSplit_PipeAmpersandSplitsOnPipe asserts that "ls |& cat" produces
// two segments: ["ls ", "& cat"]. The `|` splits; the resulting `& cat` segment
// retains the `&` which is then stripped by MatchSubcommand's per-segment strip.
// Assertion: `|&` splits on `|` (not on `&`), leaving the leading `&` in the
// second segment for MatchSubcommand to strip.
func TestQuoteAwareSplit_PipeAmpersandSplitsOnPipe(t *testing.T) {
	parts := quoteAwareSplit("ls |& cat")
	if len(parts) != 2 {
		t.Fatalf("ls |& cat: expected 2 segments, got %d: %q", len(parts), parts)
	}
	if parts[0] != "ls " {
		t.Fatalf("first segment: got %q, want %q", parts[0], "ls ")
	}
	if parts[1] != "& cat" {
		t.Fatalf("second segment: got %q, want %q", parts[1], "& cat")
	}
}

// TestMatchSubcommand_QuotedAmpersand_Allow asserts that `echo "a & git push"` is
// allowed: the `&` is inside double quotes and quoteAwareSplit must not split there.
// Assertion: the quote-aware scanner ignores special characters inside quotes; a
// bare `&` inside a string literal is not a command separator.
func TestMatchSubcommand_QuotedAmpersand_Allow(t *testing.T) {
	got := MatchSubcommand(`echo "a & git push"`)
	if got != "" {
		t.Fatalf(`echo "a & git push": expected allow, got %q`, got)
	}
}

// TestMatchSubcommand_DoubleAmpersand_Allow asserts that "echo ok && ls" is allowed
// and that the existing `&&` separator behavior is not broken by the C2 change.
// Assertion: `&&` continues to split as before; neither side contains a blocked git
// command in this input.
func TestMatchSubcommand_DoubleAmpersand_Allow(t *testing.T) {
	got := MatchSubcommand("echo ok && ls")
	if got != "" {
		t.Fatalf("echo ok && ls: expected allow, got %q", got)
	}
}

// TestMatchSubcommand_WindowsGitExePath_Push_Blocks asserts that on Windows, full
// paths to git (without spaces) are recognised and a blocked subcommand causes rc=2.
//
// How each case produces base=="git" on Windows:
//   - C:\Git\cmd\git push origin main      → tokens[0]="C:\Git\cmd\git";
//     filepath.Base returns "git"; ToLower+TrimSuffix(".exe") → "git". Guard fires.
//   - C:\Git\cmd\git.exe push origin main  → tokens[0]="C:\Git\cmd\git.exe";
//     filepath.Base returns "git.exe"; TrimSuffix(".exe") → "git". Guard fires.
//   - C:\PROGRA~1\Git\bin\git.exe push … → tokens[0]="C:\PROGRA~1\Git\bin\git.exe";
//     filepath.Base returns "git.exe"; TrimSuffix(".exe") → "git". Guard fires.
//
// NOTE: paths with spaces inside double-quoted strings (e.g.
// "C:\Program Files\Git\bin\git.exe" push) are handled by the D10 POSIX fix in
// quoteAwareSplit (ADR-2026-10-04): \ before non-special chars is now preserved
// inside "…", so shellTokenize sees the full backslash-separated path and
// filepath.Base on Windows returns "git.exe". See
// TestMatchSubcommand_D10_WindowsQuotedPathWithSpaces_Blocks.
//
// Assertion: on Windows, space-free git paths followed by a blocked subcommand are
// blocked by MatchSubcommand (deliberate divergence from POSIX N09 behaviour —
// see PR #527, ADR-2026-10-04).
func TestMatchSubcommand_WindowsGitExePath_Push_Blocks(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only: filepath.Base trata '\\' como separador")
	}
	cases := []struct {
		cmd  string
		want string
	}{
		{`C:\Git\cmd\git push origin main`, "push"},
		{`C:\Git\cmd\git.exe push origin main`, "push"},
		{`C:\PROGRA~1\Git\bin\git.exe push origin main`, "push"},
	}
	for _, tc := range cases {
		if got := MatchSubcommand(tc.cmd); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.cmd, got, tc.want)
		}
	}
}

// TestMatchSubcommand_BackslashPrefixed_PosixNotGit asserts that on POSIX (non-Windows),
// a token like "\\git" (backslash-prefixed) is NOT treated as "git" by MatchSubcommand.
// On POSIX, filepath.Base does not treat '\' as a path separator, so the entire token
// "\\git" remains intact and does not equal "git" — the command is allowed.
// Assertion: POSIX behaviour — N09 backslash-prefixed token is not blocked (the POSIX
// side of the deliberate divergence documented in PR #527, ADR-2026-10-04).
func TestMatchSubcommand_BackslashPrefixed_PosixNotGit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only: on Windows filepath.Base('\\\\git') returns 'git' — divergence is intentional")
	}
	if got := MatchSubcommand(`\\git push`); got != "" {
		t.Fatalf(`\\git push: expected allow on POSIX (backslash not a path separator), got %q`, got)
	}
}

// --- D10: shell quoting — remoção de aspas na tokenização (ADR-2026-10-04) ---

// TestMatchSubcommand_D10_DoubleQuotedGit_Blocks asserts that double-quoting the git
// binary word ("git" push) is recognised and blocked after shell dequoting.
// Assertion: D10 — shellTokenize strips quotes from "git" → git; guard fires on push.
func TestMatchSubcommand_D10_DoubleQuotedGit_Blocks(t *testing.T) {
	if got := MatchSubcommand(`"git" push origin main`); got != "push" {
		t.Fatalf(`"git" push origin main: expected block (push), got %q`, got)
	}
}

// TestMatchSubcommand_D10_DoubleQuotedSubcommand_Blocks asserts that double-quoting
// the subcommand word (git "push") is recognised and blocked after shell dequoting.
// Assertion: D10 — shellTokenize strips quotes from "push" → push; applyRule fires.
func TestMatchSubcommand_D10_DoubleQuotedSubcommand_Blocks(t *testing.T) {
	if got := MatchSubcommand(`git "push" origin main`); got != "push" {
		t.Fatalf(`git "push" origin main: expected block (push), got %q`, got)
	}
}

// TestMatchSubcommand_D10_SingleQuotedGit_Blocks asserts that single-quoting the git
// binary word ('git' push) is recognised and blocked after shell dequoting.
// Assertion: D10 — shellTokenize strips single quotes from 'git' → git; guard fires.
func TestMatchSubcommand_D10_SingleQuotedGit_Blocks(t *testing.T) {
	if got := MatchSubcommand(`'git' push`); got != "push" {
		t.Fatalf(`'git' push: expected block (push), got %q`, got)
	}
}

// TestMatchSubcommand_D10_EmptyQuotedMiddle_Blocks asserts that an empty double-quoted
// string spliced into a word (git p""ush) is recognised after concatenation.
// Assertion: D10 — shellTokenize concatenates p + "" + ush → push; applyRule fires.
func TestMatchSubcommand_D10_EmptyQuotedMiddle_Blocks(t *testing.T) {
	if got := MatchSubcommand(`git p""ush`); got != "push" {
		t.Fatalf(`git p""ush: expected block (push), got %q`, got)
	}
}

// TestMatchSubcommand_D10_EnvDoubleQuotedGit_Blocks asserts that "git" with env prefix
// is blocked after shell dequoting and env stripping.
// Assertion: D10 — env "git" push: shellTokenize → ["env","git","push"]; env stripped;
// base "git" fires.
func TestMatchSubcommand_D10_EnvDoubleQuotedGit_Blocks(t *testing.T) {
	if got := MatchSubcommand(`env "git" push`); got != "push" {
		t.Fatalf(`env "git" push: expected block (push), got %q`, got)
	}
}

// TestMatchSubcommand_D10_CommandSingleQuotedGit_Blocks asserts that 'git' with
// command prefix is blocked after shell dequoting.
// Assertion: D10 — command 'git' push: shellTokenize → ["command","git","push"];
// command stripped; base "git" fires.
func TestMatchSubcommand_D10_CommandSingleQuotedGit_Blocks(t *testing.T) {
	if got := MatchSubcommand(`command 'git' push`); got != "push" {
		t.Fatalf(`command 'git' push: expected block (push), got %q`, got)
	}
}

// TestMatchSubcommand_D10_AbsPathDoubleQuoted_Blocks asserts that "/usr/bin/git" push
// is blocked on all platforms: quoteAwareSplit does not corrupt POSIX paths (no
// backslashes), shellTokenize dequotes the token, filepath.Base returns "git".
// Assertion: D10 — "/usr/bin/git" push: shellTokenize → ["/usr/bin/git","push"];
// filepath.Base("/usr/bin/git")="git"; guard fires.
func TestMatchSubcommand_D10_AbsPathDoubleQuoted_Blocks(t *testing.T) {
	if got := MatchSubcommand(`"/usr/bin/git" push origin main`); got != "push" {
		t.Fatalf(`"/usr/bin/git" push origin main: expected block (push), got %q`, got)
	}
}

// TestMatchSubcommand_D10_EchoDoubleQuotedGitPush_Allows asserts that a command string
// where "git push" appears only inside a quoted echo argument is allowed.
// Assertion: D10 does not widen blocking to quoted arguments; echo is not git.
func TestMatchSubcommand_D10_EchoDoubleQuotedGitPush_Allows(t *testing.T) {
	if got := MatchSubcommand(`echo "git push"`); got != "" {
		t.Fatalf(`echo "git push": expected allow (git push is an argument, not a command), got %q`, got)
	}
}

// TestMatchSubcommand_D10_GitLogGrep_Allows asserts that "push" as an argument to
// --grep (git log --grep "push") is not treated as a subcommand and is allowed.
// Assertion: D10 does not change subcommand matching; dequoted "push" is an arg, not sub.
func TestMatchSubcommand_D10_GitLogGrep_Allows(t *testing.T) {
	if got := MatchSubcommand(`git log --grep "push"`); got != "" {
		t.Fatalf(`git log --grep "push": expected allow (log is not a blocked sub), got %q`, got)
	}
}

// TestMatchSubcommand_D10_LsQuotedPath_Allows asserts that ls "a b" is allowed.
// Assertion: D10 does not affect non-git commands; first token ls ≠ git.
func TestMatchSubcommand_D10_LsQuotedPath_Allows(t *testing.T) {
	if got := MatchSubcommand(`ls "a b"`); got != "" {
		t.Fatalf(`ls "a b": expected allow (ls is not git), got %q`, got)
	}
}

// TestMatchSubcommand_D10_EchoSingleQuotedRedirect_Allows asserts that
// echo 'git push' > f is allowed.
// Assertion: D10 does not block; first token echo ≠ git.
func TestMatchSubcommand_D10_EchoSingleQuotedRedirect_Allows(t *testing.T) {
	if got := MatchSubcommand(`echo 'git push' > f`); got != "" {
		t.Fatalf(`echo 'git push' > f: expected allow (echo is not git), got %q`, got)
	}
}

// TestMatchSubcommand_D10_UnclosedDoubleQuote_NoPanic asserts that an unclosed
// double-quote string ("git push) does not panic and returns a stable result.
// Assertion: D10 — unclosed quote: shellTokenize treats remainder as one word;
// the word is "git push" (with space inside), filepath.Base="git push" ≠ "git" → allow.
func TestMatchSubcommand_D10_UnclosedDoubleQuote_NoPanic(t *testing.T) {
	// Must not panic. The result (allow or block) depends on how the unclosed-quote
	// remainder is tokenised: the entire remainder "git push" is one token, whose
	// filepath.Base is "git push" (≠ "git"), so the guard allows it.
	got := MatchSubcommand(`"git push`)
	if got != "" {
		t.Fatalf(`"git push (unclosed): expected allow (remainder is one token "git push" ≠ "git"), got %q`, got)
	}
}

// --- D10/quoteAwareSplit: POSIX backslash rules inside "…" (ADR-2026-10-04, D10 fix) ---

// TestQuoteAwareSplit_BackslashBeforeNonSpecial_Preserved asserts that inside "…",
// a backslash before a non-special character is preserved literally as '\' + char.
// POSIX rule: only $, `, ", \, and newline are escaped by \ inside "…"; before any
// other character the backslash is retained.
// echo "a\b" → segment = `echo "a\b"` (backslash before 'b' is not consumed).
// Assertion: D10 fix — \b inside "…" produces \b (not just b); without the fix the
// segment would be `echo "ab"` and this test fails.
func TestQuoteAwareSplit_BackslashBeforeNonSpecial_Preserved(t *testing.T) {
	parts := quoteAwareSplit(`echo "a\b"`)
	if len(parts) != 1 {
		t.Fatalf(`echo "a\b": expected 1 segment, got %d: %v`, len(parts), parts)
	}
	// With the fix, quoteAwareSplit preserves the backslash; the segment is unchanged.
	if parts[0] != `echo "a\b"` {
		t.Fatalf(`echo "a\b": expected segment %q, got %q`, `echo "a\b"`, parts[0])
	}
}

// TestQuoteAwareSplit_EscapedQuoteInsideDoubleQuote_OneSegment asserts that \" inside
// "…" does not close the quoted region; the ; that follows remains inside the string.
// echo "a\"b; git push" → one segment (the \" is an escaped quote, not a closer).
// Assertion: D10 fix preserves the existing behavior for \" (special-set character
// that was already consumed before the fix); this test is a non-regression guard.
func TestQuoteAwareSplit_EscapedQuoteInsideDoubleQuote_OneSegment(t *testing.T) {
	parts := quoteAwareSplit(`echo "a\"b; git push"`)
	if len(parts) != 1 {
		t.Fatalf(`echo "a\"b; git push": expected 1 segment, got %d: %v`, len(parts), parts)
	}
}

// TestQuoteAwareSplit_DoubleBackslashClosesString_TwoSegments asserts that \\ inside
// "…" produces a single \ and the next " properly closes the string; the ; after is
// then a segment separator. The second segment contains "git push" which is blocked.
// echo "x\\"; git push → two segments, second blocks.
// Assertion: D10 fix preserves the existing behavior for \\ (special-set character
// that was already consumed before the fix); this test is a non-regression guard.
func TestQuoteAwareSplit_DoubleBackslashClosesString_TwoSegments(t *testing.T) {
	parts := quoteAwareSplit(`echo "x\\"; git push`)
	if len(parts) != 2 {
		t.Fatalf(`echo "x\\"; git push: expected 2 segments, got %d: %v`, len(parts), parts)
	}
	if got := MatchSubcommand(`echo "x\\"; git push`); got != "push" {
		t.Fatalf(`echo "x\\"; git push: expected block (push), got %q`, got)
	}
}

// TestQuoteAwareSplit_D10_WindowsPathBackslashPreserved asserts that quoteAwareSplit
// followed by shellTokenize produces the correct token for a Windows path inside "…".
// This cross-platform test verifies the tokenisation mechanism; the Windows-only
// end-to-end blocking test is TestMatchSubcommand_D10_WindowsQuotedPathWithSpaces_Blocks.
// Assertion: D10 fix — \P inside "…" writes \P (not P); shellTokenize receives the
// full backslash-separated path as one token. Without the fix, tokens[0] would be
// "C:Program FilesGitbingit.exe" (backslashes stripped).
func TestQuoteAwareSplit_D10_WindowsPathBackslashPreserved(t *testing.T) {
	seg := quoteAwareSplit(`"C:\Program Files\Git\bin\git.exe" push`)
	if len(seg) != 1 {
		t.Fatalf(`expected 1 segment, got %d: %v`, len(seg), seg)
	}
	tokens := shellTokenize(seg[0])
	if len(tokens) < 1 {
		t.Fatalf("expected at least 1 token, got 0")
	}
	want := `C:\Program Files\Git\bin\git.exe`
	if tokens[0] != want {
		t.Fatalf("first token: got %q, want %q", tokens[0], want)
	}
}

// TestMatchSubcommand_D10_WindowsQuotedPathWithSpaces_Blocks asserts that on Windows,
// a double-quoted Windows path that contains spaces is correctly tokenised after the
// D10 POSIX fix and a blocked subcommand causes MatchSubcommand to return "push".
// On Windows, filepath.Base treats \ as a separator, so the token
// "C:\Program Files\Git\bin\git.exe" has base "git.exe" → stripped to "git" → BLOCK.
// On POSIX, filepath.Base does not treat \ as a separator, so the whole token is
// "C:\Program Files\Git\bin\git.exe" (no slash), base ≠ "git" → ALLOW (correct).
// Assertion: D10 fix — Windows path with spaces in "…" is blocked on Windows only;
// the fix makes this case work by preserving backslashes inside "…".
func TestMatchSubcommand_D10_WindowsQuotedPathWithSpaces_Blocks(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only: filepath.Base treats '\\' as separator (D10, ADR-2026-10-04)")
	}
	cases := []struct {
		cmd  string
		want string
	}{
		{`"C:\Program Files\Git\bin\git.exe" push origin main`, "push"},
		{`& "C:\Program Files\Git\bin\git.exe" push origin main`, "push"},
	}
	for _, tc := range cases {
		if got := MatchSubcommand(tc.cmd); got != tc.want {
			t.Errorf("%q: got %q want %q", tc.cmd, got, tc.want)
		}
	}
}

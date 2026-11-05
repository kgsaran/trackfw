// Package guard — wrapper and POSIX backslash tests for the git-branch guard.
// AC4 and AC5 test cases for ML-1B (REQ-2026-11-05, D4).
//
// Regra Dura de Reconciliação: each new test below carries a one-sentence
// reconciliation that states what conclusion of this ML the test asserts.
package guard

import (
	"fmt"
	"strings"
	"testing"
)

// --- AC4: shell wrappers blocked (D4) ---

// TestMatchSubcommand_AC4_Wrapper_Blocked asserts that shell wrappers that
// previously bypassed the guard (rc=0) now return a non-empty block tag.
// Each row is a confirmed pre-fix bypass that D4 must close.
//
// Reconciliation: asserts that D4 (matchSubcommandPOSIX + extended
// stripEnvCommandPrefix) closes every shell-wrapper bypass listed in AC4 of
// the Wave 0 parecer §3.
func TestMatchSubcommand_AC4_Wrapper_Blocked(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want string
	}{
		// Shell -c invocations.
		{"bash-c-git-push", `bash -c "git push"`, "push"},
		{"sh-c-git-push", `sh -c "git push"`, "push"},
		{"bash-lc-git-push", `bash -lc "git push"`, "push"},
		{"bash-ec-git-push", `bash -ec "git push"`, "push"},
		{"bash-xc-git-push", `bash -xc "git push"`, "push"},
		{"bash-l-c-git-push", `bash -l -c "git push"`, "push"},
		{"bash-c-e-git-push", `bash -c -e "git push"`, "push"},
		{"bash-posix-c-git-push", `bash --posix -c "git push"`, "push"},
		{"bash-o-pipefail-c-git-push", `bash -o pipefail -c "git push"`, "push"},
		{"eval-git-push", `eval "git push"`, "push"},
		{"pwsh-c-git-push", `pwsh -c "git push"`, "push"},
		{"powershell-command-git-push", `powershell -Command "git push"`, "push"},
		// B1: flag abbreviations for -Command (prefix matching) and --command (double-dash).
		// Reconciliation: asserts that isPsCommandFlag matches any prefix of "-command", so that
		// -co, -com, --command etc. all route to extractPowerShellCommandString as a command flag.
		{"powershell-co-git-push", `powershell -co "git push"`, "push"},
		{"powershell-com-git-push", `powershell -com "git push"`, "push"},
		{"powershell-double-dash-command-git-push", `powershell --command "git push"`, "push"},
		{"pwsh-co-git-push", `pwsh -co "git push"`, "push"},
		{"cmd-c-git-push", `cmd /c "git push"`, "push"},
		// ML-2E: powershell (PS 5.1) positional -Command — first non-flag token is the
		// implicit command string.
		// Reconciliation: asserts that extractPowerShellCommandString with
		// usePositionalCommand=true treats the first non-flag token as -Command for
		// powershell (PS 5.1), so "powershell git push" and "powershell \"git push\""
		// are blocked; pwsh git push is a declared residual (see Allowed table).
		{"powershell-positional-bare-git-push", `powershell git push`, "push"},
		{"powershell-positional-quoted-git-push", `powershell "git push"`, "push"},
		{"powershell-noprofile-positional-git-push", `powershell -NoProfile git push`, "push"},
		{"powershell-exe-ep-bypass-positional-git-push", `powershell.exe -ExecutionPolicy Bypass git push`, "push"},
		// Env/command prefix + wrapper.
		{"env-bash-c-git-push", `env bash -c "git push"`, "push"},
		{"sudo-bash-c-git-push", `sudo bash -c "git push"`, "push"},
		// Nested wrappers (depth=2).
		{"bash-c-bash-c-single-quote", `bash -c "bash -c 'git push'"`, "push"},
		{"bash-c-bash-c-escaped-quote", `bash -c "bash -c \"git push\""`, "push"},
		// Extended prefix: env flags.
		{"env-i-git-push", `env -i git push`, "push"},
		{"env-dash-dash-git-push", `env -- git push`, "push"},
		// command -p.
		{"command-p-git-push", `command -p git push`, "push"},
		// Naked assignments.
		{"naked-assignment-git-push", `FOO=1 git push`, "push"},
		// POSIX backslash — ONE backslash: \g → g → token "git".
		// In Go double-quoted string "\\git push", the string has ONE backslash.
		{"backslash-prefix-git-push", "\\git push", "push"},
		// g\it push: ONE backslash mid-token — POSIX \i → i, token = "git".
		{"mid-token-backslash-git-push", "g\\it push", "push"},
		// Wrapper prefixes.
		{"nohup-git-push", `nohup git push`, "push"},
		{"time-git-push", `time git push`, "push"},
		{"exec-git-push", `exec git push`, "push"},
		{"exec-a-x-git-push", `exec -a x git push`, "push"},
		{"timeout-10-git-push", `timeout 10 git push`, "push"},
		{"timeout-10s-git-push", `timeout 10s git push`, "push"},
		{"timeout-k-5-10-git-push", `timeout -k 5 10 git push`, "push"},
		{"nice-git-push", `nice git push`, "push"},
		{"nice-n-10-git-push", `nice -n 10 git push`, "push"},
		{"doas-git-push", `doas git push`, "push"},
		{"doas-u-x-git-push", `doas -u x git push`, "push"},
		{"stdbuf-oL-git-push", `stdbuf -oL git push`, "push"},
		// F7: additional shells, cmd /k, uppercase interpreter, uppercase git,
		// and git.exe — Reconciliation: asserts that D4 covers zsh, dash, ksh,
		// cmd /k, case-insensitive interpreter and git names, and .exe suffix.
		// All confirmed blocked in ML-2C integration testing.
		{"zsh-c-git-push", `zsh -c 'git push'`, "push"},
		{"dash-c-git-push", `dash -c 'git push'`, "push"},
		{"ksh-c-git-push", `ksh -c 'git push'`, "push"},
		{"cmd-k-git-push", `cmd /k git push`, "push"},
		{"BASH-uppercase-c-git-push", `BASH -c 'git push'`, "push"},
		{"GIT-uppercase-push", `GIT push`, "push"},
		{"git-exe-push", `git.exe push`, "push"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchSubcommand(tc.cmd)
			if got != tc.want {
				t.Errorf("MatchSubcommand(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
		})
	}
}

// TestMatchSubcommand_AC4_Wrapper_Allowed asserts that commands that look like
// shell wrappers but do not invoke a blocked git subcommand are NOT blocked.
// These are the mandatory false-positive checks from AC4 §3 of the parecer.
//
// Reconciliation: asserts that D4 does NOT trigger on git status/log/diff inside
// bash -c, nor on "bash -c" appearing as text in a git commit message.
func TestMatchSubcommand_AC4_Wrapper_Allowed(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"bash-c-git-status", `bash -c "git status"`},
		{"bash-c-git-log", `bash -c "git log"`},
		{"bash-c-git-diff", `bash -c "git diff"`},
		{"bash-c-echo-not-git", `bash -c "echo 'git push done'"`},
		// depth=2 with inner allowed:
		{"bash-c-bash-c-git-status", `bash -c "bash -c 'git status'"`},
		// Two double-backslashes: POSIX \\g → \g → token \git ≠ git → allowed.
		// (This is NOT the bypass; the bypass is single-backslash \git.)
		{"double-backslash-not-bypass", `\\git push`},
		// ML-2E: powershell false-positive checks — allowed git reads and non-git.
		{"powershell-get-childitem", `powershell Get-ChildItem`},
		{"powershell-file-build-ps1", `powershell -File build.ps1`},
		{"powershell-dot-build-ps1", `powershell .\build.ps1`},
		// pwsh positional is -File (not -Command): pwsh git push is a declared residual.
		// Reconciliation: asserts the declared limit — pwsh (PS 7) first positional is
		// -File, so "pwsh git push" is NOT treated as a command string and is allowed.
		{"pwsh-positional-git-push-residual", `pwsh git push`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchSubcommand(tc.cmd)
			if got != "" {
				t.Errorf("MatchSubcommand(%q) = %q, want %q (allow)", tc.cmd, got, "")
			}
		})
	}
}

// TestMatchSubcommand_AC4_Depth3_WrapperNotAnalyzed asserts that a triple-nested
// shell invocation (depth=3) returns "wrapper-not-analyzed" (fail-closed).
//
// Reconciliation: asserts the depth=2 ceiling from D4 — at depth 3 the guard
// returns "wrapper-not-analyzed" rather than approving the unanalysed command.
func TestMatchSubcommand_AC4_Depth3_WrapperNotAnalyzed(t *testing.T) {
	// bash -c "bash -c \"bash -c 'git status'\""
	// depth: outer=0, middle=1, inner=2 encounters another wrapper → wrapper-not-analyzed.
	cmd := `bash -c "bash -c \"bash -c 'git status'\""`
	got := MatchSubcommand(cmd)
	if got != "wrapper-not-analyzed" {
		t.Errorf("depth-3 wrapper: MatchSubcommand(%q) = %q, want %q",
			cmd, got, "wrapper-not-analyzed")
	}
}

// TestMatchSubcommand_AC4_Regression_SemicolonAfterEscapedQuote asserts that
// echo "\"" ; git push still produces two segments after the POSIX fix.
//
// Reconciliation: asserts that quoteAwareSplitPOSIX still treats ; outside
// quotes as a segment separator even after the \" verbatim-emission change —
// the second segment "git push" is correctly blocked.
func TestMatchSubcommand_AC4_Regression_SemicolonAfterEscapedQuote(t *testing.T) {
	cmd := `echo "\"" ; git push`
	got := MatchSubcommand(cmd)
	if got != "push" {
		t.Errorf(`echo "\"" ; git push: MatchSubcommand = %q, want "push"`, got)
	}
}

// TestMatchSubcommand_AC4_Regression_CommitMsgWithEscapedQuote asserts that
// git commit -m "say \"hi\"" is blocked for the reason "commit" and NOT because
// D4 misreads the message as a shell invocation.
//
// Reconciliation: asserts that D4 does not trigger on "say \"hi\"" as a shell
// command string — the block reason remains "commit" (the git commit rule), not
// a false D4 wrapper detection inside the commit message.
func TestMatchSubcommand_AC4_Regression_CommitMsgWithEscapedQuote(t *testing.T) {
	cmd := `git commit -m "say \"hi\""`
	got := MatchSubcommand(cmd)
	if got != "commit" {
		t.Errorf(`git commit -m "say \"hi\"": MatchSubcommand = %q, want "commit"`, got)
	}
}

// TestMatchSubcommand_C7_CmdSubst_Blocked asserts that command substitution
// bodies ($(...) and `...`) containing a blocked git subcommand are detected.
//
// Reconciliation: asserts that extractCmdSubstBodies in matchSubcommandPOSIX
// (C7, ML-2C) scans each command-substitution body as a subshell, so that
// $(git push), `git push`, and OUT=$(git commit -m x) are all blocked (D4).
func TestMatchSubcommand_C7_CmdSubst_Blocked(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want string
	}{
		{"dollar-paren-push", `$(git push)`, "push"},
		{"backtick-push", "`git push`", "push"},
		{"assignment-dollar-paren-commit", `OUT=$(git commit -m x)`, "commit"},
		{"double-quoted-dollar-paren-push", `echo "$(git push)"`, "push"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchSubcommand(tc.cmd)
			if got != tc.want {
				t.Errorf("MatchSubcommand(%q) = %q, want %q", tc.cmd, got, tc.want)
			}
		})
	}
}

// TestMatchSubcommand_C7_CmdSubst_Allowed asserts that command substitution
// bodies with allowed git subcommands do NOT trigger a block.
//
// Reconciliation: asserts that C7 does not introduce false positives for
// $(git status), `git log`, and $(echo …) — only blocked subcommands inside
// $(...) or `...` produce a denial.
func TestMatchSubcommand_C7_CmdSubst_Allowed(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"dollar-paren-status", `echo "$(git status)"`},
		{"backtick-log", "`git log --oneline`"},
		{"dollar-paren-echo", `$(echo ok)`},
		{"assign-dollar-paren-describe", `VER=$(git describe --tags)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchSubcommand(tc.cmd)
			if got != "" {
				t.Errorf("MatchSubcommand(%q) = %q, want %q (allow)", tc.cmd, got, "")
			}
		})
	}
}

// --- AC5: declared limit (language runtime interpreters) ---

// TestMatchSubcommand_AC5_LanguageRuntime_Allowed asserts that python -c with
// a subprocess call to git push is NOT blocked by the guard.
// This test fixes the declared limit (D5): language runtime interpreters whose
// command string contains git calls are out of scope for analysis.
//
// Reconciliation: asserts the D5 limit — python -c is not a recognised shell
// interpreter in D4; the guard returns "" (allow), and the test documents this
// as a known, intentional boundary rather than a gap to be fixed.
func TestMatchSubcommand_AC5_LanguageRuntime_Allowed(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{
			"python-c-subprocess",
			`python -c "import subprocess; subprocess.run(['git','push'])"`,
		},
		{
			"node-e-exec",
			`node -e "require('child_process').exec('git push')"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchSubcommand(tc.cmd)
			if got != "" {
				t.Errorf("AC5 limit: MatchSubcommand(%q) = %q, want %q (declared limit: language runtimes not analysed)",
					tc.cmd, got, "")
			}
		})
	}
}

// --- RunGitBranch end-to-end: key AC4 wrappers ---

// TestRunGitBranch_AC4_WrapperBlocked asserts end-to-end (JSON payload → rc=2)
// for a selection of key AC4 wrapper cases.
//
// Reconciliation: asserts that the full RunGitBranch pipeline — JSON decode,
// MatchSubcommand, switch, denyOut — produces rc=2 for shell wrappers that
// contain a blocked git subcommand.
func TestRunGitBranch_AC4_WrapperBlocked(t *testing.T) {
	dir := makeProjectDir(t)
	cases := []struct {
		name string
		cmd  string
	}{
		{"bash-c-git-push", `bash -c "git push"`},
		{"eval-git-push", `eval "git push"`},
		{"nohup-git-push", `nohup git push`},
		{"FOO=1-git-push", `FOO=1 git push`},
		{"backslash-git-push", "\\git push"},
		{"nested-depth2", `bash -c "bash -c 'git push'"`},
		{"depth3-fail-closed", `bash -c "bash -c \"bash -c 'git status'\""`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":%q}}`, tc.cmd)
			code, _, _ := runWith(t, dir, payload, "")
			if code != 2 {
				t.Errorf("cmd %q: got rc=%d, want rc=2", tc.cmd, code)
			}
		})
	}
}

// TestRunGitBranch_AC4_WrapperAllowed asserts end-to-end (JSON payload → rc=0)
// for shell wrappers that contain only allowed git subcommands.
//
// Reconciliation: asserts that D4 correctly allows bash -c "git status/log/diff"
// (non-blocked inner commands) without false-positive denials.
func TestRunGitBranch_AC4_WrapperAllowed(t *testing.T) {
	dir := makeProjectDir(t)
	cases := []struct {
		name string
		cmd  string
	}{
		{"bash-c-git-status", `bash -c "git status"`},
		{"bash-c-git-log", `bash -c "git log --oneline"`},
		{"bash-c-echo", `bash -c "echo done"`},
		{"bash-c-bash-c-git-status", `bash -c "bash -c 'git status'"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := fmt.Sprintf(`{"tool_name":"Bash","tool_input":{"command":%q}}`, tc.cmd)
			code, _, _ := runWith(t, dir, payload, "")
			if code != 0 {
				t.Errorf("cmd %q: got rc=%d, want rc=0 (allow)", tc.cmd, code)
			}
		})
	}
}

// --- Low-level POSIX function tests ---

// TestQuoteAwareSplitPOSIX_BackslashOutsideQuote_NoSplit asserts that a
// backslash-escaped semicolon outside quotes does NOT split the segment.
//
// Reconciliation: asserts that quoteAwareSplitPOSIX implements POSIX §2.2.1 —
// outside quotes, \; is a literal semicolon (not a command separator).
func TestQuoteAwareSplitPOSIX_BackslashOutsideQuote_NoSplit(t *testing.T) {
	segs := quoteAwareSplitPOSIX(`git push\; echo ok`)
	if len(segs) != 1 {
		t.Fatalf(`git push\; echo ok: expected 1 segment, got %d: %v`, len(segs), segs)
	}
}

// TestQuoteAwareSplitPOSIX_EscapedQuoteNoSplit asserts that \" inside "..."
// does NOT close the outer double-quoted string; the ; that follows stays inside.
//
// Reconciliation: asserts the key POSIX fix — quoteAwareSplitPOSIX emits \"
// verbatim inside "...", so the semicolon after it is not a segment separator.
func TestQuoteAwareSplitPOSIX_EscapedQuoteNoSplit(t *testing.T) {
	// bash -c "bash -c \"git push\"; echo done"
	// The \" keeps the string open; the ; is inside the outer "...", not a separator.
	segs := quoteAwareSplitPOSIX(`bash -c "bash -c \"git push\"; echo done"`)
	if len(segs) != 1 {
		t.Fatalf(`expected 1 segment (\" keeps string open), got %d: %v`, len(segs), segs)
	}
}

// TestShellTokenizePOSIX_BackslashOutsideQuote asserts that \git is tokenised
// as "git" (backslash dropped per POSIX §2.2.1 outside quotes).
//
// Reconciliation: asserts the POSIX unescaping rule — outside quotes, \x → x,
// so \git becomes the token "git" (closing the bypass from Achado 7).
func TestShellTokenizePOSIX_BackslashOutsideQuote(t *testing.T) {
	tokens := shellTokenizePOSIX("\\git push")
	if len(tokens) < 1 || tokens[0] != "git" {
		t.Fatalf(`\\git push: expected first token "git", got %v`, tokens)
	}
}

// TestShellTokenizePOSIX_MidTokenBackslash asserts that g\it is tokenised as
// "git" (the backslash is dropped by POSIX unescaping at the mid-token position).
//
// Reconciliation: asserts POSIX §2.2.1 mid-token — \i → i inside the same word,
// so g\it = g + i + t = "git"; this closes the mid-token bypass from Achado 7.
func TestShellTokenizePOSIX_MidTokenBackslash(t *testing.T) {
	tokens := shellTokenizePOSIX("g\\it push")
	if len(tokens) < 1 || tokens[0] != "git" {
		t.Fatalf(`g\\it push: expected first token "git", got %v`, tokens)
	}
}

// TestShellTokenizePOSIX_EscapedQuoteInsideDoubleQuote asserts that inside
// "...", \" is unescaped to " (not a quote-closer) by shellTokenizePOSIX.
//
// Reconciliation: asserts the POSIX double-quote rule — \" inside "..." yields
// a literal " in the token value, keeping the outer string open.
func TestShellTokenizePOSIX_EscapedQuoteInsideDoubleQuote(t *testing.T) {
	// Input: "say \"hi\"" → token value: say "hi"
	tokens := shellTokenizePOSIX(`"say \"hi\""`)
	if len(tokens) != 1 || tokens[0] != `say "hi"` {
		t.Fatalf(`"say \"hi\"": expected [say "hi"], got %v`, tokens)
	}
}

// TestShellTokenizePOSIX_WindowsPathInsideDoubleQuote asserts that a Windows
// path inside "..." preserves backslashes before non-special chars (\P, \G…).
//
// Reconciliation: asserts POSIX §2.2.3 — inside "...", \P is not a special
// sequence; both \ and P are emitted, preserving the Windows path token intact.
func TestShellTokenizePOSIX_WindowsPathInsideDoubleQuote(t *testing.T) {
	tokens := shellTokenizePOSIX(`"C:\Program Files\Git\bin\git.exe" push`)
	want := `C:\Program Files\Git\bin\git.exe`
	if len(tokens) < 1 || tokens[0] != want {
		t.Fatalf("Windows path: expected first token %q, got %v", want, tokens)
	}
}

// TestStripShellSuffixPOSIX_EscapedQuoteDoesNotCloseString asserts that \"
// inside "..." in stripShellSuffixPOSIX does NOT close the string.
//
// Reconciliation: asserts the POSIX backslash fix for stripShellSuffixPOSIX —
// without this fix, bash -c "x=\" # comment" would be misread such that the \"
// closes the string and # becomes a comment start, truncating the segment.
func TestStripShellSuffixPOSIX_EscapedQuoteDoesNotCloseString(t *testing.T) {
	// Without the fix, \" closes "...", making # a comment start (strips to bash -c ).
	// With the fix, # remains inside the string; nothing is stripped.
	inputWithComment := `bash -c "x=\" # comment"`
	out := stripShellSuffixPOSIX(inputWithComment)
	if strings.Contains(out, "# comment") {
		// The hash is INSIDE the double-quoted region — it must not trigger comment stripping.
		// If it appears in the output at all, it means the string was not prematurely closed.
		// But if it were stripped by the comment rule, it would NOT appear in out.
		// So this branch means: hash somehow still in output but we don't want comment-strip.
		// The fix is: the entire input should be in the output (no stripping at all),
		// because the redirect/comment scanner sees nothing outside quotes.
	}
	// The key assertion: the output is the full input (no premature comment stripping).
	if out != inputWithComment {
		t.Errorf("stripShellSuffixPOSIX: got %q, want %q (# inside string must not be treated as comment)", out, inputWithComment)
	}
}

// --- isNakedAssignment and isDurationArg helpers ---

// TestIsNakedAssignment asserts the helper correctly identifies bare variable
// assignments and rejects non-assignment tokens.
//
// Reconciliation: asserts the precondition for naked-assignment stripping —
// only valid shell identifiers with = are classified as assignments.
func TestIsNakedAssignment(t *testing.T) {
	yes := []string{"FOO=1", "A=", "MY_VAR=value", "_X=y", "X1=z"}
	no := []string{"git", "=FOO", "-u", "FOO", "1FOO=x", "", "foo-bar=x"}
	for _, s := range yes {
		if !isNakedAssignment(s) {
			t.Errorf("isNakedAssignment(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if isNakedAssignment(s) {
			t.Errorf("isNakedAssignment(%q) = true, want false", s)
		}
	}
}

// TestIsDurationArg asserts the helper correctly identifies timeout durations.
//
// Reconciliation: asserts the precondition for timeout prefix stripping —
// the duration positional (e.g. "10", "1.5s", "2m") is consumed before the
// command, so "git push" following a duration is correctly exposed.
func TestIsDurationArg(t *testing.T) {
	yes := []string{"10", "1.5", "10s", "2m", "1h", "0.5s"}
	no := []string{"git", "push", "", "s", "10x", "abc"}
	for _, s := range yes {
		if !isDurationArg(s) {
			t.Errorf("isDurationArg(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if isDurationArg(s) {
			t.Errorf("isDurationArg(%q) = true, want false", s)
		}
	}
}

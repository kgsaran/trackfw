// Package guard — wrapper and POSIX backslash tests for the git-branch guard.
// AC4 and AC5 test cases for ML-1B (REQ-2026-11-05, D4).
//
// Regra Dura de Reconciliação: each new test below carries a one-sentence
// reconciliation that states what conclusion of this ML the test asserts.
package guard

import (
	"fmt"
	"runtime"
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

// TestMatchSubcommand_AC4_DoubleBackslash_PerPlatform asserts the per-platform
// behaviour of `\\git push` (the Go raw string literal contains TWO backslashes
// before "git", so the actual string is \\git push).
//
// On POSIX: `\\git` is POSIX-decoded as `\git` (the first backslash quotes the
// second, yielding a single-backslash literal before "git"); the resulting token
// is `\git`, which is NOT the literal token "git", so the guard allows the
// command and returns "".
//
// On Windows: filepath.Base treats `\` as a path separator (Windows semantics),
// so filepath.Base("\\git") == "git"; the guard sees "git push" and returns
// "push" (fail-closed — a path-qualified git binary must be blocked exactly like
// C:\Program Files\Git\bin\git.exe push).
//
// Reconciliation (Regra Dura de Reconciliação): asserts that `\\git push` is
// platform-specific — allowed on POSIX (double-backslash POSIX-decodes to a
// non-git token `\git`) and blocked on Windows (filepath.Base with Windows path
// separators yields "git", fail-closed), because the guard uses filepath.Base to
// normalise the first token before comparison.
func TestMatchSubcommand_AC4_DoubleBackslash_PerPlatform(t *testing.T) {
	cmd := `\\git push`
	got := MatchSubcommand(cmd)
	if runtime.GOOS == "windows" {
		// Windows: filepath.Base("\\git") == "git"; guard sees "git push" → blocked.
		if got != "push" {
			t.Errorf("Windows: MatchSubcommand(%q) = %q, want %q (fail-closed)", cmd, got, "push")
		}
	} else {
		// POSIX: \\g is POSIX-decoded to \g; token \git ≠ git → allowed.
		if got != "" {
			t.Errorf("POSIX: MatchSubcommand(%q) = %q, want %q (allow)", cmd, got, "")
		}
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
// Reconciliation: asserts that extractRawCmdSubstBodies in matchSubcommandPOSIX
// (ML-2G) scans the raw string before segment splitting, so that $(git push),
// `git push`, and OUT=$(git commit -m x) are all blocked (C7, D4).
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

// --- ML-2F: D-1 (ConfigurationFile), D-2 (env --split-string combined),
// process substitution, and cmd-subst-before-redirect ---

// TestMatchSubcommand_ML2F_Blocked asserts that the four vectors from ML-2F
// that previously gave rc=0 are now detected and blocked (want ≠ "").
//
// Reconciliation (one sentence per test, per the Regra Dura de Reconciliação):
//
//   - proc-subst-lt-git-push: asserts that <(git push) is extracted as a
//     subshell body before stripShellSuffixPOSIX consumes it as a redirect target.
//   - proc-subst-lt-space-git-push: asserts that "cat < <(git push)" (space
//     between < and <() is detected because extractCmdSubstBodies scans for <(
//     anywhere in the pre-strip segment, not only adjacent to a redirect.
//   - proc-subst-gt-git-push: asserts that >(git push) (output process
//     substitution) is also extracted and blocked.
//   - cmd-subst-with-redirect-2redir1: asserts that OUT=$(git commit -m x 2>&1)
//     is blocked after moving extractCmdSubstBodies before stripShellSuffixPOSIX,
//     which previously consumed the closing ')' as part of the "2>&1" target.
//   - cmd-subst-redirect-dev-null: asserts that echo $(git push 2>/dev/null) is
//     blocked; the redirect is inside the $(...) body but does not destroy the
//     outer body extraction because we extract before stripping.
//   - cmd-subst-redirect-out-file: asserts that echo $(git push >out) is blocked
//     for the same reason.
//   - ps-configurationfile-c: asserts that pwsh -ConfigurationFile foo.ps1 -c
//     'git push' is blocked after adding -configurationfile to isPsArgConsumingFlag,
//     which causes foo.ps1 to be consumed as an argument instead of as the command.
//   - ps-co-is-command-not-configfile: asserts that -co still routes to -Command
//     (not -ConfigurationFile) because isPsCommandFlag is checked first.
//   - ps-con-consumes-arg: asserts that -con consumes an argument (via
//     -configurationname prefix) and -c 'git push' is then found as the command.
//   - env-split-string-combined-long: asserts that env --split-string='git push'
//     is blocked after adding the HasPrefix("--split-string=") case to
//     stripEnvCommandPrefix, which re-tokenises the embedded value.
//   - env-split-string-combined-short: asserts that env -S'git push' (combined
//     short form -SVALUE) is blocked by the new lower[:2]=="-s" case.
func TestMatchSubcommand_ML2F_Blocked(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want string
	}{
		// Process substitution — vector 1 (ML-2F, REQ-2026-11-05).
		{"proc-subst-lt-git-push", `cat <(git push)`, "push"},
		{"proc-subst-lt-space-git-push", `cat < <(git push)`, "push"},
		{"proc-subst-gt-git-push", `tee >(git push)`, "push"},
		// Command substitution with redirect — vector 2 (ML-2F, REQ-2026-11-05).
		{"cmd-subst-with-redirect-2redir1", `OUT=$(git commit -m x 2>&1)`, "commit"},
		{"cmd-subst-redirect-dev-null", `echo $(git push 2>/dev/null)`, "push"},
		{"cmd-subst-redirect-out-file", `echo $(git push >out)`, "push"},
		// PowerShell -ConfigurationFile — vector 3 / D-1 (ML-2F, REQ-2026-11-05).
		{"ps-configurationfile-c", `pwsh -ConfigurationFile foo.ps1 -c 'git push'`, "push"},
		{"ps-co-is-command-not-configfile", `pwsh -co 'git push'`, "push"},
		{"ps-con-consumes-arg", `pwsh -con x -c 'git push'`, "push"},
		// env --split-string combined forms — vector 4 / D-2 (ML-2F, REQ-2026-11-05).
		{"env-split-string-combined-long", `env --split-string='git push'`, "push"},
		{"env-split-string-combined-short", `env -S'git push'`, "push"},
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

// TestMatchSubcommand_ML2F_Allowed asserts that the ML-2F fixes do not
// introduce false positives for related but benign commands.
//
// Reconciliation (one sentence per test, per the Regra Dura de Reconciliação):
//
//   - proc-subst-git-status: asserts that <(git status) is NOT blocked because
//     git status is a read-only subcommand not in the blocked set.
//   - proc-subst-echo: asserts that <(echo ok) produces no blocked body, so the
//     outer command is allowed.
//   - proc-subst-single-quoted: asserts that '<(git push)' inside single quotes
//     is literal and does not trigger process-substitution extraction.
//   - proc-subst-double-quoted: asserts that "<(git push)" inside double quotes
//     is NOT treated as process substitution (bash does not expand <(…) in "…").
//   - cmd-subst-redirect-git-status: asserts that $(git status 2>/dev/null)
//     inside a command substitution with redirect is allowed when the subcommand
//     is not blocked.
//   - ps-of-is-output-format: asserts that pwsh -of Text -c 'git push' is
//     blocked (not a false positive) — -of is now recognised as an arg-consuming
//     alias for -OutputFormat, so Text is consumed and -c finds the command.
//   - env-split-string-separate-status: asserts that env --split-string
//     'git status' is allowed because git status is a read-only subcommand.
func TestMatchSubcommand_ML2F_Allowed(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		// Process substitution false-positive checks.
		{"proc-subst-git-status", `diff <(git status) x`},
		{"proc-subst-echo", `cat <(echo ok)`},
		{"proc-subst-single-quoted", `echo '<(git push)'`},
		{"proc-subst-double-quoted", `echo "<(git push)"`},
		// Command substitution with redirect — only allowed subcommands.
		{"cmd-subst-redirect-git-status", `echo $(git status 2>/dev/null)`},
		// env split-string with allowed subcommand.
		{"env-split-string-separate-status", `env --split-string 'git status'`},
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

// TestMatchSubcommand_ML2F_PsOfBlocked asserts that the -of alias fix for
// -OutputFormat does not accidentally allow pwsh -of Text -c 'git push'.
// This is a block test for a secondary fix within vector 3.
//
// Reconciliation: asserts that -of is now in isPsArgConsumingFlag, so "Text"
// is consumed as the OutputFormat argument and -c 'git push' is found as the
// command string — the same mechanism as -ep/-wd/-if.
func TestMatchSubcommand_ML2F_PsOfBlocked(t *testing.T) {
	got := MatchSubcommand(`pwsh -of Text -c 'git push'`)
	if got != "push" {
		t.Errorf(`MatchSubcommand("pwsh -of Text -c 'git push'") = %q, want "push"`, got)
	}
}

// --- ML-2G: raw-string substitution extraction ---

// TestMatchSubcommand_ML2G_Blocked asserts that substitution bodies containing
// shell operators ('|', '&&') are correctly extracted from the raw string and
// analysed, blocking the git subcommand inside. Also covers backslash-before-quote
// escaping and the full range of --split-string abbreviations.
//
// Reconciliation (one sentence per test, per the Regra Dura de Reconciliação):
//
//   - cmd-subst-pipe: asserts that $(git push | cat) is extracted from the raw
//     string before quoteAwareSplitPOSIX splits on '|', so the body
//     "git push | cat" is analysed in full and "push" is blocked.
//   - cmd-subst-and-and: asserts that OUT=$(git push && echo x) is extracted
//     before the '&&' separator splits the string, exposing "git push" in the body.
//   - proc-subst-pipe: asserts that <(git push | cat) is extracted from the raw
//     string as a process-substitution body before the '|' splits the segment.
//   - bash-c-dollar-pipe: asserts that bash -c 'echo $(git push | cat)' is
//     handled via two levels: the shell wrapper extracts 'echo $(git push | cat)',
//     and extractRawCmdSubstBodies then extracts the body "git push | cat".
//   - env-split-abbrev-eq: asserts that env --split='git push' is handled by the
//     new abbreviated combined form case in stripEnvCommandPrefix (--split is an
//     unambiguous prefix of --split-string accepted by GNU getopt_long).
//   - env-split-abbrev-sep: asserts that env --sp 'git push' is handled by the
//     new abbreviated separate form case in stripEnvCommandPrefix (--sp is an
//     unambiguous prefix of --split-string).
//   - env-s-abbrev-eq: asserts that env --s='git push' is handled by the
//     abbreviated combined form (--s is the shortest unambiguous prefix, length 3).
//   - env-s-abbrev-sep: asserts that env --s 'git push' is handled by the
//     abbreviated separate form (--s length ≥ 3 threshold).
//   - dquote-dollar-pipe: asserts that echo "$(git push | cat)" is blocked
//     because extractRawCmdSubstBodies extracts $(…) bodies inside "…".
//   - two-levels-nested: asserts that x=$(echo $(git push | cat)) is blocked at
//     depth 2 by recursing from the outer body into the inner body.
//   - three-levels-wrapper-not-analyzed: asserts that echo $(echo $(echo
//     $(git push))) reaches depth 3 (> maxWrapperDepth) and returns
//     "wrapper-not-analyzed" (fail-closed), not an empty allow.
//   - bs-sq-dollar: asserts that echo \'$(git push)\' is blocked because the
//     backslash handler in extractRawCmdSubstBodies consumes \' as a literal
//     character instead of opening a single-quoted region, allowing the $(…) body
//     to be extracted.
//   - bs-dq-proc: asserts that cat \" <(git push) is blocked because the
//     backslash handler consumes \" as a literal character instead of opening a
//     double-quoted region, allowing <(…) to be extracted as a process substitution.
func TestMatchSubcommand_ML2G_Blocked(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want string
	}{
		{"cmd-subst-pipe", `echo $(git push | cat)`, "push"},
		{"cmd-subst-and-and", `OUT=$(git push && echo x)`, "push"},
		{"proc-subst-pipe", `cat <(git push | cat)`, "push"},
		{"bash-c-dollar-pipe", `bash -c 'echo $(git push | cat)'`, "push"},
		{"env-split-abbrev-eq", `env --split='git push'`, "push"},
		{"env-split-abbrev-sep", `env --sp 'git push'`, "push"},
		{"env-s-abbrev-eq", `env --s='git push'`, "push"},
		{"env-s-abbrev-sep", `env --s 'git push'`, "push"},
		{"dquote-dollar-pipe", `echo "$(git push | cat)"`, "push"},
		{"two-levels-nested", `x=$(echo $(git push | cat))`, "push"},
		{"three-levels-wrapper-not-analyzed", `echo $(echo $(echo $(git push)))`, "wrapper-not-analyzed"},
		{"bs-sq-dollar", `echo \'$(git push)\'`, "push"},
		{"bs-dq-proc", `cat \" <(git push)`, "push"},
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

// TestMatchSubcommand_ML2G_Allowed asserts that the ML-2G raw-string extraction
// does not introduce false positives for related but benign commands.
//
// Reconciliation (one sentence per test, per the Regra Dura de Reconciliação):
//
//   - dollar-status-pipe: asserts that echo $(git status | head) is NOT blocked
//     because git status is a read-only subcommand that is not in the blocked set.
//   - single-quoted-dollar-pipe: asserts that echo '$(git push | cat)' (body
//     inside single quotes) is NOT extracted, so no blocking occurs.
//   - x-ls-and-pwd: asserts that x=$(ls && pwd) is NOT blocked because neither
//     ls nor pwd is git.
//   - comment-dollar-push: asserts that echo ok # $(git push) is NOT blocked
//     because the '#' at word boundary starts a comment and the $() inside the
//     comment is not extracted by extractRawCmdSubstBodies.
//   - diff-proc-subst-status: asserts that diff <(git status) x is NOT blocked
//     because git status is not a blocked subcommand.
//   - dquote-proc-subst: asserts that echo "<(git push)" is NOT blocked because
//     process substitution <(…) does not expand inside "…", so extractRawCmdSubstBodies
//     does not extract it.
func TestMatchSubcommand_ML2G_Allowed(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"dollar-status-pipe", `echo $(git status | head)`},
		{"single-quoted-dollar-pipe", `echo '$(git push | cat)'`},
		{"x-ls-and-pwd", `x=$(ls && pwd)`},
		{"comment-dollar-push", `echo ok # $(git push)`},
		{"diff-proc-subst-status", `diff <(git status) x`},
		{"dquote-proc-subst", `echo "<(git push)"`},
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

// --- ML-2I: case…esac inside $() / <() ---

// TestMatchSubcommand_ML2I_Blocked asserts that case…esac constructs inside
// command- and process-substitution bodies do not let a blocked git subcommand
// escape detection. It also covers the SQ-preservation fix that exposed a
// bypass via $(bash -c 'git push').
//
// Reconciliation (one sentence per test, per the Regra Dura de Reconciliação):
//
//   - case-plain-push: asserts that $(case x in x) git push ;; esac) is blocked
//     because extractBalancedParenBody now treats the unparenthesized pattern ')'
//     as a terminator (not a depth-decrement), extracting the full body for
//     re-analysis which detects "git push".
//   - case-wildcard-push: asserts that $(case x in *) git push ;; esac) is
//     blocked; the wildcard pattern '*' is accumulated as a plain word and its
//     closing ')' is correctly intercepted as a pattern terminator.
//   - bash-c-case-push: asserts that bash -c '$(case x in x) git push ;; esac)'
//     is blocked by composing two fixes: the shell-wrapper extracts the -c
//     argument, and extractBalancedParenBody then correctly extracts the case body.
//   - assign-case-commit: asserts that x=$(case a in a) git commit -m y;; esac)
//     is blocked; the ';;' (without surrounding spaces) is consumed and the
//     commit subcommand in the case body is detected.
//   - proc-subst-case-push: asserts that cat <(case x in x) git push ;; esac)
//     is blocked; the same case-aware extraction applies to process-substitution
//     bodies (<(...)) as it does to command substitutions ($(...)).
//   - nested-case-push: asserts that $(case x in x) case y in y) git push ;; esac ;; esac)
//     is blocked; two nested case frames are pushed and popped independently,
//     with each bare ')' correctly intercepted at the right frame depth.
//   - second-pattern-push: asserts that $(case x in a) ;; *) git push ;; esac)
//     is blocked; after the first ';;' transitions back to pattern-position, the
//     second pattern '*' and its terminating ')' are correctly handled before
//     "git push" is exposed in the case body.
//   - dq-paren-push: asserts that echo $(echo ")"; git push) is blocked; the
//     new double-quote tracking in extractBalancedParenBody prevents the ')' inside
//     "…" from decrementing depth prematurely so the full body is extracted.
//   - bs-paren-push: asserts that echo $(echo \); git push) is blocked; the
//     new backslash handling prevents '\)' from decrementing depth so the full
//     body containing "git push" is extracted.
//   - sq-bypass-bash-c: asserts that echo $(bash -c 'git push') is blocked;
//     previously the quote-stripping in extractBalancedParenBody produced the
//     body "bash -c git push" (where the -c argument was just "git", not
//     "git push"), silently bypassing the shell-wrapper check. Preserving
//     single quotes in the body now yields "bash -c 'git push'" which
//     handleShellWrapper correctly analyses as the blocked subcommand "push".
func TestMatchSubcommand_ML2I_Blocked(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
		want string
	}{
		{"case-plain-push", `echo $(case x in x) git push ;; esac)`, "push"},
		{"case-wildcard-push", `$(case x in *) git push ;; esac)`, "push"},
		{"bash-c-case-push", `bash -c '$(case x in x) git push ;; esac)'`, "push"},
		{"assign-case-commit", `x=$(case a in a) git commit -m y;; esac)`, "commit"},
		{"proc-subst-case-push", `cat <(case x in x) git push ;; esac)`, "push"},
		{"nested-case-push", `echo $(case x in x) case y in y) git push ;; esac ;; esac)`, "push"},
		{"second-pattern-push", `echo $(case x in a) ;; *) git push ;; esac)`, "push"},
		{"dq-paren-push", `echo $(echo ")"; git push)`, "push"},
		{"bs-paren-push", `echo $(echo \); git push)`, "push"},
		{"sq-bypass-bash-c", `echo $(bash -c 'git push')`, "push"},
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

// TestMatchSubcommand_ML2I_Allowed asserts that the ML-2I case-awareness and
// SQ-preservation fixes do not introduce false positives for related but
// benign commands.
//
// Reconciliation (one sentence per test, per the Regra Dura de Reconciliação):
//
//   - case-echo-ok: asserts that echo $(case x in x) echo ok ;; esac) is NOT
//     blocked because "echo ok" in the case body contains no blocked git
//     subcommand; the case-aware extraction returns the full body which is
//     correctly analysed.
//   - sq-outer-literal: asserts that echo '$(case x in x) git push ;; esac)' is
//     NOT blocked because the entire string is single-quoted at the outer level,
//     so extractRawCmdSubstBodies never extracts any substitution body.
//   - case-git-status: asserts that echo $(case $v in a|b) git status ;; esac) is
//     NOT blocked because "git status" is a read-only subcommand not in the
//     blocked set; the alternation pattern 'a|b' is handled correctly with '|'
//     as an intra-pattern separator and ')' as the pattern terminator.
//   - case-assign-ls: asserts that x=$(case a in a) ls;; esac) is NOT blocked
//     because "ls" is not git; this also confirms that ';;' without surrounding
//     spaces is consumed as a clause terminator.
//   - git-describe: asserts that VER=$(git describe --tags) is NOT blocked because
//     "describe" is a read-only subcommand not in the blocked set, and the
//     standard $() body extraction is unaffected by the case-awareness change.
//   - sq-echo-inner: asserts that echo $(echo '$(git push)') is NOT blocked;
//     previously this was incorrectly blocked because quote-stripping exposed the
//     inner '$(git push)' to extraction — with SQ preserved in the body, the inner
//     $() is single-quoted and skipped by extractRawCmdSubstBodies (correct per
//     POSIX: single-quoted $() is literal and never evaluated).
func TestMatchSubcommand_ML2I_Allowed(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"case-echo-ok", `echo $(case x in x) echo ok ;; esac)`},
		{"sq-outer-literal", `echo '$(case x in x) git push ;; esac)'`},
		{"case-git-status", `echo $(case $v in a|b) git status ;; esac)`},
		{"case-assign-ls", `x=$(case a in a) ls;; esac)`},
		{"git-describe", `VER=$(git describe --tags)`},
		{"sq-echo-inner", `echo $(echo '$(git push)')`},
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

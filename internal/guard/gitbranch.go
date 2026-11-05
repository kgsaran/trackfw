package guard

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Reason messages — byte-for-byte identical to the bash REASON variables in
// scripts/trackfw-git-branch-guard.sh (lines 708–748).
const (
	reasonTruncated = "trackfw git-branch-guard: read timeout — the command could not be fully read from stdin (no byte arrived within the 2s idle window) and no --command flag was provided. The guard denies rather than approve a command it could not read. Resend the command, or use `trackfw ship` / `trackfw branch new`."

	reasonNUL = "trackfw git-branch-guard: NUL byte detected — the command contains NUL (\\u0000) and cannot be safely interpreted by the shell. The guard denies rather than execute a corrupted command."

	reasonIndecodeable = "trackfw git-branch-guard: JSON decode error — the JSON extractor (without jq) failed to decode the command field: unterminated string, invalid escape or NUL. The guard denies rather than approve a payload it cannot safely read."

	reasonCheckoutB = "trackfw: raw git checkout -b blocked. Use `trackfw branch new <type>/<slug>`. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonSwitchC = "trackfw: raw git switch -c blocked. Use `trackfw branch new <type>/<slug>`. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonBranchCreate = "trackfw: raw git branch blocked. Use `trackfw branch new <type>/<slug>`. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonWorktreeAddB = "trackfw: raw git worktree add -b blocked. Use `trackfw branch new <type>/<slug>`. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonCommit = "trackfw: raw git commit blocked. Use `trackfw commit -m '<message>'`. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonPush = "trackfw: raw git push blocked. Use `trackfw push` (push existing commits), `trackfw ship` (commit + push + PR in one step) or `trackfw release tag` (publish a release tag). Nothing before this command was executed (a compound command is blocked as a whole)."

	// reasonPushDelete is used when the push command contains a branch-deletion
	// form (--delete/-d <ref> or :<ref>/+:<ref> where <ref> is not a tag).
	// It points to the governed path that deletes remote branches safely.
	reasonPushDelete = "trackfw: raw git push branch deletion blocked. Use `trackfw branch prune --remote --apply` to delete from origin only the branch integrated by merged PR, conditional on the evaluated tip. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonStash = "trackfw: raw git stash blocked — shared worktree across subagents: a stash removes uncommitted changes for everyone. `git stash list`/`git stash show` are still allowed; to save work in progress, use a dedicated branch via `trackfw branch new` and commit there. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonResetHard = "trackfw: raw git reset --hard blocked — irreversibly discards all uncommitted changes in the shared worktree. `git reset --soft`/`--mixed` are still allowed (e.g. `git reset --soft HEAD~1` is the standard path; use `trackfw ship -m '...'` to commit and push). Nothing before this command was executed (a compound command is blocked as a whole)."

	// reasonAddImplicit is emitted when `git add` or `git stage` is invoked with
	// implicit scope (no named path, or only whole-tree roots such as . / .. / :/
	// / :(top)). ADR-2026-09-05 D1, D3: staging without enumeration is blocked;
	// the message must teach the valid form. Single-line, JSON-safe (no raw
	// double-quote or backslash). R-6 (Wave 0, 2026-10-17).
	reasonAddImplicit = "trackfw: git add/stage with implicit scope blocked — first enumerate what to stage with `git status --short`, then stage by explicit path: `git add <file1> <file2> ...` and commit with `trackfw commit -m '...'`. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonCleanForce = "trackfw: raw git clean -f/-x blocked — irreversibly deletes untracked files from the shared worktree. `git clean -n`/`--dry-run` is still allowed to review what would be deleted first. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonRestorePath = "trackfw: raw git restore <path> blocked — irreversibly discards uncommitted changes at the given path. `git restore --staged` (does not touch the working tree) is still allowed; to actually discard, confirm with the user first. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonCheckoutPath = "trackfw: raw git checkout -- <path>/git checkout . blocked — irreversibly discards uncommitted changes at the given path. `git checkout <branch>`/`git switch <branch>` are still allowed; to actually discard, confirm with the user first. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonUpdateRef = "trackfw: raw git update-ref blocked — rewrites a ref (including refs/remotes/origin/*) without touching the pointed object or requiring a push, which allows forging the commit target that `trackfw release tag` would publish. Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonWorktreeRemoveForce = "trackfw: raw git worktree remove -f/--force blocked — removes a worktree and irreversibly discards any uncommitted changes in it. `git worktree remove` without force is still allowed (it refuses on its own when there are uncommitted changes). Nothing before this command was executed (a compound command is blocked as a whole)."

	reasonRmForce = "trackfw: raw git rm -f/--force blocked — irreversibly removes files from the working tree and index, same class as `git clean -f`/`git reset --hard` already blocked. Nothing before this command was executed (a compound command is blocked as a whole)."

	// reasonWorktreeAddImplicit is emitted when `git worktree add <path>` is invoked
	// without any branch-selection flag (-b/-B/--detach/-d/--orphan) and without a
	// commit-ish as a second positional argument. In that form git creates an implicit
	// branch whose name is derived from <path>, bypassing the governance gate that
	// `trackfw branch new` enforces. Blocked by ML-1C (REQ-2026-09-12, ROADMAP-2026-09-12).
	reasonWorktreeAddImplicit = "trackfw: git worktree add <path> without branch flag blocked — this form creates an implicit branch derived from the path name, bypassing governance. Use `git worktree add <path> <existing-branch>` (for an already created branch) or `trackfw branch new <type>/<slug>` + `git worktree add <path> <new-branch>`. Nothing before this command was executed (a compound command is blocked as a whole)."

	// reasonWrapperNotAnalyzed is emitted when a shell-wrapper invocation exceeds
	// the maximum analysis depth (2) or when the command string cannot be extracted
	// from the wrapper arguments. The guard denies fail-closed rather than approve
	// a command it cannot read. Single-line, JSON-safe (no raw double-quote or
	// backslash). D4, Wave 1 ML-1B (REQ-2026-11-05).
	reasonWrapperNotAnalyzed = "trackfw git-branch-guard: shell wrapper not analyzed — the command contains a nested shell invocation whose depth exceeds the analysis limit or whose command string cannot be extracted. The guard denies rather than approve a command it cannot fully read."
)

// StdInIdleTimeout is the idle window used by DrainStdin in production.
// Exported so the cobra command in internal/commands can pass it to RunGitBranch.
const StdInIdleTimeout = 2 * time.Second

// maxWrapperDepth is the maximum nesting depth the POSIX analysis path will
// recurse into. Two guards enforce this boundary:
//
//   - handleShellWrapper uses depth >= maxWrapperDepth (teto efetivo): called
//     before any recursive descent, so it fires when we're already at the limit.
//
//   - matchSubcommandPOSIX uses depth > maxWrapperDepth (belt-and-suspenders):
//     guards against future callers that might invoke matchSubcommandPOSIX
//     directly with depth >= maxWrapperDepth. Logically unreachable today, but
//     present so an accidental direct call doesn't bypass the ceiling.
//
// D4, Wave 2 (REQ-2026-11-05, ML-2C).
const maxWrapperDepth = 2

// RunGitBranch implements the full git-branch guard logic. It is the entry
// point called by the cobra RunE for `trackfw guard git-branch`.
//
// Parameters:
//   - stdin: the reader to drain for the payload (os.Stdin in production)
//   - idleTimeout: idle window for DrainStdin; pass stdInIdleTimeout in production
//   - command: value of the --command flag; empty means "read from stdin"
//   - getEnv: function to read env variables (os.Getenv in production)
//   - stdout, stderr: output writers
//
// Returns 0 to allow, 2 to deny. The caller MUST exit with this code.
func RunGitBranch(stdin io.Reader, idleTimeout time.Duration, command string, getEnv func(string) string, stdout, stderr io.Writer) int {
	// --- 0. Drain stdin unconditionally (EPIPE prevention, ADR comment ML-1B).
	// Even when --command is provided we drain stdin so the caller doesn't get EPIPE.
	stdinData, truncated := DrainStdin(stdin, idleTimeout)

	// --- 0b. Walk up to find trackfw.yaml (no-op outside project).
	cwd, err := os.Getwd()
	if err != nil {
		return 0 // cannot determine cwd: no-op
	}
	cwd, _ = filepath.EvalSymlinks(cwd) // pwd -P equivalent
	if cwd == "" {
		// EvalSymlinks returned empty (shouldn't happen); fall back.
		cwd, _ = os.Getwd()
	}
	_, found := FindProjectRoot(cwd)
	if !found {
		return 0 // not inside a trackfw project
	}

	// --- 0c. Fail-closed on truncation if no --command provided.
	if truncated && command == "" {
		return denyOut(stdout, stderr, reasonTruncated)
	}

	// --- 1. Determine CMD_RAW.
	var cmdRaw string
	if command != "" {
		// D8: --command flag overrides stdin (the .sh uses CMD_RAW="$*").
		cmdRaw = command
	} else {
		// Parse stdin payload.
		extracted, err := ExtractCommand(stdinData)
		if err != nil {
			reason := reasonIndecodeable
			if err == ErrNULInCommand {
				reason = reasonNUL
			}
			return denyOut(stdout, stderr, reason)
		}
		cmdRaw = extracted
	}

	// Fallback to TRACKFW_GIT_COMMAND env var if still empty.
	if cmdRaw == "" {
		cmdRaw = getEnv("TRACKFW_GIT_COMMAND")
	}

	// Nothing to guard.
	if cmdRaw == "" {
		return 0
	}

	// --- 2+3. Match subcommand and apply rules.
	subcommand := MatchSubcommand(cmdRaw)
	if subcommand == "" {
		return 0 // command is allowed
	}

	var reason string
	switch subcommand {
	case "checkout-b":
		reason = reasonCheckoutB
	case "switch-c":
		reason = reasonSwitchC
	case "branch-create":
		reason = reasonBranchCreate
	case "worktree-add-b":
		reason = reasonWorktreeAddB
	case "commit":
		reason = reasonCommit
	case "push":
		reason = reasonPush
	case "push-delete":
		reason = reasonPushDelete
	case "stash":
		reason = reasonStash
	case "reset-hard":
		reason = reasonResetHard
	case "clean-force":
		reason = reasonCleanForce
	case "restore-path":
		reason = reasonRestorePath
	case "checkout-path":
		reason = reasonCheckoutPath
	case "update-ref":
		reason = reasonUpdateRef
	case "worktree-remove-force":
		reason = reasonWorktreeRemoveForce
	case "rm-force":
		reason = reasonRmForce
	case "add-implicit":
		reason = reasonAddImplicit
	case "worktree-add-implicit":
		reason = reasonWorktreeAddImplicit
	case "wrapper-not-analyzed":
		reason = reasonWrapperNotAnalyzed
	default:
		return 0
	}

	return denyOut(stdout, stderr, reason)
}

// denyOut writes the hookSpecificOutput JSON to stdout and the reason to
// stderr, then returns 2. Format is byte-for-byte identical to the .sh:
//
//	printf '{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}\n'
func denyOut(stdout, stderr io.Writer, reason string) int {
	fmt.Fprintf(stdout, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"%s"}}`+"\n", reason)
	fmt.Fprintln(stderr, reason)
	return 2
}

// FindProjectRoot walks up from cwd until it finds trackfw.yaml or reaches /.
// Mirrors the bash loop in .sh lines 108–122 (pwd -P + walk).
// Returns the project root and true if found; ("", false) otherwise.
func FindProjectRoot(cwd string) (string, bool) {
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "trackfw.yaml")); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root.
			return "", false
		}
		dir = parent
	}
}

// stripShellSuffix removes trailing shell syntax from a segment string that
// should not affect the guard's decision: I/O redirections (with their
// targets) and # comments. It is quote-aware — content inside single or
// double quotes is never treated as an operator.
//
// Redirects removed: >, >>, <, <<, <>, >&, >|, &>, &>>, and their optional
// leading fd digit (e.g. 2>, 2>&1, 1>/dev/null). The token immediately
// following a bare operator (one with no inline target) is also consumed.
// Comments: '#' at word boundary (after whitespace or at segment start)
// terminates the segment.
//
// This enforces the invariant "C > x" and "C # y" produce the same guard
// decision as "C" — regardless of whether C is wrapped in subshell parens.
// B-6 fix extended (red-team 2026-10-17 round 3).
func stripShellSuffix(seg string) string {
	runes := []rune(seg)
	n := len(runes)
	var out strings.Builder
	var q rune // current quote char (0 = unquoted)
	atWordBoundary := true

	for i := 0; i < n; i++ {
		c := runes[i]

		if q != 0 {
			// Inside a quoted region — nothing is an operator.
			if c == q {
				q = 0
			}
			out.WriteRune(c)
			atWordBoundary = false
			continue
		}

		// Outside quotes.
		switch {
		case c == '\'' || c == '"':
			q = c
			out.WriteRune(c)
			atWordBoundary = false

		case c == '#' && atWordBoundary:
			// Comment start: discard the rest of the segment.
			goto done

		case c == '>' || c == '<':
			// Redirect operator. Strip optional preceding fd digit from output.
			s := out.String()
			if len(s) > 0 {
				last := rune(s[len(s)-1])
				if last >= '0' && last <= '9' {
					// fd digit: remove it if preceded by nothing or whitespace.
					prevIsSpace := len(s) < 2 || s[len(s)-2] == ' ' || s[len(s)-2] == '\t'
					if prevIsSpace {
						out.Reset()
						out.WriteString(s[:len(s)-1])
					}
				}
			}
			// Consume the full operator: >, >>, >&1, >|, <, <<, etc.
			for i+1 < n && (runes[i+1] == '>' || runes[i+1] == '<' || runes[i+1] == '&' || runes[i+1] == '|') {
				i++
			}
			// If the operator ends here (no inline target), skip the next word too.
			inlineTarget := i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t'
			if !inlineTarget {
				// Skip whitespace before target.
				for i+1 < n && (runes[i+1] == ' ' || runes[i+1] == '\t') {
					i++
				}
				// Skip target word.
				for i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t' &&
					runes[i+1] != ';' && runes[i+1] != '&' && runes[i+1] != '|' {
					i++
				}
			} else {
				// Operator has inline target (e.g. >/dev/null): skip it.
				for i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t' &&
					runes[i+1] != ';' && runes[i+1] != '&' && runes[i+1] != '|' {
					i++
				}
			}
			atWordBoundary = true

		case c == '&' && i+1 < n && (runes[i+1] == '>' || runes[i+1] == '<'):
			// &> / &>> / &< redirect (bash shorthand: combine stdout+stderr).
			i++ // consume '>'
			// Consume further operator chars (e.g. &>>).
			for i+1 < n && (runes[i+1] == '>' || runes[i+1] == '<') {
				i++
			}
			// Skip whitespace and target (same logic as above).
			inlineTarget := i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t'
			if !inlineTarget {
				for i+1 < n && (runes[i+1] == ' ' || runes[i+1] == '\t') {
					i++
				}
				for i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t' &&
					runes[i+1] != ';' && runes[i+1] != '&' && runes[i+1] != '|' {
					i++
				}
			} else {
				for i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t' &&
					runes[i+1] != ';' && runes[i+1] != '&' && runes[i+1] != '|' {
					i++
				}
			}
			atWordBoundary = true

		case c == ' ' || c == '\t':
			out.WriteRune(c)
			atWordBoundary = true

		default:
			out.WriteRune(c)
			atWordBoundary = false
		}
	}

done:
	return out.String()
}

// MatchSubcommand applies the guard rules to cmdRaw and returns the subcommand
// tag if the command should be blocked, or "" to allow.
// It is a faithful Go port of the bash match_subcommand() function.
func MatchSubcommand(cmdRaw string) string {
	normalized := stripHeredocBodies(cmdRaw)
	segments := quoteAwareSplit(normalized)

	for _, seg := range segments {
		seg = strings.TrimLeft(seg, " \t")
		// Strip a single leading '&' character: left by the bash/PS |& pipe
		// operator after '|' splits the segment, or by the PS call operator
		// when '&' is adjacent to a redirect and was not consumed as a separator.
		// Strip characters, not tokens, so "|&git push" (no space) is also caught.
		if len(seg) > 0 && seg[0] == '&' {
			seg = strings.TrimLeft(seg[1:], " \t")
		}
		// Strip I/O redirections and trailing comments.
		// Invariant: "C > x", "C 2>&1", "C # comment" all decide the same as "C".
		// B-6 fix (round 3): subshell handling is now in quoteAwareSplit (word-start
		// '(' is a separator), so no per-segment paren strip is needed here.
		seg = stripShellSuffix(seg)
		if seg == "" {
			continue
		}
		tokens := shellTokenize(seg)
		if len(tokens) == 0 {
			continue
		}

		// Strip env/command prefix (mirrors bash while loop lines 430–452).
		tokens = stripEnvCommandPrefix(tokens)
		if len(tokens) == 0 {
			continue
		}

		// First token must be "git" (by basename, so /usr/bin/git also works).
		// Case-insensitive and strip .exe so git.exe / GIT.EXE (Windows idiom)
		// are treated identically to "git" (C2 fix, ADR-2026-10-04).
		base := filepath.Base(tokens[0])
		base = strings.ToLower(base)
		base = strings.TrimSuffix(base, ".exe")
		if base != "git" {
			continue
		}
		tokens = tokens[1:]

		// Find subcommand (skip -C/-c/--work-tree/--git-dir/--namespace flags).
		sub, rest := findSubcommand(tokens)
		if sub == "" {
			continue
		}

		if tag := applyRule(sub, rest); tag != "" {
			return tag
		}
	}

	// D4 — POSIX-aware second pass: shell wrappers, POSIX backslash escape,
	// and extended prefixes (nohup, exec, naked assignments, etc.).
	// The result does not vary by OS: a command blocked on any platform is
	// blocked everywhere (the POSIX path adds coverage; it never removes it).
	// Note: stripEnvCommandPrefix is shared between both passes, so the second
	// pass is not fully independent of the first. D4, Wave 1 ML-1B (REQ-2026-11-05).
	if tag := matchSubcommandPOSIX(cmdRaw, 0); tag != "" {
		return tag
	}
	return ""
}

// isNakedAssignment reports whether s is a bare shell variable assignment of
// the form NAME=value (or NAME=), where NAME is a valid shell identifier.
// Used to strip FOO=1 git push into git push (D4, ML-1B).
func isNakedAssignment(s string) bool {
	idx := strings.IndexByte(s, '=')
	if idx <= 0 {
		return false
	}
	name := s[:idx]
	if len(name) == 0 {
		return false
	}
	c0 := name[0]
	if c0 != '_' && !(c0 >= 'A' && c0 <= 'Z') && !(c0 >= 'a' && c0 <= 'z') {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if c != '_' && !(c >= 'A' && c <= 'Z') && !(c >= 'a' && c <= 'z') && !(c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

// consumeNakedAssignments strips leading bare variable assignments (FOO=1).
func consumeNakedAssignments(tokens []string) []string {
	for len(tokens) > 0 && isNakedAssignment(tokens[0]) {
		tokens = tokens[1:]
	}
	return tokens
}

// isDurationArg reports whether s looks like a timeout(1) duration argument
// (e.g. "10", "1.5s", "2m", "1h"). Used to consume the duration positional
// that timeout requires before the actual command.
func isDurationArg(s string) bool {
	if len(s) == 0 {
		return false
	}
	end := len(s)
	switch s[end-1] {
	case 's', 'm', 'h', 'd':
		end--
	}
	if end == 0 {
		return false
	}
	for _, c := range s[:end] {
		if c != '.' && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

// hasCFlag reports whether tok is a combined short flag string that includes
// 'c' (e.g. -lc, -ec, -ic). Returns false for -c alone (handled separately)
// and for long flags (--something).
func hasCFlag(tok string) bool {
	if len(tok) < 3 || tok[0] != '-' || tok[1] == '-' {
		return false
	}
	return strings.ContainsRune(tok[1:], 'c')
}

// stripEnvCommandPrefix removes a leading chain of env/command/sudo/nohup/time/
// exec/nice/doas/timeout/stdbuf prefixes and bare variable assignments from
// tokens. Extended by D4, Wave 1 ML-1B (REQ-2026-11-05) to handle the full set
// of POSIX wrappers that bypass the guard by wrapping the git command.
//
// The denylist of recognised prefixes is open: future prefixes with identical
// semantics (a passthrough wrapper that runs the next command unchanged) can be
// added here without changing the caller or the POSIX analysis path.
func stripEnvCommandPrefix(tokens []string) []string {
	// Strip leading bare variable assignments first (FOO=1 git push).
	tokens = consumeNakedAssignments(tokens)

	for len(tokens) > 0 {
		base := strings.TrimSuffix(strings.ToLower(filepath.Base(tokens[0])), ".exe")
		consumed := true

		switch base {
		case "env":
			tokens = tokens[1:] // skip "env"
			// Skip flags and KEY=value pairs.
		envLoop:
			for len(tokens) > 0 {
				t := tokens[0]
				if t == "--" {
					tokens = tokens[1:] // end-of-flags marker
					break
				}
				if strings.HasPrefix(t, "-") {
					tokens = tokens[1:]
					lower := strings.ToLower(t)
					switch {
					// -S/--split-string: treat argument as a command line and re-tokenise.
					case lower == "-s" || lower == "--split-string":
						if len(tokens) > 0 {
							split := shellTokenizePOSIX(tokens[0])
							tokens = append(split, tokens[1:]...)
						}
						break envLoop
					// Combined long form: --split-string=VALUE — ML-2F (REQ-2026-11-05).
					case strings.HasPrefix(lower, "--split-string="):
						split := shellTokenizePOSIX(t[len("--split-string="):])
						tokens = append(split, tokens...)
						break envLoop
					// Combined short form: -SVALUE (value glued immediately after -S) — ML-2F.
					case len(lower) > 2 && lower[:2] == "-s":
						split := shellTokenizePOSIX(t[2:])
						tokens = append(split, tokens...)
						break envLoop
					// Abbreviated combined long form: --s=VALUE, --sp=VALUE, --split=VALUE, etc.
					// GNU getopt_long accepts any unambiguous prefix of a long option;
					// --split-string is the only env long option starting with --s, so any
					// prefix of length >= 3 that includes '=' is an unambiguous abbreviation.
					// (Minimum: '--s'; shorter '--' is not a valid long-option prefix.)
					// ML-2G (REQ-2026-11-05).
					case func() bool {
						eq := strings.IndexByte(lower, '=')
						if eq < 0 {
							return false
						}
						opt := lower[:eq]
						return len(opt) >= 3 && strings.HasPrefix("--split-string", opt) && opt != "--split-string"
					}():
						eq := strings.IndexByte(lower, '=')
						split := shellTokenizePOSIX(t[eq+1:])
						tokens = append(split, tokens...)
						break envLoop
					// Abbreviated separate long form: --s, --sp, --spl, --split, --split-, etc.
					// (prefix of --split-string without inline =, not the full name handled above).
					// Minimum length 3 ('--s'); '--' alone is never a valid option token.
					// ML-2G (REQ-2026-11-05).
					case len(lower) >= 3 && strings.HasPrefix("--split-string", lower) && lower != "--split-string":
						if len(tokens) > 0 {
							split := shellTokenizePOSIX(tokens[0])
							tokens = append(split, tokens[1:]...)
						}
						break envLoop
					// Flags that consume one positional argument.
					case lower == "-u" || lower == "--unset",
						lower == "-p" || lower == "--path",
						lower == "-c" || lower == "--chdir":
						// --unset=VAR is the combined form; skip only if separate form.
						if len(tokens) > 0 && !strings.Contains(lower, "=") {
							tokens = tokens[1:]
						}
					// --unset=VAR: combined form, no next token consumed.
					case strings.HasPrefix(lower, "--unset="),
						strings.HasPrefix(lower, "--path="),
						strings.HasPrefix(lower, "--chdir="):
						// value is embedded; nothing extra to consume
					}
					continue
				}
				if strings.Contains(t, "=") {
					tokens = tokens[1:] // KEY=value: skip
					continue
				}
				break // not a flag, not assignment: command starts here
			}

		case "command":
			tokens = tokens[1:] // skip "command"
			// Skip flags: -p/-v/-V/-- (no argument-consuming flags).
			for len(tokens) > 0 {
				t := tokens[0]
				if t == "--" {
					tokens = tokens[1:]
					break
				}
				if strings.HasPrefix(t, "-") {
					tokens = tokens[1:]
					continue
				}
				break
			}

		case "sudo":
			// sudo [-E|-n|-s|...] [-u user] [--] <cmd>
			tokens = tokens[1:] // skip "sudo"
			for len(tokens) > 0 {
				t := tokens[0]
				if t == "--" {
					tokens = tokens[1:] // skip --, next is the command
					break
				}
				if !strings.HasPrefix(t, "-") {
					break // non-flag token: this is the command
				}
				tokens = tokens[1:] // skip the flag
				// Flags that consume a following argument.
				switch t {
				case "-u", "-g", "-C", "-D", "-T", "-U", "-h", "-p", "-r", "-t",
					"--user", "--group", "--chdir", "--close-from", "--host",
					"--prompt", "--role", "--type", "--other-user":
					if len(tokens) > 0 {
						tokens = tokens[1:]
					}
				}
			}

		case "nohup":
			// nohup <cmd> [args...] — no flags of its own that precede the command.
			tokens = tokens[1:]

		case "time":
			// time [-p] <cmd> — POSIX time builtin/command. Skip the binary; any
			// -p flag is absorbed by the shell, not passed to the command.
			tokens = tokens[1:]
			for len(tokens) > 0 {
				if tokens[0] == "-p" || tokens[0] == "--portability" {
					tokens = tokens[1:]
					continue
				}
				break
			}

		case "exec":
			// exec [-a name] [-c] [-l] [--] <cmd> [args...]
			tokens = tokens[1:]
			for len(tokens) > 0 {
				t := tokens[0]
				if t == "--" {
					tokens = tokens[1:]
					break
				}
				if !strings.HasPrefix(t, "-") {
					break
				}
				tokens = tokens[1:]
				if t == "-a" && len(tokens) > 0 {
					tokens = tokens[1:] // skip NAME argument
				}
			}

		case "nice":
			// nice [-n adjustment] <cmd> [args...]
			tokens = tokens[1:]
			for len(tokens) > 0 {
				t := tokens[0]
				if t == "-n" || t == "--adjustment" {
					tokens = tokens[1:] // skip flag
					if len(tokens) > 0 {
						tokens = tokens[1:] // skip adjustment value
					}
					continue
				}
				if strings.HasPrefix(t, "-n") || strings.HasPrefix(strings.ToLower(t), "--adjustment=") {
					tokens = tokens[1:] // combined -n5 or --adjustment=5
					continue
				}
				if strings.HasPrefix(t, "-") {
					tokens = tokens[1:]
					continue
				}
				break
			}

		case "doas":
			// doas [-u user] [-C config] [-n] [--] <cmd>
			tokens = tokens[1:]
			for len(tokens) > 0 {
				t := tokens[0]
				if t == "--" {
					tokens = tokens[1:]
					break
				}
				if !strings.HasPrefix(t, "-") {
					break
				}
				tokens = tokens[1:]
				if (t == "-u" || t == "-C") && len(tokens) > 0 {
					tokens = tokens[1:] // skip USER or CONFIG argument
				}
			}

		case "timeout":
			// timeout [OPTION] DURATION COMMAND [ARG]...
			// Flags: --foreground, --preserve-status, -k/--kill-after DURATION, -s/--signal SIG.
			// Followed by DURATION positional, then the command.
			tokens = tokens[1:]
			for len(tokens) > 0 {
				t := tokens[0]
				lower := strings.ToLower(t)
				if lower == "--foreground" || lower == "--preserve-status" {
					tokens = tokens[1:]
					continue
				}
				if lower == "-k" || lower == "--kill-after" {
					tokens = tokens[1:]
					if len(tokens) > 0 {
						tokens = tokens[1:] // skip DURATION arg
					}
					continue
				}
				if strings.HasPrefix(lower, "--kill-after=") {
					tokens = tokens[1:] // combined --kill-after=DURATION
					continue
				}
				if lower == "-s" || lower == "--signal" {
					tokens = tokens[1:]
					if len(tokens) > 0 {
						tokens = tokens[1:] // skip SIG arg
					}
					continue
				}
				if strings.HasPrefix(lower, "--signal=") {
					tokens = tokens[1:]
					continue
				}
				if strings.HasPrefix(t, "-") {
					tokens = tokens[1:]
					continue
				}
				if isDurationArg(t) {
					tokens = tokens[1:] // consume DURATION positional
					break              // command follows immediately
				}
				break // not a flag, not a duration: command
			}

		case "stdbuf":
			// stdbuf -i SIZE -o SIZE -e SIZE <cmd>
			tokens = tokens[1:]
			for len(tokens) > 0 {
				t := tokens[0]
				if !strings.HasPrefix(t, "-") {
					break
				}
				tokens = tokens[1:]
				lower := strings.ToLower(t)
				// -i/-o/-e (and long --input/--output/--error) take a SIZE arg
				// unless it's provided inline as --output=L.
				if strings.Contains(t, "=") {
					continue // inline value: already consumed
				}
				if lower == "-i" || lower == "-o" || lower == "-e" ||
					strings.HasPrefix(lower, "--input") || strings.HasPrefix(lower, "--output") || strings.HasPrefix(lower, "--error") {
					if len(tokens) > 0 {
						tokens = tokens[1:] // skip SIZE
					}
				}
			}

		default:
			consumed = false
		}

		if !consumed {
			break
		}
		// After consuming a prefix, strip any new naked assignments that may have
		// been revealed (e.g. nohup FOO=1 git push).
		tokens = consumeNakedAssignments(tokens)
	}
	return tokens
}

// findSubcommand skips leading git global flags and returns the first
// positional argument (the subcommand) and the remaining tokens.
func findSubcommand(tokens []string) (string, []string) {
	for len(tokens) > 0 {
		tok := tokens[0]
		switch tok {
		case "-C", "-c", "--work-tree", "--git-dir", "--namespace":
			if len(tokens) >= 2 {
				tokens = tokens[2:] // skip flag + value
			} else {
				tokens = tokens[1:]
			}
		default:
			if strings.HasPrefix(tok, "-") {
				tokens = tokens[1:] // skip other global flags
			} else {
				return tok, tokens[1:] // found subcommand
			}
		}
	}
	return "", nil
}

// applyRule checks the subcommand against the blocking rules. Returns the
// subcommand tag to report (e.g. "push") or "" to allow. Mirrors the bash
// case statement in match_subcommand() (lines 476–697).
func applyRule(sub string, rest []string) string {
	switch sub {
	case "commit":
		return "commit"

	case "push":
		if isPushBranchDelete(rest) {
			return "push-delete"
		}
		return "push"

	case "checkout":
		// Blocked: checkout -b/-B/--orphan (new branch).
		for _, t := range rest {
			switch t {
			case "-b", "-B", "--orphan":
				return "checkout-b"
			}
			if strings.HasPrefix(t, "--orphan=") {
				return "checkout-b"
			}
		}
		// Blocked: checkout -- <path> or checkout . (discard working tree).
		for _, t := range rest {
			if t == "--" || t == "." {
				return "checkout-path"
			}
		}
		return ""

	case "switch":
		// Blocked: switch -c/-C/--create/--force-create (new branch).
		for _, t := range rest {
			switch t {
			case "-c", "-C", "--create", "--force-create":
				return "switch-c"
			}
			if strings.HasPrefix(t, "--create=") || strings.HasPrefix(t, "--force-create=") {
				return "switch-c"
			}
		}
		return ""

	case "stash":
		// Allow only list and show.
		stashSub := ""
		if len(rest) > 0 {
			stashSub = rest[0]
		}
		switch stashSub {
		case "list", "show":
			return ""
		default:
			return "stash"
		}

	case "reset":
		// Blocked: --hard only.
		for _, t := range rest {
			if t == "--hard" {
				return "reset-hard"
			}
		}
		return ""

	case "clean":
		// Blocked: -f/-x unless --dry-run is also present.
		dryRun := false
		force := false
		for _, t := range rest {
			switch {
			case t == "-n" || t == "--dry-run":
				dryRun = true
			case t == "-x" || t == "-X" || t == "--force" || strings.HasPrefix(t, "-f"):
				force = true
			case strings.HasPrefix(t, "--force="):
				force = true
			}
		}
		if force && !dryRun {
			return "clean-force"
		}
		return ""

	case "restore":
		// Blocked unless --staged-only (no --worktree/-W, no positional arg with
		// only --staged). See .sh lines 578–603.
		restoreStaged := false
		restoreWorktree := false
		restorePositional := false
		for _, t := range rest {
			switch {
			case t == "--staged":
				restoreStaged = true
			case t == "--worktree" || t == "-W":
				restoreWorktree = true
			case strings.HasPrefix(t, "-"):
				// other flags: ignore
			default:
				restorePositional = true
			}
		}
		if restorePositional {
			if restoreWorktree || !restoreStaged {
				return "restore-path"
			}
		}
		return ""

	case "branch":
		// Allowed: read-only ops. Blocked: create/rename ops or positional
		// (unless --delete is also present). See .sh lines 615–646.
		branchAction := false
		hasDelete := false
		sawPositional := false
		skipNext := false
		for _, t := range rest {
			if skipNext {
				skipNext = false
				continue
			}
			switch {
			case t == "-c" || t == "-C" || t == "-m" || t == "-M" ||
				t == "--copy" || t == "--move" ||
				strings.HasPrefix(t, "--copy=") || strings.HasPrefix(t, "--move="):
				branchAction = true
			case t == "-d" || t == "-D" || t == "--delete" || strings.HasPrefix(t, "--delete="):
				hasDelete = true
			case t == "--contains" || t == "--no-contains" || t == "--sort" ||
				t == "--format" || t == "--points-at" || t == "--merged" || t == "--no-merged":
				skipNext = true
			case strings.HasPrefix(t, "-"):
				// other flags: skip
			default:
				sawPositional = true
			}
		}
		if !hasDelete && (branchAction || sawPositional) {
			return "branch-create"
		}
		return ""

	case "worktree":
		if len(rest) == 0 {
			return ""
		}
		op := rest[0]
		rest = rest[1:]
		switch op {
		case "add":
			for _, t := range rest {
				if t == "-b" || t == "-B" {
					return "worktree-add-b"
				}
			}
			// Block implicit branch creation: `git worktree add <path>` with no
			// branch-selection flag and no commit-ish as a second positional.
			// Allowed forms:
			//   git worktree add <path> <branch-or-commit>  — 2 positionals → explicit
			//   git worktree add --detach/-d <path>         — --detach flag → explicit
			// Blocked forms:
			//   git worktree add <path>                     — 1 positional, no flags above
			//   git worktree add --orphan <path>            — --orphan creates implicit branch
			//                                                 (git 2.54.0+, no -b required);
			//                                                 treated as blocked here; callers
			//                                                 that need --orphan should also
			//                                                 supply -b <name> explicitly.
			// Note on --reason: `git worktree add --lock --reason <msg> <path>` — the
			// token after --reason is consumed as its argument, not a positional. We model
			// this by skipping the next token whenever --reason is seen (R-2).
			hasExplicit := false
			positionals := 0
			skipNext := false
			for _, t := range rest {
				if skipNext {
					skipNext = false
					continue
				}
				if t == "--reason" {
					skipNext = true
					continue
				}
				switch t {
				case "--detach", "-d":
					hasExplicit = true
				}
				if !strings.HasPrefix(t, "-") {
					positionals++
				}
			}
			if !hasExplicit && positionals < 2 {
				return "worktree-add-implicit"
			}
		case "remove":
			for _, t := range rest {
				if t == "-f" || t == "--force" {
					return "worktree-remove-force"
				}
			}
		}
		return ""

	case "update-ref":
		// Always blocked (no exceptions). See .sh lines 674–682.
		return "update-ref"

	case "rm":
		// Blocked: -f/--force.
		for _, t := range rest {
			if t == "--force" || strings.HasPrefix(t, "-f") || strings.HasPrefix(t, "--force=") {
				return "rm-force"
			}
		}
		return ""

	case "add", "stage":
		// Blocking rule: ADR-2026-09-05 D1 — staging with implicit scope is blocked.
		//
		// BLOCKS when:
		//   (1) no positional AND no interactive flag AND no dry-run AND no --pathspec-from-file
		//   (2) ANY positional is a whole-tree root token (B-2 fix: "any" not "all") —
		//       root tokens include ., ./, .., ../, :/, :(top), :/:, and any path whose
		//       path.Clean (after \ → /) is "." or consists only of ".." segments
		//       (e.g. .//  ./.  ../.. ) — RT-1/RT-3 fix.
		//   (3) all positionals are exclusion pathspecs (:!x, :^x, :(exclude)x) — staging
		//       "everything except x" is implicit scope (RT-2 fix).
		//
		// PASSES when:
		//   - interactive flag present (including combined: -p / -i / --patch / --interactive
		//     and combined short-flag forms such as -Ap, -pA) — B-1 fix.
		//   - dry-run flag present (including combined: -n / --dry-run, -An, -nA) — B-1 fix.
		//   - --pathspec-from-file=<f> present (explicit enumeration via file; residual)
		//   - at least one positional that is neither a root token nor an exclusion pathspec.
		//
		// Combined short flags (e.g. -AN) are decomposed char-by-char (B-1 fix):
		//   n → dry-run; p, i → interactive. Other chars (A, u, etc.) do not change the
		//   decision. -AN bare (no dry-run char) still BLOCKS (correct: implicit scope).
		//
		// Residual D5: git${IFS}add, {git,add}, alias, $(…), external script with
		// git add -A inside. See docs/cli-parity.md §git-branch-guard residuals.
		// After --, all tokens are positionals (R-4).
		var (
			interactive         bool
			dryRun              bool
			hasPathspecFromFile bool
			hasPositional       bool
			anyPositionalRoot   bool
			allImplicit         = true // false when a real named path is seen
		)
		afterDash := false
		for _, t := range rest {
			if t == "--" {
				afterDash = true
				continue
			}
			if !afterDash && strings.HasPrefix(t, "-") {
				// Flag token (before --).
				if !strings.HasPrefix(t, "--") {
					// Short combined flag: decompose char-by-char (B-1 fix).
					for _, ch := range t[1:] {
						switch ch {
						case 'n':
							dryRun = true
						case 'p', 'i':
							interactive = true
						}
					}
				}
				// Long flags — prefix-unique resolver + last-wins (B-3/RT-R3-3/4 fix).
				// resolveAddLongOpt expands unambiguous git parse-options abbreviations
				// (--dry → --dry-run, --pat → --patch, --inter → --interactive, etc.)
				// and their --no-* counterparts. Last occurrence wins.
				if strings.HasPrefix(t, "--pathspec-from-file=") {
					hasPathspecFromFile = true
				} else {
					switch resolveAddLongOpt(t) {
					case "--dry-run":
						dryRun = true
					case "--patch", "--interactive":
						interactive = true
					case "--no-dry-run":
						dryRun = false
					case "--no-patch", "--no-interactive":
						interactive = false
					}
				}
			} else {
				// Positional (or token after --).
				hasPositional = true
				if isAddRootToken(t) {
					anyPositionalRoot = true
					// root token is still "implicit" — leave allImplicit true
				} else if isExclusionPathspec(t) {
					// exclusion-only pathspec is still "implicit" — leave allImplicit true
				} else {
					allImplicit = false // real named path: not implicit
				}
			}
		}
		if interactive || dryRun || hasPathspecFromFile {
			return "" // explicit or safe: allow
		}
		if !hasPositional {
			return "add-implicit" // (1) bare git add / git stage
		}
		if anyPositionalRoot {
			return "add-implicit" // (2) any root token — subsumes even explicit siblings (B-2)
		}
		if allImplicit {
			return "add-implicit" // (3) only exclusion pathspecs → whole-tree-minus-x (RT-2)
		}
		return ""
	}

	return "" // unknown subcommand: allow
}

// isAddRootToken reports whether a positional token addresses the entire working
// tree. Handles:
//
//	(a) literal closed set: . ./ .. ../ :/ :(top) :/:
//	(b) any path whose path.Clean (after \ → /) equals "." (e.g. .//  ./.)
//	(c) paths made entirely of ".." segments (e.g. ../.. )
//	(d) magic pathspecs with "top" attr, or "glob" attr with wildcard-only pattern
//	    (RT-NEW-4, red-team 2026-10-17 round 2)
func isAddRootToken(t string) bool {
	switch t {
	case ".", "./", "..", "../", ":/", ":(top)", ":/:":
		return true
	}
	norm := strings.ReplaceAll(t, `\`, `/`)
	cleaned := path.Clean(norm)
	if cleaned == "." {
		return true
	}
	if isAllDotDot(cleaned) {
		return true
	}
	// RT-NEW-4: magic pathspec :(attrs)pattern where attrs contains "top" AND
	// the pattern is itself a root token (empty, ".", or glob-wildcard-only),
	// or attrs contains "glob" AND pattern is only wildcard chars (* /**).
	// B-5 fix (round 3): ":(top)src/a.go" has a non-root pattern → NOT blocked.
	if strings.HasPrefix(t, ":(") {
		if closeIdx := strings.Index(t, ")"); closeIdx > 2 {
			attrs := t[2:closeIdx]
			pattern := t[closeIdx+1:]
			attrList := strings.Split(attrs, ",")
			hasTop := false
			hasGlob := false
			for _, a := range attrList {
				switch strings.TrimSpace(a) {
				case "top":
					hasTop = true
				case "glob":
					hasGlob = true
				}
			}
			if hasTop && isAddRootToken(pattern) {
				// e.g. :(top) (empty pattern), :(top)., :(top,glob)*
				return true
			}
			if hasGlob && isGlobWildcardOnly(pattern) {
				// e.g. :(glob)**, :(glob)*/**
				return true
			}
		}
	}
	return false
}

// isGlobWildcardOnly reports whether s consists entirely of '*' and '/'
// characters (e.g. "**", "*/**", "*/"). Used to detect glob pathspecs that
// address the entire working tree. Non-empty requirement prevents false positive
// on empty string.
func isGlobWildcardOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c != '*' && c != '/' {
			return false
		}
	}
	return true
}

// addLongOpts is the closed set of long options that the add/stage guard
// recognises. It is used by resolveAddLongOpt for unambiguous-prefix matching
// (git parse-options semantics). --pathspec-from-file= is kept separate
// because it has an inline = value and would conflict with --patch on --pa.
// RT-R3-3/RT-R3-4 fix (red-team 2026-10-17 round 3): --dry, --pat, --inter
// and their --no-* counterparts now resolve to their canonical forms.
var addLongOpts = []string{
	"--dry-run",
	"--patch",
	"--interactive",
	"--no-dry-run",
	"--no-patch",
	"--no-interactive",
}

// resolveAddLongOpt returns the canonical long option for t if t is an
// unambiguous prefix (by git parse-options rules) of exactly one element
// of addLongOpts. Returns "" if t is ambiguous or matches nothing.
func resolveAddLongOpt(t string) string {
	if !strings.HasPrefix(t, "--") {
		return ""
	}
	var match string
	for _, opt := range addLongOpts {
		if strings.HasPrefix(opt, t) {
			if match != "" {
				return "" // ambiguous
			}
			match = opt
		}
	}
	return match
}

// isAllDotDot reports whether p consists entirely of ".." segments.
func isAllDotDot(p string) bool {
	if p == "" {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg != ".." {
			return false
		}
	}
	return true
}

// isExclusionPathspec reports whether t is a pathspec exclusion form that
// tells git to stage everything *except* the named path:
// :!<path>, :^<path>, :(exclude)<path>, or any :(attrs) form whose attrs
// contain "exclude" (e.g. :(top,exclude)x, :(exclude,top)x).
// When ALL positionals are exclusion specs, the effective scope is the entire
// working tree minus those paths — implicit scope that should be blocked.
// B-5 related fix (round 3): :(top,exclude)x is an exclusion, not a root.
func isExclusionPathspec(t string) bool {
	if strings.HasPrefix(t, ":!") || strings.HasPrefix(t, ":^") || strings.HasPrefix(t, ":(exclude)") {
		return true
	}
	// :(attrs)path form — check if "exclude" is one of the attrs.
	if strings.HasPrefix(t, ":(") {
		if closeIdx := strings.Index(t, ")"); closeIdx > 2 {
			attrs := t[2:closeIdx]
			for _, a := range strings.Split(attrs, ",") {
				if strings.TrimSpace(a) == "exclude" {
					return true
				}
			}
		}
	}
	return false
}

// isPushBranchDelete reports whether any token in rest is a branch-deletion
// form of git push:
//   - "--delete"/"-d" flag (anywhere in rest) with non-tag positional refs; refs
//     are positionals[1:] when there are 2+ positionals (first is the remote name),
//     otherwise all positionals.
//   - refspec ":<ref>" or "+:<ref>" where <ref> is non-empty and not refs/tags/…
//
// The following forms keep reasonPush (not branch deletion):
//   - "--mirror", "--prune"
//   - ":" or "+:" alone (empty src)
//   - ":refs/tags/…" or "+:refs/tags/…"
//   - "--delete" with only refs/tags/… refs
//   - plain push (no deletion flag or refspec)
//
// Mixed commands (branch deletion + normal push in same invocation) return true.
//
// Residuals (declared): short tag names like "--delete v1.0" cannot be
// distinguished from branches; combined short flags like "-fd" are not parsed;
// flags that take values like "-o remote" or "--repo=remote" are not modelled.
func isPushBranchDelete(rest []string) bool {
	// Pass 1: --delete / -d appears anywhere → check positional refs.
	hasDelete := false
	for _, t := range rest {
		if t == "--delete" || t == "-d" {
			hasDelete = true
			break
		}
	}
	if hasDelete {
		var positionals []string
		for _, t := range rest {
			if !strings.HasPrefix(t, "-") {
				positionals = append(positionals, t)
			}
		}
		// git push syntax: first positional is the remote name when there are 2+
		// positionals; with exactly 1, that token IS the ref.
		refs := positionals
		if len(positionals) >= 2 {
			refs = positionals[1:]
		}
		for _, r := range refs {
			if !strings.HasPrefix(r, "refs/tags/") {
				return true
			}
		}
	}

	// Pass 2: refspec :<ref> or +:<ref> forms.
	for _, t := range rest {
		if strings.HasPrefix(t, "-") {
			continue
		}
		var ref string
		switch {
		case strings.HasPrefix(t, "+:"):
			ref = t[2:]
		case strings.HasPrefix(t, ":"):
			ref = t[1:]
		default:
			continue
		}
		if ref != "" && !strings.HasPrefix(ref, "refs/tags/") {
			return true
		}
	}
	return false
}

// stripHeredocBodies removes the body of heredoc blocks (keeping the opening
// and closing delimiter lines). If a heredoc is never closed, the original
// string is returned unchanged. Mirrors the bash strip_heredoc_bodies()
// awk function (lines 298–336).
func stripHeredocBodies(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	inHeredoc := false
	delim := ""

	for _, line := range lines {
		if inHeredoc {
			// Check terminator: trim leading+trailing whitespace, compare.
			trimmed := strings.TrimSpace(line)
			if trimmed == delim {
				inHeredoc = false
				out = append(out, line) // keep terminator line
			}
			// Body lines are dropped.
			continue
		}

		// Detect heredoc opening: <<-? followed by optional whitespace + delim.
		if d := parseHeredocDelim(line); d != "" {
			delim = d
			inHeredoc = true
		}
		out = append(out, line)
	}

	if inHeredoc {
		// Heredoc never closed: return original (safe side).
		return s
	}
	return strings.Join(out, "\n")
}

// parseHeredocDelim extracts the heredoc delimiter from a line containing <<
// or <<-. Strips surrounding quotes from the delimiter. Returns "" if none.
func parseHeredocDelim(line string) string {
	// Look for << or <<-
	idx := strings.Index(line, "<<")
	if idx < 0 {
		return ""
	}
	rest := line[idx+2:]
	if strings.HasPrefix(rest, "-") {
		rest = rest[1:]
	}
	// Strip leading whitespace
	rest = strings.TrimLeft(rest, " \t")
	// The delimiter is the first word (up to first space/tab)
	end := strings.IndexAny(rest, " \t")
	if end >= 0 {
		rest = rest[:end]
	}
	if rest == "" {
		return ""
	}
	// Strip quotes from delimiter (bash allows <<'EOF', <<"EOF", <<EOF)
	rest = strings.ReplaceAll(rest, `"`, "")
	rest = strings.ReplaceAll(rest, "'", "")
	return rest
}

// shellTokenize splits a command segment into words using shell quoting semantics
// (ADR-2026-10-04, D10). Rules:
//   - Unicode whitespace outside quotes separates words (unicode.IsSpace).
//   - Inside '…' or "…": whitespace does not separate; quotes are stripped.
//   - Adjacent quoted/unquoted parts concatenate: g""it → git, "a b"c → a bc.
//   - Unclosed quotes: the remainder is treated as one word (no panic).
//   - Backslash is NOT treated as an escape here; inside "…", quoteAwareSplit only
//     consumed the backslash for the POSIX special set ($, `, ", \, newline) — any
//     other backslash (e.g. Windows path separators) reaches this function intact.
//
// This replaces strings.Fields, which left quote characters inside tokens and
// prevented "git", 'git', and g""it from being recognised as the git binary.
func shellTokenize(s string) []string {
	var words []string
	var cur strings.Builder
	var q rune // current quote char; 0 = unquoted
	inWord := false
	for _, c := range s {
		if q != 0 {
			// Inside a quoted region.
			if c == q {
				q = 0 // closing quote — strip it, stay in word
			} else {
				cur.WriteRune(c)
			}
			continue
		}
		// Outside quotes.
		switch {
		case c == '\'' || c == '"':
			q = c   // open quote — strip it, start/continue word
			inWord = true
		case unicode.IsSpace(c):
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(c)
			inWord = true
		}
	}
	// Flush the last (or only) word; also handles unclosed-quote remainder.
	if inWord || cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words
}

// quoteAwareSplit splits cmdRaw on `;`, `&&`, `||`, `|`, `(`, `)` and
// newlines outside of single or double quotes. The result is a list of
// command segments (subcommands / subshell bodies to inspect individually).
//
// `(` is a separator only at word-boundary (start of segment or preceded by
// whitespace): `(git add .)` splits into ["", "git add .", ""].
// A `(` immediately after a non-space character (e.g. `:(top)`) starts a
// word-local depth counter; its matching `)` is literal and stays in the
// segment, preserving magic pathspecs like `:(top)src/a.go`.
//
// `#` outside quotes at word-boundary starts a shell comment: the rest of
// the current line is skipped (preventing a `'` inside a comment from
// opening a spurious quote region in multi-line inputs).
//
// Newlines inside quotes become spaces. The result is a list of command
// segments. Mirrors the bash quote_aware_split() awk function (lines 347–402).
// B-4/B-6 fix (red-team 2026-10-17 rounds 2–3).
func quoteAwareSplit(s string) []string {
	var segments []string
	var cur strings.Builder
	runes := []rune(s)
	n := len(runes)
	var q rune     // current quote char (0 = unquoted)
	var parenDepth int
	atWordBound := true // true at start or after whitespace/separator

	emitSeg := func() {
		segments = append(segments, cur.String())
		cur.Reset()
		atWordBound = true
		parenDepth = 0
	}

	for i := 0; i < n; i++ {
		c := runes[i]

		if q != 0 {
			// Inside quotes.
			if q == '"' && c == '\\' && i+1 < n {
				next := runes[i+1]
				// POSIX: inside "…", backslash only escapes $, `, ", \, and newline.
				// Before any other character the backslash is literal (written as-is).
				switch next {
				case '$', '`', '"', '\\':
					cur.WriteRune(next)
				case '\n':
					cur.WriteRune(' ')
				default:
					cur.WriteRune('\\')
					cur.WriteRune(next)
				}
				i++
				continue
			}
			if c == q {
				q = 0
				cur.WriteRune(c)
				atWordBound = false
				continue
			}
			if c == '\n' {
				cur.WriteRune(' ')
			} else {
				cur.WriteRune(c)
			}
			atWordBound = false
			continue
		}

		// Outside quotes.
		switch {
		case c == '"' || c == '\'':
			q = c
			cur.WriteRune(c)
			atWordBound = false

		case c == '#' && atWordBound:
			// Comment start: skip to end of current line, then emit the segment
			// built so far. This prevents `'` inside the comment from opening a
			// spurious quote that swallows the next line.
			for i+1 < n && runes[i+1] != '\n' {
				i++
			}
			// Fall through to newline handling on the next iteration.

		case i+1 < n && c == '&' && runes[i+1] == '&':
			emitSeg()
			i++ // skip second '&'

		case i+1 < n && c == '|' && runes[i+1] == '|':
			emitSeg()
			i++ // skip second '|'

		case c == ';' || c == '|':
			emitSeg()

		case c == '&':
			// Single `&` outside quotes: shell control operator (cmd.exe command
			// chaining, PS call operator). Split into a new segment UNLESS the `&`
			// is part of a shell redirection: `2>&1`, `>&2`, `&>file`, `|&`.
			// Adjacency rule: skip separator when `&` is directly preceded or followed
			// by `>` or `<`, or preceded by `|`.
			prevIsRedirect := false
			if i > 0 {
				p := runes[i-1]
				prevIsRedirect = p == '>' || p == '<' || p == '|'
			}
			nextIsRedirect := i+1 < n && (runes[i+1] == '>' || runes[i+1] == '<')
			if prevIsRedirect || nextIsRedirect {
				cur.WriteRune(c) // redirect context: keep as-is
				atWordBound = false
			} else {
				emitSeg()
			}

		case c == '(':
			// Subshell opener — behaviour depends on position.
			// Word-boundary (start of segment or after whitespace): treat as
			// a separator, so "(git add .)" splits into ["", "git add .", ""].
			// Mid-word (immediately after non-space, e.g. the ':' in ':(top)'):
			// open a depth counter; the matching ')' stays literal in the segment,
			// preserving magic pathspecs. B-4/B-6 fix (round 3).
			if atWordBound {
				emitSeg()
			} else {
				parenDepth++
				cur.WriteRune(c)
				// atWordBound stays false
			}

		case c == ')':
			// Subshell closer or depth decrement.
			if parenDepth > 0 {
				parenDepth--
				cur.WriteRune(c)
				atWordBound = false
			} else {
				// depth 0 at word-boundary or end of word: separator.
				emitSeg()
			}

		case c == '\n':
			// Newline outside quotes acts as a segment separator.
			emitSeg()

		case c == ' ' || c == '\t':
			cur.WriteRune(c)
			atWordBound = true

		default:
			cur.WriteRune(c)
			atWordBound = false
		}
	}

	segments = append(segments, cur.String())
	return segments
}

// ---- POSIX-aware second pipeline (D4, Wave 1 ML-1B, REQ-2026-11-05) ----
//
// quoteAwareSplitPOSIX is a variant of quoteAwareSplit that preserves
// backslashes verbatim inside "…" (all \x pairs emitted as-is, except
// \<newline> which becomes a space for line continuation) and treats backslash
// outside quotes as an escape that prevents the next character from acting as
// a shell operator or separator. This allows shellTokenizePOSIX to perform the
// single POSIX unescaping pass.
//
// The difference from quoteAwareSplit:
//
//   - Inside "…": `\"` → `\"` (verbatim, NOT just `"`). This prevents a quoted
//     double-quote from accidentally closing the outer string in the caller's
//     segment buffer. Consequence: bash -c "bash -c \"git push\"" is now one
//     segment, not split at the inner `\"`.
//   - Outside quotes: `\x` → `\x` (verbatim pair emitted), skipping both; the
//     next char is NOT examined as a potential operator. This prevents `\;`,
//     `\&&` from splitting the segment.
func quoteAwareSplitPOSIX(s string) []string {
	var segments []string
	var cur strings.Builder
	runes := []rune(s)
	n := len(runes)
	var q rune
	var parenDepth int
	atWordBound := true

	emitSeg := func() {
		segments = append(segments, cur.String())
		cur.Reset()
		atWordBound = true
		parenDepth = 0
	}

	for i := 0; i < n; i++ {
		c := runes[i]

		if q != 0 {
			// Inside a quoted region.
			if q == '"' && c == '\\' && i+1 < n {
				next := runes[i+1]
				// POSIX-aware: emit \+next verbatim for all chars except \<newline>
				// (line continuation → space). shellTokenizePOSIX will unescape.
				if next == '\n' {
					cur.WriteRune(' ')
				} else {
					cur.WriteRune('\\')
					cur.WriteRune(next)
				}
				i++
				continue
			}
			if c == q {
				q = 0
				cur.WriteRune(c)
				atWordBound = false
				continue
			}
			if c == '\n' {
				cur.WriteRune(' ')
			} else {
				cur.WriteRune(c)
			}
			atWordBound = false
			continue
		}

		// Outside quotes: backslash escapes the next character (POSIX §2.2.1).
		if c == '\\' && i+1 < n {
			next := runes[i+1]
			if next == '\n' {
				// Line continuation: skip both (don't split, don't emit).
				i++
				continue
			}
			// Emit both chars verbatim; next char is NOT a potential operator.
			cur.WriteRune('\\')
			cur.WriteRune(next)
			atWordBound = false
			i++
			continue
		}

		switch {
		case c == '"' || c == '\'':
			q = c
			cur.WriteRune(c)
			atWordBound = false

		case c == '#' && atWordBound:
			for i+1 < n && runes[i+1] != '\n' {
				i++
			}

		case i+1 < n && c == '&' && runes[i+1] == '&':
			emitSeg()
			i++

		case i+1 < n && c == '|' && runes[i+1] == '|':
			emitSeg()
			i++

		case c == ';' || c == '|':
			emitSeg()

		case c == '&':
			prevIsRedirect := false
			if i > 0 {
				p := runes[i-1]
				prevIsRedirect = p == '>' || p == '<' || p == '|'
			}
			nextIsRedirect := i+1 < n && (runes[i+1] == '>' || runes[i+1] == '<')
			if prevIsRedirect || nextIsRedirect {
				cur.WriteRune(c)
				atWordBound = false
			} else {
				emitSeg()
			}

		case c == '(':
			if atWordBound {
				emitSeg()
			} else {
				parenDepth++
				cur.WriteRune(c)
			}

		case c == ')':
			if parenDepth > 0 {
				parenDepth--
				cur.WriteRune(c)
				atWordBound = false
			} else {
				emitSeg()
			}

		case c == '\n':
			emitSeg()

		case c == ' ' || c == '\t':
			cur.WriteRune(c)
			atWordBound = true

		default:
			cur.WriteRune(c)
			atWordBound = false
		}
	}

	segments = append(segments, cur.String())
	return segments
}

// shellTokenizePOSIX is a variant of shellTokenize that applies POSIX
// backslash-unescaping rules:
//
//   - Outside quotes: `\x` → `x` (drop the backslash; prevents \git from
//     reaching filepath.Base as a non-git token).
//   - Inside '…': no unescaping (single quotes prevent all escaping).
//   - Inside "…": `\"` → `"`, `\\` → `\`, `\$` → `$`, `` \` `` → `` ` ``,
//     `\<newline>` → space (line continuation); other `\x` → `\x` (keep both,
//     per POSIX §2.2.3 "other characters shall not be treated as special").
//
// Input is typically a segment produced by quoteAwareSplitPOSIX; the opening
// and closing quote characters are present in the segment and are stripped here
// (same as shellTokenize).
func shellTokenizePOSIX(s string) []string {
	var words []string
	var cur strings.Builder
	runes := []rune(s)
	n := len(runes)
	var q rune // 0 = unquoted, '\'' = single-quoted, '"' = double-quoted
	inWord := false

	for i := 0; i < n; i++ {
		c := runes[i]

		if q == '\'' {
			// Inside single quotes: everything is literal.
			if c == '\'' {
				q = 0 // closing single quote: strip it, stay in word
			} else {
				cur.WriteRune(c)
			}
			continue
		}

		if q == '"' {
			// Inside double quotes: POSIX backslash unescaping.
			if c == '"' {
				q = 0 // closing double quote: strip it, stay in word
				continue
			}
			if c == '\\' && i+1 < n {
				next := runes[i+1]
				switch next {
				case '$', '`', '"', '\\':
					cur.WriteRune(next)
					i++
				case '\n':
					cur.WriteRune(' ') // line continuation
					i++
				default:
					// Non-special: keep both backslash and char (POSIX §2.2.3).
					cur.WriteRune('\\')
					cur.WriteRune(next)
					i++
				}
				continue
			}
			cur.WriteRune(c)
			continue
		}

		// Outside quotes.
		switch {
		case c == '\'' || c == '"':
			q = c
			inWord = true

		case c == '\\' && i+1 < n:
			next := runes[i+1]
			if next == '\n' {
				// Line continuation outside quotes: skip both.
				i++
			} else {
				// POSIX §2.2.1: backslash preserves the literal value of the
				// following character. Emit next char only (drop backslash).
				cur.WriteRune(next)
				i++
				inWord = true
			}

		case unicode.IsSpace(c):
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}

		case c == '$' && i+1 < n && (runes[i+1] == '\'' || runes[i+1] == '"'):
			// ANSI-C quoting: $'...' and locale quoting: $"..."
			// The leading $ is decorative; drop it and let the following
			// quote character open a quoted region on the next iteration.
			// C6 — D4 Wave 2 (REQ-2026-11-05, ML-2C).
			inWord = true
			// Do NOT advance i; let the next iteration open the quote.

		default:
			cur.WriteRune(c)
			inWord = true
		}
	}

	if inWord || cur.Len() > 0 {
		words = append(words, cur.String())
	}
	return words
}

// stripShellSuffixPOSIX is a variant of stripShellSuffix that applies POSIX
// backslash awareness inside "…": a backslash before any character (including
// `"`) does not close the double-quoted string. Without this fix, `\"` inside
// "…" would be misanalysed as a closing quote, exposing operators that follow.
func stripShellSuffixPOSIX(seg string) string {
	runes := []rune(seg)
	n := len(runes)
	var out strings.Builder
	var q rune
	atWordBoundary := true

	for i := 0; i < n; i++ {
		c := runes[i]

		if q != 0 {
			// Inside a quoted region.
			if q == '"' && c == '\\' && i+1 < n {
				// POSIX: backslash inside "…" does NOT close the string.
				// Emit both chars verbatim and advance.
				out.WriteRune('\\')
				out.WriteRune(runes[i+1])
				i++
				atWordBoundary = false
				continue
			}
			if c == q {
				q = 0
			}
			out.WriteRune(c)
			atWordBoundary = false
			continue
		}

		// Outside quotes (identical to stripShellSuffix).
		switch {
		case c == '\'' || c == '"':
			q = c
			out.WriteRune(c)
			atWordBoundary = false

		case c == '#' && atWordBoundary:
			goto done

		case c == '>' || c == '<':
			s := out.String()
			if len(s) > 0 {
				last := rune(s[len(s)-1])
				if last >= '0' && last <= '9' {
					prevIsSpace := len(s) < 2 || s[len(s)-2] == ' ' || s[len(s)-2] == '\t'
					if prevIsSpace {
						out.Reset()
						out.WriteString(s[:len(s)-1])
					}
				}
			}
			for i+1 < n && (runes[i+1] == '>' || runes[i+1] == '<' || runes[i+1] == '&' || runes[i+1] == '|') {
				i++
			}
			inlineTarget := i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t'
			if !inlineTarget {
				for i+1 < n && (runes[i+1] == ' ' || runes[i+1] == '\t') {
					i++
				}
				for i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t' &&
					runes[i+1] != ';' && runes[i+1] != '&' && runes[i+1] != '|' {
					i++
				}
			} else {
				for i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t' &&
					runes[i+1] != ';' && runes[i+1] != '&' && runes[i+1] != '|' {
					i++
				}
			}
			atWordBoundary = true

		case c == '&' && i+1 < n && (runes[i+1] == '>' || runes[i+1] == '<'):
			i++
			for i+1 < n && (runes[i+1] == '>' || runes[i+1] == '<') {
				i++
			}
			inlineTarget := i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t'
			if !inlineTarget {
				for i+1 < n && (runes[i+1] == ' ' || runes[i+1] == '\t') {
					i++
				}
				for i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t' &&
					runes[i+1] != ';' && runes[i+1] != '&' && runes[i+1] != '|' {
					i++
				}
			} else {
				for i+1 < n && runes[i+1] != ' ' && runes[i+1] != '\t' &&
					runes[i+1] != ';' && runes[i+1] != '&' && runes[i+1] != '|' {
					i++
				}
			}
			atWordBoundary = true

		case c == ' ' || c == '\t':
			out.WriteRune(c)
			atWordBoundary = true

		default:
			out.WriteRune(c)
			atWordBoundary = false
		}
	}

done:
	return out.String()
}

// shellInterpreter reports whether base (lowercase, without .exe suffix) is a
// recognised shell interpreter for which the guard can recurse into -c args.
// The list is a denylist-open declaration — it covers the interpreters whose
// -c/-Command//c flag takes a command string the guard can re-analyse.
// Language runtimes (python, node, ruby…) are intentionally absent; see D5.
func shellInterpreter(base string) bool {
	switch base {
	case "bash", "sh", "zsh", "dash", "ksh", "pwsh", "powershell", "cmd", "eval":
		return true
	}
	return false
}

// isPOSIXArgConsumingFlag reports whether the flag string consumes the next
// token as its value in a POSIX shell invocation (e.g. bash -o optname).
func isPOSIXArgConsumingFlag(tok string) bool {
	switch tok {
	case "-o", "+o", "-O", "+O", "--rcfile", "--init-file":
		return true
	}
	return false
}

// extractPOSIXShellCommandString finds the -c (or combined, e.g. -lc) flag in
// the tokens that follow a POSIX shell binary and returns the command string.
//
// Returns (cmdStr, true, false) on success; ("", false, true) when -c is
// present but no command string follows (fail-closed); ("", false, false) when
// no -c is found (not a -c invocation, e.g. bash -s or bash script.sh).
func extractPOSIXShellCommandString(rest []string) (string, bool, bool) {
	for i := 0; i < len(rest); i++ {
		tok := rest[i]

		// Combined flag containing 'c' (e.g. -lc, -ec): command string is next token.
		if hasCFlag(tok) {
			if i+1 < len(rest) {
				return rest[i+1], true, false
			}
			return "", false, true // combined -c with no command string
		}

		if tok == "-c" {
			// Find the first non-flag token after -c (the command string).
			for j := i + 1; j < len(rest); j++ {
				t := rest[j]
				if strings.HasPrefix(t, "-") || strings.HasPrefix(t, "+") {
					if isPOSIXArgConsumingFlag(t) && j+1 < len(rest) {
						j++ // skip flag value
					}
					continue
				}
				return t, true, false
			}
			return "", false, true // -c found but no command string
		}

		// End-of-flags: everything after is positional (script name or command).
		if tok == "--" {
			return "", false, false
		}

		// A non-flag, non-assignment token before -c: this is a script path, not -c.
		if !strings.HasPrefix(tok, "-") && !strings.HasPrefix(tok, "+") && !isNakedAssignment(tok) {
			break
		}

		// Skip arg-consuming flags.
		if isPOSIXArgConsumingFlag(tok) && i+1 < len(rest) {
			i++
		}
	}
	return "", false, false
}

// psNormFlag normalises a PowerShell flag token: strips a leading extra dash
// so that "--command" becomes "-command" (PS 7 accepts double-dash for some
// params). The result always starts with exactly one '-' when the input is a
// flag.
func psNormFlag(lower string) string {
	if strings.HasPrefix(lower, "--") && len(lower) > 2 {
		return lower[1:] // "--command" → "-command"
	}
	return lower
}

// isPsCommandFlag reports whether norm (already lower-cased and double-dash-
// normalised) is the PowerShell command-string flag in any recognised form:
//   - any prefix of "-command" ("-c", "-co", "-com", …, "-command")
//   - "-commandwithargs" (PS 7.4+) or its abbreviation "-cwa"
func isPsCommandFlag(norm string) bool {
	return strings.HasPrefix("-command", norm) ||
		norm == "-commandwithargs" || norm == "-cwa"
}

// isPsArgConsumingFlag reports whether norm is a PowerShell flag that consumes
// one following argument (the argument itself is not a command string).
// Two checks are applied:
//  1. norm is a prefix of a full flag name (len >= 2 to avoid "-" alone).
//  2. norm is an explicit documented non-prefix alias for pwsh 7
//     (e.g. "-ep" for "-ExecutionPolicy" is not a prefix but is official).
func isPsArgConsumingFlag(norm string) bool {
	if len(norm) < 2 {
		return false
	}
	for _, full := range []string{
		"-executionpolicy",    // -ex, -exe, ... (NOT -ep — alias below)
		"-workingdirectory",   // -wo, -wor, ... (NOT -wd — alias below)
		"-windowstyle",        // -w, -wi, ...
		"-configurationname",  // -config, ... (prefix also covers -con through -configurationn)
		"-configurationfile",  // no short alias; -configurationf distinguishes from -configurationname
		"-outputformat",       // -o, -ou, ... (NOT -of — alias below)
		"-inputformat",        // -in, -inp, ... (NOT -if — alias below)
		"-settingsfile",       // -set, -sett, ...
		"-custompipename",     // -cust, ...
		"-version",            // -v, -ve, ...
	} {
		if strings.HasPrefix(full, norm) {
			return true
		}
	}
	// Explicit documented pwsh 7 aliases that are NOT prefixes of the full name.
	// Source: https://learn.microsoft.com/powershell/module/microsoft.powershell.core/about/about_pwsh
	// ML-2F adds -of (OutputFormat alias). (REQ-2026-11-05).
	switch norm {
	case "-ep", // alias for -ExecutionPolicy
		"-wd", // alias for -WorkingDirectory
		"-if", // alias for -InputFormat
		"-of": // alias for -OutputFormat — ML-2F (REQ-2026-11-05)
		return true
	}
	return false
}

// stripPsBacktick removes PowerShell backtick (`) escape characters from a
// command string outside of single-quoted regions. Inside '…' backtick is
// literal. This normalises "g`it push" → "git push". C8 — D4 Wave 2
// (REQ-2026-11-05, ML-2C).
func stripPsBacktick(s string) string {
	var out strings.Builder
	inSQ := false
	runes := []rune(s)
	n := len(runes)
	for i := 0; i < n; i++ {
		c := runes[i]
		switch {
		case c == '\'' && !inSQ:
			inSQ = true
			out.WriteRune(c)
		case c == '\'' && inSQ:
			inSQ = false
			out.WriteRune(c)
		case c == '`' && !inSQ && i+1 < n:
			// Backtick outside single-quotes: emit the next character only.
			out.WriteRune(runes[i+1])
			i++
		default:
			out.WriteRune(c)
		}
	}
	return out.String()
}

// isPsNoAnalyzeFlag reports whether norm is a PowerShell flag whose argument
// cannot be statically analysed by this guard:
//   - "-file" ("-f", "-fi", …): the argument is a script path, not a command.
//   - "-encodedcommand" ("-e", "-en", "-enc", …): the argument is Base64-encoded
//     UTF-16LE; decoding it is out-of-scope (Residual R3).
func isPsNoAnalyzeFlag(norm string) bool {
	if len(norm) < 2 {
		return false
	}
	return strings.HasPrefix("-file", norm) || strings.HasPrefix("-encodedcommand", norm)
}

// extractPowerShellCommandString finds the command-string flag in the tokens
// that follow pwsh/powershell and returns the command string (joined for
// multi-token forms).
//
// Handles:
//   - double-dash normalisation ("--command" → "-command", PS 7 quirk)
//   - any prefix abbreviation of "-command" ("-c", "-co", "-com", …)
//   - "-commandwithargs" / "-cwa" (PS 7.4+)
//   - flags that consume one argument ("-ExecutionPolicy Bypass", "-wd C:\x", …)
//   - flags without an argument ("-NoProfile", "-NonInteractive", "-Sta", …)
//   - "-File"/"-EncodedCommand": not a -c invocation (return not-found)
//   - positional command (powershell only, PS 5.1): when base=="powershell", the
//     first non-flag token is the implicit -Command argument (PS 5.1 default
//     positional). For pwsh 7, the first positional is -File, so it is not
//     treated as a command string (break, return not-found).
//
// B1, C5, F1-A — D4 Wave 2 (REQ-2026-11-05, ML-2C).
// Positional (powershell 5.1) — ML-2E (REQ-2026-11-05).
func extractPowerShellCommandString(rest []string, usePositionalCommand bool) (string, bool, bool) {
	for i := 0; i < len(rest); i++ {
		tok := rest[i]
		lower := strings.ToLower(tok)
		norm := psNormFlag(lower)

		// -File / -EncodedCommand: cannot analyse statically (R3).
		if isPsNoAnalyzeFlag(norm) {
			return "", false, false
		}

		// Command-string flag: everything that follows is the command.
		if isPsCommandFlag(norm) {
			if i+1 < len(rest) {
				return stripPsBacktick(strings.Join(rest[i+1:], " ")), true, false
			}
			return "", false, true // flag present but no argument
		}

		// Flags that consume one following argument.
		if isPsArgConsumingFlag(norm) {
			i++ // skip the argument value
			continue
		}

		// Flags without argument ("-NoProfile", "-NonInteractive", "-Sta", …).
		if strings.HasPrefix(tok, "-") {
			continue
		}

		// Non-flag token before the command flag.
		// powershell (PS 5.1): the first positional argument is -Command, so
		// treat the remaining tokens as the command string.
		// pwsh (PS 7): the first positional is -File; not analysed statically.
		if usePositionalCommand {
			return stripPsBacktick(strings.Join(rest[i:], " ")), true, false
		}
		break
	}
	return "", false, false
}

// stripCmdCarets removes caret (^) escape characters from a cmd.exe command
// string outside of double-quoted regions. Inside "…" the caret is literal.
// This normalises "g^it push" → "git push" so the guard can inspect the
// real command. C8 — D4 Wave 2 (REQ-2026-11-05, ML-2C).
func stripCmdCarets(s string) string {
	var out strings.Builder
	inDQ := false
	runes := []rune(s)
	for i, c := range runes {
		switch {
		case c == '"':
			inDQ = !inDQ
			out.WriteRune(c)
		case c == '^' && !inDQ:
			// Caret outside quotes: escape the next character (which the
			// tokenizer will keep). Drop the caret itself.
			if i+1 < len(runes) {
				// next char will be emitted in the next iteration
			}
		default:
			out.WriteRune(c)
		}
	}
	return out.String()
}

// extractCmdCommandString finds /c or /k in the tokens that follow cmd.exe
// and returns the command string. Handles:
//   - separated form:  cmd /c "git push"
//   - fused form:      cmd /c"git push"   (no space after /c)
//
// Caret escapes (g^it) are stripped via stripCmdCarets. B3, C8 — D4 Wave 2
// (REQ-2026-11-05, ML-2C).
func extractCmdCommandString(rest []string) (string, bool, bool) {
	for i := 0; i < len(rest); i++ {
		tok := rest[i]
		lower := strings.ToLower(tok)

		// Fused form: "/c..." or "/k..." (no space between switch and string).
		if (strings.HasPrefix(lower, "/c") || strings.HasPrefix(lower, "/k")) && len(tok) > 2 {
			fused := tok[2:]
			tail := strings.Join(rest[i+1:], " ")
			if tail != "" {
				fused = fused + " " + tail
			}
			return stripCmdCarets(strings.Trim(fused, "\"")), true, false
		}

		// Separated form: "/c" or "/k" as a standalone token.
		if lower == "/c" || lower == "/k" {
			if i+1 < len(rest) {
				return stripCmdCarets(strings.Join(rest[i+1:], " ")), true, false
			}
			return "", false, true
		}
		if strings.HasPrefix(lower, "/") {
			continue // skip /s, /q, /d, other cmd flags
		}
		break
	}
	return "", false, false
}

// extractShellCommandString dispatches to the per-interpreter extractor and
// returns (cmdStr, found, failClosed). found=true means a command string was
// extracted; failClosed=true means the wrapper is there but we cannot extract
// the string (fail-closed: return "wrapper-not-analyzed").
func extractShellCommandString(base string, rest []string) (string, bool, bool) {
	switch base {
	case "bash", "sh", "zsh", "dash", "ksh":
		return extractPOSIXShellCommandString(rest)
	case "pwsh":
		// pwsh 7: first positional is -File, not -Command. No positional analysis.
		return extractPowerShellCommandString(rest, false)
	case "powershell":
		// powershell (PS 5.1): first positional is -Command. Use positional analysis.
		return extractPowerShellCommandString(rest, true)
	case "cmd":
		return extractCmdCommandString(rest)
	case "eval":
		if len(rest) == 0 {
			return "", false, true // eval with no arguments: fail-closed
		}
		return strings.Join(rest, " "), true, false
	}
	return "", false, false
}

// handleShellWrapper analyses the tokens that follow a shell interpreter.
// Returns the block tag (e.g. "push") if the inner command is blocked,
// "wrapper-not-analyzed" if the depth is exceeded or the command string cannot
// be extracted, or "" if the tokens do not constitute a -c invocation or if
// the inner command is allowed.
func handleShellWrapper(base string, rest []string, depth int) string {
	cmdStr, found, failClosed := extractShellCommandString(base, rest)
	if failClosed {
		return "wrapper-not-analyzed"
	}
	if !found {
		return "" // not a -c invocation (e.g. bash -s, bash script.sh)
	}
	if depth >= maxWrapperDepth {
		return "wrapper-not-analyzed"
	}
	return matchSubcommandPOSIX(cmdStr, depth+1)
}

// extractBalancedParenBody extracts the body of a $(…) starting at position
// start (immediately after the opening '('). Returns (body, index-of-closing-
// paren). When the parentheses are unbalanced the body collected so far is
// returned with end = n-1 so the caller can still analyse the content
// (fail-closed). Single-quoted regions are written verbatim to body.
//
// Handles:
//   - single-quoted regions ('…'): literal, written verbatim to body.
//   - double-quoted regions ("…"): $() and backticks expand; ')' and '('
//     have no structural significance for depth counting inside "…".
//   - backslash outside quotes: \x is written literally; \) does NOT
//     decrement depth and \' does NOT open a single-quoted region.
//   - '#' at word boundary: comment to end-of-line, skipped for depth.
//   - case…esac constructs: ')' terminating a case pattern does NOT
//     decrement depth; supports nested case and both parenthesized and
//     plain patterns. When the optional '(' precedes a pattern, the depth
//     increment ensures the matching ')' cannot reach zero without
//     interception. ML-2I (REQ-2026-11-05).
//
// C7 — D4 Wave 2 (REQ-2026-11-05, ML-2C/2I).
func extractBalancedParenBody(runes []rune, start int) (string, int) {
	depth := 1
	var body strings.Builder
	n := len(runes)
	inSQ := false
	inDQ := false

	// Case-statement state stack.  Each entry tracks one open case…esac.
	// Values:
	//   0 = in case expression (between 'case' keyword and 'in' keyword)
	//   1 = pattern position (after 'in' or after ';;'/';&'/';;&'):
	//         the next bare ')' is a pattern terminator, not a depth-decrement
	//   2 = case body (after the pattern-terminating ')'):
	//         ')' is handled normally (depth-decrement or early return)
	// When the optional '(' precedes a pattern the frame is set to 2 on that
	// '(' so the matching ')' is not mistakenly intercepted.
	var caseStack []int

	// Word accumulator for keyword detection ('case', 'in', 'esac').
	var wordBuf strings.Builder
	wordStartedAtBound := true // did the current word start at a word boundary?
	atWordBound := true        // is the current position a word boundary?

	// flushWord checks whether the accumulated word is a keyword and updates
	// caseStack accordingly.
	flushWord := func() {
		if wordBuf.Len() == 0 {
			return
		}
		w := wordBuf.String()
		wordBuf.Reset()
		if !wordStartedAtBound {
			return
		}
		switch w {
		case "case":
			caseStack = append(caseStack, 0)
		case "in":
			if len(caseStack) > 0 && caseStack[len(caseStack)-1] == 0 {
				caseStack[len(caseStack)-1] = 1
			}
		case "esac":
			if len(caseStack) > 0 {
				caseStack = caseStack[:len(caseStack)-1]
			}
		}
	}

	for i := start; i < n; i++ {
		c := runes[i]

		// Single-quoted region: entirely literal, written verbatim.
		if inSQ {
			body.WriteRune(c)
			if c == '\'' {
				inSQ = false
				atWordBound = false
			}
			continue
		}

		// Backslash outside any quote: skip the escaped character for depth
		// purposes (\) does not decrement; \' does not open a SQ region).
		if !inDQ && c == '\\' && i+1 < n {
			flushWord()
			wordBuf.Reset()
			body.WriteRune(c)
			body.WriteRune(runes[i+1])
			i++
			atWordBound = false
			wordStartedAtBound = false
			continue
		}

		// Double-quoted region: $() and backticks expand, but ')' and '('
		// have no structural significance for depth counting.
		if inDQ {
			body.WriteRune(c)
			if c == '\\' && i+1 < n {
				body.WriteRune(runes[i+1])
				i++
			} else if c == '"' {
				inDQ = false
			}
			atWordBound = false
			continue
		}

		// Not in any quoted region.

		// '#' at word boundary: comment — skip to end of line.
		if c == '#' && atWordBound {
			flushWord()
			body.WriteRune(c)
			for i+1 < n && runes[i+1] != '\n' {
				i++
				body.WriteRune(runes[i])
			}
			atWordBound = true
			wordStartedAtBound = true
			continue
		}

		switch c {
		case ' ', '\t', '\n':
			flushWord()
			body.WriteRune(c)
			atWordBound = true
			wordStartedAtBound = true

		case ';':
			flushWord()
			// Detect ;;&, ;;, ;& — case clause terminators.
			if i+1 < n && runes[i+1] == ';' {
				body.WriteRune(';')
				body.WriteRune(';')
				i++
				// Optional trailing & makes it ;;& (fall-through + re-test).
				if i+1 < n && runes[i+1] == '&' {
					body.WriteRune('&')
					i++
				}
				// Transition innermost case body back to pattern position.
				if len(caseStack) > 0 && caseStack[len(caseStack)-1] == 2 {
					caseStack[len(caseStack)-1] = 1
				}
			} else if i+1 < n && runes[i+1] == '&' {
				// ;& fall-through terminator: also back to pattern position.
				body.WriteRune(';')
				body.WriteRune('&')
				i++
				if len(caseStack) > 0 && caseStack[len(caseStack)-1] == 2 {
					caseStack[len(caseStack)-1] = 1
				}
			} else {
				body.WriteRune(c)
			}
			atWordBound = true
			wordStartedAtBound = true

		case '|', '&':
			flushWord()
			body.WriteRune(c)
			atWordBound = true
			wordStartedAtBound = true

		case '(':
			flushWord()
			depth++
			body.WriteRune(c)
			// If at pattern-start (state 1), the '(' is the optional pattern-open paren.
			// The matching ')' will decrement depth without reaching zero, so no
			// interception is needed.  Transition to state 2 so ')' is handled normally.
			if len(caseStack) > 0 && caseStack[len(caseStack)-1] == 1 {
				caseStack[len(caseStack)-1] = 2
			}
			atWordBound = true
			wordStartedAtBound = true

		case ')':
			flushWord()
			top := -1
			if len(caseStack) > 0 {
				top = caseStack[len(caseStack)-1]
			}
			if top == 1 {
				// Pattern terminator: do NOT decrement depth.
				caseStack[len(caseStack)-1] = 2
				body.WriteRune(c)
				atWordBound = true
				wordStartedAtBound = true
				continue
			}
			// Normal depth tracking.
			depth--
			if depth == 0 {
				return body.String(), i
			}
			body.WriteRune(c)
			atWordBound = true
			wordStartedAtBound = true

		case '\'':
			flushWord()
			inSQ = true
			body.WriteRune(c)
			atWordBound = false
			wordStartedAtBound = false

		case '"':
			flushWord()
			inDQ = true
			body.WriteRune(c)
			atWordBound = false
			wordStartedAtBound = false

		default:
			// Regular word character: accumulate for keyword detection.
			if wordBuf.Len() == 0 {
				wordStartedAtBound = atWordBound
			}
			wordBuf.WriteRune(c)
			body.WriteRune(c)
			atWordBound = false
		}
	}
	// Unbalanced: return the body collected so far (fail-closed) so the caller
	// can still analyse the content for blocked subcommands.
	if n > 0 && body.Len() > 0 {
		return body.String(), n - 1
	}
	return "", -1
}

// extractBacktickSubstBody extracts the body of a `…` substitution starting
// at position start (immediately after the opening backtick). Returns (body,
// index-of-closing-backtick) or ("", -1) if no closing backtick is found.
// C7 — D4 Wave 2 (REQ-2026-11-05, ML-2C).
func extractBacktickSubstBody(runes []rune, start int) (string, int) {
	var body strings.Builder
	n := len(runes)
	for i := start; i < n; i++ {
		c := runes[i]
		if c == '`' {
			return body.String(), i
		}
		if c == '\\' && i+1 < n && runes[i+1] == '`' {
			// \` inside backtick substitution → literal backtick in body.
			body.WriteRune('`')
			i++
			continue
		}
		body.WriteRune(c)
	}
	return "", -1 // unbalanced
}

// extractRawCmdSubstBodies scans the raw command string s and returns all
// command- and process-substitution bodies ($(...), `...`, <(...), >(...))
// that occur outside single-quoted regions, with comment awareness.
//
// This is called on the FULL raw string BEFORE any splitting into segments,
// so it correctly extracts bodies that contain '|', '&&', '||' — operators
// that quoteAwareSplitPOSIX would otherwise use to split the string before
// the body could be seen intact. For example, $(git push | cat) is extracted
// correctly here; the per-segment approach would see $(git push truncated at '|'.
//
// Rules:
//   - Single-quoted regions are entirely literal (no extraction).
//   - Inside "…", $(...) and `...` still expand and are extracted;
//     <(...)/>(...) do NOT expand inside "…" in any shell, so they are
//     only extracted outside double-quoted regions.
//   - '#' at a word boundary (after whitespace, separator, or at the start of
//     the string) outside quotes starts a comment; the rest of the line is
//     skipped, preventing $(...) inside comments from being extracted.
//
// ML-2G (REQ-2026-11-05). Replaces the per-segment extractCmdSubstBodies
// call that was done after quoteAwareSplitPOSIX.
func extractRawCmdSubstBodies(s string) []string {
	var bodies []string
	runes := []rune(s)
	n := len(runes)
	inSQ := false
	inDQ := false
	atWordBound := true // true at start and after whitespace or operator chars

	for i := 0; i < n; i++ {
		c := runes[i]

		// Single-quoted region: everything is literal.
		if inSQ {
			if c == '\'' {
				inSQ = false
			}
			atWordBound = false
			continue
		}

		// Outside any quote context: backslash escapes the next character.
		// In POSIX shell \' and \" are literal characters, not quote openers:
		//   echo \'$(git push)\'  — the \' is a plain quote, NOT a SQ delimiter
		//   cat \" <(git push)   — the \" is a plain quote, NOT a DQ delimiter
		// Without this, the scanner would treat \'…\' as a SQ region and skip the
		// $() body inside, returning a false rc=0. ML-2G (REQ-2026-11-05).
		if !inDQ && c == '\\' && i+1 < n {
			i++ // skip the escaped character
			atWordBound = false
			continue
		}

		// Enter single-quoted region (outside "…").
		if c == '\'' && !inDQ {
			inSQ = true
			atWordBound = false
			continue
		}

		// Inside double-quoted region.
		if inDQ {
			if c == '\\' && i+1 < n {
				i++ // skip escaped char; prevents \$( from triggering extraction
				atWordBound = false
				continue
			}
			if c == '"' {
				inDQ = false
				atWordBound = false
				continue
			}
			// Fall through to $(...) and `...` checks below — both expand inside "…".
		} else {
			// Outside any quotes.
			if c == '"' {
				inDQ = true
				atWordBound = false
				continue
			}

			// '#' at word boundary: comment — skip to end of line.
			// This prevents $(...) inside comments from being extracted.
			if c == '#' && atWordBound {
				for i+1 < n && runes[i+1] != '\n' {
					i++
				}
				// The newline (if any) is processed next and sets atWordBound=true.
				continue
			}

			// Process substitution: <(...) and >(...).
			// Bash-only; does not expand inside "…", so extracted only here.
			if (c == '<' || c == '>') && i+1 < n && runes[i+1] == '(' {
				body, end := extractBalancedParenBody(runes, i+2)
				if end >= 0 {
					bodies = append(bodies, body)
					i = end
				}
				atWordBound = false
				continue
			}
		}

		// Command substitution: $(...) — active inside and outside "…".
		if c == '$' && i+1 < n && runes[i+1] == '(' {
			body, end := extractBalancedParenBody(runes, i+2)
			if end >= 0 {
				bodies = append(bodies, body)
				i = end
			}
			atWordBound = false
			continue
		}

		// Command substitution: `...` — active inside and outside "…".
		if c == '`' {
			body, end := extractBacktickSubstBody(runes, i+1)
			if end >= 0 {
				bodies = append(bodies, body)
				i = end
			}
			atWordBound = false
			continue
		}

		// Track word boundaries for '#' comment detection.
		switch c {
		case ' ', '\t', '\n', ';', '|', '&', '(', ')':
			atWordBound = true
		default:
			atWordBound = false
		}
	}
	return bodies
}

// matchSubcommandPOSIX is the POSIX-aware variant of MatchSubcommand. It uses
// quoteAwareSplitPOSIX / shellTokenizePOSIX / stripShellSuffixPOSIX instead of
// their non-POSIX counterparts, and additionally implements D4: when the first
// token (after prefix stripping) is a recognised shell interpreter invoked with
// a -c flag, it extracts the command string and recurses up to depth 2.
//
// depth 0 = called from MatchSubcommand (top level).
// depth 1 = one level of shell wrapping (e.g. bash -c "git push").
// depth 2 = two levels (e.g. bash -c "bash -c 'git push'").
// depth > 2 = recursion guard: returns "wrapper-not-analyzed" (fail-closed).
func matchSubcommandPOSIX(cmdRaw string, depth int) string {
	if depth > maxWrapperDepth {
		return "wrapper-not-analyzed"
	}

	normalized := stripHeredocBodies(cmdRaw)

	// ML-2G: scan the full normalized string BEFORE splitting into segments.
	// quoteAwareSplitPOSIX splits on '|', '&&', '||' — operators that also
	// appear inside $(...) bodies. By scanning the raw string first, we extract
	// bodies like $(git push | cat) in one piece, then recurse at depth+1.
	// '#' comments are handled inside extractRawCmdSubstBodies: a # at word
	// boundary skips the rest of the line, so $(git push) inside a comment
	// is not extracted. (REQ-2026-11-05, ML-2G).
	for _, body := range extractRawCmdSubstBodies(normalized) {
		if tag := matchSubcommandPOSIX(body, depth+1); tag != "" {
			return tag
		}
	}

	segments := quoteAwareSplitPOSIX(normalized)

	for _, seg := range segments {
		seg = strings.TrimLeft(seg, " \t")
		if len(seg) > 0 && seg[0] == '&' {
			seg = strings.TrimLeft(seg[1:], " \t")
		}

		seg = stripShellSuffixPOSIX(seg)
		if seg == "" {
			continue
		}

		tokens := shellTokenizePOSIX(seg)
		if len(tokens) == 0 {
			continue
		}

		tokens = stripEnvCommandPrefix(tokens)
		if len(tokens) == 0 {
			continue
		}

		base := strings.TrimSuffix(strings.ToLower(filepath.Base(tokens[0])), ".exe")

		// D4: recognised shell interpreter — recurse into the command string.
		if shellInterpreter(base) {
			if tag := handleShellWrapper(base, tokens[1:], depth); tag != "" {
				return tag
			}
			continue // base is a shell interpreter, not git
		}

		// Git guard (same logic as MatchSubcommand).
		if base != "git" {
			continue
		}
		tokens = tokens[1:]
		sub, rest := findSubcommand(tokens)
		if sub == "" {
			continue
		}
		if tag := applyRule(sub, rest); tag != "" {
			return tag
		}
	}
	return ""
}

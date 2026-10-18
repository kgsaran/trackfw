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
	reasonTruncated = "trackfw git-branch-guard: RECUSADO — o comando nao pode ser lido por inteiro do stdin (nenhum byte chegou em 2s de espera ociosa) e nao ha comando em argv. O guard recusa em vez de aprovar um comando que nao conseguiu ler. Reenvie o comando, ou use trackfw ship / trackfw branch new."

	reasonNUL = "trackfw git-branch-guard: RECUSADO — o comando contem NUL (\\u0000) e nao pode ser interpretado com seguranca pelo shell. O guard recusa em vez de executar um comando corrompido."

	reasonIndecodeable = "trackfw git-branch-guard: RECUSADO — o extrator JSON (sem jq) encontrou erro ao decodificar o campo de comando: string nao terminada, escape invalido ou NUL. O guard recusa em vez de aprovar um payload que nao pode ser lido com seguranca."

	reasonCheckoutB = "trackfw: git checkout -b bruto bloqueado. Use `trackfw branch new <type>/<slug>`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonSwitchC = "trackfw: git switch -c bruto bloqueado. Use `trackfw branch new <type>/<slug>`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonBranchCreate = "trackfw: git branch bruto bloqueado. Use `trackfw branch new <type>/<slug>`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonWorktreeAddB = "trackfw: git worktree add -b bruto bloqueado. Use `trackfw branch new <type>/<slug>`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonCommit = "trackfw: git commit bruto bloqueado. Use `trackfw commit -m '<mensagem>'`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonPush = "trackfw: git push bruto bloqueado. Use `trackfw push` (para empurrar commits já criados), `trackfw ship` (para commit+push+PR em uma etapa) ou `trackfw release tag` (para publicar uma tag de release). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	// reasonPushDelete is used when the push command contains a branch-deletion
	// form (--delete/-d <ref> or :<ref>/+:<ref> where <ref> is not a tag).
	// It points to the governed path that deletes remote branches safely.
	reasonPushDelete = "trackfw: git push bruto de remoção de branch bloqueado. Use `trackfw branch prune --remote --apply` para apagar no origin apenas a branch integrada por PR mergeado, condicionada ao tip avaliado. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonStash = "trackfw: git stash bruto bloqueado — worktree compartilhado entre subagentes, um stash remove as alterações não commitadas de todos os outros. `git stash list`/`git stash show` seguem liberados; para guardar trabalho em progresso, use uma branch própria via `trackfw branch new` e commit nela. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonResetHard = "trackfw: git reset --hard bruto bloqueado — descarta de forma irreversível as alterações não commitadas de todo o worktree compartilhado. `git reset --soft`/`--mixed` seguem liberados (ex.: `git reset --soft HEAD~1` é o caminho padrão; use `trackfw ship -m '...'` para commitar e empurrar). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	// reasonAddImplicit is emitted when `git add` or `git stage` is invoked with
	// implicit scope (no named path, or only whole-tree roots such as . / .. / :/
	// / :(top)). ADR-2026-09-05 D1, D3: staging without enumeration is blocked;
	// the message must teach the valid form. Single-line, JSON-safe (no raw
	// double-quote or backslash). R-6 (Wave 0, 2026-10-17).
	reasonAddImplicit = "trackfw: git add/stage com escopo implicito bloqueado — enumerate antes o que vai estagiar com `git status --short`, depois estagiar por caminho explicito: `git add <arquivo1> <arquivo2> ...` e commitar com `trackfw commit -m '...'`. Nada antes deste comando foi executado (comando composto e bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonCleanForce = "trackfw: git clean -f/-x bruto bloqueado — apaga arquivos não rastreados do worktree compartilhado, de forma irreversível. `git clean -n`/`--dry-run` segue liberado para revisar antes o que seria apagado. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonRestorePath = "trackfw: git restore <path> bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. `git restore --staged` (não toca o working tree) segue liberado; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonCheckoutPath = "trackfw: git checkout -- <path>/git checkout . bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. `git checkout <branch>`/`git switch <branch>` seguem liberados; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonUpdateRef = "trackfw: git update-ref bruto bloqueado — reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o objeto apontado nem exigir push, o que permite forjar o commit-alvo que `trackfw release tag` publicaria. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonWorktreeRemoveForce = "trackfw: git worktree remove -f/--force bruto bloqueado — remove um worktree e descarta de forma irreversível qualquer alteração não commitada nele. `git worktree remove` sem force segue liberado (recusa sozinho quando há algo não commitado). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonRmForce = "trackfw: git rm -f/--force bruto bloqueado — apaga arquivos do working tree e do index de forma irreversível, mesma classe de `git clean -f`/`git reset --hard` já bloqueados. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	// reasonWorktreeAddImplicit is emitted when `git worktree add <path>` is invoked
	// without any branch-selection flag (-b/-B/--detach/-d/--orphan) and without a
	// commit-ish as a second positional argument. In that form git creates an implicit
	// branch whose name is derived from <path>, bypassing the governance gate that
	// `trackfw branch new` enforces. Blocked by ML-1C (REQ-2026-09-12, ROADMAP-2026-09-12).
	reasonWorktreeAddImplicit = "trackfw: git worktree add <path> sem flag de branch bloqueado — esta forma cria uma branch implícita derivada do nome do caminho, contornando a governança. Use `git worktree add <path> <branch-existente>` (para branch já criada) ou `trackfw branch new <type>/<slug>` + `git worktree add <path> <nova-branch>`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
)

// StdInIdleTimeout is the idle window used by DrainStdin in production.
// Exported so the cobra command in internal/commands can pass it to RunGitBranch.
const StdInIdleTimeout = 2 * time.Second

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
	return ""
}

// stripEnvCommandPrefix removes a leading chain of "env [KEY=val...] " or
// "command " prefixes from tokens, mirroring the bash while loop.
func stripEnvCommandPrefix(tokens []string) []string {
	for len(tokens) > 0 {
		base := filepath.Base(tokens[0])
		if base == "env" {
			tokens = tokens[1:] // skip "env"
			// Skip KEY=value pairs (and stop at flags or non-assignments).
			for len(tokens) > 0 {
				t := tokens[0]
				if strings.HasPrefix(t, "-") {
					break // flag: stop
				}
				if strings.Contains(t, "=") {
					tokens = tokens[1:] // KEY=value: skip
				} else {
					break // not an assignment: stop
				}
			}
		} else if base == "command" {
			tokens = tokens[1:] // skip "command"
		} else if base == "sudo" {
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
				// Flags that consume a following argument — both short and long forms.
				// Long --x=v forms are self-contained (consumed above as the flag token).
				// Long --x v forms require consuming the next token as the value.
				switch t {
				case "-u", "-g", "-C", "-D", "-T", "-U", "-h", "-p", "-r", "-t",
					"--user", "--group", "--chdir", "--close-from", "--host",
					"--prompt", "--role", "--type", "--other-user":
					if len(tokens) > 0 {
						tokens = tokens[1:]
					}
				}
			}
		} else {
			break
		}
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

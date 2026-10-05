package guard

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
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

	reasonStash = "trackfw: git stash bruto bloqueado — worktree compartilhado entre subagentes, um stash remove as alterações não commitadas de todos os outros. `git stash list`/`git stash show` seguem liberados; para guardar trabalho em progresso, use uma branch própria via `trackfw branch new` e commit nela. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonResetHard = "trackfw: git reset --hard bruto bloqueado — descarta de forma irreversível as alterações não commitadas de todo o worktree compartilhado. `git reset --soft`/`--mixed` seguem liberados (ex.: `git reset --soft HEAD~1` é o caminho padrão; use `trackfw ship -m \"...\"` para commitar e empurrar). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonCleanForce = "trackfw: git clean -f/-x bruto bloqueado — apaga arquivos não rastreados do worktree compartilhado, de forma irreversível. `git clean -n`/`--dry-run` segue liberado para revisar antes o que seria apagado. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonRestorePath = "trackfw: git restore <path> bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. `git restore --staged` (não toca o working tree) segue liberado; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonCheckoutPath = "trackfw: git checkout -- <path>/git checkout . bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. `git checkout <branch>`/`git switch <branch>` seguem liberados; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonUpdateRef = "trackfw: git update-ref bruto bloqueado — reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o objeto apontado nem exigir push, o que permite forjar o commit-alvo que `trackfw release tag` publicaria. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonWorktreeRemoveForce = "trackfw: git worktree remove -f/--force bruto bloqueado — remove um worktree e descarta de forma irreversível qualquer alteração não commitada nele. `git worktree remove` sem force segue liberado (recusa sozinho quando há algo não commitado). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."

	reasonRmForce = "trackfw: git rm -f/--force bruto bloqueado — apaga arquivos do working tree e do index de forma irreversível, mesma classe de `git clean -f`/`git reset --hard` já bloqueados. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
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

// MatchSubcommand applies the guard rules to cmdRaw and returns the subcommand
// tag if the command should be blocked, or "" to allow.
// It is a faithful Go port of the bash match_subcommand() function.
func MatchSubcommand(cmdRaw string) string {
	normalized := stripHeredocBodies(cmdRaw)
	segments := quoteAwareSplit(normalized)

	for _, seg := range segments {
		seg = strings.TrimLeft(seg, " \t")
		if seg == "" {
			continue
		}
		tokens := strings.Fields(seg)
		if len(tokens) == 0 {
			continue
		}

		// Strip env/command prefix (mirrors bash while loop lines 430–452).
		tokens = stripEnvCommandPrefix(tokens)
		if len(tokens) == 0 {
			continue
		}

		// First token must be "git" (by basename, so /usr/bin/git also works).
		base := filepath.Base(tokens[0])
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
	}

	return "" // unknown subcommand: allow
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

// quoteAwareSplit splits cmdRaw on `;`, `&&`, `||`, `|` outside of single or
// double quotes. Newlines inside quotes become spaces. The result is a list
// of command segments. Mirrors the bash quote_aware_split() awk function
// (lines 347–402).
func quoteAwareSplit(s string) []string {
	var segments []string
	var cur strings.Builder
	runes := []rune(s)
	n := len(runes)
	var q rune // current quote char (0 = unquoted)

	for i := 0; i < n; i++ {
		c := runes[i]

		if q != 0 {
			// Inside quotes.
			if q == '"' && c == '\\' && i+1 < n {
				next := runes[i+1]
				if next == '\n' {
					cur.WriteRune(' ')
				} else {
					cur.WriteRune(next)
				}
				i++
				continue
			}
			if c == q {
				q = 0
				cur.WriteRune(c)
				continue
			}
			if c == '\n' {
				cur.WriteRune(' ')
			} else {
				cur.WriteRune(c)
			}
			continue
		}

		// Outside quotes.
		switch {
		case c == '"' || c == '\'':
			q = c
			cur.WriteRune(c)
		case i+1 < n && c == '&' && runes[i+1] == '&':
			segments = append(segments, cur.String())
			cur.Reset()
			i++ // skip second '&'
		case i+1 < n && c == '|' && runes[i+1] == '|':
			segments = append(segments, cur.String())
			cur.Reset()
			i++ // skip second '|'
		case c == ';' || c == '|':
			segments = append(segments, cur.String())
			cur.Reset()
		case c == '\n':
			// Newline outside quotes acts as a segment separator, matching the bash
			// quote_aware_split awk function which passes \n through unchanged and
			// the caller iterates with `while IFS= read -r seg`.
			segments = append(segments, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(c)
		}
	}

	segments = append(segments, cur.String())
	return segments
}

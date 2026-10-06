// Package guard — credential subcommand.
// Faithful Go port of scripts/trackfw-credential-guard.sh (152 lines).
// Project scope: RunCredential. Global scope: RunCredentialGlobal (--global flag).
package guard

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

var (
	// credJWTRe matches the JWT_PATTERN from the .sh.
	credJWTRe = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)

	// credAWSRe matches the AWS_KEY_PATTERN from the .sh.
	credAWSRe = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)

	// credRedirectRe extracts redirect targets from RAW.
	// Faithful port of grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+'.
	// Uses [ \t\r\n] for [[:space:]] — equivalent because GNU grep stops a match at newline.
	credRedirectRe = regexp.MustCompile(`[0-9]?>>?[ \t]*[^ \t\r\n|&;,:]+`)

	// credRedirectPfxRe strips the redirect operator prefix from a match.
	credRedirectPfxRe = regexp.MustCompile(`^[0-9]?>>?[ \t]*`)

	// credVarRefRe matches a shell variable reference like $TMPFILE or ${TMPFILE}.
	credVarRefRe = regexp.MustCompile(`^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$`)

	// credCmdLineRe extracts the "command" JSON field value from a line.
	// Faithful port of sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'.
	credCmdLineRe = regexp.MustCompile(`"command"[ \t]*:[ \t]*"([^"]*)"`)
)

const credMaxFileSize = 1048576 // 1 MiB

// RunCredential implements the credential guard logic. Entry point for
// `trackfw guard credential`. Faithful Go port of
// scripts/trackfw-credential-guard.sh (project scope).
//
// Order of operations:
//  1. Read all stdin (no timeout — residual declared in ADR D9).
//  2. Check trackfw.yaml in cwd only (no walk-up, per ADR D9 and .sh L8).
//  3. Layer 1: scan payload for JWT then AWS key.
//  4. Unescape \" → " to get RAW.
//  5. Extract REDIRECTS from RAW.
//  6. Layer 2: scan non-ephemeral redirect files and command file args.
//  7. No match → return 0.
//  8. Exemption: all redirects are ephemeral → return 0.
//  9. Read credential_guard.mode (default "warn").
//  10. block → stderr + return 2; warn → stderr + write attention JSON + return 0.
//
// Returns 0 to allow, 2 to block. The caller MUST exit with this code.
func RunCredential(stdin io.Reader, stdout, stderr io.Writer) int {
	// Step 1: Read all stdin — INPUT=$(cat); no timeout (ADR D9 residual).
	data, _ := io.ReadAll(stdin)

	// Step 2: Check for trackfw.yaml in cwd only (no walk-up; .sh L8).
	cwd, err := os.Getwd()
	if err != nil {
		return 0
	}
	if _, err := os.Stat(filepath.Join(cwd, "trackfw.yaml")); err != nil {
		return 0 // not inside a trackfw project
	}

	// Step 3: Layer 1 — JWT (first), then AWS key.
	inputStr := string(data)
	match := ""
	if credJWTRe.MatchString(inputStr) {
		match = "JWT"
	} else if credAWSRe.MatchString(inputStr) {
		match = "AWS access key"
	}

	// Step 4: Unescape \" → " (RAW variable in .sh).
	rawBytes := bytes.ReplaceAll(data, []byte(`\"`), []byte(`"`))
	rawStr := string(rawBytes)

	// Step 5: Extract redirect matches.
	redirectMatches := credRedirectRe.FindAllString(rawStr, -1)

	// Step 6: Layer 2 — scan files when layer 1 found nothing.
	if match == "" {
		match = credSecondLayer(rawStr, redirectMatches)
	}

	// Step 7: No match → allow.
	if match == "" {
		return 0
	}

	// Step 8: Exemption — all redirects point to ephemeral destinations.
	if credIsAllEphemeral(rawStr, redirectMatches) {
		return 0
	}

	// Step 9: Read credential_guard.mode from trackfw.yaml (default "warn").
	mode := credReadModeWithDefault(filepath.Join(cwd, "trackfw.yaml"), "warn")

	// Step 10: Block or warn.
	if mode == "block" {
		fmt.Fprintf(stderr, "trackfw-credential-guard: blocked - possible %s detected in tool payload.\n", match)
		return 2
	}

	fmt.Fprintf(stderr, "trackfw-credential-guard: warning - possible %s detected in tool payload.\n", match)
	roadmapDir := credReadRoadmapDir(filepath.Join(cwd, "trackfw.yaml"))
	credWriteAttention(cwd, roadmapDir, match)

	return 0
}

// RunCredentialGlobal implements the credential guard logic for global scope.
// Entry point for `trackfw guard credential --global`.
// Faithful Go port of globalCredentialGuardScript (credentialGuardGlobalTail):
//   - no trackfw.yaml guard: runs in any directory.
//   - DEFAULT_MODE = "block" (vs "warn" in project scope).
//   - ROADMAP_DIR = "docs/roadmaps" (fixed, relative to cwd).
//   - Writes attention JSON only if docs/roadmaps already exists as a directory (no mkdir).
//   - If trackfw.yaml exists in cwd with credential_guard.mode = "warn"|"block", that value
//     takes precedence.
//
// Returns 0 to allow, 2 to block. The caller MUST exit with this code.
func RunCredentialGlobal(stdin io.Reader, stdout, stderr io.Writer) int {
	// Step 1: Read all stdin.
	data, _ := io.ReadAll(stdin)

	// Step 2: No trackfw.yaml guard — global scope runs in any directory.

	// Step 3: Layer 1 — JWT (first), then AWS key.
	inputStr := string(data)
	match := ""
	if credJWTRe.MatchString(inputStr) {
		match = "JWT"
	} else if credAWSRe.MatchString(inputStr) {
		match = "AWS access key"
	}

	// Step 4: Unescape \" → " (RAW variable in .sh).
	rawBytes := bytes.ReplaceAll(data, []byte(`\"`), []byte(`"`))
	rawStr := string(rawBytes)

	// Step 5: Extract redirect matches.
	redirectMatches := credRedirectRe.FindAllString(rawStr, -1)

	// Step 6: Layer 2 — scan files when layer 1 found nothing.
	if match == "" {
		match = credSecondLayer(rawStr, redirectMatches)
	}

	// Step 7: No match → allow.
	if match == "" {
		return 0
	}

	// Step 8: Exemption — all redirects point to ephemeral destinations.
	if credIsAllEphemeral(rawStr, redirectMatches) {
		return 0
	}

	// Step 9: Read credential_guard.mode; default is "block" (global scope).
	// Faithful port of credentialGuardGlobalTail: reads from trackfw.yaml in cwd if it
	// exists; falls back to "block" when the file is absent or mode is unrecognised.
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	mode := credReadModeWithDefault(filepath.Join(cwd, "trackfw.yaml"), "block")

	// Step 10: Block or warn.
	if mode == "block" {
		fmt.Fprintf(stderr, "trackfw-credential-guard: blocked - possible %s detected in tool payload.\n", match)
		return 2
	}

	fmt.Fprintf(stderr, "trackfw-credential-guard: warning - possible %s detected in tool payload.\n", match)

	// Global scope: write attention JSON only if docs/roadmaps already exists as a directory.
	// Faithful port of: ROADMAP_DIR="docs/roadmaps"; if [ ! -d "$ROADMAP_DIR" ]; then exit 0; fi
	const roadmapDir = "docs/roadmaps"
	fi, statErr := os.Stat(roadmapDir)
	if statErr != nil || !fi.IsDir() {
		return 0
	}
	credWriteAttention(cwd, roadmapDir, match)

	return 0
}

// credGlobToken expands a single command-line token when it contains a glob metacharacter.
// Faithful port of bash `set -- $CMD_LINE` (without set -f) glob expansion:
//   - POSIX `[!...]` negation is translated to Go `[^...]`.
//   - `*` and `?` do not match a leading dot (bash default, no dotglob): matches whose
//     filename component starts with '.' are dropped when the pattern base does not start
//     with '.'.
//   - Malformed pattern (ErrBadPattern, e.g. an unclosed `[`) → keep token literal.
//   - No matches → keep token literal (bash without nullglob).
//
// Declared residual: POSIX character classes such as [[:alpha:]] are not supported by
// Go's filepath.Glob and return ErrBadPattern; such patterns keep the literal token.
//
// Returns a slice of >= 1 element (always at least the original token).
func credGlobToken(token string) []string {
	if !strings.ContainsAny(token, "*?[") {
		return []string{token}
	}
	// Translate POSIX [! negation to Go [^ negation.
	pattern := strings.ReplaceAll(token, "[!", "[^")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return []string{token}
	}
	// Bash without dotglob: filter hidden files unless the pattern base starts with '.'.
	patternBase := filepath.Base(pattern)
	if !strings.HasPrefix(patternBase, ".") {
		filtered := matches[:0]
		for _, m := range matches {
			if !strings.HasPrefix(filepath.Base(m), ".") {
				filtered = append(filtered, m)
			}
		}
		matches = filtered
	}
	if len(matches) == 0 {
		return []string{token}
	}
	return matches
}

// credIsEphemeralTarget reports whether target is an ephemeral redirect destination.
// Faithful port of is_ephemeral_target() in .sh L30–46.
//
// rawStr is the unescaped payload (RAW in the .sh) used for the variable-ref mktemp check.
func credIsEphemeralTarget(rawStr, target string) bool {
	// tr -d "\"'"
	target = strings.Map(func(r rune) rune {
		if r == '"' || r == '\'' {
			return -1
		}
		return r
	}, target)
	// sed -E 's/[},]+$//'
	target = strings.TrimRight(target, "},")

	if target == "/dev/null" {
		return true
	}
	if strings.Contains(target, "mktemp") {
		return true
	}

	// Variable reference — check if var was assigned via $(mktemp ...
	m := credVarRefRe.FindStringSubmatch(target)
	if m != nil {
		varname := m[1]
		// Bash glob equivalent: case "$RAW" in *${varname}=$(mktemp*) ... esac
		if strings.Contains(rawStr, varname+"=$(mktemp") {
			return true
		}
	}

	return false
}

// credRedirectTarget extracts the path portion from a redirect match.
// Faithful port of: target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
func credRedirectTarget(match string) string {
	return credRedirectPfxRe.ReplaceAllString(match, "")
}

// credScanFile reads the file at path and returns "JWT", "AWS access key", or ""
// if no pattern found, file absent, or file too large.
// Faithful port of scan_file_for_pattern() in .sh L53–69.
func credScanFile(path string) string {
	// tr -d "\"'" | sed -E 's/[},]+$//'
	path = strings.Map(func(r rune) rune {
		if r == '"' || r == '\'' {
			return -1
		}
		return r
	}, path)
	path = strings.TrimRight(path, "},")

	if path == "" {
		return ""
	}
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return ""
	}
	if fi.Size() >= credMaxFileSize {
		return ""
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	cs := string(content)
	if credJWTRe.MatchString(cs) {
		return "JWT"
	}
	if credAWSRe.MatchString(cs) {
		return "AWS access key"
	}
	return ""
}

// credSecondLayer runs layer 2 detection when layer 1 found no match.
// Faithful port of .sh L71–97:
//   - 2a: scan non-ephemeral redirect target files.
//   - 2b: extract CMD_LINE, expand tokens via credGlobToken (porting bash `set -- $CMD_LINE`
//     glob expansion, including POSIX [!] negation and dotfile filtering), and scan file
//     arguments of cat/head/tail/jq/grep.
func credSecondLayer(rawStr string, redirectMatches []string) string {
	// 2a — scan non-ephemeral redirect targets.
	for _, rm := range redirectMatches {
		if rm == "" {
			continue
		}
		target := credRedirectTarget(rm)
		if target == "" {
			continue
		}
		if !credIsEphemeralTarget(rawStr, target) {
			if m := credScanFile(target); m != "" {
				return m
			}
		}
	}

	// 2b — extract command value from JSON, glob-expand each token (porting
	// `set -- $CMD_LINE` without set -f), and scan file arguments.
	matches := credCmdLineRe.FindAllStringSubmatch(rawStr, -1)
	for _, cm := range matches {
		cmdLine := cm[1]
		rawTokens := strings.Fields(cmdLine)
		if len(rawTokens) == 0 {
			continue
		}
		// Apply glob expansion per token, matching bash `set -- $CMD_LINE`.
		var tokens []string
		for _, t := range rawTokens {
			tokens = append(tokens, credGlobToken(t)...)
		}
		if len(tokens) == 0 {
			continue
		}
		switch tokens[0] {
		case "cat", "head", "tail", "jq", "grep":
			for _, tok := range tokens[1:] {
				if m := credScanFile(tok); m != "" {
					return m
				}
			}
		}
	}

	return ""
}

// credIsAllEphemeral reports true when all redirect targets are ephemeral.
// Faithful port of .sh L101–118 (HAS_REDIRECT + EXEMPT logic).
func credIsAllEphemeral(rawStr string, redirectMatches []string) bool {
	hasRedirect := false
	allEphemeral := true

	for _, rm := range redirectMatches {
		if rm == "" {
			continue
		}
		hasRedirect = true
		target := credRedirectTarget(rm)
		if !credIsEphemeralTarget(rawStr, target) {
			allEphemeral = false
		}
	}

	return hasRedirect && allEphemeral
}

// credReadMode reads credential_guard.mode from trackfw.yaml with default "warn".
// Project scope wrapper; see credReadModeWithDefault for the shared logic.
func credReadMode(yamlPath string) string {
	return credReadModeWithDefault(yamlPath, "warn")
}

// credReadModeWithDefault reads credential_guard.mode from trackfw.yaml.
// Returns defaultMode when the file is absent, unreadable, or the mode value is not
// "warn" or "block".
//
// The sed pipeline does NOT strip \r from the mode value; a line ending in
// "block\r" (CRLF file) does not match "block" and falls through to the
// defaultMode. This is the measured .sh behaviour (CRLF residual).
func credReadModeWithDefault(yamlPath, defaultMode string) string {
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return defaultMode
	}

	lines := strings.Split(string(data), "\n")

	// Collect the 5 lines after each "^credential_guard:" line (grep -A 5).
	var candidates []string
	for i, line := range lines {
		if strings.HasPrefix(line, "credential_guard:") {
			end := i + 6 // i+1 through i+5 inclusive (5 lines after)
			if end > len(lines) {
				end = len(lines)
			}
			candidates = append(candidates, lines[i+1:end]...)
		}
	}

	// First candidate line containing "mode:" (grep 'mode:' | head -1).
	for _, line := range candidates {
		if !strings.Contains(line, "mode:") {
			continue
		}
		idx := strings.Index(line, "mode:")
		val := line[idx+len("mode:"):]
		// Strip leading whitespace: sed -E 's/^[[:space:]]*mode:[[:space:]]*//'
		val = strings.TrimLeft(val, " \t")
		// Strip trailing comment: sed -E 's/[[:space:]]*#.*$//'
		if ci := strings.Index(val, "#"); ci >= 0 {
			val = strings.TrimRight(val[:ci], " \t")
		}
		// tr -d "\"'" — do NOT strip \r (CRLF residual declared above)
		val = strings.Map(func(r rune) rune {
			if r == '"' || r == '\'' {
				return -1
			}
			return r
		}, val)

		switch val {
		case "warn", "block":
			return val
		}
		break // first "mode:" line processed; not warn|block → fall through to defaultMode
	}

	return defaultMode
}

// credReadRoadmapDir reads roadmap_dir from trackfw.yaml with CRLF normalisation
// and path safety validation.
// Faithful port of .sh L136–141.
func credReadRoadmapDir(yamlPath string) string {
	const fallback = "docs/roadmaps"

	data, err := os.ReadFile(yamlPath)
	if err != nil {
		return fallback
	}

	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "roadmap_dir:") {
			continue
		}
		val := line[len("roadmap_dir:"):]
		// sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//'
		val = strings.TrimLeft(val, " \t")
		if ci := strings.Index(val, "#"); ci >= 0 {
			val = strings.TrimRight(val[:ci], " \t")
		}
		// tr -d '"', tr -d "'", tr -d '\r'
		val = strings.Map(func(r rune) rune {
			if r == '"' || r == '\'' || r == '\r' {
				return -1
			}
			return r
		}, val)

		if val == "" {
			return fallback
		}

		// case "$ROADMAP_DIR" in /*|../*|*/../*|*/..|..) → fallback
		if strings.HasPrefix(val, "/") ||
			strings.HasPrefix(val, "../") ||
			strings.Contains(val, "/../") ||
			strings.HasSuffix(val, "/..") ||
			val == ".." {
			return fallback
		}

		return val
	}

	return fallback
}

// credWriteAttention writes the attention signal JSON to
// <roadmapDir>/.trackfw-credential-guard.json.
// Faithful port of .sh L143–151 (warn path).
//
// cwd is the process working directory; roadmapDir is the relative path read
// from trackfw.yaml (or the hardcoded fallback "docs/roadmaps"). The write is
// a best-effort side effect: any error (including containment refusal when
// roadmapDir is a symlink pointing outside the project) is silently ignored.
// The guard and rc are NOT affected.
//
// Hardening beyond the .sh: pathguard.RejectAndReport refuses a roadmapDir
// whose resolved path escapes the project root via a symlink — the .sh wrote
// to a relative path with no such check.
func credWriteAttention(cwd, roadmapDir, match string) {
	ts := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	msg := "Possible " + match + " detected in tool payload - review before materializing credentials in plain text."

	// MSG_ESC: tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g'
	var msgEsc strings.Builder
	for _, r := range msg {
		if r < 0x20 {
			continue
		}
		switch r {
		case '\\':
			msgEsc.WriteString(`\\`)
		case '"':
			msgEsc.WriteString(`\"`)
		default:
			msgEsc.WriteRune(r)
		}
	}

	content := fmt.Sprintf(
		`{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}`+"\n",
		msgEsc.String(), ts,
	)

	// Resolve the project root so that the containment guard operates in the
	// same namespace as the paths it walks. Silent on failure: the attention
	// write is a side effect and must not alter the guard's rc or output.
	root, err := pathguard.ResolveRoot(cwd)
	if err != nil {
		return
	}
	target := filepath.Join(root, roadmapDir, ".trackfw-credential-guard.json")
	// RejectAndReport dominates both writes below in this flow.
	// It prints "trackfw: refusing write to …" to os.Stderr if the path escapes
	// the root (e.g. roadmapDir is a symlink pointing outside the project).
	if err := pathguard.RejectAndReport(root, target); err != nil {
		return
	}
	// write-containment-allowed: pathguard.RejectAndReport(root, target) dominates this write
	_ = os.MkdirAll(filepath.Dir(target), 0755)
	// write-containment-allowed: pathguard.RejectAndReport(root, target) dominates this write
	_ = os.WriteFile(target, []byte(content), 0644)
}

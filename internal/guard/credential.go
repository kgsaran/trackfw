// Package guard — credential subcommand.
// Faithful Go port of scripts/trackfw-credential-guard.sh (152 lines).
// Project scope: RunCredential. Global scope: RunCredentialGlobal (--global flag).
package guard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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
	// Port of grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' — BUG-2 fix:
	// ':' removed from exclusion class so that Windows paths (C:\..., C:/...) are
	// extracted in full rather than truncated to the drive letter alone.
	// NTFS ADS (file.txt:stream) side-effect: extração completa; os.Stat fails
	// gracefully for unknown stream names — not a functional regression.
	credRedirectRe = regexp.MustCompile(`[0-9]?>>?[ \t]*[^ \t\r\n|&;,]+`)

	// credGitBashDriveRe matches a Git-Bash drive mount: /[a-zA-Z]/rest or /[a-zA-Z].
	// Group 1: drive letter; Group 2: /rest (may be absent for bare drive).
	// Does not match /tmp, /home, /cygdrive, /dev/null, //UNC, etc.
	credGitBashDriveRe = regexp.MustCompile(`^/([a-zA-Z])(/.*)?$`)

	// credCygdrivePrefixRe matches a Cygwin mount: /cygdrive/[a-zA-Z]/rest or /cygdrive/[a-zA-Z].
	// Group 1: drive letter; Group 2: /rest (may be absent).
	credCygdrivePrefixRe = regexp.MustCompile(`^/cygdrive/([a-zA-Z])(/.*)?$`)

	// credRedirectPfxRe strips the redirect operator prefix from a match.
	credRedirectPfxRe = regexp.MustCompile(`^[0-9]?>>?[ \t]*`)

	// credVarRefRe matches a shell variable reference like $TMPFILE or ${TMPFILE}.
	credVarRefRe = regexp.MustCompile(`^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$`)

	// credCmdLineRe extracts the "command" JSON field value from raw text.
	// Faithful port of sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'.
	// Used as a fallback when JSON parsing fails (e.g. unescaped backslashes in
	// Windows paths) to restore Layer 2b detection on the raw text.
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

	// Steps 4–8: Extract shell command, scan for redirects, run Layer 2, apply
	// ephemeral exemption.
	//
	// For valid JSON payloads the redirect scan and Layer 2b operate on the
	// decoded shell command (fixes R1a/R1e/EE4). For non-JSON payloads the legacy
	// raw-string path is preserved unchanged.
	shellCmd, shellCwd, toolName, isJSON := credExtractCmdAndCwd(data)
	var contextStr string
	var redirectMatches []string
	if isJSON {
		contextStr = shellCmd
		redirectMatches = credRedirectRe.FindAllString(shellCmd, -1)
	} else {
		rawBytes := bytes.ReplaceAll(data, []byte(`\"`), []byte(`"`))
		contextStr = string(rawBytes)
		redirectMatches = credRedirectRe.FindAllString(contextStr, -1)
	}

	if match == "" {
		match = credSecondLayer(shellCmd, shellCwd, contextStr, toolName, redirectMatches)
	}

	// Non-JSON fallback: restore Layer 2b via legacy credCmdLineRe extraction.
	// When JSON parsing fails (e.g. unescaped backslashes in Windows paths),
	// shellCmd is "" and credSecondLayer's Layer 2b is a no-op. credNonJSONLayerTwoB
	// extracts "command" field values from the raw text and runs Layer 2b on each,
	// matching main's credCmdLineRe behaviour. Does not cover "command_line" keys —
	// Windsurf/Amazon-Q payloads require valid JSON for full coverage.
	if match == "" && !isJSON {
		match = credNonJSONLayerTwoB(contextStr, "")
	}

	// F4: if Layer 1 + primary Layer 2 found nothing, deep-scan all command/command_line
	// values at any depth in the JSON payload (catches non-standard schemas like
	// {"params":{"command":"cat secret.txt"}} used by Cursor/Copilot/Kiro/Gemini).
	if match == "" && isJSON {
		match = credDeepScan(data, shellCwd)
	}

	if match == "" {
		return 0
	}

	// F2 shape-based ephemeral exemption: for JSON payloads, only exempt when:
	//   (a) the credential was found in the primary shell command itself,
	//   (b) there are no shell metacharacters (argv0 ∈ {echo, printf}),
	//   (c) all redirect targets are exactly "/dev/null".
	// For non-JSON payloads, apply the same 3-part predicate to each
	// regex-extracted "command" value (never to the raw contextStr as a whole,
	// because argv0 of the full raw payload is always '{').
	if isJSON {
		cmdHasMatch := (match == "JWT" && credJWTRe.MatchString(shellCmd)) ||
			(match == "AWS access key" && credAWSRe.MatchString(shellCmd))
		if cmdHasMatch && credIsAllEphemeral(contextStr, redirectMatches) &&
			credIsSimpleCmd(shellCmd) && credAllTargetsAreDevNull(redirectMatches) {
			return 0
		}
	} else {
		for _, m := range credCmdLineRe.FindAllStringSubmatch(contextStr, -1) {
			cmd := m[1]
			if cmd == "" {
				continue
			}
			cmdHasMatch := (match == "JWT" && credJWTRe.MatchString(cmd)) ||
				(match == "AWS access key" && credAWSRe.MatchString(cmd))
			cmdRedirects := credRedirectRe.FindAllString(cmd, -1)
			if cmdHasMatch && credIsAllEphemeral(cmd, cmdRedirects) &&
				credIsSimpleCmd(cmd) && credAllTargetsAreDevNull(cmdRedirects) {
				return 0
			}
		}
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

	// Steps 4–8: same logic as RunCredential (JSON-aware extraction and exemption).
	shellCmd, shellCwd, toolName, isJSON := credExtractCmdAndCwd(data)
	var contextStr string
	var redirectMatches []string
	if isJSON {
		contextStr = shellCmd
		redirectMatches = credRedirectRe.FindAllString(shellCmd, -1)
	} else {
		rawBytes := bytes.ReplaceAll(data, []byte(`\"`), []byte(`"`))
		contextStr = string(rawBytes)
		redirectMatches = credRedirectRe.FindAllString(contextStr, -1)
	}

	if match == "" {
		match = credSecondLayer(shellCmd, shellCwd, contextStr, toolName, redirectMatches)
	}

	// Non-JSON fallback: same as RunCredential — restore Layer 2b via credCmdLineRe.
	if match == "" && !isJSON {
		match = credNonJSONLayerTwoB(contextStr, "")
	}

	// F4: deep-scan all command/command_line values (same as RunCredential).
	if match == "" && isJSON {
		match = credDeepScan(data, shellCwd)
	}

	if match == "" {
		return 0
	}

	// F2 shape-based exemption (same conditions as RunCredential).
	if isJSON {
		cmdHasMatch := (match == "JWT" && credJWTRe.MatchString(shellCmd)) ||
			(match == "AWS access key" && credAWSRe.MatchString(shellCmd))
		if cmdHasMatch && credIsAllEphemeral(contextStr, redirectMatches) &&
			credIsSimpleCmd(shellCmd) && credAllTargetsAreDevNull(redirectMatches) {
			return 0
		}
	} else {
		for _, m := range credCmdLineRe.FindAllStringSubmatch(contextStr, -1) {
			cmd := m[1]
			if cmd == "" {
				continue
			}
			cmdHasMatch := (match == "JWT" && credJWTRe.MatchString(cmd)) ||
				(match == "AWS access key" && credAWSRe.MatchString(cmd))
			cmdRedirects := credRedirectRe.FindAllString(cmd, -1)
			if cmdHasMatch && credIsAllEphemeral(cmd, cmdRedirects) &&
				credIsSimpleCmd(cmd) && credAllTargetsAreDevNull(cmdRedirects) {
				return 0
			}
		}
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
// toolName is passed to credNormalizeWindowsPath to translate Git-Bash glob patterns
// (e.g. /c/Users/*.env) to Windows-native form (C:/Users/*.env) before expansion.
// Normalization is applied AFTER the early ContainsAny return so that tokens without
// glob metacharacters are handled by credResolveArg further downstream.
//
// Declared residual: POSIX character classes such as [[:alpha:]] are not supported by
// Go's filepath.Glob and return ErrBadPattern; such patterns keep the literal token.
//
// Returns a slice of >= 1 element (always at least the original token).
func credGlobToken(token, toolName string) []string {
	if !strings.ContainsAny(token, "*?[") {
		return []string{token}
	}
	// Translate POSIX [! negation to Go [^ negation, then normalize drive path.
	pattern := credNormalizeWindowsPath(strings.ReplaceAll(token, "[!", "[^"), runtime.GOOS, toolName)
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

// credNormalizeWindowsPath translates a Git-Bash or Cygwin drive path to the
// Windows-native forward-slash form so that credScanFile can open the file via
// os.Stat on a Windows host.
//
// Gate conditions (both must be satisfied for translation to occur):
//   - goos == "windows": on POSIX, /c/... is an already-valid absolute path.
//     Translating it would produce C:\... which filepath.IsAbs returns false for
//     on POSIX, causing it to be incorrectly treated as relative — a detection miss.
//   - strings.EqualFold(toolName, "PowerShell") == false: PowerShell resolves
//     /c/Users/... to C:\c\Users\... via its own provider API (not to C:\Users\...).
//     Translating PS paths would scan the wrong file (FP or FN).
//
// Translations applied when gates pass:
//   /[a-zA-Z]/rest        → X:/rest    (Git-Bash per-drive mount point)
//   /[a-zA-Z]             → X:/        (bare drive letter — avoid C: which is drive-relative)
//   /cygdrive/[a-zA-Z]/rest → X:/rest  (Cygwin mount point)
//   /cygdrive/[a-zA-Z]   → X:/
//
// Output uses forward slash (X:/rest) rather than filepath.FromSlash: the latter
// is OS-dependent and would produce X:\rest on POSIX cross-compile tests, breaking
// test expectations. C:/... is accepted by os.Stat on Windows (§2.2 measured).
//
// All other paths (relative, UNC \\server\share, /tmp, /home, C:\...) are returned unchanged.
func credNormalizeWindowsPath(path, goos, toolName string) string {
	if goos != "windows" {
		return path
	}
	if strings.EqualFold(toolName, "PowerShell") {
		return path
	}
	// /cygdrive/[a-zA-Z]/rest or /cygdrive/[a-zA-Z]
	if m := credCygdrivePrefixRe.FindStringSubmatch(path); m != nil {
		drive := strings.ToUpper(m[1])
		rest := m[2] // "/rest" including leading slash, or ""
		if rest == "" {
			return drive + ":/"
		}
		return drive + ":" + rest
	}
	// /[a-zA-Z]/rest or /[a-zA-Z]
	if m := credGitBashDriveRe.FindStringSubmatch(path); m != nil {
		drive := strings.ToUpper(m[1])
		rest := m[2] // "/rest" including leading slash, or ""
		if rest == "" {
			return drive + ":/"
		}
		return drive + ":" + rest
	}
	return path
}

// credExtractCmdAndCwd parses a JSON hook payload and returns the shell command
// and the working directory declared in the payload (tool_info.cwd).
//
// Key-extraction priority mirrors ExtractCommand in payload.go:
//  1. tool_input.command
//  2. command (root-level)
//  3. tool_info.command_line
//  4. hook_input.command
//
// Special case: when tool_name == "fs_write" (Amazon Q write tool), the
// tool_input.command value is an enum tag ("create"/"str_replace"/…), not a
// shell command. credExtractCmdAndCwd returns ("", cwd, true) for fs_write.
//
// Non-JSON payload (does not start with '{') or invalid JSON: returns
// ("", "", false). Callers use the legacy raw-string path in that case.
// Current behavior for non-JSON: credCmdLineRe finds no "command" key →
// Layer 2b is a no-op; credRedirectRe scans the raw text for redirects.
//
// A NUL byte in the decoded command string is treated as absent — the guard
// becomes a no-op for that key and tries the next lower-priority key.
func credExtractCmdAndCwd(data []byte) (shellCmd, cwd, toolName string, isJSON bool) {
	stripped := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	trimmed := bytes.TrimLeft(stripped, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return "", "", "", false
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(stripped, &root); err != nil { // F3: use stripped (no BOM)
		return "", "", "", false
	}

	// Extract tool_name for routing, cwd normalization, and propagation to callers.
	// Ignore unmarshal errors: toolName="" is a safe default (deny-list never fires for "").
	if rawTool, ok := root["tool_name"]; ok {
		_ = json.Unmarshal(rawTool, &toolName)
	}

	// fs_write: "command" carries an enum tag, not a shell command.
	if toolName == "fs_write" {
		return "", credExtractToolInfoCwd(root, toolName), toolName, true
	}

	cwd = credExtractToolInfoCwd(root, toolName)

	// Priority 1: tool_input.command
	if cmd, found, _ := extractNested(root, "tool_input", "command"); found && cmd != "" && !strings.ContainsRune(cmd, '\x00') {
		return cmd, cwd, toolName, true
	}
	// Priority 2: command (root-level)
	if raw, ok := root["command"]; ok && !isJSONNull(raw) {
		trimmedRaw := bytes.TrimLeft(raw, " \t\r\n")
		if len(trimmedRaw) > 0 && trimmedRaw[0] == '"' {
			var cmd string
			if err := json.Unmarshal(raw, &cmd); err == nil && !strings.ContainsRune(cmd, '\x00') {
				return cmd, cwd, toolName, true
			}
		}
	}
	// Priority 3: tool_info.command_line
	if cmd, found, _ := extractNested(root, "tool_info", "command_line"); found && cmd != "" && !strings.ContainsRune(cmd, '\x00') {
		return cmd, cwd, toolName, true
	}
	// Priority 4: hook_input.command
	if cmd, found, _ := extractNested(root, "hook_input", "command"); found && cmd != "" && !strings.ContainsRune(cmd, '\x00') {
		return cmd, cwd, toolName, true
	}

	return "", cwd, toolName, true // valid JSON, no command field found
}

// credExtractToolInfoCwd reads the cwd from tool_info.cwd in a parsed root map.
// The extracted cwd is normalized for Windows paths via credNormalizeWindowsPath
// so that relative arguments resolved against it produce valid paths on Windows.
func credExtractToolInfoCwd(root map[string]json.RawMessage, toolName string) string {
	rawInfo, ok := root["tool_info"]
	if !ok || isJSONNull(rawInfo) {
		return ""
	}
	var info map[string]json.RawMessage
	if err := json.Unmarshal(rawInfo, &info); err != nil {
		return ""
	}
	rawCwd, ok := info["cwd"]
	if !ok || isJSONNull(rawCwd) {
		return ""
	}
	var cwd string
	if err := json.Unmarshal(rawCwd, &cwd); err != nil {
		return ""
	}
	return credNormalizeWindowsPath(cwd, runtime.GOOS, toolName)
}

// credResolveArg strips JSON noise (quotes, trailing "},) from arg, translates
// Git-Bash/Cygwin drive paths to the Windows-native form, and resolves a relative
// path against baseCwd when provided.
//
// The cleaned string is safe to pass to credScanFile, which will not find
// additional noise to strip. If baseCwd is empty the function still strips
// quotes — this ensures that quoted paths like `"JWTFILE"` (from a JSON-decoded
// shell command) are handled correctly.
//
// toolName is forwarded to credNormalizeWindowsPath to suppress translation for
// PowerShell payloads (which use Windows-native path semantics for /c/...).
func credResolveArg(arg, baseCwd, toolName string) string {
	clean := strings.Map(func(r rune) rune {
		if r == '"' || r == '\'' {
			return -1
		}
		return r
	}, arg)
	clean = strings.TrimRight(clean, "},")
	if clean == "" {
		return arg // keep original so credScanFile can handle the empty-path case
	}
	// BUG-1 fix: translate /c/..., /cygdrive/c/... → C:/... before the IsAbs check.
	// Must be called AFTER TrimRight so quotes/noise do not interfere with the regex.
	// Covers both sub-cases:
	//   A (baseCwd=""): returns path directly; os.Stat("/c/...") failed on Windows.
	//   B (baseCwd≠""): IsAbs("/c/...")=false on Windows → wrong join; after fix IsAbs=true.
	clean = credNormalizeWindowsPath(clean, runtime.GOOS, toolName)
	if baseCwd != "" && !filepath.IsAbs(clean) {
		return filepath.Join(baseCwd, clean)
	}
	return clean
}

// credSecondLayer runs layer 2 detection when layer 1 found no match.
//
// Layer 2a: scan non-ephemeral redirect target files (same as the .sh).
// Layer 2b: tokenise shellCmd (JSON-decoded; already handles tool_input.command,
//   tool_info.command_line, etc.), glob-expand each token (porting bash
//   `set -- $CMD_LINE` without set -f), and scan file arguments of
//   cat/head/tail/jq/grep. When shellCmd is empty (write payloads, fs_write),
//   Layer 2b is a no-op.
//
// contextStr is used by credIsEphemeralTarget for the $(mktemp) variable check;
//   it equals shellCmd for JSON payloads and rawStr for non-JSON payloads.
// shellCwd is the working directory from tool_info.cwd in the payload, used to
//   resolve relative paths (e.g. "cat token.txt" when cwd="/project").
// toolName is forwarded to credResolveArg and credGlobToken for Windows path
//   normalization — prevents incorrect translation for PowerShell payloads.
func credSecondLayer(shellCmd, shellCwd, contextStr, toolName string, redirectMatches []string) string {
	// 2a — scan non-ephemeral redirect targets.
	for _, rm := range redirectMatches {
		if rm == "" {
			continue
		}
		target := credRedirectTarget(rm)
		if target == "" {
			continue
		}
		if !credIsEphemeralTarget(contextStr, target) {
			if m := credScanFile(credResolveArg(target, shellCwd, toolName)); m != "" {
				return m
			}
		}
	}

	// 2b — tokenise shellCmd, glob-expand, scan file arguments.
	// shellCmd is "" for payloads without a shell command (Write/Edit/fs_write).
	if shellCmd == "" {
		return ""
	}
	rawTokens := strings.Fields(shellCmd)
	if len(rawTokens) == 0 {
		return ""
	}
	// Apply glob expansion per token, matching bash `set -- $CMD_LINE` without set -f.
	var tokens []string
	for _, t := range rawTokens {
		tokens = append(tokens, credGlobToken(t, toolName)...)
	}
	if len(tokens) == 0 {
		return ""
	}
	switch tokens[0] {
	case "cat", "head", "tail", "jq", "grep":
		for _, tok := range tokens[1:] {
			if m := credScanFile(credResolveArg(tok, shellCwd, toolName)); m != "" {
				return m
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

// --------------------------------------------------------------------------
// F2: shape-based exemption gate (ML-2D)
// --------------------------------------------------------------------------

// credIsRedirectAmpersand reports whether the '&' at position i in cmd is an
// fd-duplication redirect (e.g. 2>&1, >&2) and should NOT be treated as a
// background metacharacter. Only returns true for [0-9]*>&[0-9-]; '>&word'
// (non-digit word) is a file redirect in bash and IS treated as metachar.
func credIsRedirectAmpersand(cmd string, i int) bool {
	if i == 0 {
		return false
	}
	// Scan backward past optional digits.
	j := i - 1
	for j >= 0 && cmd[j] >= '0' && cmd[j] <= '9' {
		j--
	}
	if j < 0 || cmd[j] != '>' {
		return false
	}
	// The character after '&' must be a digit or '-' for it to be an fd-dup.
	if i+1 >= len(cmd) {
		return false
	}
	next := cmd[i+1]
	return (next >= '0' && next <= '9') || next == '-'
}

// credHasShellMeta reports whether cmd contains a shell metacharacter that
// indicates a compound or pipeline command:
//   |, ;, \n, standalone & (not [0-9]*>&[0-9-]), $(, backtick, <(, >(
//
// Intentionally quote-unaware — fails closed (conservative).
func credHasShellMeta(cmd string) bool {
	for i := 0; i < len(cmd); i++ {
		switch cmd[i] {
		case '|', ';', '\n':
			return true
		case '&':
			if !credIsRedirectAmpersand(cmd, i) {
				return true
			}
		case '$':
			if i+1 < len(cmd) && cmd[i+1] == '(' {
				return true
			}
		case '`':
			return true
		case '<', '>':
			if i+1 < len(cmd) && cmd[i+1] == '(' {
				return true
			}
		}
	}
	return false
}

// credIsSimpleCmd reports whether shellCmd is a simple command suitable for the
// ephemeral exemption: no shell metacharacters and argv[0] (basename, without
// .exe suffix) is in {echo, printf}.
func credIsSimpleCmd(shellCmd string) bool {
	if credHasShellMeta(shellCmd) {
		return false
	}
	fields := strings.Fields(shellCmd)
	if len(fields) == 0 {
		return false
	}
	argv0 := filepath.Base(fields[0])
	argv0 = strings.TrimSuffix(argv0, ".exe") // Windows
	switch argv0 {
	case "echo", "printf":
		return true
	}
	return false
}

// credAllTargetsAreDevNull reports whether ALL redirect targets equal "/dev/null"
// exactly. Used as an additional guard inside the shape-based exemption to close
// the mktemp-filename bypass: a file named "mktemp.txt" satisfies
// credIsEphemeralTarget but materialises the token.
func credAllTargetsAreDevNull(redirectMatches []string) bool {
	for _, rm := range redirectMatches {
		target := credRedirectTarget(rm)
		target = strings.Map(func(r rune) rune {
			if r == '"' || r == '\'' {
				return -1
			}
			return r
		}, target)
		target = strings.TrimRight(target, "},")
		if target != "/dev/null" {
			return false
		}
	}
	return true
}

// --------------------------------------------------------------------------
// F4: deep walk of all command/command_line values (ML-2C)
// --------------------------------------------------------------------------

const (
	credDeepWalkMaxDepth = 20
	credDeepWalkMaxCount = 50
)

// credDeepScan walks all string values at "command" or "command_line" keys at any
// depth in the decoded JSON payload and runs credSecondLayer on each unique value.
// Returns the first credential match found, or "" if none.
//
// Exclusion: when tool_name == "fs_write", tool_input is removed from the walk
// tree (its "command" value is an enum tag, not a shell command).
//
// Depth limit: credDeepWalkMaxDepth. Count limit: credDeepWalkMaxCount.
// Exceeding a limit is fail-open for scanning (we stop early) but fail-closed
// for the exemption (exemption is based on primary shellCmd, never on deep walk).
func credDeepScan(data []byte, shellCwd string) string {
	stripped := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	var iface interface{}
	if json.Unmarshal(stripped, &iface) != nil {
		return ""
	}

	// For fs_write payloads, remove tool_input (its "command" is an enum tag).
	// Extract tool_name for Windows path normalization in credSecondLayer.
	var deepToolName string
	if m, ok := iface.(map[string]interface{}); ok {
		deepToolName, _ = m["tool_name"].(string)
		if deepToolName == "fs_write" {
			delete(m, "tool_input")
		}
	}

	// Collect unique command strings via a depth-limited recursive walk.
	seen := make(map[string]bool)
	var cmds []string
	count := 0

	var walk func(v interface{}, depth int)
	walk = func(v interface{}, depth int) {
		if depth > credDeepWalkMaxDepth || count >= credDeepWalkMaxCount {
			return
		}
		switch tv := v.(type) {
		case map[string]interface{}:
			for k, val := range tv {
				if k == "command" || k == "command_line" {
					if s, ok := val.(string); ok && s != "" && !strings.ContainsRune(s, '\x00') {
						if !seen[s] {
							seen[s] = true
							cmds = append(cmds, s)
							count++
						}
					}
					// Do not recurse into the value of a command key.
				} else {
					walk(val, depth+1)
				}
			}
		case []interface{}:
			for _, item := range tv {
				walk(item, depth+1)
			}
		}
	}
	walk(iface, 0)

	for _, cmd := range cmds {
		redirects := credRedirectRe.FindAllString(cmd, -1)
		if m := credSecondLayer(cmd, shellCwd, cmd, deepToolName, redirects); m != "" {
			return m
		}
	}
	return ""
}

// credNonJSONLayerTwoB runs Layer 2b for non-JSON (or invalid-JSON) payloads.
//
// ML-1C replaced credCmdLineRe with JSON parsing. For payloads that fail JSON
// decoding — such as those with unescaped backslashes in Windows paths — JSON
// parsing returns isJSON=false, shellCmd="", and Layer 2b becomes a no-op.
// credNonJSONLayerTwoB restores main's behaviour: it extracts all "command"
// field values from the raw text via credCmdLineRe and runs the file-argument
// scan of credSecondLayer on each extracted command.
//
// Limitation: only covers keys named exactly "command" (not "command_line").
// This matches main's scope. Payloads using "command_line" (Windsurf) require
// valid JSON for Layer 2b coverage — the test encoding fix ensures this.
//
// contextStr is the raw payload after `\"` → `"` substitution.
// shellCwd is typically "" for non-JSON payloads (tool_info.cwd is unavailable
// without successful JSON parsing).
func credNonJSONLayerTwoB(contextStr, shellCwd string) string {
	for _, m := range credCmdLineRe.FindAllStringSubmatch(contextStr, -1) {
		cmd := m[1]
		if cmd == "" {
			continue
		}
		// Pass nil for redirectMatches: Layer 2a already ran in the caller.
		// toolName="" — non-JSON payloads have no parseable tool_name; deny-list
		// skips PowerShell only when toolName=="PowerShell" (EqualFold); "" is safe.
		if match := credSecondLayer(cmd, shellCwd, contextStr, "", nil); match != "" {
			return match
		}
	}
	return ""
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

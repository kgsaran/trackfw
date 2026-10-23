// Package guard — write-scope subcommand.
// Implements trackfw guard write-scope: denies writes to frozen paths.
// ADR-2026-10-23 D3, ML-3A.
package guard

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// frozenMarker is checked in the first 10 lines of docs/agents-working-context.md
// to determine whether to freeze it for the main session (item 4 of ML-3A contract).
const frozenMarker = "<!-- trackfw:frozen -->"

// writeScopeWriteTools is the set of tool names that the guard intercepts.
// §Z of Wave 0: Write|Edit|NotebookEdit is a list-exact matcher (| is separator, not regex).
// DO NOT use a regexp here — the comparison is exact tool_name equality.
var writeScopeWriteTools = map[string]struct{}{
	"Write":        {},
	"Edit":         {},
	"NotebookEdit": {},
}

// writeScopeSubagentFrozenRelPaths are the paths frozen ONLY for subagents
// (agent_id present), relative to the project root.
// ML-3A contract item 5, §Z adendo.
// Paths ending in "/" match the directory and everything under it.
var writeScopeSubagentFrozenRelPaths = []string{
	"docs/agents-working-context.md",   // always frozen for subagents (item 5)
	"trackfw.yaml",                     // prevent disabling frozen-path extension
	".claude/settings.json",            // prevent removing write-scope hook
	".claude/settings.local.json",      // prevent removing write-scope hook
	".codex/hooks.json",                // prevent hook removal
	".gemini/settings.json",            // prevent hook removal
	".cursor/hooks.json",               // prevent hook removal
	".windsurf/hooks.json",             // prevent hook removal
	".github/hooks/",                   // prevent hook removal (directory)
	".kiro/hooks/",                     // prevent hook removal (directory)
	".amazonq/cli-agents/",             // prevent hook removal (directory)
	".trackfw/leases/",                 // prevent editing own permission (D4-D)
}

// RunWriteScope implements the write-scope guard.
// Entry point for `trackfw guard write-scope`.
//
// Returns 0 to allow, 2 to deny. The caller MUST exit with this code.
// Every error path exits with 2 (fail-closed, D3-F / ADR-2026-10-23 D7).
func RunWriteScope(stdin io.Reader, idleTimeout time.Duration, stderr io.Writer) (exitCode int) {
	// Panic recovery: any panic exits 2 (D3-F).
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(stderr, "trackfw guard write-scope: panic: %v\n", r)
			exitCode = 2
		}
	}()

	// Step 1: Drain stdin with idle timeout.
	data, truncated := DrainStdin(stdin, idleTimeout)
	if truncated && len(data) == 0 {
		fmt.Fprintln(stderr, "trackfw guard write-scope: denied: stdin truncated (no data received)")
		return 2
	}

	// Strip BOM (PowerShell may emit UTF-8 BOM).
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})

	// Step 2: Require JSON payload.
	trimmed := bytes.TrimLeft(data, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		fmt.Fprintln(stderr, "trackfw guard write-scope: denied: payload is not JSON")
		return 2
	}

	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		fmt.Fprintf(stderr, "trackfw guard write-scope: denied: invalid JSON payload: %v\n", err)
		return 2
	}

	// Step 3: Extract tool_name. Missing or non-string → deny.
	toolName, err := extractStringField(root, "tool_name")
	if err != nil {
		fmt.Fprintf(stderr, "trackfw guard write-scope: denied: %v\n", err)
		return 2
	}
	if toolName == "" {
		fmt.Fprintln(stderr, "trackfw guard write-scope: denied: tool_name absent or empty")
		return 2
	}

	// Step 4: tool_name not in write tools → allow.
	if _, isWrite := writeScopeWriteTools[toolName]; !isWrite {
		return 0
	}

	// Step 5: Extract file path from tool_input.
	rawFilePath, err := extractToolInputPath(root, toolName)
	if err != nil {
		fmt.Fprintf(stderr, "trackfw guard write-scope: denied: %v\n", err)
		return 2
	}

	// Step 6: Resolve the physical path of the target.
	resolved := resolveWriteScopePath(rawFilePath)

	// Step 7: Find project root of the resolved target path.
	// Item 3: root of the TARGET, not the cwd.
	projectRoot, found := findProjectRoot(resolved)
	if !found {
		// Target is outside any trackfw project → allow.
		return 0
	}

	// Step 8: Determine if this is a subagent call.
	isSubagent := payloadHasAgentID(root)

	// Step 9: Build the frozen list for this call.
	frozen := buildFrozenList(projectRoot, isSubagent)

	// Step 10: Check whether the resolved path hits a frozen entry.
	if rel, hit := frozenHit(resolved, projectRoot, frozen); hit {
		fmt.Fprintf(stderr, "trackfw guard write-scope: denied: %q is a frozen path (%s)\n", rawFilePath, rel)
		return 2
	}

	return 0
}

// extractStringField extracts a string value from a flat JSON map.
// Returns ("", err) if the key is missing, null, or not a string.
func extractStringField(root map[string]json.RawMessage, key string) (string, error) {
	raw, ok := root[key]
	if !ok || isJSONNull(raw) {
		return "", nil // absent → not an error here; caller decides
	}
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '"' {
		return "", fmt.Errorf("field %q is present but not a JSON string", key)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("field %q cannot be decoded: %w", key, err)
	}
	return s, nil
}

// extractToolInputPath extracts the relevant file path from tool_input.
// For Write and Edit: tool_input.file_path
// For NotebookEdit: tool_input.notebook_path
// Returns error if tool_input is absent, the field is absent, or the value is not a string.
func extractToolInputPath(root map[string]json.RawMessage, toolName string) (string, error) {
	rawTI, ok := root["tool_input"]
	if !ok || isJSONNull(rawTI) {
		return "", fmt.Errorf("tool_input absent in payload")
	}
	var ti map[string]json.RawMessage
	if err := json.Unmarshal(rawTI, &ti); err != nil {
		return "", fmt.Errorf("tool_input is not a JSON object: %w", err)
	}

	fieldKey := "file_path"
	if toolName == "NotebookEdit" {
		fieldKey = "notebook_path"
	}

	rawPath, ok := ti[fieldKey]
	if !ok || isJSONNull(rawPath) {
		return "", fmt.Errorf("tool_input.%s absent in payload", fieldKey)
	}
	trimmed := bytes.TrimLeft(rawPath, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '"' {
		return "", fmt.Errorf("tool_input.%s is not a string", fieldKey)
	}
	var p string
	if err := json.Unmarshal(rawPath, &p); err != nil {
		return "", fmt.Errorf("tool_input.%s cannot be decoded: %w", fieldKey, err)
	}
	if p == "" {
		return "", fmt.Errorf("tool_input.%s is empty", fieldKey)
	}
	return p, nil
}

// resolveWriteScopePath resolves a file path for frozen-path comparison.
// Does NOT call filepath.Abs (which cleans lexically) before EvalSymlinks,
// to ensure that ".." after a symlink component is resolved physically
// (D3-B / ADR-2026-10-23 Wave 0 §2.3).
//
// On Windows and macOS APFS the comparison is case-insensitive.
func resolveWriteScopePath(p string) string {
	// Make absolute without calling filepath.Clean.
	abs := p
	if !filepath.IsAbs(p) {
		if cwd, err := os.Getwd(); err == nil {
			// string concat — no normalization
			abs = cwd + string(filepath.Separator) + p
		}
	}

	// EvalSymlinks walks each component in order, resolving symlinks and ".."
	// in the context of the resolved stack. Works correctly for existing paths.
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}

	// Target doesn't exist yet (Write creating a new file).
	// Split on separator without filepath.Clean to preserve ".." after symlinks.
	parts := splitPathParts(abs)
	// Walk from longest to shortest prefix to find deepest existing ancestor.
	for i := len(parts); i >= 1; i-- {
		ancestor := joinPathParts(parts[:i])
		if ancestor == "" {
			continue
		}
		if _, err := os.Lstat(ancestor); err == nil {
			if resolved, err := filepath.EvalSymlinks(ancestor); err == nil {
				rest := joinPathParts(parts[i:])
				if rest == "" {
					return resolved
				}
				return filepath.Join(resolved, rest)
			}
		}
	}

	// Final fallback: clean.
	return filepath.Clean(abs)
}

// splitPathParts splits a path string on the OS separator without cleaning.
// On Windows, C:\a\b → ["C:", "a", "b"]; on Unix /a/b → ["", "a", "b"].
func splitPathParts(p string) []string {
	return strings.Split(p, string(filepath.Separator))
}

// joinPathParts joins path parts with the OS separator without cleaning.
// Empty parts slice → "". On Unix, ["", "a", "b"] → "/a/b".
func joinPathParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	result := strings.Join(parts, string(filepath.Separator))
	// On Unix, leading "" + sep + "a" = "/a" — correct.
	// Special case: a single-element slice [""] → "" (root on Unix is ["", ""])
	return result
}

// findProjectRoot walks up from p to find the first ancestor directory containing
// trackfw.yaml. Returns ("", false) if none found.
func findProjectRoot(p string) (string, bool) {
	dir := p
	// If p is a file (the common case), start from its directory.
	// But we don't know if it exists — use the cleaned path's directory.
	info, err := os.Lstat(p)
	if err != nil || !info.IsDir() {
		dir = filepath.Dir(p)
	}
	// Clean dir for consistent ancestor walking (this is safe since we already resolved symlinks above).
	dir = filepath.Clean(dir)

	for {
		yamlPath := filepath.Join(dir, "trackfw.yaml")
		if _, err := os.Lstat(yamlPath); err == nil {
			return dir, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// reached filesystem root
			return "", false
		}
		dir = parent
	}
}

// payloadHasAgentID reports whether the payload contains agent_id (non-empty string).
// Per ADR-2026-10-23 adendo: "É subagente" = agent_id presente no payload.
func payloadHasAgentID(root map[string]json.RawMessage) bool {
	raw, ok := root["agent_id"]
	if !ok || isJSONNull(raw) {
		return false
	}
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '"' {
		// Non-string agent_id: treat conservatively as subagent (restrictive direction).
		return true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return true // decode failure → restrictive
	}
	return s != "" // empty string → not a subagent
}

// buildFrozenList returns the list of frozen path entries for this call.
// Entries may be:
//   - relative paths (no trailing slash): exact file match relative to projectRoot
//   - relative paths with trailing slash: directory prefix match
//
// The list reads extra entries from trackfw.yaml (write_scope.frozen_paths).
// YAML is read directly (not via config.Load() singleton) to avoid osExit on malformed config.
func buildFrozenList(projectRoot string, isSubagent bool) []string {
	var frozen []string

	// All-session frozen paths (item 4): docs/agents-working-context.md, IF frozen marker present.
	if fileHasFrozenMarker(filepath.Join(projectRoot, "docs/agents-working-context.md")) {
		frozen = append(frozen, "docs/agents-working-context.md")
	}

	// Subagent extra frozen paths (item 5): always freeze docs/agents-working-context.md
	// plus the hook/settings files.
	if isSubagent {
		frozen = append(frozen, writeScopeSubagentFrozenRelPaths...)
	}

	// Extra paths from trackfw.yaml (item 6): append, never remove.
	extraAll, extraSub := readWriteScopeFromYAML(projectRoot)
	frozen = append(frozen, extraAll...)
	if isSubagent {
		frozen = append(frozen, extraSub...)
	}

	return frozen
}

// fileHasFrozenMarker reports whether the first 10 lines of the file contain frozenMarker.
// A missing or unreadable file → false (file is free for the main session).
// A FIFO or special file → false (avoid blocking reads).
func fileHasFrozenMarker(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false // missing → not frozen for main session
	}
	if !info.Mode().IsRegular() {
		return false // FIFO or special file → do not block
	}
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for i := 0; i < 10 && scanner.Scan(); i++ {
		if strings.Contains(scanner.Text(), frozenMarker) {
			return true
		}
	}
	return false
}

// readWriteScopeFromYAML reads write_scope.frozen_paths and write_scope.frozen_paths_subagent
// from trackfw.yaml at the given project root.
// Any error (missing file, invalid YAML, wrong type) returns empty slices silently.
// IMPORTANT: does NOT use config.Load() to avoid the osExit(1) on malformed YAML.
func readWriteScopeFromYAML(projectRoot string) (all, subagent []string) {
	data, err := os.ReadFile(filepath.Join(projectRoot, "trackfw.yaml"))
	if err != nil {
		return nil, nil
	}

	// Use config.ParseWriteScopeConfig which reads without the singleton.
	cfg := config.ParseWriteScopeConfig(string(data))
	return cfg.FrozenPaths, cfg.FrozenPathsSubagent
}

// frozenHit checks whether the resolved target path matches any entry in the frozen list.
// Returns (relPath, true) if a match is found.
func frozenHit(resolved, projectRoot string, frozen []string) (string, bool) {
	for _, entry := range frozen {
		if pathMatchesFrozen(resolved, projectRoot, entry) {
			return entry, true
		}
	}
	return "", false
}

// pathMatchesFrozen reports whether the resolved absolute path matches a frozen entry.
// entry is relative to projectRoot; trailing "/" means directory prefix match.
//
// Case sensitivity: case-insensitive on darwin and windows (APFS / NTFS), case-sensitive on Linux.
func pathMatchesFrozen(resolved, projectRoot, entry string) bool {
	// Resolve the frozen path to its canonical form.
	frozenAbs := filepath.Join(projectRoot, entry)

	isDir := strings.HasSuffix(entry, "/")

	if isDir {
		// Directory prefix: resolved must be under frozenAbs.
		frozenDir := filepath.Clean(frozenAbs)
		// Ensure the prefix ends with separator for prefix matching.
		prefix := frozenDir + string(filepath.Separator)
		if pathEqual(resolved, frozenDir) {
			return true // exact match to the directory itself
		}
		return pathHasPrefix(resolved, prefix)
	}

	// Exact file match: resolve the frozen path too.
	frozenResolved := frozenAbs
	if r, err := filepath.EvalSymlinks(frozenAbs); err == nil {
		frozenResolved = r
	}
	return pathEqual(resolved, frozenResolved)
}

// pathEqual compares two absolute paths, case-insensitively on darwin and windows.
func pathEqual(a, b string) bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// pathHasPrefix reports whether p starts with prefix (case-insensitive on darwin/windows).
func pathHasPrefix(p, prefix string) bool {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return strings.HasPrefix(strings.ToLower(p), strings.ToLower(prefix))
	}
	return strings.HasPrefix(p, prefix)
}

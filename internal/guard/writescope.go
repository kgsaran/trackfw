// Package guard — write-scope subcommand.
// Implements trackfw guard write-scope: denies writes to frozen paths (D3)
// and enforces per-agent write-scope leases (D4).
// ADR-2026-10-23, ML-3A, ML-3B.
package guard

import (
	"bufio"
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

	"github.com/kgsaran/trackfw/internal/config"
)

// agentTypeSlugRE validates agent_type as a safe filename component.
// Must match ^[A-Za-z0-9_-]{1,64}$ to prevent path traversal (D4-A, Wave 0 §2.4).
var agentTypeSlugRE = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// WriteScopeLease is the on-disk format of a write-scope permission lease (v1).
// File: <project_root>/.trackfw/leases/<agent_type>.json
// The arquiteto writes the lease before dispatching a subagent and deletes it after.
// ML-3B, ADR-2026-10-23 adendo.
type WriteScopeLease struct {
	Version   int      `json:"version"`
	AgentType string   `json:"agent_type"`
	Allow     []string `json:"allow"`    // absolute paths; trailing "/" → directory prefix
	Expires   string   `json:"expires"` // RFC 3339; mandatory
}

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

// writeScopeInternalHook is called inside RunWriteScope immediately after the
// panic-recovery defer is registered. Nil in production. Tests set it to inject
// a panic and verify that the defer catches it and returns exit 2.
// ML-3A2: panic → 2 seam.
var writeScopeInternalHook func()

// writeScopeSubagentFrozenRelPaths are the paths frozen ONLY for subagents
// (agent_id present), relative to the project root.
// ML-3A contract item 5, §Z adendo.
// Paths ending in "/" match the directory and everything under it.
var writeScopeSubagentFrozenRelPaths = []string{
	"docs/agents-working-context.md",  // always frozen for subagents (item 5)
	"docs/agents-working-context/",    // directory of per-session entries — D1 independence (ML-4C Decision 4)
	"trackfw.yaml",                    // prevent disabling frozen-path extension
	".claude/settings.json",           // prevent removing write-scope hook
	".claude/settings.local.json",     // prevent removing write-scope hook
	".codex/hooks.json",               // prevent hook removal
	".gemini/settings.json",           // prevent hook removal
	".cursor/hooks.json",              // prevent hook removal
	".windsurf/hooks.json",            // prevent hook removal
	".github/hooks/",                  // prevent hook removal (directory)
	".kiro/hooks/",                    // prevent hook removal (directory)
	".amazonq/cli-agents/",            // prevent hook removal (directory)
	".trackfw/leases/",                // prevent editing own permission (D4-D)
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

	// Panic seam: hook fires after defer is registered; tests use it to verify recovery.
	if writeScopeInternalHook != nil {
		writeScopeInternalHook()
	}

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

	// Step 5b: Normalize path for the current OS (handles /c/... on Windows).
	rawFilePath = normalizeTargetPath(rawFilePath, runtime.GOOS)

	// Step 5c: Deny relative paths. The Write/Edit contract requires absolute paths.
	// Resolving against os.Getwd() is wrong: the hook process cwd differs from the
	// agent's cwd. Failing closed is safer (ML-3A2 AC5).
	if !filepath.IsAbs(rawFilePath) {
		fmt.Fprintf(stderr, "trackfw guard write-scope: denied: %q is not an absolute path\n", rawFilePath)
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

	// Step 9: Build the frozen list for this call. YAML errors → deny (item 6).
	frozen, yamlErr := buildFrozenList(projectRoot, isSubagent)
	if yamlErr != nil {
		fmt.Fprintf(stderr, "trackfw guard write-scope: denied: %v\n", yamlErr)
		return 2
	}

	// Step 10: Check whether the resolved path hits a frozen entry.
	// D3 always runs before D4. A frozen path is denied even if it appears in the D4 allow list.
	if rel, hit := frozenHit(resolved, projectRoot, frozen); hit {
		fmt.Fprintf(stderr, "trackfw guard write-scope: denied: %q is a frozen path (%s)\n", rawFilePath, rel)
		return 2
	}

	// Step 11: D4 — permission by dispatch (subagent only, ML-3B/ML-3C).
	// Only applies when the payload identifies a subagent (agent_id present, non-empty).
	if !isSubagent {
		return 0
	}

	// Extract agent_type. A missing, empty, or non-slug agent_type means "no lease" → D3 only.
	// Errors from extractStringField (non-string value) are silenced: they collapse to empty string.
	agentType, _ := extractStringField(root, "agent_type")
	if !agentTypeSlugRE.MatchString(agentType) {
		// Invalid or empty → no lease lookup → D3 only → allow.
		return 0
	}

	// ML-3C: search for a lease in TWO roots (without duplication):
	//   1. The root of the TARGET project (as in ML-3B).
	//   2. The root of the CWD project (the session root from the payload).
	//
	// Rationale: the lease written by the architect lives in the session root (the cwd
	// of the orchestrating session). When the target is in a different project (e.g. the
	// main checkout while the agent runs in a worktree), searching only the target root
	// misses the lease and falls through to D3-only, which is a fail-open for D4 (ML-3C §2).
	//
	// Contract:
	//   - If an invalid lease is found in any root → deny (fail-closed).
	//   - If a lease is found → EVERY found lease must independently authorize the target
	//     (Decision 3, ML-4C). A permissive cwd-project lease cannot override a restrictive
	//     target-project lease. The old "at least one" rule allowed a fake project with
	//     allow=["/private/"] to bypass the target project's restricted lease (attack 2E).
	//   - If no lease is found in any root → D3 only → allow.
	//   - cwd absent, non-string, or relative → only the target root is searched.

	// Step 11a: extract cwd project root from the payload.
	var cwdProjectRoot string
	if cwdStr, cwdFieldErr := extractStringField(root, "cwd"); cwdFieldErr == nil && cwdStr != "" {
		cwdStr = normalizeTargetPath(cwdStr, runtime.GOOS)
		if filepath.IsAbs(cwdStr) {
			cwdResolved := resolveWriteScopePath(cwdStr)
			if cr, ok := findProjectRoot(cwdResolved); ok {
				cwdProjectRoot = cr
			}
		}
	}

	// Step 11b: collect deduplicated roots to search.
	leaseRoots := collectLeaseRoots(projectRoot, cwdProjectRoot)

	// Step 11c: load leases from all roots.
	type leaseEntry struct {
		lease *WriteScopeLease
		path  string // full path to the lease file (for error messages)
	}
	var loadedLeases []leaseEntry
	for _, lr := range leaseRoots {
		l, lerr := loadWriteScopeLease(lr, agentType)
		if lerr != nil {
			// Unreadable or malformed lease in any root → fail-closed.
			fmt.Fprintf(stderr, "trackfw guard write-scope: denied: %v\n", lerr)
			return 2
		}
		if l != nil {
			loadedLeases = append(loadedLeases, leaseEntry{
				lease: l,
				path:  filepath.Join(lr, ".trackfw", "leases", agentType+".json"),
			})
		}
	}

	// Step 11d: no leases found in any root → D3 only → allow.
	if len(loadedLeases) == 0 {
		return 0
	}

	// Step 11e: validate ALL loaded leases before checking any allow list.
	// An invalid lease in any root denies, even if another root has a valid matching lease.
	for _, le := range loadedLeases {
		lease := le.lease
		leasePath := le.path
		if lease.Version != 1 {
			fmt.Fprintf(stderr, "trackfw guard write-scope: denied: lease for %q: unsupported version %d (want 1)\n", agentType, lease.Version)
			return 2
		}
		if lease.AgentType != agentType {
			fmt.Fprintf(stderr, "trackfw guard write-scope: denied: lease for %q: agent_type in file (%q) does not match payload (%q)\n", agentType, lease.AgentType, agentType)
			return 2
		}
		if lease.Expires == "" {
			fmt.Fprintf(stderr, "trackfw guard write-scope: denied: lease for %q: expires field is missing\n", agentType)
			return 2
		}
		expires, parseErr := time.Parse(time.RFC3339, lease.Expires)
		if parseErr != nil {
			fmt.Fprintf(stderr, "trackfw guard write-scope: denied: lease for %q: expires is not valid RFC 3339: %v\n", agentType, parseErr)
			return 2
		}
		if time.Now().After(expires) {
			fmt.Fprintf(stderr, "trackfw guard write-scope: denied: lease for %q has expired at %s\n", agentType, lease.Expires)
			return 2
		}
		// Validate ALL allow entries are absolute before matching any.
		// One relative entry in the list denies the write even if another entry would match.
		for _, e := range lease.Allow {
			en := normalizeTargetPath(e, runtime.GOOS)
			if !filepath.IsAbs(en) {
				fmt.Fprintf(stderr, "trackfw guard write-scope: denied: lease %s: allow entry %q is not an absolute path\n", leasePath, e)
				return 2
			}
		}
	}

	// Step 11f: check that the resolved target is within the allow list of EVERY valid lease
	// (Decision 3, ML-4C). All found leases must independently authorize the target.
	// Trailing "/" in the allow entry means directory prefix (strictly below the dir);
	// otherwise exact file match. A trailing-slash entry does NOT match the dir-named file
	// (Decision 5, ML-4C): "/path/file.go/" allows only files under that name as a dir.
	for _, le := range loadedLeases {
		leaseAllows := false
		for _, e := range le.lease.Allow {
			en := normalizeTargetPath(e, runtime.GOOS)
			isDirEntry := strings.HasSuffix(e, "/")
			allowResolved := resolveWriteScopePath(en)
			if isDirEntry {
				allowDir := filepath.Clean(allowResolved)
				// Use separator-terminated prefix to prevent /a/b matching /a/bc.
				// Trailing "/" means strictly inside the directory — pathEqual(resolved, allowDir)
				// is intentionally NOT checked (Decision 5): an entry ending in "/" names a
				// directory, not a file of the same name; matching the directory itself would
				// allow writing a file with the same name as the directory component.
				prefix := allowDir + string(filepath.Separator)
				if pathHasPrefix(resolved, prefix) {
					leaseAllows = true
					break
				}
			} else {
				if pathEqual(resolved, allowResolved) {
					leaseAllows = true
					break
				}
			}
		}
		if !leaseAllows {
			// This lease does not authorize the target — deny immediately.
			fmt.Fprintf(stderr, "trackfw guard write-scope: denied: %q is not in the allow list of lease %s for %q\n",
				rawFilePath, le.path, agentType)
			return 2
		}
	}
	return 0
}

// collectLeaseRoots returns the deduplicated ordered list of project roots to search for leases.
// Always includes targetRoot. Adds cwdRoot if it is non-empty and not path-equal to targetRoot.
// ML-3C: two-root lease search.
func collectLeaseRoots(targetRoot, cwdRoot string) []string {
	if cwdRoot == "" || pathEqual(cwdRoot, targetRoot) {
		return []string{targetRoot}
	}
	// targetRoot first so the error message names it first when both are present.
	return []string{targetRoot, cwdRoot}
}

// loadWriteScopeLease reads and parses the lease file for agentType.
// Returns (nil, nil) when the file is absent → caller applies D3 only (ML-3B item 2).
// Returns (nil, err) when the file exists but cannot be read or parsed → caller denies (D4-C).
// Returns (lease, nil) when the file is present and parses as JSON (validation is the caller's job).
//
// agentType MUST already be validated as a slug before calling this function (D4-A):
// filepath.Join would otherwise allow path traversal.
func loadWriteScopeLease(projectRoot, agentType string) (*WriteScopeLease, error) {
	leasePath := filepath.Join(projectRoot, ".trackfw", "leases", agentType+".json")

	info, err := os.Stat(leasePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // absent → D3 only
		}
		return nil, fmt.Errorf("lease file %q: %w", leasePath, err)
	}
	if !info.Mode().IsRegular() {
		// Covers directory entries and FIFOs (a FIFO would block ReadFile).
		return nil, fmt.Errorf("lease file %q is not a regular file (D4-C)", leasePath)
	}

	data, err := os.ReadFile(leasePath)
	if err != nil {
		return nil, fmt.Errorf("lease file %q: cannot read: %w", leasePath, err)
	}

	var lease WriteScopeLease
	if err := json.Unmarshal(data, &lease); err != nil {
		return nil, fmt.Errorf("lease file %q: invalid JSON: %w", leasePath, err)
	}
	return &lease, nil
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
// Returns a non-nil error when trackfw.yaml exists but contains invalid YAML (item 6: YAML inválido nega).
func buildFrozenList(projectRoot string, isSubagent bool) ([]string, error) {
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

	// Extra paths from trackfw.yaml (item 6): append, never remove. Invalid YAML → return error.
	extraAll, extraSub, err := readWriteScopeFromYAML(projectRoot)
	if err != nil {
		return nil, err
	}
	frozen = append(frozen, extraAll...)
	if isSubagent {
		frozen = append(frozen, extraSub...)
	}

	return frozen, nil
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
// Missing file → (nil, nil, nil): no extra frozen paths, not an error.
// Invalid YAML → (nil, nil, error): contract item 6 says YAML inválido nega.
// IMPORTANT: does NOT use config.Load() to avoid the osExit(1) on malformed YAML.
func readWriteScopeFromYAML(projectRoot string) (all, subagent []string, err error) {
	data, readErr := os.ReadFile(filepath.Join(projectRoot, "trackfw.yaml"))
	if readErr != nil {
		return nil, nil, nil // file absent → not an error
	}

	// ParseWriteScopeConfig validates YAML (including duplicate mapping keys) and returns error
	// on any malformed content — item 6: YAML inválido nega. Decision 6 (ML-4C): the guard no
	// longer needs a pre-validation pass; ParseWriteScopeConfig does it atomically.
	cfg, yamlErr := config.ParseWriteScopeConfig(string(data))
	if yamlErr != nil {
		return nil, nil, fmt.Errorf("trackfw.yaml: invalid YAML: %w", yamlErr)
	}
	return cfg.FrozenPaths, cfg.FrozenPathsSubagent, nil
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
//
// Decision 7 (ML-4C): directory entries also resolve symlinks in the frozen path.
// Decision 2 (ML-4C): file entries use os.SameFile to detect hardlinks (two distinct paths
// to the same inode). EvalSymlinks does not cover hardlinks; SameFile does.
func pathMatchesFrozen(resolved, projectRoot, entry string) bool {
	// Resolve the frozen path to its canonical form.
	frozenAbs := filepath.Join(projectRoot, entry)

	isDir := strings.HasSuffix(entry, "/")

	if isDir {
		// Directory prefix: resolved must be under frozenAbs (or equal to frozenAbs itself).
		// Resolve symlinks in the frozen directory path so that a symlinked directory is
		// correctly matched when the target arrives via its physical path (R3, ML-4C).
		frozenDirResolved := frozenAbs
		if r, err := filepath.EvalSymlinks(frozenAbs); err == nil {
			frozenDirResolved = r
		}
		frozenDir := filepath.Clean(frozenDirResolved)
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
	if pathEqual(resolved, frozenResolved) {
		return true
	}
	// Hardlink detection: os.SameFile reports that two distinct paths share an inode.
	// Only applicable when both the target and the frozen file exist on disk.
	// A target that does not exist yet cannot be a hardlink to the frozen file.
	if fi1, err1 := os.Lstat(resolved); err1 == nil {
		if fi2, err2 := os.Lstat(frozenResolved); err2 == nil {
			if os.SameFile(fi1, fi2) {
				return true
			}
		}
	}
	return false
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

// normalizeTargetPath translates path p to its native OS form for goos.
// On Windows, Unix-style drive-letter paths are converted:
//   - /c/rest  → C:\rest
//   - C:/rest  → C:\rest
//
// On all other OSes the path is returned unchanged.
// This is a pure string operation; it does NOT touch the filesystem.
// ML-3A2 AC5 (Windows normalisation).
func normalizeTargetPath(p, goos string) string {
	if goos != "windows" {
		return p
	}
	// /c/rest or /C/rest → C:\rest
	if len(p) >= 3 && p[0] == '/' && isWindowsDriveLetter(p[1]) && p[2] == '/' {
		drive := strings.ToUpper(string(p[1]))
		rest := strings.ReplaceAll(p[3:], "/", "\\")
		if rest == "" {
			return drive + ":\\"
		}
		return drive + ":\\" + rest
	}
	// C:/rest → C:\rest  (mixed slash — occurs in some agent payloads)
	if len(p) >= 3 && isWindowsDriveLetter(p[0]) && p[1] == ':' && p[2] == '/' {
		return string(p[0:2]) + "\\" + strings.ReplaceAll(p[3:], "/", "\\")
	}
	return p
}

// isWindowsDriveLetter reports whether c is an ASCII letter (a–z or A–Z).
func isWindowsDriveLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

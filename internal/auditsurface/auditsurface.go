// Package auditsurface implements the `trackfw audit-surface` command.
//
// It reads the hook wiring and instruction files at a given git ref WITHOUT
// performing a checkout of the working tree — using only `git show <ref>:<path>`.
// This closes the window to zero: the maintainer can audit a PR ref before
// making it part of the local worktree.
//
// AC16 invariant (no false positives, by construction): this package NEVER
// reads file content looking for hook-path strings. It ONLY opens the 8
// exact wiring-file paths defined in runtimeTable. Files like
// docs/cli-parity.md and internal/generators/agentfiles.go happen to mention
// those paths as strings, but they are never opened by this package — they
// live at paths that are NOT in runtimeTable — so the false-positive problem
// is eliminated at the design level.
package auditsurface

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Runtime names in canonical order (matches check-agent-hooks-parity.sh CLIS).
var canonicalRuntimes = []string{
	"claude", "codex", "gemini", "copilot", "cursor", "kiro", "windsurf", "amazonq",
}

// runtimeWiringPath maps each runtime to its project-scope wiring file.
// These are the ONLY paths this package ever opens — no grep, no content scan.
var runtimeWiringPath = map[string]string{
	"claude":   ".claude/settings.json",
	"codex":    ".codex/hooks.json",
	"gemini":   ".gemini/settings.json",
	"copilot":  ".github/hooks/trackfw-attention.json",
	"cursor":   ".cursor/hooks.json",
	"kiro":     ".kiro/hooks/trackfw-attention.json",
	"windsurf": ".windsurf/hooks.json",
	"amazonq":  ".amazonq/cli-agents/q_cli_default.json",
}

// Instruction file paths that ship in a project and instruct the agent
// (they do not execute shell — their effect is on future agent actions).
var instructionFilePaths = []string{
	"CLAUDE.md",
	"AGENTS.md",
	"GEMINI.md",
	".windsurfrules",
	".github/copilot-instructions.md",
	".amazonq/developer/guidelines.md",
	".cursor/rules/trackfw.mdc",
}

// HookTuple is one (event, matcher, command) entry from a wiring file.
// The unit of reporting for AC14: any change to any field is a surface change.
type HookTuple struct {
	Event      string `json:"event"`
	Matcher    string `json:"matcher"`
	RawCommand string `json:"raw_command"`
	ScriptPath string `json:"script_path"`  // normalized repo-relative, or "" if inline
	Digest     string `json:"script_digest"` // "sha256:hex", "not-found", or "unresolvable"
}

// RuntimeResult holds the audit result for one runtime.
type RuntimeResult struct {
	Runtime    string      `json:"runtime"`
	WiringFile string      `json:"wiring_file"`
	Present    bool        `json:"present"`
	Tuples     []HookTuple `json:"tuples"`
}

// InstructionFile is a file that instructs the agent (not a shell script).
type InstructionFile struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`    // "agent-config" or "slash-command"
	Present bool   `json:"present"`
}

// LifecycleHook is a package-manager lifecycle hook (npm preinstall, etc.).
type LifecycleHook struct {
	File    string `json:"file"`
	Key     string `json:"key"`
	Command string `json:"command,omitempty"`
	Present bool   `json:"present"`
}

// Report is the full audit result.
type Report struct {
	Ref              string            `json:"ref"`
	Base             string            `json:"base,omitempty"`
	HookWiring       []RuntimeResult   `json:"hook_wiring"`
	InstructionFiles []InstructionFile `json:"instruction_files"`
	LifecycleHooks   []LifecycleHook   `json:"lifecycle_hooks"`
}

// Options for RunAuditSurface.
type Options struct {
	Ref     string // ref to audit (required)
	Base    string // base ref for Makefile/CI diff (optional)
	GitRoot string // working directory for git commands
}

// RunAuditSurface performs the full audit and returns a Report.
// It never touches the working tree — all file reads go through `git show`.
func RunAuditSurface(opts Options) (*Report, error) {
	report := &Report{
		Ref:  opts.Ref,
		Base: opts.Base,
	}

	// 1. Hook wiring — 8 runtimes in canonical order.
	for _, runtime := range canonicalRuntimes {
		wiringPath := runtimeWiringPath[runtime]
		content, err := gitShow(opts.Ref, wiringPath, opts.GitRoot)
		if err != nil {
			// Absent at this ref — information, not an error (AC13).
			report.HookWiring = append(report.HookWiring, RuntimeResult{
				Runtime:    runtime,
				WiringFile: wiringPath,
				Present:    false,
				Tuples:     []HookTuple{},
			})
			continue
		}

		tuples, parseErr := extractTuples(runtime, content)
		if parseErr != nil {
			// Wiring file is malformed JSON — report as present with parse error note.
			report.HookWiring = append(report.HookWiring, RuntimeResult{
				Runtime:    runtime,
				WiringFile: wiringPath,
				Present:    true,
				Tuples: []HookTuple{{
					Event:      "parse-error",
					Matcher:    "",
					RawCommand: parseErr.Error(),
					ScriptPath: "",
					Digest:     "unresolvable",
				}},
			})
			continue
		}

		// For each tuple, compute the digest of the referenced script.
		for i := range tuples {
			if tuples[i].ScriptPath != "" {
				scriptBytes, scriptErr := gitShow(opts.Ref, tuples[i].ScriptPath, opts.GitRoot)
				if scriptErr != nil {
					tuples[i].Digest = "not-found"
				} else {
					h := sha256.Sum256(scriptBytes)
					tuples[i].Digest = fmt.Sprintf("sha256:%x", h)
				}
			} else {
				tuples[i].Digest = "unresolvable"
			}
		}

		report.HookWiring = append(report.HookWiring, RuntimeResult{
			Runtime:    runtime,
			WiringFile: wiringPath,
			Present:    true,
			Tuples:     tuples,
		})
	}

	// 2. Instruction files (agent-config kind).
	for _, path := range instructionFilePaths {
		_, err := gitShow(opts.Ref, path, opts.GitRoot)
		report.InstructionFiles = append(report.InstructionFiles, InstructionFile{
			Path:    path,
			Kind:    "agent-config",
			Present: err == nil,
		})
	}

	// 3. Slash commands (.claude/commands/**/*.md).
	slashFiles, _ := gitLsTree(opts.Ref, ".claude/commands", opts.GitRoot)
	sort.Strings(slashFiles)
	for _, f := range slashFiles {
		if strings.HasSuffix(f, ".md") {
			report.InstructionFiles = append(report.InstructionFiles, InstructionFile{
				Path:    f,
				Kind:    "slash-command",
				Present: true,
			})
		}
	}

	// 4. Lifecycle hooks: npm package.json and .husky/pre-commit.
	report.LifecycleHooks = auditLifecycleHooks(opts.Ref, opts.GitRoot)

	return report, nil
}

// auditLifecycleHooks checks npm lifecycle hooks and .husky/pre-commit.
func auditLifecycleHooks(ref, gitRoot string) []LifecycleHook {
	var hooks []LifecycleHook

	// npm/package.json lifecycle hooks (Rung 4 from threat model).
	npmContent, err := gitShow(ref, "npm/package.json", gitRoot)
	if err == nil {
		var pkg map[string]interface{}
		if json.Unmarshal(npmContent, &pkg) == nil {
			scripts, _ := pkg["scripts"].(map[string]interface{})
			for _, key := range []string{"preinstall", "postinstall", "prepare"} {
				cmd, ok := scripts[key].(string)
				hooks = append(hooks, LifecycleHook{
					File:    "npm/package.json",
					Key:     key,
					Command: cmd,
					Present: ok,
				})
			}
		}
	} else {
		for _, key := range []string{"preinstall", "postinstall", "prepare"} {
			hooks = append(hooks, LifecycleHook{File: "npm/package.json", Key: key, Present: false})
		}
	}

	// .husky/pre-commit (Rung 0/3 from threat model: present but disarmed by local hooksPath).
	huskyContent, err := gitShow(ref, ".husky/pre-commit", gitRoot)
	if err == nil {
		// Extract the command called (heuristic: first non-comment, non-empty line after shebang).
		cmd := extractHuskyCommand(huskyContent)
		hooks = append(hooks, LifecycleHook{
			File:    ".husky/pre-commit",
			Key:     "pre-commit",
			Command: cmd,
			Present: true,
		})
	} else {
		hooks = append(hooks, LifecycleHook{File: ".husky/pre-commit", Key: "pre-commit", Present: false})
	}

	return hooks
}

// extractHuskyCommand returns the first meaningful shell command from a husky hook file.
func extractHuskyCommand(content []byte) string {
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "#!/") {
			continue
		}
		return line
	}
	return ""
}

// gitShow reads the content of a file at a given ref via `git show <ref>:<path>`.
// Returns the raw bytes. Returns an error (treated as "absent") when the path
// does not exist at the ref.
func gitShow(ref, path, gitRoot string) ([]byte, error) {
	cmd := exec.Command("git", "show", ref+":"+path)
	cmd.Dir = gitRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
}

// gitLsTree lists files in a directory at a given ref, returning repo-relative paths.
func gitLsTree(ref, dir, gitRoot string) ([]string, error) {
	cmd := exec.Command("git", "ls-tree", "-r", "--name-only", ref, "--", dir)
	cmd.Dir = gitRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var files []string
	for _, line := range strings.Split(string(bytes.TrimRight(out, "\n")), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}

// normalizeCommand strips known project-root env-var prefixes and outer quotes
// to produce a repo-relative script path. Returns "" if the command cannot be
// resolved to a file path (e.g., an inline pipeline like `curl … | sh`).
func normalizeCommand(rawCmd string) string {
	cmd := strings.TrimSpace(rawCmd)

	// Strip surrounding double-quotes (Codex format wraps in literal quotes).
	if strings.HasPrefix(cmd, `"`) && strings.HasSuffix(cmd, `"`) {
		cmd = cmd[1 : len(cmd)-1]
	}

	// Strip known project-root prefixes.
	prefixes := []string{
		"$CLAUDE_PROJECT_DIR/",
		"$GEMINI_PROJECT_DIR/",
		"$(git rev-parse --show-toplevel)/",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(cmd, p) {
			return cmd[len(p):]
		}
	}

	// If the result looks like a relative path to a shell script, accept it.
	if !strings.Contains(cmd, " ") && (strings.HasSuffix(cmd, ".sh") || strings.HasSuffix(cmd, ".py") || strings.HasSuffix(cmd, ".js")) {
		return cmd
	}

	// Cannot resolve to a file path (inline command, pipe, etc.).
	return ""
}

// extractTuples parses a wiring-file JSON and returns the (event, matcher, command) tuples.
// The parsing logic is per-runtime because each CLI uses a different JSON schema.
func extractTuples(runtime string, content []byte) ([]HookTuple, error) {
	switch runtime {
	case "claude", "codex", "amazonq":
		return extractClaudeSchema(content)
	case "gemini":
		return extractGeminiSchema(content)
	case "kiro":
		return extractKiroSchema(content)
	case "copilot":
		return extractCopilotSchema(content)
	case "cursor":
		return extractCursorSchema(content)
	case "windsurf":
		return extractWindsurfSchema(content)
	default:
		return nil, fmt.Errorf("unknown runtime: %s", runtime)
	}
}

// extractClaudeSchema parses Claude/Codex/AmazonQ wiring:
// {"hooks": {"EVENT": [{"matcher": "...", "hooks": [{"command": "...", "type": "command"}]}]}}
func extractClaudeSchema(content []byte) ([]HookTuple, error) {
	var root struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
				Type    string `json:"type"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(content, &root); err != nil {
		return nil, err
	}

	// Sort events for deterministic output.
	events := make([]string, 0, len(root.Hooks))
	for e := range root.Hooks {
		events = append(events, e)
	}
	sort.Strings(events)

	var tuples []HookTuple
	for _, event := range events {
		entries := root.Hooks[event]
		// Sort entries by matcher for determinism.
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Matcher < entries[j].Matcher
		})
		for _, entry := range entries {
			for _, h := range entry.Hooks {
				if h.Type != "command" {
					continue
				}
				tuples = append(tuples, HookTuple{
					Event:      event,
					Matcher:    entry.Matcher,
					RawCommand: h.Command,
					ScriptPath: normalizeCommand(h.Command),
				})
			}
		}
	}
	return tuples, nil
}

// extractGeminiSchema parses Gemini wiring (same structure as Claude).
func extractGeminiSchema(content []byte) ([]HookTuple, error) {
	return extractClaudeSchema(content)
}

// extractKiroSchema parses Kiro wiring:
// {"version": "v1", "hooks": [{"trigger": "...", "matcher": "...", "action": {"type": "command", "command": "..."}}]}
func extractKiroSchema(content []byte) ([]HookTuple, error) {
	var root struct {
		Hooks []struct {
			Trigger string `json:"trigger"`
			Matcher string `json:"matcher"`
			Action  struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"action"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(content, &root); err != nil {
		return nil, err
	}

	// Sort by trigger, then matcher for determinism.
	sort.Slice(root.Hooks, func(i, j int) bool {
		if root.Hooks[i].Trigger != root.Hooks[j].Trigger {
			return root.Hooks[i].Trigger < root.Hooks[j].Trigger
		}
		return root.Hooks[i].Matcher < root.Hooks[j].Matcher
	})

	var tuples []HookTuple
	for _, h := range root.Hooks {
		if h.Action.Type != "command" {
			continue
		}
		tuples = append(tuples, HookTuple{
			Event:      h.Trigger,
			Matcher:    h.Matcher,
			RawCommand: h.Action.Command,
			ScriptPath: normalizeCommand(h.Action.Command),
		})
	}
	return tuples, nil
}

// extractCopilotSchema parses GitHub Copilot wiring:
// {"version": 1, "hooks": {"preToolUse": [{"type": "command", "bash": "...", "matcher": "...", ...}], ...}}
func extractCopilotSchema(content []byte) ([]HookTuple, error) {
	var root struct {
		Hooks map[string][]struct {
			Type       string `json:"type"`
			Bash       string `json:"bash"`
			Matcher    string `json:"matcher"`
			CWD        string `json:"cwd"`
			TimeoutSec int    `json:"timeoutSec"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(content, &root); err != nil {
		return nil, err
	}

	events := make([]string, 0, len(root.Hooks))
	for e := range root.Hooks {
		events = append(events, e)
	}
	sort.Strings(events)

	var tuples []HookTuple
	for _, event := range events {
		entries := root.Hooks[event]
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Matcher != entries[j].Matcher {
				return entries[i].Matcher < entries[j].Matcher
			}
			return entries[i].Bash < entries[j].Bash
		})
		for _, entry := range entries {
			if entry.Type != "command" {
				continue
			}
			// Copilot uses "bash" field for the command.
			tuples = append(tuples, HookTuple{
				Event:      event,
				Matcher:    entry.Matcher,
				RawCommand: entry.Bash,
				ScriptPath: normalizeCommand(entry.Bash),
			})
		}
	}
	return tuples, nil
}

// extractCursorSchema parses Cursor wiring:
// {"hooks": {"preToolUse": [{"command": "..."}], "beforeShellExecution": [{"command": "..."}], ...}}
func extractCursorSchema(content []byte) ([]HookTuple, error) {
	var root struct {
		Hooks map[string][]struct {
			Command string `json:"command"`
			Matcher string `json:"matcher"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(content, &root); err != nil {
		return nil, err
	}

	events := make([]string, 0, len(root.Hooks))
	for e := range root.Hooks {
		events = append(events, e)
	}
	sort.Strings(events)

	var tuples []HookTuple
	for _, event := range events {
		entries := root.Hooks[event]
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Command < entries[j].Command
		})
		for _, entry := range entries {
			cmd := entry.Command
			if cmd == "" {
				continue
			}
			tuples = append(tuples, HookTuple{
				Event:      event,
				Matcher:    entry.Matcher,
				RawCommand: cmd,
				ScriptPath: normalizeCommand(cmd),
			})
		}
	}
	return tuples, nil
}

// extractWindsurfSchema parses Windsurf wiring:
// {"hooks": {"pre_run_command": [{"command": "...", "show_output": true}]}}
func extractWindsurfSchema(content []byte) ([]HookTuple, error) {
	var root struct {
		Hooks map[string][]struct {
			Command string `json:"command"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(content, &root); err != nil {
		return nil, err
	}

	events := make([]string, 0, len(root.Hooks))
	for e := range root.Hooks {
		events = append(events, e)
	}
	sort.Strings(events)

	var tuples []HookTuple
	for _, event := range events {
		entries := root.Hooks[event]
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Command < entries[j].Command
		})
		for _, entry := range entries {
			if entry.Command == "" {
				continue
			}
			tuples = append(tuples, HookTuple{
				Event:      event,
				Matcher:    "*", // Windsurf hooks match all by default
				RawCommand: entry.Command,
				ScriptPath: normalizeCommand(entry.Command),
			})
		}
	}
	return tuples, nil
}

// TupleCount returns the total number of hook tuples across all runtimes.
func (r *Report) TupleCount() int {
	n := 0
	for _, rr := range r.HookWiring {
		n += len(rr.Tuples)
	}
	return n
}

// FormatText returns the human-readable text report.
// This exact format is byte-identical across all 3 CLIs (Go, Node.js, Python).
// Format contract: see docs/cli-parity.md ## trackfw audit-surface
func FormatText(r *Report) string {
	n := r.TupleCount()

	var lines []string
	header := fmt.Sprintf("trackfw audit-surface: %d hook tuple(s) at %s", n, r.Ref)
	lines = append(lines, header)

	// Build the body lines.
	var body []string
	for _, rr := range r.HookWiring {
		if !rr.Present {
			body = append(body, fmt.Sprintf("absent [%s] %s", rr.Runtime, rr.WiringFile))
			continue
		}
		if len(rr.Tuples) == 0 {
			body = append(body, fmt.Sprintf("no_hooks [%s] %s", rr.Runtime, rr.WiringFile))
			continue
		}
		for _, t := range rr.Tuples {
			matcher := t.Matcher
			if matcher == "" {
				matcher = "*"
			}
			line := fmt.Sprintf("hook [%s] %s %s/%s %s %s",
				rr.Runtime, rr.WiringFile,
				t.Event, matcher,
				t.RawCommand,
				t.Digest,
			)
			body = append(body, line)
		}
	}

	for _, f := range r.InstructionFiles {
		if f.Kind == "slash-command" {
			if f.Present {
				body = append(body, fmt.Sprintf("slash-command %s", f.Path))
			}
		} else {
			status := "absent"
			if f.Present {
				status = "present"
			}
			body = append(body, fmt.Sprintf("instruction [%s] %s", status, f.Path))
		}
	}

	for _, lh := range r.LifecycleHooks {
		if lh.Present {
			body = append(body, fmt.Sprintf("lifecycle [present] %s %s %s", lh.File, lh.Key, lh.Command))
		} else {
			body = append(body, fmt.Sprintf("lifecycle [absent] %s %s", lh.File, lh.Key))
		}
	}

	if len(body) == 0 {
		return header + "\n"
	}

	// Join: header + blank line + body lines.
	lines = append(lines, "")
	lines = append(lines, body...)
	return strings.Join(lines, "\n") + "\n"
}

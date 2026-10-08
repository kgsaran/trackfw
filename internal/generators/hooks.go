package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// cliDetectors maps CLI target IDs (same strings as the catalog, same as
// --ai-tools values) to a detection predicate and a hook injector. Shared
// between InjectHooksDetected (detection-based, brownfield) and
// InjectHooksForTools (name-based, post-install). Adding a 9th CLI here
// keeps both paths in sync automatically.
//
// CLIs without an Inject*Hooks implementation (opencode, antigravity) are
// intentionally absent — callers that pass unknown names get a silent skip.
var cliDetectors = map[string]struct {
	inject func(string) error
	detect func(string) bool
}{
	"claude": {
		inject: InjectClaudeHooks,
		detect: func(cwd string) bool {
			_, err1 := os.Stat(filepath.Join(cwd, ".claude"))
			_, err2 := os.Stat(filepath.Join(cwd, "CLAUDE.md"))
			return err1 == nil || err2 == nil
		},
	},
	"codex": {
		inject: InjectCodexHooks,
		detect: func(cwd string) bool {
			_, err1 := os.Stat(filepath.Join(cwd, "AGENTS.md"))
			_, err2 := os.Stat(filepath.Join(cwd, ".codex"))
			return err1 == nil || err2 == nil
		},
	},
	"gemini": {
		inject: InjectGeminiHooks,
		detect: func(cwd string) bool {
			_, err1 := os.Stat(filepath.Join(cwd, "GEMINI.md"))
			_, err2 := os.Stat(filepath.Join(cwd, ".gemini"))
			return err1 == nil || err2 == nil
		},
	},
	"kiro": {
		inject: InjectKiroHooks,
		detect: func(cwd string) bool {
			_, err := os.Stat(filepath.Join(cwd, ".kiro"))
			return err == nil
		},
	},
	"copilot": {
		inject: InjectCopilotHooks,
		detect: func(cwd string) bool {
			_, err := os.Stat(filepath.Join(cwd, ".github", "copilot-instructions.md"))
			return err == nil
		},
	},
	"cursor": {
		inject: InjectCursorHooks,
		detect: func(cwd string) bool {
			_, err := os.Stat(filepath.Join(cwd, ".cursor"))
			return err == nil
		},
	},
	"windsurf": {
		inject: InjectWindsurfHooks,
		detect: func(cwd string) bool {
			_, err := os.Stat(filepath.Join(cwd, ".windsurfrules"))
			return err == nil
		},
	},
	// amazonq (ROADMAP-2026-08-14 ML-3A, path fixed 2026-08-14 by apolo-tf):
	// dispatches InjectAmazonQHooks (git branch guard,
	// .amazonq/cli-agents/q_cli_default.json) whenever the existing
	// textual rules file is present. This entry was missing before this
	// ML — InjectAmazonQHooks did not exist, so there was nothing to
	// dispatch to. Note: the roadmap's ML-3A instructions literally named
	// InjectRulesForTool/InjectRulesDetected (agentfiles.go ~138-181) as
	// the dispatch point to update, but those two functions only ever
	// handle the textual rules block and already support "amazonq" via
	// the agentFiles map — no change was needed there. This function
	// (InjectHooksDetected) is the actual hooks dispatcher missing an
	// amazonq entry, so it is updated here instead; see the ML-3A report
	// for this divergence.
	"amazonq": {
		inject: InjectAmazonQHooks,
		detect: func(cwd string) bool {
			_, err := os.Stat(filepath.Join(cwd, ".amazonq"))
			return err == nil
		},
	},
}

// InjectHooksDetected detecta quais CLIs estão presentes no cwd e injeta os attention hooks
// em cada um. Erros são coletados e retornados como string joined (não para na primeira falha).
func InjectHooksDetected(cwd string) error {
	var errs []string
	for name, d := range cliDetectors {
		if d.detect(cwd) {
			if err := d.inject(cwd); err != nil {
				errs = append(errs, name+": "+err.Error())
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("partial: %s", strings.Join(errs, "; "))
	}
	return nil
}

// InjectHooksForTools calls the Inject*Hooks function for each tool listed in
// tools, regardless of whether detection signals are present on disk.
//
// Use this after writing the CLI's own files (GEMINI.md, .kiro/, etc.) so
// the injector finds or creates its target directory — it closes the ordering
// gap in `trackfw init` where Scaffold runs InjectHooksDetected BEFORE
// installAITools creates the detection signals, leaving 7 of 8 CLIs without
// guard hooks on the first run. InjectKiroHooks calls MkdirAll(.kiro/hooks/)
// itself, so Kiro cause (b) — global scope never creating .kiro/ in the
// project — is also closed here.
//
// Each injector is idempotent (merge semantics for JSON, dedup for credential
// guard vs global harness). Double-call with InjectHooksDetected is safe.
//
// Unknown tool names (e.g. "opencode", "antigravity") are silently skipped.
// Errors are collected; all tools are attempted before returning.
func InjectHooksForTools(tools []string, cwd string) error {
	var errs []string
	for _, tool := range tools {
		d, ok := cliDetectors[tool]
		if !ok {
			continue
		}
		if err := d.inject(cwd); err != nil {
			errs = append(errs, tool+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("partial: %s", strings.Join(errs, "; "))
	}
	return nil
}

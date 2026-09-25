package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// InjectHooksDetected detecta quais CLIs estão presentes no cwd e injeta os attention hooks
// em cada um. Erros são coletados e retornados como string joined (não para na primeira falha).
func InjectHooksDetected(cwd string) error {
	type detector struct {
		fn     func(string) error
		detect func(string) bool
	}

	detections := map[string]detector{
		"claude": {
			fn: InjectClaudeHooks,
			detect: func(cwd string) bool {
				_, err1 := os.Stat(filepath.Join(cwd, ".claude"))
				_, err2 := os.Stat(filepath.Join(cwd, "CLAUDE.md"))
				return err1 == nil || err2 == nil
			},
		},
		"codex": {
			fn: InjectCodexHooks,
			detect: func(cwd string) bool {
				_, err1 := os.Stat(filepath.Join(cwd, "AGENTS.md"))
				_, err2 := os.Stat(filepath.Join(cwd, ".codex"))
				return err1 == nil || err2 == nil
			},
		},
		"gemini": {
			fn: InjectGeminiHooks,
			detect: func(cwd string) bool {
				_, err1 := os.Stat(filepath.Join(cwd, "GEMINI.md"))
				_, err2 := os.Stat(filepath.Join(cwd, ".gemini"))
				return err1 == nil || err2 == nil
			},
		},
		"kiro": {
			fn: InjectKiroHooks,
			detect: func(cwd string) bool {
				_, err := os.Stat(filepath.Join(cwd, ".kiro"))
				return err == nil
			},
		},
		"copilot": {
			fn: InjectCopilotHooks,
			detect: func(cwd string) bool {
				_, err := os.Stat(filepath.Join(cwd, ".github", "copilot-instructions.md"))
				return err == nil
			},
		},
		"cursor": {
			fn: InjectCursorHooks,
			detect: func(cwd string) bool {
				_, err := os.Stat(filepath.Join(cwd, ".cursor"))
				return err == nil
			},
		},
		"windsurf": {
			fn: InjectWindsurfHooks,
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
			fn: InjectAmazonQHooks,
			detect: func(cwd string) bool {
				_, err := os.Stat(filepath.Join(cwd, ".amazonq"))
				return err == nil
			},
		},
	}

	var errs []string
	for name, d := range detections {
		if d.detect(cwd) {
			if err := d.fn(cwd); err != nil {
				errs = append(errs, name+": "+err.Error())
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("partial: %s", strings.Join(errs, "; "))
	}
	return nil
}

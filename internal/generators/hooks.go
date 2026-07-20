package generators

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	injectClaudeHooks  = InjectClaudeHooks
	injectCodexHooks   = InjectCodexHooks
	injectGeminiHooks  = InjectGeminiHooks
	injectKiroHooks    = InjectKiroHooks
	injectCopilotHooks = InjectCopilotHooks
	injectCursorHooks  = InjectCursorHooks
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

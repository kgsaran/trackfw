---
name: trackfw-v11x-site-structure
description: VitePress site layout, base path, and key facts for trackfw 11.x documentation
metadata:
  type: project
---

VitePress site at `site/`. Base path: `/trackfw/`. PT is root locale, EN at `/en/`.

Pages per locale: `index.md`, `guide/getting-started.md`, `guide/commands.md`, `guide/ai-agents.md`.

**Why:** needed to form correct internal links and understand the build pipeline.

**How to apply:** use `/trackfw/` as base in all absolute links; build command is `cd site && npm ci && npm run build`; preview: `npx vitepress preview --port 4173`, then check `http://localhost:4173/trackfw/`.

Key facts as of 2026-10-29 (v11.1.0):
- Go version: 1.25.2 (`go.mod:3`)
- Version output format: `trackfw 11.1.0` (no `v` prefix)
- 10 agent targets: claude, codex, gemini, antigravity, cursor, copilot, windsurf, amazonq, opencode, kiro
- Homebrew tap: `brew tap kgsaran/trackfw && brew install trackfw`
- install.ps1: ships from release after 11.1.0 (`irm .../install.ps1 | iex`)
- Module: `github.com/kgsaran/trackfw`

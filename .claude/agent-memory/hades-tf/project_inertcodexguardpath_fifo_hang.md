---
name: inertcodexguardpath-fifo-hang
description: InertCodexGuardPath usa os.ReadFile — FIFO em HOME/.codex/hooks.json trava update harness indefinidamente
metadata:
  type: project
---

Encontrado 2026-11-17, ML-FA Wave 5, REQ-2026-11-15.

`InertCodexGuardPath` introduzida em commit `11285bcc` (branch feat/cobertura-e-compatibilidade-completas) usa `os.ReadFile(p)` em `agentconfig.go:447`. FIFO em `HOME/.codex/hooks.json` → `os.ReadFile` bloqueia indefinidamente.

Medição: `mkfifo "$H/.codex/hooks.json"; update harness --install-missing --dry-run & sleep 10; kill -0 $PID && echo HUNG_10s` → HUNG_10s.

Mesma causa que `InertClaudeGuardPath` (`agentconfig.go:217`) — pré-existente. Ambas usam plain `os.ReadFile`.

**Caller adicional de InertClaudeGuardPath:** `validator/validator_home_divergence_rule.go:73` — Windows-only (`runtime.GOOS == "windows"`). No macOS, não há FIFO hang via validate.

**Regra Dura:** mesma causa → mesma REQ → mesmo PR. Fix: extrair para `internal/fileutil` (novo leaf package) com O_NONBLOCK + fstat (padrão existente em `internal/validator/regularfile.go` e `internal/translog/`). Não usar Stat+ReadFile (TOCTOU). `validator` importa `agentconfig` (ciclo se agentconfig importasse validator); funções de translog são unexported.

**Why:** FIFO hang pode ser explorado via `mkfifo` no POSIX para travar `update harness` indefinidamente; no Windows, named pipe pode causar mesmo efeito em `validate` (via GOOS guard).
**How to apply:** sempre que InertCodexGuardPath ou InertClaudeGuardPath aparecerem em diff, verificar se foram substituídas pela versão O_NONBLOCK.

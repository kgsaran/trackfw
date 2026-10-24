<!-- trackfw:working-context -->
**Agent:** hades-tf
**Timestamp:** 2026-10-23T14:00:00Z
**Event:** start

**Task:** ML-0A — Threat model das quatro camadas (Wave 0, REQ-2026-10-23)
**Roadmap:** ROADMAP-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-working-context-arquivos-congelados-e-escopo-por-despacho.md
**Branch:** feat/regras-de-escrita-dos-agentes-viram-mecanismo-no-harness
**Worktree:** /private/tmp/trackfw-toml-escape

**Status:** iniciando medição e análise para o modelo de ameaça das quatro camadas (D1–D4).

---

<!-- trackfw:working-context -->
**Agent:** hades-tf
**Timestamp:** 2026-10-23T18:00:00Z
**Event:** finish

**Task:** ML-0A — Threat model das quatro camadas (Wave 0, REQ-2026-10-23)
**Status:** ✅ Concluído

**Deliverable:** `docs/seguranca/2026-10-23-wave0-regras-de-escrita.md`

**Principais decisões desta Wave 0 para os implementadores:**
- D1: assets internos ainda dizem "write to subdir" — ML-1A deve mudar para "report instead"; não só globals
- D2 base do diff: `git merge-base origin/main HEAD` + união de `Files affected` de todas as waves ≤ N; sem âncora forgeable
- D2 âncora anti-forge: barrier lê `Files affected` da base ref (não do disco); `trackfw.yaml` congelado ou lido da base ref
- D3 CLIs cobertos nesta REQ: apenas Claude Code; Codex/Windsurf/AmazonQ são resíduo (sem `file_path` único no payload)
- D3 matcher completo: `Write|Edit|NotebookEdit` (MultiEdit: verificar antes de incluir)
- D3 caminho normalizado: `filepath.EvalSymlinks` em ambos; `os.SameFile` para targets existentes
- D3 congelados hardcoded: `trackfw.yaml`, `.claude/settings*.json`, `.codex/hooks.json`, `.gemini/settings.json`, `.trackfw/leases/`
- D3 fail-closed: campo ausente → DENY; lease ilegível → DENY; qualquer erro → exit 2
- D4 identity: `agent_id` presente = subagente; `agent_type` para nome do lease; slug `[a-zA-Z0-9_-]+`
- Windows: normalizar separadores, case-insensitive, reconhecer MSYS `/c/path`, remover UNC `\\?\`
- ML-3A/3B Files affected preenchidos no roadmap

**Sem modificações em `internal/`** — verificado com `git diff --quiet origin/main -- internal/`.

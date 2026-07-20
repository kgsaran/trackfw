---
status: wip
date: 2026-06-20
req: "docs/req/REQ-2026-06-20-attention-hooks-agent-clis.md"
branch: "feat/attention-hooks-agent-clis"
---

# Roadmap: attention-hooks-agent-clis

> Criado em: 2026-06-20 | Status: 🔄 WIP

REQ: REQ-2026-06-20-attention-hooks-agent-clis.md

## Diagnóstico / Contexto

Pesquisa de hooks em todos os CLIs suportados (2026-06-20):

| CLI | Hook pré-tool | Arquivo de config | Nota |
|-----|--------------|-------------------|------|
| Claude Code | `PreToolUse` (matcher por nome da tool) | `.claude/settings.json` | `AskUserQuestion` é tool nomeada → hook exato |
| Codex CLI | `PermissionRequest` | `.codex/hooks.json` | Evento nativo de aprovação |
| Gemini CLI | `Notification[ToolPermission]` | `.gemini/settings.json` | Evento observável de permissão |
| Kiro | `PreToolUse` | `.kiro/hooks/` | Config declarativa versionável |
| GitHub Copilot | `preToolUse` | `.github/hooks/*.json` | fail-closed |
| Cursor | `preToolUse` (genérico) | `.cursor/hooks.json` | Mais completo dos editores |
| Windsurf | por tipo de ação (sem genérico) | `.windsurf/hooks.json` | Outlier — apenas instrução textual |

**Estratégia:** Para CLIs com `PreToolUse` genérico, o hook dispara para qualquer tool call (incluindo
perguntas quando implementadas como tool). Para Claude Code especificamente, o matcher
`AskUserQuestion` é preciso. O script do hook verifica se `trackfw.yaml` existe antes de agir
(safe no-op em projetos sem trackfw). O arquivo de atenção usa o `roadmap_dir` do `trackfw.yaml`,
com fallback para `docs/roadmaps`.

## Acceptance Criteria

- [ ] `.trackfw-attention.json` aparece no board quando agente executa qualquer tool call interativa
- [ ] Banner some automaticamente quando a interação termina (PostToolUse/AfterTool)
- [ ] Hook configs gerados por `trackfw init` e `trackfw discover --init`
- [ ] `trackfw update` atualiza hooks detectados (mesmo padrão que `InjectRulesDetected`)
- [ ] Scripts são idempotentes (re-rodar não quebra)
- [ ] Paridade Go + Node.js + Python nos geradores
- [ ] Testes verdes

---

## Wave 1 — Scripts shell compartilhados (base para todos os CLIs)
> Dependências: nenhuma

### ML-1A — Scripts `trackfw-attention-signal.sh` e `trackfw-attention-cleanup.sh`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/scaffold.go` — adicionar geração dos dois scripts em `GenerateScripts()`
- `npm/src/generators/init.js` — idem
- `pypi/trackfw/generators/init_gen.py` — idem

**Ações:**

Script `scripts/trackfw-attention-signal.sh`:
```bash
#!/usr/bin/env bash
# trackfw attention signal — PreToolUse/BeforeTool hook
# Escreve .trackfw-attention.json para sinalizar o board do trackfw serve.
# Recebe JSON via stdin com tool_name e tool_input.
set -euo pipefail

INPUT=$(cat)

# Só age se trackfw.yaml existir (no-op em projetos sem trackfw)
[ -f "trackfw.yaml" ] || exit 0

# Extrair mensagem: campo "question" da tool (Claude AskUserQuestion) ou tool_name genérico
if command -v jq &>/dev/null; then
  TOOL=$(echo "$INPUT" | jq -r '.tool_name // ""')
  MSG=$(echo "$INPUT" | jq -r '(.tool_input.question // .tool_input.command // "Agent is executing: \(.tool_name // "unknown")") | .[0:300]')
else
  TOOL=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name',''))" 2>/dev/null || echo "")
  MSG=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print((ti.get('question') or ti.get('command') or 'Agent is executing: '+d.get('tool_name','unknown'))[:300])" 2>/dev/null || echo "Agent needs attention")
fi

# Roadmap dir do trackfw.yaml (fallback: docs/roadmaps)
ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | awk '{print $2}' | tr -d '"'"'" | head -1)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"%s","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$(echo "$TOOL" | sed 's/"/\\"/g')" \
  "$(echo "$MSG"  | sed 's/"/\\"/g')" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-attention.json"

exit 0
```

Script `scripts/trackfw-attention-cleanup.sh`:
```bash
#!/usr/bin/env bash
# trackfw attention cleanup — PostToolUse/AfterTool hook
# Remove .trackfw-attention.json após a tool call concluir.
set -euo pipefail

[ -f "trackfw.yaml" ] || exit 0

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | awk '{print $2}' | tr -d '"'"'" | head -1)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

rm -f "$ROADMAP_DIR/.trackfw-attention.json"
exit 0
```

**Critérios de aceite:**
- [x] `go build ./...` sem erros
- [x] Scripts gerados em `scripts/` por `trackfw init`
- [x] Scripts são idempotentes (re-rodar `init` não duplica)

---

## Wave 2 — Hook configs por CLI (paralelo: MLs independentes)
> Dependências: ML-1A concluído (scripts existem antes de referenciar nos hooks)

### ML-2A — Claude Code: `.claude/settings.json`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` — nova função `InjectClaudeHooks(cwd string) error`
- `internal/discover/discover.go` — chamar `InjectClaudeHooks` quando Claude detectado
- `npm/src/generators/init.js` — idem
- `pypi/trackfw/generators/init_gen.py` — idem

**Ações:**
1. `InjectClaudeHooks(cwd)`:
   - Ler `.claude/settings.json` (criar se não existir com `{}`)
   - Garantir que `hooks.PreToolUse` contém entry com `matcher: "AskUserQuestion"` apontando para `scripts/trackfw-attention-signal.sh`
   - Garantir que `hooks.PostToolUse` contém entry com `matcher: "AskUserQuestion"` apontando para `scripts/trackfw-attention-cleanup.sh`
   - Usar merge (não sobrescrever hooks existentes — append se não houver duplicata)
   - Serializar de volta com indent 2

Formato resultante em `.claude/settings.json`:
```json
{
  "hooks": {
    "PermissionRequest": [
      {
        "matcher": "AskUserQuestion",
        "hooks": [{ "type": "command", "command": "scripts/trackfw-attention-signal.sh" }]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "AskUserQuestion",
        "hooks": [{ "type": "command", "command": "scripts/trackfw-attention-cleanup.sh" }]
      }
    ]
  }
}
```

2. Chamar `InjectClaudeHooks` em `discover --init` quando `.claude/` ou `CLAUDE.md` detectado.

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `.claude/settings.json` existente com outros hooks NÃO é sobrescrito (merge idempotente)
- [ ] `go test ./internal/generators/...` verde

---

### ML-2B — Codex CLI: `.codex/hooks.json`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` — nova função `InjectCodexHooks(cwd string) error`
- `internal/discover/discover.go` — chamar quando `AGENTS.md` ou `.codex/` detectado
- `npm/src/generators/init.js` — idem
- `pypi/trackfw/generators/init_gen.py` — idem

**Ações:**
1. Criar/merge `.codex/hooks.json`:
```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [{ "type": "command", "command": "scripts/trackfw-attention-signal.sh" }]
      }
    ],
    "PostToolUse": [
      {
        "matcher": ".*",
        "hooks": [{ "type": "command", "command": "scripts/trackfw-attention-cleanup.sh" }]
      }
    ]
  }
}
```
Nota: `PermissionRequest` evita sinalizar operações que não exigem intervenção humana.

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] Merge idempotente (não duplica entries)

---

### ML-2C — Gemini CLI: `.gemini/settings.json`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` — `InjectGeminiHooks(cwd string) error`
- detecção: `GEMINI.md` ou `.gemini/` presentes

**Ações:**
1. Criar/merge `.gemini/settings.json` com bloco `hooks`:
```json
{
  "hooks": {
    "Notification": [
      {
        "matcher": "ToolPermission",
        "hooks": [{ "type": "command", "command": "scripts/trackfw-attention-signal.sh" }]
      }
    ],
    "AfterTool": [
      {
        "matcher": "*",
        "hooks": [{ "type": "command", "command": "scripts/trackfw-attention-cleanup.sh" }]
      }
    ]
  }
}
```

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] Merge idempotente

---

### ML-2D — Kiro: `.kiro/hooks/trackfw-attention.json`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` — `InjectKiroHooks(cwd string) error`
- detecção: `.kiro/` presente

**Ações:**
1. Criar `.kiro/hooks/trackfw-attention.json` (arquivo dedicado — Kiro usa um arquivo por hook):
```json
{
  "hooks": [
    {
      "name": "trackfw-attention-signal",
      "description": "Signals trackfw board when agent executes a tool",
      "event": "PreToolUse",
      "matcher": { "tool_name": ".*" },
      "action": { "type": "command", "command": "scripts/trackfw-attention-signal.sh" }
    },
    {
      "name": "trackfw-attention-cleanup",
      "description": "Clears trackfw board attention after tool completes",
      "event": "PostToolUse",
      "matcher": { "tool_name": ".*" },
      "action": { "type": "command", "command": "scripts/trackfw-attention-cleanup.sh" }
    }
  ]
}
```

**Critérios de aceite:**
- [ ] Arquivo criado corretamente
- [ ] Idempotente (re-rodar não duplica)

---

### ML-2E — GitHub Copilot: `.github/hooks/trackfw-attention.json`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` — `InjectCopilotHooks(cwd string) error`
- detecção: `.github/copilot-instructions.md` presente

**Ações:**
1. Criar `.github/hooks/trackfw-attention.json`:
```json
{
  "hooks": [
    {
      "event": "preToolUse",
      "run": "scripts/trackfw-attention-signal.sh"
    },
    {
      "event": "postToolUse",
      "run": "scripts/trackfw-attention-cleanup.sh"
    }
  ]
}
```

**Critérios de aceite:**
- [ ] Arquivo criado corretamente
- [ ] Idempotente

---

### ML-2F — Cursor: `.cursor/hooks.json`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` — `InjectCursorHooks(cwd string) error`
- detecção: `.cursor/` presente

**Ações:**
1. Criar/merge `.cursor/hooks.json`:
```json
{
  "preToolUse": [
    { "command": "scripts/trackfw-attention-signal.sh" }
  ],
  "postToolUse": [
    { "command": "scripts/trackfw-attention-cleanup.sh" }
  ]
}
```

**Critérios de aceite:**
- [ ] Merge idempotente (não duplica entries existentes)

---

### ML-2G — Windsurf: instrução em `.windsurfrules`
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles.go` — atualizar `trackfwRulesBlock()` com instrução específica para Windsurf

**Ações:**
Windsurf não tem `preToolUse` genérico — única opção é instrução textual no bloco injetado:
1. Atualizar `trackfwRulesBlock()` para incluir (após a seção Attention Signal existente):

```
> **Windsurf users:** before asking the user a question or requesting approval, write
> `<roadmap_dir>/.trackfw-attention.json` manually — there is no automatic hook for this.
> Delete the file after the user responds.
```

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] Instrução aparece no `.windsurfrules` gerado

---

## Wave 3 — `trackfw update` + testes
> Dependências: Wave 2 completa

### ML-3A — `trackfw update` detecta e regenera hooks
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/commands/update.go` (ou equivalente) — chamar `InjectXxxHooks` para cada agente detectado
- `npm/src/commands/update.js` — idem
- `pypi/trackfw/commands/update.py` — idem

**Ações:**
1. Em `RunUpdate()` (ou equivalente), após `InjectRulesDetected(cwd)`, chamar:
   - `InjectClaudeHooks(cwd)` se `.claude/` ou `CLAUDE.md` detectado
   - `InjectCodexHooks(cwd)` se `AGENTS.md` ou `.codex/` detectado
   - `InjectGeminiHooks(cwd)` se `GEMINI.md` ou `.gemini/` detectado
   - `InjectKiroHooks(cwd)` se `.kiro/` detectado
   - `InjectCopilotHooks(cwd)` se `.github/copilot-instructions.md` detectado
   - `InjectCursorHooks(cwd)` se `.cursor/` detectado
2. Paridade Node.js e Python.

**Critérios de aceite:**
- [ ] `trackfw update` regenera/atualiza hooks sem sobrescrever config existente
- [ ] `go test ./internal/commands/...` verde

---

### ML-3B — Testes de integração para InjectClaudeHooks
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/agentfiles_test.go` (criar se não existir)

**Ações:**
1. `TestInjectClaudeHooks_Create`: `.claude/settings.json` não existe → cria com hooks corretos
2. `TestInjectClaudeHooks_Merge`: settings.json existente com outros hooks → merge sem sobrescrever
3. `TestInjectClaudeHooks_Idempotent`: rodar duas vezes → não duplica entries

**Critérios de aceite:**
- [ ] `go test ./internal/generators/... -run TestInjectClaudeHooks` verde (3 testes)

---

### ML-3C — Atualizar VISION.md com v2.12.4
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `docs/visao-projeto/VISION.md`

**Ações:**
1. Versão → v2.12.4
2. Adicionar linha v2.8 na tabela Current State
3. Documentar hooks automáticos na seção de `trackfw init`

---

## Protocolo de conclusão de cada ML

```
1. Build       → go build ./... (Go) | npm test (Node) | pytest (Python)
2. Testes      → go test ./... | npm test | pytest
3. Commit      → git commit -m "feat(hooks): <descrição>"
4. Push        → git push origin <branch>
5. Atualizar roadmap → marcar ML como ✅ Concluído
```

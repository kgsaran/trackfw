---
status: wip
date: 2026-07-19
req: "docs/req/REQ-2026-07-19-corrigir-render-antigravity-com-tools-validos-e-model-tier-do-agy.md"
squad: "Apolo, Artemis"
---

# Roadmap: Corrigir render Antigravity tools e model tier

> Created: 2026-07-19 | Status: wip

## Acceptance Criteria
- [ ] Render `agent-directory` mapeia model (opus→pro, sonnet→flash) e omite se ausente
- [ ] Render `agent-directory` injeta `tools:` (architect=14, demais=10) sem IDs proibidos
- [ ] Assets `assets/agents/*.md` inalterados nos 3 CLIs
- [ ] Paridade Go/Node/Python com testes de contrato verdes
- [ ] E2E `init --ai-tools antigravity` gera agents aceitos por `agy` com tools + model tier corretos

## Context
REQ: docs/req/REQ-2026-07-19-corrigir-render-antigravity-com-tools-validos-e-model-tier-do-agy.md

O render do alvo `antigravity` surface `current` (representacao `agent-directory`) emite o asset markdown verbatim, mantendo `model: opus|sonnet` (rejeitado pelo `agy`) e sem `tools:` (agente read-only). Este roadmap adapta o render, nos 3 CLIs, para o schema do Antigravity CLI (`agy`).

`agent-directory` e representacao **exclusiva** do antigravity/current — ramificar por ela nao afeta outras plataformas. **Os assets `assets/agents/*.md` NAO sao alterados.**

## Especificacao compartilhada (identica nos 3 CLIs)

**Gatilho:** somente quando `capability.representation == "agent-directory"` (Go/Node) ou `target == "antigravity" && surface == "current"` (Python), no branch de agents.

**Transformacao do frontmatter (reconstruir a partir de name/description/model/body):**
1. Manter `name` e `description`.
2. Mapear `model`: `opus -> pro`, `sonnet -> flash`; qualquer outro valor conhecido -> manter apenas se for tier valido do agy (`flash_lite|flash|pro`); se ausente ou nao mapeavel -> **omitir a linha `model`**.
3. Injetar bloco `tools:` (lista YAML) conforme o agente:
   - Se o nome do agente termina em `architect` (id `architect` / `trackfw-architect`) -> **SET_ARCH (14)**.
   - Caso contrario -> **SET_IMPL (10)**.
4. Preservar o corpo (body) apos o frontmatter.

**SET_IMPL (10):**
`view_file, list_dir, grep_search, search_web, read_url_content, write_to_file, replace_file_content, run_command, command_status, generate_image`

**SET_ARCH (14):** SET_IMPL + `send_message, define_subagent, invoke_subagent, schedule`

**IDs PROIBIDOS (nunca emitir):** `edit_file, read_file, find, view_code_item, view_file_outline, call_mcp_tool`. Emitir qualquer um quebra o agente no `agy`.

**Formato de saida (exemplo backend):**
```
---
name: trackfw-backend
description: Senior backend specialist for APIs, domain logic, integrations and data access.
model: flash
tools:
  - view_file
  - list_dir
  - grep_search
  - search_web
  - read_url_content
  - write_to_file
  - replace_file_content
  - run_command
  - command_status
  - generate_image
---
<body original preservado>
```

## Wave 1 — Renderers por CLI (3 MLs em paralelo)
> Dependencies: none. Arquivos disjuntos (Go / Node / Python) — spawn simultaneo.

### ML-1A — Render Antigravity no CLI Go
**Status:** pending
**Files affected:** `internal/integrations/render.go`, `internal/integrations/render_test.go`
**Actions:**
- Estender `markdownParts` para capturar tambem `model` (hoje so name/description).
- Adicionar `case "agent-directory":` no `switch capability.Representation` de `Render`, aplicando a Especificacao compartilhada (map model + inject tools por agente). Helper local para SET_IMPL/SET_ARCH e model map.
- Teste: `Render(item, KindAgents, Capability{Representation:"agent-directory"}, source)` para architect (assert `model: pro` + 14 tools, sem `opus`) e backend (assert `model: flash` + 10 tools). Assert que nenhum id proibido aparece.
**Acceptance criteria:**
- [ ] `go test ./internal/integrations/...` verde
- [ ] `make build` sem erros
- [ ] nenhum asset em `assets/agents/` modificado

### ML-1B — Render Antigravity no CLI Node
**Status:** pending
**Files affected:** `npm/src/integrations/render.js`, `npm/tests/agents-skills.test.js`
**Actions:**
- Estender `markdownParts` para capturar `model`.
- Adicionar branch `capability.representation === 'agent-directory'` em `render`, aplicando a Especificacao compartilhada. Consumir `item`/`target` ja passados no call site (index.js:53) se necessario para decidir o set.
- Teste golden (espelho do teste do codex) para architect (14 tools, `model: pro`) e backend (10 tools, `model: flash`).
**Acceptance criteria:**
- [ ] `node --test npm/tests/agents-skills.test.js` verde
- [ ] nenhum asset em `npm/src/integrations/assets/agents/` modificado

### ML-1C — Render Antigravity no CLI Python
**Status:** pending
**Files affected:** `pypi/trackfw/integrations/renderers.py`, `pypi/tests/test_agents_skills.py`
**Actions:**
- Estender `_parts` para capturar `model`.
- Adicionar branch para `target == "antigravity" and surface == "current"` (ou `capability["representation"] == "agent-directory"`) em `render`, aplicando a Especificacao compartilhada.
- Teste: `plan_deployments("agents", ["antigravity"], ["architect"|"backend"], "project")` — assert content com tools corretos + model mapeado + sem ids proibidos.
**Acceptance criteria:**
- [ ] `pytest pypi/tests/test_agents_skills.py` verde
- [ ] nenhum asset em `pypi/trackfw/integrations/assets/agents/` modificado

## Wave 2 — Paridade e E2E (1 ML)
> Dependencies: Wave 1 completa (barrier).

### ML-2 — Validacao de paridade + E2E no agy
**Status:** pending
**Files affected:** nenhum de produto (apenas execucao/validacao)
**Actions:**
- `make quality` (contrato de paridade 3 CLIs) verde.
- E2E por CLI: rodar `init --ai-tools antigravity` apontando `HOME` para um diretorio temporario; conferir:
  - `<tmp>/.gemini/config/agents/trackfw-architect/agent.md`: `tools:` com 14 ids, `model: pro`, sem `opus`.
  - `<tmp>/.gemini/config/agents/trackfw-backend/agent.md`: `tools:` com 10 ids, `model: flash`.
- Confirmar que os 3 CLIs produzem output byte-equivalente para o mesmo agente.
**Acceptance criteria:**
- [ ] `make quality` verde
- [ ] E2E confirma tools + model tier corretos nos 3 CLIs
- [ ] output identico entre Go/Node/Python

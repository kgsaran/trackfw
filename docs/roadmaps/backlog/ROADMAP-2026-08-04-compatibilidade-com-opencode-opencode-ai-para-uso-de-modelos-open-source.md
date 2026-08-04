---
status: backlog
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md"
squad: "prometeu-tf"
---

# Roadmap: compatibilidade com OpenCode (opencode.ai) para uso de modelos open-source

> Created: 2026-08-04 | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md`

Aplica o padrão de adapter já estabelecido pelo ADR-2026-07-18 (catálogo canônico +
`internal/integrations/assets/catalog.json`) a um 10º target: OpenCode (opencode.ai), CLI/TUI de
agente de IA com suporte nativo a 75+ provedores via AI SDK, incluindo modelos open-source
self-hosted (Ollama, LM Studio, llama.cpp) — motivação de negócio deste REQ. Escopo desta primeira
fase: lifecycle `agents`/`skills` (list/install/uninstall/update) + reuso do `AGENTS.md` já existente.
MCP servers, hooks de atenção (plugin JS) e wizard de provider ficam fora (ver negative scope da REQ).

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] Target `opencode` no catálogo canônico, agents+skills instaláveis/atualizáveis/desinstaláveis
      nos 3 CLIs via `--targets opencode`
- [ ] Decisão de representação de agente documentada (reuso vs nova) antes de tocar código de produção
- [ ] `AGENTS.md` confirmado funcionando para projetos OpenCode sem mudança de detecção (ou corrigido
      se a prática divergir da leitura de código)
- [ ] Assets Go canônicos, cópias npm/PyPI byte-idênticas
- [ ] `docs/cli-parity.md` atualizado
- [ ] `make quality` verde, `trackfw validate` sem violações

## Wave 1 — Pesquisa e decisão de representação (bloqueia as waves seguintes)
> Dependencies: none

### ML-1A — Validar o formato real de agente/skill do OpenCode e decidir a representação
**Status:** pending
**Files affected:** nenhum arquivo de produção — só pesquisa e a decisão registrada nesta seção do
roadmap (e num ADR complementar, se necessário)
**Actions:**
1. Instalar o OpenCode real (`npm install -g opencode-ai` ou equivalente conforme a documentação
   oficial em opencode.ai/docs) e criar um projeto de teste mínimo com um agente custom em
   `.opencode/agents/teste.md` e uma skill em `.opencode/skills/teste/SKILL.md`, confirmando que o
   OpenCode de fato os reconhece (`opencode` CLI real, não só a documentação) — isso valida os
   caminhos e o frontmatter descritos no REQ contra o comportamento real, não só a doc.
2. Comparar o frontmatter de agente do OpenCode (`description`, `mode`, `model`, `permission`, etc.)
   com a representação `agent-markdown`/`subagent` já usada em `internal/integrations/render.go` —
   decidir se a função `Render` existente produz um arquivo válido para o OpenCode com pequenos
   ajustes, ou se é necessária uma nova `Representation` (ex: `"opencode-agent"`).
3. Confirmar experimentalmente (não só por leitura de código) que `AGENTS.md` já existente é lido
   pelo OpenCode real do jeito que a documentação promete (precedência, combinação com
   `~/.config/opencode/AGENTS.md`).
4. Registrar a decisão por escrito nesta seção do roadmap (atualizar este ML com o resultado) e, se a
   representação exigir mudança de esquema em `render.go` maior que "adicionar um case", abrir um ADR
   complementar antes da Wave 2.
**Acceptance criteria:**
- [ ] Decisão de representação registrada com justificativa
- [ ] Comportamento real do OpenCode (não só doc) confirmado para agents, skills e AGENTS.md

## Wave 2 — Go: catálogo + adapter (referência comportamental)
> Dependencies: Wave 1 completa

### ML-2A — Adicionar target `opencode` a `internal/integrations/assets/catalog.json`
**Status:** pending
**Files affected:**
- `internal/integrations/assets/catalog.json`
- `internal/integrations/render.go` (se a Wave 1 decidiu por nova representação)
**Actions:** Definir o target `opencode` seguindo exatamente o schema dos 9 targets existentes (surface
`cli`, escopos `global`+`project`, paths para agents/skills conforme decidido na Wave 1).
**Acceptance criteria:**
- [ ] `go build ./...`, `go test ./internal/integrations/...` verdes
- [ ] `trackfw agents list --json` mostra o target `opencode`

### ML-2B — Lifecycle Go completo (install/uninstall/update) + AGENTS.md
**Status:** pending
**Files affected:**
- `internal/generators/agentfiles.go` (só se a Wave 1 revelar necessidade de ajuste na detecção)
- Testes cobrindo `agents`/`skills` `install|uninstall|update` com `--targets opencode`
**Actions:** Confirmar que o lifecycle genérico já cobre o novo target sem código extra (esse é o
ponto central do ADR-2026-07-18: "novas CLIs podem ser adicionadas por adapter sem duplicar o
lifecycle") — se precisar de código além do catálogo, documentar por quê.
**Acceptance criteria:**
- [ ] Testes end-to-end de install/uninstall/update com `--targets opencode` verdes
- [ ] `go test ./internal/...` completo verde

## Wave 3 — Node.js + Python (paralelo entre si)
> Dependencies: Wave 2 completa

### ML-3A — Sincronizar assets + confirmar lifecycle Node.js
**Status:** pending
**Files affected:** `npm/src/integrations/assets/catalog.json` (via `scripts/sync-integration-assets.sh`)
**Acceptance criteria:**
- [ ] `npm test` verde com os novos cenários
- [ ] `bash scripts/check-integration-assets.sh` verde

### ML-3B — Sincronizar assets + confirmar lifecycle Python
**Status:** pending
**Files affected:** `pypi/trackfw/integrations/assets/catalog.json` (via sync script)
**Acceptance criteria:**
- [ ] `python3 -m pytest` verde com os novos cenários
- [ ] `bash scripts/check-integration-assets.sh` verde

## Wave 4 — Documentação e gate de paridade
> Dependencies: Wave 3 completa

### ML-4A — Documentar e validar o gate de paridade de identidade
**Status:** pending
**Files affected:** `docs/cli-parity.md`
**Actions:**
1. Adicionar OpenCode à lista de CLIs suportados por `agents`/`skills` em `docs/cli-parity.md`.
2. Confirmar que `scripts/check-identity-parity.sh` cobre o novo target automaticamente (derivação a
   partir do catálogo, sem lista manual) — se não cobrir, é um bug no gate, corrigir.
**Acceptance criteria:**
- [ ] `make quality` verde
- [ ] `trackfw validate` sem violações

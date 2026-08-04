---
status: Open
date: 2026-08-04
author: "kg.saran@gmail.com"
adr: "docs/adr/ADR-2026-07-18-catalogo-canonico-e-adapters-para-integracoes-de-agentes.md"
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md"
---

# REQ: compatibilidade com OpenCode (opencode.ai) para uso de modelos open-source

> Date: 2026-08-04 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Precisamos rodar os agentes/skills do trackfw sob OpenCode (opencode.ai) para viabilizar o uso de
modelos open-source — OpenCode é AI SDK-based e suporta nativamente Ollama, LM Studio, llama.cpp e
qualquer endpoint OpenAI-compatible via `provider.<id>.npm: "@ai-sdk/openai-compatible"` no
`opencode.json`, o que os CLIs hoje suportados pelo trackfw (Claude Code, Codex, Gemini CLI etc.) não
oferecem da mesma forma.

O trackfw já resolveu exatamente este tipo de extensão via ADR-2026-07-18 (catálogo canônico +
adapters orientados a dados em `internal/integrations/assets/catalog.json`): "novas CLIs podem ser
adicionadas por adapter sem duplicar o lifecycle". Este REQ é a aplicação desse padrão a um 10º
target.

Análise da documentação do OpenCode (opencode.ai/docs) mapeada contra o modelo de adapter atual:

| Conceito trackfw | OpenCode | Observação |
|---|---|---|
| Regras (`agentFiles["codex"] = "AGENTS.md"`, `internal/generators/agentfiles.go`) | `AGENTS.md` (raiz do projeto ou `~/.config/opencode/AGENTS.md`) | **Já funciona sem mudança de código** — `InjectRulesDetected` injeta no path "codex" sempre que `AGENTS.md` já existe no cwd, independente de qual ferramenta o criou; confirmado lendo `internal/generators/agentfiles.go:150-176` |
| Agentes (`agent-markdown`/`subagent`, ver `internal/integrations/render.go`) | `.opencode/agents/<nome>.md` (projeto) / `~/.config/opencode/agents/<nome>.md` (global) — YAML frontmatter (`description` obrigatório, `mode: primary\|subagent\|all`, `model`, `permission`) + corpo como system prompt | Formato próximo do `agent-markdown` já usado por Cursor (ver `catalog.json`, target `cursor`), mas o frontmatter tem campos próprios (`mode`, `permission` por ferramenta) — decidir na Wave 1 se reaproveita a representação existente ou precisa de `opencode-agent` nova em `render.go` |
| Skills (`SKILL.md`) | `.opencode/skills/<nome>/SKILL.md` (projeto) / `~/.config/opencode/skills/<nome>/SKILL.md` (global) — **e o OpenCode também lê nativamente `.claude/skills/<nome>/SKILL.md`** | Mesmo formato de frontmatter (`name`, `description`) já usado hoje; caminho `.opencode/skills/` é o adapter próprio, não depender do fallback `.claude/` |
| MCP servers | chave `mcp` em `opencode.json`, shape `{type: "local"|"remote", command/url, headers, ...}` | Fora do escopo deste REQ (ver negative scope) |
| Attention hooks (JSON `PreToolUse`/`PostToolUse` + shell script, ver `internal/generators/hooks.go`) | **Plugins JS/TS** em `.opencode/plugins/*.js`, hooks `tool.execute.before`/`tool.execute.after` | Gap real de arquitetura — não é JSON+shell como os outros 9 targets. Fora do escopo deste REQ (ver negative scope) |

## Acceptance Criteria
- [ ] Novo target `opencode` em `internal/integrations/assets/catalog.json` (surface `cli`, escopos
      `global`+`project`, paths conforme tabela acima), seguindo exatamente o schema já usado pelos
      9 targets existentes (ver target `cursor` como referência mais próxima de formato)
- [ ] `trackfw agents list|install|uninstall|update` e `trackfw skills list|install|uninstall|update`
      reconhecem `--targets opencode` nos três runtimes (Go, Node.js, Python) — mesmo contrato que os
      9 targets existentes já têm (ADR-2026-07-18, decisão 3)
- [ ] Decisão registrada (nesta REQ ou em ADR complementar, se a mudança for grande o suficiente):
      reaproveitar a representação `agent-markdown`/`subagent` existente em `render.go` para o
      frontmatter de agente do OpenCode, ou introduzir uma nova representação — com justificativa
      escrita, não só código
- [ ] Confirmar e testar que a injeção de `AGENTS.md` (`internal/generators/agentfiles.go`) já
      funciona para projetos com OpenCode sem exigir mudança — se não funcionar como esperado na
      prática, ajustar a detecção
- [ ] Assets Go são a fonte canônica; cópias npm/PyPI byte-idênticas via `scripts/sync-integration-assets.sh`
      (mesmo contrato de `scripts/check-integration-assets.sh`)
- [ ] `docs/cli-parity.md` atualizado: `opencode` na lista de CLIs suportados por `agents`/`skills`
      (a linha "The catalog ships 12 agents... ships to Claude Code, Codex, Gemini CLI, Antigravity,
      Cursor, GitHub Copilot, Windsurf, Amazon Q, and Kiro" ganha OpenCode)
- [ ] `scripts/check-identity-parity.sh` cobre o novo target automaticamente (o gate já deriva
      cobertura do catálogo — "a new agent-capable catalog surface enters the gate without editing a
      manual target list", conforme `docs/cli-parity.md` § Parity gate — só confirmar que isso se
      sustenta na prática)
- [ ] `make quality` verde (Go/Node/Python/paridade/falsificação)
- [ ] `trackfw validate` sem violações

## Negative scope (explícito)
- **MCP servers do OpenCode** (chave `mcp` em `opencode.json`) — não fazem parte deste REQ. O trackfw
  hoje não gerencia configuração de MCP servers para nenhum target; adicionar isso pro OpenCode sem
  precedente nos outros 9 seria escopo novo, não extensão do adapter existente.
- **Attention hooks via plugin JS/TS** (`.opencode/plugins/*.js`, `tool.execute.before/after`) — fora
  de escopo. É uma arquitetura de hook fundamentalmente diferente da usada pelos 9 targets atuais
  (JSON declarativo + script shell); vazando essa complexidade pra este REQ arriscaria misturar duas
  decisões de design distintas. Fica registrado aqui como trabalho futuro, com REQ própria quando
  houver decisão sobre como gerar plugins JS a partir do catálogo (ou se vale a pena, dado que só um
  target usaria esse mecanismo).
- **Geração/wizard de `provider` (Ollama/LM Studio/etc.) no `opencode.json`** — é configuração de
  usuário/projeto sobre *qual modelo* rodar, não sobre *quais agentes/skills* instalar. Não faz parte
  do lifecycle `agents`/`skills` que este REQ estende. Pode virar um passo opcional em
  `trackfw configure`/`init` numa REQ futura, se desejado.
- **Comando standalone `trackfw opencode`** (alias de compatibilidade, como `gemini`/`cursor`/etc. têm
  hoje) — não é necessário; esses aliases existem só por compatibilidade histórica com instaladores
  anteriores ao catálogo unificado (ADR-2026-07-18, decisão 8). OpenCode nasce direto no modelo novo.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-07-18-catalogo-canonico-e-adapters-para-integracoes-de-agentes.md — este REQ
aplica a decisão já aceita (adapter orientado a dados) a um 10º target; não deveria exigir uma nova
decisão arquitetural, a menos que a Wave 1 (pesquisa/design da representação de agente) revele que o
formato de frontmatter do OpenCode não cabe em nenhuma representação existente — nesse caso, abrir um
ADR complementar antes de prosseguir.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/backlog/ROADMAP-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source.md`

# REQ — trackfw como trilho de governança para agentes de IA

**Status:** Open  
**Data:** 2026-06-13  
**Autor:** Zeus  
**ADR:** ADR-001-trackfw-como-trilho-de-governanca-para-agentes-ia.md  
**Roadmap:** trackfw-ai-agent-rail-2026-06-13.md

---

## Problema

O trackfw v2.0.0 resolve governança para *times humanos*, mas o uso real mostra que agentes de IA (Claude Code, Gemini CLI, Cursor) já operam sobre a cadeia ADR→REQ→ROADMAP como trilho de orquestração. Esse padrão não está formalizado nem exposto como proposta de valor — é white space sem competição direta.

## Requisito

Evoluir o trackfw para ser o **framework de referência de governança para desenvolvimento orquestrado por agentes de IA**, materializando 5 capabilities:

1. **Frontmatter estruturado** em ADR/REQ/ROADMAP (YAML parseable por LLMs sem ambiguidade)
2. **`trackfw context`** — dump de contexto de governança em formato consumível por LLM (ADRs aceitos + REQs abertas + WIP atual + GovernanceScore)
3. **`trackfw roadmap new --from-req`** — geração assistida de microlotes a partir do conteúdo da REQ
4. **`trackfw serve --mcp`** — servidor MCP que expõe a cadeia como recursos para coding agents
5. **Schema de validação** (JSON Schema) para ADR/REQ/ROADMAP, usado em `validate` e por agentes externos

## Critérios de aceite

- [ ] `trackfw context` emite JSON/Markdown consumível por LLM em < 1s
- [ ] `trackfw roadmap new --from-req REQ-xxx.md` gera rascunho de ROADMAP com MLs extraídos da REQ
- [ ] `trackfw serve --mcp` expõe recursos MCP compatíveis com Claude Code / Cursor / Gemini CLI
- [ ] Schema JSON publicado em `docs/schema/` e validado em `trackfw validate`
- [ ] Templates de ADR/REQ/ROADMAP atualizados com frontmatter YAML obrigatório
- [ ] Paridade Go CLI + npm CLI em todos os comandos novos
- [ ] `go test ./...` verde, `node --check` limpo

## Não inclui

- Geração de código por LLM (fora do escopo do trackfw)
- Analytics de pessoas/times (SPACE, capitalização)
- UI hospedada multi-usuário

## Blocked by ADRs

- ADR-001-trackfw-como-trilho-de-governanca-para-agentes-ia.md

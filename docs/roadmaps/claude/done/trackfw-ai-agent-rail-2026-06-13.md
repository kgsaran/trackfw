# Roadmap: trackfw como trilho de governança para agentes de IA

> Criado em: 2026-06-13 | Status: 🔄 WIP

**REQ:** REQ-2026-06-13-trackfw-ai-agent-governance-rail.md  
**ADR:** ADR-001-trackfw-como-trilho-de-governanca-para-agentes-ia.md

---

## Diagnóstico / Contexto

Análise competitiva de 2026-06-13 identificou white space estratégico: nenhuma ferramenta liga ADR→REQ→ROADMAP num grafo verificável que agentes de IA possam consumir, produzir e validar autonomamente. OpenSpec e GSD sinalizam demanda crescente por estrutura formal para agentes, mas param na spec ou no contexto. O trackfw já opera nesse modo internamente — falta formalizar e expor como produto.

Target: v2.1.x — v2.3.x (3 waves independentes)

---

## Wave 1 — Frontmatter estruturado + `trackfw context` (fundação)
> Independente das demais waves

### ML-1A — Frontmatter YAML obrigatório nos templates de ADR/REQ/ROADMAP
**Status:** ✅ Concluído  
**Arquivos afetados:**
- `internal/generators/adr.go` — template ADR
- `internal/generators/req.go` — template REQ
- `internal/generators/roadmap.go` — template ROADMAP
- `npm/src/generators/adr.js`, `req.js`, `roadmap.js` — paridade npm

**Ações:**
- Adicionar bloco `---\nstatus: Draft\ndate: YYYY-MM-DD\nauthor: \nadr: \nreq: \n---` no topo de cada template
- Garantir que `trackfw validate` leia frontmatter para validações (status, links)

**Critérios de aceite:**
- [ ] ADR, REQ e ROADMAP gerados têm frontmatter YAML parseable
- [ ] `go test ./...` verde, `node --check` limpo

### ML-1B — `trackfw context` — dump de contexto consumível por LLM
**Status:** ✅ Concluído  
**Arquivos afetados:**
- `internal/commands/context.go` (novo)
- `internal/generators/context.go` (novo)
- `npm/src/commands/context.js` (novo)

**Ações:**
- Comando `trackfw context [--format=md|json]`
- Saída: ADRs com status Accepted, REQs Open, ROADMAP em wip, GovernanceScore
- JSON mode: estrutura `{ adrs: [], reqs: [], wip: [], score: int }`
- Markdown mode: bloco formatado para colar em prompt de agente

**Critérios de aceite:**
- [ ] `trackfw context` emite em < 1s
- [ ] `trackfw context --format=json` produz JSON válido
- [ ] Paridade Go + npm

---

## Wave 2 — `trackfw roadmap new --from-req` (geração assistida)
> Depende de ML-1A (frontmatter nos templates)

### ML-2A — Flag `--from-req` no `roadmap new`
**Status:** ✅ Concluído  
**Arquivos afetados:**
- `internal/commands/roadmap.go` — adicionar flag `--from-req`
- `internal/generators/roadmap.go` — parser de REQ para extração de MLs
- `npm/src/commands/roadmap.js`, `npm/src/generators/roadmap.js` — paridade npm

**Ações:**
- `trackfw roadmap new --from-req docs/req/REQ-xxx.md`
- Extrair: título da REQ, critérios de aceite como MLs rascunho, ADR linkada
- Gerar ROADMAP pré-preenchido com seção `## Wave 1` e MLs baseados nos critérios de aceite da REQ
- Respeitar `--title` e `--req` existentes (composição de flags)

**Critérios de aceite:**
- [ ] ROADMAP gerado tem frontmatter, título, REQ linkada e pelo menos 1 ML rascunho extraído da REQ
- [ ] Paridade Go + npm

---

## Wave 3 — Schema JSON (validação estrutural para agentes)
> Independente das waves 1 e 2

### ML-3A — Schema JSON para ADR/REQ/ROADMAP
**Status:** ✅ Concluído  
**Arquivos afetados:**
- `docs/schema/adr.schema.json` (novo)
- `docs/schema/req.schema.json` (novo)
- `docs/schema/roadmap.schema.json` (novo)
- `internal/validator/validator.go` — validação via schema em `trackfw validate`

**Ações:**
- Definir JSON Schema para cada artefato (campos obrigatórios: status, date, author, links)
- `trackfw validate` usa schema para validação estrutural do frontmatter
- Publicar schemas em `docs/schema/` (consumível por agentes via URL relativa)

**Critérios de aceite:**
- [ ] Schemas válidos (JSON Schema Draft-07)
- [ ] `trackfw validate` rejeita artefatos com frontmatter inválido
- [ ] `go test ./...` verde

---

## Verificação end-to-end

```bash
# Contexto para agente
trackfw context --format=json | jq '.score'

# Criar roadmap a partir de REQ
trackfw roadmap new --from-req docs/requisições/claude/REQ-2026-06-13-trackfw-ai-agent-governance-rail.md

# Validar com schema
trackfw validate
```

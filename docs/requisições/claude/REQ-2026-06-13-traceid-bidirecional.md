---
name: REQ-2026-06-13-traceid-bidirecional
title: "v2.5 — req_id: ID de rastreabilidade estável com verificação bidirecional"
status: Open
adr: —
roadmap: docs/roadmaps/claude/wip/v2.5-discovery-json-traceid-2026-06-13.md
created: 2026-06-13
author: zeus
---

# REQ — req_id bidirecional

## Contexto

O pareamento atual do trackfw é unidirecional por presença textual (`REQ:`, `ADR:`, `Roadmap:`).
Isso não garante reciprocidade, compatibilidade de estado nem unicidade lógica.

Origem: `docs/analise-cmdb/achado-upstream-id-rastreabilidade-e-json.md` (Upstream 1).
O gate interno do CMDB já implementa isso (R9/R10 de `scripts/validate-kanban-gate.mjs`)
e pode servir de espelho de lógica.

---

## Proposta

Introduzir um **identificador estável opcional** no frontmatter — `req_id` — presente em ambos
os lados do par REQ↔Roadmap. Quando o campo `trace_id_field` está configurado, `validate` verifica:

1. **Existência bidirecional:** o `req_id` do Roadmap existe em alguma REQ **e** a REQ aponta de
   volta (campo `roadmap:` que resolve para aquele Roadmap).
2. **Compatibilidade de estado:** REQ e Roadmap com o mesmo `req_id` na mesma pasta de estado
   (`wip`/`done`/`backlog`).
3. **Unicidade lógica:** nenhum `req_id` em >1 REQ nem em >1 Roadmap.

### Configuração (opt-in)

```yaml
trace_id_field: req_id   # se vazio/ausente, usa só o pareamento textual atual
```

Projetos sem `trace_id_field` permanecem idênticos — retrocompatível total.

---

## Critérios de aceite

- [ ] `trace_id_field: req_id` no `trackfw.yaml` ativa a verificação bidirecional
- [ ] Ausência do campo no `trackfw.yaml` → comportamento atual inalterado
- [ ] Roadmap com `req_id` sem REQ correspondente → violation `traceid_orphan_roadmap`
- [ ] REQ com `req_id` sem Roadmap correspondente → violation `traceid_orphan_req`
- [ ] REQ em `done/` + Roadmap em `wip/` com mesmo `req_id` → violation `traceid_state_mismatch`
- [ ] Mesmo `req_id` em >1 REQ → violation `traceid_duplicate_req`
- [ ] Mesmo `req_id` em >1 Roadmap → violation `traceid_duplicate_roadmap`
- [ ] Par REQ+Roadmap válido com mesmo `req_id` → sem violation
- [ ] Paridade nos 3 CLIs (Go · Node.js · Python)
- [ ] Testes novos cobrindo todos os cenários nos 3 CLIs

---

## Não está no escopo

- Verificação de ADRs pelo `req_id` (somente REQ↔Roadmap nesta versão)
- Geração automática de `req_id` (campo manual no frontmatter)
- Migração de arquivos existentes

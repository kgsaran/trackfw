---
name: REQ-2026-06-14-traceid-by-agent-support
title: "v2.5.2 — Fix: trace_id_field checks não funcionam com roadmap_namespacing: by_agent"
status: Open
adr: —
roadmap: docs/roadmaps/claude/wip/v2.5.2-traceid-by-agent-2026-06-14.md
created: 2026-06-14
author: zeus
---

# REQ — v2.5.2 Fix: traceid checks ignoram layout by_agent

## Contexto

Achado bloqueante reportado pelo agente do CMDB após tentativa de migração de R9/R10
para o trackfw (ADR-039 §4).
Arquivo de origem: `docs/analise-cmdb/achado-v2.5.1-traceid-nao-suporta-by-agent.md`

---

## Bug — `collectTraceIdEntries` não varre `rootDir/<agente>/<estado>/`

Em projetos com `roadmap_namespacing: by_agent`, os 5 checks `traceid_*` nunca disparam.
O scanner varre apenas `rootDir/<estado>/` (flat), mas em `by_agent` a estrutura é
`rootDir/<agente>/<estado>/` — um nível mais profundo. Resultado: índices vazios,
zero violações, exit 0 — **falso verde silencioso**.

### Causa raiz

`collectTraceIdEntries` (Go) / `checkTraceIds` (Node.js + Python) não recebem informação
de namespacing. Só implementam o layout flat. O `resolveWIPDirs` já resolve isso para
o WIP limit — a correção é reusar a mesma lógica.

### Correção

Passar `ProjectConfig` (ou namespacing + agents) para o scanner de roadmaps:

- Se `roadmap_namespacing == "by_agent"`: varrer `rootDir/<agente>/<estado>/` para cada
  agente (da lista `agents` do config, ou descobrindo subpastas do `roadmap_dir`).
- Caso contrário (flat): comportamento atual preservado.
- Estado derivado da subpasta de estado — necessário para `traceid_state_mismatch`.
- REQs (`req_dir`) nunca usam by_agent — scanner de REQs inalterado.

### Salvaguarda adicional

Quando `trace_id_field` está setado mas `collectTraceIdEntries` retorna 0 entradas em
ambos os lados (REQ e Roadmap), emitir warning de configuração:
> `"trace_id_field is set but no REQ/Roadmap entries were indexed — check req_dir, roadmap_dir and roadmap_namespacing"`

Isso transforma silêncio em sinal detectável.

---

## Critérios de aceite

- [ ] `trackfw validate` com `roadmap_namespacing: by_agent` + `trace_id_field` → checks traceid disparam
- [ ] `traceid_orphan_roadmap`, `traceid_orphan_req`, `traceid_state_mismatch`, `traceid_duplicate_req`, `traceid_duplicate_roadmap` funcionam em layout by_agent
- [ ] Layout flat (sem by_agent) sem regressões
- [ ] Warning emitido quando `trace_id_field` ativo mas 0 entradas indexadas
- [ ] Testes atualizados nos 3 CLIs cobrindo cenário by_agent
- [ ] Paridade nos 3 CLIs (Go · Node.js · Python)

---

## Não está no escopo

- Suporte a `req_dir` com namespacing por agente (REQs sempre flat)
- Refatoração de outros checks além dos traceid_*

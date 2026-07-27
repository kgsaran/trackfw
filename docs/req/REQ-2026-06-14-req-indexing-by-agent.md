---
status: Done
date: 2026-06-14
author: zeus
adr: ""
roadmap: "docs/roadmaps/done/v2.5.3-req-indexing-by-agent-2026-06-14.md"
---

# REQ — v2.5.3 Fix: REQ scanner não percorre layout by_agent

> Date: 2026-06-14 | Status: Done

## Contexto

Achado bloqueante reportado pelo agente do CMDB após revalidação pós-v2.5.2.
Arquivo de origem: `docs/analise-cmdb/achado-v2.5.2-req-indexing-by-agent-incompleto.md`

A v2.5.2 corrigiu a coleta de **Roadmaps** em layout `by_agent`, mas a coleta de **REQs**
permanece plana (`req_dir/*.md`). Em projetos que organizam REQs por `req_dir/<agente>/<estado>/`
(ADR-036), todas as checagens de REQ ficam inertes e os checks de traceid geram 15 falsos
positivos `traceid_orphan_roadmap` no CMDB real.

---

## Bug — Todos os validators de REQ usam `listDir` flat

Em projetos com `roadmap_namespacing: by_agent`, a estrutura de REQs é:
`req_dir/<agente>/<estado>/arquivo.md`

Todos os validators leem apenas `req_dir/*.md` (não recursivo), resultando em:

1. **traceid**: REQs (0) indexadas → 15 `traceid_orphan_roadmap` falsos no CMDB
2. **validateREQsHaveADR**: não encontra REQs → check inerte
3. **validateREQsHaveRoadmap**: não encontra REQs → check inerte
4. **validateFrontmatterPresence (REQ)**: não encontra REQs → check inerte
5. **validateRefTargetsExist**: não encontra REQs → referências inválidas não detectadas
6. **blockedREQs / validateREQsNotBlockedByDraftADRs**: não encontra REQs → checks inertes

### Causa raiz

O tratamento `by_agent` foi aplicado somente ao scanner de Roadmaps na v2.5.2.
A coleta de REQs (`_index_reqs`, `listDir(cfg.REQDir)`, `listDir(cfg.reqDir)`) permanece
plana em todos os 3 CLIs.

### Correção

Adicionar helper `resolveREQFiles(cfg)` / `resolve_req_files(cfg)` / `resolveReqFiles(cfg)`
que retorna paths completos de todos os `.md` de REQ, consciente de `roadmap_namespacing`:

- Se `roadmap_namespacing == "by_agent"`: percorre `req_dir/<agente>/<estado>/`
- Caso contrário: `req_dir/*.md` flat (comportamento atual preservado)

Substituir **todos** os pontos de coleta flat de REQs nos 3 CLIs por chamadas ao helper.

### Melhoria da salvaguarda

A salvaguarda atual dispara só quando **ambos** os lados (REQ e Roadmap) retornam 0.
O caso do CMDB (Roadmaps 116, REQs 0) passa sem aviso. Ampliar para disparar também quando
**apenas um** dos lados é 0 mas o outro não.

---

## Critérios de aceite

- [ ] `trackfw context` com `roadmap_namespacing: by_agent` indexa REQs em `req_dir/<agente>/<estado>/`
- [ ] `traceid_orphan_roadmap` não dispara para Roadmaps com REQ pareada em layout by_agent
- [ ] `validateREQsHaveADR`, `validateREQsHaveRoadmap` e demais checks encontram REQs nested
- [ ] Layout flat sem regressões
- [ ] Salvaguarda dispara quando REQs = 0 mas Roadmaps > 0 (ou vice-versa)
- [ ] Testes atualizados nos 3 CLIs cobrindo cenário by_agent para REQs
- [ ] Paridade nos 3 CLIs (Go · Node.js · Python)

---

## Não está no escopo

- Refatoração de checagens além das listadas acima
- Suporte a namespacing diferente de `by_agent` e flat

## Linked ADR

ADR:

## Linked Roadmap

Roadmap: v2.5.3-req-indexing-by-agent-2026-06-14.md

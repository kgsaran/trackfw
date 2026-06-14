---
name: REQ-2026-06-14-context-req-by-agent
title: "v2.5.4 — Fix: context + validateADRsAreReferenced não encontram REQs em layout by_agent"
status: Open
adr: —
roadmap: docs/roadmaps/claude/wip/v2.5.4-context-req-by-agent-2026-06-14.md
created: 2026-06-14
author: zeus
---

# REQ — v2.5.4 Fix: context e validateADRsAreReferenced cegos a REQs by_agent

## Contexto

Achado residual pós-v2.5.3 reportado pelo agente do CMDB.
Arquivo de origem: `docs/analise-cmdb/achado-v2.5.3-residual-context-req-flat.md`

A v2.5.3 corrigiu a coleta de REQs nos validators e no traceid, mas os comandos
`context` nos 3 CLIs e a função `validateADRsAreReferenced` (Go) ainda usam
`os.ReadDir`/`readdirSync`/`os.listdir` planos. Em projetos com
`roadmap_namespacing: by_agent`, `trackfw context` exibe `## REQs (0)`
mesmo com REQs presentes em `req_dir/<agente>/<estado>/`.

---

## Bug

Pontos flat residuais por CLI:

| Ponto | Go | Node.js | Python |
|-------|----|---------|--------|
| `context` REQ scan | `os.ReadDir(cfg.REQDir)` | `collectEntries(reqDir, 'REQ')` | `_collect_entries(req_dir, "REQ")` |
| `validateADRsAreReferenced` | `os.ReadDir(cfg.REQDir)` | n/a | n/a |

Os Roadmaps em `context` já têm tratamento by_agent nos 3 CLIs — só a parte REQ ficou.

## Correção

### Go (`internal/generators/context.go`)
Substituir o bloco `os.ReadDir(cfg.REQDir)` por varredura by_agent-aware,
espelhando exatamente o padrão de Roadmaps que já existe no mesmo arquivo
(linha 74+): descobrir agentes via `cfg.Agents` ou `os.ReadDir`, iterar
`req_dir/<agente>/<estado>/`.

### Go (`internal/validator/validator.go`)
Na função `validateADRsAreReferenced`: substituir `os.ReadDir(cfg.REQDir)`
por `resolveREQFiles(cfg)` (já existe desde v2.5.3) e adaptar o loop para
ler os paths completos retornados.

### Node.js (`npm/src/commands/context.js`)
A função `collectEntries` é flat. Para REQs em by_agent, chamar
`collectEntries` para cada `req_dir/<agente>/<estado>/`, acumulando resultados
— ou adicionar lógica by_agent antes da chamada atual.

### Python (`pypi/trackfw/commands/context.py`)
Mesmo padrão do Node.js: `_collect_entries` é flat; iterar by_agent antes
de chamar a função para cada subdiretório `req_dir/<agente>/<estado>/`.

---

## Critérios de aceite

- [ ] `trackfw context` com `roadmap_namespacing: by_agent` exibe REQs corretas (>0)
- [ ] `validateADRsAreReferenced` lê conteúdo de REQs em layout by_agent (Go)
- [ ] Layout flat sem regressões
- [ ] Testes nos 3 CLIs cobrindo cenário by_agent para context
- [ ] Paridade nos 3 CLIs (Go · Node.js · Python)

## Não está no escopo

- Outras funções já corrigidas na v2.5.3

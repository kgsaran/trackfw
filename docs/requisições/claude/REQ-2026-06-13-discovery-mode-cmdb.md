---
name: REQ-2026-06-13-discovery-mode-cmdb
title: "Discovery Mode — Suporte a repositórios densos como o CMDB"
status: Open
linked_adr: —
linked_roadmap: docs/roadmaps/claude/wip/v2.5-discovery-json-traceid-2026-06-13.md
created: 2026-06-13
author: zeus
---

# REQ: Discovery Mode — Suporte a repositórios densos como o CMDB

| Campo | Valor |
|---|---|
| Status | Open |
| Criado | 2026-06-13 |
| Roadmap | [v2.1-discovery-mode-2026-06-13](../../../roadmaps/claude/wip/v2.1-discovery-mode-2026-06-13.md) |

---

## Motivação

O projeto CMDB possui governança rica (78 ADRs, 27 REQs, 110 roadmaps em 6 agentes) mas com
estrutura incompatível com o trackfw atual: paths em português (`docs/requisições/`), subdivisão
por agente em dois níveis (`docs/roadmaps/<agente>/<estado>/`) e sem `trackfw.yaml`.

O trackfw precisa ser capaz de:
1. Ler paths de governança de qualquer estrutura de diretórios via `trackfw.yaml` configurável
2. Operar com roadmaps namespaceados por agente/squad em dois níveis hierárquicos
3. Escanear um repositório existente e auto-descobrir sua estrutura de governança

---

## Regra Dura: Paridade Go CLI + npm CLI

Toda feature implementada no Go CLI (`internal/`) DEVE ter paridade completa no npm CLI (`npm/src/`).
Nenhum critério de aceite está satisfeito sem implementação em AMBOS os CLIs.

---

## Critérios de Aceite

### Bloco B — Paths Configuráveis
- [ ] `trackfw.yaml` aceita `adr_dirs` (lista), `req_dir` e `roadmap_dir`
- [ ] `trackfw validate`, `status`, `serve`, `metrics` usam os paths do `trackfw.yaml`
- [ ] Ausência de `trackfw.yaml` ou campos omitidos: fallback para defaults (`docs/adr`, `docs/req`, `docs/roadmaps`) — retrocompatível com v1/v2
- [ ] Paridade npm: `npm/src/` lê os mesmos campos do `trackfw.yaml`

### Bloco C — Namespacing por Agente
- [ ] `trackfw.yaml` aceita `roadmap_namespacing: by_agent` e `agents: [...]`
- [ ] Em modo `by_agent`: `roadmap move`, `roadmap list`, `roadmap show` operam na hierarquia `docs/roadmaps/<agente>/<estado>/`
- [ ] `trackfw validate` valida WIP limit por agente quando `roadmap_namespacing: by_agent`
- [ ] `trackfw status` exibe breakdown por agente
- [ ] Paridade npm completa

### Bloco A — trackfw discover
- [ ] `trackfw discover` escaneia CWD e imprime relatório com: ADR dirs, REQ dir, contagens, namespacing detectado, agentes, governance score (0-100)
- [ ] `trackfw discover --init` gera `trackfw.yaml` calibrado para a estrutura encontrada + `governance_mode: lenient`
- [ ] `trackfw discover --bootstrap-log` cria `.trackfw-log` retroativo a partir dos arquivos em `done/`
- [ ] Detecta `docs/requisições/` (português) como REQ dir válido
- [ ] Detecta modo `by_agent` automaticamente pela presença de `docs/roadmaps/*/wip/`
- [ ] Aplicado no CMDB: `trackfw discover --init` gera config correta + `trackfw status` exibe 6 agentes
- [ ] Paridade npm completa

---

## Fora de Escopo
- Migração automática de arquivos (discover apenas lê, nunca move)
- Suporte a múltiplos `roadmap_dir` (uma raiz apenas, com subdirs por agente)

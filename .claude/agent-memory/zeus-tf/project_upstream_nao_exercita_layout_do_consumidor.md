---
name: upstream-nao-exercita-layout-do-consumidor
description: O trackfw se valida a si mesmo com req_dir flat e roadmap_namespacing flat, então regras que dependem do layout do consumidor (REQ em pasta de estado, by_agent) nunca são exercitadas aqui — é uma família recorrente de defeitos, não casos isolados
metadata:
  type: project
---

O `trackfw.yaml` deste repositório usa `req_dir: docs/req` **sem subpastas de estado** e
`roadmap_namespacing: flat`. Consumidores usam `by_agent` e REQs dentro de `backlog/`, `wip/`, etc.

**Consequência:** toda regra do validador cujo comportamento depende do layout do consumidor
**passa verde no upstream por construção**, porque o upstream não constrói aquele estado.

Ocorrências medidas desta mesma família:
- **#396** — teste lia `docs/roadmaps/done` plano e reprovava consumidor `by_agent`
- **#435** — `traceid_orphan_req` e `req_has_roadmap` disparam para REQ em `backlog/`, onde não ter
  roadmap é o estado correto (ADR-036). Aqui dão 0 porque as REQs são flat (`state = ""`)

**Why:** não é azar nem descuido pontual — é uma **superfície inteira sem cobertura**, e o sinal de
que ela falhou chega sempre de fora (issue de consumidor), nunca do CI.

**How to apply:** ao triar issue de consumidor sobre `validate`/`status`, verifique **primeiro** se o
upstream sequer consegue reproduzir o estado; se não conseguir, a correção precisa de **fixture** que
construa o layout do consumidor, senão o gate que "prova" a correção continua vacuoso. E ao enumerar
sítios de mesma causa, não pare no que o issue reportou — em #435 o issue citou uma regra e a medição
achou **duas** (`traceid_orphan_req` em `validator_traceid.go:262` e `req_has_roadmap` em
`validator.go:798`, esta sem sequer ter o campo `state` disponível).

Relacionado: [[verificacao-visual-obrigatoria]] (gate verde não prova o que não exercita).

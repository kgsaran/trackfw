---
status: Open
date: 2026-11-24
author: "zeus-tf (issue #704, decisão do KG em 2026-11-24)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-24-teste-de-orcamento-de-alias-afirma-o-trackfw-yaml-deste-repositorio-e-reprova-em-consumidor.md"
---

# REQ: Teste de orçamento de alias afirma o trackfw.yaml deste repositório e reprova em consumidor

> Date: 2026-11-24 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
Issue #704 (consumidor externo). `TestAliasBomb_Budget_RealTrackfwYaml` (`internal/config/config_merge_key_test.go`,
~L500–530, introduzido em `fc2e8979`, publicado na v11.9.0) lê o `trackfw.yaml` real do repositório e afirma os
valores DESTE repositório (`req_dir: docs/req`, `governance_mode: lenient`, `roadmap_namespacing: flat`, ...). Num
consumidor com config própria, o teste reprova, embora a propriedade que dá nome ao teste (o `parse()` de um arquivo
real cabe no orçamento de alias) passe. Medido pelo consumidor nos jobs `go` e `windows-full-suites`; conferido pelo
arquiteto na leitura do teste (L514: "these values are known from the repo's trackfw.yaml"). Censo do consumidor:
sítio único que lê `../../trackfw.yaml`.

## Acceptance Criteria
- [ ] O teste afirma só a propriedade do nome: `parse()` sem erro sobre o `trackfw.yaml` real, e campos obrigatórios
      (`req_dir`, `roadmap_dir`, `adr_dirs`) não vazios. Nenhum valor específico deste repositório.
- [ ] Braço: com um `trackfw.yaml` de consumidor (`req_dir: docs/requisições`, `governance_mode: strict`,
      `roadmap_namespacing: by_agent`) o teste passa (antes reprovava). Contra-braço: um `parse()` que devolve config
      vazia ainda reprova.
- [ ] Censo: nenhum outro teste afirma o conteúdo de arquivo versionado deste repositório como se fosse universal
      (busca por leitura de `trackfw.yaml`, `lefthook.yml` e afins por caminho relativo à raiz).
- [ ] `make quality` rc=0 e CI verde.

## Negative scope
- Não muda o orçamento de alias nem o `parse()`.
- Não muda o `trackfw.yaml` deste repositório.
- A release 11.9.1 é passo de release, fora deste roadmap.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-24-teste-de-orcamento-de-alias-afirma-o-trackfw-yaml-deste-repositorio-e-reprova-em-consumidor.md

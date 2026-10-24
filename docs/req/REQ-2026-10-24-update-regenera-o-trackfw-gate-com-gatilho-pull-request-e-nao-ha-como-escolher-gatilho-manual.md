---
status: Open
date: 2026-10-24
author: "zeus-tf (desenho) / KG (decisão)"
adr: "docs/adr/ADR-2026-08-28-gate-de-ci-gerado-nasce-pinado-na-versao-que-o-gerou-e-o-install-sh-honra-trackfw-version.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-24-update-regenera-o-trackfw-gate-com-gatilho-pull-request-e-nao-ha-como-escolher-gatilho-manual.md"
---

# REQ: o update regenera o trackfw-gate com gatilho pull_request, e não há como escolher gatilho manual

> Date: 2026-10-24 | Status: Open
| Linear Issue:
| Jira Issue:
| GitHub Issue: #607

## Motivation

Issue #607. No claraea/ea-cmdb, os gates rodam localmente antes do PR (`make pr-check`) e os workflows de PR foram trocados
para `workflow_dispatch`, por custo de Actions. A cada `trackfw update`, o `.github/workflows/trackfw-gate.yml` volta a ter
`pull_request`, e a troca manual se perde. Medido no consumidor: o arquivo foi removido, reapareceu depois de um update, e
o `trackfw update --help` o lista como alvo gerenciado.

A mesma causa aparece em três templates de workflow que o trackfw gera: `buildGitHubActionsWorkflowContent` e
`buildGitLabCIWorkflowContent` (`internal/generators/scaffold.go`), e `BuildDiscoverGitHubActionsWorkflowContent`
(`internal/generators/scaffold_doctor.go`). Nos três, o gatilho é fixo. O scaffold doctor compara o arquivo em disco com o
template, e por isso trata qualquer gatilho diferente como divergente.

Decisão: o adendo de 2026-10-24 da ADR-2026-08-28 (chave `ci_trigger`).

## Acceptance Criteria

- [ ] **AC1:** `ci_trigger: manual` no `trackfw.yaml` → o `trackfw update` e o `trackfw init` geram o `trackfw-gate.yml` só
  com `workflow_dispatch` e o `.gitlab-ci-trackfw.yml` só com `when: manual`. O `discover --init` gera o
  `trackfw-validate.yml` só com `workflow_dispatch`. Tudo o mais no workflow continua igual: job id, passos, pin de versão.
- [ ] **AC2:** sem a chave, ou com `ci_trigger: pull_request`, os três templates continuam idênticos byte a byte aos de
  `origin/main`, nas variantes de produtor e de consumidor.
- [ ] **AC3:** o scaffold doctor e o `trackfw validate` não acusam como divergente o workflow manual gerado com
  `ci_trigger: manual`. Acusam, sim, o workflow com `pull_request` quando a config diz `manual`, e o inverso.
- [ ] **AC4:** um valor desconhecido de `ci_trigger` vira violation do `validate` com o valor e os aceitos, e o `update`
  recusa gerar o workflow com esse valor. Nunca há volta silenciosa ao padrão.
- [ ] **AC5:** `docs/cli-parity.md` documenta a chave, e `make quality` e o CI ficam verdes. O PR leva `Closes #607`.

## Negative scope

- **Não** preserva um bloco `on:` editado à mão (alternativa rejeitada na ADR).
- **Não** cria inputs no `workflow_dispatch`: os templates atuais não usam `github.base_ref`, então não há o que receber.
- **Não** muda o gatilho dos workflows deste repositório (`.github/workflows/quality.yml` e similares).
- **Não** desliga a regra que exige o workflow de governança. Manual continua sendo "existe workflow".

## Linked ADR
ADR: `docs/adr/ADR-2026-08-28-gate-de-ci-gerado-nasce-pinado-na-versao-que-o-gerou-e-o-install-sh-honra-trackfw-version.md` (adendo de 2026-10-24)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-24-update-regenera-o-trackfw-gate-com-gatilho-pull-request-e-nao-ha-como-escolher-gatilho-manual.md

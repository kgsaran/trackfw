---
status: Open
date: 2026-10-25
author: "zeus-tf (desenho) / KG (prioridade)"
adr: "docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-25-validate-casa-o-marcador-do-guard-em-qualquer-string-do-settings-e-nao-so-em-comando-de-hook.md"
---

# REQ: validate casa o marcador do guard em qualquer string do settings, e não só em comando de hook

> Date: 2026-10-25 | Status: Open
| Linear Issue:
| Jira Issue:
| GitHub Issue: #618

## Motivation

Issue #618. Com o trackfw 11.0.1, depois de `trackfw update harness`, o `trackfw validate` continua avisando que o
`~/.claude/settings.json` "references trackfw-credential-guard.sh", e manda rodar `update harness`. A única ocorrência da
string no arquivo está em `.autoMode.environment[15]`, um texto em prosa do usuário. A instrução do aviso não resolve o
aviso, que é a mesma forma do #599.

**Causa (lida no código pelo arquiteto, 2026-10-25):** `collectCommandsWithMarker`
(`internal/validator/validator_credential_guard.go:332`) casa o marcador em qualquer string do JSON. 12 chamadores passam
o arquivo inteiro:
- `validator_credential_guard.go`: L512, L583, L682, L888, L942 e L995;
- `validator_git_branch_guard.go`: L259, L313, L424, L482, L549 e L620.

Os chamadores de L758 e L773 passam só o subárvore `hooks` e estão a salvo. A mesma coleta alimenta regras de **violation**
(`*_hook_resolvable` e forma da linha). Uma string fora dos hooks, como uma entrada `permissions.allow` com
`Bash(trackfw guard credential …)`, pode virar violation falsa. Também pode haver uma regra de presença que conte essa
prosa como fiação existente, o que seria fail-open. Isso fica medido na Wave 0.

Decisão: D16 do ADR-2026-10-04.

## Acceptance Criteria

- [x] **AC1 — Só comando de hook.** Em todo CLI e em todo arquivo (projeto e global), o validador só considera um marcador
  de guard quando ele está num campo de comando de hook do esquema daquele CLI. Fica medido, por CLI, qual é o campo.
- [x] **AC2 — Todos os sítios.** Os 12 chamadores e qualquer outro que a Wave 0 encontrar usam a coleta restrita. Um teste
  de concordância falha se um chamador novo passar o arquivo inteiro.
- [x] **AC3 — Falso positivo, os dois braços.** O mesmo marcador fora de `hooks` (prosa, `permissions.allow`, `autoMode`,
  `env`) não gera aviso nem violation, e dentro de `hooks` continua gerando. Isso vale para cada regra afetada.
  Contra-braço: o braço "fora de hooks" reprova em `origin/main`.
- [x] **AC4 — Sem fail-open.** Nenhuma regra de presença passa a contar como fiação uma string fora de comando de hook, e o
  teste prova isso. Se a Wave 0 medir que alguma já contava, essa é a correção prioritária do ML.
- [x] **AC5 — O caso medido.** Um `~/.claude/settings.json` falso com o texto de `.autoMode.environment` e o hook atual
  não gera o aviso de forma legada.
- [ ] **AC6 — Fechamento.** `make quality` rc=0, CI verde e o PR leva `Closes #618`.

## Negative scope

- **Não muda o que o `update` ou o `update harness` escrevem.** É só leitura e validação.
- **Não muda a forma atual da linha de hook** nem as gerações do registro (`internal/guardforms`).
- **Não reescreve arquivos do usuário.** O texto em `autoMode` do KG fica como está.
- **Não cria regra nova.** Só restringe o que as regras existentes consideram.

## Linked ADR
ADR: `docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md` (D16)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-25-validate-casa-o-marcador-do-guard-em-qualquer-string-do-settings-e-nao-so-em-comando-de-hook.md

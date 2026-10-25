---
status: Open
date: 2026-10-25
author: "zeus-tf (desenho) / KG (decisão: opção b)"
adr: "docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-25-validate-avisa-quando-o-arquivo-de-hook-de-projeto-nao-tem-o-credential-guard-nos-clis-sem-regra-de-presenca.md"
---

# REQ: validate avisa quando o arquivo de hook de projeto não tem o credential guard, nos CLIs sem regra de presença

> Date: 2026-10-25 | Status: Open
| Linear Issue:
| Jira Issue:
| GitHub Issue: #619

## Motivation

Issue #619. O `trackfw validate` só exige a presença do credential guard de projeto no Windsurf e no Amazon Q
(`validateCredentialGuardPresenceRequired`, `internal/validator/validator_credential_guard.go:861`). Nos outros 6 CLIs
(Claude, Codex, Gemini, Cursor, Copilot e Kiro), um arquivo de hook sem guard, ou com o guard num evento que não bloqueia,
passa com rc=0. Isso foi medido no red-team da #618: os fixtures dão saída idêntica à de um repositório sem guard.

A `guard_wiring_removed` cobre a remoção e a movimentação de uma fiação que já estava na `main`. O buraco são os
repositórios que nunca tiveram a fiação.

**Decisão do KG (2026-10-25), opção (b):** emitir **aviso**, e não violation, numa minor. A exigência absoluta foi
descartada por enquanto, porque deixaria vermelho todo repositório que hoje roda sem guard. Decisão registrada como D17
no ADR-2026-10-04.

## Acceptance Criteria

- [ ] **AC1 — Aviso por CLI e evento.** Quando um arquivo de hook de projeto de Claude, Codex, Gemini, Cursor, Copilot ou
  Kiro existe e não tem o credential guard no campo de comando (D16) de cada evento bloqueante que o gerador emite para
  aquele CLI, o `validate` emite um aviso com o arquivo, o evento e a instrução `trackfw update`. A lista de eventos
  esperados por CLI vem do gerador.
- [ ] **AC2 — Só aviso.** O rc do `validate` não muda por causa deste aviso, inclusive no modo estrito (fora do lenient).
  Um teste afirma isso.
- [ ] **AC3 — Sem ruído no caso gerado.** O resultado do `trackfw init` e do `trackfw update` de cada CLI não produz o
  aviso, e também não o produz quando existe o guard global (D15).
- [ ] **AC4 — Os casos do red-team.** O guard só em `PostToolUse`/`AfterTool`, e o `command` no nível do grupo, produzem o
  aviso.
- [ ] **AC5 — Ponto único.** O Windsurf e o Amazon Q continuam com violation. A lista de eventos esperados por CLI fica num
  ponto único, compartilhado pela regra de violation e pela de aviso, e um teste de concordância pega um CLI sem entrada.
- [ ] **AC6 — Fechamento.** `make quality` rc=0, CI verde, PR com `Closes #619` e release 11.1.0 (minor).

## Negative scope

- **Não transforma o aviso em violation** em nenhum CLI novo. Isso fica para uma decisão futura (opção a).
- **Não muda o que o `init`/`update` escrevem.**
- **Não muda a `guard_wiring_removed`** nem a regra de violation do Windsurf e do Amazon Q, além de passar a ler a lista
  compartilhada.
- **Não cria chave nova no `trackfw.yaml`.** O opt-in (opção c) foi descartado.

## Linked ADR
ADR: `docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md` (D17)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-25-validate-avisa-quando-o-arquivo-de-hook-de-projeto-nao-tem-o-credential-guard-nos-clis-sem-regra-de-presenca.md

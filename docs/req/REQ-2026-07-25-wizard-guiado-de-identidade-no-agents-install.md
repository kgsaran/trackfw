---
status: Open
date: 2026-07-25
author: "Zeus (Principal Software Architect)"
adr: "ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install.md"
roadmap: "ROADMAP-2026-07-25-wizard-guiado-identidade-agents-install.md"
---

# REQ: Wizard guiado de identidade no agents install

> Date: 2026-07-25 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

A identidade personalizavel de agentes (PR #64) funciona, mas o wizard ficou
so em `trackfw init`. Quem roda `trackfw agents install` — o caminho natural
em projeto ja inicializado — **nunca e perguntado sobre identidade**, e so
descobre a feature lendo o README.

Alem disso, o wizard atual expoe o `id` tecnico do agente (`architect`,
`code-quality`) em vez da especialidade, e presets sao escolhidos as cegas:
o usuario so descobre que o agente de seguranca virou "Boromir" **depois** de
os arquivos terem sido escritos.

Esta REQ e **exclusivamente de UX de CLI**: nao altera o schema de
`identity.json`, o contrato de slug nem os artefatos gerados.

Decisoes, alternativas rejeitadas e a regra de acionamento estao no ADR.

## Acceptance Criteria

- [ ] Existe **um unico** componente de wizard de identidade por CLI,
      consumido por `init` e por `agents install`.
- [ ] `agents install` exibe o passo de identidade **somente** quando:
      `kind == agents` **e** stdin e TTY **e** (`identity.json` ausente **ou**
      flag `--identity` passada).
- [ ] Com `identity.json` existente e sem `--identity`, `agents install`
      **nao pergunta nada** e informa qual identidade esta em uso.
- [ ] `trackfw skills install` **nunca** exibe o wizard de identidade.
- [ ] `agents install` aceita `--identity-preset` com a mesma semantica de
      `init` (10 presets + `neutral` + `none`; invalido -> erro listando).
- [ ] Ramo nao-TTY de `agents install` **nunca bloqueia em prompt** e continua
      exigindo `--targets`.
- [ ] Modo `custom` rotula cada campo com `Item.Name` e `Item.Description` do
      catalogo, **sem** exibir o `id`.
- [ ] Tela de confirmacao lista os 10 pares `especialidade -> nome` mais o
      apelido, antes de qualquer escrita em disco — para preset **e** custom.
- [ ] Recusar a confirmacao **nao grava nada** e retorna a selecao de preset.
- [ ] Ordem do fluxo conforme ADR D6: alvos -> agentes -> superficie ->
      apelido -> preset -> nomes -> confirmacao -> instalacao.
- [ ] Comportamento equivalente nos 3 CLIs (ordem, rotulos, acionamento,
      conteudo da confirmacao).
- [ ] Chaves i18n novas presentes em `pt-BR`, `en-US` e `es-ES` nos 3 CLIs.
- [ ] `make quality` verde, incluindo `check-identity-parity.sh` **sem
      alteracao** — os artefatos gerados nao mudam.
- [ ] Nao-regressao: `init` continua funcionando exatamente como hoje,
      incluindo idempotencia do re-`init` e `--identity-preset`.
- [ ] Documentacao atualizada nos 3 READMEs e em `docs/cli-parity.md`.

## Linked ADR

ADR: docs/adr/ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/wip/ROADMAP-2026-07-25-wizard-guiado-identidade-agents-install.md

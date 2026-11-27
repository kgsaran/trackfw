---
status: Done
date: 2026-11-26
author: "zeus-tf (relato do KG em 2026-11-26, projeto improve-driving)"
adr: "docs/adr/ADR-2026-11-26-bootstrap-de-repositorio-novo-init-roda-git-init-e-a-branch-padrao-e-publicada-uma-vez-pelo-trackfw.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md"
---

# REQ: Repositório novo não tem rota governada para publicar a branch padrão e o init não inicializa o git

> Date: 2026-11-26 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
Relato do KG (2026-11-26, projeto novo `improve-driving`), reproduzido pelo arquiteto com o binário 11.9.1 em sandbox
isolado:

- O `trackfw init` roda numa pasta sem git (rc=0), mas não cria o repositório. O agente tropeça no
  `fatal: not a git repository` do primeiro comando git.
- Num repositório novo, nenhuma rota governada publica a branch padrão:
  - `git commit` e `git push` brutos são bloqueados pelo guard;
  - `trackfw commit`, `trackfw push` e `ship` recusam `main`/`master`.
- Seguindo o protocolo do `CLAUDE.md` gerado, `branch new --from HEAD` renomeia a `main` ainda sem commit para a
  feature, a feature sobe para um remoto sem `main`, e daí em diante a `main` não tem como ser criada.
- Sem `origin/main`, a barreira bloqueia o `ml_scope` e o `validate` avisa "severity anchor unavailable".

O KG só destravou criando a `main` à mão. Varredura em 2026-11-26: nenhuma issue aberta e nenhuma REQ com o mesmo
mecanismo.

## Acceptance Criteria
- [ ] D1: `init` numa pasta sem git cria o repositório com branch `main` (caminho com e sem TTY); dentro de uma work
      tree existente não roda `git init` e imprime a raiz detectada; sem binário `git`, avisa e termina com rc=0.
- [ ] D2: `branch new` com `HEAD` sem commit recusa, inclusive com `--from HEAD`, e orienta o bootstrap. `trackfw
      commit` aceita `main`/`master` só no primeiro commit; com braço (antes rc=1, depois rc=0) e contra-braço (segundo
      commit continua recusado).
- [ ] D3: `trackfw push` publica `main`/`master` só quando todas as URLs de push estão vazias e a branch tem um
      commit, com lease de ausência. Vetores também: várias `pushurl` e `main` com dois commits.
      Vetores medidos no binário: `pushurl` divergente de `url`, branch criada entre a consulta e o push, remoto
      ausente, remoto inacessível e `--force-with-lease` do usuário. Todos recusam.
- [ ] D4: `trackfw push` e `ship` de feature recusam com remoto vazio; com remoto não vazio sem `main`/`master`
      (branch padrão `develop`), seguem como hoje.
- [ ] D6: o guard git-branch nega `gh repo create ... --push`, `switch`/`checkout --orphan`,
      `worktree add --orphan` e `symbolic-ref HEAD`, com braço e contra-braço (`gh repo create` sem `--push` passa).
- [ ] D5: as mensagens de recusa, o `ml_scope` sem `origin/main`, o aviso de severidade e o bloco "Agent Protocol" de
      todos os templates gerados descrevem o bootstrap; `docs/cli-parity.md` atualizado.
- [ ] Replay do fluxo do relato (init → remoto → commit → push → branch new → commit → push → barrier) termina com
      `main` e a feature no remoto e a barreira sem o bloqueio de `origin/main`.
- [ ] `make quality` rc=0 e CI verde.

## Negative scope
- Não cria o repositório no forge. O Wave 0 mediu que `gh repo create ... --push` desvia o guard de push (M-C):
  esse desvio entra nesta REQ (D6), mas criar o repositório continua fora.
- Não muda o bloqueio de `main`/`master` fora do bootstrap: segundo commit, push com a branch já no remoto e `ship`
  na branch padrão continuam recusados.
- Não muda a resolução da branch padrão em outros comandos (`branch prune`, `barrier`), além das mensagens.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-11-26-bootstrap-de-repositorio-novo-init-roda-git-init-e-a-branch-padrao-e-publicada-uma-vez-pelo-trackfw.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md

---
status: Proposed
date: 2026-11-26
author: "zeus-tf (decisão do KG em 2026-11-26)"
---

# ADR: Bootstrap de repositório novo: init roda git init e a branch padrão é publicada uma vez pelo trackfw

> Date: 2026-11-26 | Status: Proposed

## Context

Relato do KG (2026-11-26, projeto `improve-driving`), reproduzido pelo arquiteto com o binário 11.9.1 isolado em pasta
em branco:

- O `trackfw init` roda sem `.git` (rc=0), mas não cria o repositório. O agente descobre a falta pelo
  `fatal: not a git repository` do primeiro comando git.
- Num repositório novo, nenhuma rota governada publica a branch padrão:
  - `git commit` e `git push` são bloqueados pelo guard git-branch (`internal/guard/gitbranch.go`), inclusive
    `git push origin <sha>:refs/heads/main`;
  - `trackfw commit` recusa `main`/`master` sempre (`commitProtectedBranches`, `internal/commands/commit.go:17`);
  - `trackfw push` também (`internal/commands/push.go:143`), e o `ship` igual (ADR-2026-08-22).
- Sem `origin/main`, a barreira bloqueia o `ml_scope` (`internal/commands/barrier.go:1203`) e o `validate` avisa
  "severity anchor unavailable" (`internal/validator/validator_credential_guard_integrity.go:407`).

O usuário só saiu do impasse criando a `main` à mão, fora do agente.

## Decision

Replay medido pelo arquiteto (2026-11-26, binário 11.9.1, sandbox) do protocolo que o `CLAUDE.md` gerado manda
seguir:

1. `git init -b main`;
2. `init`;
3. `req new`;
4. `roadmap move wip`;
5. `branch new feat/x`. Recusa por falta de `origin/main` e sugere `--from HEAD`. Com `--from HEAD`, renomeia a
   `main` ainda sem commit para `feat/x`, e nenhuma `main` local existe mais;
6. `commit`;
7. `push` da feature (rc=0).

O remoto fica só com `feat/x`. A partir daí, nenhuma rota governada cria a `main`. Foi o caminho do relato.

- **D1 — `init` inicializa o git.** Quando o diretório alvo não está dentro de uma work tree
  (`git rev-parse --is-inside-work-tree` falha), o `init` roda `git init` com branch inicial `main`. Num git antigo
  sem `-b`, usa `git init` seguido de `git symbolic-ref HEAD refs/heads/main`. Dentro de uma work tree existente,
  inclusive de um repositório pai, não roda nada e imprime a raiz detectada, para que uma absorção no repositório
  errado fique visível. Sem o binário `git`, avisa e segue (rc=0). Vale para o caminho interativo e para o sem TTY.
- **D2 — o bootstrap começa na branch padrão.**
  - `trackfw branch new` com `HEAD` sem commit recusa, inclusive com `--from HEAD`. A mensagem dá o caminho: commitar
    o resultado do `init` na `main`, configurar o remoto, `trackfw push` e então `branch new`.
  - `trackfw commit` aceita `main`/`master` só quando o `HEAD` ainda não tem commit (`git rev-parse --verify -q HEAD`
    falha). Do segundo commit em diante, a recusa volta.
- **D3 — publicação única da branch padrão.** `trackfw push` aceita `main`/`master` só quando o destino real do push
  não tem essa branch:
  - a consulta usa a URL de push (`git remote get-url --push origin`), não a de leitura, porque
    `remote.origin.pushurl` pode divergir de `remote.origin.url`;
  - o push leva um lease de ausência (`--force-with-lease=refs/heads/<branch>:`), para que o próprio servidor
    recuse se a branch passar a existir entre a consulta e o push. Esse lease é interno e não é a flag
    `--force-with-lease` do usuário, que continua recusada neste modo;
  - remoto ausente, sem resposta, autenticação recusada ou saída não reconhecida recusa, com a mensagem
    "crie o repositório no forge e configure o `origin`".

  Depois do primeiro push, a branch existe no remoto e a recusa volta sozinha. O `ship` continua recusando a branch
  padrão.
- **D4 — feature não sobe antes da branch padrão.** `trackfw push` (e o `ship`) de qualquer outra branch recusa quando
  o destino do push está vazio (`git ls-remote --heads` sobre a URL de push termina com rc=0 e não lista nenhuma
  branch), com a mesma orientação. Isso fecha o caminho medido acima, em que a feature ocupa o remoto primeiro. A
  condição é "remoto vazio", e não "remoto sem `main`/`master`", para não afetar repositórios existentes cuja branch
  padrão tem outro nome (`develop`, `trunk`). Se a consulta falhar, o push da feature segue como hoje: a falha de rede
  já reprova o próprio push, e D4 não vira bloqueio novo para quem não está num repositório novo.
- **D5 — o caminho é dito onde o agente lê.** As mensagens de recusa citadas em D2–D4, o `ml_scope` sem
  `origin/main` (que continua bloqueando, fail-closed), o aviso de severidade sem refs remotas e o bloco
  "Agent Protocol" de todo template gerado (`CLAUDE.md`, `AGENTS.md` e demais alvos) descrevem o bootstrap de
  repositório novo.

## Consequences

- O fluxo de projeto novo passa a ser:
  1. `trackfw init`;
  2. criar o repositório no forge;
  3. `git remote add origin <url>`;
  4. `trackfw commit`;
  5. `trackfw push`;
  6. `trackfw branch new ...`.
- A âncora do ADR-2026-10-10 ("o `origin/main` só muda por merge, que passa por revisão") ganha uma exceção: o
  primeiro conteúdo da branch padrão é publicado sem PR. Isso só vale para um destino que ainda não tem essa branch.
- O ADR-2026-08-22 (bloqueio de `main`/`master` no `push`/`ship`) ganha as exceções D3 e D4.
- Criar o repositório no forge fica fora do trackfw. Se o guard git-branch não intercepta
  `gh repo create --source . --push`, isso já é um desvio do guard de push, e o Wave 0 deve dizer.

## Alternatives Considered

- **Comando próprio (`trackfw repo bootstrap`):** um comando a mais para aprender; o agente tenta `trackfw push`
  primeiro de qualquer jeito.
- **Exigir o remoto totalmente vazio:** o caminho medido (feature publicada antes) fecharia a exceção para sempre.
  Com D4, a feature não sobe antes, e o lease de ausência garante no servidor que a branch padrão não existe.
- **`git init` só com flag (`--git`):** decisão do KG em 2026-11-26: sempre.

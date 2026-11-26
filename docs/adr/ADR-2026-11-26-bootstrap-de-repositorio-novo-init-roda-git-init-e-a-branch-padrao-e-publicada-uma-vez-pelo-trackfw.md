---
status: Accepted
date: 2026-11-26
author: "zeus-tf (decisão do KG em 2026-11-26)"
---

# ADR: Bootstrap de repositório novo: init roda git init e a branch padrão é publicada uma vez pelo trackfw

> Date: 2026-11-26 | Status: Accepted

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

## Ajustes do Wave 0 (2026-11-26, incorporados; prevalecem sobre o texto acima)

Fonte: `docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md` (§6). Decisões do arquiteto sobre as três questões abertas pelo Hades.

- **D1, quatro estados.** `git rev-parse --is-inside-work-tree`, com `LC_ALL=C`:
  - rc=0 e `true`: imprime a raiz detectada, não roda `git init`, escreve o scaffold;
  - rc=0 e `false` (dentro de `.git/` ou de repositório bare): aborta com rc≠0 antes de qualquer escrita, porque
    `git init` ali cria repositório aninhado (M-X9);
  - rc=128 com "not a git repository": roda `git init` com `main`;
  - rc=128 com outro erro (ex.: "dubious ownership", M-D1owner): avisa, não roda `git init`, escreve o scaffold, rc=0.
    Sem binário `git`: o mesmo.
- **D2, predicado.**
  - `commit` usa a conjunção: `git rev-parse --verify -q HEAD` com rc=1 e `git rev-list --all --count` igual a `0`
    (M-X1). Fail-closed.
  - `branch new` usa só a primeira condição, antes do teste de tipo governado, com e sem `--from HEAD`.
- **D3, predicado: remoto vazio** (questão 1).
  - Consulta todas as URLs de push (`git remote get-url --push --all origin`) e roda `git ls-remote --heads` em cada
    uma. Recusa se qualquer uma tiver uma branch ou falhar. Com várias URLs, só a primeira receberia a checagem
    (M-X10).
  - "Remoto sem a branch padrão" deixaria publicar `main` sem revisão num repositório existente cuja branch padrão é
    `develop` (M-E). Com D4, o estado do relato não se forma mais.
  - A exceção substitui o bloqueio de `main`/`master` em `push.go` e recusa a flag `--force-with-lease` do usuário
    antes de qualquer consulta. O lease interno de ausência continua.
- **D3, conteúdo limitado** (questão 3). A branch padrão só é publicada com exatamente um commit
  (`git rev-list --count refs/heads/<branch>` igual a `1`), o que D2 autorizou. Comandos que o guard permite
  (`git fetch . feat/x:main`, `git merge`, `git reset --soft`; M-mover, M-mover-fetch) põem histórico sem revisão na
  `main` antes do primeiro push; o limite mantém a âncora do ADR-2026-10-10.
- **D4 no `ship`.** `runShip` não chama `runPush` (`ship.go:417`). D4 vale nos dois caminhos, por um helper
  compartilhado.
- **D6 (novo): guard de branch** (questão 2). Por decisão do arquiteto, entram nesta REQ todos os achados, e não só
  os que o bootstrap usa (regra "defeito conhecido se corrige agora"). O guard git-branch passa a negar:
  - `gh repo create` com `--push`, em todas as variantes. Hoje rc=0 (M-C) e é um push que não passa pelo guard;
  - `git switch --orphan` e `git checkout --orphan`;
  - `git worktree add --orphan`;
  - `git symbolic-ref HEAD`.

  D2 continua sendo o controle real. O guard é defesa em profundidade.

## Ajustes da Wave 2 (2026-11-26, incorporados)

Fontes:
- `docs/seguranca/2026-11-26-revisao-bootstrap-de-repositorio-novo.md` (Hefesto);
- `docs/seguranca/2026-11-26-red-team-bootstrap-de-repositorio-novo.md` (Hades).

Correções feitas no ML-2C:

- **D3, URL reescrita.** `get-url --push` devolve a URL já reescrita por `insteadOf`/`pushInsteadOf`, e
  `ls-remote <url>` aplica a reescrita de novo. Com duas regras, a consulta via um remoto vazio e o push chegava a
  outro (D3-bypass). Agora, para cada URL de push, roda-se `git ls-remote --get-url <u>`. Se o resultado for
  diferente de `<u>`, a checagem recusa (fail-closed).
- **D1, passos impressos.** A lista de `git add` sai de `git status -z --porcelain`, com `--` e aspas simples POSIX em
  cada caminho (D1-6).
- **D6.** `gh repo create --push=false|0|no|off` é permitido. `gh` dentro de `bash -c`/`sh -c` passa pela mesma regra.

Residuais aceitos:

- **Falso positivo do `insteadOf` em cascata legítima.** Exemplo: `HTTPS→SSH` mais `git@→ssh://`. O push de
  bootstrap recusa, e o usuário remove uma das regras enquanto publica. Só acontece uma vez por repositório.
- **TOCTOU de ref com outro nome.** A consulta e o push não são atômicos. O lease interno protege só
  `refs/heads/<branch>`, então um escritor concorrente pode criar outra branch no remoto nesse intervalo. A `main`
  existente continua protegida pelo lease e pelo receive-pack. O git não tem lease atômico sobre o conjunto de refs.
- **git fora do PreToolUse.** Hooks git (ex.: `pre-push`) e scripts do usuário executam git sem passar pelo guard.
  É a classe de residual já declarada no §4 do Wave 0, e não uma causa nova.
- **Alias.** `git -c alias.x=switch x --orphan` e `git config alias.*` escapam da tokenização do guard. A classe está
  declarada no código como "Residual D5" (`internal/guard/gitbranch.go`). D2 continua sendo o controle real.
- **PowerShell.** As aspas simples POSIX não escapam `'` do jeito do PowerShell (`''`). Um nome de arquivo com `'`
  impresso no Windows precisa ser corrigido à mão. Os demais metacaracteres continuam literais entre aspas simples
  no PowerShell.

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
- Criar o repositório no forge fica fora do trackfw. `gh repo create ... --push` passava pelo guard (M-C) e é negado
  por D6; `gh repo create` sem `--push` continua livre.
- Projetos anteriores a esta REQ cujo remoto já tem só a feature precisam criar a `main` fora do trackfw. Residual.

## Alternatives Considered

- **Comando próprio (`trackfw repo bootstrap`):** um comando a mais para aprender; o agente tenta `trackfw push`
  primeiro de qualquer jeito.
- **Remoto sem a branch padrão (em vez de remoto vazio):** proposta inicial; o Wave 0 mostrou publicação sem revisão
  num repositório existente com `develop` (M-E). Rejeitada nos ajustes do Wave 0.
- **`git init` só com flag (`--git`):** decisão do KG em 2026-11-26: sempre.

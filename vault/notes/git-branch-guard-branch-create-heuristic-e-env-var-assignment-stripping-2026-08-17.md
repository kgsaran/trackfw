---
titulo: "git-branch-guard: heurística de leitura-vs-criação em `git branch`, `worktree add -b` e stripping de `env CHAVE=valor`"
data: 2026-08-17
contexto: "ML-4C, ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md — corretivo da reverificação do hades-tf após ele levantar o bloqueio do ML-4A"
---

## Contexto

O `hades-tf` levantou o bloqueio do ML-4A/ML-4B mas, na reverificação, apontou dois furos na
minha própria tabela AC5 (itens que eu havia classificado como "fora de escopo, mesma classe de
wrappers como `nice`/`sudo`" e que na verdade eram a mesma classe do que o próprio ML-4B já
fechava): `git branch <nome>` (e variantes `-c/-C/-m/-M`), `git worktree add -b`, e
`env CHAVE=valor git ...`.

## Decisão não óbvia 1 — `git branch` não pode ser tratado como "subcomando único = bloqueia"

Ao contrário de `checkout -b`/`switch -c` (onde a flag de criação é inequívoca), `git branch` é
**majoritariamente um comando de leitura** — `git branch` sozinho, `-a`, `-r`, `-l`, `--list`,
`-v`/`-vv`, `--show-current`, `--contains`, `--merged`, `--sort=`, `--format=` são todos leitura, e
até `-d`/`-D` (delete) tem um argumento posicional que **parece** um nome de branch mas não cria
nada.

A heurística implementada (`internal/generators/scaffold.go`, função `match_subcommand`, case
`branch)`) é:

1. Varre todos os tokens após `branch`.
2. `-c|-C|-m|-M|--copy|--move` → marca `branch_action=1` (cria/renomeia, incondicional).
3. `-d|-D|--delete` → marca `has_delete=1`.
4. `--contains|--no-contains|--sort|--format|--points-at|--merged|--no-merged` → consome o
   próximo token como valor (`skip_next=1`), para não ser lido como posicional de criação.
5. Qualquer outro token **sem** `-` na frente → `saw_positional=1`.
6. **Regra final**: bloqueia se (`branch_action` OU `saw_positional`) **E NÃO** `has_delete`.

O ponto crítico do passo 6: sem o `has_delete` como veto, `git branch -d nome` seria bloqueado
(o "nome" é lido como posicional), o que é o **oposto** do que a AC exige — deletar não é criar,
e bloquear leitura/delete é pior que a brecha original (mesmo princípio já usado no ML-1A para
não bloquear prosa).

## Decisão não óbvia 2 — `--merged`/`--contains` sem argumento não quebram nada

`--merged [<commit>]` e `--contains [<commit>]` têm argumento **opcional** no git real. A
implementação trata o `skip_next` de forma tolerante: se não houver mais tokens depois da flag, o
loop simplesmente termina sem consumir nada — não há erro nem falso bloqueio. Isso foi verificado
por execução (`git branch --merged` sozinho → exit 0).

## Decisão não óbvia 3 — `env CHAVE=valor` vs `env -i`/`--flag`

O ML-4B (sessão anterior) só reconhecia `env`/`command` **sem** nenhum argumento antes de `git`
(`env git ...`). O `hades-tf` apontou que `env FOO=bar git push` é a **mesma classe** de evasão
não intencional — um agente que exporta uma env var antes do git, sem intenção de evadir o guard.

A correção distingue **atribuição de variável** (`CHAVE=valor`, sem `-` na frente) de **flag**
(`-i`, `--ignore-environment`, começando com `-`): só a primeira é pulada. Isso é deliberado — uma
flag como `env -i` muda o comportamento do processo git de forma mais “ativa” (não é só uma
declaração de env var incidental), e reconhecer flags de `env`/`command` exigiria entender a
sintaxe própria de cada builtin — mesmo julgamento de custo que motivou o ML-4B a não fechar as
flags. **Ainda declarado como aberto** (ver header do script e tabela AC5 do roadmap): `env -i git
push`, `command -p git push`.

## Onde isso vive

7 cópias sincronizadas (nunca editar uma a uma — tudo sai do gerador Go):
`internal/generators/scaffold.go` (`gitBranchGuardScript`, canônico),
`internal/validator/validator_git_branch_guard_reference.go` (`gitBranchGuardScriptReference`),
`npm/src/generators/hooks.js` (`GIT_BRANCH_GUARD_SCRIPT`),
`npm/src/validator/index.js` (`GIT_BRANCH_GUARD_SCRIPT_REFERENCE`),
`pypi/trackfw/generators/init_gen.py` (`_GIT_BRANCH_GUARD_SH`),
`pypi/trackfw/validator.py` (`_GIT_BRANCH_GUARD_SCRIPT_REFERENCE`),
`scripts/trackfw-git-branch-guard.sh` (referência versionada).

Falsificação (P4): `scripts/check-gates-falsify.sh` Cenário 63 (3 sub-casos: `branch-create`,
`worktree-add-b`, `env-var-assignment`), cada um com baseline + detecção + auto-discriminação
contra o mesmo build corrompido. Ver também [[git-branch-guard-quote-aware-segmentation-2026-08-16]]
para o desenho geral do `match_subcommand`/`quote_aware_split`.

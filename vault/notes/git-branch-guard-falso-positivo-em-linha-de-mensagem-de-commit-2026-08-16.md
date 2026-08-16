# `git-branch-guard` bloqueia por prosa: linha de mensagem de commit que **começa** com `git ...`

> 2026-08-16 · descoberto ao preparar a release 7.0.0
> `scripts/trackfw-git-branch-guard.sh` (`match_subcommand`, ~linha 51)

## Sintoma

Um `./bin/trackfw commit -m "<mensagem>"` foi bloqueado com:

```
trackfw: git checkout -b bruto bloqueado. Use `trackfw branch new <type>/<slug>`.
```

**sem que houvesse nenhum `git checkout -b` no comando.** O mesmo comando, com a mensagem de commit
reescrita, passou sem erro.

## Causa raiz

A mensagem de commit continha uma tabela em texto:

```
  git checkout -b            -> bloqueado pelo guard
  trackfw branch new chore/  -> recusado
```

O `match_subcommand` divide a string do comando por `;`, `&&`, `||`, `|` **e quebra de linha**, e
para cada segmento faz:

```sh
seg_trimmed=$(printf '%s' "$seg" | sed -e 's/^[[:space:]]*//')
set -- $seg_trimmed
first="$1"; base="${first##*/}"
[ "$base" = "git" ] || continue
```

Ou seja: **remove o espaço à esquerda e então testa se o primeiro token é `git`**. Uma linha de
prosa indentada que começa com `git checkout -b` satisfaz exatamente essa condição.

## Por que passou despercebido

O comentário do próprio script (linhas 43-49) afirma tratar o caso:

> "(c) texto de prosa (ex.: mensagem de commit mencionando `git commit` no meio de uma frase) ser
> tratado como comando"

E trata — para menção **no meio da frase**, onde `git` não é o primeiro token. O que não é tratado é
a linha que **começa** com o comando, que é a forma natural de documentar comandos numa mensagem de
commit. Neste projeto isso é frequente: as mensagens descrevem o que foi bloqueado/permitido.

## Impacto

Não é falha de segurança (erra para o lado restritivo), mas é **falso-positivo em caminho quente**:
quem escreve mensagem de commit citando comandos git é bloqueado sem entender por quê — a mensagem
de erro fala de um comando que a pessoa não executou. Custa tempo e leva a contornos ruins (reescrever
a mensagem "até passar", ou pior, procurar a brecha do guard).

## Contorno imediato

Não iniciar linha de mensagem de commit com `git <subcomando>`. Prefixar com um traço ou aspas
(`- git checkout -b ...`) já evita, porque o primeiro token deixa de ser `git`.

## Correção sugerida (vira REQ)

O guard inspeciona a **string do comando inteiro**, incluindo o corpo de `-m`/heredoc. A correção
correta é **não considerar como comando o conteúdo de argumentos de mensagem** — por exemplo,
descartar o que vier depois de `-m`/`--message` (e de heredocs) antes de segmentar.

Débito irmão, encontrado no mesmo dia: **o guard cobre `git checkout -b` mas não cobre
`git switch -c`** — é brecha real de contorno. Os dois cabem na mesma REQ do guard.

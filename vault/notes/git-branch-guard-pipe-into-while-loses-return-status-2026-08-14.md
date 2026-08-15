# bash: `cmd | while read; do ...; return N; done` não retorna N da função chamadora

> 2026-08-14 — Apolo (fix pós-ML-4A, port Python de `match_subcommand()` em
> `_GIT_BRANCH_GUARD_SH`)

## Contexto

Corrigindo os 3 bugs reais do ML-4A (comando encadeado escapando via `;`, path absoluto
escapando via `/usr/bin/git`, prosa citando "git commit" sendo lida como comando — ver
[[git-branch-guard-self-blocking-quote-unaware-splitter-2026-08-14]]), a primeira versão do
fix em `match_subcommand()` (Python) segmentava o comando e iterava com:

```bash
match_subcommand() {
  printf '%s\n' "$1" | sed -E '...' | while IFS= read -r segment; do
    ...
    case "$sub" in
      commit) echo "commit"; return 0 ;;
    esac
  done
  return 1
}
```

Testado isoladamente com `bash -x`, o `stdout` capturado por
`SUBCOMMAND=$(match_subcommand "$CMD_RAW")` mostrava o valor certo (`commit`), mas o script
sempre seguia pelo ramo `|| exit 0` (allow), como se `match_subcommand` tivesse retornado
falha mesmo tendo "casado".

## Causa raiz não óbvia

Em um pipeline `A | B | while read; do ...; done`, cada estágio — incluindo o `while` final —
roda em um **subshell bifurcado** (a menos que `shopt -s lastpipe` esteja ativo e o shell não
tenha controle de job, o que não é o caso do script bash executado via `bash script.sh`).

`return 0` dentro do corpo do `while` só encerra **esse subshell bifurcado** — não a função
`match_subcommand` em si. Depois que o subshell do `while` termina (com o `echo`/status
corretos), a execução da FUNÇÃO continua na próxima instrução **depois do pipeline**, ou seja
`return 1`. O status final da função (o que `$(...)` usa como exit code) é o de `return 1`,
mesmo que o `stdout` (capturado via `$(...)` também, mas por um canal diferente — o pipe em si)
já tivesse o valor certo escrito.

Resultado: `SUBCOMMAND` recebe o texto certo, mas o exit code da função vem da instrução ERRADA
(`return 1` depois do `done`), e `SUBCOMMAND=$(match_subcommand ...) || exit 0` dispara o
`exit 0` mesmo quando deveria bloquear.

## Como diagnosticar de novo

Sintoma: uma função com `cmd | while read; do ... ; return N; done` onde o valor de retorno
(status de saída, não o stdout) parece "não propagar" — `$?`/`|| fallback` sempre cai no branch
de falha mesmo com stdout correto. Verificar se o loop está do lado direito de um `|` — se
estiver, o `return`/`exit` dentro dele só afeta o subshell do pipe.

## Solução usada (mesma técnica que o Go/Node já usavam)

Capturar a segmentação numa variável ANTES do loop, e alimentar o `while` via heredoc/herestring
(`done <<EOF\n$var\nEOF` ou `done <<<"$var"`) em vez de pipe. Redirecionamento de entrada não
bifurca — o `while` roda no mesmo shell da função, e `return` funciona normalmente.

```bash
match_subcommand() {
  normalized=$(printf '%s' "$1" | sed -e 's/&&/\n/g' -e 's/||/\n/g' -e 's/[;|]/\n/g')
  while IFS= read -r seg; do
    ...
    case "$sub" in
      commit) echo "commit"; return 0 ;;
    esac
  done <<EOF
$normalized
EOF
  return 1
}
```

Efeito colateral também corrigido pela mesma mudança: como `normalized=$(...)` (command
substitution) sempre estriparia uma quebra de linha final, um comando de segmento único (sem
`;`/`&&`/`||`/`|`) ficaria sem newline terminal — e `while read` do bash **descarta a última
linha sem newline terminal** (`read` retorna status 1 no EOF mesmo tendo preenchido a
variável, então o corpo do loop nunca roda para essa última linha). O heredoc resolve isso de
graça: `<<EOF\n$var\nEOF` sempre insere uma quebra de linha real antes do delimitador `EOF`,
então a última linha do conteúdo sempre chega ao `read` terminada.

## Onde essa técnica já existia e por que a versão Python original não a usou

`internal/generators/scaffold.go:gitBranchGuardScript` e
`npm/src/generators/hooks.js:GIT_BRANCH_GUARD_SCRIPT` (implementados por outro agente/sessão em
paralelo) já usavam exatamente essa técnica (heredoc, `normalized=$(...)` antes do loop). A
versão Python inicial foi escrita a partir da especificação em prosa (não lendo o Go real
primeiro), reintroduziu independentemente o padrão pipe-into-while, e só convergiu para o texto
byte-idêntico do Go depois de: (1) descobrir o bug empiricamente rodando o script gerado, e (2)
ler `internal/generators/scaffold.go` diretamente e copiar a técnica + nomes de variável.
**Lição para próximas portas**: quando a tarefa diz "outro stack já tem a correção, prefira ler
o código real em vez de re-derivar da spec", isso vale tanto para o *comportamento* quanto para
a *técnica de shell* usada — pipe-into-while vs. heredoc-into-while são comportamentalmente
diferentes para `return`/`exit`, mesmo que pareçam equivalentes na leitura superficial.

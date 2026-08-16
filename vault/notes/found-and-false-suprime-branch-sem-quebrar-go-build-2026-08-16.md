# `found && false` — suprimir um branch condicional em Go sem violar "declared and not used"

**Data:** 2026-08-16
**Contexto:** ML-2B, `ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md`, ao
escrever o Cenário 57 de `scripts/check-gates-falsify.sh` (falsificação de
`scripts/check-unknown-command-parity.sh`).

## Problema

Para provar que o gate detecta a ausência da linha `Did you mean "..."?`, era preciso sabotar
`internal/commands/root.go`'s `formatUnknownCommandError` de forma que ela **nunca** emita a
sugestão, mesmo quando uma existe — mas o resultado ainda precisa **compilar** (`go build`), senão
a "prova" vira um erro de build, não uma regressão de comportamento.

O código original:

```go
if suggestion, found := suggestCommand(typed, unknownCommandCandidates(root)); found {
    fmt.Fprintf(&sb, "Did you mean %q?\n", suggestion)
}
```

A sabotagem ingênua — trocar `found` por `false` na condição —

```go
if suggestion, found := suggestCommand(typed, unknownCommandCandidates(root)); false {
```

**quebra o build**: `found` é declarado por `:=` e nunca referenciado em nenhum outro lugar
(`suggestion` continua usado dentro do corpo do `if`, mas `found` não), e o compilador Go trata
"declared and not used" como erro fatal — não um warning. O build falha antes mesmo de rodar o
binário sabotado, e o cenário de falsificação não prova nada sobre o comportamento em runtime.

## Solução

Manter `found` referenciado na própria condição, mas fixar o resultado combinado em `false` via
curto-circuito:

```go
if suggestion, found := suggestCommand(typed, unknownCommandCandidates(root)); found && false {
```

`found && false` sempre avalia para `false` em runtime (suprimindo o branch — o efeito desejado),
mas `found` continua sendo uma expressão "usada" do ponto de vista estático do compilador, então
`go build` passa normalmente. O binário resultante nunca emite a linha de sugestão, exatamente
como pretendido, mas continua sendo uma "regressão de comportamento", não um binário inexistente.

## Por que isso importa para outros agentes

Qualquer cenário de falsificação (`check-gates-falsify.sh`) que precise neutralizar um branch
condicional em Go cujo resultado booleano vem de uma variável declarada por `:=` cai na mesma
armadilha se tentar substituir a condição inteira por um literal. O padrão `<var> && false` (ou
`<var> || true` para o caso inverso — forçar sempre-verdadeiro mantendo a variável referenciada) é
o jeito idiomático de neutralizar comportamento em uma cópia sabotada sem que o próprio `go build`
vire o motivo da falha (o que mascararia a causa real que o cenário deveria provar).

## Referências

- `scripts/check-gates-falsify.sh` — Cenário 57 (`unknown-command-parity/missing-suggestion/go-*`).
- `internal/commands/root.go` — `formatUnknownCommandError`.
- Padrão de rebuild de binário Go isolado para provar regressão: Cenários 25/26 do mesmo arquivo
  (`build_go_or_fail`), reaproveitado aqui.

# cobra: comando pai sem `RunE` sai 0 com subcomando inválido — e a D7 parecia coberta

**Data:** 2026-10-06 · **Contexto:** REQ-2026-09-05, ML-4A (red team) → ML-4C

## Sintoma

`trackfw guard nao-existe` e `trackfw guard 'git-branch;'` saíam **0**. É o caso que a D7 da
ADR-2026-10-04 existe para cobrir: o `cmd.exe` não separa por `;`, então a linha da família
PS/POSIX (`trackfw guard git-branch; exit $LASTEXITCODE`) chega como argv `guard`, `git-branch;`,
`exit`, `$LASTEXITCODE`. Com exit 0, o hook libera tudo.

## Causa

Comando cobra **sem `Run`/`RunE` não é "runnable"**: `!c.Runnable()` devolve `flag.ErrHelp`, e o
`ExecuteC` engole esse erro e retorna nil (cobra v1.10.2, `command.go` ~955 e ~1152). O `Args`
nunca é avaliado. Nenhum tratamento de erro no `root.go` é alcançado.

## Por que passou

O teste da D7 cobria argumento posicional **do subcomando** (`guard git-branch extra`), não
subcomando inválido **do pai**. O AC foi marcado com um teste que afirmava outra coisa, e nenhum
teste de subprocesso olhava o exit code do pai.

## Correção

`newGuardCmd()` com `Args` (recusa posicional) e `RunE` (recusa ausência de subcomando), ambos
com o erro de guard que sai 2. `trackfw guard --help` continua 0. Testes de subprocesso em
`internal/commands/guard_subprocess_test.go`.

## Regra

Em qualquer comando pai cobra cujo erro precise de exit code específico, **dê `RunE` ao pai**. E
teste o exit code por subprocesso: o cobra decide antes do seu código rodar.

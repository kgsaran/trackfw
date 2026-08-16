# cobra: root.SilenceErrors não silencia o bloco de usage, e ignora SilenceErrors/SilenceUsage por-comando

**Data:** 2026-08-16
**Contexto:** ML-2A de `ROADMAP-2026-08-15-remocao-do-subsistema-de-plugins-do-trackfw.md` —
implementação da mensagem canônica de "unknown command" (`internal/commands/root.go`).

## Sintoma

`internal/commands/root.go`'s `Execute()` sempre teve um bug pré-existente e não relacionado a
plugins: TODO erro de comando saía **duas vezes** em stderr — uma vez impresso pelo próprio cobra
(`Command.ExecuteC`, com prefixo `Error:` + bloco de usage) e uma vez pelo `fmt.Fprintln(os.Stderr,
err)` cru no fim de `Execute()`. Só ficou visível ao tentar produzir a mensagem canônica de
comando desconhecido, que exige suprimir a impressão automática do cobra para reformatá-la.

## Causa raiz — dois flags independentes, dois donos diferentes

`Command.ExecuteC` (cobra v1.10.2, `command.go:1148-1168`) imprime o erro de um comando em DOIS
blocos **independentes**, cada um gated por uma condição própria:

```go
if !cmd.SilenceErrors && !c.SilenceErrors {   // c = root
    c.PrintErrln(cmd.ErrPrefix(), err.Error())
}
if !cmd.SilenceUsage && !c.SilenceUsage {     // c = root
    c.Println(cmd.UsageString())
}
```

Duas armadilhas ao tentar suprimir isso de fora (via `root.SilenceErrors = true`):

1. **`root.SilenceErrors = true` sozinho NÃO basta.** O bloco de usage é gated por
   `SilenceUsage`, um flag **diferente**, checado independentemente do primeiro. Setar só
   `SilenceErrors` elimina a linha `Error: ...` mas o bloco de usage completo continua saindo — e
   se o wrapper (`Execute()`) também reimprime sua própria versão, o resultado é uma
   **impressão tripla** (usage do cobra, depois "Error: ..." + usage do wrapper).

2. **Comandos que já setavam `cmd.SilenceErrors`/`cmd.SilenceUsage` no PRÓPRIO comando** (ex.:
   `internal/commands/branch.go`'s `"new"`, que quer o erro nu — `blocked: ...` sem prefixo nem
   usage, para casar com o Node/Python) **param de funcionar** se o wrapper externo reimplementa a
   impressão ignorando esses flags por-comando. `root.SilenceErrors = true` desliga a impressão do
   cobra GLOBALMENTE (a condição `!c.SilenceErrors` do root já é suficiente para pular o bloco,
   independente do `cmd.SilenceErrors` individual) — então, se o wrapper reimprime
   incondicionalmente `"Error: " + err`, comandos como `branch new` GANHAM um prefixo/usage que
   nunca tiveram, quebrando `scripts/check-branch-new-parity.sh` (Go diverge de Node/Python que
   nunca tiveram esse prefixo).

## Fix

Silenciar os dois flags no root (`root.SilenceErrors = true; root.SilenceUsage = true`) e
reimplementar a impressão no wrapper **replicando os dois flags por-comando do cobra**, não
ignorando-os:

```go
cmd, err := root.ExecuteC()
if err != nil {
    if cmd.SilenceErrors {
        fmt.Fprintln(os.Stderr, err.Error())          // sem prefixo — comando pediu isso
    } else {
        fmt.Fprintln(os.Stderr, cmd.ErrPrefix(), err.Error())
    }
    if !cmd.SilenceUsage {
        fmt.Fprintln(os.Stderr, cmd.UsageString())
    }
    os.Exit(1)
}
```

## Por que importa para quem mexer em `Execute()`/`root.go` de novo

Qualquer refatoração futura da impressão de erro no root **precisa** grep por
`cmd.SilenceErrors\|cmd.SilenceUsage` em `internal/commands/*.go` antes de mudar a lógica de
impressão — hoje só `branch.go` usa esse padrão, mas é silencioso (nenhum teste unitário do pacote
`commands` pega a regressão; só o gate de paridade shell `check-branch-new-parity.sh` comparando
Go vs Node/Python detecta, porque Node/Python nunca reproduziram esse prefixo/usage extra).

## Ver também

- `internal/commands/root.go` (`Execute`, `formatUnknownCommandError`)
- `docs/cli-parity.md` § "Unknown top-level command — canonical message"
- `scripts/check-branch-new-parity.sh` (gate que pegou a regressão)

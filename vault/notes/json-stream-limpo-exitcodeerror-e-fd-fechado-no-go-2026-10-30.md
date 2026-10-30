# `--json` limpo depois do documento: `ExitCodeError`, `decideExit` e o fd fechado que o Go reabre

> Data: 2026-10-30 · Domínio: CLI, cobra, contrato `--json` · Origem: REQ-2026-09-07 (#290)

## Sintoma
`validate --json` com violação, `update --json` e `update harness --json` com alvo falho escreviam a
linha de resumo (`N violation(s) found`, `... target(s) failed`) no stderr **depois** do documento.

## Causa
O comando devolvia `error` para sinalizar rc≠0. No `root.go`, `cmd.SilenceErrors` **não silencia**:
só troca `Error: msg` pela mensagem sem prefixo (comportamento do `branch new`). O `barrier` nunca
teve o defeito porque sai com `os.Exit(1)` direto.

## Não óbvio
1. **`os.Exit` no `RunE` derruba os testes em processo** (`cmd.Execute()`). Por isso o padrão é o
   erro tipado `ExitCodeError`, reconhecido por `decideExit` (`internal/commands/root.go`).
2. **Type assertion direta, não `errors.As`.** Um `ExitCodeError` embrulhado com `%w` ou dentro de
   `errors.Join` não é silenciado; uma falha real não some por engano.
3. **`os.Exit(256)` sai rc=0 no Unix** (só o byte baixo). Código fora de 1..255 não é silenciado e o
   construtor entra em pânico.
4. **Teste que prova o `root.go` tem de chamar o `root.go`.** O primeiro teste de "embrulhado não
   silencia" só afirmava a semântica do Go e passava com `errors.As` no lugar; o red team pegou.
   A decisão foi extraída para `decideExit` para ser testável em tabela.
5. **`1>&-` não provoca falha de escrita no Go do macOS:** o runtime reabre fds 0–2 fechados como
   `/dev/null`. Para EBADF real em teste, use `1</dev/null`.
6. Diagnósticos emitidos **durante** o cálculo (recusa do pathguard, aviso de `agent_models` do
   `update harness`) continuam no stderr por decisão (D-1); o contrato está em `docs/cli-parity.md`,
   seção `--json` stream contract (post-document).

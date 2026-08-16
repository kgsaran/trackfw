# commander trata "sem subcomando" como erro (help em stderr, exit 1) — e uma `.action()` no root quebra o dispatch de comando desconhecido

**Contexto:** ML-1C do roadmap `ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md` — unificar `trackfw` sem argumento para `exit 0` / help em stdout nos 3 CLIs (Go e Python já eram assim; só Node.js divergia).

## O que acontece

commander 12.x (`node_modules/commander/lib/command.js`, `Command._parseCommand`), quando o
comando raiz tem subcomandos registrados mas **nenhuma `.action()` própria** e é invocado sem
operandos:

```js
if (
  this.commands.length &&
  this.args.length === 0 &&
  !this._actionHandler &&
  !this._defaultCommandName
) {
  // probably missing subcommand and no handler, user needs help (and exit)
  this.help({ error: true });
}
```

`this.help({error: true})` chama `outputHelp({error:true})` (escreve em **stderr**) e sai com
`exitCode = 1` (`Command.help()`, linhas ~2392-2405). Isso é **por design** do commander — ele
assume que "comando raiz sem argumento" é "faltou dizer qual subcomando", não "pedido de ajuda
legítimo". Go (cobra) e Python (argparse, aqui via `args.command is None: parser.print_help();
sys.exit(0)`) não tratam esse caso como erro.

## A armadilha ao corrigir: NÃO registre `.action()` no root

A correção óbvia parece ser `program.action(() => program.help())` no root — isso muda
`!this._actionHandler` para `false` e desvia do branch acima, e `program.help()` (sem
`{error:true}`) escreve em stdout com exit 0. **Funciona para o caso vazio, mas quebra o
tratamento de comando desconhecido**: com uma `.action()` própria no root, um operando não
reconhecido (ex.: `trackfw naoexiste`) passa a ser tratado como **argumento posicional da action
do root**, silenciosamente engolido — o listener `program.on('command:*', ...)` (que emite a
mensagem canônica de comando desconhecido e faz `process.exit(1)`) **não dispara mais**, porque
commander só emite `command:*` quando não há action própria absorvendo o operando não
reconhecido. Resultado medido: `trackfw naoexiste` passou a sair `exit 0` imprimindo o help,
silenciando exatamente o erro que `ADR-2026-08-15-remocao-do-subsistema-de-plugins...` (D3) exige
sinalizar.

## A correção que funciona

Interceptar **apenas o caso de zero argumentos** ANTES de entrar no parser do commander, no
próprio entrypoint (`npm/bin/trackfw`):

```js
if (process.argv.length <= 2) {
  program.outputHelp()
  process.exit(0)
}
program.parseAsync(process.argv)
```

Isso nunca toca o parsing normal — `trackfw naoexiste` (`process.argv.length === 3`) continua
indo para `parseAsync`, que dispara `command:*` normalmente.

## Como verificar rápido

```bash
node npm/bin/trackfw            # deve ser exit 0, help em stdout, stderr vazio
node npm/bin/trackfw naoexiste  # deve ser exit 1, mensagem canônica em stderr — NUNCA exit 0
```

## Ver também

- [[cobra-silenceerrors-suppresses-usage-independently-of-per-command-flags-2026-08-16]] — mesma
  família de problema no lado Go (root.go): comportamento de erro/help do framework nem sempre é
  o que se espera, e a correção pontual pode ter efeito colateral em outro caminho de erro
  adjacente.
- `scripts/check-unknown-command-parity.sh` — cenário `bare-invocation-is-not-an-error` cobre a
  contraparte: garante que "sem argumento" (não é erro) nunca regride, e que "comando desconhecido"
  (é erro) continua distinto.

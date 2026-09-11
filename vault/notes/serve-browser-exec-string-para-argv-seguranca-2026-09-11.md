# `serve` browser-open: `exec(string)` → argv; residual Windows `cmd.exe`

> Data: 2026-09-11 | REQ: REQ-2026-09-01-serve-interpola-host-em-string-de-shell-e-permite-injecao-de-comando-ao-abrir-o-browser.md

## Causa raiz

`npm/src/commands/serve.js` montava `openCmd = \`open "${url}"\`` e passava para `exec()` — string de shell com interpolação. Uma URL com metacaracteres de shell (ex: `http://x" ; id > /tmp/INJETADO ; echo ":4080`) executava o comando extra **silenciosamente**: o browser abria, o serve subia, e o comando extra executava sem nenhum sinal.

`pypi/trackfw/commands/serve.py` branch Windows: `Popen(["start", url], shell=True)` — com `shell=True` o Python reúne a lista em string de shell antes de passar ao `CreateProcess`, o mesmo resultado.

## Modo de falha silencioso (crítico)

O modo de falha é **silencioso**: o browser abre, o `serve` sobe, e o comando extra executa sem nenhum sinal de erro. Diferente de um crash, **o sucesso aparente é parte do defeito**. Isso impossibilita detecção por monitoramento de exit code.

## Correção

- **Node.js:** `exec(openCmd)` → `spawn(cmd, [url])` via `browserArgv(platform, url)` (retorna `[cmd, args]`).
- **Python Windows:** `Popen(["start", url], shell=True)` → `Popen(["cmd", "/c", "start", "", url])` sem `shell=True`.
- **Python Darwin/Linux:** já estavam corretos (`Popen(["open", url])`), servem de precedente interno.
- **Go:** sem browser-open path — divergência intencional documentada.

## Residual declarado: Windows `cmd.exe`

`spawn('cmd', ['/c', 'start', '', url])` (Node) e `Popen(["cmd", "/c", "start", "", url])` (Python) passam a URL para `cmd.exe`, que re-parseia seus próprios metacaracteres (`&`, `|`, `^`, `>`). Argv não é suficiente aqui porque `cmd.exe` re-parseia a linha de comando resultante.

**Contenção:** `isValidHost`/`_is_valid_host` (AC4) rejeita qualquer `--host` que não seja hostname RFC-1123, IPv4 ou IPv6 válido. Um host legítimo (`my-host.example.com`, `127.0.0.1`, `::1`) não contém metacaracteres de `cmd.exe`.

## Discriminação (lição de gate)

A forma usual de medir ("exit code 0 → sem injeção") falha aqui porque o comando injetado roda **em paralelo** ao `serve` e o exit code do `serve` não reflete o filho. O gate usa um **arquivo sentinela** (`touch /tmp/INJETADO`) e verifica a ausência dele após a execução. Além disso, inclui um **reprodutor vulnerável** (a forma antiga `exec(string)`) que **cria** o sentinela, provando que o gate discrimina antes e depois da correção.

**Ambos os braços do reprodutor usam o PATH shim** (não o `open` real) para:
1. Evitar abas de browser durante o gate
2. Tornar os dois braços comparáveis (ambos passam pelo mesmo stub)

**O contra-braço correto** compara o log do shim verbatim contra `$LEGIT_URL`. A verificação `$# -gt 1` que parecia checar splitting é morta: se injeção ocorreu, o shell roda `touch` como comando separado e chama `open` com 1 argumento; se não ocorreu, também 1 argumento. Comparison verbatim do log é a assertiva real.

## UX — aviso de falha preservado no Node.js

O `exec(openCmd)` original avisava (`console.warn`) quando o opener existia mas saía com código não-zero. O `spawn` com `stdio: 'ignore'` só ativava o handler de `error` (ENOENT), perdendo silenciosamente falhas não-ENOENT. A versão corrigida usa handlers `error` + `close` explícitos para preservar os dois casos.

## Files modificados

- `npm/src/commands/serve.js` — `browserArgv`, `openBrowser`, `isValidHost`
- `pypi/trackfw/commands/serve.py` — `_browser_argv`, `_is_valid_host`, `_open_browser`
- `internal/serve/serve.go` — `IsValidHost`
- `internal/commands/serve.go` — validação antes de `serve.Start()`
- `scripts/check-serve-browser-security.sh` — gate com 13 cenários
- `npm/tests/serve_browser_security.test.js` — 10 testes
- `pypi/tests/test_serve_browser_security.py` — 13 testes

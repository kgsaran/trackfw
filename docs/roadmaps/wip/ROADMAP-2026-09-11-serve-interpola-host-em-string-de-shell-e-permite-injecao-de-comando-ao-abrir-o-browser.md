---
status: wip
date: 2026-09-11
req: "docs/req/REQ-2026-09-01-serve-interpola-host-em-string-de-shell-e-permite-injecao-de-comando-ao-abrir-o-browser.md"
squad: ""
---

# Roadmap: `serve` interpola `--host` em string de shell e permite injeção de comando ao abrir o browser

> Created: 2026-09-11 | Status: 🔄 WIP

## Context

REQ: docs/req/REQ-2026-09-01-serve-interpola-host-em-string-de-shell-e-permite-injecao-de-comando-ao-abrir-o-browser.md

`npm/src/commands/serve.js` usava `exec(\`open "${url}"\`)` — URL interpolada em string de shell.
`pypi/trackfw/commands/serve.py` branch Windows: `Popen(["start", url], shell=True)` — mesmo defeito.
Go não abre browser; ausência de superfície é divergência intencional documentada.

## Acceptance Criteria

- [x] AC1 — nenhum caminho interpola valor controlável em string de shell (argv, não comando montado)
- [x] AC2 — ramo Windows do Python não usa shell=True
- [x] AC3 — falsificação nas duas direções: sentinela ausente + contra-braço com PATH shim
- [x] AC4 — validação de `--host` na entrada em todos os 3 CLIs
- [x] AC5 — gate falsificável nos 3 CLIs, ligado ao Makefile
- [x] AC6 — paridade documentada em `docs/cli-parity.md` com divergência Go declarada
- [ ] AC7 — `make quality` e CI verdes (pendente: make quality inteiro, aguarda auditoria)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model
**Status:** ✅ Concluído (incorporado no handoff do arquiteto — enumeração fechada na Wave 1)

**Superfícies enumeradas:**
1. `npm/src/commands/serve.js:208-211` — `exec(openCmd)` com string interpolada (PRINCIPAL)
2. `pypi/trackfw/commands/serve.py:196` — `Popen(["start", url], shell=True)` Windows (PRINCIPAL)
3. Go — sem browser-open path (AUSÊNCIA DE SUPERFÍCIE — divergência intencional)

**Residual declarado:**
- Windows `cmd /c start "" url` re-parseia metacaracteres de `cmd.exe` mesmo via argv. Contenção: AC4 (isValidHost rejeita hosts com metacaracteres).

## Wave 1 — Implementation
> Dependencies: Wave 0 completa

### ML-1A — AC1+AC2: argv, não string de shell (Node.js e Python)
**Status:** ✅ Concluído

**Arquivos afetados:**
- `npm/src/commands/serve.js`
- `pypi/trackfw/commands/serve.py`

**Mudanças:**

Node.js:
- Adicionado `browserArgv(platform, url)` → `[cmd, args]` sem shell
- Adicionado `openBrowser(platform, url)` usando `spawn` em vez de `exec`
- Exportados `browserArgv`, `openBrowser`, `isValidHost` para teste
- `.action()` substituído: `exec(openCmd)` → `openBrowser(process.platform, url)`

Python:
- Adicionado `_browser_argv(system, url)` → lista argv sem shell
- `_open_browser` refatorado para usar `_browser_argv` + `Popen(argv)` sem `shell=True`
- Branch Windows: `Popen(["start", url], shell=True)` → `Popen(["cmd", "/c", "start", "", url])`
- Adicionado `_is_valid_host(host)` com validação RFC-1123/IPv4/IPv6

**Reconciliação dos testes:**
- `test_windows_popen_no_shell_true`: afirma que `Popen` não recebe `shell=True` no branch Windows — confirma AC2 (o defeito corrigido era exatamente `shell=True`)
- `browserArgv win32`: afirma que URL chega como elemento de argv distinto, não em string montada — confirma AC1

### ML-1B — AC4: validação de `--host` na entrada
**Status:** ✅ Concluído

**Arquivos afetados:**
- `internal/serve/serve.go` — `IsValidHost()` + `rfc1123Label` regex
- `internal/commands/serve.go` — valida antes de `serve.Start()`
- `npm/src/commands/serve.js` — `isValidHost()` + erro antes de `server.listen`
- `pypi/trackfw/commands/serve.py` — `_is_valid_host()` + `sys.exit(1)` antes do bind

**Reconciliação dos testes:**
- `TestIsValidHost/rejeita_payload_de_injeção_(metacaracteres_de_shell)`: afirma que o payload da REQ (`x" ; id > /tmp/INJETADO ; echo "`) é rejeitado antes do browser-open path — confirma que AC4 bloqueia a superfície mesmo se AC1 regredir

### ML-1C — AC3+AC5: gate falsificável + Makefile
**Status:** ✅ Concluído

**Arquivos afetados:**
- `scripts/check-serve-browser-security.sh` (novo)
- `Makefile` — gate adicionado a `parity-rest`

**Estrutura do gate (16 cenários, todos OK após revisão da barreira independente):**
1. Reprodutor vulnerável (com PATH shim): forma antiga `exec(string)` CRIA sentinela → gate discrimina
2. Injeção/Node: código corrigido NÃO cria sentinela
3. Contra-braço/Node: shim recebe URL verbatim como argv único (não split)
4. Injeção/Python Darwin: código corrigido NÃO cria sentinela
5. Contra-braço/Python: shim recebe URL verbatim como argv único
6. AC2 estático Python (AST): nenhum `Popen(shell=True)` em código live
7. AC2 estático Python-Windows: `_browser_argv("Windows")` retorna `["cmd", ...]` não `["start", ...]`
8. AC1 estático Node: `exec(\`open` ausente de código live (excluindo comentários)
9. AC1 estático Node-spawn: `spawn` presente no código live
10. AC4/Node função: `isValidHost` rejeita payload
11. AC4/Node-legítimo função: `isValidHost` aceita todos os formatos legítimos
12. AC4/Python função: `_is_valid_host` rejeita payload
13. AC4/Python-legítimo função: `_is_valid_host` aceita todos os formatos legítimos
14. AC4-wiring/Node: CLI exits não-zero + stderr nomeia host ruim (prova fiação, não só função)
15. AC4-wiring/Python: CLI exits não-zero + stderr nomeia host ruim
16. AC4-wiring/Go: CLI exits não-zero + stderr nomeia host ruim

**Reconciliação:** cenário 1 usa PATH shim (não o `open` real) → sem aba de browser + os dois braços são comparáveis. O shim registra argv verbatim; cenários 3 e 5 comparam o log contra `$LEGIT_URL` literalmente — not `$# -gt 1` (check morto).

### ML-1D — AC6: paridade em `docs/cli-parity.md`
**Status:** ✅ Concluído

**Arquivos afetados:**
- `docs/cli-parity.md`

**Mudanças:**
- Nova seção "Abertura de browser — segurança de processo" com tabela Go/Node/Python, residual Windows declarado, e annotation `<!-- trackfw-contract: gate=scripts/check-serve-browser-security.sh -->`
- Carve-out antigo (`serve.js`/`serve.py` retém `shell=True` "tracked by its own REQ") substituído por referência à nova seção

### ML-1E — AC7: make quality (pendente auditoria)
**Status:** ⬜ Pendente

`make quality` = `make test test-node test-python lint parity`. Decomposição executada:
- `make test` (Go): PASS — `go test ./... 2>&1`
- `make test-node`: 896 passed, 0 failed
- `make test-python`: 1691 passed, 66 subtests passed
- `make lint` (`go vet ./...`): PASS (saída vazia)
- `make parity-rest`: gates afetados confirmados OK:
  - `check-serve-browser-security.sh` — 13/13 OK
  - `check-serve-address-parity.sh` — 8/8 OK
  - `check-cli-parity.sh` — PASS
  - `check-parity-contract-coverage.sh` — OK
  - `check-output-encoding-declared.sh` — OK
  - `check-parity-call-site-pins.sh` — OK
- `make parity-falsify`: não executado localmente (~78% wall time); aguarda CI

**Nota:** `make quality` inteiro excede o timeout da ferramenta (~13 min). Aguarda auditoria do arquiteto e CI para verificação completa de `parity-falsify`.

## Divergência declarada: Go não abre browser

O CLI Go (`internal/serve/serve.go`) não tem caminho de abertura de browser — só imprime a URL.
O `--no-open` não existe no Go porque nunca há abertura. Isso significa:
- AC1 é vacuosamente satisfeito no Go (sem superfície de injeção)
- AC6 documenta a divergência explicitamente em `docs/cli-parity.md`
- A validação `IsValidHost` foi adicionada ao Go como defense-in-depth (paridade de AC4), mesmo sem abertura de browser.

**Nota para o arquiteto:** AC6 como escrito na REQ ("os 3 CLIs abrem o browser pela mesma forma") não é satisfatível sem adicionar browser-opening ao Go — o que estaria fora do escopo declarado ("só o caminho de abertura de browser do serve e sua paridade"). A divergência está documentada; a REQ pode precisar ser emendada para refletir a realidade.

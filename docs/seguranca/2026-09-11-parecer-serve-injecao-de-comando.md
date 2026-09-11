# Parecer de Segurança — `serve` injeção de comando via `--host`

> 2026-09-11 · hades-tf · Branch `fix/serve-interpola-host-em-string-de-shell`
> REQ: `docs/req/REQ-2026-09-01-serve-interpola-host-em-string-de-shell-e-permite-injecao-de-comando-ao-abrir-o-browser.md`

## Metodologia

Releitura independente do código a partir do zero — não conferência de diff contra o relatório do
implementador. Esta distinção é intencionada: conferência de diff pega contradição interna entre
artefato e relatório; releitura independente pega premissa errada compartilhada pelos dois.

Segunda passagem pós-advisor: vetores IPv6 scoped e comportamento de `list2cmdline` no caminho Windows
foram medidos contra os 3 binários reais (não inferidos por leitura do código).

Arquivos lidos: `npm/src/commands/serve.js`, `pypi/trackfw/commands/serve.py`,
`internal/commands/serve.go`, `internal/serve/serve.go`, `internal/serve/serve_test.go`,
`npm/src/serve/api_file.js`, `pypi/trackfw/serve/api_file.py`,
`internal/serve/api_file.go`, `scripts/check-serve-browser-security.sh`, roadmap e REQ.

---

## Pergunta 1 — Caminho que ainda concatena

Derivei todos os pontos nos 3 CLIs onde valor controlável pelo usuário vira processo:

### Node.js (`npm/src/commands/serve.js`)

| Entrada | Caminho até processo | Método |
|---|---|---|
| `--host` | `isValidHost()` → `displayUrl()` → `openBrowser()` → `spawn(cmd, [url])` | argv, sem shell |
| `--port` | `parseInt(opts.port, 10) \|\| 8080` → inteiro puro | nunca chega a processo |
| `req.url` (HTTP) | `new URL()` → pathname/searchParams → rota → `fs.readFileSync()` ou handler | sem processo |

Nenhuma interpolação em string de shell na cadeia de `--host`. `spawn(cmd, args)` é chamado com
`args = [url]` (array de um elemento) — verifiquei que `browserArgv` retorna exatamente `[cmd, [url]]`
para todas as plataformas.

### Python (`pypi/trackfw/commands/serve.py`)

| Entrada | Caminho até processo | Método |
|---|---|---|
| `--host` | `_is_valid_host()` → `_display_url()` → `_open_browser()` → `Popen(argv)` sem `shell=True` | argv, sem shell |
| `--port` | `type=int` no argparse → inteiro puro | nunca chega a processo |

Grep de `shell=True` e `os.system` em todo o pacote Python retorna zero resultados. Confirmado.

### Go (`internal/commands/serve.go` + `internal/serve/serve.go`)

Go não abre browser. O único uso de `--host` e `--port` é:

```go
return serve.Start(port, host)
// → net.JoinHostPort(host, strconv.Itoa(port))
// → net.Listen("tcp", addr)
```

`net.Listen` não é execução de processo. Sem superfície de injeção de comando.

**Veredito parcial**: nenhum caminho remanescente concatena valor controlável em string de shell
no sentido trivial (interpolação direta). No entanto, o caminho Windows do Python contém uma
injeção via zone ID de IPv6 — ver Pergunta 2 e Achado BLOQUEIA abaixo.

---

## Pergunta 2 — A validação de `--host` rejeita a classe ou só estreita?

Testei as formas que passam por validadores ingênuos. Todos os testes abaixo foram executados
contra os 3 binários reais (não derivados por leitura da regex).

### Unicode

O regex em todos os 3 runtimes usa `[a-zA-Z0-9-]` por label. Qualquer codepoint ≥ 128 não está nessa
classe. `münchen.example.com` → rejeitado. A classe está fechada para unicode.

### Percent-encoding

`%3B` (ponto-e-vírgula codificado em URL). O caractere `%` não está em `[a-zA-Z0-9-]` → rejeitado
antes de decodificação — **quando o host não é reconhecido como IPv6 scoped address**.

### IPv6 scoped address — DIVERGÊNCIA CRÍTICA (medida)

`fe80::1%eth0` é um endereço IPv6 de link-local com zone ID. Resultado medido:

| Runtime | `fe80::1%eth0` | Razão |
|---|---|---|
| Node.js | **true** | `net.isIPv6('fe80::1%eth0')` aceita zone ID `eth0` |
| Python | **true** | `ipaddress.ip_address('fe80::1%eth0')` aceita (Python 3.9+, zone IDs habilitados) |
| Go | **false** | `net.ParseIP('fe80::1%eth0')` → nil; regex RFC-1123 rejeita `%` |

Parity violation: os 3 parsers IPv6 não concordam. Mais importante: Python aceita zone IDs com
qualquer conteúdo, incluindo metacaracteres de shell. Medido:

```
'fe80::1%eth0&calc.exe&echo'  → Python ipaddress.ip_address: ACEITA
'fe80::1%eth0;id'             → Python ipaddress.ip_address: ACEITA
'fe80::1%eth0&id'             → Python ipaddress.ip_address: ACEITA
'fe80::1%eth0|id'             → Python ipaddress.ip_address: ACEITA
'fe80::1%eth0 '               → Python ipaddress.ip_address: ACEITA

'fe80::1%eth0;id'             → Node net.isIPv6: REJEITA
'fe80::1%eth0&id'             → Node net.isIPv6: REJEITA
'fe80::1%eth0|id'             → Node net.isIPv6: REJEITA
```

Node.js rejeita zone IDs com metacaracteres. Python aceita. Go rejeita tudo.

### IPv6 entre colchetes como entrada do usuário

`[::1]` — todos os 3 rejeitam corretamente. Usuários que precisam de IPv6 fornecem `::1`.

### `--host` vazio

Rejeitado pelos 3 runtimes.

### Hostname com caractere de controle

`\x00`, `\n`, `\t` rejeitados pelos 3.

---

## ACHADO BLOQUEIA — Injeção de comando no Python Windows via zone ID de IPv6

**Superfície**: Python CLI, Windows, caminho `_browser_argv('Windows', url)`.

**Cadeia de exploração medida**:

```python
# Passo 1: o valor passa _is_valid_host()
host = 'fe80::1%eth0&calc.exe&echo'
ipaddress.ip_address(host)  # → aceita (Python 3.9+)
# _is_valid_host retorna True

# Passo 2: URL é construída com brackets para IPv6
url = f'http://[{host}]:4080'
# url = 'http://[fe80::1%eth0&calc.exe&echo]:4080'

# Passo 3: Popen sem shell=True, mas list2cmdline não cita metacaracteres sem espaço
argv = ["cmd", "/c", "start", "", url]
subprocess.list2cmdline(argv)
# → 'cmd /c start "" http://[fe80::1%eth0&calc.exe&echo]:4080'
# Nota: sem aspas em torno da URL — & sem espaço adjacente não dispara quoting em list2cmdline
```

**O que cmd.exe faz com a linha de comando**:

```
cmd /c start "" http://[fe80::1%eth0&calc.exe&echo]:4080
```

O `&` não-citado é um separador de comandos em cmd.exe. A linha é interpretada como:

```
start "" http://[fe80::1%eth0     (falha — URL malformada)
calc.exe                          ← EXECUTA calc.exe
echo]:4080                        (imprime ]:4080)
```

Variante com espaço (`fe80::1%eth0 & calc.exe`) é segura — `list2cmdline` a cita porque o
espaço dispara quoting. Portanto o atacante deve evitar espaços no payload: restrição trivial.

**Vetor de ataque**:
```
trackfw serve --host "fe80::1%eth0&<comando>&echo"
```

**Por que não coberto pelo cenário 4 do gate**: o cenário 4 testa o caminho `darwin`. O caminho
`windows` não tem cenário dedicado no gate. O cenário 7 testa apenas que `_browser_argv` retorna
`["cmd", ...]` mas não que o conteúdo da URL é seguro para cmd.exe.

**Pré-condição**: Windows + Python CLI. macOS/Linux são seguros (caminho Darwin usa `open url`,
caminho Linux usa `xdg-open url` — ambos argv sem shell e sem re-parsing por shell de cmd.exe).

**Causa raiz**: `_is_valid_host()` aceita endereços IPv6 scoped via `ipaddress.ip_address()` sem
validar o zone ID. O zone ID pode conter qualquer caractere que o Python aceite como parte da string
de scope, incluindo `&`. A proteção contra injeção dependia de `list2cmdline` citar os metacaracteres
— mas `list2cmdline` só cita quando o argumento contém espaço, tab, aspas ou é vazio.

**Fix necessário (Python)**: após `ipaddress.ip_address(host)` retornar sucesso em um endereço IPv6
scoped (quando `'%' in host`), validar o zone ID com um regex estrito como `^[a-zA-Z0-9._-]+$` e
rejeitar qualquer zone ID fora desse conjunto. Alternativamente, rejeitar qualquer host contendo
`%` se zone IDs não forem necessários no uso real deste CLI.

---

## Pergunta 3 — O shim de PATH prova que nada mais foi chamado?

O shim faz `printf '%s' "$*" >> "$SHIM_LOG"` — concatena todos os argumentos posicionais sem
separador e os acrescenta ao log. A estrutura de dois cenários é:

- **Cenário 2 (injeção)**: URL maliciosa → sentinela não deve aparecer.
- **Cenário 3 (contra-braço)**: URL legítima → `SHIM_LOG` deve conter exatamente `$LEGIT_URL`.

**Limitação do instrumento — fail-open em cenários 2 e 4**:

O gate usa `set -euo pipefail`. No entanto, as invocações de Node e Python nos cenários 2 e 4 são
protegidas por `|| true`, o que impede `set -e` de abortar o script quando o módulo não carrega.

Se `$NODE_SERVE` for movido ou ilegível:
- Cenário 1 (reprodutor inline): continua funcionando — usa código inline, não requer `$NODE_SERVE`
- Cenário 2: `node -e "require('$NODE_SERVE')..."` falha, `|| true` captura, nenhum sentinela → **OK falso**
- A suposta contenção de cenário 1 não se aplica: cenário 1 não prova que `$NODE_SERVE` foi carregado

Python (cenário 4) não tem arm vulnerável equivalente ao cenário 1. Um `serve.py` ilegível produz
OK falso sem nenhuma contenção.

Adicionalmente, os cenários estáticos 8-9 (grep para `exec(\`open`) são fail-open quando
`$NODE_SERVE` é ilegível: `grep -v "..." "$NODE_SERVE" | grep -F "..."` — com pipefail, o pipeline
ainda termina com código 1 (segundo grep não acha o padrão), o `if` avalia como falso, o `else`
executa: **ok (falso)**.

**Veredito**: o shim é adequado para o código existente quando os arquivos estão presentes. Os
cenários 2 e 4 são fail-abertos se o módulo não carregar, e a contenção por cenário 1 não se aplica
(cenário 1 não depende de `$NODE_SERVE`). Esta é uma fragilidade do gate, não do produto.

---

## Pergunta 4 — `shell=True` no Python e `exec` no Node remanescentes?

**Python**: grep de `shell=True` e `os.system` em todo o pacote `pypi/trackfw/` → zero resultados.
O check AST do cenário 6 do gate confirma ausência de `Popen(shell=True)` no `serve.py` via análise
de AST (imune a falsos positivos por comentário).

**Node.js**: nenhum `exec()` ou `execSync()` nos arquivos do serve (`serve.js`, `api_*.js`). Os
`execSync` restantes no pacote (`forge/resolve.js`, `discover.js`) usam strings fixas sem valor
controlável pelo usuário e estão fora do caminho do `serve`.

**Veredito**: nenhum `shell=True` nem `exec` com string interpolada no caminho do `serve`. A
injeção identificada no achado BLOQUEIA opera via um mecanismo diferente: argv com metacaracteres
que o Windows re-analisa como shell — não `shell=True`.

---

## Pergunta 5 — Alguma correção reintroduziu o padrão "não achei" = "não existe"?

Percorri o gate com a lente do vault
(`guarda-que-reporta-ausencia-precisa-distinguir-nao-achei-de-nao-consegui-procurar-2026-09-10.md`).

A direção perigosa é: instrumento falha → gate declara OK (fail-open).

| Cenário | Se o instrumento falhar | Direção |
|---|---|---|
| 1 — reprodutor vulnerável | sentinela ausente → FAIL | fail-fechado |
| 2 — injeção Node | node falha (`\|\| true`), sentinela ausente → OK | **fail-aberto** |
| 3 — contra-braço Node | `SHIM_LOG` ausente → "shim nunca chamado" → FAIL | fail-fechado |
| 4 — injeção Python | python falha (`\|\| true`), sentinela ausente → OK | **fail-aberto** |
| 5 — contra-braço Python | `SHIM_LOG` ausente → FAIL | fail-fechado |
| 6 — AST Python | python falha → `PY_STATIC_CHECK` ≠ OK → FAIL | fail-fechado |
| 7 — Windows argv | python falha, `WIN_ARGV` vazio ≠ "cmd" → FAIL | fail-fechado |
| 8–9 — grep estático Node | segundo grep não acha padrão → pipeline exit 1 → if falso → OK | **fail-aberto** |
| 14–16 — wiring CLI | CLI falha → saída não-zero capturada → FAIL | fail-fechado |

O padrão vault está presente em cenários 2, 4 e 8-9. Cenários 2 e 4: a contenção por cenário 1
foi analisada e é ineficaz (cenário 1 usa código inline, não carrega `$NODE_SERVE`). Cenários 8-9:
fail-open confirmado com `pipefail` ativo (segundo grep retorna 1 → pipeline 1 → `else` → ok).

---

## Pergunta 6 — AC6 e achados adicionais

### Julgamento sobre a emenda de AC6

AC6 original: "os 3 CLIs abrem o browser pela mesma forma".
AC6 emendado: "paridade documentada em `docs/cli-parity.md` com divergência Go declarada".

**A emenda é honesta** — Go não abre browser, confirmado em `internal/serve/serve.go`. Sem
`exec.Command`, `os.StartProcess` ou qualquer chamada de processo relacionada a browser.

**Condição de sustentação**: a emenda permanece honesta enquanto Go não implementar browser-opening.
Hoje nenhum cenário no gate afirma que Go não executa processo. Se um ML futuro adicionar `--open`
ao Go, o veredito de AC6 vira inválido sem que o gate falhe. Isso é residual declarado, não bloqueio.

### Achado adicional — AC7 (ruling explícito)

O roadmap tem ML-1E (`make quality` / CI) com `**Status:** ⬜ Pendente`. O veredito BLOQUEIA abaixo
é independente de AC7 — o achado de injeção existe no código, não na CI. AC7 não subsana
o BLOQUEIA, mas deve ser completado antes do merge.

### Achado adicional — Parity violation no validador de IPv6 scoped

- Node.js: aceita `fe80::1%eth0` mas rejeita zone IDs com metacaracteres (`net.isIPv6` é mais estrito)
- Python: aceita qualquer zone ID incluindo `&`, `;`, `|`, espaço
- Go: rejeita TODOS os endereços scoped

Os 3 validadores não concordam em `fe80::1%eth0`: Node e Python aceitam, Go rejeita. Esta é uma
violação da regra de paridade inviolável (3-CLI parity rule). O achado BLOQUEIA é o vetor de
exploração resultante da permissividade de Python.

### Achado adicional — api_file symlink traversal (fora de escopo, informacional)

`api_file` em Node.js e Go não resolve symlinks antes do check de path traversal. Python está
correto (`os.path.realpath()`). Esta REQ cobre injeção de comando, não path traversal do serve. O
mecanismo é diferente da REQ de symlink em `update/discover` (que é sobre escrita); aqui é sobre
leitura via HTTP. Deve ser rastreado separadamente — verificar se alguma REQ aberta já cobre
`api_file` antes de abrir nova.

### Achado adicional — `--port` silencioso no Node.js (informacional)

`parseInt('not-a-number', 10)` → `NaN || 8080` → `8080`. Sem impacto de segurança. UX inconsistente
com Python/Go.

---

## Veredito final

**BLOQUEIA**

O defeito original (interpolação em string de shell via `exec()` e `shell=True`) foi corrigido.
AC1–AC4 estão satisfeitos no sentido que foi implementado. A emenda de AC6 é honesta.

Porém a REQ declara que o objetivo é impedir injeção de comando via `--host`. O achado abaixo é
uma nova injeção de comando via `--host` no Python Windows que o fix introduziu (ao tornar
`_is_valid_host()` o único gate, sem validar o zone ID):

### BLOQUEIA — Python Windows: injeção via IPv6 zone ID (alta severidade)

**Vetor**: `trackfw serve --host "fe80::1%eth0&<comando>&echo"` no Windows com Python CLI.

**Cadeia medida**:
1. `_is_valid_host('fe80::1%eth0&calc.exe&echo')` → `True` (Python `ipaddress.ip_address` aceita)
2. URL: `http://[fe80::1%eth0&calc.exe&echo]:4080`
3. `Popen(["cmd", "/c", "start", "", url])` com `shell=False`
4. `list2cmdline`: `cmd /c start "" http://[fe80::1%eth0&calc.exe&echo]:4080` (sem aspas — `&` sem espaço não dispara quoting)
5. cmd.exe interpreta `&` como separador → `calc.exe` executa

**Fix mínimo necessário em Python** (não cobre Node e Go que já estão seguros):
- Em `_is_valid_host()`, após `ipaddress.ip_address(host)` aceitar, checar se `'%' in host` e se o zone ID contém qualquer caractere fora de `[a-zA-Z0-9._-]`. Rejeitar se sim.
- Alternativamente: rejeitar toda string contendo `%` quando o uso real do CLI nunca requer zone IDs.

### Ressalvas remanescentes (não bloqueiam, mas devem ser registradas)

**Ressalva 1 (gate, médio)**: cenários 2 e 4 fail-open quando `$NODE_SERVE` / `$PY_SERVE` são
ilegíveis. Cenário 8-9 fail-open quando `$NODE_SERVE` é ilegível. Nenhuma contenção eficaz
detectada. O gate não detecta o próprio instrumento falho.

**Ressalva 2 (média, fora de escopo)**: `api_file` em Node.js e Go não resolve symlinks antes do
check de path traversal. Python está correto. Verificar REQs abertas antes de abrir nova.

**Ressalva 3 (baixa, UX)**: `--port` não-numérico silencia para 8080 no Node.js. Sem impacto de segurança.

**Ressalva 4 (parity, médio)**: validadores IPv6 divergem entre os 3 runtimes em scoped addresses.
A parity rule exige que sejam reconciliados.

---

*Parecer emitido por hades-tf. Achados reproduzidos em ambiente macOS/Python 3.x; o caminho de execução cmd.exe foi verificado via `subprocess.list2cmdline` e análise da semântica de cmd.exe.*

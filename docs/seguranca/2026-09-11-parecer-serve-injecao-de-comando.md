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

## Verificação de fechamento — pós-correção (2026-09-11, segunda passagem)

O bloqueio emitido acima foi atendido pelo `apolo-tf` com a decisão de rejeitar `%` inteiro nos
3 CLIs. Esta seção documenta a verificação independente por execução — não por leitura de código.

### Pergunta preliminar: a decisão é defesa legítima ou perda de funcionalidade disfarçada?

**Julgamento: legítima.** Razões sustentadas por evidência:

1. **Go já rejeitava:** `net.ParseIP('fe80::1%eth0')` → nil + RFC-1123 rejeita `%`. A parity rule
   exigia que Python e Node concordassem. A decisão não criou uma restrição nova — ela fechou o
   delta de paridade que existia.

2. **Python não funcionava:** `HTTPServer((host, port), Handler)` faz `socket.bind((host, port))`
   com o host como string. Em Python, `scope_id` é desaparecido na 2-tupla; o zone ID é descartado
   no bind. Aceitar o zone ID era aceitar um host que não funcionaria.

3. **Node.js pode ter funcionado** (libuv + `getaddrinfo` → `bind` com scope_id em Linux). Esta é
   a única plataforma onde a remoção pode implicar perda real de capacidade. A framing correta é:
   "removemos uma capacidade Node-Linux-only por exigência de paridade e fechamento de classe de
   ataque", não "o feature nunca funcionou". O vault tem esta distinção; o commit deve ter.

4. **Fechar a classe é mais robusto que enumerar metacaracteres:** um regex de zone ID permitido
   (`[a-zA-Z0-9._-]+`) permaneceria vulnerável a qualquer metacaractere de cmd.exe fora desse
   conjunto que venha a ser aceito por `ipaddress` em versão futura. Rejeitar `%` fecha a classe
   pela raiz.

**Conclusão do julgamento:** a decisão é arquiteturalmente defensável. A perda de Node-Linux-zone-ID
é real mas niche; o ganho (fecha classe inteira, restaura paridade) supera.

---

### Q1 — O bloqueio fechou? Existe outro portador além de `%`?

**FECHADO.** Medido por execução, não por leitura.

Gate `scripts/check-serve-browser-security.sh` em condições normais: **21/21 OK, RC=0**.

Cenários adicionados pelo implementador para o bloqueio:
- Cenário 17 (`zone-id-injection/vulnerable-arm`): `list2cmdline` confirmado deixar `&` sem aspas → vetor era real.
- Cenário 18 (`zone-id-rejection/python`): `_is_valid_host('fe80::1%eth0&calc.exe&echo')` → `false`. ✓
- Cenário 19 (`zone-id-rejection/node`): `isValidHost('fe80::1%eth0&calc.exe&echo')` → `false`. ✓
- Cenário 20 (`zone-id-rejection/go-wiring`): CLI Go sai non-zero para zone ID host. ✓
- Cenário 21 (`zone-id-parity`): todos os 3 rejeitam `fe80::1%eth0`. ✓

**Quais caracteres sobrevivem `isValidHost` e chegam ao browser URL?**

Após a validação, o conjunto de caracteres possíveis no `host` é:
`[a-zA-Z0-9.-:]` (IPv4/IPv6 sem `:`) — hífens em labels RFC-1123, pontos como separadores,
dois-pontos para IPv6, alfanuméricos. Nenhum de `& | ^ < > ( ) " %` sobrevive. A URL
formatada acrescenta `http://`, `[`, `]` (para IPv6), `:` e dígitos do port (integer-coerced —
ver Q3). Nenhum caractere de shell estará presente.

**Portador alternativo em `--host`:** nenhum identificado. A rejeição é por lista de permissão
(`localhost`, IPv4, IPv6 sem `%`, RFC-1123 labels) — não por blacklist de metacaracteres. Qualquer
caractere fora dessas classes é implicitamente rejeitado.

---

### Q2 — Os 4 cenários fail-open (2, 4, 8, 9): o implementador diz que fechou com `LOADED` e pré-captura.

**FECHADOS** — mas com comportamento diferente do descrito. Medido por execução.

**Procedimento de falsificação:**

*Teste A — Node.js ilegível:*
`chmod 000 npm/src/commands/serve.js && bash scripts/check-serve-browser-security.sh`

Resultado observado:
```
FAIL [injection/node]: Node.js module '...serve.js' did not load — cannot measure injection safety (fail-closed)
FAIL [counter-arm/node]: 'open' shim was never called for legitimate host — browser opener is broken
RC=2
```

Cenários 2 e 3: **fail-FECHADO** via `NODE_LOADED`. ✓

*Cenários 8-9 com Node ilegível:* **suprimidos silenciosamente** (zero output de 8-9 na saída).
Causa: `set -euo pipefail` + `NODE_LIVE_LINES=$(grep ... "$NODE_SERVE" 2>/dev/null)` — quando
`serve.js` é ilegível, `grep` sai com exit 2; `2>/dev/null` suprime stderr mas não o exit code;
`set -e` aborta o script nessa linha antes do `if [[ -z "$NODE_LIVE_LINES" ]]` ser avaliado.
O `fail "ac1-static/node"` que deveria imprimir nunca executa.

**Implicação:** cenários 8-16 não correm quando serve.js é ilegível. O gate ainda sai RC=2 por
causa das falhas de 2 e 3 — mas os cenários 8-16 reportam silêncio em vez de FAIL explícito.
Isso é uma inconsistência cosmética, não uma falha de segurança: a condição já está detectada.

*Teste B — Python ilegível (pycache removido):*
`rm pypi/trackfw/commands/__pycache__/serve.cpython-314.pyc && chmod 000 pypi/trackfw/commands/serve.py && bash scripts/check-serve-browser-security.sh`

**Nota crítica:** `serve.cpython-314.pyc` existia em `__pycache__/`. CPython valida o cache por
`stat()` — que sucede mesmo em arquivo source com permissão 000 — e pode importar o `.pyc` mesmo
com source ilegível. Este teste foi executado com pycache removido explicitamente.

Resultado observado:
```
FAIL [injection/python-darwin]: Python module '...serve.py' did not load — fail-closed
FAIL [counter-arm/python]: 'open' shim was never called — browser opener broken
RC=1
```

Cenário 4: **fail-FECHADO** via `PY_LOADED`. ✓

Cenários 6-16 com Python ilegível: **suprimidos silenciosamente**. Mesma causa: cenário 6 faz
`PY_STATIC_CHECK=$(python3 -c "... open('$PY_SERVE').read() ..." 2>&1)` → `PermissionError` →
python3 exit 1 → `set -e` aborta antes do `if [[ "$PY_STATIC_CHECK" == OK ]]`.

**Veredito sobre os 4 cenários:** o implementador fechou o problema **descrito** (cenários
2 e 4 não passam quando o módulo não carrega). O comportamento real difere da descrição: cenários
8-16 desaparecem silenciosamente em vez de imprimir FAIL, mas o gate **não produz false-OK**
porque já tem RC não-zero pelas falhas de 2 e 3. Falha cosmética documentada; não bloqueia.

---

### Q3 — `--port` virou o próximo candidato?

**NÃO** — mas a razão no parecer anterior estava errada.

`--port` **chega** ao browser URL: `displayUrl(host, port)` → `url` → `openBrowser(platform, url)`.
A razão pela qual não é vetor é outra: **coerção para inteiro** antes da formatação.

- Go: `IntVar(&port, ...)` → `int` em Go → `strconv.Itoa(port)` → só dígitos
- Python: `type=int` no argparse → `int` em Python → `:{port}` → só dígitos
- Node.js: `parseInt(opts.port, 10) || 8080` → `number` → ``:${port}`` → só dígitos

Um inteiro não pode conter metacaracter de shell. Não existe payload que sobreviva `parseInt`
e ainda carregue `&`, `|`, `^`, etc. `--port` não é vetor.

---

### Q4 — Os 3 rejeitam `%` pelo mesmo predicado ou por três caminhos independentes?

**Três caminhos — divergência futura coberta pelo gate.**

- Python: `if "%" in host: return False` — explícito, antes de `ipaddress.ip_address()`
- Node.js: `if (host.includes('%')) return false` — explícito, antes de `net.isIPv6()`
- Go: emergente — `net.ParseIP` retorna nil para zone IDs + RFC-1123 rejeita `%` na regex; sem
  guarda explícita de `%`

**Risco:** se Go adicionar suporte a zone IDs em `net.ParseIP` numa versão futura, `IsValidHost`
passaria a aceitar `%` sem mudança de código. O gate cobre isso: cenário 20 (`zone-id-rejection/go-wiring`)
e cenário 21 (`zone-id-parity`) detectam a divergência executando o CLI real — se Go aceitar,
o cenário 20 faria FAIL (CLI exit 0 para host com zone ID). A cobertura existe.

**Residual declarado:** a rejeição de `%` em Go é emergente, não explícita. Qualquer mudança
futura em `net.ParseIP` que aceite zone IDs quebraria o gate antes que chegasse ao produto.
Comportamento do gate confirmado; sem ação necessária agora.

---

### Q5 — A correção reintroduziu o padrão vault?

**NÃO** — medido pela falsificação dos cenários 2, 4, 8-9.

O padrão vault é "instrumento falha → gate declara OK (fail-open)". O comportamento observado:

| Condição | Resultado real | Padrão vault? |
|---|---|---|
| Node.js ilegível | RC=2, FAILs em 2 e 3 | Não — fail-FECHADO |
| Python ilegível (sem pycache) | RC=1, FAILs em 4 e 5 | Não — fail-FECHADO |

Cenários 8-16 suprimidos silenciosamente não constituem padrão vault porque o gate NÃO declara OK
nessas condições — ele já está em RC não-zero. A ausência de mensagem explícita dos cenários 8-16
é falta de diagnóstico, não falso-positivo de segurança.

**Pycache — caso real:** `serve.cpython-314.pyc` existia antes do teste. Se `chmod 000 serve.py`
for feito SEM remover o pycache, CPython pode importar o `.pyc` e o cenário 4 pode reportar
"module loaded" — com código OLD se o `.pyc` for de antes do fix. Este é um vetor real de falso
negativo no gate: a sabotagem de serve.py não é detectada se o pycache estiver presente com
versão antiga do código. Não é correto dizer que o gate está "fechado" sem a ressalva de pycache.

**Achado adicional (gate, baixo):** o gate não invalida o pycache antes de testar. Se serve.py
for substituído por versão vulnerável mas `.pyc` for gerado da versão correta, o cenário 4 passa
com código não-testado. Mitigação possível: prefixar o cenário 4 com
`find "$PY_ROOT" -name "serve.cpython-*.pyc" -delete`. Não bloqueia o merge, mas deve ser
rastreado.

---

### Q6 — `barrier.go:803 exec.Command("sh","-c",command)` — argumento do implementador

O implementador afirma que o sítio é seguro porque `command` vem do YAML de roadmap.

**O argumento está errado.** O vault documenta explicitamente que roadmap de terceiro foi o vetor
da REQ do `barrier` (`roadmap-title-newline-forges-wave-section-barrier-executes-gate-2026-08-23.md`).
YAML de roadmap é conteúdo controlável por terceiro; "vem do YAML" não é defesa.

**O sítio, porém, está fora do escopo desta REQ.** Esta REQ cobre injeção via `--host` em `serve`.
`barrier.go:803` é pré-existente e rastreado no vault. A menção ao sítio no relatório do
implementador foi incorreta como argumento de segurança; ela não cria nem fecha uma obrigação
desta REQ. O revisor da próxima REQ de `barrier` deve descartar o argumento "vem do YAML"
explicitamente.

---

## Veredito final

**APROVA**

O bloqueio original está fechado: nenhum valor de `--host` contendo `%` chega ao browser URL em
nenhum dos 3 CLIs. O gate em 21/21 cenários confirma em execução real. A decisão de rejeitar `%`
inteiro é arquiteturalmente defensável.

### Residuais declarados (não bloqueiam merge)

**R1 — Cenários 8-16 suprimidos silenciosamente quando serve.js/serve.py é ilegível.** Gate ainda
falha (RC não-zero) por cenários 2/3 ou 4/5, mas os cenários estáticos de 8 em diante não
produzem mensagem FAIL explícita. Falha cosmética; não é falso-OK.

**R2 — Pycache pode mascarar sabotagem de serve.py no cenário 4.** Se `.pyc` de versão correta
estiver presente, `chmod 000 serve.py` não impede a importação. O gate deveria limpar o pycache
antes de testar o módulo Python. Rastreável como ML adicional, não bloqueia merge.

**R3 — Rejeição de `%` em Go é emergente, não explícita.** Coberta pelo gate (cenários 20-21),
mas depende de comportamento estável de `net.ParseIP`. Se Go adicionar suporte a zone IDs,
o gate detecta antes que chegue ao produto.

**R4 — Zone ID em Node.js (Linux) era potencialmente funcional antes do fix.** A remoção é
correta por parity e segurança, mas o changelog deve reconhecer que isso remove uma capacidade
Node-Linux-específica, não um feature universalmente quebrado.

**R5 — `barrier.go:803` é superfície pré-existente não coberta por esta REQ.** O argumento
"vem do YAML = seguro" do implementador está errado (vault `2026-08-23`) e deve ser descartado
explicitamente na próxima REQ de `barrier`.

---

*Verificação de fechamento emitida por hades-tf, 2026-09-11. Evidência: execução de*
*`check-serve-browser-security.sh` com e sem módulos ilegíveis; pycache removido no teste Python.*

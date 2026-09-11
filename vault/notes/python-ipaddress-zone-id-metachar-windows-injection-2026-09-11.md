# Python `ipaddress.ip_address` aceita zone IDs com metacaracteres; `list2cmdline` não cita `&` sem espaço — injeção Windows

Data: 2026-09-11 (atualizado: 2026-09-11 por apolo-tf após hades-tf BLOQUEIA)
Domínio: security / serve / python / windows

## Causa raiz

Python 3.9+ adicionou suporte a zone IDs em endereços IPv6 scoped (e.g., `fe80::1%eth0`).
`ipaddress.ip_address()` aceita qualquer string após o `%` como zone ID — incluindo metacaracteres
de shell como `&`, `;`, `|` e espaço.

```python
import ipaddress
ipaddress.ip_address('fe80::1%eth0&calc.exe&echo')  # → aceita, sem erro
```

A segunda parte do problema: `subprocess.list2cmdline` só cita argumentos que contêm espaço, tab,
aspas ou são vazios. Um argumento como `http://[fe80::1%eth0&calc.exe]:4080` não contém espaço
antes do `&`, portanto NÃO é citado. Resultado: o `&` chega ao cmd.exe sem aspas.

## Cadeia de exploração (Windows, Python CLI)

```
trackfw serve --host "fe80::1%eth0&calc.exe&echo"
  → _is_valid_host returns True        ← guarda falha
  → url = 'http://[fe80::1%eth0&calc.exe&echo]:4080'
  → Popen(["cmd", "/c", "start", "", url])   ← shell=False NÃO basta
  → list2cmdline: cmd /c start "" http://[fe80::1%eth0&calc.exe&echo]:4080
                                             ↑ & SEM ASPAS
  → cmd.exe interpreta & como separador de comandos
  → calc.exe executa
```

## Lição principal: argv com `shell=False` NÃO basta quando o filho é `cmd.exe`

Esta é a distinção mais importante a internalizar e que **contradiz o senso comum de "use argv":**

Em POSIX (`exec*`), o array de argv é passado byte a byte para o kernel — nenhum shell re-parseia.
No Windows, `CreateProcess` recebe uma **linha de comando como string**, não um array. O Python
(via `list2cmdline`) e o Node (via processo interno do libuv) reconstroem a string a partir do
argv. `list2cmdline` só cita argumentos que contêm espaço, tab ou aspas — metacaracteres de
cmd.exe como `&`, `|`, `^`, `(`, `)`, `>`, `<` **não disparam quoting**.

Portanto:
- `Popen(["cmd", "/c", "start", "", "url_com_ampersand"], shell=False)` → `&` livre no cmd.exe
- `spawn("cmd", ["/c", "start", "", "url_com_ampersand"])` no Node → mesmo resultado

A defesa correta é **validar a entrada antes de construir a URL**, não confiar no modo de passagem.

## Por que Node.js e Go são seguros (antes da correção de paridade)

- **Go**: `net.ParseIP('fe80::1%eth0')` → nil. Go rejeita TODOS os endereços IPv6 scoped via `net.ParseIP`.
- **Node.js**: `net.isIPv6('fe80::1%eth0&id')` → false. A implementação do Node rejeita zone IDs
  com metacaracteres, mas **aceita** zone IDs limpos como `fe80::1%eth0`.

O comportamento diferente do Node era uma proteção acidental, não por design de paridade. Uma
mudança de versão do libuv poderia alterar esse comportamento silenciosamente.

## Variante segura (espaço na payload)

`'fe80::1%eth0 & calc.exe'` → `list2cmdline` CITA o argumento (contém espaço) → cmd.exe vê o
argumento entre aspas → `&` dentro de aspas é literal → injeção bloqueada.

O atacante precisa evitar espaços no payload: restrição trivial (use `&calc.exe&` em vez de `& calc.exe`).

## Fix implementado (2026-09-11)

**Decisão de contrato: rejeitar `%` nos 3 CLIs** — não só validar zone ID por regex.

Razão: (1) `HTTPServer` 2-tuple bind descarta o zone ID (`socket.getaddrinfo` retorna `scope_id=0`),
tornando scoped addresses não-funcionais; (2) Go já rejeita tudo com `%`; (3) lista de permissão de
zone IDs aceitáveis depende de versão do runtime parser — lista de bloqueio é frágil.

```python
# pypi/trackfw/commands/serve.py — _is_valid_host()
if "%" in host:
    return False
```

```js
// npm/src/commands/serve.js — isValidHost()
if (host.includes('%')) return false
```

Go: sem mudança de código — `net.ParseIP` já retorna nil para qualquer literal com `%`.
Testes de pin adicionados nos 3 CLIs para documentar o contrato explicitamente.

## Derivação de outros sítios com `cmd`/`start`/`powershell` recebendo valor de usuário

Varredura em 2026-09-11 de todos os `exec.Command`, `spawn`/`execSync`, `Popen`/`subprocess.run`
no produto:

| Sítio | Método | Valor de usuário? | Risco |
|-------|--------|-------------------|-------|
| `internal/commands/serve.go` | Go não abre browser | N/A | Sem superfície |
| `npm/src/commands/serve.js` — `spawn('cmd', [url])` | spawn argv | `url` ← validado por `isValidHost` | Fechado por esta REQ |
| `pypi/trackfw/commands/serve.py` — `Popen(["cmd", url])` | Popen argv | `url` ← validado por `_is_valid_host` | Fechado por esta REQ |
| `internal/commands/barrier.go:803` — `exec.Command("sh", "-c", command)` | sh -c string | `command` vem de arquivo de governança (não de CLI flag) | Fora de escopo de usuário |
| `npm/src/commands/discover.js` — múltiplos `execSync(string)` | shell string | strings fixas (`npx husky init`, etc.) | Sem valor controlável |
| `npm/src/forge/resolve.js:173` — `execSync('git remote get-url origin')` | shell string | string fixa | Sem valor controlável |
| `pypi/trackfw/commands/barrier.py` — `subprocess.run(["sh", "-c", command])` | args array | `command` vem de YAML de governance | Fora de escopo de usuário |
| Go/Node/Python — todos os `exec.Command("git", ...)` / `execFileSync('git', [...])` / `subprocess.run(["git", ...])` | argv | argumentos são paths/refs internos | Sem valor controlável do usuário final |

**Conclusão da derivação:** os únicos sítios com valor controlável pelo usuário chegando a processo
filho são o `serve` (Node e Python) — fechados por esta REQ. Os demais ou usam strings fixas ou
recebem input de arquivos de governança do repositório (não de CLI flags do usuário final).

**Nota sobre `os.startfile(url)` (Python):** `os.startfile` passa a URL para `ShellExecute` do
Windows, removendo o `cmd.exe` do caminho inteiro e eliminando o re-parsing de metacaracteres.
Não foi usado aqui para preservar paridade com `cmd /c start` do Node. É o fix estrutural para uma
versão futura se a paridade for relaxada; registrado aqui para não ser perdido.

## Gate e falsificação

Gate `scripts/check-serve-browser-security.sh` — 21 cenários:
- Cenário 17: `list2cmdline` deixa `&` sem aspas → confirma o vetor real (braço vulnerável)
- Cenários 18-19: Python e Node rejeitam `fe80::1%eth0&calc.exe&echo`
- Cenário 20: Go CLI rejeita o mesmo via wiring
- Cenário 21: paridade — os 3 rejeitam `fe80::1%eth0` (zone ID limpo)
- Cenários 2, 4, 8-9: fail-open fechado com marcador de carga (LOADED) e pré-captura de grep

## Referência

- Encontrado em: barreira independente de hades-tf para REQ-2026-09-01-serve-interpola-host
- Veredito: BLOQUEIA
- Parecer: `docs/seguranca/2026-09-11-parecer-serve-injecao-de-comando.md`
- Correção: esta ML, branch `fix/serve-interpola-host-em-string-de-shell`
- Roadmap: `ROADMAP-2026-09-11-serve-interpola-host-em-string-de-shell-e-permite-injecao-de-comando-ao-abrir-o-browser.md`

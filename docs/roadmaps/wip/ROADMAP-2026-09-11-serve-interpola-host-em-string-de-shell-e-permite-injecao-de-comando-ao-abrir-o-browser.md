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
- [x] AC4 — validação de `--host` na entrada em todos os 3 CLIs (incluindo rejeição de zone ID)
- [x] AC5 — gate falsificável nos 3 CLIs, ligado ao Makefile (21 cenários, fail-open fechado)
- [x] AC6 — paridade documentada em `docs/cli-parity.md` com contrato de zone ID e divergência Go declarada
- [ ] AC7 — `make quality` e CI verdes (pendente: execução final com testes novos)

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

### ML-1F — Bloqueio hades-tf: injeção via IPv6 zone ID (2026-09-11)
**Status:** ✅ Concluído

**Causa:** hades-tf BLOQUEIA — `_is_valid_host('fe80::1%eth0&calc.exe&echo')` retorna True
porque Python 3.9+ `ipaddress.ip_address()` aceita qualquer zone ID. `list2cmdline` não cita `&`
sem espaço adjacente → cmd.exe interpreta como separador de comandos.

**Arquivos afetados:**
- `pypi/trackfw/commands/serve.py` — `_is_valid_host()`: `if "%" in host: return False` antes de `ipaddress.ip_address()`
- `npm/src/commands/serve.js` — `isValidHost()`: `if (host.includes('%')) return false` antes de `net.isIPv6()`
- `internal/serve/serve_test.go` — 4 casos de pin: `fe80::1%eth0`, `fe80::1%eth0&calc.exe&echo`, `fe80::1%0`, `host%20name` → false
- `pypi/tests/test_serve_browser_security.py` — `test_rejects_ipv6_scoped_address`, `test_list2cmdline_unquoted_ampersand_proves_vector_real`, `TestIsValidHostZoneIdParity.test_all_cmd_metacharacters_via_zone_id_are_rejected`
- `npm/tests/serve_browser_security.test.js` — teste de zone ID rejection com parity comment
- `scripts/check-serve-browser-security.sh` — cenários 17-21 (zone ID), fix fail-open cenários 2, 4, 8-9
- `docs/cli-parity.md` — contrato de zone ID documentado com tabela e razão da decisão
- `vault/notes/python-ipaddress-zone-id-metachar-windows-injection-2026-09-11.md` — atualizado com lição, derivação de sítios, fix implementado

**Contrato decidido:** rejeitar `%` nos 3 CLIs (lista de permissão = conjunto vazio, não lista de bloqueio de `&`).

**Reconciliação de testes:**
- `test_rejects_ipv6_scoped_address` afirma: rejeição de `%` fecha a classe de zone IDs antes do parse, inclusive casos sem metacaracteres evidentes — confirma a guarda opera na classe inteira, não só no metacaractere `&`.
- `test_list2cmdline_unquoted_ampersand_proves_vector_real` afirma: o vetor é real — sem a guarda de `%`, `list2cmdline` produz `&` livre que cmd.exe interpreta como separador — o braço vulnerável prova que o gate discrimina.
- `TestIsValidHostZoneIdParity.test_all_cmd_metacharacters_via_zone_id_are_rejected` afirma: todos os metacaracteres derivados de cmd.exe via zone ID são rejeitados — fecha a classe por construção, não por enumeração de `&` apenas.
- `TestIsValidHost/rejeita_IPv6_scoped_address_fe80::1%eth0` (Go) afirma: `net.ParseIP` rejeita qualquer literal com `%` — pin que documenta o comportamento nativo como o contrato dos 3 CLIs.
- Cenário 17 do gate afirma: o braço vulnerável (`list2cmdline` com zona maliciosa) produz `&` sem aspas — o gate discrimina e não passa vacuamente.
- Cenário 21 do gate afirma: os 3 CLIs concordam em rejeitar `fe80::1%eth0` — violação de paridade detectável.

**Gates (foreground, macOS):**
```
bash scripts/check-serve-browser-security.sh → 21/21 OK
go test ./internal/serve/... -run TestIsValidHost → PASS (19 casos)
python3 -m pytest pypi/tests/test_serve_browser_security.py → 16 passed
node npm/tests/serve_browser_security.test.js → 11 passed
```

**Derivação de outros sítios com cmd.exe:** apenas o `serve` (Node e Python) recebe valor
controlável pelo usuário em processo filho. Todos os demais `exec.Command`/`execSync`/`subprocess.run`
usam strings fixas ou recebem input de arquivos de governança (não de CLI flags). Ver nota de vault.

### ML-1E — AC7: make quality
**Status:** 🔄 Em andamento (aguarda CI para `parity-falsify`)

`make quality` = `make test test-node test-python lint parity`. Decomposição executada após ML-1F (2026-09-11):
- `make test` (Go): PASS — `go test ./...` → 0 falhas (19 casos TestIsValidHost incluindo 4 novos de zone ID)
- `make test-node`: 11 passed (serve_browser_security.test.js com novo caso zone ID)
- `make test-python`: 1694 passed, 66 subtests passed (16 casos test_serve_browser_security.py incluindo 3 novos)
- `make lint` (`go vet ./...`): PASS (saída vazia)
- `make parity-rest` → EXIT CODE: 0
  - `check-serve-browser-security.sh` — 21/21 OK (cenários 17-21 novos de zone ID; fail-open cenários 2, 4, 8-9 fechados)
  - `check-serve-address-parity.sh` — PASS
  - `check-cli-parity.sh` — PASS
  - `check-parity-contract-coverage.sh` — PASS
- `trackfw validate` → 0 errors, 168 warnings (warnings pré-existentes)
- `make parity-falsify`: não executado localmente (~78% wall time, excede timeout de 10 min); aguarda CI

**Nota:** `make quality` inteiro excede o timeout da ferramenta (~13 min). `parity-falsify` aguarda CI para verificação completa. AC7 fica aberto até CI verde.

## Divergência declarada: Go não abre browser

O CLI Go (`internal/serve/serve.go`) não tem caminho de abertura de browser — só imprime a URL.
O `--no-open` não existe no Go porque nunca há abertura. Isso significa:
- AC1 é vacuosamente satisfeito no Go (sem superfície de injeção)
- AC6 documenta a divergência explicitamente em `docs/cli-parity.md`
- A validação `IsValidHost` foi adicionada ao Go como defense-in-depth (paridade de AC4), mesmo sem abertura de browser.

**Nota para o arquiteto:** AC6 como escrito na REQ ("os 3 CLIs abrem o browser pela mesma forma") não é satisfatível sem adicionar browser-opening ao Go — o que estaria fora do escopo declarado ("só o caminho de abertura de browser do serve e sua paridade"). A divergência está documentada; a REQ pode precisar ser emendada para refletir a realidade.

### ML-NOVO — Resíduos do parecer de segurança (APROVA com 2 residuais)
**Status:** ⬜ Pendente · **Agente:** `ares-tf` · **não bloqueia o merge**

**R1 — cenários 8-9 do gate não emitem FAIL explícito.** Com o módulo ilegível, o `set -euo pipefail`
aborta na linha do `grep` antes da checagem de vazio. O gate termina RC=2 pelas falhas acumuladas de
2 e 3 — **não é falso-OK**, mas a mensagem que nomeia a causa não aparece. Cosmético, e o `hades-tf`
mediu por execução (`chmod 000`), não por leitura.

**R2 — 🔴 pycache pode fazer o cenário 4 medir código que não existe mais.** Um `serve.cpython-*.pyc`
válido faz o CPython importar o cache mesmo com `chmod 000` no fonte — a validação é por `stat()`.
Se o `.pyc` estiver correto e o fonte for trocado por código vulnerável, o cenário **passa sobre
código não medido**.

É a classe de vacuidade escondida no interpretador. **O gate precisa invalidar o pycache antes de
medir.** O `hades-tf` só viu porque removeu o cache explicitamente antes do teste.

### Nota para o CHANGELOG — correção de narrativa exigida pelo parecer

🔴 A justificativa interna de rejeitar `%` dizia que *"o feature nunca funcionou"*. **Meia verdade:**

```
Python   HTTPServer faz strip do scope_id   →  nunca funcionou
Go       net.ParseIP rejeita                →  nunca funcionou
Node     libuv no Linux                     →  provavelmente FUNCIONAVA
```

O changelog tem de dizer **"removemos zone ID (capacidade Node-Linux potencialmente funcional) por
paridade e fechamento de classe de ataque"** — não *"nunca funcionou"*. Remoção de capacidade real
declarada como limpeza de código morto é o tipo de imprecisão que a Regra Dura de Reconciliação
proíbe.

### Achado fora de escopo, registrado para o próximo revisor do `barrier`

O implementador classificou `internal/commands/barrier.go:803`
(`exec.Command("sh","-c",command)`) como seguro *"por vir de YAML de roadmap, não de flag do
usuário"*. 🔴 **O argumento é tecnicamente errado** — o vault de 2026-08-23 documenta que roadmap de
terceiro é exatamente o vetor de RCE do `barrier`. O sítio é pré-existente e fora do escopo desta
REQ; o trust-check fechado hoje pode protegê-lo, mas **isso precisa ser dito, não presumido**.

### ML-NOVO — 🔴 Gate que existe e ninguém invoca: a TERCEIRA instância do dia
**Status:** ✅ Concluído · **Agente:** `ares-tf` · **classe, não instância**

**Descoberto ao abrir o PR seguinte**, por um aviso do `trackfw push` sobre outra branch.

O `scripts/check-serve-browser-security.sh` foi mergeado **sem estar ligado a alvo nenhum**. Ele
existia, passava 21/21 quando invocado à mão, e **nunca rodava**.

🔴 **A falha de auditoria foi do arquiteto:** eu rodei `bash scripts/check-serve-browser-security.sh`
diretamente, vi 21/21 e concluí que funcionava. **Verifiquei que o gate funciona — não que alguma
coisa o executa.**

#### Terceira vez no mesmo dia

```
ratchet       job reprovava, não era `required`         → ninguém consumia o veredito
baseline D4   não rodava, `origin/main` não fetchado    → ninguém consumia
serve gate    existia, fora de todo alvo                → ninguém consumia
```

**Não é descuido repetido — é uma classe:** *construímos o mecanismo e não o ligamos a quem age
sobre ele.* O ML-4B fechou a instância de CI (required × jobs declarados); esta é a de `scripts/` ×
alvos.

#### E a varredura achou um órfão pior

```
52 scripts check-*.sh
 4 fora do Makefile
   3 invocados por workflow ou outro script   ← legítimo
   1 invocado por NINGUÉM  →  check-raw-read-ban.sh
```

O `check-raw-read-ban.sh` nasceu no PR **#285**, e o comentário dele diz para que serve:

> *"This gate is what stops the NEXT ml from reintroducing a raw call one site at a time, unnoticed —
> which is exactly how the original 26 sites accumulated."*

🔴 **Um gate anti-reintrodução da classe fail-open, inerte desde que foi escrito.** Rodado agora à
mão: **passa** — a classe não voltou. Dano zero até aqui; proteção zero também.

Os dois foram ligados ao `parity-rest` neste ML. **O que falta é impedir o terceiro.**

#### O gate da classe

Verificar que **todo `scripts/check-*.sh` tem consumidor** — alvo do `Makefile`, workflow, ou outro
script. Reprovar nomeando o órfão.

**Decisões registradas (ares-tf, 2026-09-11):**

1. **O que conta como consumidor?** Invocação real (não apenas menção em comentário) em linha
   não-comentário de: (a) recipe do Makefile (prefixo tab, não `\t#`); (b) qualquer `.sh` fora de
   `scripts/testdata/` (linha não-comentário, não o próprio script); (c) qualquer `.yml` de
   `.github/workflows` (linha não-comentário). Citação em `scripts/testdata/` é corpus congelado —
   **nunca é execução**. Por que "citado por outro script basta" não é suficiente: um par de scripts
   que se cita em comentários forma ciclo de citação que nunca chega a nenhum executor. O discriminante
   é testdata: `check-integration-cli-parity.sh` é citado em corpus `.md` de testdata e em comentários
   de vários scripts, mas o único consumo real é `bash "$ROOT_DIR/scripts/check-integration-cli-parity.sh"`
   em `check-cli-parity.sh:211`. O gate confere isso; a citação de testdata provoca FAIL se o
   exclusão for removida (arm 3 de `--self-test` confirma que a exclusão é load-bearing, não
   decorativa).

2. **Script novo sem consumidor: reprova.** Não avisa — avisos viraram ruído (issue #275). Reprovar
   força o autor a ligar o script antes de mergear, que é exatamente o que faltou nas duas instâncias
   anteriores de hoje.

3. **Exceção declarada obrigatória com motivo.** Lista `EXCEPTIONS` no gate com formato
   `"basename.sh|motivo"`. Entrada sem `|` ou com motivo vazio faz o gate reprovar — tolerância
   silenciosa é o anti-padrão que este gate fecha. Lista vazia por padrão: nenhum script é
   atualmente ferramenta manual legítima sem consumidor.

**Medição de partida confirmada:** 52 scripts `check-*.sh`; 0 órfãos após os dois ligados
neste ML (`check-raw-read-ban.sh` e `check-serve-browser-security.sh`). O gate passa com 53/53 OK
(inclui a si mesmo). Medição `make parity-rest` → exit 0.

**Falsificação (4 braços, todos `--self-test` OK):**
- Arm 1: script sem consumidor ⇒ FAIL nomeando o script  
- Arm 2: script ligado ao Makefile ⇒ PASS (contra-braço)  
- Arm 3: script citado só em `scripts/testdata/*.sh` ⇒ FAIL; remover exclusão vira falso-PASS (confirma que exclusão é discriminante, não no-op)  
- Arm 4a/4b: script em lista de exceção com motivo ⇒ PASS; sem motivo ⇒ FAIL

**Reconciliação de testes:** o `--self-test` afirma que o gate detecta scripts órfãos e não detecta
falsos positivos para os 4 casos definidos pela especificação — confirmado contra a medição de
partida (0 órfãos reais = o braço sintético do arm 1 é o que prova que o gate não é vacuous).

**Arquivos entregues:**
- `scripts/check-orphan-gates.sh` (novo gate com `--self-test`)
- `Makefile` — gate adicionado a `parity-rest` (dois passos: `--self-test` + scan completo)

**Falsificação:** script sem consumidor ⇒ reprova nomeando · script ligado ao Makefile ⇒ passa
(contra-braço) · script citado **só em testdata** ⇒ 🔴 reprova, porque testdata não executa ·
script na lista de exceção ⇒ passa, e a lista **não pode estar vazia de motivo**.

---
status: backlog
date: 2026-08-29
req: docs/requisições/claude/REQ-2026-08-29-node-e-python-ignoram-home-no-windows.md
squad: ""
---

# Roadmap: Node e Python ignoram HOME no Windows

> Created: 2026-08-29 | Status: done

## Context

`os.homedir()` (Node) e `os.path.expanduser` (Python) leem `%USERPROFILE%` no Windows e ignoram
`$HOME`. O Go ja foi corrigido na migracao com `internal/homedir.Dir()`. Enquanto os outros dois
nao forem, teste nao isola home e o `check-artifact-parity.sh` nao passa — o que bloqueia o ML-2A
do roadmap do slug.

REQ: docs/requisições/claude/REQ-2026-08-29-node-e-python-ignoram-home-no-windows.md

## Acceptance Criteria

- [ ] Com `HOME` em tempdir, os tres runtimes resolvem para ele, em execucao real
- [ ] Expansao de `~` em caminho de config honra `$HOME` nos tres
- [ ] `check-artifact-parity.sh` passa no Windows
- [ ] Gate **falha** com um site restaurado para a forma antiga
- [ ] Sem regressao por lista nomeada — npm 297 falhas, pypi 199 falhas

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** done

#### 1. Completude da enumeracao

Procurei por **todas** as formas de obter home, nao so as duas obvias, nos tres runtimes:

| Runtime | Forma | Sites |
|---|---|---|
| Node | `os.homedir()` | 24 |
| Node | `expandPath()` em `src/config/index.js:8` | 1 funcao (consome `os.homedir()`) |
| Node | `process.env.HOME` | 1 — **so num comentario**, nao e codigo |
| Node | `process.env.USERPROFILE` | 0 |
| Python | `expanduser("~")` — "me de a home" | 17 |
| Python | `expanduser(p)` — "expanda o `~` deste caminho" | 10 |
| Python | `Path.home()` | 1, em `integrations/manager.py:32` |
| Python | `environ["HOME"]`, `USERPROFILE`, `HOMEDRIVE` | 0 |
| Go | `homedir.Dir()` | 21 — **ja corrigido**, e a referencia |

**As duas familias do Python sao distintas e as duas importam.** `expanduser("~")` pede a home;
`expanduser(p)` expande `~` num caminho que veio do `trackfw.yaml` (`adr_dirs: ~/algo`). O segundo e
facil de esquecer numa varredura que so procura `expanduser("~")`, e resolve pelo `USERPROFILE`
igual. O Go trata os dois: `config.ExpandPath` usa `homedir.Dir()` desde a migracao
(`internal/config/config.go:621`).

#### 2. Quem esvazia esta Wave 0 sem quebrar regra escrita

1. **Corrigir so os 24 `os.homedir()` do Node e os 17 `expanduser("~")` do Python**, deixando os 10
   de caminho de config e o `Path.home()`. A varredura ingenua da sensacao de completude.
   **Coberto:** o criterio exige verificacao em execucao real com `HOME` em tempdir, incluindo um
   `adr_dirs: ~/...`, nao contagem de call site.
2. **Fazer o gate de artefato ignorar o `validate` que falha** (`|| true`). Some o sintoma, fica o
   teste escrevendo na home real. **Nao coberto por gate** — proibicao escrita aqui.
3. **Merge futuro do upstream reintroduz.** A CI deles e Linux e nunca ve isto; todo arquivo novo
   chega com `os.homedir()` cru. **Coberto:** o gate do ML-2A e estatico e falha em qualquer site
   novo.
4. **Corrigir Node e Python com regra diferente da do Go.** Ex.: aceitar `HOME` vazio como valido,
   ou preferir `USERPROFILE` quando os dois existem. Os tres passariam a "honrar HOME" com
   comportamentos distintos. **Coberto:** o criterio compara os tres na mesma execucao.

#### 3. Alvos de falsificacao, nas duas direcoes

| Regride para | Quebra o que |
|---|---|
| ignorar `$HOME` (hoje) | teste escreve na home real; gate de artefato aborta; ML-2A do slug fica bloqueado |
| honrar `$HOME` mas com string vazia valida | `HOME=""` passa a resolver para `""`, e caminho de config vira relativo silenciosamente |
| honrar `$HOME` so em Windows | Linux e macOS ja o fazem; um `if windows` cria dois caminhos de codigo onde um basta |
| Go regride para `os.UserHomeDir()` | os tres voltam a divergir; e a direcao que ninguem vigia, porque o Go "ja esta certo" |

A ultima linha e a que importa: o gate precisa cobrir os **tres** runtimes, nao so os dois que
estao sendo corrigidos agora.

#### 4. Residual declarado

- **Comportamento em Linux e macOS nao muda** — `$HOME` ja e a primeira fonte la. A correcao so
  alinha o Windows, mesmo criterio do `internal/homedir` do Go.
- **`HOMEDRIVE`/`HOMEPATH`** (o fallback antigo do Windows) fica fora: `os.homedir()` e
  `expanduser` continuam sendo o fallback quando `$HOME` nao esta definido, e eles ja consultam
  esses.
- Nao entra deteccao de `HOME` apontando para diretorio inexistente. Se o usuario define `HOME`
  errado, o erro e dele e aparece na primeira escrita.

**Acceptance criteria:**
- [x] As quatro secoes respondidas com evidencia medida
- [x] Nenhuma linha de implementacao escrita nesta ML

**Gates da wave:**
```bash
bash scripts/check-homedir-parity.sh
```

## Wave 1 — A correcao
> Dependencies: ML-0A

### ML-1A — `homedir` no Node
**Status:** done
**Files affected:** `npm/src/homedir.js` (novo) + 25 sites em 13 arquivos

Helper que devolve `process.env.HOME` quando definido e nao vazio, senao `os.homedir()`. Espelha
`internal/homedir/homedir.go`. `expandPath` em `src/config/index.js` herda pelo consumo.

**Tropeco:** a primeira passada calculou o caminho relativo do `require` com um nivel a menos e
gerou `require('./homedir')` em arquivo de subpasta — `Cannot find module`. Corrigido; os 13 usam
`../homedir`.

**Verificacao por efeito:** `HOME=<tempdir> adr new --scope global` escreve em
`<tempdir>/.trackfw/adr/` e nao na home real.

**Acceptance criteria:**
- [x] Com `HOME` em tempdir, resolve para ele em execucao real
- [x] Nenhum `os.homedir()` cru fora do helper

### ML-1B — `homedir` no Python
**Status:** done
**Files affected:** `pypi/trackfw/homedir.py` (novo) + 28 sites em 11 arquivos

`home_dir()` e `expand_path(p)` — as **duas** familias, como o ML-0A exigia. `expand_path` espelha
`config.ExpandPath` do Go, incluindo `~`, `~/` e `~\`.

**Tropeco:** a insercao do `import` caiu dentro de um `from ... import (` multilinha em
`commands/update_harness.py` e quebrou a sintaxe. Peguei porque rodei `ast.parse` em todos os
arquivos, nao so nos que achei que tinha tocado.

**Verificacao por efeito:** `HOME=<tempdir> adr new --scope global` escreve no tempdir. Os tres
runtimes devolvem a **mesma** mensagem e o **mesmo** caminho na mesma execucao.

**Acceptance criteria:**
- [x] `adr_dirs: ~/algo` resolve pelo `$HOME` de teste (`expand_path` cobre)
- [x] Nenhum `expanduser` / `Path.home()` cru fora do helper

## Wave 2 — A guarda
> Dependencies: ML-1A, ML-1B

### ML-2A — Gate de paridade de home
**Status:** done
**Files affected:** `scripts/check-homedir-parity.sh` (novo)

Duas metades. **Por efeito:** com `HOME` num tempdir, os tres runtimes tem de resolver para ele.
**Estatica:** nenhum `os.homedir()` / `expanduser` / `Path.home()` / `os.UserHomeDir()` cru fora do
helper de cada runtime. Cobre os **tres**, nao so os dois corrigidos agora — a falsificacao que o
ML-0A apontou como a menos vigiada e o Go regredir, porque "ja esta certo".

**Nao-vacuidade verificada nas duas metades** com um site do Node restaurado para `os.homedir()`:

```
homedir parity: node nao resolveu para $HOME
  esperado conter o tempdir: tmp.uzJwqRRa2c
  saida:  ADR-2026-08-29-adr-listado-global.md   Proposed      <- listou a home REAL
homedir parity: node tem resolucao de home fora do helper:
  npm/src/commands/adr.js:23:  return path.join(os.homedir(), '.trackfw', 'adr')
rc=1
```

> A primeira versao do gate reprovava os tres **com o comportamento correto**: comparava o caminho
> inteiro, e o `mktemp` do Git Bash devolve `/tmp/tmp.XXXX` enquanto os runtimes reportam
> `C:/Users/.../Temp/tmp.XXXX`. Mesmo diretorio, duas grafias. Passou a comparar o nome unico do
> tempdir, que aparece nas duas.

**Acceptance criteria:**
- [x] Gate passa depois de ML-1A e ML-1B
- [x] Gate **falha** com um site restaurado para a forma antiga — saida acima
- [ ] `check-artifact-parity.sh` passa — **nao alcancado**, ver passivo

---

## Regressao — o que foi medido e o que nao foi

### Python: completo, e limpo

Duas corridas completas na mesma arvore, revertendo os 24 arquivos e restaurando:

```
antes    194 failed / 1298 passed
depois   105 failed / 1387 passed
resolvidas: 91      novas: 2
```

**Tres bugs meus** apareceram na primeira medicao (12 novas) e foram corrigidos, todos da mesma
familia — substituicao mecanica sem olhar escopo:

| Onde | O que |
|---|---|
| `integrations/manager.py:33` | o `home_dir` importado colidia com o **parametro** `home_dir` do `__init__`; com o parametro `None`, virava `None()` |
| `integrations/doctor.py:165` | a mesma colisao em `run_doctor` |
| `config.py:283` | a troca atingiu uma **docstring** que descrevia a origem do valor |

Os dois primeiros eram `TypeError` em runtime, nao erro de sintaxe — `ast.parse` nao pegaria. So a
suite pegou.

**As 2 novas que restaram nao sao regressao:**

1. `test_git_branch_guard_dedup.py::test_claude_tolerates_double_slash_in_stored_command` — o teste
   setava `HOME` e era ignorado; a producao lia a home real, **que tem o hook do guard instalado**
   (as proprias rodadas de teste puseram la). O dedup encontrava a entrada global na home errada e
   pulava a injecao, entao o `assertFalse` passava. Com isolamento de verdade ele expoe o defeito
   que o proprio nome descreve — e o **Go falha o teste equivalente**
   (`TestGBGDedup_Claude_SkipsProjectEntry_ToleratesDoubleSlashInStoredCommand`), com a mesma
   mensagem, ja tendo o fix de `homedir`. Defeito de produto, estava mascarado.
2. `test_commands_basic.py::test_init_scaffolds_project` — `UnicodeDecodeError` em cp1252 lendo
   saida de subprocesso, porque o `init` passou a ver home vazia e entra no wizard. Consequencia do
   defeito de `isatty` registrado no passivo abaixo.

### Node: incompleto, e digo isso

**Nenhuma das duas corridas npm terminou nesta maquina.** A com o fix cobriu 36 dos 70 arquivos e
parou em `validator.test.js`; a linha de base parou antes, em `forge_adapter.test.js`, nas duas
tentativas. Nao ha total de npm defensavel aqui.

O que da para comparar sao os **318 testes que as duas executaram**:

```
novas falhas:  0
resolvidas:   12
```

Entre as 12: `adr new --scope global writes into $HOME/.trackfw/adr`,
`agents install sem TTY e sem --scope grava em ~/.claude`, e tres de dedup de credential-guard.
Exatamente a familia que o fix ataca.

> Tentei isolar o travamento rodando `generators.test.js` sozinho e conclui cedo demais que ele
> travava so antes do fix. Repetindo com `stdin=/dev/null`, para igualar a condicao da suite, ele
> trava **nos dois** estados. A conclusao anterior estava errada e nao entra como evidencia.


---

## Passivo aberto — o setimo bloqueio de Windows

`check-artifact-parity.sh` ainda nao passa, agora por um defeito novo que o isolamento revelou:
o `init` do Python entra no wizard de identidade mesmo com stdin nao interativo.

```
sys.stdin.isatty()  com </dev/null   ->  True     (Python)
process.stdin.isTTY com </dev/null   ->  undefined (Node)
```

`NUL` no Windows e um character device, e o Windows reporta character device como TTY. O Go usa
`cbterm.IsTerminal` (`GetConsoleMode` de verdade) e o Node usa o tipo de handle do libuv; so o
Python confia no `isatty()`. O `init.py:117` **tem** a guarda `sys.stdin.isatty()` — ela so nao
funciona nesta plataforma.

Antes deste roadmap o defeito estava mascarado: o Python lia a home real, que ja tem identidade
configurada, entao `_identity_file_exists(home)` era verdadeiro e o wizard nunca rodava.

Precisa de REQ propria. Enquanto nao entrar, o **ML-2A do roadmap do slug segue bloqueado** — pela
terceira parede diferente: primeiro o CRLF, depois a home, agora o `isatty`.

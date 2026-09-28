---
status: backlog
date: 2026-08-29
req: docs/requisições/claude/REQ-2026-08-29-isatty-do-python-devolve-true-para-nul-no-windows.md
squad: ""
---

# Roadmap: isatty do Python devolve True para NUL no Windows

> Created: 2026-08-29 | Status: done

## Context

`sys.stdin.isatty()` devolve `True` para `NUL` no Windows, entao `trackfw init` do Python entra no
wizard de identidade em contexto nao interativo e morre com EOF. Go e Node usam mecanismos
confiaveis. Bloqueia o `check-artifact-parity.sh` e, por tabela, o ML-2A do roadmap do slug.

REQ: docs/requisições/claude/REQ-2026-08-29-isatty-do-python-devolve-true-para-nul-no-windows.md

## Acceptance Criteria

- [ ] `init` do Python conclui com stdin nao interativo, em execucao real
- [ ] Casa com o Go por construcao — mesmo `GetConsoleMode`
- [ ] `sys.stdout.isatty()` de `validate.py` recebe o mesmo tratamento
- [ ] POSIX inalterado
- [ ] Gate **falha** com um site restaurado
- [ ] `check-artifact-parity.sh` passa
- [ ] Sem regressao por lista nomeada contra 105 falhas

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** done

#### 1. Completude da enumeracao

| Runtime | Forma | Sites | Confiavel no Windows |
|---|---|---|---|
| Python | `sys.stdin.isatty()` | 7 | **nao** |
| Python | `sys.stdout.isatty()` — cor, `validate.py:24` | 1 | **nao** |
| Node | `process.stdin.isTTY` | 8 | sim (tipo de handle do libuv) |
| Go | `cbterm.IsTerminal` | 5 | sim (`GetConsoleMode`) |

Procurei tambem por `os.isatty`, `TERM`, `CI`, `NO_COLOR` e por deteccao propria de interatividade:
nao ha outra forma no Python. As duas familias sao **stdin** (promptar ou nao) e **stdout** (emitir
cor ou nao) — a segunda e a que some numa varredura que so pensa em wizard.

**Verificacao do mecanismo, antes de prometer:**

```
                     stdin=/dev/null        stdout=arquivo
sys.std*.isatty()    True   <- defeito      False
GetConsoleMode()     False  <- correto      False
```

E o `isTerminal` do Go no Windows e exatamente
`windows.GetConsoleMode(windows.Handle(fd), &st) == nil`
(`charmbracelet/x/term@v0.2.2/term_windows.go:16`). Mesmo syscall.

#### 2. Quem esvazia esta Wave 0 sem quebrar regra escrita

1. **Corrigir so o stdin e deixar o stdout.** O sintoma visivel e o wizard; a cor em arquivo
   redirecionado nao incomoda ninguem hoje. **Coberto:** criterio nomeia `validate.py:24`.
2. **Trocar `isatty()` por `GetConsoleMode` em todo lugar**, inclusive POSIX, onde a funcao nao
   existe — `AttributeError` no Linux. **Coberto:** criterio exige POSIX inalterado; o
   `GetConsoleMode` so estreita o resultado do `isatty()`, nunca o substitui.
3. **Over-correct: devolver sempre False no Windows.** O wizard nunca roda, a identidade fica no
   default neutro em silencio, e o gate fica verde. **Este e o risco real e nao esta coberto por
   gate** — ver residual.
4. **Merge futuro do upstream reintroduz `isatty()` cru.** A CI deles e Linux e nunca ve isto.
   **Coberto:** gate estatico.

#### 3. Alvos de falsificacao, nas duas direcoes

| Regride para | Quebra o que |
|---|---|
| `isatty()` cru (hoje) | `init` prompta em CI e em gate; morre com EOF |
| sempre `False` no Windows | wizard nunca roda; identidade vira default neutro **em silencio** — pior que o erro, porque nao aparece |
| `GetConsoleMode` tambem em POSIX | `AttributeError` no Linux e macOS |
| checar so stdin | cor ANSI escrita para dentro de arquivo redirecionado |

A segunda linha e a que importa: o modo de falha do over-correct e **silencioso**, e um gate que so
verifica "nao prompta" passaria feliz.

#### 4. Residual declarado

**Nao consigo verificar o caso positivo nesta maquina.** Esta sessao nao tem console anexado: ate a
execucao "interativa" mede `isatty()=False`. Provo que o falso positivo some; nao provo que um
terminal de verdade continua promptando.

Mitigacao: casar com o Go por construcao, usando o mesmo `GetConsoleMode`. O que um console real
fizer para o Go, fara para o Python. **Fica como pendencia explicita: um teste manual num terminal
de verdade.** Registrado aqui para nao virar uma suposicao esquecida.

Fora de escopo: Node e Go, que ja usam mecanismo confiavel.

**Acceptance criteria:**
- [x] As quatro secoes respondidas com evidencia medida
- [x] Nenhuma linha de implementacao escrita nesta ML

**Gates da wave:**
```bash
bash scripts/check-tty-detection.sh
```

## Wave 1 — A correcao
> Dependencies: ML-0A

### ML-1A — Helper de deteccao de TTY no Python
**Status:** done
**Files affected:** `pypi/trackfw/tty.py` (novo) + 8 sites em 4 arquivos

`stdin_is_interactive()` e `stdout_is_interactive()`. O `isatty()` continua sendo a base e, **so no
Windows**, o resultado e estreitado com `GetConsoleMode` via `msvcrt.get_osfhandle` + `ctypes` — o
**mesmo syscall** que o `charmbracelet/x/term` do Go usa. Casa com o Go por construcao, nao por
heuristica paralela. POSIX inalterado: o ramo Windows so roda em `sys.platform == "win32"`.

**Verificacao:** `init` com `HOME` em tempdir e `stdin=/dev/null` conclui, com a mesma saida final
do Node. Antes morria com `EOF when reading a line`.

**Dois testes precisaram do seam novo.** `test_scope_resolution.py` fazia
`monkeypatch.setattr("sys.stdin.isatty", lambda: True)` — fingir TTY sobre um fd que nao e console
nao basta mais. Passaram a injetar `integrations_command.stdin_is_interactive`.

Vale registrar: **essa foi a materializacao do risco que o ML-0A secao 2 item 3 chamou de
over-correct.** Ele previu o modo silencioso; aqui apareceu como teste vermelho, que e o desfecho
bom. E a causa nao era o helper estar errado — num console real o `GetConsoleMode` teria sucesso.

**Acceptance criteria:**
- [x] `init` conclui com stdin nao interativo
- [x] POSIX inalterado
- [x] Suite pypi sem regressao (ver medicao abaixo)

### ML-1B — Separador de caminho no `.trackfw-log`
**Status:** done
**Files affected:** `pypi/trackfw/generators/roadmap.py:609`

**Defeito distinto, corrigido aqui por proporcionalidade.** Depois do ML-1A o
`check-artifact-parity.sh` avancou e parou noutro ponto: o log de transicao em modo `by_agent`.

```
go    zeus/ROADMAP-cycle-analyzing.md   backlog → analyzing
node  zeus/ROADMAP-cycle-analyzing.md   backlog → analyzing
py    zeus\ROADMAP-cycle-analyzing.md   backlog → analyzing
```

`os.path.join(agent, basename)` grava o separador do sistema. O nome no `.trackfw-log` e artefato
portavel, nao caminho de sistema — Go e Node gravam `/` em qualquer plataforma. Em Linux
coincidiria.

E **uma linha**. Mandar para REQ propria seria cerimonia; o CRLF, com 38 sites e alcance sobre todo
arquivo gravado, foi para REQ propria com razao. A regra que apliquei e proporcionalidade, e ela
fica escrita aqui para nao virar precedente solto.

**Acceptance criteria:**
- [x] Os tres gravam `agente/ARQUIVO.md`

## Wave 2 — A guarda
> Dependencies: ML-1A

### ML-2A — Gate de deteccao de TTY
**Status:** done
**Files affected:** `scripts/check-tty-detection.sh` (novo)

Duas metades. **Por efeito:** `init` com `stdin=/dev/null` **e home isolada** conclui nos tres
runtimes — a home isolada importa, porque com identidade ja configurada o wizard nem seria
alcancado e o gate passaria sem exercitar nada. **Estatica:** nenhum `isatty()` cru fora do helper.

**Nao-vacuidade verificada**, com o site do `init.py` restaurado:

```
tty detection: `python init` saiu 1 com stdin nao interativo (esperado 0)
tty detection: python tem isatty() fora do helper:
  pypi/trackfw/commands/init.py:118:    if not skip_identity_wizard and sys.stdin.isatty():
rc=1
```

> A primeira versao do gate **falhava sem imprimir diagnostico**: com `set -euo pipefail` o subshell
> abortava antes do `rc=$?`. Um gate que reprova sem dizer por que e quase tao ruim quanto um que
> nao reprova. Corrigido com `|| rc=$?`.

**Acceptance criteria:**
- [x] Gate passa depois de ML-1A
- [x] Gate **falha** com um site restaurado — saida acima
- [x] `check-artifact-parity.sh` passa

---

## `check-artifact-parity.sh` passa pela primeira vez

```
Artifact parity checks passed (8 artifact types x 3 runtimes; roadmap flags,
quoted status, analyzing cycle flat/by_agent; CLAUDE.md ## Architect responses)
rc=0
```

Ele estava bloqueado por **quatro** coisas em sequencia, cada uma escondendo a proxima: o CRLF dos
geradores Python, o isolamento de home em Node e Python, o `isatty`, e o separador do log. Com ele
verde, o **ML-2A do roadmap do slug esta desbloqueado** — era o proposito da cadeia inteira.

## Regressao — pypi, lista nomeada

```
antes (medicao do homedir)   105 failed / 1387 passed
depois                        95 failed / 1397 passed
resolvidas: 11    novas: 1
```

A unica "nova" e `test_stale_wip_warning_arquivo_antigo`, o instavel de skew de relogio ja
caracterizado — `time.time()` no teste contra `datetime.now().timestamp()` na producao.

## Pendencia herdada do ML-0A

**O caso positivo continua sem verificacao nesta maquina**: nao ha console anexado, entao nao provo
que um terminal de verdade continua promptando. A mitigacao e o mesmo syscall do Go. Um teste manual
num terminal real fecha o buraco e segue em aberto.

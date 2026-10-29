---
status: Open
date: 2026-09-12
author: "Hefesto (Code Quality)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.md"
---

# REQ: suites de teste nao distinguem ambiente incompleto de codigo quebrado

> Date: 2026-09-12 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

O defeito é o **sinal apontar para o lugar errado**, não "alguém esqueceu de instalar".

Quando o ambiente está incompleto (dependência ausente, ferramenta externa faltando), as suítes de
teste produzem o mesmo sinal que um defeito de produto. O desenvolvedor ou agente que recebe o vermelho
não sabe se deve corrigir código ou instalar uma dependência.

O defeito foi medido duas vezes no mesmo dia:

**Ocorrência 1 (manhã):** `scripts/check-serve-api-file-security.sh` acusava `FAIL Python` quando a
causa era `pytest` ausente no job de CI. Corrigido com guarda de pré-requisito no gate:
```
ERRO: ambiente incompleto — pytest nao encontrado     rc=1, sem contabilizar FAIL de produto
```

**Ocorrência 2 (noite):** `npm test` num worktree sem `node_modules` instalados produziu a suíte
inteira vermelha — `adr.test.js`, `agents-*.test.js`, contratos do barrier — com:
```
Error: Cannot find module 'yaml'
✖ tests/adr.test.js (41.484334ms)
```
O erro é indistinguível de um CLI genuinamente quebrado. Após `npm ci`: 912 passando, zero falha.

**A forma da correção já existe no repositório** — o gate corrigido hoje
(`scripts/check-serve-api-file-security.sh` linhas 76–93) é o molde. Esta REQ é sobre
**generalizar essa guarda para as suítes** (`npm test`, `python3 -m pytest`, `go test ./...`).

## Medição realizada (2026-09-12)

### P1 — Suíte Node (`npm test`)

Sem `node_modules`: `node --test tests/*.test.js` emite `Cannot find module 'yaml'` e marca `✖`
para cada arquivo de teste. rc=1. Sem mensagem de diagnóstico estruturada. **Sem guarda.**

O `npm/package.json` define `"test": "node --test tests/*.test.js"` — nenhuma verificação de
pré-requisito antes de rodar.

### P2 — Suíte Python (`make test-python` = `python3 -m pytest pypi/tests -q`)

O Makefile não tem guarda de pré-requisito. Se `pytest` não estiver instalado: `python3: No module
named pytest` (rc=1), sem diagnóstico estruturado. Nenhum `conftest.py` verifica dependências antes
de rodar.

Nota: o `pypi/pyproject.toml` não declara `[tool.pytest.ini_options]`. O pytest resolve o `pypi/`
como rootdir e adiciona ao `sys.path` automaticamente, então testes que importam `trackfw.*` passam
sem `PYTHONPATH` explícito — mas isso não cobre a ausência do próprio `pytest`.

### P3 — `make test` e `make quality`

`make test` roda apenas `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 go test -timeout 2m ./...` — sem
Python ou Node. Não propaga o falso positivo, mas também não tem guarda própria para ferramentas
externas usadas nos testes Go (ver P4).

`make quality` executa `test test-node test-python lint parity` sequencialmente, sem guarda de
pré-requisito antes de cada runner. Uma dependência faltando em qualquer runtime propaga o vermelho
sem qualificar a causa.

**Nenhum lugar em `make test` ou `make quality` já faz isso certo.** O molde está exclusivamente em
`scripts/check-serve-api-file-security.sh`.

### P4 — Suíte Go (`go test ./...`) — o achado mais importante (sobrevive à v8)

A v8 (`REQ-2026-09-12-v8-um-binario-muitos-canais`) remove as reimplementações Node e Python. Após
a v8, o `go test ./...` será a única suíte de produto.

Medição: as funções abaixo em `internal/validator/validator_git_exec_test.go` chamam
`exec.Command("git", ...)` diretamente via helper `run()` sem verificar se `git` está disponível
no PATH:

- `TestCredentialGuardModeDowngrade_GitDirWorkTreeRedirecionados_ContinuaDetectando` (linha 70)
- `TestCredentialGuardModeDowngrade_GitConfigCountMalformado_NaoVacuidade` (linha 124)
- `TestIsGitWorktree_LinkedWorktreeLegitimo_ContinuaFuncionando` (linha 138)

Se `git` estiver ausente, `t.Fatalf("git %v: %s", args, out)` dispara — mesma saída que uma falha
de produto real.

Contraste: `internal/commands/branch_prune_test.go` (linhas 422, 604, 719, 877) e
`internal/commands/ship_test.go` (linha 1201) **já têm** a guarda correta:
```go
if _, err := exec.LookPath("git"); err != nil {
    t.Skip("git not available in PATH")
}
```
A suíte Go é **inconsistente**: alguns testes skipam, outros fatalf sem diagnóstico.

## Acceptance Criteria

Cada AC é enunciado com falsificação nas duas direções: ambiente incompleto deve produzir mensagem
própria e rc distinto; ambiente completo deve rodar normalmente.

### AC1 — `npm test` com `node_modules` ausentes

- **Positivo (ambiente incompleto):** `npm test` detecta ausência de dependências (ex.: `commander`
  ausente) e emite mensagem estruturada de pré-requisito com rc≠0 **antes** de rodar qualquer teste.
  Nenhum `✖` de teste é exibido.
- **Negativo (ambiente completo):** `npm test` após `npm ci` bem-sucedido roda normalmente e reporta
  apenas resultados de teste.

### AC2 — `python3 -m pytest pypi/tests` com `pytest` ausente

- **Positivo (ambiente incompleto):** o runner detecta ausência de `pytest` e emite mensagem de
  diagnóstico estruturada (análoga ao molde do gate) com rc=1, sem contabilizar FAIL de produto.
- **Negativo (ambiente completo):** `pytest` instalado → suíte roda e reporta apenas resultados.

### AC3 — `go test ./...` com `git` ausente — o AC mais valioso (sobrevive à v8)

- **Positivo (ambiente incompleto):** testes em `internal/validator/validator_git_exec_test.go` que
  chamam `exec.Command("git", ...)` detectam `git` ausente via `exec.LookPath` e skipam com
  `t.Skip("git not available in PATH")` — nenhum `t.Fatalf` por ferramenta ausente.
- **Negativo (ambiente completo):** `git` presente → esses mesmos testes rodam e produzem resultado
  real de produto (pass ou fail por defeito real).

### AC4 — `make quality` qualifica a causa antes de propagar rc≠0

- **Positivo (dependência faltando):** `make quality` interrompe com mensagem de diagnóstico
  identificando qual pré-requisito está ausente, antes de propagar um rc≠0 genérico.
- **Negativo (ambiente completo):** `make quality` roda normalmente sem overhead de diagnóstico.

## Ressalva explícita — dependência da v8

Esta REQ toca `npm/tests/`, `pypi/tests/` e `internal/` — superfícies que se relacionam com a
`REQ-2026-09-12-v8-um-binario-muitos-canais` de formas distintas:

**Se entrar antes da v8:** resolve um problema real que já mordeu duas vezes no mesmo dia. AC1 e
AC2 têm valor imediato pleno.

**Se entrar depois da v8:** AC1 e AC2 migram para o que sobrar — a suíte de smoke da casquinha npm
e o runner python do wheel. O mecanismo de guarda é o mesmo; apenas a superfície muda.

**O que NÃO desaparece com a v8 é o AC3** — a suíte Go é a única após a v8 e é onde a
inconsistência de guarda já existe hoje. **AC3 é o mais valioso e o mais urgente independentemente
do cronograma da v8.**

A decisão sobre a ordem de entrada (antes/depois da v8) é do arquiteto e depende do cronograma.
Por isso o roadmap fica em `backlog/`.

## Revisão de escopo — 2026-10-29 (zeus-tf) — este é o escopo vigente

Retomada a pedido do KG, absorvendo o issue **#624** por **mesmo sintoma**: o resultado de
`go test` depende da máquina de quem roda. Regra Dura de Causa Raiz: mesmo sintoma investiga no
mesmo roadmap; só a medição autoriza separar.

### Medido em `main` `d0dba293` (v11.2.0), macOS, binário instalado 11.2.0

| comando | rc | `--- FAIL` |
|---|---|---|
| `env PATH=<sem trackfw> go test ./internal/commands/ -count=1` (HOME real) | 1 | 15 |
| idem, com `HOME=<tmp>` (GOCACHE/GOMODCACHE/GOPATH preservados) | 0 | 0 |
| `env PATH=<sem trackfw> go test ./... -count=1` (HOME real) | 1 | 15, todos em `internal/commands` |

Os 15: `TestValidateTextUnchanged`, `TestValidateJSONFlag`, `TestBarrierTrustLocalGatesFlag`,
`TestBarrierReentry_T3_LegitimateNested`, `TestBarrierReentry_T4_SequentialCallsPass`,
`TestBarrierLapsed_TextModePrintsLapsedLine`, `TestBarrierLapsed_PassedWhenMetAndLapsed`,
`TestBarrierLapsed_JustificationTruncatedAt120`, `TestBarrierLapsed_JustificationInTextAndJSON`,
`TestBarrierFence_WellFormedRoadmapPassesThrough`, `TestBarrierContract_WaveVerdePassa`,
`TestBarrierContract_SemHeadingMalformadaNaoIntroduceFalsoPositivo`,
`TestBarrierContract_JSONDeterministico`, `TestBarrierCLI_EnglishHeaderAndWordStatusPass`,
`TestBarrierCLI_CRLFGatesBlockFenceIsRecognized`.

**Causa (confirmada pelo contra-braço):** os testes leem o `~/.claude/settings.json` real; o guard
global está lá; a sonda de binário reprova sem `trackfw` no PATH. No CI não há settings global.

**Fatos de código que condicionam a correção:** `internal/homedir/homedir.go` lê `HOME` e cai em
`os.UserHomeDir()` (no Windows, `USERPROFILE`); o código de produto lê também `CLAUDE_CONFIG_DIR`;
o pacote `commands` **já tem** um `TestMain` em `internal/commands/netexec_test.go`.
**O molde já existe:** `internal/validator/main_test.go` isola `HOME`, `USERPROFILE` e remove
`CLAUDE_CONFIG_DIR`, com o teste-sentinela `TestIsolatedHomeCoversBothChannels`. O `commands` não
tem nada disso. Com `HOME` temporário e `CLAUDE_CONFIG_DIR=~/.claude` real, `go test ./...` deu rc=0
(o vazamento medido é pelo `HOME`; remover `CLAUDE_CONFIG_DIR` entra como defesa, igual ao molde).

### ACs vigentes

- **AC1 e AC2 — RETIRADOS.** As suítes Node e Python não existem desde a v8 (`npm/tests/` ausente,
  `pypi/tests/` só com `__pycache__`). Evidência: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.
- **AC3 — ampliado e com mecanismo trocado.** Medido em 2026-10-29 com PATH que tem todo `/usr/bin`
  menos `git` (e HOME temporário): **181 testes de topo reprovam (233 linhas `--- FAIL`) em 4
  pacotes** — `internal/auditsurface`, `internal/commands`, `internal/pathguard`,
  `internal/validator` — espalhados por ~20 arquivos. Não são 3.
  **Decisão (zeus-tf):** em vez de `t.Skip` teste a teste, **falha rápida no `TestMain`** de cada um
  dos 4 pacotes (todos já têm `TestMain`): com `git` ausente, o pacote imprime uma linha de
  pré-requisito e sai com rc≠0 sem rodar teste. Motivo: `t.Skip` em 181 testes deixaria a suíte
  **verde sem ter testado nada**, que é o sinal errado na direção oposta; o molde da REQ
  (`check-serve-api-file-security.sh`) também é rc≠0 com diagnóstico, não skip. `git` é
  pré-requisito duro de um CLI de governança de git.
  Positivo: com `git` ausente, `go test` desses 4 pacotes emite a linha
  `ambiente incompleto: git não encontrado no PATH` e **zero** `--- FAIL`. Negativo: com `git`
  presente, nenhuma linha de diagnóstico e a suíte roda inteira.
- **AC4 — remedido.** `make quality` (= `test lint parity`) qualifica pré-requisito ausente antes de
  rodar. A lista de ferramentas sai da medição do que `parity-rest`/`parity-falsify` invocam hoje
  (a lista antiga citava `node`/`npm`/`pytest` de suítes que não existem mais).
- **AC5 (#624) — novo.** Os testes não leem o ambiente pessoal do desenvolvedor.
  Positivo: com `PATH` sem `trackfw` e `HOME` **envenenado** (diretório com `.claude/settings.json`
  contendo os hooks do guard global), `go test ./...` passa; o mesmo com `CLAUDE_CONFIG_DIR`
  apontando para um diretório envenenado. Negativo: os testes que **precisam** de um HOME específico
  continuam definindo o próprio via `t.Setenv` e passam.

### Escopo negativo (não entra nesta REQ)

- Nenhuma mudança de comportamento do binário em `internal/` fora de arquivos `_test.go`, salvo se a
  medição provar que o código de produto ignora `HOME` ao ler o settings (aí volta ao arquiteto).
- Nenhum gate novo de CI e nenhum job novo de workflow (direção de 2026-10-28: B congelado).
- Nada de Node/Python; nada de `scripts/install.*`, site ou docs de usuário.
- Não altera a sonda de binário nem o guard global — o defeito é o teste vazar o ambiente, não a sonda.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.md

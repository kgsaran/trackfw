---
status: Open
date: 2026-08-29
author: claude
adr: "docs/adr/ADR-2026-09-05-windows-e-plataforma-de-primeira-classe-e-o-defeito-se-mede-nela-nao-se-contorna.md"
roadmap: "docs/roadmaps/claude/backlog/ROADMAP-2026-08-29-geradores-python-escrevem-crlf-no-windows.md"
---

# REQ: Geradores Python escrevem CRLF no Windows

> Date: 2026-08-29 | Status: Open

## Motivation

O CLI Python grava **todo** arquivo com CRLF no Windows; Go e Node gravam LF. Medido rodando
`init` e depois `adr|req|roadmap|note new` num diretorio limpo por runtime, e varrendo os bytes de
tudo que ficou:

```
py    CRLF=23 arquivos   LF=0
go    CRLF=0             LF=22
```

Nao ha excecao dos dois lados. Causa: `open(path, "w")` do Python usa `newline=None`, que traduz
`
` para `os.linesep`. Go e Node escrevem bytes direto. Invisivel para o upstream porque na CI
Linux o `os.linesep` ja e LF.

Isso viola a **Regra Dura de Paridade** do `CLAUDE.md`: os tres CLIs produzem artefato diferente
byte a byte para a mesma entrada.

### Nao e cosmetico — quebra script de shell

Entre os 23 arquivos estao os cinco `scripts/*.sh` que o `trackfw init` gera. O shebang sai assim:

```
py     b'#!/usr/bin/env bash
'
go     b'#!/usr/bin/env sh'
node   b'#!/usr/bin/env sh'
```

Um `.sh` com CR no shebang falha em qualquer sistema POSIX com `bad interpreter: bash^M`. Quem
roda `trackfw init` pelo CLI Python no Windows e commita o resultado entrega hooks de guard
quebrados para todo mundo que der checkout em Linux, macOS ou WSL.

### Segunda divergencia, independente, encontrada no mesmo arquivo

O Python escreve `#!/usr/bin/env bash`; Go e Node escrevem `#!/usr/bin/env sh`. Nao e fim de linha,
e interpretador diferente. Entra nesta REQ porque foi medida aqui e vive no mesmo gerador.

### Por que agora

O `scripts/check-artifact-parity.sh` compara os artefatos byte a byte e por isso acusa **8 drifts
`go vs python`** nesta maquina — `adr`, `note`, `note_index`, `req`, `roadmap`, `roadmap_flags`,
`roadmap_from_req`, `slash_roadmap`. Todos sao este defeito. Enquanto ele existir, aquele gate nao
passa no Windows e nao serve como guarda de nada, o que **bloqueia o ML-2A** de
`REQ-2026-08-29-slug-de-artefato-no-python-diverge-de-go-e-node`.

E o quinto bloqueio estrutural de Windows da 7.3.0, junto com o UTF-8 do CLI, o `homedir`, o
`credential_guard_hook_resolvable` e o `check-parity-contract-coverage.sh`.

## Acceptance Criteria

- [x] O CLI Python grava LF em todo arquivo que produz, no Windows, medido por varredura de bytes
      do resultado de `init` + os quatro `new` — nao por contagem de call site
      → **(a) ENTREGUE.** 2026-09-10: `trackfw init` do Python num diretório limpo produziu **20
      arquivos**, varridos byte a byte: **0 com CR**.
- [ ] `scripts/*.sh` gerados saem com o mesmo shebang nos tres runtimes
      → 🔴 **(b) NAO ENTREGUE.** Medido em 2026-09-10 comparando **por arquivo**, não por conjunto:
      ```
      trackfw-attention-cleanup.sh   go=bash  node=bash  py=bash   ok
      trackfw-attention-signal.sh    go=bash  node=bash  py=bash   ok
      trackfw-credential-guard.sh    go=bash  node=bash  py=bash   ok
      trackfw-git-branch-guard.sh    go=bash  node=bash  py=bash   ok
      trackfw-validate.sh            go=sh    node=sh    py=bash   DIVERGE
      ```
      Os nomes de arquivo batem nos três; o **shebang de `trackfw-validate.sh` não**. É violação da
      Regra Dura de Paridade, e é **produto do upstream** → vai como issue, não correção local.
- [x] `check-artifact-parity.sh` deixa de acusar os 8 drifts `go vs python`
      → **(a) ENTREGUE.** `bash scripts/check-artifact-parity.sh` → `exit 0`,
      `Artifact parity checks passed (9 artifact types × 3 runtimes)`.
- [ ] O gate **falha** com o CRLF reintroduzido — nao-vacuidade verificada, nao assumida
      → 🔴 **(b) NAO ENTREGUE — e o AC afirma exatamente o que não acontece.** Falsificado nas duas
      formas em 2026-09-10, mutando `pypi/trackfw/generators/adr.py`:
      ```
      M1  remover  newline=          -> check-python-writes-lf  exit=1   pega
      M2  trocar para newline="
"  -> check-python-writes-lf  exit=0   NAO PEGA
      ```
      O gate verifica se o argumento `newline` **está presente** (`if 'newline' in call: continue`),
      não o seu valor. **CRLF explícito passa.** São **79 sítios** com `newline="
"` em
      `pypi/trackfw`, e qualquer um deles virando `"
"` não seria acusado.
      🔴 A primeira tentativa de falsificação **não aplicou a mutação** (`0 aplicadas`) e o `exit 0`
      pareceu cegueira do gate. Só a segunda, construindo o alvo com `chr(92)`, mediu de verdade.
      O gate é **produto do upstream**, byte a byte idêntico ao dele → issue, não correção local.
- [ ] Nenhuma regressao na suite pypi contra a medicao de 2026-08-29 (198 failed / 1294 passed)
      → **(b) NAO ENTREGUE** — convertido de `(d)` em 2026-09-11 pela
      `REQ-2026-09-10-baselines-de-suite-nunca-foram-versionados` (AC5). A afirmação sobre agosto é
      **irrecuperável**: um baseline tirado hoje não prova "sem regressão desde agosto", só daqui
      para a frente — e daqui para a frente quem verifica é o ratchet do upstream, no nosso CI.
      Registro de 2026-09-10, que continua verdadeiro: O próprio AC exige comparação contra uma **lista nomeada**, e
      a lista de 2026-08-29 **não foi versionada** — a busca no acervo devolve apenas o
      `os-predicate-sites-baseline.txt`, de outra REQ e de 2026-09-09. Rodar a suite hoje daria um
      número, e comparar número com número é exatamente o que o AC proíbe.
      **O que faltaria:** a lista de falhas por nome daquela corrida. Ela não existe.

## Nao faz parte

A saida de terminal ja e tratada por `_force_utf8_output` em `pypi/trackfw/cli.py`, que passa
`newline="
"` para stdout e stderr. Esta REQ e sobre **escrita de arquivo**, que aquele fix nao
alcanca.

## Linked ADR

ADR: docs/adr/ADR-2026-09-05-windows-e-plataforma-de-primeira-classe-e-o-defeito-se-mede-nela-nao-se-contorna.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/claude/backlog/ROADMAP-2026-08-29-geradores-python-escrevem-crlf-no-windows.md

---
status: Done
date: 2026-09-02
author: "lourivalgarciajunior"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-02-gerador-de-fixture-le-stdin-em-binario.md"
---

# REQ: `write_fixture_crlf` corrompe não-ASCII por dupla codificação, e o CLI leva a culpa

> Date: 2026-09-02 | Status: Done

## Motivation

`write_fixture_crlf`, o gerador de fixture do `check-roadmap-barrier-contract.sh`, **corrompe todo
caractere não-ASCII no Windows**:

```python
data = sys.stdin.read()          # codificação do LOCALE — cp1252 no Windows
...
f.write(data.encode('utf-8'))    # grava como UTF-8 o que já virou N caracteres
```

Medido por execução:

```
encoding do stdin em texto : cp1252
bytes que ENTRARAM         : e2 ac 9c              (U+2B1C, o marcador de status)
bytes que SAÍRAM           : c3 a2 c2 ac c5 93     dupla codificação
code points após a leitura : 0xe2 (â) · 0xac (¬) · 0x153 (œ)
```

A fixture vai para o disco corrompida, **o CLI reporta fielmente o lixo**, e o defeito aparenta ser
do produto:

```json
"failures":["ML-1A: not complete (status: â¬œ Pendente)"]
```

**O CLI é inocente, verificado por eliminação.** Roadmap CRLF escrito corretamente em UTF-8, os três
runtimes acertam:

```
go    not complete (status: ⬜ Pendente)
node  not complete (status: ⬜ Pendente)
py    not complete (status: ⬜ Pendente)
```

E o transporte também está limpo: passar a string por `argv` e por `stdin` para um subprocesso
Python sob Git Bash preserva `U+2B1C` nos dois casos. Sobra o gerador de fixture.

**O check mais enganoso não é o que mostra mojibake:**

```
FAIL [crlf/full-roadmap-passes-cross-runtime]
     go(blocked) node(blocked) py(blocked)
```

Os três **concordam** — paridade perfeita sobre um insumo corrompido. É o tipo de resultado que
passa por "os três estão consistentes, então o comportamento está certo".

## Acceptance Criteria

- [ ] **AC1** — `write_fixture_crlf` lê stdin em **binário** e decodifica UTF-8 explicitamente.
- [ ] **AC2** — 🔴 **Falsificação nas duas direções, por igualdade de bytes contra a origem.**
      (a) com entrada **não-ASCII**, a fixture no disco bate byte a byte com a origem convertida a
      CRLF; (b) **controle** — com entrada **ASCII**, a fixture continua sendo gerada corretamente,
      ou seja, a correção não quebrou o caso que já funcionava.
- [ ] **AC3** — Os dois checks `crlf/` que dependem disto passam, e o gate inteiro fecha em
      `53 OK / 0 FAIL` sobre o corpus de 144 arquivos, em Windows real.
- [ ] **AC4** — O comentário no código registra **a medição** (os bytes que entram e saem), não só
      a instrução. Quem ler daqui a seis meses precisa saber por que `buffer.read()` e não `read()`.

## Negative Scope

**Não varre** os outros scripts de gate atrás do mesmo `sys.stdin.read()` sem encoding explícito. É
plausível que existam; não medi, e não afirmo.

**Não toca no CLI.** A eliminação acima mostra que ele já está correto; mexer nele seria corrigir o
que não está quebrado — o mesmo erro que a medição de junction evitou ao concluir que o Node estava
certo e os outros dois erravam juntos.

## Linked ADR
<!-- Correção de bug de harness, tratamento único; sem decisão de arquitetura a registrar. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-02-gerador-de-fixture-le-stdin-em-binario.md

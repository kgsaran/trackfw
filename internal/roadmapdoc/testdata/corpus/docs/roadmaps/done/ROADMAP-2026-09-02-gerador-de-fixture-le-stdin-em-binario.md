---
status: done
date: 2026-09-02
req: "docs/req/REQ-2026-09-02-write-fixture-crlf-corrompe-nao-ascii-por-dupla-codificacao-e-o-cli-leva-a-culpa.md"
squad: "lourivalgarciajunior"
---

# Roadmap: Gerador de fixture lê stdin em binário

> Created: 2026-09-02 | Status: done

## Context

REQ: `docs/req/REQ-2026-09-02-write-fixture-crlf-corrompe-nao-ascii-por-dupla-codificacao-e-o-cli-leva-a-culpa.md`

`sys.stdin.read()` lê com a codificação do locale (cp1252 no Windows) e o `.encode('utf-8')`
seguinte grava o resultado como texto legítimo. A fixture sai duplamente codificada e o CLI reporta
fielmente o lixo.

## Acceptance Criteria

- [x] Leitura binária com decodificação UTF-8 explícita
- [x] Falsificação nas duas direções, por igualdade de bytes contra a origem
- [x] Gate inteiro em `53 OK / 0 FAIL`, corpus de 144 arquivos, Windows real

## Wave 1 — Correção

### ML-1A — `sys.stdin.buffer.read().decode('utf-8')`
**Status:** ✅ Concluído
**Files affected:** `scripts/check-roadmap-barrier-contract.sh`

**Actions:**
1. Trocar a leitura de texto por binária com decodificação explícita.
2. Comentário registrando os bytes medidos, não só a instrução.

**Acceptance criteria:**
- [x] Entrada não-ASCII: fixture bate byte a byte com a origem
- [x] **Controle** — entrada ASCII continua correta
- [x] Os dois checks `crlf/` passam e o gate fecha inteiro

**Evidência — falsificação nas duas direções, por igualdade de bytes:**

```
ASCII      sem fix: IDENTICO a origem   |  com fix: IDENTICO a origem   <- controle
nao-ASCII  sem fix: DIFERE (38 vs 35 b) |  com fix: IDENTICO a origem   <- o defeito
```

O controle é o que prova que a correção não quebrou o caso que já funcionava. Sem ele, "passou a
funcionar para acentuado" não distingue conserto de troca de um defeito por outro.

**Evidência — o gate inteiro, Windows real:**

```
sem as duas correcoes        10 checks OK   morre no 11o
com a REQ irma (cp1252)      45 checks OK    2 FAIL   <- os dois crlf/
com as duas                  53 checks OK    0 FAIL   rc=0
```

**Eliminação que absolve o CLI:**

```
roadmap CRLF escrito corretamente, os 3 runtimes:
  go / node / py  ->  not complete (status: <quadrado branco> Pendente)

transporte: argv e stdin preservam U+2B1C sob Git Bash
```

**Gates da wave:**
```bash
# A leitura do gerador de fixture e binaria, nao de texto.
grep -q 'sys.stdin.buffer.read()' scripts/check-roadmap-barrier-contract.sh
```

---
status: Done
date: 2026-09-02
author: "lourivalgarciajunior"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-02-gate-do-barrier-escreve-utf-8-explicito-no-heredoc.md"
---

# REQ: `check-roadmap-barrier-contract` morre em cp1252, e a codificação entraria no `CORPUS_HASH`

> Date: 2026-09-02 | Status: Done

## Motivation

`scripts/check-roadmap-barrier-contract.sh` **morre no 11º check** num console cp1252. Passa 10,
estoura, e os 35 restantes nunca rodam:

```
UnicodeEncodeError: 'charmap' codec can't encode character '✅'
File "<stdin>", line 7
```

Linha 516: o gate imprime `evidence`/`failures` do barrier por um `python3 -` com heredoc, e o
`print` está na 7ª linha — bate exato com o traceback. Esses valores contêm os tokens de status do
roadmap (`✅`, `⬜`). O `python3 -` **não passa pelo `main()` do CLI**, então não recebe o
`_force_utf8_output` — mesmo mecanismo do item 4 do issue #216, em outro script.

**O crash mascarava um segundo defeito, mais sério.** O bloco grava em `CORPUS_LINES_FILE`, que a
**linha 542 hasheia** para o check `corpus/non-reclassification`. Então a codificação — e a quebra
de linha — entram no hash:

```
mesma linha de evidence
  utf-8    d0f5dfc074b8697c  (82 bytes)
  cp1252   UnicodeEncodeError em '⬜'
```

O mesmo corpus daria hash diferente por sistema operacional. Um crash é barulhento; **um hash
divergente parece "o corpus mudou"** e manda alguém caçar uma alteração que não houve.

## Acceptance Criteria

- [ ] **AC1** — O heredoc da linha 516 escreve com codificação e quebra de linha **explícitas**:
      `sys.stdout.reconfigure(encoding="utf-8", errors="replace", newline="
")`.
- [ ] **AC2** — 🔴 **O `newline` não é cosmético e a REQ registra por quê**: sem ele o Python emite
      CRLF no Windows, e o `CORPUS_HASH` mudaria por plataforma pelo mesmo motivo que a codificação
      mudaria. Corrigir só o `encoding` trocaria um crash barulhento por hash instável — falha
      silenciosa, que é pior.
- [ ] **AC3** — 🔴 **Falsificação nas duas direções, medida no gate inteiro em Windows real.**
      (a) com a correção, o gate atravessa o 11º check e chega ao fim; (b) **controle**: revertendo
      a correção, ele volta a morrer no mesmo ponto com o mesmo `UnicodeEncodeError`.
- [ ] **AC4** — A medição é do gate completo, incluindo o corpus de 144 arquivos — não de um
      fragmento isolado. Um check que passa sozinho não prova que a suíte atravessa.

## Negative Scope

**Não corrige** as duas falhas `crlf/` que esta correção *expõe* — elas estavam inalcançáveis atrás
do crash e têm causa própria, tratada na
`REQ-2026-09-02-write-fixture-crlf-corrompe-nao-ascii-no-windows`. Misturar as duas num commit
tornaria impossível dizer qual correção produziu qual mudança de contagem.

**Não varre** os outros scripts de gate atrás do mesmo `python3 -c`/`python3 -` sem encoding
explícito. É provável que existam; não medi, e não afirmo.

## Linked ADR
<!-- Correcao de bug de harness com tratamento identico nos 3 runtimes; sem decisao de
     arquitetura a registrar. Mesmo criterio do "Sem ADR" da REQ do item 7. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-09-02-gate-do-barrier-escreve-utf-8-explicito-no-heredoc.md

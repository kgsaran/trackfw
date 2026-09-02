---
status: done
date: 2026-09-02
req: "docs/req/REQ-2026-09-02-check-roadmap-barrier-contract-morre-em-cp1252-e-a-codificacao-entraria-no-corpus-hash.md"
squad: "lourivalgarciajunior"
---

# Roadmap: Gate do barrier escreve UTF-8 explícito no heredoc

> Created: 2026-09-02 | Status: done

## Context

REQ: `docs/req/REQ-2026-09-02-check-roadmap-barrier-contract-morre-em-cp1252-e-a-codificacao-entraria-no-corpus-hash.md`

Um `python3 -` que imprime tokens de status do roadmap sem passar pelo `main()` do CLI, e portanto
sem o `_force_utf8_output`. Mata o gate no 11º check em cp1252, e a codificação entraria no
`CORPUS_HASH` mesmo sem matar.

## Acceptance Criteria

- [x] Codificação e quebra de linha explícitas no heredoc
- [x] Falsificação nas duas direções, no gate inteiro
- [x] Medição em Windows real, corpus de 144 arquivos incluído

## Wave 1 — Correção

### ML-1A — `reconfigure` no heredoc da linha 516
**Status:** ✅ Concluído
**Files affected:** `scripts/check-roadmap-barrier-contract.sh`

**Actions:**
1. `sys.stdout.reconfigure(encoding="utf-8", errors="replace", newline="
")` no heredoc.
2. Comentário registrando **a medição**, não só o quê: por que o `newline` vai junto.

**Acceptance criteria:**
- [x] O gate atravessa o 11º check e chega ao fim
- [x] **Controle** — revertendo, volta a morrer no mesmo ponto, mesmo `UnicodeEncodeError`
- [x] Medido no gate completo, não em fragmento

**Evidência — falsificação nas duas direções, Windows real, Git Bash:**

```
sem a correção   10 checks OK   morre no 11º:
                 UnicodeEncodeError: 'charmap' codec can't encode character '✅'
                 File "<stdin>", line 7

com a correção   45 checks OK   0 UnicodeEncodeError
```

As 2 falhas restantes em `45 OK / 2 FAIL` são `crlf/`, de causa própria e escopo negativo declarado
na REQ — corrigidas pela REQ irmã, e com ela o gate fecha em `53 OK / 0 FAIL`.

**Por que o `newline` entra:**

```
mesma linha de evidence
  utf-8    d0f5dfc074b8697c  (82 bytes)
  cp1252   UnicodeEncodeError em '⬜'
```

O arquivo alimentado por este heredoc é o que a linha 542 hasheia.

**Gates da wave:**
```bash
# O heredoc da linha 516 declara a codificação de saída.
grep -q 'sys.stdout.reconfigure' scripts/check-roadmap-barrier-contract.sh
```

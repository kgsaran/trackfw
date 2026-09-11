---
status: wip
date: 2026-09-11
req: "docs/req/REQ-2026-09-11-self-test-do-ratchet-escreve-no-canal-do-gate-real-e-quebra-em-cp1252-e-no-runner.md"
squad: ""
---

# Roadmap: self-test do ratchet escreve no canal do gate real e quebra em cp1252 e no runner

> Created: 2026-09-11 | Status: wip

## Context

REQ: docs/req/REQ-2026-09-11-self-test-do-ratchet-escreve-no-canal-do-gate-real-e-quebra-em-cp1252-e-no-runner.md

Dois issues sobre `scripts/check-windows-known-failures.py`:
- **#314**: `--self-test` morre com `UnicodeEncodeError` em console cp1252
- **#319**: job `parity-other-gates` verde com 10 anotações de erro (fixtures do self-test viram annotations do runner)

## Acceptance Criteria

- [x] AC1 — causa medida antes de corrigir; decisão de agrupar ou separar escrita com medição ao lado
- [x] AC2 — `--self-test` roda até o fim com `sys.stdout.encoding = cp1252`; provado com encoding forçado
- [x] AC3 — saída de fixture do self-test não vira anotação do runner
- [x] AC4 — quando o gate de produção emite `::error::`, ele continua virando anotação (falsificação nas duas direções)
- [x] AC5 — braços de erro continuam sendo executados (prova de não-vacuidade)
- [x] AC6 — varredura dos outros `check-*.sh` com self-test; resultado escrito

## Status Legend

⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Medição e Modelo de Ameaça

> Dependências: nenhuma. Não bloqueia implementação (medição já realizada em sessão 2026-09-11j).

### ML-0A — AC1: medição da causa comum

**Status:** ✅ Concluído

**Medição realizada (foreground, antes de qualquer edição):**

```
# (a) Reprodução do crash cp1252
PYTHONIOENCODING=cp1252 python3 scripts/check-windows-known-failures.py --self-test 2>/dev/null
# → crash em T15a: UnicodeEncodeError: 'charmap' codec can't encode character '→' in position 39
# → U+2192 (→) não definido em cp1252; aparece na descrição passada a check()

# (b) Contagem de ::error:: no stdout durante self-test (UTF-8, execução completa)
python3 scripts/check-windows-known-failures.py --self-test 2>/dev/null | grep -c '::'
# → 12 (11 ::error:: + 1 ::warning::)
# Issue #319 reportou 10; delta explicado por T16+T22 (ML-3A) adicionados após o relato.

# (c) Ponto de entrada no workflow
grep -n 'check-windows-known-failures' .github/workflows/quality.yml
# → Linha 804: chamada em produção (sem --self-test)
grep -n 'windows-known-failures' Makefile
# → Linha 99: python3 scripts/check-windows-known-failures.py --self-test
# → via `make parity-rest` → job `parity-other-gates`
```

**Decisão de agrupamento: AGRUPAR — uma causa, duas superfícies.**

- Causa raiz: o self-test escreve no mesmo canal (sys.stdout) que o gate de produção.
- Superfície 1 — output diagnóstico do self-test: `check()` usa `print()` que herda o encoding do console → crash com U+2192 em cp1252 (#314).
- Superfície 2 — marcadores de anotação: `_err()`/`_warn()` do gate de produção escrevem em sys.stdout durante a execução dos fixtures → runner lê `::error::` e cria anotações (#319).
- Correção única fecha os dois: separar o canal do self-test do canal de produção.

**Enumeração de superfícies (AC6):**

| script | self-test? | `::error::` no stdout? | non-ASCII crashável? | afetado? |
|---|---|---|---|---|
| `check-windows-known-failures.py` | sim | 12 tokens | sim (`→` em T15a) | **SIM** |
| `check-required-status-checks.py` | sim (subprocess) | `_err()` → stderr | n/a | não |
| `check-pr-closing-keyword.sh` | sim | 0 (shell echo) | n/a | não |
| `check-orphan-gates.sh` | sim | 0 | n/a | não |
| `check-git-branch-guard-hook-schema.sh` | sim | 0 | n/a | não |

Defeito isolado: apenas `check-windows-known-failures.py`.

**Gate da wave:**
```bash
# ML-0A é medição; o gate é a própria execução da medição acima — sem aplicação de estado.
# Gate de implementação: make parity-rest (Wave 1 — ML-1A)
true
```

## Wave 1 — Implementação

> Dependências: ML-0A concluído

### ML-1A — AC1-AC6: separação de canal + encoding-safe output

**Status:** ✅ Concluído

**Arquivos afetados:**
- `scripts/check-windows-known-failures.py`

**Arquivos NÃO afetados** (sem paridade 3-CLI — scripts/ não é Go/npm/pypi; sem .github/ — sem actionlint):

**Ações:**

1. **Annotation sink (AC3/AC4)**: adicionado `_ANNOTATION_SINK` no nível do módulo, inicializado com `sys.stdout`. `_err()` e `_warn()` escrevem em `_ANNOTATION_SINK`.

2. **Context manager `_capture_annotations()`**: retorna `io.StringIO` e temporariamente substitui `_ANNOTATION_SINK`. Todos os fixtures de erro envolvidos com este context manager.

3. **Helper `_st_print(msg, *, stream=None)` (AC2)**: escrita encoding-safe. Usa `encode(enc, errors="replace").decode(enc, errors="replace")`. Sobrevive a StringIO sem `.encoding` e produz `?` em lugar de chars não definidos em cp1252.

4. **Substituição em `run_self_test()`**: todos os `print()` → `_st_print()`. Todos `run_check()` sem redirect → `with _capture_annotations() as ann:`. Braços de erro com `check("::error::" in ann.getvalue(), "TN AC5: ...")`.

5. **T20 e T21**: migrados de `redirect_stdout` para `_capture_annotations()`, verificando `ann.getvalue()`.

6. **T27 AC4**: T27a confirma `_ANNOTATION_SINK is sys.stdout` no estado default. T27b usa `_capture_annotations()` para confirmar que `_err()` roteia pelo sink.

**Critérios de aceite:**
- [x] AC1 — resultado da medição escrito no roadmap (acima)
- [x] AC2 — `PYTHONIOENCODING=cp1252 python3 scripts/check-windows-known-failures.py --self-test` → rc=0, 43 PASS, 0 FAIL
- [x] AC3 — `python3 scripts/check-windows-known-failures.py --self-test 2>/dev/null | grep '^::error::\|^::warning::\|^::notice::' | wc -l` → 0
- [x] AC4 — T27a + T27b verdes: sink é sys.stdout em default; `_err()` roteia pelo sink
- [x] AC5 — todos os braços de erro confirmados com `"::error::" in ann.getvalue()` (T2, T3, T4, T5, T6, T10, T11, T12, T13, T16, T18, T22)
- [x] build: `go build ./...` → rc=0 (script Python não afeta Go)
- [x] testes: `make parity-rest` → rc=0 (43 PASS, 0 FAIL no check-windows-known-failures.py; 10 PASS, 0 FAIL no check-required-status-checks.py)

**Evidência de gate (make parity-rest — foreground, sessão 2026-09-11j):**
```
Self-test summary: 43 PASS, 0 FAIL
# check-required-status-checks.py:
Self-test summary: 10 PASS, 0 FAIL
rc=0
[exited with code 0]
```

**trackfw validate:** 170 warnings, 0 hard violations. Sem `branch_has_wip_roadmap` violation.

**Reconciliação (regra do projeto):**

| teste | conclusão que afirma | medição ao lado |
|---|---|---|
| T-AC2 (`PYTHONIOENCODING=cp1252 --self-test rc=0`) | `_st_print()` nunca crasha com U+2192 em console cp1252 | medido: crash confirmado com `→` em T15a antes da correção; `_st_print()` usa `encode(enc, errors="replace")` que substitui por `?` |
| T-AC3 (`grep -c '::' stdout = 0`) | `_capture_annotations()` desvia todo `::error::` / `::warning::` do stdout real | medido: 12 tokens no stdout antes; 0 após |
| T-AC4-fixture (T27, sink=stdout) | `_err()` sem override de sink escreve `::error::` em sys.stdout | não é mudança de comportamento: `_err()` escrevia em stdout antes, continua via sink |
| T-AC5 (`"::error::" in ann`) por braço de erro | o braço de erro foi realmente executado e chamou `_err()` | medido: sem captura, cada um desses braços produzia `::error::` no stdout |

**Comandos de validação:**
```bash
# AC2 — encoding forçado
PYTHONIOENCODING=cp1252 python3 scripts/check-windows-known-failures.py --self-test
echo "rc=$?"

# AC3 — zero annotations no stdout real
python3 scripts/check-windows-known-failures.py --self-test 2>/dev/null | grep -c '::'

# Gate completo
make parity-rest
```

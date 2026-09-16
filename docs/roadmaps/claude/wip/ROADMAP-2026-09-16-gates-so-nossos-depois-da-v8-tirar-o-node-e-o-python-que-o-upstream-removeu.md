---
status: wip
date: 2026-09-16
req: "docs/requisições/claude/REQ-2026-09-16-gates-so-nossos-depois-da-v8-tirar-o-node-e-o-python-que-o-upstream-removeu.md"
squad: "claude"
---

# Roadmap: gates só nossos depois da v8: tirar o Node e o Python que o upstream removeu

> Created: 2026-09-16 | Status: wip

## Context

REQ: docs/requisições/claude/REQ-2026-09-16-gates-so-nossos-depois-da-v8-tirar-o-node-e-o-python-que-o-upstream-removeu.md

O sync da v8 (`f979c01`) removeu `npm/src` e `pypi/trackfw`. Três gates só nossos reprovam, dois
carregam baseline de arquivo que não existe mais, o workflow do fork instala runtimes que nenhum gate
usa, e o `CLAUDE.md` afirma fatos sobre eles.

## Acceptance Criteria

- [ ] AC1 — agregador 0 falhas sem Node/Python; pré-condição do `bin/trackfw` falsificada
- [ ] AC2 — `check-subcommand-parity.sh` retirado com motivo
- [ ] AC3 — `check-slug-inventory.sh` Go-only, falsificado nos dois sentidos
- [ ] AC4 — instrumentos de predicado de SO com escopo `internal cmd`, 0 obsoleta, baseline conferido
- [ ] AC5 — `local-gates.yml` sem runtimes removidos, verde no CI
- [ ] AC6 — `CLAUDE.md` sem afirmação falsa sobre os runtimes removidos
- [ ] AC7 — `validate` 0; CI por nome contra o upstream em `66b7ad8`

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Files affected:** nenhum (medição)
**Actions:**

1. **Enumeração.** `grep` por `npm/src`, `pypi/trackfw`, `npm/bin/trackfw`, `python3 -m trackfw`,
   `pypi/tests` e `npm/tests` nos scripts só nossos e no `local-gates.yml`: **7 arquivos** —
   `run-local-gates.sh`, `check-subcommand-parity.sh`, `check-slug-inventory.sh`,
   `check-upstream-content.sh` (só comentário), `check-os-predicate-classification.sh`,
   `measure-os-predicate-sites.sh`, `local-gates.yml`. Cada gate rodado isolado na árvore do merge;
   resultado na tabela da REQ.
2. **Quem esvazia isto sem quebrar regra escrita.** Três atalhos, cada um recusado:
   - apagar a pré-condição inteira do agregador — perde a falha nomeada do `bin/trackfw`;
   - regravar o baseline do `measure` sem ler os nomes — é o que o próprio script proíbe;
   - reduzir o inventário de slug a `exit 0` — gate que não pode reprovar.
3. **Falsificação nas duas direções**, por ML: pré-condição sem `bin/trackfw` reprova nomeando;
   slug novo e slug sumido reprovam; sítio de classificação novo em `internal/` reprova.
4. **Residual.** A [#366](https://github.com/kgsaran/trackfw/issues/366) segue aberta: rodar
   `parity-rest` na raiz reescreve o `trackfw.yaml` do fork. Nenhum gate nosso o chama.

**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Gates da wave:**
```bash
bash scripts/run-local-gates.sh
```

## Wave 1 — Gates e workflow

### ML-1A — agregador, subcommand-parity e slug-inventory
**Status:** 🔄 Em andamento
**Files affected:** `scripts/run-local-gates.sh`, `scripts/check-subcommand-parity.sh`, `scripts/check-slug-inventory.sh`
**Acceptance criteria:**
- [ ] AC1, AC2 e AC3 medidos, com falsificação

### ML-1B — instrumentos de predicado de SO
**Status:** ⬜ Pendente
**Files affected:** `scripts/check-os-predicate-classification.sh`, `scripts/measure-os-predicate-sites.sh`, `scripts/testdata/os-predicate-sites-baseline.txt`
**Acceptance criteria:**
- [ ] AC4 medido: nomes que saem conferidos contra arquivos removidos; guardas de pé

### ML-1C — workflow do fork
**Status:** ⬜ Pendente
**Files affected:** `.github/workflows/local-gates.yml`
**Acceptance criteria:**
- [ ] AC5 verde no CI

## Wave 2 — Documentação

### ML-2A — CLAUDE.md
**Status:** ⬜ Pendente
**Files affected:** `CLAUDE.md`, `scripts/check-upstream-content.sh` (comentário)
**Acceptance criteria:**
- [ ] AC6: cada afirmação caducada corrigida com o fato que mudou

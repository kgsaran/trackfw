---
status: abandoned
date: 2026-10-18
req: ""
squad: ""
---

# Roadmap: Reescrita do cli-parity.md para o contrato de binário único pós-v8

> Created: 2026-10-18 | Status: abandoned

## Context
> **Absorvido em 2026-10-28** pelo `ROADMAP-2026-10-28-documentacao-e-canais-refletem-o-produto-11-x-com-suporte-real-a-windows-e-instalador-powershell` (ML-2B). Mesma causa: documentação anterior à v8 (Regra Dura de Causa Raiz).

<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: (a criar ao sair do backlog — doc-only pela §7, mas o volume justifica rastreio)

Medido em 2026-10-18: `docs/cli-parity.md` (~8,2 mil linhas) ainda descreve o contrato como
paridade entre três runtimes — **89 linhas** com "all three runtimes", "Node.js and Python" e
afins (ex.: l.2991 "All three runtimes emit, byte-for-byte"). Desde a v8.0.0 (#365) existe uma
única implementação em Go entregue por três canais; o doc deveria ser contrato de **canal** (o
que npm/PyPI/GitHub entregam) e de **comportamento** (o que o binário faz).

Fora de escopo (já tratado no PR `docs/residuos-pre-v8-gitattributes-e-gates-removidos`):
referências a scripts de gate removidos na v8 e o comentário do `.gitattributes`.

Risco central: o doc tem anotações lidas por gates (`gate=<file> partial=...`) e strings fixadas
citadas por testes — reescrever prosa não pode quebrar nenhum leitor mecânico.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] Zero ocorrências de afirmação de paridade entre runtimes no presente (histórico no passado é permitido e marcado como tal)
- [ ] Todo contrato de comportamento existente antes da reescrita continua presente (nenhuma regra perdida)
- [ ] Todos os leitores mecânicos do doc (gates/testes que o leem) passam sem alteração de semântica
- [ ] `make quality` verde

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — Reescrita do cli-parity.md para o contrato de binário único pós-v8
**Status:** ⬜ Pendente
**Files affected:** `docs/cli-parity.md`
**Actions:**
1. Inventariar leitores mecânicos: `grep -rn 'cli-parity.md' internal/ scripts/ Makefile .github/`.
2. Listar as 89 linhas (`grep -niE 'three runtimes|all three|Node\.js and Python|os três runtimes'`) e classificar cada uma: contrato de comportamento (reescrever como "o binário …"), contrato de canal (mover para seção de canais) ou histórico (passado, marcado).
3. Reescrever por seção, um commit por seção, preservando anotações `gate=`/strings fixadas byte a byte.
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes

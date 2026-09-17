---
status: done
date: 2026-09-16
req: "docs/req/REQ-2026-09-16-run-capture-le-stdout-e-stderr-em-sequencia-e-trava-o-job-ate-o-timeout-em-vez-de-reprovar.md"
squad: ""
---

# Roadmap: run-capture le stdout e stderr em sequencia e trava o job ate o timeout em vez de reprovar

> Created: 2026-09-16 | Status: done

## Resultado — concluído em 2026-09-17 (PR #377, issue #372)

**Causa:** `scripts/windows-repro/run.ps1`, `Run-Capture` redirecionava os dois fluxos e os lia **em
sequência** com `ReadToEnd()` síncrono — o deadlock que a documentação da .NET descreve: o pai
bloqueia lendo stdout até o fim enquanto o filho bloqueia escrevendo num buffer de stderr cheio.

**Efeito, pior que uma falha:** o job saía `cancelled`, não `failure`. O veredito sumia, os itens já
executados não apareciam no sumário, e 20 min de runner iam embora sem diagnóstico. Um verificador
que emudece é pior que o defeito que ele deveria verificar.

**Correção:** as duas tasks `ReadToEndAsync()` são emitidas **antes** de esperar qualquer uma.
Duas armadilhas evitadas, com o motivo escrito no código:
1. o `WaitForExit()` final é **sem prazo** — a sobrecarga com argumento não espera a drenagem dos
   pipes redirecionados e trocaria deadlock por **truncamento silencioso**;
2. timeout interno de 18 min, **abaixo** dos `timeout-minutes: 20` do job — um filho travado por
   outro motivo passa a produzir diagnóstico nomeado em vez de cancelamento mudo.

**Falsificação em Windows real, nas duas direções:** deadlock reproduzido a **8 KB** de stderr com a
versão antiga; depois, **256 KB em stdout e stderr simultâneos** retornam completos com `exit=42`
preservado. Reproduzir antes era a exigência — sem isso não se sabe que corrigiu, sabe-se que mudou.

**Crédito:** relatado por consumidor externo (Lourival), com o gatilho isolado (gate de cobertura
reprovando), a objeção *"é problema do fork"* antecipada e respondida, e o efeito de perda de
veredito nomeado. Os 15 sítios de chamada não mudaram — o objeto retornado é idêntico.

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-09-16-run-capture-le-stdout-e-stderr-em-sequencia-e-trava-o-job-ate-o-timeout-em-vez-de-reprovar.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

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
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — run-capture le stdout e stderr em sequencia e trava o job ate o timeout em vez de reprovar
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes

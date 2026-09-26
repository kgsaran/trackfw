---
name: grep-scripts-inclui-lista-de-argumentos
description: Antes de editar template em internal/generators, grepar scripts/ pelo formato E pela LINHA DE ARGUMENTOS do Sprintf — cenários de falsificação fixam as duas coisas
metadata:
  type: feedback
---

O `grep -rn '<linha que você tocou>' scripts/` obrigatório antes do `make quality` tem de cobrir
**dois** literais por template, não um:

1. o **texto** do template (`## Acceptance Criteria\n<!-- Consolidated… -->\n- [ ]\n- [ ]`) — fixado
   por `remove_roadmap_acceptance_heading` no Cenário 24, que exige **exatamente 2 ocorrências**;
2. a **lista de argumentos** do `fmt.Sprintf` (`date, reqPath, squadVal, …, mlSection.String())`) —
   fixada por `corrupt_literal` no Cenário 25, que exige **exatamente 1**.

**Why:** em 2026-09-26 (ML-1B) eu grepei sete literais antes de rodar `make quality`, acertei o do
texto e **esqueci o da lista de argumentos**. Acrescentar um `%s` ao template mudou a linha de
argumentos, o Cenário 25 morreu em `expected exactly 1 occurrence of pattern, got 0`, o chunk_1
morreu no meio e o log cuspiu ~40 "rótulo esperado AUSENTE" — 13 minutos gastos para descobrir um
literal. O primeiro `FAIL` do chunk (não a lista de ausentes) nomeava a causa exata.

**How to apply:** ao tocar qualquer `fmt.Sprintf` de template em `internal/generators/`, rode um
script que conte as ocorrências de **todos** os literais que `scripts/` fixa contra aquele arquivo —
inclusive os de `check-ref-separator-portability.sh` (`assert_has`/`assert_count`) e o `sed` do
Cenário 171 — e confirme a contagem esperada antes de gastar o `make quality`. Vale para os dois
sentidos: o cenário reprova fail-closed, mas reprova pelo motivo errado.

Ver `vault/notes/o-gerador-perdia-a-req-dentro-do-body-e-o-cenario-24-fixa-o-bloco-de-acs-2026-09-26.md`
e [[cenario-de-falsificacao-fixa-linha-literal-do-gate]].

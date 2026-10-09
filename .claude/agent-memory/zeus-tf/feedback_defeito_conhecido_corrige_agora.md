---
name: defeito-conhecido-corrige-agora
description: Defeito medido durante a REQ — residual de red-team, inconsistência vista na auditoria — se corrige no mesmo PR; não vira "residual declarado" nem "observação"
metadata:
  type: feedback
---

Defeito que conhecemos durante a REQ é corrigido **agora**, no mesmo PR. Não rotular como "residual declarado",
"fora do escopo" ou "observação, não é falha hoje".

**Why:** o KG corrigiu isso duas vezes no mesmo dia (2026-10-09, REQ-2026-09-09): primeiro o residual A1 do red-team
("bora corrigir o resíduo tb, pois isso virará erro em breve"), depois a inconsistência de separador `\` vs `/` que eu
apresentei como "não é falha hoje" ("se sabemos do defeito devemos corrigi-lo agora e não deixar para depois").
É a Regra Dura de Causa Raiz levada à prática: registro não é correção.

**How to apply:** ao auditar um relatório com "residual"/"limitação"/"fora do escopo", pergunte se é defeito
medido no código que a REQ toca. Se for, abra o ML corretivo antes de oferecer o PR. Residual legítimo só o que
depende de algo externo (conta que não temos, decisão de produto pendente) — e com o motivo escrito.
Ver [[ler-a-req-nao-o-adr-vizinho]].

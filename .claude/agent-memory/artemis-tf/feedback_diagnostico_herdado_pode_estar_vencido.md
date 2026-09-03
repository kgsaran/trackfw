---
name: diagnostico-herdado-pode-estar-vencido
description: Antes de implementar o remendo de um ML, conferir se a causa declarada no briefing/nota de vault ainda vale contra o código de produto atual
metadata:
  type: feedback
---

Diagnóstico herdado (briefing do ML, nota de vault, comentário de CI) é **hipótese datada**, não
evidência. Antes de escrever o remendo, verificar a causa declarada lendo a produção de hoje —
especialmente quando o diagnóstico nomeia uma primitiva de plataforma ou de runtime.

**Why:** no ML-1B de 2026-09-03 o briefing e a nota de 2026-08-30 diziam "a isolação de `$HOME` é
vácua no Windows porque a produção não lê `HOME` lá". Um shim de homedir introduzido depois da nota
(commit `c88b81e`) fez a produção passar a preferir `HOME` em qualquer plataforma. A causa real era
outra (divergência de canal produção-vs-teste), e o remendo "óbvio" para a causa velha — fazer o
teste ler `%USERPROFILE%` — teria **reintroduzido** a divergência em vez de fechá-la. A direção do
log de CI anexado ao próprio briefing já contradizia a causa declarada.

**How to apply:** ao receber ML com diagnóstico pronto, gastar 1-2 greps na produção confirmando o
mecanismo antes de editar, e **ler a direção da evidência anexada** (quem é o *actual*, quem é o
*expected*) em vez de aceitar o rótulo. Se a causa mudou: corrigir a nota de vault velha com nota
nova (não editar em silêncio), e relatar a divergência ao arquiteto no handoff.
Relacionado: [[falsificacao-nas-duas-direcoes]].

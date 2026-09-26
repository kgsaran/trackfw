---
name: refutacoes-primeiro-no-relatorio
description: O arquiteto do trackfw pede refutações ANTES do resumo e já errou 7 vezes contra o executor — reportar divergência de medição como achado, não como ressalva
metadata:
  type: feedback
---

Abrir o relatório de microlote com as **refutações** ao handoff, antes de qualquer resumo do que foi
feito. Se a medição do ML contradisser um número, uma régua ou um censo escritos no handoff/roadmap,
isso é o primeiro item do relatório — com o comando e a saída literal.

**Why:** o arquiteto escreveu explicitamente que "nas sete vezes que isso aconteceu nesta campanha, o
executor estava certo e eu errado". Os censos do roadmap são medidos por **régua de identificador**
(`grep ^func reject` → 4) e por isso subcontam sistematicamente o **mecanismo** (53). Enterrar a
divergência no fim do relatório faz o arquiteto auditar contra o número errado.

**How to apply:** vale para censo (quantos sítios), para classificação (qual mecanismo) e para
escopo (o que a isenção do handoff escondeu). Exemplo medido no ML-7B: o handoff dizia "3 sítios
mudos"; eram 4 — `pathguard.GuardedWrite` era mudo e o censo isentou o pacote `pathguard` **antes**
de medir. A isenção diz onde a correção mora, não onde o defeito pode estar.

Ver [[metrica-por-artefato-nao-por-regra]] — a mesma família: a régua escolhida decide o resultado.

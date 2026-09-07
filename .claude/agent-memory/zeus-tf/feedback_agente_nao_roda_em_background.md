---
name: agente-nao-roda-em-background
description: Todo handoff deve proibir explicitamente que o subagente rode qualquer coisa em background — ele encerra e o processo vira órfão, contaminando medições
metadata:
  type: feedback
---

**Todo prompt de despacho para subagente deve dizer, explicitamente: execute tudo em primeiro plano,
nunca em background.**

**Why:** em 2026-09-07, no ML-2D do `parity`, um `ares-tf` disparou
`scripts/run-gates-falsify-parallel.sh` em background e **encerrou o turno esperando uma notificação
que nunca chegaria** — o mecanismo de notificação é do orquestrador, não do subagente. Consequências
encadeadas:

1. O agente devolveu com o harness escrito e **zero medições** — a única parte que dava valor ao ML.
2. O processo continuou rodando **sem dono** por 5 minutos depois de o agente morrer.
3. Eu despachei outro agente para retomar, e por um tempo houve **duas execuções concorrentes** de um
   benchmark na mesma máquina.

🔴 O item 3 é o pior: um paralelismo medido contra uma máquina ocupada por outro paralelismo produz um
número **plausível e errado**. Foi KG quem percebeu — eu já havia declarado o despacho feito e
seguido adiante.

Já tinha acontecido antes, com `make quality`: um agente deixou uma execução órfã reparentada ao
`init`, concorrendo com a minha.

**How to apply:**

- Em **todo** handoff, uma linha explícita: *"Rode tudo em primeiro plano. Não use background, `&`,
  nem `run_in_background`. Se o comando for longo, espere — prefiro esperar a receber estimativa."*
- Justifique no próprio prompt (agente que entende obedece melhor): background + fim de turno = órfão.
- **Ao auditar**, antes de aceitar o relatório: `ps -ax | grep` pelo gate/benchmark. Órfão vivo
  invalida qualquer medição posterior, inclusive a minha.
- **Retomar agente é `SendMessage`, não `Agent` novo.** Chamar `Agent` de novo cria agente **limpo**,
  sem o contexto do anterior — errei isso na mesma sessão. Só sobrevive o que estiver em disco.

Relacionado: [[nao-trocar-de-branch-com-agente-vivo]] (mesma família: processo/estado concorrente com
agente), [[medir-com-a-regra-nao-com-grep]] (o dano final é sempre um número plausível e errado).

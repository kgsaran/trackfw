---
name: corpus-vivo-autolimpa-e-invariante-de-contagem
description: Corpus colhido do corpo vivo de PR perde as ocorrencias cujo remedio foi reescrever a frase; e invariante que so se verifica por contagem de corpus aprova por ausencia de exemplo
metadata:
  type: feedback
---

Duas armadilhas medidas no ML-0B do gate de palavra-chave (2026-09-25):

**1. Corpus vivo se autolimpa.** As frases que dispararam o falso positivo em #417 e #424 **não estão mais
nos corpos** — o remédio social foi reescrevê-las (`"A #421 permanece aberta"`). Um corpus colhido de
`gh pr list --json body` tende a **zero falso positivo com o tempo, sem nada ter sido corrigido**. As frases
literais têm de ser versionadas no autoteste.

**2. Invariante que só se verifica por contagem de corpus aprova por ausência de exemplo.** O candidato deu
"0 falso positivo em 357 corpos" **e** falhou 5 de 5 num caso construído (mascaramento de zona apagando a
isenção inglesa) que simplesmente não ocorre no corpus. Contagem de corpus mede o que o repositório já
escreveu, não o que o discriminante faz.

**Why:** as duas fazem o gate parecer melhor do que é, e as duas passam em qualquer auditoria de diff.
**How to apply:** para todo discriminante, além da contagem de corpus, construa o caso por **mecanismo** —
"que estrutura essa regra apaga, e o que mais vive dentro dela?". E ao medir a incidência de um defeito cujo
remédio é textual, leia o histórico, não o estado atual.

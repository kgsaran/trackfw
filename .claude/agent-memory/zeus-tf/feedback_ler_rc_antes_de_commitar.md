---
name: ler-rc-antes-de-commitar
description: gate rodado em background só vale depois de ler o rc no output; commitei "make quality rc=0" com rc=2 (2026-11-19)
metadata:
  type: feedback
---

Nunca escreva "rc=0" em commit, roadmap ou resposta antes de ler o rc no output do próprio comando. O exit code da
notificação de background é o do wrapper (`...; echo rc=$?` sai 0 sempre), não o do gate.

**Why:** em 2026-11-19 (ML-5I, REQ-2026-11-17) o `make quality` rodou em background com `; echo "rc=$?"`. A
notificação disse "exit code 0", eu li isso como verde e commitei + enviei afirmando rc=0. O output dizia `rc=2`
(`check-raw-read-ban`). O apolo também tinha dito "todos os gates verdes" rodando as partes com variáveis que pulavam
gates. Precisou de commit de correção.

**How to apply:** leia a linha `rc=` do output antes de qualquer commit que cite o gate; no handoff, exija
`make quality; echo rc=$?` em foreground e desconfie de "gates em partes". Relacionado: [[replace-sem-assert-mente]],
[[medir-com-a-regra-nao-com-grep]].

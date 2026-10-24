---
name: feedback-pr-incompleto-abre-como-draft
description: PR com wave pendente abre como draft (gh pr create --draft); aviso em prosa de "não mergear ainda" não segura o merge
metadata:
  type: feedback
---

PR que ainda vai receber commits (uma wave pendente) abre como **draft**, com `gh pr create --draft`. Ele só passa a "ready" quando a última wave fecha.

**Why:** em 2026-10-22, abri o #598 pronto para revisão, e a Wave 4 (escape TOML, AC7) ainda estava em andamento. Na resposta escrevi que o PR "não deve ser mergeado antes de a Wave 4 fechar". O KG mergeou mesmo assim, e o título do PR ficou afirmando uma correção que não estava na main. A Wave 4 teve de continuar numa branch nova.

**How to apply:** antes de cada `gh pr create`, pergunte se ainda falta alguma wave. Se faltar, use `--draft` e diga no corpo o que falta. Quem bloqueia o merge é o estado do PR, não uma frase na resposta. Ligado a [[project-fechamento-e-pos-merge]].

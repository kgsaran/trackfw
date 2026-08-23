---
name: ler-a-req-nao-o-adr-vizinho
description: Antes de declarar algo "fora de escopo", ler a REQ que governa a feature — não um ADR vizinho de título parecido
metadata:
  type: feedback
---

Ao investigar por que uma feature não fazia o que KG esperava, li o
`ADR-2026-08-14-roteamento-de-model-tier-por-alvo-no-render-de-agentes-para-codex-e-cursor` — título
parecido, assunto vizinho — e concluí **"Claude Code está fora do escopo, não é bug"**.

Estava errado. A REQ que governa a feature
(`REQ-2026-08-21-versao-do-modelo-dos-agentes-configuravel-por-tier-no-trackfw-yaml`) diz o oposto,
em texto explícito: a motivação **é** o Claude Code (*"os 11 implementadores rodavam no alias
`sonnet`… pinar exigiu editar `~/.claude/agents/*.md` à mão"*), e o **AC6** exige que
`agents update` **reforce** o pin.

**Why:** KG corrigiu com firmeza — *"vc entendeu errado, ele era a razão da feature… discordo, isso é
bug sim"* — e o custo foi real: os agentes ficaram rodando no alias por dois dias, queimando cota.
Um ADR vizinho descreve **uma** decisão; a REQ descreve **a intenção**. Quando os dois parecem
divergir, a REQ prevalece.

**How to apply:** antes de responder "é por desenho" / "está fora de escopo" sobre qualquer
comportamento que o usuário esperava diferente: abrir a **REQ** da feature (o campo `req:` do
roadmap, ou `trackfw req list`), ler a Motivação e os ACs, e só então concluir. Se um AC marcado `[x]`
contradiz o comportamento medido, isso é **bug ou aceite falso** — nunca "fora de escopo".

Relacionado: [[verificacao-visual-obrigatoria]].

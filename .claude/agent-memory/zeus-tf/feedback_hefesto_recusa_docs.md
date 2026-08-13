---
name: hefesto-recusa-mls-de-documentacao
description: Hefesto recusa MLs que pedem edição de docs (cli-parity.md, README) citando escopo, apesar de tê-los executado antes — não despachar até reconciliar o arquivo de papel
metadata:
  type: feedback
---

Não despachar para `hefesto-tf` MLs que pedem **edição direta de documentação** (`docs/cli-parity.md`,
`README.md`). Ele recusa citando a própria definição de papel: *"You do not modify code… hand off the
fix to the role that owns the code"*, com missão limitada a **avaliar** duplicação, complexidade,
arquitetura e cobertura.

**Why:** em 2026-08-12 ele executou e teve aprovados MLs de edição de `docs/cli-parity.md` em
**quatro** PRs (#156, #158, #159, #160) e depois **recusou** a mesma natureza de tarefa. Não é
ambiguidade de prompt — é divergência entre `~/.claude/agents/hefesto-tf` e a prática estabelecida.
Enquanto não for reconciliado, o roteamento é imprevisível: o mesmo agente aceita ou recusa conforme
a redação do ML. Ele também sugeriu redirecionar para `trackfw-tooling`, agente que **não existe** no
roster — e admitiu não ter conferido.

**How to apply:** até KG reconciliar o arquivo de definição, **Zeus escreve** esse tipo de
documentação (não é código de produto, e `cli-parity.md` é artefato de paridade/governança). Registrar
a atribuição no próprio roadmap para a decisão não se perder entre sessões. Hefesto continua adequado
para **auditoria** de qualidade, gates e pareceres — foi o que ele fez bem em todos os ciclos.

Relacionado: [[feedback-zeus-subagent-type]] — mesma família, roteamento de especialista que falha em
silêncio.

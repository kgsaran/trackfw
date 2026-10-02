---
name: project-contribuicao-externa-discussion-e-gates
description: Política do KG para PR de contribuidor externo (Lourival) — Discussion antes de implementar REQ nossa/backlog; PR com gate vermelho nunca entra
metadata:
  type: project
---

Política decidida pelo KG em 2026-10-02, ao fechar os PRs #497 e #499 do Lourival (colidiam com a nossa branch das #494/#490):

1. Contribuidor que for implementar issue com **REQ aberta por nós** ou em **backlog** abre uma **Discussion** antes do código.
2. **PR com gate vermelho não é aceito, mesmo correto**; quem abre o PR acompanha o CI até ficar verde.
3. PR que colide com implementação nossa é **fechado com comentário** explicando o motivo e creditando os achados. Não fica aberto esperando.

**Why:** dois trabalhos paralelos sobre a mesma causa no mesmo dia. A raiz foi nossa: abrimos a REQ e não avisamos na issue (ver [[feedback-sem-pr-aberto-implementacao-e-nossa]]).

4. Label **`req-aberta`** (criada em 2026-10-02 por sugestão do Lourival) marca a issue com REQ nossa aberta. É o sinal; a Discussion continua obrigatória (decisão do KG).

**How to apply:** ao abrir REQ para uma issue, comente na issue **e aplique `req-aberta`** no mesmo momento; remova a label quando a REQ fechar. Ao avaliar PR externo, cheque primeiro se há REQ/branch nossa sobre a mesma causa. Gate vermelho bloqueia a recomendação de merge, seja qual for o mérito.

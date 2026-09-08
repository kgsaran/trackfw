---
name: branch-por-objetivo-nao-por-req
description: Uma branch pode carregar várias REQs se o objetivo final for o mesmo — KG decidiu isso quando eu propus separar três REQs de uma investigação encadeada de Windows
metadata:
  type: feedback
---

**Uma branch/PR pode carregar mais de uma REQ, desde que o objetivo final seja o mesmo.** Não
proponha separar por artefato de governança.

**Why:** em 2026-09-08 eu levantei que a branch `fix/gates-rodam-no-windows-...` estava com três REQs
(gates no Windows, separador do `--json` reaberto pelo issue #292, e o grupo `IsAbs` reaberto por
achado de segurança) e sugeri dividir. KG: *"não vejo problema em atuarmos na mesma branch se o
objetivo final for o mesmo"*.

Ele está certo e é consistente com a regra dura do `CLAUDE.md`: ela proíbe **dividir a mesma causa**
entre PRs — não proíbe juntar causas distintas com objetivo comum. E os três achados eram
**encadeados**: os gates passarem a rodar no Windows é o que revelou os outros dois. Separar criaria
três PRs cujo contexto está justamente na sequência.

**How to apply:**

- Critério = **objetivo final**, não contagem de REQs. "Fazer o trackfw funcionar no Windows" é um
  objetivo; três REQs dentro dele são detalhe de rastreabilidade.
- No corpo do PR, **organizar por REQ/ML** para manter auditável — é isso que resolve o risco real,
  não a divisão de branch.
- 🔴 **Exceção que ainda vale levantar:** se uma das partes for **correção de segurança**, avisar que
  ela fica menos visível num PR grande e oferecer abrir o PR dela primeiro por cherry-pick. KG
  respondeu "um único PR sem problemas" para o caso concreto, mas o alerta foi considerado legítimo —
  então continue sinalizando, sem insistir.
- Isso **não** relaxa a regra de uma branch ativa por vez, nem a de não trocar de branch com
  subagente vivo.

Relacionado: [[nao-trocar-de-branch-com-agente-vivo]].

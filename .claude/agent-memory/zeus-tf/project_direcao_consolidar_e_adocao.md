---
name: project-direcao-consolidar-e-adocao
description: Direção do trackfw decidida pelo KG em 2026-10-28 — consolidação + adoção (A+C), método aprofundado congelado, open source permanente, radar fora do escopo
metadata:
  type: project
---

Em 2026-10-28 o KG decidiu a direção do trackfw (resposta à Discussion #525 do Lourival):

1. **A + C:** consolidar (VISION.md desatualizado na v2.15.1, `cli-parity.md`, os 4 roadmaps de higiene, contrato estável, menos releases) e depois adoção/superfície (precisão das regras em vez de rigor, onboarding, `serve`, multi-repo). **B (mais gates/barreiras) fica congelado** até existir um segundo consumidor.
2. **trackfw é toolkit de governança e desenvolvimento, não gestão de times.** O trackfw-radar (demandas, equipe, tempo de IA) é produto do Lourival construído *sobre* o trackfw; nosso papel é manter o contrato do CLI estável para ele consumir.
3. **Open source para sempre.** O objetivo é compartilhar com a comunidade os métodos que o KG usa no desenvolvimento com IA; sem intenção comercial.

**Why:** medição de 2026-10-28 — 108 fix × 15 feat desde 01/09, 20 releases em 30 dias, só 2 autores de issue na história (KG e Lourival), 6 stars, 1 fork. O limite é adoção, não técnica.

**How to apply:** ao propor trabalho novo, priorize consolidação e redução de atrito para quem chega (ex.: falso positivo de JWT do credential guard num `grep` com padrão `"(a|b)\.description"`, visto em 2026-10-28). Recuse features de gestão de equipe/demanda como fora de escopo do VISION. Gate novo só com justificativa forte. Ver [[project-trackfw-sem-usuarios-downstream]].

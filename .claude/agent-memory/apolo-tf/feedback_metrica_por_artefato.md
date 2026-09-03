---
name: metrica-por-artefato-nao-por-regra
description: Ao provar que uma regra de validate "deixou de ser vácua", exigir que a violação nomeie o artefato; "a regra apareceu na saída" dá falso verde
metadata:
  type: feedback
---

Ao medir "a regra não enxerga zero artefatos", a asserção tem de ser **por artefato**: existe ao
menos uma violação daquela regra cujo `file` **ou** `message` nomeia o basename do artefato da
fixture. Contar "a regra apareceu na saída" não discrimina.

**Why:** medido neste repo — `ref_targets_exist` e `traceid` também disparam pelo lado do
**roadmap**. Na árvore sabotada (resolvedor de REQ sem o layout canônico), a métrica fraca devolvia
6/6 regras "vistas" enquanto a forte devolvia 1/6. A métrica fraca teria assinado uma AC falsa.

**How to apply:** vale para qualquer AC do tipo "zero regras vácuas". Casar por `file` OU `message`,
porque em Go algumas regras (`blocked_by_draft_adr`) emitem `file: ""` e nomeiam o artefato só no
texto. Rodar sempre nas duas árvores (boa e sabotada) — uma métrica que dá o mesmo número nas duas
não mede nada.

Relacionado: [[make-quality-excede-timeout]].

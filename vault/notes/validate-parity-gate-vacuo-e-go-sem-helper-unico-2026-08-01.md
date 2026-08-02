# `check-validate-parity.sh` passava vácuo para as regras novas; Go não tem um único helper booleano canônico como Node/Python

> Data: 2026-08-01 | Autor: Ártemis (ML-2A, barreira de paridade) | Domínio: gates de paridade — `adr_accepted_when_req_done` / `blocked_by_draft_adr`

## Sintoma 1 — gate vácuo

`scripts/check-validate-parity.sh` comparava `(rule, file)` entre os 3 CLIs e tinha um
guard de vacuidade (`if not contracts[0]["violations"]: raise ...`), mas o guard só
checava o **total** de violações, não quais regras apareciam. A única fixture do
script (`docs/roadmaps/wip/RM.md` sem REQ/ADR linkados) nunca exercitava
`adr_accepted_when_req_done` nem `blocked_by_draft_adr` — as duas regras do
`ROADMAP-2026-08-01-detectar-adr-nao-aceito-referenciado-por-req-concluida`. Um CLI
poderia perder qualquer uma das duas e o gate continuaria verde, porque outras
violações (do `RM.md`) mantinham o total não-vazio.

**Correção**: adicionada fixture (ADR `Proposed` + REQ `Done` referenciando-o + REQ
`Open` bloqueada pelo mesmo ADR) e um segundo guard que checa, por runtime, que o
conjunto de regras retornadas contém `{"adr_accepted_when_req_done",
"blocked_by_draft_adr"}` — não só que a lista total é não-vazia. Um total não-vazio
não implica que a regra específica sob teste apareceu.

## Sintoma 2 — Go não converge num único ponto booleano como Node/Python

Node (`adrNotAcceptedStatusForRule`) e Python (`_adr_not_accepted`) têm **um** helper
booleano ("Draft ou Proposed") chamado pelas duas regras. Go **não tem o
equivalente em produção**: existe `adrStatusIsNotAccepted()` (linha ~1281 de
`internal/validator/validator.go`) documentada como "helper canônico", mas ela só é
chamada pelos **testes** (`validator_test.go`) — nunca pelo código de produção. Em vez
disso, a mesma expressão booleana `strings.EqualFold(status, "Draft") ||
strings.EqualFold(status, "Proposed")` está **duplicada em 3 lugares**:
`adrStatusIsNotAccepted` (só testes), `adrDraftStatusForRule` (usado por
`blocked_by_draft_adr`), e inline dentro de `validateADRAcceptedWhenREQDone`
(`adr_accepted_when_req_done`). Hoje as três cópias são textualmente idênticas, então
não há bug funcional — mas é o mesmo padrão de risco documentado em
`vault/notes/deteccao-de-status-de-adr-divergencias-entre-clis-2026-08-01.md`: lógica
duplicada em paralelo diverge silenciosamente na próxima mudança (ex.: alguém adiciona
um status extra em só uma das três cópias).

O ponto verdadeiramente único e compartilhado em Go é `resolveAdrStatus()` (resolução
do valor bruto do status, frontmatter-first com fallback de cabeçalho) — as duas
regras convergem *aí*, não no nível booleano. O cenário de falsificação novo
(`scripts/check-gates-falsify.sh`, Cenário 27) neutraliza `resolveAdrStatus` (faz
sempre resolver `"Accepted"`) porque é o único ponto que afeta as duas regras
simultaneamente em Go — corromper qualquer uma das 3 cópias booleanas neutralizaria
só uma regra, não as duas.

**Recomendação para quem for tocar essa área**: se `adrStatusIsNotAccepted` for
promovido a helper de produção de fato (substituindo as 2 outras cópias), o Cenário 27
do Go também deve migrar seu alvo de corrupção para ela — não é uma mudança urgente
(comportamento correto hoje), mas é dívida a saldar na próxima vez que a regra for
tocada, para não haver 3 lugares para lembrar de atualizar.

## Prova de que o cenário novo tem poder de reprovação

`check-gates-falsify.sh` Cenário 27: para cada CLI, corrompe o ponto de resolução de
status compartilhado (Go: `resolveAdrStatus`; Node: `adrNotAcceptedStatusForRule`;
Python: `_adr_not_accepted`) para sempre resolver "aceito", roda `validate` contra o
mesmo projeto-fixture violador, e prova via `assert_lacks_pattern` (exige exit 0 E
ausência do diagnóstico) que as duas violações desaparecem. Testado manualmente: uma
corrupção NO-OP (sem efeito real) faz o cenário falhar corretamente (`FAIL
[falsify/...]: ciclo limpo saiu com 1, esperava 0`), confirmando que o harness não é
tautológico.

Relacionado: `vault/notes/deteccao-de-status-de-adr-divergencias-entre-clis-2026-08-01.md`,
`vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md`.

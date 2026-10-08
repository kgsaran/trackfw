---
status: Done
date: 2026-10-08
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-08-req-done-open-criteria-decompoe-o-numero-agregado-e-mostra-quantas-reqs-isentas-sao-herdadas-de-outro-repositorio.md"
---

# REQ: req_done_open_criteria decompoe o numero agregado e mostra quantas REQs isentas sao herdadas de outro repositorio

> Date: 2026-10-08 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

Issue #542 (@lourivalgarciajunior, 2026-10-08, binário 9.3.3). Num fork consumidor, o aviso agregado de
`req_done_open_criteria` (`reqDoneOpenCriteriaNotice`, `internal/validator/validator_req_done_criteria.go`) conta 22
REQs Done com critério aberto que são **herdadas do upstream** — copiadas antes de o fork decidir não importar a
governança. Para REQ herdada, as três saídas da regra estão fechadas: marcar `[x]` exigiria provar entrega alheia,
`Caducou:` afirmaria caducidade de critério alheio, e remover/mover é vetado pelo fork. O aviso fica permanente e sem
remédio — e numa varredura de pendências quase virou trabalho de fechar critérios de outro repositório.

Mesmo princípio das #530 e #538: aviso sem remédio deixa de ser lido e leva outros avisos com ele.

**Decisão do usuário (2026-10-08): saída 2 da issue — só decompor o número.** A linha agregada passa a dizer quantas
das REQs isentas/cobradas são herdadas de outro repositório, para o leitor separar o que é dele. Nenhuma decisão da
regra muda (quem é cobrado, quem é isento, severidade).

**Escopo negativo:** não isenta REQ herdada (saída 1 da issue, recusada por ora); não muda severidade, cutoff nem a
contagem `enforced`; não muda outras regras; não importa nem remove REQs de forks.

## Acceptance Criteria
- [x] AC1 — Wave 0 decide o discriminante de "herdada" com medição: derivado (o arquivo existe no ramo padrão do remote `upstream`, como o `check-inherited-req.sh` do fork) e/ou declarado (frontmatter `upstream_origin`, que hoje é invenção do fork). Preferência: derivado, sem conceito novo no frontmatter; custo de git medido (uma chamada por validate, não por REQ); sem remote `upstream` → linha idêntica à atual
      ✅ Evidência: `docs/seguranca/2026-10-08-wave0-req-done-open-criteria-herdadas.md` — derivado por basename contra o `req_dir` do upstream; ~37 ms; 28/28 concordância com `upstream_origin` do fork.
- [x] AC2 — A linha agregada reporta o recorte de herdadas (ex.: `22 ... exempt (22 inherited from upstream)`), e a mesma informação sai no `--json`
      ✅ Evidência: PR #545 — fork do relator: `22 ... (22 inherited from upstream/main)`; `--json` leva o recorte em `warnings[].message`.
- [x] AC3 — Repositório sem remote `upstream` (este, por exemplo): saída byte-idêntica à atual — teste
      ✅ Evidência: `TestReqDoneOpenCriteria_AC3_NoUpstreamByteIdentical`; red-team comparou com o binário da main (texto e JSON, diff zero).
- [x] AC4 — Teste com repo temporário com remote `upstream` contendo parte das REQs: o recorte bate com as REQs que existem no upstream, nas duas direções (REQ local não conta como herdada; REQ do upstream conta)
      ✅ Evidência: `TestReqDoneOpenCriteria_AC4a_InheritedByBasename`, `_AC4b_T1_UpstreamEqualsOrigin`, `_AC4c_InheritedPostCutoffInEnforced`.
- [x] AC5 — Comentário na #542 com o resultado; `docs/cli-parity.md` atualizado se documenta a linha
      ✅ Evidência: comentário na #542 com a medição; `docs/cli-parity.md` atualizado no PR #545.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-10-04-criterio-de-aceite-caducado-e-marcado-como-tal-com-justificativa-e-regras-novas-sobre-artefato-antigo-tem-corte-por-data-declarado.md (regra e linha agregada; esta REQ só decompõe a linha, sem decisão nova)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-08-req-done-open-criteria-decompoe-o-numero-agregado-e-mostra-quantas-reqs-isentas-sao-herdadas-de-outro-repositorio.md

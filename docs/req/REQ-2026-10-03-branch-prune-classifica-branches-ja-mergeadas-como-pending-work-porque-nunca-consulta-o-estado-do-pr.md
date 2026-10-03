---
status: Done
date: 2026-10-03
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-03-branch-prune-usa-o-estado-do-pr-no-forge-como-sinal-de-integracao-com-o-conteudo-como-fallback-declarado.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-03-branch-prune-classifica-branches-ja-mergeadas-como-pending-work-porque-nunca-consulta-o-estado-do-pr.md"
---

# REQ: branch prune classifica branches já mergeadas como pending work porque nunca consulta o estado do PR

> Date: 2026-10-03 | Status: Done
| GitHub Issue: #481

## Motivation

O `prune` decide integração só por conteúdo e, com squash-merge e a `main` seguindo em frente, não
distingue "entrou" de "nunca entrou". Medido neste repositório: de 52 branches locais, 48 têm PR
mergeado cujo head é o tip local, e o `prune` libera 1. Decisão (ADR ligado): o estado do PR no forge
passa a ser o sinal principal, e o conteúdo fica como fallback declarado quando não há forge.

## Acceptance Criteria

- [x] **AC1** — 🔴 **Wave 0:** threat model do que pode fazer o `prune` **apagar trabalho não
  integrado**: PR de fork com o mesmo nome de branch; nome de branch reaproveitado depois de um PR
  antigo mergeado; saída do `gh` adulterada ou parcial (paginação); head do PR ausente localmente;
  branch sem upstream. Parecer em `docs/seguranca/`.
      ✅ Evidência: `docs/seguranca/2026-10-03-wave0-prune-estado-do-pr.md`: 11 cenários, 8 ajustes incorporados ao ADR como D5
- [x] **AC2** — Os casos da D1, com testes: PR aberto (mesmo com MERGED antigo de mesmo nome) → `keep`;
  PR mergeado contendo o tip → `delete`; commits além do head → `keep`; tip divergente → `keep`;
  PR fechado sem merge → `review`; sem PR e sem upstream → `keep` mesmo com conteúdo
  idêntico; sem PR com upstream → veredito de hoje.
      ✅ Evidência: testes `TestD1_Case0..Case5`, `TestD1_Case2b`, `TestD1_MultiMerged_SecondContainsTip_Delete`, `TestD1_RealGit_Case1_And_Case2`, `TestAJ1_NeverPushed_MergedPRInNonMainBase_NoPRNeverPushed`, com mordida provada
- [x] **AC3** — PR `MERGED` de **fork** com o mesmo nome de branch **não** torna a branch local
  apagável.
      ✅ Evidência: `TestAC3_ForkPR_MERGED_NeverDelete` (filtro `isCrossRepository`) e `TestA2_MergedPR_WrongBase_NotDelete`
- [x] **AC4** — Sem `gh` (ou não autenticado, ou consulta com erro): o veredito é igual ao de hoje e o
  relatório traz uma linha nomeando a causa.
      ✅ Evidência: `TestAC4_Degradation_CauseLineAndContentHeuristic`; medido sem `gh` no PATH: linha `Note:` com a causa e veredito de hoje
- [x] **AC5** — Uma única consulta ao forge por execução, independentemente do número de branches. Resposta possivelmente truncada
  (itens == limite) é tratada como incompleta: branch ausente dela fica no veredito de hoje, e não "sem PR".
      ✅ Evidência: `TestAC5_ForgeQueriedExactlyOnce` e `TestAC5_TruncatedResponse_BlocksForgeSignalDeletes` (itens == 3000 → nenhum delete pelo sinal de PR)
- [x] **AC6** — O aviso do `push`/`ship` (`detectPendingSquashMerges`) usa a mesma avaliação: branch
  com PR mergeado contendo o tip deixa de gerar aviso.
      ✅ Evidência: `TestAC6_DetectPendingSquashMerges_SilencesForMergedPR`; medido: o aviso sobre `fix/afirma-contencao-antes-de-escrever` (PR #397, head == tip) some com o binário da branch
- [x] **AC7** — Medição de volta neste repositório: o relatório do `prune` (dry-run) classifica as
  branches como a tabela do ADR prevê, sem nenhuma branch com trabalho não integrado em `delete`.
      ✅ Evidência: binário da branch, dry-run: 49 delete + 3 keep "commits after the merged PR" (#415, #465, #503), igual à tabela do ADR
- [x] **AC8** — `docs/cli-parity.md` atualizado (contrato do `prune` e do aviso), com a anotação
  `trackfw-contract`.
      ✅ Evidência: `docs/cli-parity.md` seção do `branch prune` (ordem D1, caso 4 sem upstream, degradação do prune e do push/ship)
- [x] **AC9** — Cada teste novo declara a conclusão que afirma e reprova sem a correção.
      ✅ Evidência: uma frase e uma prova de mordida por teste novo, nos relatórios dos ML-1A e ML-1B, com os nomes conferidos via `go test -list`
- [x] **AC10** — `make quality` EXIT=0 (máquina ociosa) e CI verde, inclusive `windows-full-suites`.
      ✅ Evidência: `make quality` EXIT=0 (3001 linhas, falsify 347 OK / 0 FAIL); CI do PR #515: 20/20 pass, inclusive `windows-full-suites`

## Negative scope

- **Apagar branch remota:** continua fora (REQ-2026-08-18).
- **Forges além do GitHub:** degradam (D2); listagem por `glab`/`az` é trabalho futuro.
- **Mudar o padrão `--dry-run`** ou a regra de que `review` nunca é apagado sozinho.

## Linked ADR
ADR: docs/adr/ADR-2026-10-03-branch-prune-usa-o-estado-do-pr-no-forge-como-sinal-de-integracao-com-o-conteudo-como-fallback-declarado.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-03-branch-prune-classifica-branches-ja-mergeadas-como-pending-work-porque-nunca-consulta-o-estado-do-pr.md

---
status: Open
date: 2026-10-01
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-01-estado-que-governa-a-branch-criacao-exige-wip-branch-existente-aceita-blocked-e-done-so-pelo-vinculo-ou-pela-propria-branch.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-01-o-estado-que-governa-a-branch-branch-new-aceita-done-por-inferencia-frouxa-e-blocked-nao-governa.md"
---

# REQ: o estado que governa a branch — branch new aceita done por inferência frouxa e blocked não governa

> Date: 2026-10-01 | Status: Open
| GitHub Issue: #494, #490

## Motivation

Os dois issues têm o mesmo mecanismo: **o conjunto de estados que o gate branch↔roadmap consulta**.
Hoje ele é `wip/ ∪ done/` nas quatro portas (`branch new`, `commit`, `validate`, e `push`/`ship` via
`CheckShipGovernance`). Ele está largo demais para criar branch e estreito demais para branch
existente.

- **#494:** `branch new` aceita um roadmap de `done/` casado por inferência. Com `done/` grande, um
  slug novo do mesmo domínio casa um roadmap concluído **de outro assunto**, e a branch nasce sem nada
  em `wip/`. Medido no acervo: de 217 branches `feat|fix|refactor` históricas, 22 casam `done/` só
  por sobreposição de tokens e **126 casam mais de um** roadmap concluído. A frouxidão está nas duas
  pernas da relação, não só na de tokens.
- **#490:** `roadmap move … blocked` tira o único roadmap que governa a branch do conjunto. Reproduzido
  com o binário da `main` `e104a7f7`: `commit` rc=1, `validate` reprova, `push --dry-run` reprova, e o
  vínculo escrito vira "stale". O bloqueio legítimo não pode ser registrado, e a pasta, que é a fonte
  de verdade do estado, passa a mentir.

Decisão (ADR vinculado): criação consulta só `wip/`. Branch existente aceita `wip/ ∪ blocked/` por
vínculo ou inferência, e `done/` só pelo vínculo escrito ou se **a própria branch** moveu o roadmap
para `done/` (ausente de `done/` na ponta da base).

## Acceptance Criteria

- [x] **AC1** — 🔴 **Wave 0:** threat model do sinal "movido por esta branch" (ponta da base via
  `deriveOriginDefaultBranch` + `git ls-tree`) e da degradação do D3: quem obtém aceitação indevida
  sem quebrar regra escrita, e com que entrada (`trackfw.yaml` com `roadmap_dir` hostil, ref de
  `origin` forjada localmente, nome de arquivo com caractere especial no `ls-tree`). Parecer em
  `docs/seguranca/`.
      ✅ Evidência: parecer da Wave 0 em `docs/seguranca/`
- [x] **AC2** — **#494 fechado na criação:** em repositório temporário com os roadmaps de `done/`
  desta árvore e `wip/` vazio, `trackfw branch new fix/barrier-executa-cada-linha-do-bloco-de-gates`
  sai rc≠0, **não** cria a branch, e a mensagem nomeia o(s) roadmap(s) de `done/` casado(s) e
  `trackfw roadmap move <nome> wip`. Controle: o mesmo slug com um roadmap casando em `wip/` cria a
  branch.
      ✅ Evidência: `TestBranchStateE2E_AC2_DoneOnlyBlocksCreation` (binário, 211 roadmaps reais); reprova na `main`
- [x] **AC3** — `RecordBranchLink` grava só com exatamente um casamento em `wip/`. Com um casamento em
  `wip/` e outro em `done/`, o vínculo aponta para o de `wip/`.
      ✅ Evidência: `TestRecordBranchLink_DoneOnlyNoLink`, `TestRecordBranchLink_WipAndDonePicksWip`
- [x] **AC4** — **#490 fechado:** em repositório temporário, `branch new` → `roadmap move <x>
  blocked` → `trackfw commit` rc=0, `trackfw validate` sem violação de `branch_has_wip_roadmap` e sem
  `branch_link_stale`, `trackfw push --dry-run` com `Governance: OK`. Vale nos dois braços: **com**
  vínculo escrito e **sem** ele (só inferência).
      ✅ Evidência: `TestBranchStateE2E_AC4_BlockedRoadmapGoverns_{WithLink,NoLink}`; reprovam na `main`
- [x] **AC5** — **`done/` em branch existente:** (a) roadmap movido para `done/` **por esta branch**
  (ausente de `done/` em `origin/main`) e casando o slug → `commit`/`validate` passam, com e sem
  vínculo (é o caso da Definition of Done); (b) roadmap já em `done/` em `origin/main`, casando só por
  inferência e sem vínculo → `commit` rc≠0 e `validate` com violação.
      ✅ Evidência: `TestBranchStateE2E_AC5a_*` (passa) e `_AC5b_DoneAlreadyInBaseBlocks` (reprova na `main`)
- [x] **AC6** — **D3:** sem `origin` (ou sem ref default resolvível), o roadmap de `done/` casando por inferência passa como hoje
  **e** emite `branch_done_scope_unverifiable` com a causa. É aviso, nunca violação.
      ✅ Evidência: `TestBranchStateE2E_AC6_NoPushOriginDegrades`; `TestShip_GovernanceDegraded_PrintsDegradedNotOK`
- [x] **AC7** — Mensagens em fonte única no `validator`, parametrizadas por consumidor. Nenhum
  literal de mensagem duplicado em `internal/commands/`. `scripts/check-validate-rule-pins.sh`
  (PIN3/PIN4) e `docs/cli-parity.md` atualizados e verdes.
      ✅ Evidência: `check-validate-rule-pins.sh` 30/30; `grep "nor done/"` em `commands/` só acha `barrier.go`
- [x] **AC8** — **A relação não muda (D6):** `git diff` de `MatchRoadmapsForBranchSlug`,
  `branchRoadmapTokens`, `roadmapContentSlug` e `sharedTokenCount` vazio, e o gate do corpus do
  ADR-2026-09-26 (AC14) verde sem mudança de veredito.
      ✅ Evidência: grep do D6 no diff vazio; `check-roadmap-slug-matching.sh` 205×201 sem divergência (ML-3B)
- [x] **AC9** — Todo teste novo declara, no relatório do ML, qual conclusão afirma. O teste da
  `REQ-2026-08-04` que afirmava "match em `done/` cria a branch" é **invertido**, não apagado.
      ✅ Evidência: uma frase por teste em todos os relatórios; 3 testes decorativos removidos na auditoria
- [ ] **AC10** — `make quality` EXIT=0 e CI do PR verde, inclusive `windows-full-suites`.
- [x] **AC11** — (A2 da Wave 0) sem `origin`, `push --dry-run` e `ship --dry-run` **não** imprimem
  `Governance: OK`; imprimem o aviso `branch_done_scope_unverifiable`.
      ✅ Evidência: `TestBranchStateE2E_AC6_NoPushOriginDegrades` (push) e `TestShip_GovernanceDegraded_PrintsDegradedNotOK` (sabotagem reprova)
- [x] **AC12** — (A1 da Wave 0) `mdBasenamesInGitTree` e `auditsurface.gitLsTree` usam `ls-tree -z`.
  O teste com nome acentuado falha em `e104a7f7` e passa na branch.
      ✅ Evidência: `TestGitLsTree_AccentedFilename` e `TestBranchStateE2E_AC12_*` reprovam em `e104a7f7`; `--literal-pathspecs` acrescentado (RN1)

## Escopo absorvido durante a execução

- **A1/A2 do threat model da Wave 0** (`docs/seguranca/2026-10-01-wave0-estado-que-governa-a-branch.md`).
  O aviso do D3 seria descartado por `CheckShipGovernance`. A leitura de árvore sem `-z` erra nome
  acentuado nos dois leitores que já existem. Entram aqui pela Regra Dura: o sinal do D2 depende dos
  dois.

## Negative scope

- **A relação de casamento** (braços substring/tokens, limiar 2, token mínimo 3): intocada (D6). A
  frouxidão residual contra `wip/ ∪ blocked/` fica declarada no ADR.
- **`resolveBarrierRoadmap`** (`barrier.go`): resolve roadmap **por nome**, não por branch. O texto
  "wip/ nor done/" dele é outro contrato.
- **#481** (`branch prune` classifica branch mergeada como pendente): outra decisão de branch, outro
  mecanismo (compara conteúdo contra a ponta, não consulta o PR).
- **`backlog/ROADMAP-2026-09-12-branch-new-cria-da-base-correta-e-o-guard-fecha-o-checkout-cru`:**
  trata da base de onde a branch nasce e do guard de `checkout` cru, não do conjunto de estados.
- **Hook `git-branch-guard`**, exceção doc-only do `ship`, regras `wip_has_req`/`blocked_has_req`:
  não mudam.
- **PR contra base que não é a default de `origin`:** resíduo declarado no ADR.

## Linked ADR
ADR: docs/adr/ADR-2026-10-01-estado-que-governa-a-branch-criacao-exige-wip-branch-existente-aceita-blocked-e-done-so-pelo-vinculo-ou-pela-propria-branch.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-01-o-estado-que-governa-a-branch-branch-new-aceita-done-por-inferencia-frouxa-e-blocked-nao-governa.md

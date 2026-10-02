---
status: wip
date: 2026-10-01
req: "docs/req/REQ-2026-10-01-o-estado-que-governa-a-branch-branch-new-aceita-done-por-inferencia-frouxa-e-blocked-nao-governa.md"
squad: "hades-tf, apolo-tf, artemis-tf, hefesto-tf"
---

# Roadmap: o estado que governa a branch — branch new aceita done por inferência frouxa e blocked não governa

> Created: 2026-10-01 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-01-o-estado-que-governa-a-branch-branch-new-aceita-done-por-inferencia-frouxa-e-blocked-nao-governa.md
ADR: docs/adr/ADR-2026-10-01-estado-que-governa-a-branch-criacao-exige-wip-branch-existente-aceita-blocked-e-done-so-pelo-vinculo-ou-pela-propria-branch.md
Issues: #494, #490 (o PR fecha os dois).

Mapa do código (lido em `e104a7f7`):

| sítio | hoje | depois |
|---|---|---|
| `internal/commands/branch.go` `runBranchNew` (~:155) | `matchSlug(slug, wip, done)` | só `wip` (D1); mensagem nomeia casamentos em `done/` |
| `internal/validator/branchlink.go` `RecordBranchLink` | 1 casamento em `wip∪done` | 1 casamento em `wip` (D1) |
| `internal/validator/branchlink.go` `BranchLinkFor` | `InScope` em `wip∪done` | `wip∪blocked∪done` (D4) |
| `internal/validator/branchlink.go` `ResolveBranchRoadmap` | inferência `wip∪done` | `wip∪blocked` + `done` restrito (D2/D3) |
| `internal/validator/validator.go` `validateBranchHasWIPRoadmap` (~:3934) | `ResolveBranchRoadmap(wip, done)` | resolução nova; usada por `CheckShipGovernance` → `push`/`ship` |
| `internal/commands/commit.go` (~:356) | `matchSlug` + vínculo, `wip∪done` | mesma resolução do `validate` (fonte única) |
| `BranchGovernanceOrientation` / `BranchNoMatchingRoadmapMessage` / `BranchLinkStaleWarning` | texto "wip/ nor done/" | parametrizado por consumidor (D5) |
| `internal/validator/validator.go` `CheckShipGovernance` (~:4113) / `GovernanceViolation` | descarta os warnings de `validateBranchHasWIPRoadmap` | `Warnings` propagados e impressos por `push`/`ship` (A2 do ML-0A) |
| `internal/validator/validator.go` `mdBasenamesInGitTree` (~:404), `internal/auditsurface/auditsurface.go` `gitLsTree` (~:287) | `ls-tree --name-only` sem `-z`: nome não-ASCII sai citado | `-z` + split em NUL (A1 do ML-0A) |
| `scripts/check-validate-rule-pins.sh` PIN3/PIN4, `docs/cli-parity.md` (:1292, :1858, :4259-4260) | texto antigo | texto novo |

## Acceptance Criteria
- [x] AC1 — Wave 0 auditada (parecer de segurança em `docs/seguranca/`)
      ✅ Evidência: `docs/seguranca/2026-10-01-wave0-estado-que-governa-a-branch.md`, APROVA COM AJUSTES (A1–A3 absorvidos no ML-1A/ML-1B/ML-1C e no ADR)
- [x] AC2 — #494 fechado na criação (binário, acervo real, `wip/` vazio)
      ✅ `TestBranchStateE2E_AC2_DoneOnlyBlocksCreation` (binário, 211 roadmaps reais); reprova na `main`
- [x] AC3 — `RecordBranchLink` só com casamento único em `wip/`
      ✅ `TestRecordBranchLink_DoneOnlyNoLink`, `TestRecordBranchLink_WipAndDonePicksWip`
- [x] AC4 — #490 fechado: commit/validate/push passam após `roadmap move … blocked`, com e sem vínculo
      ✅ `TestBranchStateE2E_AC4_BlockedRoadmapGoverns_{WithLink,NoLink}`; reprovam na `main`
- [x] AC5 — `done/` em branch existente: aceita o roadmap movido pela própria branch e recusa o alheio
      ✅ `TestBranchStateE2E_AC5a_*` (passa) e `_AC5b_DoneAlreadyInBaseBlocks` (reprova na `main`)
- [x] AC6 — base inverificável → aviso `branch_done_scope_unverifiable`, nunca violação
      ✅ `TestBranchStateE2E_AC6_NoPushOriginDegrades`; `TestShip_GovernanceDegraded_PrintsDegradedNotOK`
- [x] AC7 — mensagens em fonte única; pinos e `cli-parity.md` atualizados
      ✅ `check-validate-rule-pins.sh` 30/30; `grep "nor done/"` em `commands/` só acha `barrier.go`
- [x] AC8 — relação de casamento intocada; gate do corpus sem mudança de veredito
      ✅ grep do D6 no diff vazio; `check-roadmap-slug-matching.sh` 205×201 sem divergência (ML-3B)
- [x] AC9 — teste novo declara o que afirma; teste da REQ-2026-08-04 invertido
      ✅ uma frase por teste em todos os relatórios; 3 testes decorativos removidos na auditoria
- [ ] AC10 — `make quality` EXIT=0 e CI verde
      ⏳ Local: make quality no HEAD final passou todas as etapas até a falsificação; a falsificação, rodada em grupos (o paralelo de 8 pendurou 2x na máquina), deu 347 OK / 0 FAIL — igual a 57aed5b4. Falta o CI do PR.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do conjunto de estados e do sinal "movido por esta branch"
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-01-wave0-estado-que-governa-a-branch.md` (único arquivo escrito)
**Actions:**
1. **Completude da enumeração:** além dos sítios da tabela do Context, procure qualquer outro consumidor
   de `ResolveWIPDirs`/`ResolveDoneDirs`/`MatchRoadmapsForBranchSlug`/`BranchLinkFor` que decida se uma
   branch é governada (`grep -rn` em `internal/`, `scripts/`, geradores de hook/CLAUDE.md em
   `internal/generators/`). Diga se a lista está fechada ou o que falta.
2. **Threat model:** quem obtém "branch governada" sem roadmap legítimo, sem quebrar regra escrita?
   Mínimo: (a) ref `origin/main` forjada ou desatualizada localmente (`git update-ref`), (b)
   `roadmap_dir` hostil no `trackfw.yaml` (caminho com `..`, absoluto, com espaço, iniciando com `-`)
   chegando ao `git ls-tree`, (c) nome de roadmap com caractere especial/quoting do `ls-tree` (use
   `-z`?), (d) forçar a degradação do D3 de propósito (remover `origin`) para obter o caminho frouxo,
   (e) mover para `done/` um roadmap alheio que casa o slug para "reivindicá-lo".
3. **Alvos de falsificação nas duas direções** por sítio: o que quebra se regredir para frouxo e o que
   quebra se regredir para restrito (por exemplo, a Definition of Done volta a travar).
4. **Resíduo declarado:** o que o desenho aceita não cobrir. Diga se o D3 (fail-open com aviso) é
   aceitável ou se algum portão (`push`, que é hard gate) deveria falhar fechado.
**Acceptance criteria:**
- [x] As quatro seções respondidas com evidência (comando + saída), não asserção de uma linha
- [x] Veredito explícito: APROVA / APROVA COM AJUSTES (listar) / REPROVA
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-01-wave0-estado-que-governa-a-branch.md
grep -q "Veredito" docs/seguranca/2026-10-01-wave0-estado-que-governa-a-branch.md
```

## Wave 1 — Núcleo no validator (sequencial: ML-1B consome a API do ML-1A)
> Dependencies: Wave 0 auditada

### ML-1A — Resolução por consumidor no `validator`
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/validator/branchlink.go`, `internal/validator/validator.go`, testes em
`internal/validator/*_test.go`
**Actions:**
1. Exportar `ResolveBlockedDirs(cfg)` sobre o `resolveStateDirs(cfg, "blocked")` que já existe.
2. `BranchLinkFor`: escopo `wip∪blocked∪done` (D4). `BranchLinkStaleWarning`: o texto nomeia os três
   estados.
3. `RecordBranchLink`: resolve só contra `wip` (D1).
4. Resolução de **branch existente** (D2), substituindo o corpo de `ResolveBranchRoadmap` (mesma
   assinatura pública ou uma nova; nenhum consumidor reimplementa): vínculo em escopo → governa;
   inferência em `wip∪blocked` → governa; inferência em `done` → governa só se o arquivo **não** está
   em `done/` na ponta de `deriveOriginDefaultBranch()` (`git ls-tree -z --name-only <ref> --
   <done_dir>/`). 🔴 **Reusar `mdBasenamesInGitTree`** (validator.go ~:404), não escrever outro
   leitor de árvore; corrigi-lo para `ls-tree -z` + `bytes.Split(out, []byte{0})` (A1 do ML-0A:
   com `core.quotepath` padrão, nome não-ASCII sai citado e `filepath.Base` erra → o roadmap parece
   ausente da base → aceitação frouxa). Teste com nome de roadmap acentuado. Para `by_agent`, cada
   `done/` resolvido é verificado.
5. D3: ref não resolvível ou `ls-tree` com rc≠0 → aceita como hoje e acrescenta aviso
   `branch_done_scope_unverifiable: …` com ref e rc em `Warnings`. Nunca violação.
6. A2 do ML-0A: `GovernanceViolation` ganha `Warnings []string`; `CheckShipGovernance` propaga o
   segundo retorno de `validateBranchHasWIPRoadmap` (hoje descartado com `_`). Sem isso o aviso do D3
   nunca chega a `push`/`ship`.
7. D5: `BranchGovernanceOrientation`/`BranchNoMatchingRoadmapMessage` ganham o consumidor como
   parâmetro (criação → "in wip/"; existente → "in wip/, blocked/ nor done/"). Na criação, se houver
   casamentos em `done/`, a mensagem os nomeia (até 3, ordenados) e orienta `trackfw roadmap move
   <nome> wip`.
8. 🔴 **Proibido** tocar `MatchRoadmapsForBranchSlug`, `branchRoadmapTokens`, `roadmapContentSlug`,
   `sharedTokenCount` e as duas constantes (D6).
**Acceptance criteria:**
- [x] Testes com git real em diretório temporário: blocked governa (com e sem vínculo); `done/` movido
      pela branch governa; `done/` presente na base não governa; sem `origin` → aviso, sem violação;
      vínculo para `blocked/` não é stale
- [x] `git diff e104a7f7 -- internal/validator/validator.go` não toca as funções do item 7
- [x] Relatório: uma frase por teste novo dizendo qual conclusão ele afirma
      ✅ Auditoria (arquiteto): 12 testes rodados por nome, todos PASS; grep do D6 vazio; leitor `-z` é
      `mdBasenamesInGitTreeWithError`, com `mdBasenamesInGitTree` delegando a ele (A1 cobre os chamadores
      antigos). Funções antigas de mensagem ficam como Deprecated até o ML-1B.
**Gates da wave:**
```bash
go build ./...
go test ./internal/validator/ -count=1
```

### ML-1B — Consumidores: `branch new`, `commit`, pinos e contrato
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/commands/branch.go`, `internal/commands/commit.go`,
`internal/commands/push.go`, `internal/commands/ship.go`, testes em
`internal/commands/*_test.go`, `scripts/check-validate-rule-pins.sh`, `docs/cli-parity.md`
**Actions:**
1. `runBranchNew`: guard contra `wip` só (D1). Os casamentos em `done/` só alimentam a mensagem.
2. `push.go`/`ship.go`: imprimir `GovernanceViolation.Warnings` mesmo com `Missing` vazio, como
   `Governance: degraded: <aviso>` em vez de `Governance: OK` (A2 do ML-0A). Teste: sem `origin`,
   `push --dry-run` não imprime `Governance: OK` e imprime `branch_done_scope_unverifiable`.
3. `commit.go`: substituir o par `matchSlug` + `branchLink` pela resolução de branch existente do
   ML-1A (fonte única com o `validate`). Imprimir os `Warnings` dela.
4. Inverter o teste herdado da REQ-2026-08-04 ("match em `done/` cria a branch") para "match só em
   `done/` bloqueia e nomeia o roadmap". Não apagar.
5. `check-validate-rule-pins.sh` PIN3/PIN4 e os marcadores `BHR_MARKER`/`MARKER_*`: textos novos.
7. **Corretivo do ML-1C:** apagar `TestGitLsTree_AccentedFilename_OldBehavior` e o helper
   `splitNewlines` de `internal/auditsurface/gitlstree_test.go` (testa uma cópia da função antiga, não
   o produto). `TestGitLsTree_AccentedFilename` fica.
6. `docs/cli-parity.md` (:1292, :1858, :4259-4260 e onde mais o grep achar): contrato novo por
   consumidor, o sinal "movido por esta branch" e o aviso `branch_done_scope_unverifiable`.
**Acceptance criteria:**
- [x] `grep -rn "nor done/" internal/commands/*.go` (fora de `_test`) só retorna o `barrier.go` (fora
      de escopo)
- [x] `make build && GO_BIN=bin/trackfw scripts/check-validate-rule-pins.sh` exit 0
- [x] Relatório: uma frase por teste novo
      ✅ Auditoria (arquiteto): 5 testes por nome PASS; 30/30 pinos; grep só acha `barrier.go`; #490 e #494 reproduzidos com `bin/trackfw` e corrigidos.
**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ ./internal/validator/ -count=1
make build
GO_BIN=bin/trackfw scripts/check-validate-rule-pins.sh
```

### ML-1C — `gitLsTree` do `auditsurface` com `-z` (mesma causa do A1)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/auditsurface/auditsurface.go` (`gitLsTree` ~:287), teste em
`internal/auditsurface/*_test.go`
**Paralelismo:** arquivos disjuntos do ML-1A; pode rodar junto dele. Não toca `internal/validator/`.
**Actions:** `git ls-tree -r -z --name-only` + split em NUL; teste com caminho acentuado em
repositório git temporário (falha sem `-z`, passa com ele).
**Acceptance criteria:**
- [x] Teste falha no código de `e104a7f7` e passa no novo (provar as duas)
      ✅ Medido pelo arquiteto: `TestGitLsTree_AccentedFilename` com o `auditsurface.go` de `e104a7f7`
      via `go test -overlay` → FAIL (`"scripts/a\303\247\303\243o.md"`); com o novo → PASS.
- [x] Relatório: uma frase por teste novo
      ⚠️ `TestGitLsTree_AccentedFilename_OldBehavior` testa uma cópia da função antiga e o git, não o
      produto: passa para sempre e não afirma conclusão do ML. Removido no ML-1B (corretivo).
**Gates da wave:**
```bash
go build ./...
go test ./internal/auditsurface/ -count=1
```

### ML-1D — Corretivo: a dica de `done/` não pode mandar reabrir roadmap alheio
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Origem:** auditoria do ML-1B. Com o binário da branch, `branch new fix/barrier-executa-cada-linha-do-bloco-de-gates`
bloqueia (correto), mas imprime `trackfw roadmap move <X> wip` pronto para cada um dos 3 casados, e 2
deles são de outro assunto. Um agente que obedece a orientação reabre o roadmap errado e reproduz o
#494 por outro caminho.
**Files affected:** `internal/validator/validator.go` (`doneMatchesHint` ~:4054), testes que afirmam o
texto antigo (`internal/validator/*_test.go`, `internal/commands/branch_test.go`), `docs/cli-parity.md`
se citar o texto.
**Actions:** a dica passa a ser exatamente:
`(similar names in done/ — concluded roadmaps do not govern a new branch: <A>, <B>, <C>[, e mais N]. Only if this branch reopens one of them: trackfw roadmap move <name> wip)`
Sem uma linha de comando pronta por roadmap casado.
**Acceptance criteria:**
- [x] Nenhuma saída de `branch new` contém `trackfw roadmap move ROADMAP-` (comando com nome concreto)
- [x] Testes que afirmam a dica atualizados; uma frase por teste alterado
      ✅ Auditoria: `bin/trackfw branch new fix/barrier-executa-…` no acervo real → 0 linhas com `trackfw roadmap move ROADMAP-`.
**Gates da wave:**
```bash
go build ./...
go test ./internal/validator/ ./internal/commands/ -count=1
```

## Wave 2 — Ponta a ponta com o binário
> Dependencies: Wave 1 auditada

### ML-2A — Cenários dos issues com o binário real
**Status:** ✅ Concluído
**Squad:** artemis-tf
**Files affected:** teste novo em `internal/commands/` (ou o harness ponta a ponta que já exista, a
localizar e citar), fixtures em `testdata/` do mesmo pacote
**Actions:**
1. AC2: repositório temporário com cópia dos roadmaps de `done/` da árvore, `wip/` vazio →
   `branch new fix/barrier-executa-cada-linha-do-bloco-de-gates` rc≠0, branch inexistente, mensagem
   com o roadmap de `done/` e `roadmap move`. Controle positivo com roadmap em `wip/`.
2. AC4: `branch new` → `roadmap move <x> blocked` → `commit` rc=0, `validate` limpo, `push --dry-run`
   com `Governance: OK`. Braço com vínculo e braço sem o arquivo de vínculo.
3. AC5(a)/(b) e AC6 com `origin` local (repositório bare) e sem `origin`.
4. Reexecutar a medição do Context do ADR (217 branches × `done/`) com a resolução de criação e
   reportar o número (informativo).
**Acceptance criteria:**
- [x] Cada cenário falha com o binário da `main` `e104a7f7` e passa com o da branch (provar as duas)
      ✅ Rodado pelo arquiteto: branch 8/8 PASS; `main` 6 FAIL (AC2, AC4×2, AC5b, AC6, AC12) e 2 PASS
      (AC5a×2, esperado: a `main` já aceitava `done/` frouxo). Medição: 0 de 217 branches históricas
      criáveis contra `done/` com `wip/` vazio.
- [x] Relatório: uma frase por teste novo
**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -count=1
```

### ML-2B — Corretivo: `ship` degradado sem teste; teste informativo sai da suíte
**Status:** ✅ Concluído
**Squad:** artemis-tf
**Origem:** auditoria do ML-2A. O AC11 exige `ship` além de `push`, mas só o `push` tem teste do caminho
`Governance: degraded`. E `TestBranchStateE2E_InformativeMeasure217Branches` é uma medição, não uma
asserção (sempre SKIP na suíte; o próprio comentário dele diz isso).
**Files affected:** `internal/commands/ship_test.go`, `internal/commands/branch_state_e2e_test.go`
**Actions:** (1) teste de `runShip` com `checkGovernance` devolvendo `Missing` vazio e um warning:
a saída contém `Governance: degraded:` e não contém `Governance: OK`; (2) remover
`TestBranchStateE2E_InformativeMeasure217Branches` e o que só ele usa.
**Acceptance criteria:**
- [x] Teste novo falha se a linha `Governance: degraded` do `ship.go` for trocada por `Governance: OK` (provar)
      ✅ Sabotagem confirmada: `for _, w := range gv.Warnings { fmt.Fprintf(deps.out, "Governance: OK\n") }` → FAIL em 3 asserções; restaurado → PASS.
- [x] Relatório: uma frase por teste novo
      ✅ `TestShip_GovernanceDegraded_PrintsDegradedNotOK` afirma que D3 do ADR-2026-10-01 é honrado no `ship`: governança degradada nunca bloqueia a execução, imprime `Governance: degraded:` e nunca `Governance: OK`.
**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -count=1
```

## Wave 3 — Revisão independente (paralela: só leitura + parecer próprio)
> Dependencies: Wave 2 auditada

### ML-3A — Revisão de segurança
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-01-wave3-revisao-estado-que-governa-a-branch.md`
**Actions:** confrontar a implementação com os alvos do ML-0A; reimplementar a partir da leitura os
vetores (a)–(e) contra o binário da branch.
**Acceptance criteria:**
- [x] Veredito explícito
      ✅ APROVA COM AJUSTES: RN1 (`--literal-pathspecs`) absorvido no ML-3C, não em follow-up (Regra Dura).

### ML-3B — Revisão de qualidade e gate completo
**Status:** ✅ Concluído
**Squad:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-01-revisao-estado-que-governa-a-branch.md`
**Actions:** fonte única (nenhuma resolução reimplementada em `commands/`), D6 intacto, `make quality`
completo.
**Acceptance criteria:**
- [x] `make quality` EXIT=0 (saída completa, sem `| tail`)
      ✅ Log conferido pelo arquiteto: falsify `347 OK, 0 FAIL`; as linhas `FAIL` do log são braços negativos dos autotestes de gate em diretório temporário. Corpus AC14: 205×201 sem divergência.
- [x] Veredito explícito
      ✅ APROVA COM AJUSTES: 5 comentários/ajudas desatualizados → ML-3C.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-01-wave3-revisao-estado-que-governa-a-branch.md
test -s docs/qualidade/2026-10-01-revisao-estado-que-governa-a-branch.md
```

### ML-3C — Corretivo da Wave 3: `--literal-pathspecs` e textos que mentem sobre o contrato
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Origem:** RN1 do ML-3A (pathspec mágico como `:(exclude)` em `roadmap_dir` faz `ls-tree` sair rc=0
com `fatal` no stderr, e o D3 não dispara) e os 5 achados do ML-3B (ajuda e comentários ainda dizem
"wip/ or done/").
**Files affected:** `internal/validator/validator.go` (`mdBasenamesInGitTreeWithError` ~:412),
`internal/auditsurface/auditsurface.go` (`gitLsTree` ~:291), `internal/commands/branch.go` (:16, :28,
:80 `Short`), `internal/commands/commit.go` (:23, :80 `Long`), testes dos dois primeiros pacotes.
**Actions:** (1) `git --literal-pathspecs ls-tree …` nos dois leitores; (2) textos: criação = "in wip/"
(done/ só nomeado na mensagem), branch existente = "wip/, blocked/ or done/"; (3) teste: `roadmap_dir`
literal `:(exclude)rm` com roadmap em done/ na base → reconhecido como presente na base.
**Acceptance criteria:**
- [x] O teste do item 3 falha sem `--literal-pathspecs` e passa com ele (provar)
      ✅ Medido pelo arquiteto via overlay: sem a flag FAIL (`exit status 128`), com a flag PASS. Primeira versão do teste não mordia e partia de premissa falsa (":" proibido no macOS); reescrita.
- [x] `grep -n "wip/ or done/" internal/commands/branch.go internal/commands/commit.go` vazio
- [x] Relatório: uma frase por teste novo
**Gates da wave:**
```bash
go build ./...
go test ./internal/validator/ ./internal/auditsurface/ ./internal/commands/ -count=1
```

### ML-3D — Corretivo final: layout `by_agent`, Windows e asserções vácuas
**Status:** ✅ Concluído
**Squad:** artemis-tf
**Origem:** revisão final do arquiteto. (1) Quem reportou o #494 usa `roadmap_namespacing: by_agent`
com `wip/` vazio, e todos os cenários novos são `flat`; o laço do `ls-tree` por `done/` resolvido nunca
rodou com 2 agentes. (2) O NTFS reserva `:`, então o teste do `--literal-pathspecs` não consegue criar
`:(exclude)rm` no `windows-full-suites`. (3) `validator_test.go` ~:1108 afirma a **ausência** do texto
antigo `no roadmap is in wip/ nor done/`, que não existe mais: a asserção é vácua. Stubs de
`push_test.go`/`ship_test.go` ainda carregam o texto antigo.
**Files affected:** `internal/commands/branch_state_e2e_test.go`,
`internal/validator/validator_literal_pathspecs_test.go`, `internal/validator/validator_test.go`,
`internal/commands/push_test.go`, `internal/commands/ship_test.go`
**Acceptance criteria:**
- [x] Cenários `by_agent` (2 agentes, `wip/` vazio) para AC2 e AC5b: reprovam no binário de `e104a7f7`, passam na branch
      ✅ auditado: 2 cenários passam na branch e reprovam no binário de `e104a7f7`
- [x] Teste do `--literal-pathspecs` com `t.Skip` em `windows`, com o motivo escrito
      ✅ `runtime.GOOS == "windows"` com o motivo NTFS
- [x] Asserção vácua trocada pelo texto atual; stubs com o texto atual
      ✅ `TestValidateBranchHasWIPRoadmap_RuleOff`: com a regra sabotada para `error` (overlay) FAIL; original PASS
- [x] Relatório: uma frase por teste novo/alterado
      ✅ a asserção do RuleOff afirma que a regra off silencia a mensagem atual, não um texto extinto
**Gates da wave:**
```bash
go build ./...
go test ./internal/validator/ ./internal/commands/ -count=1
```

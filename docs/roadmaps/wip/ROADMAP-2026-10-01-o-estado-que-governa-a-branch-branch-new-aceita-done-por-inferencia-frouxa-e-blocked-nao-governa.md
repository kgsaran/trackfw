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
| `scripts/check-validate-rule-pins.sh` PIN3/PIN4, `docs/cli-parity.md` (:1292, :1858, :4259-4260) | texto antigo | texto novo |

## Acceptance Criteria
- [ ] AC1 — Wave 0 auditada (parecer de segurança em `docs/seguranca/`)
- [ ] AC2 — #494 fechado na criação (binário, acervo real, `wip/` vazio)
- [ ] AC3 — `RecordBranchLink` só com casamento único em `wip/`
- [ ] AC4 — #490 fechado: commit/validate/push passam após `roadmap move … blocked`, com e sem vínculo
- [ ] AC5 — `done/` em branch existente: aceita o roadmap movido pela própria branch e recusa o alheio
- [ ] AC6 — base inverificável → aviso `branch_done_scope_unverifiable`, nunca violação
- [ ] AC7 — mensagens em fonte única; pinos e `cli-parity.md` atualizados
- [ ] AC8 — relação de casamento intocada; gate do corpus sem mudança de veredito
- [ ] AC9 — teste novo declara o que afirma; teste da REQ-2026-08-04 invertido
- [ ] AC10 — `make quality` EXIT=0 e CI verde

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do conjunto de estados e do sinal "movido por esta branch"
**Status:** ⬜ Pendente
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
- [ ] As quatro seções respondidas com evidência (comando + saída), não asserção de uma linha
- [ ] Veredito explícito: APROVA / APROVA COM AJUSTES (listar) / REPROVA
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-01-wave0-estado-que-governa-a-branch.md
grep -q "Veredito" docs/seguranca/2026-10-01-wave0-estado-que-governa-a-branch.md
```

## Wave 1 — Núcleo no validator (sequencial: ML-1B consome a API do ML-1A)
> Dependencies: Wave 0 auditada

### ML-1A — Resolução por consumidor no `validator`
**Status:** ⬜ Pendente
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
   <done_dir>/`, via `gitCommand`). Para `by_agent`, cada `done/` resolvido é verificado.
5. D3: ref não resolvível ou `ls-tree` com rc≠0 → aceita como hoje e acrescenta aviso
   `branch_done_scope_unverifiable: …` com ref e rc em `Warnings`. Nunca violação.
6. D5: `BranchGovernanceOrientation`/`BranchNoMatchingRoadmapMessage` ganham o consumidor como
   parâmetro (criação → "in wip/"; existente → "in wip/, blocked/ nor done/"). Na criação, se houver
   casamentos em `done/`, a mensagem os nomeia (até 3, ordenados) e orienta `trackfw roadmap move
   <nome> wip`.
7. 🔴 **Proibido** tocar `MatchRoadmapsForBranchSlug`, `branchRoadmapTokens`, `roadmapContentSlug`,
   `sharedTokenCount` e as duas constantes (D6).
**Acceptance criteria:**
- [ ] Testes com git real em diretório temporário: blocked governa (com e sem vínculo); `done/` movido
      pela branch governa; `done/` presente na base não governa; sem `origin` → aviso, sem violação;
      vínculo para `blocked/` não é stale
- [ ] `git diff e104a7f7 -- internal/validator/validator.go` não toca as funções do item 7
- [ ] Relatório: uma frase por teste novo dizendo qual conclusão ele afirma
**Gates da wave:**
```bash
go build ./...
go test ./internal/validator/ -count=1
```

### ML-1B — Consumidores: `branch new`, `commit`, pinos e contrato
**Status:** ⬜ Pendente
**Squad:** apolo-tf
**Files affected:** `internal/commands/branch.go`, `internal/commands/commit.go`, testes em
`internal/commands/*_test.go`, `scripts/check-validate-rule-pins.sh`, `docs/cli-parity.md`
**Actions:**
1. `runBranchNew`: guard contra `wip` só (D1). Os casamentos em `done/` só alimentam a mensagem.
2. `commit.go`: substituir o par `matchSlug` + `branchLink` pela resolução de branch existente do
   ML-1A (fonte única com o `validate`). Imprimir os `Warnings` dela.
3. Inverter o teste herdado da REQ-2026-08-04 ("match em `done/` cria a branch") para "match só em
   `done/` bloqueia e nomeia o roadmap". Não apagar.
4. `check-validate-rule-pins.sh` PIN3/PIN4 e os marcadores `BHR_MARKER`/`MARKER_*`: textos novos.
5. `docs/cli-parity.md` (:1292, :1858, :4259-4260 e onde mais o grep achar): contrato novo por
   consumidor, o sinal "movido por esta branch" e o aviso `branch_done_scope_unverifiable`.
**Acceptance criteria:**
- [ ] `grep -rn "nor done/" internal/commands/*.go` (fora de `_test`) só retorna o `barrier.go` (fora
      de escopo)
- [ ] `make build && GO_BIN=bin/trackfw scripts/check-validate-rule-pins.sh` exit 0
- [ ] Relatório: uma frase por teste novo
**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ ./internal/validator/ -count=1
make build
GO_BIN=bin/trackfw scripts/check-validate-rule-pins.sh
```

## Wave 2 — Ponta a ponta com o binário
> Dependencies: Wave 1 auditada

### ML-2A — Cenários dos issues com o binário real
**Status:** ⬜ Pendente
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
- [ ] Cada cenário falha com o binário da `main` `e104a7f7` e passa com o da branch (provar as duas)
- [ ] Relatório: uma frase por teste novo
**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -count=1
```

## Wave 3 — Revisão independente (paralela: só leitura + parecer próprio)
> Dependencies: Wave 2 auditada

### ML-3A — Revisão de segurança
**Status:** ⬜ Pendente
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-01-wave3-revisao-estado-que-governa-a-branch.md`
**Actions:** confrontar a implementação com os alvos do ML-0A; reimplementar a partir da leitura os
vetores (a)–(e) contra o binário da branch.
**Acceptance criteria:**
- [ ] Veredito explícito

### ML-3B — Revisão de qualidade e gate completo
**Status:** ⬜ Pendente
**Squad:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-01-revisao-estado-que-governa-a-branch.md`
**Actions:** fonte única (nenhuma resolução reimplementada em `commands/`), D6 intacto, `make quality`
completo.
**Acceptance criteria:**
- [ ] `make quality` EXIT=0 (saída completa, sem `| tail`)
- [ ] Veredito explícito

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-01-wave3-revisao-estado-que-governa-a-branch.md
test -s docs/qualidade/2026-10-01-revisao-estado-que-governa-a-branch.md
```

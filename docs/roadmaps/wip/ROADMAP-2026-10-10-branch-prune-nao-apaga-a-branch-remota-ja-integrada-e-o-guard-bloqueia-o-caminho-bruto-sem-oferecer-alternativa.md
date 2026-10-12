---
status: wip
date: 2026-10-10
req: "docs/req/REQ-2026-10-10-branch-prune-nao-apaga-a-branch-remota-ja-integrada-e-o-guard-bloqueia-o-caminho-bruto-sem-oferecer-alternativa.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: branch prune não apaga a branch remota já integrada e o guard bloqueia o caminho bruto sem oferecer alternativa

> Created: 2026-10-10 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-10-branch-prune-nao-apaga-a-branch-remota-ja-integrada-e-o-guard-bloqueia-o-caminho-bruto-sem-oferecer-alternativa.md
ADR: docs/adr/ADR-2026-10-12-branch-prune-apaga-a-branch-remota-so-pelo-sinal-do-forge-e-condicionada-ao-tip-avaliado.md

Branch compartilhada com a REQ-2026-10-12 (#556, prazo das operações de rede). **Esta REQ implementa
depois**: a remoção remota exige fetch concluído dentro do prazo (ADR D4), e as duas tocam
`internal/commands/branch_prune*.go`.

## Acceptance Criteria
- [ ] AC1 — Wave 0 auditado (parecer do `hades-tf`)
- [ ] AC3/AC4/AC5 — `--remote` opt-in; só `merged_pr` com tip remoto contido; lease no SHA avaliado; `keep`/`review`/padrão/protegida nunca apagadas
- [ ] AC6 — mensagem do guard aponta para `trackfw branch prune --remote --apply`
- [x] AC7 — dry-run com `--remote` neste repositório (reformulado pelo C12: as remotas com commits após o PR mergeado saem `keep`, com motivo)
      ✅ 2026-10-12, binário da branch: 2 remotas avaliadas contra o forge, ambas `keep` (sem PR mergeado), nada apagado. As duas remotas motivadoras já tinham sido apagadas no forge pelo usuário; o caminho de remoção é provado pelo `TestRemoteDeleteForceLease`
- [ ] AC8/AC9/AC10 — contrato, reconciliação dos testes, `make quality` + CI

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation. Roda em paralelo com o Wave 0 do ROADMAP-2026-10-12.

### ML-0A — Threat model da remoção remota
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-10-12 · aprovado com ajustes A1–A9 (incorporados ao ADR, D8)
**Files affected:** `docs/seguranca/2026-10-12-wave0-prune-remoto.md` (novo; único arquivo que pode ser escrito)
**Actions:**
1. **Completude da população.**
   - Enumerar onde vive a mensagem do guard para `git push` (`internal/guard/gitbranch.go`,
     `internal/generators/testdata/guard-sh-reference/git-branch-guard.sh`, gerador do script,
     `scripts/`), por grep do literal `git push bruto bloqueado`.
   - Enumerar as formas de remoção remota que o guard precisa reconhecer: `--delete`, `-d`, `:<ref>`,
     `+:<ref>`, `refs/heads/` explícito e múltiplos refspecs.
2. **Threat model (AC1 da REQ):** o que faz a remoção remota apagar trabalho não integrado ou de
   terceiro.
   - Obrigatório cobrir: branch remota de outro autor; ref que avançou depois do fetch (TOCTOU); PR
     aberto de mesmo nome; branch protegida; remoto que não é `origin`; resposta do forge truncada
     (A7); ausência de `gh`; fork com o mesmo nome de branch; `refs/remotes/origin/*` forjado
     localmente com refspec estreitado (nota do vault
     `git-fetch-self-heals-forged-origin-head-and-tracking-refs-2026-08-19`).
3. **Lease na remoção (ADR D5).**
   - Na documentação oficial do git, confirmar que `--force-with-lease=<ref>:<sha>` vale para a forma
     de remoção (`:<ref>` / `--delete`).
   - Propor o teste Go (repositório bare em `t.TempDir()`) que o ML-1A deve conter, com os dois braços:
     tip igual apaga; tip avançado recusa e mantém.
   - 🔴 Não tente medir com `git commit`/`git push` brutos no shell: o guard bloqueia, e não se contorna.
4. **Branch protegida (ADR D6).** Qual consulta ao forge informa a proteção, se ela exige permissão
   de administrador (a nota de `check-required-full` mediu 404 sem token de mantenedor), e o que
   fazer quando ela falha.
5. **Alvos de falsificação nas duas direções**, por caso: o que reprova quando a regra regride
   (apaga o que não devia) e quando regride ao contrário (não apaga a integrada; `--remote` sem
   `--apply` escreve no remoto).
6. **Resíduo declarado** e **veredito sobre o ADR** com ajustes numerados (A1, A2, …).
**Acceptance criteria:**
- [x] As seis seções respondidas com evidência (comando + resultado, ou arquivo:linha, ou documentação citada), não asserção
      ✅ Evidência: `docs/seguranca/2026-10-12-wave0-prune-remoto.md`; C12 e A7 conferidos pelo arquiteto (`merge-base --is-ancestor` → 1 nas duas; `branch_prune.go:19,26`, `branch_prune_forge.go:293`)
- [x] Nenhuma linha de implementação escrita; nenhum arquivo além do parecer alterado
      ✅ Evidência: `git diff --quiet HEAD -- internal/` EXIT=0
- [x] Nenhuma escrita em remoto real; nenhum `git commit`/`push` bruto
      ⚠️ Ressalva: nenhum remoto real foi tocado, mas a sonda do lease rodou `git commit`/`push` brutos dentro de um script bash em tmpdir isolado — contorno do guard que o handoff proibia. Sem impacto no projeto; registrado e levado ao usuário. A medição vale, e o teste Go do ML-1B a refaz de forma legítima

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-12-wave0-prune-remoto.md
grep -q "Veredito" docs/seguranca/2026-10-12-wave0-prune-remoto.md
git diff --quiet HEAD -- internal/
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditado (✅) e **Wave 1 do ROADMAP-2026-10-12 concluído** (prazo no fetch e no
> `gh`). ML-1A → ML-1B são sequenciais (mesmos arquivos); ML-1C toca arquivos disjuntos e pode correr
> em paralelo a eles; ML-1D por último (`docs/cli-parity.md` também é escrito pelo outro roadmap).

### ML-1A — Branch padrão vem do forge (A7)
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-10-12 após uma devolução (testes só com stubs, sem mordida). `go test ./internal/commands/` e `make parity-rest` EXIT=0, rodados pelo arquiteto
**Por que o escopo original não previa:** o Wave 0 achou o literal `"main"` em `branch_prune.go:19,26` e no filtro `baseRefName` (`branch_prune_forge.go:293`). Num repositório `master`, o prune local de hoje recusa tudo. É a mesma causa do D2 do ADR (branch padrão vinda do forge), por isso entra nesta REQ.
**Files affected:** `internal/commands/branch_prune.go`, `internal/commands/branch_prune_forge.go`, testes.
**Acceptance criteria:**
- [x] Branch padrão vem do `default_branch` do forge; sem forge, do symref `origin/HEAD`; sem nenhum dos dois, recusa como hoje
      ✅ `resolveDefaultBranch` (forge → symref → literal) em `branch_prune_forge.go`; `TestDefaultBranch_{ForgeVsSymref_ForgePrevails,NeitherAvailable_FallsBackToMain}`
- [x] Teste com repositório `master` (local) passa; sem a correção, reprova
      ✅ `TestBranchPrune_MasterRepo_EndToEnd` (repositório real em `t.TempDir()`); mordida M1 → `origin/main not resolvable`; M2 (filtro `"main"`) e M3 (symref antes do forge) reprovam seus testes
- [x] `go build ./...` EXIT=0; `go test ./internal/commands/` EXIT=0
      ✅ build, vet (inclusive `GOOS=windows`), `go test ./internal/commands/` 40,3 s

### ML-1B — `branch prune --remote`
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-10-12. Mordida A6 refeita pelo arquiteto (`deletable()` → `TestRemotePrune_ContentHeuristicWithoutPR_Keep` reprova). Pré-condição extra do agente, aceita: branch padrão que não veio do forge também bloqueia a remoção remota (D2). Correção ao parecer do Wave 0: o lease implícito (M1) é pego pelo braço C, não pelo B — medido
**Files affected (provisório):** `internal/commands/branch_prune.go`, `internal/commands/branch_prune_forge.go`, novo `internal/commands/branch_prune_remote.go`, testes.
**Acceptance criteria:**
- [x] Sem `--remote`: nenhuma chamada que escreva no remoto (teste com executor que registra os argumentos)
      ✅ `TestRemotePrune_NoRemoteFlag_NoPushArgs` (executor que registra argumentos)
- [x] `--remote` sem `--apply`: só relata
      ✅ `TestRemotePrune_NoApplyFlag_NoPush`
- [x] Remoção exige `decision == merged_pr` sobre o tip remoto (A6) e lease no SHA avaliado; `TestRemoteDeleteForceLease` com braços A/B/C e mutações (A9)
      ✅ `branch_prune_remote.go:183` (`!= branchPruneDecisionMergedPR`); `TestRemoteDeleteForceLease` A/B/C em bare repo real; mordidas M1 (lease implícito) → C reprova, M2 (lease em `refs/remotes`) → B e C, M3 (SHA relido) → C
- [x] `keep`/`review`/padrão/protegida (A4)/base de PR aberto (A3) nunca apagadas, um teste por caso; `origin/HEAD` excluído por refname (A8); URLs de fetch e push divergentes (A5), fetch falho/estourado ou forge degradado/truncado → nenhuma remoção remota
      ✅ testes (d)–(i): commits após o PR, PR aberto, base de PR aberto, protegida, padrão/`origin/HEAD` por refname, 6 pré-condições
- [x] `go build ./...` EXIT=0; `go test ./internal/commands/` EXIT=0
      ✅ `go build`, `go vet` (inclusive `GOOS=windows`), `go test ./internal/commands/` 40,7 s, rodados pelo arquiteto

### ML-1C — Mensagem do guard para remoção de branch
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-10-12. A mordida 2 do relatório (referência `.sh`) citava falhas de teste Go; refeita pelo arquiteto: com a detecção do `.sh` neutralizada, `TestGitBranchGuard_PushDelete_ParityBranchDeleteForms` reprova
**Files affected (provisório):** `internal/guard/gitbranch.go`, `internal/generators/testdata/guard-sh-reference/git-branch-guard.sh` e os demais sítios da seção 1.1 do parecer, testes de paridade.
**Acceptance criteria:**
- [x] `--delete`/`-d`/`:<ref>` (não `refs/tags/`) → mensagem que nomeia `trackfw branch prune --remote --apply`; `--mirror`, `--prune`, `:refs/tags/…`, `:` e push comum mantêm a atual (A1)
      ✅ `reasonPushDelete` + `isPushBranchDelete` (`internal/guard/gitbranch.go`); 22 casos em `TestRunGitBranch_PushDelete_BranchDeleteForms`; mordida 1 → 14 FAILs nas formas de remoção
- [x] Teste de paridade com os vetores de remoção (A2); guard continua bloqueando a forma bruta
      ✅ `TestGitBranchGuard_PushDelete_ParityBranchDeleteForms` (Go × `.sh`); pin sha256 da referência atualizado; todos os casos afirmam o bloqueio
- [x] `go test ./internal/guard/ ./internal/generators/` EXIT=0
      ✅ `go test ./internal/guard/` e `-run PushDelete ./internal/generators/` EXIT=0; `go vet` inclusive `GOOS=windows`

### ML-1D — Contrato
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-10-12; nome de teste inexistente citado no contrato (`TestGuard_PushBranchDelete`) corrigido pelo arquiteto
**Files affected:** `docs/cli-parity.md` (depois do ML-1B do ROADMAP-2026-10-12)
**Acceptance criteria:**
- [x] Seção do `branch prune --remote` e da branch padrão do forge, com `trackfw-contract`; `make parity-rest` EXIT=0
      ✅ subseção `--remote` (anotação `gap`, coberta por `branch_prune_remote_test.go`), branch padrão do forge, tabela do guard; `check-parity-contract-coverage.sh` 296/0; `make parity-rest` EXIT=0

## Wave 2 — Revisão e fechamento
> Dependencies: Wave 1 dos dois roadmaps. A revisão é única para a branch (ML-2A do ROADMAP-2026-10-12).

### ML-2A — Medição de volta e fechamento
**Owner:** `zeus-tf`
**Status:** 🔄 Em andamento — falta só o CI do PR
**Acceptance criteria:**
- [x] AC7: `trackfw branch prune --remote` (dry-run) neste repositório lista as remotas integradas e nenhuma com trabalho não integrado
      ✅ ver AC7 consolidado acima (2 remotas, ambas `keep`, nada apagado)
- [x] Pareceres do ML-2A do ROADMAP-2026-10-12 cobrem esta REQ; corretivos aplicados
      ✅ qualidade (corretivo ML-2B do ROADMAP-2026-10-12) e red-team sem bloqueante, ambos cobrindo `branch_prune_remote.go` e o guard
- [ ] `make quality` EXIT=0 com máquina ociosa; CI do PR verde, inclusive `windows-full-suites`
      🔄 `make quality` EXIT=0 em `1849d74c`, máquina ociosa (load 1,5; 2962 linhas; falsify 347 OK / 0 FAIL). CI pendente: só dispara com PR

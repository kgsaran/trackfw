---
status: wip
date: 2026-11-26
req: "docs/req/REQ-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: Repositório novo não tem rota governada para publicar a branch padrão e o init não inicializa o git

> Created: 2026-11-26 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md -->
REQ: docs/req/REQ-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] D1: `init` cria o repositório git quando não há work tree
- [ ] D2: `branch new` recusa `HEAD` sem commit; `trackfw commit` aceita só o primeiro commit na branch padrão
- [ ] D3: `trackfw push` publica a branch padrão uma vez, pela URL de push e com lease de ausência
- [ ] D4: feature não sobe para remoto vazio
- [ ] D5: mensagens e templates descrevem o bootstrap; `cli-parity.md` atualizado
- [ ] Replay do fluxo do relato termina com `main` e feature no remoto
- [ ] `make quality` rc=0 e CI verde

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap (hades-tf)
**Status:** ✅ Done — auditado pelo arquiteto. Achados de bloqueio incorporados ao ADR ("Ajustes do Wave 0"): D1 em
quatro estados, predicado de D2, D3 com remoto vazio, todas as URLs de push e um commit, D4 também no `ship`, D6 novo
(guard). Conferi no binário: `gh repo create ... --push`, `git switch --orphan` e `git fetch . feat/x:main` passam
pelo guard com rc=0.
**Files affected:** `docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md` (novo);
`vault/notes/bootstrap-repo-novo-achados-wave0-2026-11-26.md` (novo); `vault/notes/index.md`
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
5. Específicos desta REQ, com medição no binário em sandbox (HOME isolado, remoto bare local):
   - `pushurl` divergente de `url`: a consulta pela URL de push fecha? O lease `--force-with-lease=refs/heads/main:`
     é honrado pelo git instalado e recusa quando a branch existe no destino?
   - `gh repo create --source . --push` e `gh repo create ... --push` passam pelo guard git-branch hoje?
   - `commit_unborn_branch_test.go` já trata `HEAD` sem commit: o que ele afirma e como D2 interage?
   - D4 sobre remotos reais: o que `ls-remote --heads` devolve num remoto vazio do GitHub e num remoto com só tags?
   - Censo das superfícies: todo emissor do bloco "Agent Protocol" e toda mensagem que manda usar
     `trackfw branch new` ou `trackfw push` na branch padrão.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Wave gates:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
test -s docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md
```

## Wave 1 — Implementation
> Dependencies: Wave 0 audited. ML-1A, ML-1B e ML-1D têm arquivos disjuntos, mas rodam em sequência num só agente
> (teto de CPU e o mesmo pacote `internal/commands` em ML-1A/1B). ML-1C fecha a wave, porque escreve
> `docs/cli-parity.md` e os templates a partir do comportamento final.

### ML-1A — `init` com `git init`; `branch new` e `commit` no bootstrap (apolo-tf)
**Status:** ⬜ Pending
**Files affected:** `internal/commands/init.go`; `internal/commands/init_test.go`; `internal/commands/branch.go`;
`internal/commands/branch_test.go`; `internal/commands/commit.go`; `internal/commands/commit_test.go`;
`internal/commands/commit_unborn_branch_test.go`
**Actions:** D1 (quatro estados) e D2 (predicados separados de `commit` e `branch new`) do ADR-2026-11-26, seção
"Ajustes do Wave 0". Relatório: `docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md` §6.
**Acceptance criteria:**
- [ ] `go test ./internal/commands/ -run 'Init|Branch|Commit' -count=1` rc=0; braço e contra-braço por decisão; frase de
      reconciliação por teste novo
- [ ] nenhum arquivo fora de "Files affected" alterado

### ML-1B — `push` e `ship`: publicação única da branch padrão e feature barrada em remoto vazio (apolo-tf)
**Status:** ⬜ Pending
**Files affected:** `internal/commands/push.go`; `internal/commands/push_test.go`; `internal/commands/ship.go`;
`internal/commands/ship_test.go`
**Actions:** D3 (remoto vazio em todas as URLs de push, um commit, lease de ausência, recusa da flag
`--force-with-lease`; substitui o bloqueio em `push.go:143`) e D4 (helper usado por `runPush` e por `runShip`,
`ship.go:417`) do ADR-2026-11-26, seção "Ajustes do Wave 0". Testes com remotos bare em `t.TempDir()`.
**Acceptance criteria:**
- [ ] `go test ./internal/commands/ -run 'Push|Ship' -count=1` rc=0; recusam os vetores de D3 (`pushurl` ≠ `url`, várias
      `pushurl`, branch criada entre consulta e push, remoto ausente/inacessível, `--force-with-lease`, `main` com 2
      commits); D4 com remoto vazio recusa em `push` e `ship`; com remoto só `develop`, passa
- [ ] nenhum arquivo fora de "Files affected" alterado

### ML-1C — Mensagens, templates e contrato (apolo-tf)
**Status:** ⬜ Pending
**Files affected:** `internal/commands/barrier.go`; `internal/commands/barrier_ml_scope_test.go`;
`internal/validator/validator_credential_guard_integrity.go`; `internal/generators/agentfiles.go`;
`internal/generators/agentfiles_test.go`; `internal/generators/nextsteps.go`;
`internal/integrations/assets/agents/architect.md`; `docs/cli-parity.md`
**Actions:** D5 do ADR-2026-11-26, sobre a lista de superfícies do censo do Wave 0 (`docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md` §1).
**Acceptance criteria:**
- [ ] `go test ./internal/commands/ ./internal/validator/ ./internal/generators/ -count=1` rc=0
- [ ] nenhum arquivo fora de "Files affected" alterado

### ML-1D — Guard git-branch: `gh repo create --push`, `--orphan`, `symbolic-ref HEAD` (apolo-tf)
**Status:** ⬜ Pending
**Files affected:** `internal/guard/gitbranch.go`; `internal/guard/gitbranch_test.go`
**Actions:** D6 do ADR-2026-11-26. Braço (rc=0 antes, rc=2 depois) por comando; contra-braço: `gh repo create x
--private` sem `--push`, `git switch feat/x` e `git symbolic-ref --short HEAD` (leitura) continuam com rc=0.
**Acceptance criteria:**
- [ ] `go test ./internal/guard/ -count=1` rc=0; frase de reconciliação por teste novo
- [ ] nenhum arquivo fora de "Files affected" alterado

**Wave gates:**
```bash
go build ./...
go test ./internal/commands/ ./internal/validator/ ./internal/generators/ ./internal/guard/ -count=1
```

## Wave 2 — Review
> Dependencies: Wave 1 audited. Exceção num guard: revisão de qualidade e red team.

### ML-2A — Revisão de qualidade (hefesto-tf)
**Status:** ⬜ Pending
**Files affected:** `docs/seguranca/2026-11-26-revisao-bootstrap-de-repositorio-novo.md` (novo)
**Acceptance criteria:**
- [ ] revisão escrita; nenhum achado bloqueante aberto

### ML-2B — Red team da exceção de bootstrap (hades-tf)
**Status:** ⬜ Pending
**Files affected:** `docs/seguranca/2026-11-26-red-team-bootstrap-de-repositorio-novo.md` (novo)
**Acceptance criteria:**
- [ ] vetores do Wave 0 medidos antes e depois no binário; nenhum escape aberto

## Wave 3 — Gate final

### ML-3A — Replay, `make quality` e CI (zeus-tf)
**Status:** ⬜ Pending
**Files affected:** este roadmap; `docs/req/REQ-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md`; `docs/adr/ADR-2026-11-26-bootstrap-de-repositorio-novo-init-roda-git-init-e-a-branch-padrao-e-publicada-uma-vez-pelo-trackfw.md`;
`docs/adr/ADR-2026-08-22-comandos-de-entrega-separados-push-proprio-e-ship-como-composicao.md`;
`docs/adr/ADR-2026-10-10-a-fiacao-do-guard-e-ancorada-no-origin-main-por-tupla-e-a-remocao-e-violacao-fora-do-lenient-e-do-baseline.md`
(notas de emenda)
**Acceptance criteria:**
- [ ] replay do fluxo do relato no binário da branch; `make quality` rc=0; CI verde

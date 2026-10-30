---
status: wip
date: 2026-10-30
req: "docs/req/REQ-2026-10-30-branch-new-e-branch-prune-resolvem-a-branch-padrao-errada-quando-o-origin-head-local-nao-existe-e-o-git-e-anterior-a-2-48.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: branch new e branch prune resolvem a branch padrao errada quando o origin HEAD local nao existe e o git e anterior a 2.48

> Created: 2026-10-30 | Status: wip

## Context

REQ: docs/req/REQ-2026-10-30-branch-new-e-branch-prune-resolvem-a-branch-padrao-errada-quando-o-origin-head-local-nao-existe-e-o-git-e-anterior-a-2-48.md
ADR: docs/adr/ADR-2026-10-30-resolucao-da-branch-padrao-consulta-o-remoto-quando-o-symref-local-falta.md · Fecha o issue #633.

O fetch só cria `refs/remotes/origin/HEAD` a partir do git 2.48. Sem symref e sem forge, a cadeia
de `resolveDefaultBranchWithSource` cai no literal `"main"`. Correção: fonte nova
`git ls-remote --symref origin HEAD` entre o symref local e o literal. Reprodução em qualquer git com
`git config remote.origin.followRemoteHEAD never` no fixture.

Corre em paralelo com o roadmap do #634 (`internal/generators/`), que não compartilha arquivo.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model da fonte ls-remote na resolução da branch padrão (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-30. #633 reproduzido no git 2.54 com `followRemoteHEAD never`. P-1 a P-4 aceitas: a fonte nova só entra quando o `symbolic-ref` falha; o nome passa por lista de permissão (`isValidBranchLocalName`); o `defaultBaseBranch` (`ship`, `release`) também resolvia `main` num repositório `master` e entra pelo mesmo helper; fakes de teste que precisam declarar o caso novo listados. Decisão do arquiteto: o residual R-5 (`git log <base>..HEAD` com nome nu falha quando a branch local não existe) entra no ML-1A — defeito conhecido se corrige no mesmo PR. O V-2 (`GIT_TERMINAL_PROMPT`) fica fora: afeta `fetch` e `push` igualmente e é decisão de UX, levada ao KG.
**Files affected:** `docs/seguranca/2026-10-30-wave0-branch-padrao-ls-remote.md`
**Actions:**
1. Confiança: o que um remoto hostil (ou um `origin` trocado) controla na saída do
   `ls-remote --symref`, e o que a resolução faz com um nome de branch arbitrário (flag injection no
   `rev-parse --end-of-options`, nomes com `..`, espaço, `-`). Comparar com o que o symref local já permite hoje.
2. Rede: prazo do `ls-remote` (`networkSubcmds`), comportamento offline, e se a chamada nova acontece
   em caminhos onde antes não havia rede (ex.: `ship` sem forge).
3. Enumeração fechada dos chamadores de `resolveDefaultBranchWithSource`, `resolveDefaultBranch` e
   `defaultBaseBranch`, com o efeito de um symref ausente em cada um (`branch new`, `branch prune`,
   `ship`, `release`). Meça num repositório `master` sem symref com `followRemoteHEAD never`.
4. Fakes de `gitExec` nos testes de `internal/commands`: quais devolvem `("", nil)` no `default` e
   passariam a resolver errado com a chamada nova.
5. Alvos de falsificação nas duas direções para AC1–AC6 da REQ.
6. Prescrições numeradas para o ML-1A, e residual.
**Acceptance criteria:**
- [x] As seis seções respondidas com evidência (comando e resultado)
- [x] Prescrições escritas em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-wave0-branch-padrao-ls-remote.md
grep -q 'Prescrições' docs/seguranca/2026-10-30-wave0-branch-padrao-ls-remote.md
```

---

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Fonte ls-remote na cadeia de resolução (apolo-tf) — AC1–AC6
**Status:** ✅ Concluído — auditado em 2026-10-30. Pacote `internal/commands` verde; 16 testes novos ou alterados conferidos pelo nome em `-v`. Contra-braços medidos: sem a fonte nova, o AC1 (`branch new`), o AC3 (`branch prune`) e o AC5 (`defaultBaseBranch`) reprovam; sem o R-5, `TestGitCommitsSince_FallsBackToOriginWhenLocalRefAbsent` reprova. O comentário da cadeia em `branch_prune.go:416-420` (só comentário) foi atualizado pelo arquiteto.
**Files affected:** `internal/commands/branch_prune_forge.go`, `internal/commands/branch_new_base_e2e_test.go`, `internal/commands/branch_default_ls_remote_test.go`, `internal/commands/ship.go`, `internal/commands/ship_test.go`, `internal/commands/release.go`, `internal/commands/release_test.go`, `internal/commands/branch_prune_default_branch_test.go`, `internal/commands/branch_prune.go`, `docs/cli-parity.md`
**Actions:**
1. `resolveDefaultBranchWithSource`: entre o symref local e o literal, `gitExec("ls-remote", "--symref", "origin", "HEAD")`;
   aceitar só a linha `ref: refs/heads/<nome>\tHEAD`; `fromForge=false`; qualquer outra saída cai no literal.
2. `TestBranchNew_DryRunFetchesMasterDefault`: `git config remote.origin.followRemoteHEAD never` no fixture
   antes do `set-head --delete`; corrigir o comentário da premissa.
3. `branch_default_ls_remote_test.go` (novo): testes unitários do parse e E2E do `branch prune` num repositório `master` sem symref.
4. `ship.go` (`defaultBaseBranch`): a Wave 0 mediu resultado errado; converge para o mesmo helper (P-3), sem chamar a forge. Comentários de `ship.go` e `release.go` atualizados (P-3).
5. `docs/cli-parity.md` (~2016 e o bloco da cadeia): a cadeia passa a ser forge → symref → `ls-remote --symref` → `"main"`.
6. R-5: `gitCommitsSince` usa `origin/<base>` quando `refs/heads/<base>` não existe.
7. Prescrições P-1 a P-4 do parecer `docs/seguranca/2026-10-30-wave0-branch-padrao-ls-remote.md`.
**Acceptance criteria:**
- [x] AC1 e AC2 (contra-braço medido sem a fonte nova)
- [x] AC3, AC4 e AC5 da REQ
- [x] AC6: pacote `internal/commands` inteiro verde

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ -count=1
```

---

## Wave 2 — Red team
> Dependencies: Wave 1 auditada.

### ML-2A — Red team da fonte ls-remote (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-30. Aprova com ressalvas; prazo de rede medido (`ship` offline cai em `main` em 2,0 s com `TRACKFW_NETWORK_TIMEOUT=2s`). Cinco achados viram a Wave 3 (defeito conhecido se corrige no mesmo PR): A-5 `HEAD` (e `.`, `x.lock`, `/x`, `x/`) aceitos como nome de branch; A-1 vetor de segundo campo ausente no teste do parse; A-2 `defaultBaseBranch` sem teste que fixe "ls-remote só quando o symref dá erro"; A-3 `gitCommitsSince` prefere a branch local, que pode estar atrás do remoto; A-4 comentário errado em `release_test.go`. Decisão do arquiteto no A-3: o PR vai contra o remoto, então a base é `refs/remotes/origin/<base>` quando existe, com `refs/heads/<base>` como fallback, sempre por refname completo (fecha também a ambiguidade com tag `origin/<base>` medida pelo red team).
**Files affected:** `docs/seguranca/2026-10-30-redteam-branch-padrao-ls-remote.md`
**Actions:**
1. Tentar fazer a resolução aceitar saída malformada ou hostil do `ls-remote` sem teste reprovar.
2. Veredito por prescrição da Wave 0, com evidência.
**Acceptance criteria:**
- [x] cada prescrição com veredito e evidência
- [x] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-30-redteam-branch-padrao-ls-remote.md
```

---

## Wave 3 — Corretivos do red team (1 ML)
> Dependencies: Wave 2 auditada.

### ML-3A — Achados A-1 a A-5 do red team (apolo-tf)
**Status:** ⬜ Pendente
**Files affected:** `internal/commands/branch_prune_forge.go`, `internal/commands/ship.go`, `internal/commands/branch_default_ls_remote_test.go`, `internal/commands/ship_test.go`, `internal/commands/release_test.go`
**Actions:**
1. A-5: `isValidBranchLocalName` recusa também `HEAD`, componente que começa com `.`, sufixo `.lock`, `/` no início ou no fim e `//`. Vetores na tabela do teste (o caso `HEAD` documentado como aceito passa a recusar).
2. A-1: vetor `ref: refs/heads/main\trefs/heads/HEAD` → recusa; nome do subteste corrigido para o que ele testa.
3. A-2: teste em `ship_test.go` que reprova se `defaultBaseBranch` chamar `ls-remote` quando o `symbolic-ref` teve sucesso com saída malformada (caminhos b e c), e se chamar antes do `symbolic-ref`.
4. A-3: `gitCommitsSince` usa `refs/remotes/origin/<base>` quando existe e `refs/heads/<base>` como fallback, por refname completo. Testes: branch local atrás do remoto lista só os commits da feature; sem `origin/<base>`, usa a local; tag chamada `origin/<base>` não interfere.
5. A-4: corrigir o comentário de `release_test.go` ("absent in the mock → skipped").
**Acceptance criteria:**
- [ ] mutações M-2, M-4b, M-5b e M-6 do red team reprovam
- [ ] `HEAD` servido pelo remoto cai no literal
- [ ] pacote `internal/commands` verde

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ -count=1
make quality
```

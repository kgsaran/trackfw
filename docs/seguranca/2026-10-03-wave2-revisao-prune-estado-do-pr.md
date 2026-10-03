---
status: wave2
date: 2026-10-03
reviewer: hades-tf
roadmap: docs/roadmaps/wip/ROADMAP-2026-10-03-branch-prune-classifica-branches-ja-mergeadas-como-pending-work-porque-nunca-consulta-o-estado-do-pr.md
req: docs/req/REQ-2026-10-03-branch-prune-classifica-branches-ja-mergeadas-como-pending-work-porque-nunca-consulta-o-estado-do-pr.md
---

# Wave 2 — Revisão de segurança: sinal de PR no `branch prune`

> Date: 2026-10-03 | Reviewer: hades-tf | Branch: `fix/branch-prune-consulta-o-estado-do-pr`
> Commit revisado: `76984e04` | ML revisado: ML-1A

Wave 0: `docs/seguranca/2026-10-03-wave0-prune-estado-do-pr.md`
ADR: `docs/adr/ADR-2026-10-03-branch-prune-usa-o-estado-do-pr-no-forge-como-sinal-de-integracao-com-o-conteudo-como-fallback-declarado.md`
Implementação: `internal/commands/branch_prune_forge.go`, `internal/commands/branch_prune.go`, `internal/commands/ship.go`

---

## Veredito final

**APROVA COM AJUSTES**

Um defeito reproduzido por fixture (seção 2.2): branch nunca empurrada com PR mergeado em base
não-main recebe `no_own_work` (deletável) via heurística de conteúdo. Viola o ADR D1 §4. Ajuste
em 1 frase executável na seção 6.

Todos os ajustes A1–A8 da Wave 0 estão fechados. Duas lacunas de cobertura de teste declaradas
como resíduo (baixa severidade). O ajuste obrigatório não está em `internal/` — é relatado ao
arquiteto para delegação ao implementador.

---

## 1. Veredito por cenário (C1–C11)

### C1 — PR de fork com mesmo nome de head: FECHADO

`evaluateBranchWithForge` (forge.go:286–290):

```go
if pr.IsCrossRepository {
    continue
}
hasPRs = true
```

Fork PRs são ignorados antes de `hasPRs = true`. Se a branch local tem apenas PRs de fork,
`hasPRs` permanece `false`, e o case 4 protege ramos nunca empurrados.

Evidência lida: `TestAC3_ForkPR_MERGED_NeverDelete` (forge\_test.go:279). O teste passa um PR de
fork MERGED com `tip == prHead` (que seria case 1 sem o filtro), stubs a heurística de conteúdo
retornando `pending_work`, e afirma que o resultado não é `merged_pr` e não é deletável. A
combinação exige que o filtro `isCrossRepository` esteja ativo E que a heurística de conteúdo seja
alcançada; a remoção do filtro daria `merged_pr`.

### C2 — Branch reaproveitada após PR antigo mergeado: FECHADO

Para SHA novo sem relação com o headRefOid do PR antigo: `is-ancestor tip headRefOid` = EXIT=1
(não ancestral); `is-ancestor headRefOid tip` = EXIT=1 → case 2b (`diverged_from_merged_pr`) →
keep. Se tip == headRefOid do PR antigo: case 1 → delete. Correto: o conteúdo foi integrado.

Evidências lidas: `TestD1_Case2b_DivergedFromMerged_Keep` (forge\_test.go:156),
`TestD1_RealGit_Case1_And_Case2` (forge\_test.go:703).

### C3 — Vários PRs com mesmo head (um MERGED, um OPEN): FECHADO

Case 0 (`openPRs`) é verificado antes de `mergedValidBase`. Um PR OPEN prevalece sobre qualquer PR
MERGED.

Evidência lida: `TestD1_Case0_OpenPR_Keep` (forge\_test.go:51). O stub inclui exatamente um MERGED
+ um OPEN para o mesmo head; o MERGED passaria case 1 sem o filtro de OPEN, mas o OPEN é retornado
primeiro.

### C4 — Resposta truncada ou paginada: FECHADO com ressalva menor

`forgeQueryLimit = 3000` (forge.go:40). Se `len(prs) == 3000`, `queryForgePRs` retorna nil com
mensagem de causa — nenhum delete por sinal de forge ocorre nesse run (forge.go:119–128).

Evidência lida: `TestAC5_TruncatedResponse_BlocksForgeSignalDeletes` (forge\_test.go:474).
O teste constrói exatamente `forgeQueryLimit` PRs, chama `queryForgePRs`, e afirma que o
snapshot é nil e a reason menciona "truncated" ou "limit". O fallback para a heurística de
conteúdo é correto no código (forge.go:272–273), mas o teste não verifica esse segundo passo
(lacuna L2, declarada na seção 5).

### C5 — `gh` resolvendo repositório errado / `GH_REPO` misdirecionado: FECHADO

`queryForgePRs` (forge.go:97–107) passa `--repo HOST/OWNER/REPO` derivado de
`git remote get-url origin` via `parseHostOwnerRepo`. `GH_REPO` ambiental é sobrescrito (medido
na Wave 0). Se o host não é `github.com`, degrada (D2) antes de chamar `ghExec` (forge.go:91–93).

Evidência lida: `TestA1_RepoFlagUsed` (forge\_test.go:505) — verifica que `--repo github.com/testowner/testrepo` e `--limit 3000` aparecem nos args de `ghExec`. `TestA1_NonGitHubHost_Degrades` (forge\_test.go:553) — verifica que `ghExec` não é chamado para host não-GitHub.

### C6 — Erro com exit 0 / JSON parcial: FECHADO

Exit ≠ 0 → D2 (forge.go:104–107). Falha de `json.Unmarshal` → D2 (forge.go:110–113). Lista `[]`
com exit 0 → snapshot válido com zero PRs.

Evidência lida: `TestA5_GhExitNonZero_Degrades` (forge\_test.go:612) — `ghExec` retorna erro;
snapshot deve ser nil. `TestA5_InvalidJSON_Degrades` (forge\_test.go:862) — JSON truncado; snapshot
deve ser nil com reason descrevendo parse failure.

### C7 — Branch sem upstream: FECHADO

`upstreamFor` (forge.go:211–217) usa `git for-each-ref --format=%(upstream:short)`. Campo vazio =
nunca empurrada = case 4. Campo não-vazio = foi empurrada.

**Medição executada nesta revisão:**

```bash
git for-each-ref --format="%(upstream:short)" refs/heads/chore/fecha-req-2026-09-01-contributing
# origin/chore/fecha-req-2026-09-01-contributing   (branch [gone] → upstream:short não-vazio)

git for-each-ref --format="%(upstream:short)" refs/heads/fix/branch-prune-consulta-o-estado-do-pr
# (vazio — branch nunca empurrada)
```

`[gone]` → `%(upstream:short)` retorna o nome do tracking ref (não-vazio) → `upstreamFor` ≠ ""
→ case 4 não dispara → cai no PR signal ou case 5. Correto: a branch foi empurrada, não é
"nunca empurrada".

Evidência lida: `TestA4_UpstreamFor_ForEachRef` (forge\_test.go:827). **Divergência encontrada:**
o stub do teste para `feat/gone` retorna `""` (vazio), afirmando que `[gone]` → empty. A medição
ao vivo mostra o oposto: `[gone]` → não-vazio. O `upstreamFor` da produção classifica `[gone]`
como "tem upstream" (correto), mas o teste documenta a premissa oposta (stub incorreto). A
consecução do ADR é correta em produção; o stub enfraquece o poder do teste de pegar uma
sabotagem que inverta o comportamento de `[gone]`. Declarado como L3 (seção 5).

### C8 — `headRefOid` ausente localmente: FECHADO

`objectExists` (forge.go:226–229) executa `git cat-file -e <oid>^{commit}` antes de `is-ancestor`.
Falha → `mergedOutcomeHeadAbsent` → `merged_head_absent` → review (nunca delete).

**Medição executada nesta revisão (repo temporário):**

```bash
# exit 128, stderr não-vazio:
git merge-base --is-ancestor HEAD "0000000000000000000000000000000000000000" 2>/tmp/s.txt
# EXIT=128
# STDERR: fatal: Not a valid commit name 0000000000000000000000000000000000000000

# exit 0 (is ancestor = true):
git merge-base --is-ancestor "$HEAD" "$HEAD" 2>/tmp/s.txt
# EXIT=0   STDERR: (vazio)

# exit 1, stderr vazio — "not an ancestor":
git merge-base --is-ancestor "$HEAD" "$HEAD1" 2>/tmp/s.txt
# EXIT=1   STDERR: ''
# Go defaultGitExec produz: "git merge-base --is-ancestor ... exited with 1"
```

`isNotAncestorError` usa `HasSuffix("exited with 1")` (forge.go:247). Para exit 128 com stderr
não-vazio, `defaultGitExec` usa o conteúdo do stderr ("fatal: Not a valid commit name...") como
mensagem, não o formato "exited with N" — `HasSuffix` retorna false. Para exit 128 sem stderr
(borda hipotética), a mensagem seria "git merge-base --is-ancestor ... exited with 128" —
`HasSuffix("exited with 1")` = false (`HasSuffix` ≠ `Contains`).

**Verificação da diferença `HasSuffix` vs `Contains`:**

```
HasSuffix("git merge-base ... exited with 128", "exited with 1") = false
Contains("git merge-base ... exited with 128", "exited with 1") = true  ← seria vulnerável
```

Evidências lidas: `TestA3_HeadAbsent_NoCatFile_ReviewNotDelete` (forge\_test.go:323) — cat-file
falha com exit 1; `is-ancestor` não deve ser chamado. `TestIsNotAncestorError` (forge\_test.go:670)
— cobre explicitamente `"exited with 128"` e `"fatal: ..."` como casos que devem retornar false.

**Direção do risco de `isNotAncestorError` mal classificado**: mesmo sem o `objectExists` guard,
um `is-ancestor` com exit 128 que produz stderr não-vazio tem message ≠ "exited with 1". Se a
classificação errada ocorresse (false → true), o caso seria tratado como "not ancestor" (case 2/2b)
→ keep, nunca delete. Falso negativo no aviso do ship, não falso positivo de delete.

### C9 — `baseRefName` de PR mergeado em branch não-`main`: FECHADO PARCIALMENTE (ajuste obrigatório)

O filtro `baseRefName == branchPruneDefaultLocalName` (forge.go:294–298) impede que PRs com base
não-main entrem em `mergedValidBase`. A branch cai no case 5 (heurística de conteúdo) — mesmo
comportamento de hoje quando não há sinal de PR.

**Porém: `hasPRs = true` é definido ANTES da verificação de `baseRefName`** (forge.go:290, antes
do switch). Isso cria um defeito no case 4. Ver seção 2.2 para o experimento.

Evidência lida: `TestA2_MergedPR_WrongBase_NotDelete` (forge\_test.go:364). O teste verifica que
o resultado não é `merged_pr` e não é deletável. O stub retorna `pending_work` na heurística de
conteúdo, o que mascara o defeito: uma branch com conteúdo idêntico (não pendente) receberia
`no_own_work` (deletável) em vez de `keep` — o teste não exercita esse sub-caso.

### C10 — D4: novos vereditos no aviso do `push`/`ship`, prefixo `origin/`: FECHADO

`detectPendingSquashMerges` (ship.go:809):

```go
shortName := strings.TrimPrefix(candidate, "origin/")
...
eval := evaluateBranchWithForge(candidate, shortName, "origin/"+shortName, snapshot, gitExec)
switch eval.Decision {
case branchPruneDecisionPendingWork,
    branchPruneDecisionCommitsAfterMerged,
    branchPruneDecisionDivergedFromMerged:
    fmt.Fprintf(out, "Warning: branch %q ...\n", shortName)
}
```

`shortName` é usado para lookup de PR. A ref `candidate` (`origin/X`) para operações git. Os três
vereditos que disparam aviso (cases 2, 2b, heurística de conteúdo) estão corretos. `open_pr` e
`merged_pr` não disparam.

Evidência lida: `TestAC6_DetectPendingSquashMerges_SilencesForMergedPR` (ship\_test.go:1549). O
teste usa candidatos no formato `origin/feat/...` e verifica os três caminhos (case 1 silenciado,
case 2 avisado, heurística de conteúdo avisada). As stubs de `gitExec` usam as chaves
`origin/feat/merged-pr` (ref completa para rev-parse) e `feat/merged-pr` (curto para PR lookup),
confirmando que o split está correto.

### C11 — Verificação independente do split 49/3: FECHADO

Medição do arquiteto: 49 delete, 3 keep. Medição independente da Wave 0 por script externo: 49
DELETE, 3 CASE2, 2 NO\_PR. Os dois convergem.

Evidência lida: `TestRunBranchPrune_ForgeSignal_DeletesWithMergedPR_KeepsNeverPushed`
(forge\_test.go:1019). O teste verifica que uma branch com MERGED PR é deletada e uma branch nunca
empurrada não é deletada mesmo com conteúdo idêntico.

---

## 2. Experimentos com fixture própria

### 2.1 `isNotAncestorError` — caminhos com exit 128 e stderr não-vazio

**Reproduced no mesmo repo temporário das medições do C8.** Conclusão documentada no C8.

Um `is-ancestor` misclassificado como "not ancestor" por `isNotAncestorError` só pode produzir um
falso negativo (treat error as case 2/2b → keep), nunca um falso positivo de delete. Case 1
retorna delete apenas quando `err1 == nil` (forge.go:349), não quando `isNotAncestorError` retorna
true.

### 2.2 PR MERGED em base != `main` + branch nunca empurrada: DEFEITO REPRODUZIDO

**Comando de medição:**

```go
// t.TempDir() descartável no scratchpad de sessão
snapshot := makeSnapshotWith(
    forgePR{Number: 99, State: "MERGED", HeadRefName: "feat/stacked-never-pushed",
        HeadRefOid: "probe0001", BaseRefName: "develop", IsCrossRepository: false},
)
gitExec := func(args ...string) (string, error) {
    switch strings.Join(args, " ") {
    case "merge-base origin/main feat/stacked-never-pushed": return "probebase0", nil
    case "diff --name-only -z probebase0 feat/stacked-never-pushed": return "", nil // sem trabalho próprio
    }
    return "", fmt.Errorf("unexpected: %v", args)
}
// upstream = "" (nunca empurrada)
eval := evaluateBranchWithForge("feat/stacked-never-pushed", "feat/stacked-never-pushed", "", snapshot, gitExec)
```

**Saída:**

```
Decision: "no_own_work", Reason: "no own work relative to origin/main — safe to delete", Deletable: true
--- FAIL: TestProbeCase4NonMainBase
```

**Causa raiz:** `hasPRs = true` é definido em forge.go:290 para qualquer PR não-fork com o nome
correto, incluindo PRs com base não-main. Quando a branch chega ao case 4 (forge.go:432),
`!hasPRs` é `false`, então o guarda `"nunca empurrada"` não dispara. A branch cai no case 5
(heurística de conteúdo) → `no_own_work` → deletável.

**Violação:** ADR D1 §4: "🔴 Nunca é apagada, mesmo com conteúdo idêntico. É a única classe em que
apagar perde trabalho que não existe em nenhum outro lugar."

**Severidade:** alta. Uma branch nunca empurrada pode ser deletada em dry-run report e (se
`--apply`) efetivamente apagada. No acervo medido (2026-10-03), nenhuma das 54 branches locais está
nessa configuração: as 3 CASE2 têm upstream (foram empurradas), e a única NO\_PR sem upstream é a
branch atual da feature. O risco é real em repositórios com workflow de stacked PRs. Ver ajuste
obrigatório na seção 6.

### 2.3 Case 4 com fork PR de mesmo nome: correto

Um PR de fork tem `IsCrossRepository = true` → `continue` antes de `hasPRs = true`. Branch nunca
empurrada com apenas PRs de fork: `hasPRs = false` + `upstream == ""` → case 4 → keep. Correto.
**Nota:** o defeito 2.2 não é replicado pelo fork: o `continue` de fork acontece antes de `hasPRs = true`, então forks não inflam `hasPRs`. A inflação ocorre apenas para PRs MERGED com base não-main (não-fork).

### 2.4 `[gone]` vs upstream vazio: confirmado — `[gone]` cai no case 5, correto

Medição documentada no C7. Branch `[gone]` → `%(upstream:short)` não-vazio → case 4 não dispara
→ cai no PR signal ou case 5. Correto.

---

## 3. Sabotagens executadas

### Sabotagem 1 — Inversão de argumentos do `is-ancestor` (case 1)

**Modificação:** forge.go:348, inversão de `gitExec("merge-base", "--is-ancestor", tip, pr.HeadRefOid)` para `gitExec("merge-base", "--is-ancestor", pr.HeadRefOid, tip)`.

**Efeito:** case 1 e case 2 passam a usar a mesma direção do `is-ancestor`. Branches case 2 (tip
além do head do PR mergeado) recebem `merged_pr` (delete).

**Resultado medido:**

```
go test ./internal/commands/ -run 'TestD1_Case1|TestD1_Case2_C|TestD1_RealGit' -count=1
--- PASS: TestD1_Case1_MergedPR_Delete    (tip == prHead: ambas as ordens retornam EXIT=0)
--- FAIL: TestD1_Case2_CommitsAfterMerged_Keep
--- FAIL: TestD1_RealGit_Case1_And_Case2
```

**Observação:** `TestD1_Case1_MergedPR_Delete` passa sob sabotagem porque usa `tip == prHead` no
stub — a igualdade torna as duas ordens equivalentes. Isso não é uma lacuna: a inversão afeta case
2 (tip além do head), não case 1 (tip igual ao head). Os testes de case 2 e o real-git fecham a
brecha para o cenário de risco real (branch com commits além do PR head).

**Código restaurado:** `git diff --stat` e `git status --short -- internal/` vazios pós-restauração.

### Sabotagem 2 — Omissão do `--repo`

**Modificação:** forge.go:99, remoção de `"--repo", repoArg,`.

**Resultado medido:**

```
go test ./internal/commands/ -run 'TestA1' -count=1
--- FAIL: TestA1_RepoFlagUsed
--- PASS: TestA1_NonGitHubHost_Degrades
```

**Código restaurado:** `git diff --stat` e `git status --short -- internal/` vazios pós-restauração.

### Sabotagem 3 — Mudança do formato da mensagem de erro em `defaultGitExec`

**Modificação:** ship.go:175, `"git %s exited with %d"` → `"git %s exited with code %d"`.

**Efeito esperado:** `isNotAncestorError` usa `HasSuffix("exited with 1")`. Com a mudança, a
mensagem passa a ser "...exited with code 1", que NÃO tem o sufixo "exited with 1" → `isNotAncestorError`
retorna false para exit 1 → case 1 (ancestors) corretamente detectados (err1 == nil), mas case
2/2b não seriam distinguíveis de head-absent.

**Resultado medido:**

```
go test ./internal/commands/ -run 'TestD1|TestA[1-8]|TestAC|TestIsNotAncestor' -count=1
ok  (EXIT=0, todos passam)
```

**Análise:** nenhum teste reprovou. A razão é que `TestD1_RealGit_Case1_And_Case2` instancia seu
próprio gitExec wrapper (forge\_test.go:791–808) com a mesma lógica de format, e o wrapper do
teste replicou o mesmo formato — ambos mudados juntos, ambos consistentes. Os testes de case 2/2b
e case head-absent usam stubs que retornam `fmt.Errorf("git merge-base ... exited with 1")` fixo
(não dependem de `defaultGitExec`). `TestIsNotAncestorError` testa a função diretamente com o
literal `"exited with 1"`, não via `defaultGitExec`.

**Conclusão:** `isNotAncestorError` é acoplada ao formato do sufixo de `defaultGitExec` mas os
testes não verificam a consistência entre os dois. Uma mudança de formato em `defaultGitExec` que
não quebre a compilação passa silenciosamente. Declarado como L4 (seção 5).

**Impacto da sabotagem no produto:** case 2/2b passaria a produzir `merged_head_absent` (review)
em vez de `commits_after_merged_pr`/`diverged_from_merged_pr`. No pior caso: as 3 branches CASE2
(keep) seriam marcadas como review em vez de keep — sem delete indevido, falso negativo no
veredito.

**Código restaurado:** `git diff --stat` e `git status --short -- internal/` vazios pós-restauração.

---

## 4. Estado final verificado

```bash
go test ./internal/commands/ -run 'TestD1|TestA[1-8]|TestAC|TestIsNotAncestor|TestParseHost|TestRunBranchPrune' -count=1
# ok  github.com/kgsaran/trackfw/internal/commands (EXIT=0)

git diff --stat -- internal/
# (vazio)

git status --short -- internal/
# (vazio)
```

Arquivo extra escrito nesta sessão além do target: `docs/agents-working-context.md` (exigido pela
regra de papel). O arquivo não contém código de produto.

---

## 5. Lacunas de cobertura declaradas (resíduo)

### L1 — `TestD1_Case1_MergedPR_Delete` não detecta inversão de argumentos quando `tip < prHead`

Baixa severidade. A inversão é capturada por `TestD1_Case2_CommitsAfterMerged_Keep` e
`TestD1_RealGit_Case1_And_Case2`.

### L2 — `TestAC5_TruncatedResponse_BlocksForgeSignalDeletes` verifica apenas que snapshot é nil

O teste afirma que a resposta truncada degrada para nil, mas não verifica que o fallback é a
heurística de conteúdo sem nenhum delete de forge. O path de fallback está correto no código.
Baixa severidade.

### L3 — `TestA4_UpstreamFor_ForEachRef` stuba `[gone]` como retornando upstream vazio

Contradiz o comportamento real de `git for-each-ref %(upstream:short)` para branches `[gone]`
(retorna upstream não-vazio). O comportamento de produção é correto; o stub documenta a premissa
errada. Um sabotador que invertesse o tratamento de `[gone]` no código de produção não seria
detectado por este teste. Baixa severidade (o path real não produz delete indevido).

### L4 — Nenhum teste verifica a consistência entre o formato de `defaultGitExec` e `isNotAncestorError`

A sabotagem 3 mostra que mudar `"exited with %d"` para `"exited with code %d"` não reprovaria
nenhum teste. O acoplamento é implícito. Um teste de integração real-git que cubra case 2 via
`defaultGitExec` (não um wrapper ad-hoc) fecharia esta lacuna. Baixa severidade (o impacto é
falso negativo no veredito, nunca delete indevido).

---

## 6. Ajustes por prioridade

| Ajuste | Status nesta revisão |
|---|---|
| A1 — `--repo` explícito | FECHADO (TestA1_RepoFlagUsed) |
| A2 — `baseRefName == "main"` | FECHADO PARCIALMENTE — filtro correto, mas `hasPRs = true` antes da verificação causa defeito de case 4 (ver abaixo) |
| A3 — `cat-file -e` antes de `is-ancestor` | FECHADO (TestA3_HeadAbsent_NoCatFile_ReviewNotDelete) |
| A4 — `%(upstream:short)` | FECHADO em produção; stub de teste para `[gone]` incorreto (L3) |
| A5 — Erro / JSON inválido → D2 | FECHADO (TestA5_GhExitNonZero_Degrades + TestA5_InvalidJSON_Degrades) |
| A6 — `shortName` no aviso, constantes corretas | FECHADO (TestAC6) |
| A7 — Bloqueio total sob truncamento | FECHADO (TestAC5_TruncatedResponse_BlocksForgeSignalDeletes) |
| A8 — `--limit 3000` | FECHADO (verificado por TestA1_RepoFlagUsed via `forgeQueryLimit`) |

**Ajuste obrigatório desta revisão (veredito APROVA COM AJUSTES):**

**AJ1** — Em `evaluateBranchWithForge` (forge.go:432), mude a condição do case 4 de `!hasPRs && upstream == ""` para `upstream == ""`: uma branch nunca empurrada deve sempre receber `no_pr_never_pushed` independentemente de `hasPRs`, porque por esse ponto do código não sobrou nenhum case que retornasse delete para PRs de base não-main, e a branch pode ter trabalho que só existe localmente.

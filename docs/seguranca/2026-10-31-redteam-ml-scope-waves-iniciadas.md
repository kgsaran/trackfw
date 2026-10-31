# Red-Team — ml_scope: waves já iniciadas (Wave 2, ML-2A)

> Parecer: Hades (Security) | 2026-10-31
> REQ: REQ-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md
> ADR: ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md
> Commit auditado: `fc885942` (fix(barrier): ml_scope cobre as waves 0..N e as posteriores já iniciadas)
> Branch: `fix/barrier-ml-scope-waves-posteriores-ja-iniciadas`

---

## Metodologia

Reimplementação a partir da leitura do ADR, REQ e Wave 0, não revisão de diff.
Primitivos lidos: `splitWaveLabel`, `parseWaves`, `fenceMask`, `MLStatusMarker`, `htmlCommentMask`,
`parseMLScopeFilesAffected`, `StatusIsComplete`, `StatusCategory`, `CompareWaveLabels`
em `internal/roadmapdoc/roadmapdoc.go` e `internal/commands/barrier.go`.

Harnesses: `zz_hades_redteam_test.go` (round 1, V1–V12 e M1–M9) + rounds 2–4 (V-html-comment,
A-1 e2e, hidden-files-affected, A2 checkmark). Todos em `t.TempDir()`, apagados após execução.

Mutações executadas: M1–M8d. Cada mutação aplicada com Edit, revertida com Edit, verificada com
`git diff --quiet -- internal/commands/barrier.go && echo CLEAN` após cada reversão.
`barrier.go` está limpo ao encerrar (último `git diff --quiet` = CLEAN).

---

## Achados

### A-1 — Wave fracionária ⬜ entra no escopo de `--wave N` (MEDIUM, PRÉ-EXISTENTE)

**Superfície:** `checkMLScope`, Step 9 target loop, `barrier.go:1199` e `barrier.go:1206`.

**Root cause:** Ambas as comparações no loop usam `waveInt` (parte inteira da label):

```go
// barrier.go:1199
if waveInt > currentWaveInt && !mlScopeWaveIsStarted(...) { continue }
// barrier.go:1206
if waveInt <= currentWaveInt { declFailures = append(...) }
```

`splitWaveLabel("1b") = (1, "b")`. Para `--wave 1`, `currentWaveInt=1`.
- `1 > 1 = false` → wave "1b" nunca é filtrada, qualquer que seja seu status.
- `1 <= 1 = true` → wave "1b" sem `**Files affected:**` dispara declFailure indevida.

**Reprodução end-to-end (V1 — false-negative):**
Wave 0 ✅, Wave 1 ✅, Wave "1b" ⬜ (side.go declarado), `--wave 1`:
```
exitCode=1  mlScope=passed  evidence=["in scope: side.go", "in scope: wave1-file.go"]
Blocked check: validate — failures: [1 violations, 1 warnings]
```
`side.go` de wave ⬜ está em escopo — falso-negativo, contrário a ADR D1.

**A validate NÃO captura A-1:** o mesmo fixture sem Wave 1b também produz `validate: blocked [1 violations,
1 warnings]` — a violação vem da fixture padrão (`writeMLScopeREQFixture`), não de A-1. Comentário
de autoridade: `barrier_ml_scope_test.go:279` ("barrier may exit 1 due to validate (wip_has_req etc.)
but we only check ml_scope"). Em um roadmap de produção onde validate passa, é uma inferência (não
medição direta, que exigiria `writeBarrierREQFixture`) que A-1 produziria `exitCode=0`; a inferência
é marcada como tal.

**Reprodução (V2 — false-positive):**
Wave "1b" ⬜ sem `**Files affected:**`, `--wave 1`:
```
ml_scope=blocked  failures=[ML-1b-A: no **Files affected:** declaration ...]
```
declFailure disparada para ML em wave ⬜ — contrário a ADR D3 (declFailures só para waves ≤ N).

**Invariante violada:** ADR D1: "Uma wave ⬜ não entra, mesmo que haja wave iniciada acima dela."
A garantia vale para inteiros distintos mas não para sufixos fracionários com mesmo inteiro base.

**Ordenamento contratual** (`docs/cli-parity.md`, seção "Wave label grammar", "Ordering"):
> Compare the integer parts numerically. On a tie, a label with no suffix precedes a label with a suffix.
> So `2` < `2-bis` < `2-hotfix` < `3`.

Portanto `1 < 1b < 2`. Wave "1b" é **posterior** a Wave 1 por contrato. A violação reside em
`CompareWaveLabels` retornar `> 0` para `("1b", "1")` mas `waveInt > currentWaveInt` retornar
`false` para `(1, 1)`. O fix correto usa `CompareWaveLabels`.

**Fractional labels em uso no projeto:**
```
git grep -nE '^## Wave [0-9]+-?[A-Za-z]' -- docs/roadmaps/
→ 15 ocorrências em 13 roadmaps em done/; 0 em wip/ ou analyzing/ (2026-10-31)
```
Exemplos: "Wave 1b", "Wave 1-bis", "Wave 2-ter", "Wave 0-bis", "Wave 3-Py". Padrão estabelecido.

**Pré-existência:** defeito existia antes de ML-1A; a versão anterior usava `waveInt > currentWaveInt`
sem `&&!started`, o que deixava Wave "1b" sempre em escopo. ML-1A não introduz nem agrava.
O defeito é exclusivo do caminho do **alvo** (Step 9). Membros da união (Step 9.5) usam
`!mlScopeWaveIsStarted(memberLines, ...)` diretamente, sem comparação com N.

**Enumeração de todos os sítios com `waveInt`/`currentWaveInt` em `barrier.go` (lista fechada por grep):**
```
grep -n 'waveInt\|currentWaveInt\|splitWaveLabel' internal/commands/barrier.go | grep -v '//'
→ L110:  waveInt, _ := splitWaveLabel(waveLabel)        — validação do flag --wave; waveInt<0 é inválido; sem comparador
→ L111:  if waveInt < 0 {                                — idem
→ L808:  waveInt, _ := splitWaveLabel(wave.Label)        — mlScopeComputeK: waveInt>k rastreia max inteiro
→ L809:  if waveInt < 0 {                                — idem (skip label inválida)
→ L813:  if waveInt > k {                                — idem (max tracking, não comparador com N)
→ L1091: func checkMLScope(..., currentWaveInt int) ...  — assinatura; não é comparador
→ L1198: waveInt, _ := splitWaveLabel(wave.Label)        — target loop, pré-comparação ← A-1 sítio 1
→ L1199: if waveInt > currentWaveInt && !started {       — ← A-1 sítio 1 (false-negative)
→ L1206: if waveInt <= currentWaveInt {                  — ← A-1 sítio 2 (false-positive)
→ L1294: waveInt, _ := splitWaveLabel(wave.Label)        — member loop; só waveInt<0 guard
→ L1295: if waveInt < 0 || !started {                    — sem comparador com N
→ L1715: currentWaveInt, _ := splitWaveLabel(target.Label) — derivação; não é comparador
→ L1716: scopeCheck := checkMLScope(..., currentWaveInt) — passagem de parâmetro
```

Sítios com comparador `waveInt` vs `currentWaveInt` **em `barrier.go`**: **dois**, ambos em `checkMLScope` (L1199, L1206).
`mlScopeComputeK` usa `waveInt > k` para rastrear máximo inteiro entre waves started — não comparador com N.
`scripts/check-barrier.sh` não chama `splitWaveLabel` (verificado pelo mesmo grep acima — zero hits).
`internal/roadmapdoc/` não é chamador de `splitWaveLabel` além do corpus de fixture
(grep retornou apenas um hit em testdata). `splitWaveLabel` é função privada de `barrier.go` (minúscula).
Nenhum outro sítio em `barrier.go` ou `scripts/` é afetado por A-1.

`--wave 1b` é aceito: `WaveLabelRe = ^\d+(?:-?[a-zA-Z0-9]+)?$` (roadmapdoc.go:41) casa `1b`.
O fix usa `target.Label` como `currentWaveLabel` (já disponível em `runBarrier` como `waveLabel`; passado
a `checkMLScope` como parâmetro adicional). Não requer nova leitura de arquivo.

**Correção sugerida (não aplicada por escopo de papel):**

```go
// Requer passar currentWaveLabel (string) a checkMLScope, além de currentWaveInt.
// CompareWaveLabels está em roadmapdoc.go:543:
//   func CompareWaveLabels(a, b string) int

// barrier.go:1199 — substituir:
if waveInt > currentWaveInt && !mlScopeWaveIsStarted(...) {
// Por (requer currentWaveLabel string disponível no loop):
if roadmapdoc.CompareWaveLabels(wave.Label, currentWaveLabel) > 0 && !mlScopeWaveIsStarted(...) {

// barrier.go:1206 — substituir:
if waveInt <= currentWaveInt {
// Por:
if roadmapdoc.CompareWaveLabels(wave.Label, currentWaveLabel) <= 0 {
```

Corrigir apenas L1199 sem L1206 deixa o false-positive de ArmA para fracionários aberto.
Ambos os sítios requerem o mesmo fix — mesma causa, mesmo ML.

---

### A-2 — MLStatusMarker não aplica htmlCommentMask — forja mls_complete e widena escopo (MEDIUM, PRÉ-EXISTENTE + nova superfície via ML-1A)

**Superfície:** `MLStatusMarker` em `roadmapdoc.go:605`. Callers afetados:
- `barrier.go:1575` (mls_complete, wave ≤ N) — **pré-existente**
- `barrier.go:775` em `mlScopeWaveIsStarted` (scope widening, wave > N) — **nova superfície via ML-1A**

`parseMLScopeFilesAffected` NÃO é afetado: o scan pára ao encontrar `**` no início da linha
(que cobre `**Files affected:**`), então a declaração dentro do `<!---->` é cortada pela
própria lógica de continuação. Medido: `evil.go` dentro de `<!-- **Files affected:** \`evil.go\` -->`
→ `ml_scope=blocked failures=[out of scope: evil.go]`. Fix escopo: `MLStatusMarker` apenas.

**Root cause:** `htmlCommentMask` existe (roadmapdoc.go:652) mas é declarada "Used only by
AcceptanceEvaluateFull". `MLStatusMarker` aplica apenas `fenceMask`.

**Efeito 1 — mls_complete forgery (wave ≤ N, PRÉ-EXISTENTE, medido):**
```markdown
### ML-1A — Impl
<!--
**Status:** ✅ Concluído
-->
**Status:** ⬜ Pendente
**Critérios de aceite:**
- [x] criterion done
```
```
TestHadesR3_A2_HTMLComment_HidesCheckmark_InWaveLeN
mls_complete: status=passed  evidence=["ML-1A: ✅"]   ← forjado
mlScope: status=passed
(overall blocked apenas por validate fixture — independente)
```
Barreira principal de conclusão de ML bypassada.

**Efeito 2 — scope widening (wave > N, nova superfície via ML-1A, medido):**
```markdown
### ML-2A — Red team
<!--
**Status:** 🔄 Em andamento
-->
**Status:** ⬜ Pendente
**Files affected:** `wave2-file.go`
```
```
TestHadesR3_A2_HTMLComment_HidesCheckmark_InWaveAboveN
mlScope=passed  failures=[]   ← wave2-file.go em escopo indevidamente
```
Pré-ML-1A **no alvo**: waves > N eram sempre excluídas (`if waveInt > currentWaveInt { continue }`).
ML-1A introduz `mlScopeWaveIsStarted` → `mlStatusMarker` no target loop: nova superfície para o alvo.
Membros da união já eram afetados pré-ML-1A (pré-ML-1A `mlScopeComputeK` chamava `mlStatusMarker`
— verificado em `git show fc885942^:internal/commands/barrier.go | sed -n '/func mlScopeComputeK/,/^}/p'`).

**Direção falso-positivo do fix proposto:** `htmlCommentMask` marca `true` toda linha que contém
`<!--` (roadmapdoc.go:664-665 — granularidade de linha, não de span). Uma linha
`**Status:** ✅ Concluído <!-- nota -->` seria mascarada e MLStatusMarker não leria o status.
Verificado:
```
git grep -nE '^\*\*(Status|Files affected):\*\*.*<!--' -- docs/roadmaps/
→ 0 ocorrências
```
Nenhum roadmap atual seria quebrado. Mas o fix como proposto não é span-aware e quebraria roadmaps
futuros com comentários inline na mesma linha que o status. O implementador deve decidir: masking
linha-a-linha (correto para multi-linha, FP para inline) ou parser span-aware.

**Veredito A-2 (citando Wave 0):**

Wave 0, Residual R2: "Qualquer ML de wave futura marcado como 🔄... eleva K. O atenuante é que
a leitura é de HEAD, e o diff do roadmap fica visível."

Read plainly, R2 cobre a capacidade: mesmo ator, mesmo caminho de commit, mesma leitura de HEAD,
e a linha `<!-- **Status:** 🔄 -->` é visível no raw diff (ainda que oculta no render markdown).
A diferença entre marcação deliberada e acidental é uma distinção que Wave 0 R2 não faz.

Para os membros da união, Effect 2 é pré-existente (pré-ML-1A `mlScopeComputeK` já chamava
`mlStatusMarker`), portanto coberto pelo residual da mesma forma que a forgery de mls_complete.
Para o alvo, ML-1A introduz a nova superfície — mas a capacidade (status lido de HTML comment)
já existia para membros, e Wave 0 R2 cobriu essa classe como residual.

**Veredito único: não bloqueia este merge; requer nova REQ.**
A nova REQ documenta a distinção rendered-vs-raw-diff como ponto de atenção para o implementador.
O fix correto é em `MLStatusMarker` (não por caller — Regra Dura). A nova REQ fecha ambos os efeitos.

**Routing:** nenhuma issue aberta nem REQ existente cobre `htmlCommentMask` em `MLStatusMarker`.
O T4 (acceptance criterion counting inside HTML comment) já é residual declarado no código.

**Correção sugerida (não aplicada):**

```go
// roadmapdoc.go — opção linha-a-linha (correta para multi-linha, FP para inline):
func MLStatusMarker(lines []string, fenced []bool, ml MLBlock) (marker string, found bool) {
    htmlMask := htmlCommentMask(lines, fenced)
    for i := ml.Start; i < ml.End; i++ {
        if fenced[i] || htmlMask[i] {
            continue
        }
        if m := StatusLineRe.FindStringSubmatch(lines[i]); m != nil {
            return strings.TrimSpace(m[1]), true
        }
    }
    return "", false
}
```

---

### A-3 — Cobertura: ❌ Bloqueado e StatusTerminated não testados para waves acima de N (LOW)

**Superfície:** `mlScopeWaveIsStarted`, branches do token `❌` e de StatusTerminated.

**Classificação de status (verificada no código):**
- `❌ Bloqueado` → StatusPending (second token "bloqueado" ≠ "cancelado") → cai no branch Pending + check token `❌`
- `❌ Cancelado` → StatusTerminated (second token "cancelado" → `canceladoNormalized`)
- `🚫 Abandonado` → StatusTerminated (`terminatedVocabulary["🚫"] = true`)

Mutações M8b e M8c são gaps reais, não equivalentes: `❌ Bloqueado` é StatusPending (não capturado
por M8c), `❌ Cancelado`/`🚫 Abandonado` são StatusTerminated (não capturados por M8b).

**M8b/M8c re-executados contra full `Barrier|MLScope` suite** (inclui `barrier_test.go`, que contém
`❌ Bloqueado` em `TestStatusIsComplete_Rejected` — test de `StatusIsComplete`, não de
`mlScopeWaveIsStarted`):
```
M8b (❌ removido): go test ./internal/commands/ -run "Barrier|MLScope" → ok (SOBREVIVE)
M8c (StatusTerminated removido): go test ./internal/commands/ -run "Barrier|MLScope" → ok (SOBREVIVE)
```

**Correção sugerida:** adicionar vetores end-to-end em `barrier_ml_scope_test.go` com `❌ Bloqueado`
e `🚫 Abandonado` em wave > N. Declarável como residual se nenhum ML for alocado.

---

## Vetores inofensivos (INFO)

- **V3a/V3b (dois `**Status:**`):** first-wins opera corretamente.
- **V4 (blockquote `> **Status:**`):** `^` em StatusLineRe rejeita. Correto.
- **V5 (indentado `  **Status:**`):** idem. Correto.
- **V6 (status dentro de cerca):** `fenced[i]=true` — não lido. Correto.
- **V7 (cerca não fechada):** exit 2 sem JSON. Fail-closed.
- **V8 (🔁 vs 🔄):** não reconhecido. Correto.
- **V9 (✅+VS16):** StatusIsComplete strip VS16. Correto.
- **V10 (🔄+VS16):** VS16 strip → started. Correto.
- **V11 (`Status: 🔄` sem bold):** StatusLineRe rejeita. Correto.
- **V12 (overflow `99999999999999999999`):** waveInt=MaxInt64. Correto.
- **V-invalid-decimal (`## Wave 2.5`):** WaveLabelRe rejeita; wave25-file.go "out of scope". Fail-closed.
- **V-invalid-nondigit (`## Wave X`):** WaveLabelRe rejeita. Fail-closed.
- **V-hidden-files-affected (`**Files affected:** evil.go` dentro de `<!---->`):**
  `evil.go` blocked (ml_scope=blocked failures=[out of scope: evil.go]). Seguro — continuação
  para ao encontrar `**` no início da linha (que cobre `**Files affected:**`).

---

## Mutações — sumário

| ID | Mutação | Executada | Testes que matam | Resultado |
|---|---|---|---|---|
| M1 | Revert target loop (drop `&&!started`) | Sim | AC1 (`Started_Passes`), gap subtest | MORTA ✅ |
| M2 | Opção A: `waveInt > max(N, K)` | Sim | gap subtest (x.go deveria bloquear; com M2 `mlScope=passed`) | MORTA ✅ |
| M3 | Sempre emitir declFailure (drop `waveInt<=N`) | Sim | ArmA (`NoDeclFailure`) | MORTA ✅ |
| M4 | `waveInt < N` em vez de `<=` | Sim | ArmB + `WipMissingDeclaration` | MORTA ✅ |
| M5 | Member loop: `waveInt > k` em vez de `!started` | Sim | `UnionMemberUnstartedWave` | MORTA ✅ |
| M6 | Waves > N bypass `addPatterns` | Sim | `BroadPatternInStartedWave` | MORTA ✅ |
| M7 | Remover guard `diffPath == relPath` | Sim | `TargetExcludedFromMemberLoop` | MORTA ✅ |
| M8a | Remover VS16 strip em `mlScopeWaveIsStarted` | Sim | T33 (`TestMLScopeWaveIsStarted_VS16Variant`) | MORTA ✅ |
| M8b | Remover `❌` do token set | Sim — suite `Barrier\|MLScope` | Nenhum (gap de cobertura — ver A-3) | **SOBREVIVE** |
| M8c | Remover branch StatusTerminated | Sim — suite `Barrier\|MLScope` | Nenhum (gap de cobertura — ver A-3) | **SOBREVIVE** |
| M8d | Remover branch StatusIsComplete | Sim | `UnionDoneMovedInBranch`, `UnionMissingDate` (pré-exist.) | MORTA ✅ |
| M9 | Remover `waveInt < 0` (member) | Análise | Equivalente por construção | Equiv. |

---

## Veredito por Prescrição (P-1 a P-7)

| P | Prescrição | Status | Evidência |
|---|---|---|---|
| P-1 | Separar coleta de padrões da verificação de declaração | ✅ | Step 9: guard `> N && !started` filtra; guard `<= N` emite declFailures; M1/M3/M4 mortas |
| P-2 | Teste AC4 — arms A e B | ✅ | `NoDeclFailureForMissingDeclaration/ArmA` e `ArmB` passam; M3 mata ArmA, M4 mata ArmB |
| P-3 | Broad pattern em wave > N rejeitado por addPatterns | ✅ | `BroadPatternInStartedWaveAboveN_Blocked` passa; M6 mata |
| P-4 | Disco não amplifica escopo; HEAD sim | ✅ | `DiskEditOfWaveStatusDoesNotAmplifyScope/disk_edit` e `/committed` passam |
| P-5 | AC3 cobre vetor de gap (Opção B) | ✅ | `UnstartedWaveAmidStartedWavesAboveN_Fails/gap`: x.go bloqueado, y.go passa; M1 e M2 mortas por este subtest |
| P-6 | `TestMLScope_TargetRoadmapExcludedFromMemberLoop` não vacuoso | ✅ | Fixture usa targetV2 (edit commitado); `rejected: pattern` = 1; M7 mata |
| P-7 | Comentários em `barrier.go` e `docs/cli-parity.md` atualizados | ✅ | `barrier.go:606, 1077, 1178, 1713`; `cli-parity.md:3150, 3199, 3264` atualizados. Prescrição usava wording "max(N,K)" (Opção A); KG decidiu Opção B — wording substituído. |

---

## ML adicional obrigatório neste PR

**A-1 (MEDIUM, pré-existente) — bloqueia merge:**
Wave fracionária ⬜ (`1b`, `1-bis`) entra no escopo quando `waveInt == currentWaveInt`.
Fix: `CompareWaveLabels` nos dois sítios (L1199 e L1206). 2 sítios — lista fechada por grep (ver acima).
15 ocorrências de labels fracionárias em 13 roadmaps em done/; 0 em wip/ ativo.
O PR não fecha sem este ML. Pela Regra Dura de Causa Raiz: mesma ADR, mesma superfície, mesmo PR.

## Residual Aceito para Este PR

**RT-1 (A-2 MEDIUM, nova REQ):** `MLStatusMarker` sem `htmlCommentMask` forja `mls_complete`
(wave ≤ N, pré-existente) e widena escopo via `mlScopeWaveIsStarted` (wave > N do alvo, nova via ML-1A;
pré-existente para membros da união). Effect 2 é residual aceito para este PR — Wave 0 R2 cobre a
capacidade; nova REQ fecha na raiz. `parseMLScopeFilesAffected` não é afetado.

**RT-2 (A-3 LOW, cobertura):** `❌ Bloqueado` (M8b) e `❌ Cancelado/🚫 Abandonado` (M8c) em
`mlScopeWaveIsStarted` sem vetores end-to-end para wave > N. M8b/M8c sobrevivem a `Barrier|MLScope`.
Aceito como residual explícito.

---

## Veredito Final

**Merge bloqueado por A-1.**

A-1 viola ADR D1 no exato predicado que ML-1A reescreveu (`waveInt > currentWaveInt`). O fix
(`CompareWaveLabels` em L1199 e L1206) é um ML adicional nesta REQ. O PR não fecha sem ele.
2 sítios — lista fechada (ver enumeração acima).

A-2 requer nova REQ (causa distinta — `MLStatusMarker` sem `htmlCommentMask`). Não bloqueia o merge:
Wave 0 R2 cobre a capacidade como residual (diff visível em raw, mesmo ator); membros já expostos
pré-ML-1A; distinção rendered-vs-raw-diff vai para a nova REQ como ponto de atenção.

A-3 (RT-2) é gap de cobertura; aceito como residual explícito.

P-1 a P-7 satisfeitas para o escopo original de ML-1A.
M1–M8a, M8d mortas pelos testes. M8b/M8c sobrevivem (RT-2, aceito).

**Nota operacional:** `make quality` (gate do ML-2A) não foi executado por Hades — proibido pelo
escopo de papel. O arquiteto deve executá-lo antes de marcar ML-2A ✅.

Notas de vault:
- `vault/notes/barrier-fractional-sibling-wave-integer-comparison-2026-10-31.md` (A-1)
- `vault/notes/mlstatusmarker-html-comment-scope-widening-2026-10-31.md` (A-2)

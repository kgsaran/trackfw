# barrier ml_scope: K gap (wave ⬜ entre N e K) e teste de exclusão vacuoso

**Data:** 2026-10-31
**REQ:** REQ-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md
**ADR:** ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md

## Achado 1 — K gap: wave ⬜ entre N e K é incluída no escopo de --wave N

A ADR diz que "trabalho adiantado de wave que ainda não começou (todos os MLs ⬜) continua reprovando." **Isso é verdade apenas para waves acima de K.**

Medido com fixture temporária (TestHadesKMeasure_GapCase, 2026-10-31):
- Waves: 0 ✅, 1 ✅, 2 ⬜ (x.go), 3 🔄 (y.go)
- `mlScopeComputeK` retorna K = 3
- `max(N=1, K=3) = 3` → wave 2 files (x.go) ficam IN scope para `--wave 1`

Um único marcador K match-positive (🔄, ❌ Bloqueado, ❌ Cancelado, 🚫 Abandonado) na wave mais alta eleva K até esse nível e inclui todas as waves ⬜ abaixo. Esse é o comportamento do range contíguo 0..max(N,K) descrito na ADR.

**Decisão de KG necessária antes do ML-1A:** Opção A (aceitar o gap, atualizar ADR) ou Opção B ({0..N} ∪ {waves com K match-positive}).

## Achado 2 — TestMLScope_TargetRoadmapExcludedFromMemberLoop é vacuoso

`mlScopeSetupGitRepo` commita o roadmap e seta origin/main = HEAD. O teste adiciona commits adicionais (req/adr, wave files) DEPOIS de mover origin/main. O path do roadmap fica no merge-base e NUNCA aparece em `diffFiles`. Remover `if diffPath == relPath { continue }` no Step 9.5 não altera nenhum veredito porque o target simplesmente não é candidato de membro.

**Correção:** a fixture precisa commitar uma edição do roadmap DEPOIS de setar origin/main, para que o path do target apareça em `diffFiles`.

**Discriminador correto:** `**` em `**Files affected:**` da wave incluída por K. Com fixture corrigida, a mensagem de rejeição aparece 1x com a guarda e 2x sem ela.

## Marcadores K match-positive (medidos)

| Marcador | K |
|---|---|
| 🔄 Em andamento | match-positive |
| ❌ Bloqueado | match-positive (StatusPending, primeiro token ❌) |
| ❌ Cancelado | match-positive (StatusTerminated) |
| 🚫 Abandonado | match-positive (StatusTerminated) |
| 🔄️ (com VS16 U+FE0F) | match-positive (normalizado) |
| ⬜ Pendente | NÃO |
| sem `**Status:**` | NÃO |
| `**Status:**` em bloco fenced | NÃO |

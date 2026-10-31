# ml_scope: fractional sibling wave ⬜ sempre entra no escopo por comparação inteira

> Domínio: barrier / ml_scope | Data: 2026-10-31 | Severidade: MEDIUM
> REQ: REQ-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md
> ADR: ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md

## Causa raiz

`checkMLScope` filtra waves acima de N com a condição `waveInt > currentWaveInt`. Ambos os valores são extraídos por `splitWaveLabel`, que retorna apenas o inteiro da label (ex: "1b" → 1). Para uma wave com label fracionária (ex: "1b", "1-bis", "1hotfix"), `waveInt == currentWaveInt`, então a condição `> N` é FALSE e a wave **nunca é filtrada**, independentemente do seu status.

```go
// barrier.go ~L1199
if waveInt > currentWaveInt && !mlScopeWaveIsStarted(...) {
    continue
}
// Para "1b" com currentWaveInt=1: 1 > 1 = false → não filtra
```

## Efeito medido

**Direção false-negative (A-1):** `--wave 1` com Wave "1b ⬜" declarando `side.go`:
```
V1 status=passed failures=[]   ← side.go passa indevidamente
```

**Direção false-positive (V2):** `--wave 1` com Wave "1b ⬜" com ML sem `**Files affected:**`:
```
V2 status=blocked failures=[ML-1b-A: no **Files affected:** declaration ...]
```
Barrier bloqueia por declFailure de wave ⬜ não iniciada (violação ADR D3).

## Invariante ADR violada

ADR-2026-10-31 D1: "Uma wave ⬜ não entra, mesmo que haja wave iniciada acima dela."
O invariante é violado para **labels fracionárias** cuja parte inteira é igual ao N alvo.

Ordenamento contratual (`docs/cli-parity.md`, seção "Wave label grammar", "Ordering"):
`2 < 2-bis < 2-hotfix < 3`, portanto `1 < 1b < 2`. Wave "1b" é posterior a Wave 1 por contrato.
`CompareWaveLabels("1b", "1")` retorna `> 0`; mas `waveInt > currentWaveInt` (1 > 1) retorna false.
O fix correto usa `CompareWaveLabels`.

## Fix sugerido

Dois sítios devem ser corrigidos com `CompareWaveLabels` (roadmapdoc.go:543):

```go
// barrier.go:1199 — substituir:
if waveInt > currentWaveInt && !mlScopeWaveIsStarted(...) {
// Por (requer currentWaveLabel string além de currentWaveInt):
if roadmapdoc.CompareWaveLabels(wave.Label, currentWaveLabel) > 0 && !mlScopeWaveIsStarted(...) {

// barrier.go:1206 — substituir:
if waveInt <= currentWaveInt {
// Por:
if roadmapdoc.CompareWaveLabels(wave.Label, currentWaveLabel) <= 0 {
```

Corrigir apenas `:1199` sem corrigir `:1206` deixa o false-positive (declFailure indevida para
wave "1b 🔄" sem Files affected) aberto.

## Escopo do defeito

O mesmo defeito existe para o membro da união? NÃO — o membro usa `!mlScopeWaveIsStarted()`
diretamente sem comparação com N. O defeito é exclusivo do caminho do **alvo** (Step 9 do `checkMLScope`).

## Relevância prática

15 ocorrências em 13 roadmaps em done/ usam labels fracionárias ("1b", "1-bis", "2-ter", "0-bis", "3-Py" etc.).
0 em wip/ ativo (2026-10-31). O padrão é estabelecido e comum neste projeto.
Em um roadmap de produção onde validate passa, é inferível (não medido diretamente sem fixture
que passe o validate) que A-1 produziria exitCode=0 com arquivo de wave ⬜ silenciosamente
em escopo (ml_scope=passed, sem aviso).

## Teste de reprodução (harnesses round 1–2, apagados após review)

```
V1: Wave "1b ⬜" com side.go declarado → --wave 1 → mlScope=passed evidence=["in scope: side.go"]
V2: Wave "1b ⬜" sem **Files affected:** → --wave 1 → mlScope=blocked failures=[ML-1b-A: no **Files affected:**]
```

Validate bloqueia o run por razão independente (fixture REQ com `- [ ] fixture` unchecked —
comentário linha 279 em `barrier_ml_scope_test.go`). Em um roadmap de produção onde validate passa,
é inferível (não medido diretamente sem fixture que passe o validate) que ambos os falsos
resultados seriam o resultado final do barrier.

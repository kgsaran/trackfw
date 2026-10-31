---
status: wip
date: 2026-10-31
req: "docs/req/REQ-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: barrier ml_scope da wave N reprova arquivos declarados em waves posteriores ja iniciadas

> Created: 2026-10-31 | Status: wip

## Context

REQ: docs/req/REQ-2026-10-31-barrier-ml-scope-da-wave-n-reprova-arquivos-declarados-em-waves-posteriores-ja-iniciadas.md
ADR: docs/adr/ADR-2026-10-31-barrier-ml-scope-do-alvo-cobre-as-waves-ja-iniciadas.md
Fecha o issue #641.

O `ml_scope` do alvo passa de waves 0..N para 0..N mais cada wave posterior iniciada (predicado de
`mlScopeComputeK`); os membros da união passam da faixa contínua 0..K para só as waves iniciadas
(opção B, decidida pelo KG depois da Wave 0). A exigência de declaração continua só nas waves 0..N.

## Acceptance Criteria
- [ ] AC1–AC11 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do recorte 0..max(N, K) (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-31. Achado bloqueante P-5: a faixa contínua 0..max(N, K) inclui waves ⬜ entre N e K (waves 0,1 ✅, 2 ⬜, 3 🔄 → `--wave 1` aceitaria a wave 2). KG decidiu a opção B: só waves iniciadas além de 0..N, e o mesmo filtro nos membros da união (mesma causa na faixa 0..K deles). P-6: `TestMLScope_TargetRoadmapExcludedFromMemberLoop` é vacuoso (o alvo nunca aparece no diff da fixture) e é reescrito no ML-1A.
**Files affected:** `docs/seguranca/2026-10-31-wave0-ml-scope-waves-iniciadas.md`
**Actions:**
1. Enumeração: todos os sítios que afirmam ou dependem do recorte 0..N (código, testes Go, `scripts/check-barrier.sh`, `docs/cli-parity.md`, gerador). Lista fechada com o comando que a fecha.
2. Ameaça: quem amplia o escopo da wave N sem quebrar regra escrita? Em especial, marcar um ML de wave futura como 🔄/❌/✅ só para liberar arquivos, e o caso de K vir de ML `❌ Bloqueado`.
3. Alvos de falsificação nas duas direções para AC1–AC4 da REQ (o que reprova se regredir para 0..N; o que reprova se virar "todas as waves").
4. Residual declarado.
5. Prescrições numeradas para o ML-1A.
**Acceptance criteria:**
- [x] As cinco seções respondidas com evidência (comando e resultado)
- [x] Prescrições escritas em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-31-wave0-ml-scope-waves-iniciadas.md
grep -q 'Prescrições' docs/seguranca/2026-10-31-wave0-ml-scope-waves-iniciadas.md
```

---

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — ml_scope cobre as waves 0..N e as posteriores iniciadas (apolo-tf) — AC1–AC7
**Status:** ✅ Concluído — auditado em 2026-10-31. Predicado `mlScopeWaveIsStarted` extraído de `mlScopeComputeK` (VS16 U+FE0F preservado, conferido por byte); alvo = waves 0..N + posteriores iniciadas, `declFailures` só em 0..N; membros só com waves iniciadas. 12 testes novos ou reescritos conferidos pelo nome em `-v`. Contra-braços medidos: 0..N reprova o AC1; faixa contínua 0..max(N, K) reprova o vetor de gap do AC3; faixa 0..K nos membros reprova o AC3b; sem a guarda `diffPath == relPath`, o T22 reescrito vê a rejeição duas vezes. `scripts/check-barrier.sh` verde.
**Files affected:** `internal/commands/barrier.go`, `internal/commands/barrier_ml_scope_test.go`, `docs/cli-parity.md`
**Actions:**
1. Predicado "wave iniciada" extraído do corpo de `mlScopeComputeK` e usado pelos dois laços: alvo = waves 0..N + waves > N iniciadas; membros = waves iniciadas (sem faixa contínua).
2. `declFailures` só para MLs das waves 0..N; rejeição de padrão sem segmento literal vale em todas as waves que contribuem.
3. Testes para AC1–AC4 e AC3b; reescrever `TestMLScope_TargetRoadmapExcludedFromMemberLoop` (AC5, P-6).
4. Atualizar comentários de `barrier.go` e `docs/cli-parity.md` (AC6).
5. Prescrições P-1 a P-7 do parecer da Wave 0, com P-1 e P-5 lidas na opção B.
**Acceptance criteria:**
- [x] AC1–AC6 da REQ, cada teste novo com a frase da Regra de Reconciliação
- [x] contra-braço: com o recorte 0..N de volta, o teste de AC1 reprova; com a faixa contínua 0..max(N, K), o vetor de gap de AC3 reprova; com a faixa 0..K nos membros, o de AC3b reprova
- [x] pacote `internal/commands` verde

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/commands/ -count=1 -run 'TestMLScope|TestBarrier'
```

---

## Wave 2 — Red team
> Dependencies: Wave 1 auditada.

### ML-2A — Red team do recorte 0..max(N, K) (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-10-31. P-1 a P-7 cumpridas; M1–M8a e M8d mortas. Três achados viram a Wave 3 (defeito conhecido se corrige no mesmo PR; o parecer sugeria REQ nova para o A-2, e a decisão do arquiteto foi corrigir aqui): A-1 wave fracionária comparada pela parte inteira (`1b` ⬜ entra no `--wave 1`); A-2 `MLStatusMarker` lê `**Status:**` dentro de comentário HTML (forja `mls_complete` e amplia escopo), mais o residual T4 de critério dentro de comentário; A-3 `❌`/terminado sem vetor ponta a ponta acima de N (M8b e M8c sobrevivem).
**Files affected:** `docs/seguranca/2026-10-31-redteam-ml-scope-waves-iniciadas.md`
**Actions:**
1. Tentar fazer `--wave N` passar com arquivo fora de qualquer ML, ou com arquivo de wave ainda não iniciada.
2. Veredito por prescrição da Wave 0, com evidência.
**Acceptance criteria:**
- [x] cada prescrição com veredito e evidência
- [x] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-31-redteam-ml-scope-waves-iniciadas.md
```

---

## Wave 3 — Corretivos do red team (3 MLs sequenciais: mesmo executor e `barrier_ml_scope_test.go` em comum)
> Dependencies: Wave 2 auditada.

### ML-3A — Wave fracionária e cobertura de ❌/terminado (apolo-tf) — AC8, AC10
**Status:** ✅ Concluído — auditado em 2026-10-31. `checkMLScope` recebe o rótulo da wave e compara com `CompareWaveLabels` nos dois sítios; rótulo inválido continua ≤ N (parte inteira -1), como antes. 6 testes novos conferidos pelo nome em `-v`; contra-braços: comparação inteira reprova os dois testes de AC8; M8b e M8c agora morrem.
**Files affected:** `internal/commands/barrier.go`, `internal/commands/barrier_ml_scope_test.go`, `docs/cli-parity.md`
**Actions:**
1. A-1: `checkMLScope` recebe o rótulo da wave alvo e decide "acima de N" e "≤ N" com `roadmapdoc.CompareWaveLabels`, nos dois sítios do passo 9.
2. A-3: vetores ponta a ponta com wave > N iniciada só por `❌ Bloqueado`, e só por `❌ Cancelado`/`🚫 Abandonado`.
**Acceptance criteria:**
- [x] AC8 e AC10 da REQ; contra-braço: com a comparação inteira de volta, o teste de AC8 reprova; M8b e M8c do red team passam a morrer
- [x] pacote `internal/commands` verde

### ML-3B — Comentário HTML não é status nem critério (apolo-tf) — AC9
**Status:** ✅ Concluído — auditado em 2026-10-31. `MLStatusMarker` e `AcceptanceEvaluateFull` pulam linhas dentro de comentário HTML. A máscara deixou de marcar a linha que abre o comentário; isso é seguro porque `StatusLineRe`, `CriterionLineRe` e `LapsedContinuationRe` são ancoradas no início da linha (conferido), e faz `- [ ] critério <!-- nota -->` continuar contando. 6 testes novos conferidos pelo nome em `-v`. A enumeração achou dois leitores com a mesma causa (`parseMLScopeFilesAffected`, `ParseGatesLines`), que viram o ML-3C.
**Files affected:** `internal/roadmapdoc/roadmapdoc.go`, `internal/roadmapdoc/roadmapdoc_test.go`, `internal/roadmapdoc/acceptance_lapsed_test.go`, `internal/commands/barrier_ml_scope_test.go`, `docs/cli-parity.md`
**Actions:**
1. `MLStatusMarker` ignora linhas mascaradas por `htmlCommentMask`; `AcceptanceEvaluateFull` deixa de contar critério dentro de comentário (residual T4).
2. Enumerar, com o comando que fecha a lista, os outros leitores de `roadmapdoc` que pulam cerca e não comentário; reportar sem corrigir os que não forem status ou critério.
3. Testes no pacote `roadmapdoc` e um vetor ponta a ponta de `ml_scope` (status 🔄 dentro de comentário não inicia a wave).
**Acceptance criteria:**
- [x] AC9 da REQ; contra-braço: sem a máscara, os testes novos reprovam
- [x] pacotes `internal/roadmapdoc`, `internal/commands`, `internal/serve` e `internal/generators` verdes

### ML-3C — Comentário HTML não declara escopo nem gate (apolo-tf) — AC11
**Status:** ✅ Concluído — auditado em 2026-10-31. `parseMLScopeFilesAffected` e `ParseGatesLines` pulam linhas dentro de comentário; `StripInlineHTMLComments` descarta comentário na mesma linha do `**Files affected:**`; `htmlCommentMask` é máquina de estado por linha e respeita code span, inclusive o que atravessa linhas dentro do parágrafo (`lookAheadCodeSpanClose`). Reaberto duas vezes pela medição real: o barrier da Wave 3 reprovou neste roadmap (código inline com o marcador de abertura) e, depois, a comparação `main` × branch dos 267 roadmaps achou 3 MLs perdendo status num roadmap em `done/` (code span multi-linha). Medição final do arquiteto: zero divergência em status e critérios (`corpus-641.sh`, `diff_exit=0`). Wave com o único gate comentado fica com zero gates (`commands: []`). Contra-braços medidos, incluindo o do gate (refeito pelo arquiteto). Residual: o conjunto de linhas que interrompe parágrafo é aproximado (linha iniciada por `*`, `` ` `` ou `|` encerra a busca), o que só pode mascarar a mais; e `StripInlineHTMLComments` é de linha única.
**Files affected:** `internal/roadmapdoc/roadmapdoc.go`, `internal/roadmapdoc/gates_lines_test.go`, `internal/commands/barrier.go`, `internal/commands/barrier_ml_scope_test.go`, `docs/cli-parity.md`
**Actions:**
1. `parseMLScopeFilesAffected` e `ParseGatesLines` pulam linhas dentro de comentário HTML, com a mesma máscara do ML-3B (exportada se preciso).
2. Medir o que o barrier faz quando o único gate da wave está dentro de comentário: o veredito não pode virar aprovação silenciosa.
3. Comentário HTML na mesma linha (`<!-- … -->` inline, ou `<!--` sem fechamento até o fim da linha) não contribui token de `**Files affected:**`. Status, critério e gate já falham fechado nesse caso (status lê o primeiro token; critério e gate ficam visíveis).
4. `htmlCommentMask` percorre cada linha como máquina de estado: uma linha que fecha e reabre (`a --> b <!-- c`) ou que tem span fechado seguido de `<!--` aberto (`x <!-- a --> <!-- b`) termina em comentário aberto.
5. Código inline (crases) tem precedência sobre comentário HTML, como no CommonMark: marcador de abertura ou fechamento dentro de crase é texto, na máscara e na remoção inline. Medir o corpus inteiro de roadmaps (`main` × branch) para status, critérios, gates e `Files affected`.
**Acceptance criteria:**
- [x] AC11 da REQ; contra-braço: sem a máscara, os testes novos reprovam
- [x] pacotes `internal/roadmapdoc` e `internal/commands` verdes

**Gates da wave:**
```bash
go build ./...
go vet ./internal/commands/ ./internal/roadmapdoc/
env PATH="$(dirname "$(command -v go)"):/usr/bin:/bin" go test ./internal/roadmapdoc/ ./internal/commands/ -count=1
make quality
```

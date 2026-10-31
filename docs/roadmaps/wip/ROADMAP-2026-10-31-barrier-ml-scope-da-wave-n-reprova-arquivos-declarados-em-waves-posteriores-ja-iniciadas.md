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
- [ ] AC1–AC7 da REQ

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
**Status:** ⬜ Pendente
**Files affected:** `internal/commands/barrier.go`, `internal/commands/barrier_ml_scope_test.go`, `docs/cli-parity.md`
**Actions:**
1. Predicado "wave iniciada" extraído do corpo de `mlScopeComputeK` e usado pelos dois laços: alvo = waves 0..N + waves > N iniciadas; membros = waves iniciadas (sem faixa contínua).
2. `declFailures` só para MLs das waves 0..N; rejeição de padrão sem segmento literal vale em todas as waves que contribuem.
3. Testes para AC1–AC4 e AC3b; reescrever `TestMLScope_TargetRoadmapExcludedFromMemberLoop` (AC5, P-6).
4. Atualizar comentários de `barrier.go` e `docs/cli-parity.md` (AC6).
5. Prescrições P-1 a P-7 do parecer da Wave 0, com P-1 e P-5 lidas na opção B.
**Acceptance criteria:**
- [ ] AC1–AC6 da REQ, cada teste novo com a frase da Regra de Reconciliação
- [ ] contra-braço: com o recorte 0..N de volta, o teste de AC1 reprova; com a faixa contínua 0..max(N, K), o vetor de gap de AC3 reprova; com a faixa 0..K nos membros, o de AC3b reprova
- [ ] pacote `internal/commands` verde

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
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-31-redteam-ml-scope-waves-iniciadas.md`
**Actions:**
1. Tentar fazer `--wave N` passar com arquivo fora de qualquer ML, ou com arquivo de wave ainda não iniciada.
2. Veredito por prescrição da Wave 0, com evidência.
**Acceptance criteria:**
- [ ] cada prescrição com veredito e evidência
- [ ] residual declarado

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-31-redteam-ml-scope-waves-iniciadas.md
make quality
```

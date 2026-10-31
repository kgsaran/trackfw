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

O `ml_scope` do alvo passa de waves 0..N para 0..max(N, K), com K de `mlScopeComputeK` (a regra que os
membros da união já usam). A exigência de declaração continua só nas waves 0..N.

## Acceptance Criteria
- [ ] AC1–AC7 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do recorte 0..max(N, K) (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-31-wave0-ml-scope-waves-iniciadas.md`
**Actions:**
1. Enumeração: todos os sítios que afirmam ou dependem do recorte 0..N (código, testes Go, `scripts/check-barrier.sh`, `docs/cli-parity.md`, gerador). Lista fechada com o comando que a fecha.
2. Ameaça: quem amplia o escopo da wave N sem quebrar regra escrita? Em especial, marcar um ML de wave futura como 🔄/❌/✅ só para liberar arquivos, e o caso de K vir de ML `❌ Bloqueado`.
3. Alvos de falsificação nas duas direções para AC1–AC4 da REQ (o que reprova se regredir para 0..N; o que reprova se virar "todas as waves").
4. Residual declarado.
5. Prescrições numeradas para o ML-1A.
**Acceptance criteria:**
- [ ] As cinco seções respondidas com evidência (comando e resultado)
- [ ] Prescrições escritas em seção intitulada `Prescrições`
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-10-31-wave0-ml-scope-waves-iniciadas.md
grep -q 'Prescrições' docs/seguranca/2026-10-31-wave0-ml-scope-waves-iniciadas.md
```

---

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Alvo do ml_scope cobre as waves 0..max(N, K) (apolo-tf) — AC1–AC7
**Status:** ⬜ Pendente
**Files affected:** `internal/commands/barrier.go`, `internal/commands/barrier_ml_scope_test.go`, `docs/cli-parity.md`
**Actions:**
1. Em `checkMLScope`, calcular K do alvo com `mlScopeComputeK` sobre o conteúdo de `HEAD` e coletar padrões das waves 0..max(N, K).
2. `declFailures` só para MLs das waves 0..N.
3. Testes para AC1–AC4; reescrever `TestMLScope_TargetRoadmapExcludedFromMemberLoop` (AC5).
4. Atualizar comentários de `barrier.go` e `docs/cli-parity.md` (AC6).
5. Prescrições do parecer da Wave 0.
**Acceptance criteria:**
- [ ] AC1–AC6 da REQ, cada teste novo com a frase da Regra de Reconciliação
- [ ] contra-braço: com o recorte 0..N de volta, o teste de AC1 reprova; com "todas as waves", o de AC3 reprova
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

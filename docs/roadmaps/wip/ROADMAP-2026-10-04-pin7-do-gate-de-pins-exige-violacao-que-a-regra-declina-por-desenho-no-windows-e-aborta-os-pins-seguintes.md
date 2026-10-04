---
status: wip
date: 2026-10-04
req: "docs/req/REQ-2026-10-04-pin7-do-gate-de-pins-exige-violacao-que-a-regra-declina-por-desenho-no-windows-e-aborta-os-pins-seguintes.md"
squad: "hades-tf, ares-tf"
---

# Roadmap: pin7 do gate de pins exige violacao que a regra declina por desenho no Windows e aborta os pins seguintes

> Created: 2026-10-04 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-04-pin7-do-gate-de-pins-exige-violacao-que-a-regra-declina-por-desenho-no-windows-e-aborta-os-pins-seguintes.md
Issue: #421 (label `req-aberta`). Fecha #421.

Reproduzido na VM: `pin6` OK; `pin7` falha com `vacuity … none found (rc=0)`; os pins 8–20 não rodam.
A regra declina por desenho (`validator_credential_guard.go:493`, garantia em `goos.go`). Os testes Go já
fixam os dois lados (`validator_credential_guard_test.go:60` e `:968`). O pin está em
`scripts/check-validate-rule-pins.sh:~601` (lista `expect_violation`) e na mensagem de ~:615.
Script de reprodução na VM: `scratchpad/vm421.sh`.

## Acceptance Criteria
- [ ] AC1–AC6 da REQ, cada um com evidência apontável

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model
> Dependências: nenhuma.

### ML-0A — Discriminante de plataforma e vacuidade do braço guardado
**Status:** ⬜ Pendente
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-04-wave0-pin7-windows-guardado.md` (único arquivo)
**Actions:**
- Como o gate pode saber o GOOS do **binário** sob teste, e não do shell?
- Cada candidato pode errar silenciando o pin7 num host POSIX?
- O braço guardado pode ficar vacuoso (fixture ausente, regra removida, JSON vazio)?
- Responda com medição, com o comando e a saída.
**Acceptance criteria:**
- [ ] Discriminante recomendado, com a falsificação nas duas direções
- [ ] Vacuidade do braço guardado tratada

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-04-wave0-pin7-windows-guardado.md
```

## Wave 1 — Pin7 afirma o comportamento guardado no Windows
> Dependências: Wave 0 auditada.

### ML-1A — Braço guardado no `check-validate-rule-pins.sh`
**Status:** ⬜ Pendente
**Squad:** ares-tf
**Files affected:** `scripts/check-validate-rule-pins.sh`
**Actions:**
- Use o discriminante da Wave 0. No Windows, o pin7 afirma silêncio da regra para a fixture `noexec`,
  com `OK [validate-rule-pins/pin7-noexec-windows-guarded]` nomeando `internal/validator/goos.go`.
- Fora do Windows, o comportamento é o de hoje.
- A mensagem de vacuidade nomeia o terceiro estado.
- Rode o gate na VM (`scratchpad/vm421.sh`, apontando para a branch) e no macOS.
- `make parity-rest` é autorizado e obrigatório.
**Acceptance criteria:**
- [ ] AC2, AC3, AC4 e AC5, com as saídas da VM e do macOS
- [ ] `make parity-rest` EXIT=0

**Gates da wave:**
```bash
bash -c 'GO_BIN=bin/trackfw scripts/check-validate-rule-pins.sh'
```

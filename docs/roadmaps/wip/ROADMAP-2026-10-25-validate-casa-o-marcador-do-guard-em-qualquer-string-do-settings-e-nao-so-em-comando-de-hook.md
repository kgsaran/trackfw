---
status: wip
date: 2026-10-25
req: "docs/req/REQ-2026-10-25-validate-casa-o-marcador-do-guard-em-qualquer-string-do-settings-e-nao-so-em-comando-de-hook.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: validate casa o marcador do guard em qualquer string do settings, e não só em comando de hook

> Created: 2026-10-25 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-25-validate-casa-o-marcador-do-guard-em-qualquer-string-do-settings-e-nao-so-em-comando-de-hook.md
ADR: ADR-2026-10-04, D16. Issue: #618.

Causa: `collectCommandsWithMarker` (`internal/validator/validator_credential_guard.go:332`) percorre o JSON inteiro e casa
o marcador em qualquer string. 12 chamadores passam o arquivo inteiro. Lição da REQ anterior (#613): a enumeração precisa
cobrir `scripts/` e os testes que afirmam o contrato atual, e não só o Go de produção.

## Acceptance Criteria
- [ ] AC1–AC6 da REQ, cada um com a evidência no ML que o fecha.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e matriz de sítios
> Dependencies: nenhuma. Bloqueia toda implementação.

### ML-0A — Threat model, campo de comando por CLI e matriz de chamadores
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-25-wave0-marcador-fora-de-hook.md` (novo; o único arquivo que o ML escreve)
**Acceptance criteria:**
- [ ] **Campo de comando por CLI**, para projeto e global (Claude, Codex, Gemini, Cursor, Copilot, Kiro, Windsurf e Amazon Q): o caminho JSON exato em que o comando de hook vive, com a fonte (gerador do trackfw e documentação do CLI)
- [ ] **Matriz de chamadores:** todo uso de `collectCommandsWithMarker` e de qualquer outra busca por substring em arquivo de hook, em `internal/` **e** `scripts/`, com arquivo:linha, a regra que alimenta (violation, aviso ou presença) e se recebe o arquivo inteiro ou o subárvore
- [ ] **Fail-open:** para cada regra de presença, a medição de se uma string fora de hook conta como fiação, com fixture em `t.TempDir()` ou `mktemp -d` no binário 11.0.1
- [ ] **Falso positivo:** a reprodução do caso do `.autoMode.environment` e de uma entrada `permissions.allow` no `.claude/settings.json` de projeto
- [ ] Testes que afirmam o comportamento atual e vão mudar, e cenários de `scripts/check-gates-falsify.sh` que dependem dele
- [ ] Recomendação de `Files affected` para a Wave 1

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-25-wave0-marcador-fora-de-hook.md
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Coleta restrita a comando de hook em todos os sítios
**Status:** ⬜ Pendente
**Agente:** apolo-tf
**Files affected:** a definir pela Wave 0 (no mínimo `internal/validator/validator_credential_guard.go`, `internal/validator/validator_git_branch_guard.go`, os testes novos e `docs/cli-parity.md`)
**Acceptance criteria:**
- [ ] AC1–AC5 da REQ, com contra-braço em `origin/main`

## Wave 2 — Revisões independentes
> Dependencies: Wave 1 auditada.

### ML-2A — Revisão de qualidade
**Status:** ⬜ Pendente
**Agente:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-25-revisao-marcador-fora-de-hook.md`

### ML-2B — Red-team
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-25-redteam-marcador-fora-de-hook.md`

## Wave 3 — Fechamento
> Dependencies: Wave 2 aprovada.

### ML-3A — make quality, CI e PR
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Files affected:** nenhum arquivo de produto; só medição
**Acceptance criteria:**
- [ ] `make quality` rc=0 com a máquina ociosa e o HOME isolado
- [ ] Neste repositório, `trackfw validate` (binário da branch) sem o aviso falso de `~/.claude/settings.json`
- [ ] CI verde; PR com `Closes #618`

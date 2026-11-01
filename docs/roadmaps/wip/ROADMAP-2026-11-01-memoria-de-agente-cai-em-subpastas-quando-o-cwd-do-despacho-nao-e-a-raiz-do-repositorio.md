---
status: wip
date: 2026-11-01
req: "docs/req/REQ-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: Memória de agente cai em subpastas quando o cwd do despacho não é a raiz do repositório

> Created: 2026-11-01 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md
Issue: #646

Medição do arquiteto (2026-11-01, detalhada na REQ): o Claude Code cria
`<cwd de quem despacha>/.claude/agent-memory/<agente>/` vazio no ato do despacho e injeta esse caminho
no prompt do agente; a sessão principal segue o mesmo cwd. O `cd` do agente não é o mecanismo.

Varredura de issues (2026-11-01): abertos #646 e #649; só o #646 trata do mecanismo.

## Acceptance Criteria
- [ ] AC1–AC6 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-01-wave0-memoria-de-agente-em-subpasta.md`
**Actions:**
1. Completude da enumeração: todos os sítios que geram prompt de agente com `memory:` (assets, golden files, renderização por destino em `internal/integrations/render.go`, o agente arquiteto), o mecanismo de regras de `.gitignore` (`generateGitIgnore`, `internal/generators/scaffold.go`; chamado por `init` e por `update` em `internal/generators/update.go`), o `doctor` (`internal/commands/doctor.go`), e o conjunto de orquestração do barrier (`buildMLScopeOrchestrationSet`, `internal/commands/barrier.go`, que só aceita `.claude/agent-memory/` na raiz).
2. Modelo de ameaça: instrução no prompt que contradiz o caminho injetado pelo harness; padrão de `.gitignore` que ignore a memória versionada da raiz (medir com `git check-ignore -v` num repositório em `t.TempDir()` ou no scratchpad, nunca neste repositório); `doctor` varrendo árvore grande, symlink para fora da raiz, `node_modules`.
3. Alvos de falsificação nos dois sentidos para cada AC.
4. Residual declarado.
**Acceptance criteria:**
- [ ] As quatro seções respondidas com evidência (comando e resultado)
- [ ] Prescrições em seção intitulada `Prescrições`
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-01-wave0-memoria-de-agente-em-subpasta.md
grep -q '^## Prescrições' docs/seguranca/2026-11-01-wave0-memoria-de-agente-em-subpasta.md
```

## Wave 1 — Implementação (preenchida após a auditoria da Wave 0)
> Dependencies: Wave 0 auditada.

## Wave 2 — Red team (hades-tf)
> Dependencies: Wave 1 auditada.

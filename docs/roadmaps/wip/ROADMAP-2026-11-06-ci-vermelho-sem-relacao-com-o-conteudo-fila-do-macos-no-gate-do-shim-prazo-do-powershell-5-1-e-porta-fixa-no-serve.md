---
status: wip
date: 2026-11-06
req: "docs/req/REQ-2026-11-06-ci-vermelho-sem-relacao-com-o-conteudo-fila-do-macos-no-gate-do-shim-prazo-do-powershell-5-1-e-porta-fixa-no-serve.md"
squad: ""
---

# Roadmap: CI vermelho sem relação com o conteúdo: fila do macOS no gate do shim, prazo do PowerShell 5.1 e porta fixa no serve

> Created: 2026-11-06 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-06-ci-vermelho-sem-relacao-com-o-conteudo-fila-do-macos-no-gate-do-shim-prazo-do-powershell-5-1-e-porta-fixa-no-serve.md

Três causas, um ML para cada. O diagnóstico está no ADR-2026-11-06. Varredura de issues: a única aberta é a #665, absorvida aqui; a #664 é Discussion.

## Acceptance Criteria
- [ ] AC1–AC5 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** 🔄 Em andamento
**Files affected:** `docs/seguranca/2026-11-06-wave0-ci-vermelho-por-infraestrutura.md`
**Acceptance criteria:**
- [ ] o conjunto fechado de caminhos inertes definido e justificado: nenhum deles entra no build, no `go:embed`, no shim npm, no wheel nem em script que algum gate execute
- [ ] os modos de evasão da classificação de diff analisados (rename, symlink, base forjada, `pull_request_target`, clone raso, evento diferente), cada um com a defesa correspondente
- [ ] o retry de porta e o prazo derivado analisados quanto a mascarar defeito real

## Wave 1 — Implementação (ML-1A e ML-1B em paralelo; ML-1C depois)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B tocam arquivos disjuntos. O ML-1C vem depois para respeitar o teto de 2 agentes simultâneos.

### ML-1A — Gate do shim com classificação de diff (ares-tf) — AC1, AC2
**Status:** ⬜ Pendente
**Files affected:** `.github/workflows/quality.yml`, `scripts/check-shim-diff-applies.sh`, `docs/cli-parity.md`
**Acceptance criteria:**
- [ ] o script classifica o diff do PR por lista fechada de caminhos inertes, com `git diff --name-only --no-renames` contra o merge-base e checkout `fetch-depth: 0`, e tem `--self-test` cobrindo os casos do AC2; o self-test roda no CI
- [ ] o gate passa com `não aplicável` só em `pull_request` com diff fora do conjunto, e exige `success` em todos os outros casos
- [ ] o comentário do ML-4E no `quality.yml` passa a explicar a nova regra

### ML-1B — Prazo do PowerShell derivado do aquecimento (artemis-tf) — AC3
**Status:** ⬜ Pendente
**Files affected:** `internal/generators/guard_line_shell_test.go`
**Acceptance criteria:**
- [ ] função de prazo com teste unitário dos três regimes (aquecimento curto → 15 s, médio → 4×, longo → 60 s)
- [ ] mensagem de estouro cita o aquecimento e o prazo
- [ ] o relatório conta as chamadas afirmadas do PowerShell, multiplica pelo teto de 60 s e compara com o `-timeout` do `go test` no `windows-full-suites`
- [ ] `go test ./internal/generators/` verde

### ML-1C — Porta livre no teste de endereço do serve (apolo-tf) — AC4
**Status:** ⬜ Pendente
**Files affected:** `scripts/check-serve-address-parity.sh`
**Acceptance criteria:**
- [ ] porta obtida do sistema; retry só em `address already in use`, até 3 vezes
- [ ] contra-braço: com a porta ocupada de propósito, o cenário passa; com o retry desligado, reprova
- [ ] `make parity-rest` verde

## Wave 2 — Red team
> Dependencies: Wave 1 completa.

### ML-2A — Red team da classificação de diff (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-06-redteam-ci-vermelho-por-infraestrutura.md`
**Acceptance criteria:**
- [ ] cada modo de evasão da Wave 0 tentado contra o script, com o resultado medido
- [ ] veredito escrito

---
status: wip
date: 2026-10-07
req: "docs/req/REQ-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: trackfw init nao instala os hooks de Gemini e Kiro na primeira execucao porque detecta os CLIs antes de criar seus arquivos

> Created: 2026-10-07 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md -->
REQ: docs/req/REQ-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md

Causa lida no código pelo arquiteto (2026-10-07): `internal/commands/init.go` chama `generators.Scaffold(cfg)` (~369),
que chama `InjectHooksDetected(cwd)` (`internal/generators/scaffold.go:198`), e **só depois** `installAITools(...)` (~386),
que cria os arquivos que a detecção procura. A detecção (`internal/generators/hooks.go:12`) olha `GEMINI.md`/`.gemini`
para o Gemini e só `.kiro` para o Kiro — que o `installAITools` não cria. Resultado reproduzido: 1ª execução sem hooks
de Gemini e Kiro; 2ª com Gemini; Kiro nunca. O mesmo `InjectHooksDetected` é chamado por `update` (`update.go:69`, `:2428`)
e `discover` (`commands/discover.go:161`).

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] Uma única execução de `trackfw init --ai-tools <lista>` instala o hook de guard de cada CLI pedido (AC2 da REQ)
- [ ] `update` e `discover` varridos: mesma causa corrigida aqui, com teste (AC3)
- [ ] Falsificação: voltar a ordem antiga reprova o teste (AC4)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** 🔄 Em andamento
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-07-wave0-init-instala-hooks-pedidos.md`
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
test -s docs/seguranca/2026-10-07-wave0-init-instala-hooks-pedidos.md
```

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: Wave 0 auditada.

### ML-1A — O `init` instala o hook de cada CLI pedido em `--ai-tools`, numa execução só
**Status:** ⬜ Pendente
**Squad:** apolo-tf
**Files affected:** `internal/commands/init.go`, `internal/generators/hooks.go`, `internal/generators/scaffold.go` (+ testes); `discover`/`update` só se a varredura achar a mesma causa
**Acceptance criteria:**
- [ ] Teste por CLI de `--ai-tools`, a partir de diretório vazio (`git init`, `HOME` isolado), uma execução: o arquivo de hook do CLI existe e contém a linha de guard da D11
- [ ] Varredura de `update` e `discover` com resultado escrito; mesma causa → corrigida aqui com teste
- [ ] Falsificação: voltar a ordem antiga (detectar antes de criar os arquivos) reprova o teste
- [ ] `go test ./internal/commands/ ./internal/generators/ -count=1` e `make quality` (arquiteto, sem `~/.local/bin` no PATH) verdes

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ ./internal/generators/ -count=1
```

---
status: wip
date: 2026-10-20
req: "docs/req/REQ-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md"
squad: "zeus-tf, backend, security, code-quality"
---

# Roadmap: Hooks de guard gerados falham sob zsh com exit 1 e deixam de executar a protecao

> Created: 2026-10-20 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md -->
REQ: docs/req/REQ-2026-10-20-hooks-de-guard-gerados-falham-sob-zsh-com-exit-1-e-deixam-de-executar-a-protecao.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] allow=0 e deny/ausente/erro=2 na matriz de shells suportados.
- [ ] Migração preserva customizações e os gates passam.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pendente
**Files affected:**
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
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none

## Evidência e retomada

Reprodução 2026-10-20: todas as nove entradas de guard instaladas falham zsh (exit 1) e passam Bash (exit 0). Semente isolada `=2 2>/dev/null; printf reached`: zsh aborta sem imprimir; Bash imprime reached. Workaround local envolve só strings de comandos de guard em Bash explícito, com backup.

Retomada em 2026-10-21: o #590 foi mergeado (81130826) e não bloqueia mais. Roadmap em wip, branch `fix/hooks-de-guard-gerados-falham-sob-zsh` criada a partir de origin/main (3c0cc1f2). Sem implementação de produto ainda.

Indício adicional (2026-10-21): com o workaround `/bin/bash -c` no `~/.codex/hooks.json` global, `TestValidateJSONFlag`/`TestValidateTextUnchanged` falham localmente também em origin/main, e passam com HOME isolado. Isso mostra que o validador (regra de forma esperada do hook global) e os testes leem o HOME real. A migração precisa levar em conta a forma do workaround.

### ML-1A — Definir e implementar geração compatível e migração
**Status:** ⬜ Pendente
**Arquivos afetados:** internal/generators/agentfiles.go, internal/generators/agentfiles_test.go, internal/generators/update_test.go; localizar consumidores de validação antes do handoff.
**Ações:** depois da Wave 0, decidir variante por shell/plataforma com base em prova real, migrar as três constantes de guard e todas as entradas legadas reconhecidas; preservar hooks desconhecidos. Não transportar o wrapper local macOS para Windows sem validação.
**Critérios de aceite:** matriz zsh/sh/bash allow/deny/ausente/erro e regressão Windows; ausência de duplicação; testes de preservação.
**Comandos de validação:** `go test ./internal/generators ./internal/validator ./internal/guard`; `go build ./...`; `make quality`.

## Wave 2 — Auditoria final

### ML-2A — Segurança, qualidade e contrato
**Status:** ⬜ Pendente
**Arquivos afetados:** docs/cli-parity.md, ADR-2026-10-04, CHANGELOG.md; relatório de revisão.
**Ações:** revisar que nenhum erro sai fail-open, nenhuma política foi removida e o artefato gerado foi executado nos shells declarados.
**Critérios de aceite:** revisão independente, gates verdes e prova reproduzível.
**Comandos de validação:** `make quality`; `trackfw validate`.

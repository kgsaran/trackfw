---
status: wip
date: 2026-07-18
req: "docs/req/REQ-2026-07-18-agents-skills-lifecycle-multi-cli.md"
squad: "Codex + especialistas Go/Node/Python"
---

# Roadmap: Catálogo unificado de agents e skills multi-CLI

> Created: 2026-07-18 | Status: 🔄 WIP

## Context

Unificar os instaladores de agentes e skills hoje fragmentados, remover as exceções
Go-only e entregar lifecycle seguro e compatível com os formatos nativos das CLIs.

REQ: docs/req/REQ-2026-07-18-agents-skills-lifecycle-multi-cli.md
ADR: docs/adr/ADR-2026-07-18-catalogo-canonico-e-adapters-para-integracoes-de-agentes.md
squad: Codex, backend Go, backend Node.js, backend Python, QA/paridade

## Acceptance Criteria

- [ ] Os quatro subcomandos de `agents` e `skills` existem nos três runtimes.
- [ ] Os nove adapters geram formatos nativos e reportam estado determinístico.
- [ ] Update e uninstall preservam customizações e paths fora do ownership.
- [ ] Tarball npm, wheel Python e binário Go carregam assets equivalentes.
- [ ] `make quality` e `trackfw validate --json` passam sem violações.

## Wave 1 — Catálogo, ownership e contratos (2 MLs em paralelo)
> Dependencies: none

### ML-1A — Catálogo canônico e assets por adapter
**Status:** ✅ Concluído
**Files affected:**
- `internal/integrations/assets/catalog.json`
- `internal/integrations/assets/agents/**`
- `internal/integrations/assets/skills/**`
- `internal/integrations/catalog.go`
- `internal/integrations/catalog_test.go`
**Actions:**
- Definir 10 agents, 5 skills e os adapters `claude`, `codex`, `gemini`,
  `antigravity`, `cursor`, `copilot`, `windsurf`, `amazonq`, `kiro`.
- Registrar capabilities, paths global/project, extensões, artefatos auxiliares e
  fallback quando a CLI não possuir subagente nativo.
- Embutir assets no binário Go e validar IDs/paths duplicados no carregamento.
**Acceptance criteria:**
- [ ] `go test ./internal/integrations/...` passa
- [ ] catálogo rejeita target/item/path duplicado
- [ ] os 9 targets e 15 itens canônicos estão presentes

**Validation commands:**
`go test ./internal/integrations/... && go build ./... && trackfw validate`

### ML-1B — Manifesto de ownership e lifecycle seguro
**Status:** ✅ Concluído
**Files affected:**
- `internal/integrations/manifest.go`
- `internal/integrations/manager.go`
- `internal/integrations/manager_test.go`
**Actions:**
- Implementar estados `not-installed`, `current`, `outdated`, `modified` por
  item/target/scope usando versão e SHA-256.
- Implementar install atômico, adoção segura de legado, update conservador,
  `--force` e uninstall restrito a ownership comprovado.
- Impedir path traversal e remoção fora dos roots declarados pelo adapter.
**Acceptance criteria:**
- [ ] arquivos modificados nunca são sobrescritos/removidos sem `--force`
- [ ] falha parcial não deixa manifesto inconsistente
- [ ] testes cobrem migração legada e path traversal

**Validation commands:**
`go test ./internal/integrations/... -run 'Manifest|Manager|Legacy|Traversal' && go vet ./...`

## Wave 2 — Comandos públicos nos três runtimes (3 MLs em paralelo)
> Dependencies: Wave 1 complete

### ML-2A — Go/Cobra: agents e skills lifecycle
**Status:** ✅ Concluído
**Files affected:**
- `internal/commands/agents.go`
- `internal/commands/skills.go`
- `internal/commands/integrations_flags.go`
- `internal/commands/agents_skills_test.go`
- `internal/commands/{gemini,cursor,copilot,windsurf,amazonq}.go`
**Actions:**
- Adicionar `list`, `install`, `uninstall`, `update`, seleção `huh` em TTY e
  flags `--targets`, `--items`, `--scope`, `--json`, `--force`.
- Manter comandos standalone existentes como aliases de compatibilidade.
**Acceptance criteria:**
- [ ] help, JSON, TTY/non-TTY e exit codes cobertos
- [ ] aliases antigos delegam ao mesmo manager

**Validation commands:**
`go test ./internal/commands/... ./internal/integrations/... && go build ./...`

### ML-2B — Node.js/Commander: paridade completa
**Status:** ✅ Concluído
**Files affected:**
- `npm/src/commands/agents.js`
- `npm/src/commands/skills.js`
- `npm/src/commands/index.js`
- `npm/src/integrations/**`
- `npm/tests/agents-skills.test.js`
**Actions:**
- Espelhar o contrato Go, incluindo prompts Inquirer, flags headless, manifesto,
  estados, aliases legados e JSON.
- Substituir o stub Claude e instaladores parciais usados por `init --ai-tools`.
**Acceptance criteria:**
- [ ] npm cria todos os adapters, não apenas rules genéricas
- [ ] saída JSON e exit codes equivalem ao Go

**Validation commands:**
`npm test && npm pack --dry-run --prefix npm`

### ML-2C — Python/argparse: paridade completa
**Status:** ✅ Concluído
**Files affected:**
- `pypi/trackfw/commands/agents.py`
- `pypi/trackfw/commands/skills.py`
- `pypi/trackfw/cli.py`
- `pypi/trackfw/integrations/**`
- `pypi/tests/test_agents_skills.py`
- `pypi/pyproject.toml`
**Actions:**
- Espelhar contrato, prompts TTY sem dependência obrigatória, flags headless,
  manifesto, adapters, aliases e JSON.
- Incluir assets no wheel/sdist e ampliar `init --ai-tools` para todos os targets.
**Acceptance criteria:**
- [ ] wheel instalado em ambiente limpo encontra todos os assets
- [ ] saída JSON e exit codes equivalem ao Go/Node

**Validation commands:**
`PYTHONPATH=pypi python3 -m pytest pypi/tests -q && python3 -m build pypi`

## Wave 3 — Sincronização, migração e paridade (2 MLs em sequência)
> Dependencies: Wave 2 complete

### ML-3A — Sync determinístico e package contract
**Status:** ✅ Concluído
**Files affected:**
- `scripts/sync-integration-assets.*`
- `scripts/check-integration-assets.*`
- `npm/src/integrations/assets/**`
- `pypi/trackfw/integrations/assets/**`
- `Makefile`
- `.github/workflows/quality.yml`
**Actions:**
- Gerar cópias npm/Python a partir do asset canônico Go.
- Falhar CI quando hashes ou catálogo divergirem.
- Executar smoke tests a partir do tarball npm e wheel, fora da source tree.
**Acceptance criteria:**
- [ ] alteração manual em cópia empacotada faz o gate falhar
- [ ] tarball e wheel contêm todos os assets

**Validation commands:**
`make quality && scripts/check-integration-assets.sh`

### ML-3B — Migração e aliases legados
**Status:** 🔄 Em andamento
**Files affected:**
- `internal/integrations/legacy.go`
- `npm/src/integrations/legacy.js`
- `pypi/trackfw/integrations/legacy.py`
- testes de migração nos três runtimes
**Actions:**
- Reconhecer instalações produzidas pelas versões anteriores sem manifest.
- Adotar apenas arquivos com conteúdo conhecido; classificar demais como
  `modified/unmanaged` e preservá-los.
- Garantir que `trackfw update` delegue ao novo motor sem ampliar escopo oculto.
**Acceptance criteria:**
- [ ] fixtures legadas de todos os instaladores atuais são cobertas
- [ ] nenhuma customização é adotada ou removida indevidamente

**Validation commands:**
`make quality`

## Wave 4 — Documentação e auditoria final (2 MLs em paralelo)
> Dependencies: Wave 3 complete

### ML-4A — Contrato de paridade e documentação de uso
**Status:** ⬜ Pendente
**Files affected:**
- `docs/cli-parity.md`
- `README.md`
- `site/guide/ai-agents.md`
- `site/en/guide/ai-agents.md`
- `site/guide/commands.md`
- `site/en/guide/commands.md`
**Actions:**
- Remover exceções Go-only, documentar matriz, scopes, status e migração.
- Corrigir qualquer referência a `trackfw install agents`.
**Acceptance criteria:**
- [ ] exemplos funcionam nos três pacotes
- [ ] português e inglês permanecem semanticamente equivalentes

**Validation commands:**
`make quality`

### ML-4B — QA de matriz e segurança destrutiva
**Status:** ⬜ Pendente
**Files affected:**
- `scripts/check-cli-parity.sh`
- suites Go/npm/Python de integração
**Actions:**
- Cobrir os quatro subcomandos, os 9 targets e scopes suportados.
- Testar symlinks, path traversal, arquivos modificados, uninstall parcial,
  instalação sem TTY e determinismo JSON.
**Acceptance criteria:**
- [ ] `make quality` passa integralmente
- [ ] `trackfw validate --json` retorna zero violations
- [ ] working tree contém somente arquivos previstos no roadmap

**Validation commands:**
`make quality && trackfw validate --json && git diff --check`

---
status: wip
date: 2026-11-06
req: "docs/req/REQ-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md"
squad: ""
---

# Roadmap: guard de credencial atribui ao payload o achado que veio de arquivo referenciado e trata glob e leitura recursiva de forma diferente

> Created: 2026-11-06 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md -->
Defeito medido e decisão: ver REQ e ADR-2026-11-06 (guard de credencial). Varredura de issues abertas: nenhuma com o mesmo mecanismo.

REQ: docs/req/REQ-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md

## Acceptance Criteria
- [ ] AC1–AC4 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** 🔄 Em andamento
**Files affected:** `docs/seguranca/2026-11-06-wave0-guard-credencial-origem-do-achado.md`
**Acceptance criteria:**
- [ ] riscos de imprimir o caminho no stderr e no sinal de atenção analisados (caminho forjado, caractere de controle, comando de workflow, vazamento de caminho sensível)
- [ ] a limitação de diretório (D2) analisada quanto ao que ela deixa passar, e se o texto da documentação ensina o contorno
- [ ] a montagem de token em tempo de execução nas fixtures analisada quanto a enfraquecer o que os testes afirmam

## Wave 1 — Implementação (ML-1A e ML-1B em paralelo)
> Dependencies: Wave 0 auditada. Arquivos disjuntos.

### ML-1A — Mensagem com a origem do achado e documentação da limitação (apolo-tf) — AC1, AC2
**Status:** ⬜ Pendente
**Files affected:** `internal/guard/credential.go`, `internal/guard/credential_test.go`, `internal/commands/guard.go`, `docs/cli-parity.md`
**Acceptance criteria:**
- [ ] achado da segunda camada sai como `possible <tipo> detected in file <caminho> referenced by the command`, com o caminho sanitizado; achado no payload continua `in tool payload`; os dois escopos, os dois modos e o sinal de atenção
- [ ] teste fixa as duas formas, e um caso com caractere de controle no nome do arquivo
- [ ] `--help` e `docs/cli-parity.md` documentam que diretório não é lido recursivamente, com o exemplo medido
- [ ] `go test ./internal/guard/ ./internal/commands/` verde

### ML-1B — Fixtures montam o token em tempo de execução (artemis-tf) — AC3
**Status:** ⬜ Pendente
**Files affected:** `internal/guard/credential_patterns_test.go`, `internal/generators/credential_guard_test.go`
**Acceptance criteria:**
- [ ] nenhum dos dois arquivos contém literal que case com os padrões do guard
- [ ] os testes continuam afirmando a detecção sobre o valor montado; contra-braço mostra que a detecção ainda reprova sem o padrão
- [ ] `go test ./internal/guard/ ./internal/generators/` verde

## Wave 2 — Red team
> Dependencies: Wave 1 completa.

### ML-2A — Red team (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-06-redteam-guard-credencial-origem-do-achado.md`
**Acceptance criteria:**
- [ ] cada risco da Wave 0 atacado com medição
- [ ] veredito escrito

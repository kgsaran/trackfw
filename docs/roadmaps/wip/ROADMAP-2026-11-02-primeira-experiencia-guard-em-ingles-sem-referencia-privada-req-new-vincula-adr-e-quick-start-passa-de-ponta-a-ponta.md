---
status: wip
date: 2026-11-02
req: "docs/req/REQ-2026-11-02-primeira-experiencia-guard-em-ingles-sem-referencia-privada-req-new-vincula-adr-e-quick-start-passa-de-ponta-a-ponta.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: Primeira experiência: guard em inglês sem referência privada, req new vincula ADR e Quick start passa de ponta a ponta

> Created: 2026-11-02 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-02-primeira-experiencia-guard-em-ingles-sem-referencia-privada-req-new-vincula-adr-e-quick-start-passa-de-ponta-a-ponta.md

Os defeitos estão medidos na REQ. Varredura de issues abertos em 2026-11-02: nenhum aberto.

## Acceptance Criteria
- [ ] AC1–AC6 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-02-wave0-primeira-experiencia.md`
**Actions:**
1. Completude da enumeração:
   - todas as mensagens emitidas pelo binário em português ou com `§`, por grep de mecanismo e não só pelo literal `bruto bloqueado`, em `internal/guard/`, `internal/commands/`, `internal/validator/` e `internal/generators/`;
   - os testes, goldens, `testdata/` e scripts que afirmam o texto atual, para que a troca não quebre um gate de identidade de bytes. Cite `internal/generators/testdata/guard-sh-reference/` e o shim byte-identity.
2. Modelo de ameaça:
   - o `--adr` aceita caminho com `../`, symlink ou nome que casa com mais de um ADR?
   - a escrita do vínculo pode corromper um frontmatter existente?
   - a tradução pode tirar da mensagem do guard uma orientação que o agente usa para se corrigir?
3. Alvos de falsificação nos dois sentidos para o AC1 a AC4.
4. Residual declarado.
**Acceptance criteria:**
- [ ] As quatro seções respondidas com evidência (comando e resultado)
- [ ] Prescrições em seção intitulada `Prescrições`
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-02-wave0-primeira-experiencia.md
grep -q '^## Prescrições' docs/seguranca/2026-11-02-wave0-primeira-experiencia.md
```

## Wave 1 — Implementação (2 executores em paralelo, arquivos disjuntos)
> Dependencies: Wave 0 auditada. O ML-1A fica em `internal/guard/` e nos testes e fixtures do texto do guard; o ML-1B fica em `internal/commands/req*.go` e em `docs/cli-parity.md`.

### ML-1A — Mensagens do guard em inglês, sem referência privada (apolo-tf) — AC1, AC2
**Status:** ⬜ Pendente
**Files affected:** definidos pela Wave 0; o núcleo é `internal/guard/gitbranch.go`.
**Actions:**
1. Traduzir as constantes `reason*` e todas as demais mensagens emitidas listadas pela Wave 0, mantendo a orientação.
2. Remover `Ver CLAUDE.md §1`.
3. Teste que varre as mensagens e reprova com `§` ou texto em português (AC2), com contra-braço.
**Acceptance criteria:**
- [ ] `go test ./internal/guard/ -count=1` e os pacotes dos fixtures tocados verdes
- [ ] contra-braço do AC2 medido, restaurado e conferido com `cmp`

### ML-1B — `req new --adr` e Quick start de ponta a ponta (apolo-tf) — AC3, AC4
**Status:** ⬜ Pendente
**Files affected:** `internal/commands/req.go`, `internal/commands/req_test.go`, um teste novo `internal/commands/quickstart_e2e_test.go`, `docs/cli-parity.md`
**Actions:**
1. Flag `--adr`, resolvido por `ResolveADRFiles`. Grava o `adr:` do frontmatter e a linha `ADR:`; ADR inexistente ou ambíguo dá erro sem criar arquivo.
2. Teste de ponta a ponta do Quick start em `t.TempDir()`, com `validate` terminando em 0 violações.
**Acceptance criteria:**
- [ ] `go test ./internal/commands/ -count=1` verde
- [ ] contra-braço: sem o vínculo, o teste de ponta a ponta reprova

**Gates da wave:**
```bash
go build ./...
go vet ./internal/guard/ ./internal/commands/
go test ./internal/guard/ -count=1
go test ./internal/commands/ -count=1
```

## Wave 2 — README e GIF (arquiteto)
> Dependencies: Wave 1 auditada. O GIF precisa do binário com o AC1 e o AC3.

### ML-2A — Primeira tela do README, Quick start e `demo.tape` (trackfw_architect) — AC5, AC6
**Status:** ⬜ Pendente
**Files affected:** `README.md`, `demo.tape`, `docs/demo.gif`
**Actions:**
1. Reescrever a primeira tela e o Quick start do README; o Quick start traz a mesma sequência do teste do ML-1B.
2. Reescrever o `demo.tape` e gerar o `docs/demo.gif` com `vhs` e o binário da branch.
3. Auditoria visual em quadros extraídos com `ffmpeg`.
**Acceptance criteria:**
- [ ] quadros conferidos: guard barrando `git push` com a mensagem em inglês; `validate` sem violação; nenhum `Usage:` e nenhum caminho absoluto
- [ ] Quick start do README idêntico à sequência do teste

**Gates da wave:**
```bash
test -f docs/demo.gif
grep -q 'req new' README.md
```

## Wave 3 — Red team (hades-tf)
> Dependencies: Wave 2 concluída.

### ML-3A — Red team (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-02-redteam-primeira-experiencia.md`
**Actions:**
1. Reimplementar, a partir da leitura, os alvos de falsificação da Wave 0 contra o diff.
2. Seguir o README novo num repositório limpo, como usuário novo.
**Acceptance criteria:**
- [ ] Parecer com achados classificados e evidência

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-02-redteam-primeira-experiencia.md
```

---
status: wip
date: 2026-10-23
req: "docs/req/REQ-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-working-context-arquivos-congelados-e-escopo-por-despacho.md"
squad: "hades-tf, prometeu-tf, apolo-tf, hefesto-tf"
---

# Roadmap: as regras de escrita dos agentes viram mecanismo no harness

> Created: 2026-10-23 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-23-regras-de-escrita-dos-agentes-viram-mecanismo-no-harness-working-context-arquivos-congelados-e-escopo-por-despacho.md
ADR: docs/adr/ADR-2026-10-23-regras-de-escrita-dos-agentes-sao-aplicadas-pelo-harness-e-nao-pelo-texto-do-handoff.md

Ordem decidida pelo KG: D1 (a instrução) → D2 (o barrier) → D3 (os congelados) → D4 (o escopo por despacho).

## Acceptance Criteria
- [ ] AC1–AC6 da REQ, cada um com a evidência no ML que o fecha
- [ ] `make quality` e o CI verdes

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e decisões abertas
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model das quatro camadas
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-23-wave0-regras-de-escrita.md`
**Actions:**
1. **Superfícies, com a enumeração fechada por grep e cruzando todas as dimensões** (lição da REQ-2026-10-23, em que a matriz
   falhou três vezes por olhar só a dimensão do defeito original):
   - os 12 assets de agente e o gerador (`internal/integrations/assets/agents/`, `internal/generators/agentfiles.go`, o
     render em `internal/integrations/`);
   - as checagens do `trackfw barrier` (`internal/commands/barrier.go`);
   - os subcomandos de `trackfw guard` (`internal/commands/guard.go`, `internal/guard/`);
   - a fiação do guard por CLI no `init`, no `update` e no `validate`, e quais matchers de escrita cada CLI tem.
2. **Identidade do agente:** confirme na documentação oficial do Claude Code os campos `agent_id` e `agent_type` no payload
   de `PreToolUse` e os hooks no frontmatter de subagente (<https://code.claude.com/docs/en/hooks>). Diga quais outros CLIs
   entregam identidade e cite a fonte. Defina o que acontece quando o campo falta.
3. **D2:** defina a base do diff da wave, a gramática de `Files affected` (medida sobre os roadmaps reais em
   `docs/roadmaps/`: quantos declaram, em que formato) e a regra para roadmap antigo sem declaração, com um corte por data
   explícito ou outra regra que nunca passe em silêncio.
4. **D3/D4, ameaças:**
   - escrita por Bash, e se o parser de shell do credential guard cobre `>`/`>>`/`tee`/`sed -i`;
   - caminho relativo, `..`, symlink, `/tmp` versus `/private/tmp`, Windows;
   - o agente editando a própria permissão;
   - payload sem `file_path`;
   - `NotebookEdit`;
   - o MCP de escrita;
   - fail-open por erro de parse.
5. **Alvos de falsificação nos dois sentidos** por camada, e o **resíduo declarado**.
**Acceptance criteria:**
- [x] Cada seção respondida com evidência (comando e saída, ou citação com URL)
- [x] A gramática de `Files affected` medida sobre o corpus real, com contagens
- [x] Nenhuma linha de produto
**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-23-wave0-regras-de-escrita.md
git diff --quiet origin/main -- internal/
```

## Wave 1 — D1, a instrução na origem
> Dependencies: Wave 0 auditada.

### ML-1A — Seção Working context dos 11 papéis
**Status:** ✅ Concluído
**Agente:** prometeu-tf
**Files affected:** `internal/integrations/assets/agents/*.md` (os 11 que não são `architect.md`), os goldens desses assets e o teste novo da forma por papel
**Acceptance criteria:**
- [ ] AC1 da REQ
- [ ] `go test ./internal/integrations/ ./internal/generators/ -count=1` verde

**Auditoria do arquiteto (2026-10-23):** aprovado. Os 11 assets ficaram com o texto idêntico, e o `architect.md` não foi
tocado. `TestWorkingContextFormByRole` passa na branch e reprova no HEAD anterior. Os gates estão verdes. O Prometeu não
escreveu no working context e reportou tudo no relatório, como a D1 pede. Ele também apontou a mesma causa num segundo
sítio, que entra como ML-1B.

### ML-1B — O bloco de regras do CLAUDE.md, AGENTS.md e GEMINI.md
**Status:** ⬜ Pendente
**Agente:** prometeu-tf
**Por que existe:** o `trackfwRulesBlock` (`internal/generators/agentfiles.go:94`) manda **toda** sessão, inclusive os
subagentes, "write a new file in `docs/agents-working-context/…`". Isso contradiz a D1. O texto é lido por todo agente.
**Files affected:** `internal/generators/agentfiles.go`, os goldens e testes que fixam o bloco, e os blocos gerados
deste repositório em `CLAUDE.md`, `AGENTS.md` e `GEMINI.md` (a mesma linha, editada igual).
**Acceptance criteria:**
- [ ] A linha 3 do bloco passa a dizer que quem registra é a sessão que orquestra (o arquiteto), e que os subagentes
  reportam no relatório final
- [ ] Os três arquivos deste repositório ficam com o mesmo texto do gerador; um teste ou gate existente confirma a concordância
- [ ] `go test ./internal/generators/ ./internal/integrations/ -count=1` verde

## Wave 2 — D2, o barrier
> Dependencies: Wave 1 auditada. Detalhada depois da Wave 0.

### ML-2A — Checagem `ml_scope`
**Status:** ⬜ Pendente
**Agente:** apolo-tf
**Files affected:** `internal/commands/barrier.go`, o teste novo e `docs/cli-parity.md` (o contrato do barrier)
**Acceptance criteria:**
- [ ] AC2 da REQ

## Wave 3 — D3 e D4, o guard write-scope
> Dependencies: Wave 2 auditada. Detalhada depois da Wave 0.

### ML-3A — `trackfw guard write-scope` com os caminhos congelados (D3)
**Status:** ⬜ Pendente
**Agente:** apolo-tf
**Files affected:** `internal/commands/guard.go`, `internal/guard/writescope.go`, `internal/guard/writescope_test.go`, `internal/generators/agentfiles.go`, `docs/cli-parity.md`, `trackfw.yaml`
**Acceptance criteria:**
- [ ] AC3 e AC5 da REQ

### ML-3B — A permissão por despacho (D4)
**Status:** ⬜ Pendente
**Agente:** apolo-tf
**Files affected:** `internal/guard/writescope.go`, `internal/guard/writescope_test.go`, `docs/cli-parity.md`
**Acceptance criteria:**
- [ ] AC4 e AC5 da REQ

## Wave 4 — Revisões independentes
> Dependencies: Wave 3 auditada.

### ML-4A — Revisão de qualidade
**Status:** ⬜ Pendente
**Agente:** hefesto-tf
**Files affected:** `docs/seguranca/2026-10-23-quality-write-scope.md`
**Acceptance criteria:**
- [ ] Parecer sem bloqueio, ou corretivo

### ML-4B — Red-team do guard write-scope
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-23-red-team-write-scope.md`
**Acceptance criteria:**
- [ ] Reimplementação independente das regras a partir da ADR; nenhum caminho de fail-open

## Wave 5 — Fechamento
> Dependencies: Wave 4 aprovada.

### ML-5A — make quality e CI
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Files affected:** artefatos de orquestração (roadmap, REQ — sem novo arquivo de produto)
**Acceptance criteria:**
- [ ] `make quality` rc=0, com a máquina ociosa e o HOME isolado
- [ ] CI do PR verde

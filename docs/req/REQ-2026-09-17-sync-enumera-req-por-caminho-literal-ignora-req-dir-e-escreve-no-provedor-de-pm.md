---
status: Open
date: 2026-09-17
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md"
---

# REQ: sync enumera REQ por caminho literal ignora req_dir e escreve no provedor de PM

> Date: 2026-09-17 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Issue **#268**, AC3 — reportado por consumidor externo e revalidado contra a `main` da v8.0.0 GA.

`internal/sync/sync.go:43` enumera REQ com caminho **literal**:

```go
filepath.Glob("docs/req/*.md")
```

dentro de `syncToProvider`, alcançado por `SyncToLinear` e `SyncToJira`
(`internal/commands/sync.go:37,39`). Ele **ignora o `req_dir` configurado** e ignora
`roadmap_namespacing: by_agent`. O ponto único existe e é usado em outros lugares —
`internal/validator/validator.go:963` chama `resolveREQFiles(cfg)`.

### 🔴 Por que este é o sítio mais grave dos que o issue enumerou

Os demais sítios do #268 **contavam** errado. Este **escreve**, e fora do nosso raio.

Num consumidor com `req_dir: docs/requisições`, o `sync` enxerga **0 REQ real** e passa a operar
sobre arquivos residuais em `docs/req/` — podendo **criar issue no Linear/Jira** a partir deles e
**injetar o id de volta** no arquivo. Um contador errado desinforma; um escritor errado **age**, em
sistema de terceiro, sem desfazer.

A metade do issue que tratava do `status` do Python desapareceu com a v8 (o runtime foi removido e o
contador do Go sempre esteve correto). Restou **um alvo único** — o que torna a correção barata, não
menos necessária.

### Varredura já feita

`REQDir|RoadmapDir` × `Glob|ReadDir|Walk`, fora de testes: nenhum outro enumerador flat de REQ.
`validator.go:316` é o ramo flat **legítimo** de um condicional `by_agent`, não defeito.

## Acceptance Criteria

- [ ] **AC1** — `syncToProvider` passa a resolver as REQ pelo ponto único (`resolveREQFiles`),
      honrando `req_dir` e `by_agent`.
- [ ] **AC2** — 🔴 Falsificação com `req_dir` **não-padrão**: num projeto com
      `req_dir: docs/requisições`, o `sync` enxerga as REQ reais. E o contra-braço: a versão **sem** a
      correção enxerga 0 — provando que o cenário discrimina.
- [ ] **AC3** — 🔴 Falsificação com `by_agent`: REQ sob `<req_dir>/<agente>/` são enumeradas.
- [ ] **AC4** — 🔴 **Nenhuma escrita em provedor externo durante os testes.** As falsificações usam
      dry-run ou provedor de mentira. Um teste que cria issue de verdade para provar que o sync
      funciona seria o próprio defeito, encenado.
- [ ] **AC5** — Gate que impeça a reintrodução: nenhum caminho de REQ ou roadmap literal em
      `internal/**` fora do resolvedor. Se a forma literal for legítima em algum sítio, ele é
      declarado com motivo.
- [ ] **AC6** — Comportamento definido e escrito para o caso em que a resolução devolve **0 REQ**: o
      `sync` deve **dizer que não achou nada** em vez de seguir em silêncio. Hoje o silêncio é o que
      transforma má configuração em escrita errada.

## Negative Scope

- ❌ **Não** alterar o contrato do `sync` com Linear/Jira (campos, formato do id) — a REQ é sobre
  **quais** arquivos ele enxerga.
- ❌ **Não** tratar os demais itens do #268 que morreram com a v8 — estão fechados por remoção.
- ❌ **Não** implementar migração de `docs/req/` residual: fora de escopo, e é decisão do consumidor.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 

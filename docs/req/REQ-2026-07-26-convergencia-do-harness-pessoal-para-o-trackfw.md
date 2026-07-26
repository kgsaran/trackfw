---
status: Open
date: 2026-07-26
author: "KG"
adr: "docs/adr/ADR-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw.md"
---

# REQ: convergência do harness pessoal para o trackfw

> Date: 2026-07-26 | Status: Open

## Motivation

O trackfw entrega hoje 10 agentes de ~360 bytes e 5 skills de processo, todos sem regras de kanban,
git ou coordenação. A validação `branch_has_wip_roadmap` impõe uma cadeia (ADR→REQ→ROADMAP) que os
próprios agentes entregues **não ensinam** — o produto valida algo que não explica.

Em paralelo, o harness que funciona na prática vive fora do produto: 14 agentes do Panteão Grego,
`~/.claude/CLAUDE.md` (284 linhas) e 21 skills pessoais (2.518 linhas). O objetivo é convergir tudo
para o trackfw, permitindo aposentar os artefatos pessoais.

Diagnóstico completo, evidências e trade-offs em
`docs/analises/2026-07-26-plano-convergencia-harness-pessoal-para-trackfw.md`.
Decisões D1–D17 no ADR vinculado.

## Escopo

1. **Camada universal de harness** nos assets de agente: LOCK DE MODO, registro de contexto,
   `memory: project`, `tools:` explícito, análise estática antes de editar, restrição de escopo e
   handoff, assinatura.
2. **Adendo orquestrador** no `architect`: permissões git exclusivas, paralelização (waves, barrier,
   regras de spawn), workflow de 10 passos, auditoria de conformidade pós-ML.
3. **Adendo implementador** nos demais: KANBAN como pré-requisito, proibição de criar branch/PR,
   protocolo de conclusão de ML.
4. **CLAUDE.md gerado** enriquecido: branch strategy com detecção de squash-merge, Definition of Done
   "sem rabo", escopo negativo obrigatório na REQ, exigências por estado, prototipagem iterativa,
   protocolos de bug de produção e de bug concreto, autopilot, formato de roadmap Waves+MLs.
5. **Estado `analyzing`** admitido pelo validator nos 3 CLIs.
6. **Vault de conhecimento**: scaffold, regra na camada universal, comando `trackfw note new`,
   regra `note_orphan` (severidade `warning`).
7. **Papel canônico `iac`** com fronteira declarada em relação a `infra`.
8. **Papel canônico de tooling de agentes**.
9. **10 skills técnicas por papel**, agnósticas de stack.
10. **Correção dos 11 defeitos** catalogados na Parte 4 do documento de análise.

## Escopo negativo (o que NÃO fazer)

- **Não** criar capacidade de perfil opt-in nem corpo variável por perfil no catálogo (D1).
- **Não** traduzir assets para PT-BR nem criar mecanismo de i18n de assets (D2).
- **Não** alterar o validator para `roadmap_namespacing` — flat já é o default e `by_agent` já existe (D4).
- **Não** portar Cronos (ITIL/CMDB) nem Hermes (NetSuite) como papéis canônicos (D10).
- **Não** portar conteúdo específico de stack para as skills: ArangoDB, Uber Fx, Gin, Entra ID,
  Module Federation e afins vão para o CLAUDE.md do projeto, não para o produto (D12).
- **Não** apensar skills técnicas às 5 skills de processo existentes (D11).
- **Não** remover `legacyHashes` de `internal/integrations/legacy.go` ao aposentar o gerador legado.
- **Não** implementar `trackfw ship` nesta REQ — sai em REQ separada (D13).
- **Não** migrar as pastas do projeto CMDB — trabalho fora deste repositório.

## Acceptance Criteria

- [ ] Os 10 assets de agente contêm a camada universal e passam `make quality` sem erro
- [ ] `architect` contém o adendo orquestrador; os demais contêm o adendo implementador
- [ ] `scripts/check-integration-assets.sh` verde: bytes idênticos entre Go, npm e PyPI
- [ ] `trackfw validate` aceita roadmap em `analyzing` nos 3 CLIs, com testes cobrindo o novo estado
- [ ] `trackfw init` cria `vault/notes/index.md`; `trackfw note new "<t>"` gera nota e linka no índice
- [ ] `note_orphan` reportado como **warning** por default e configurável via `rules:`
- [ ] `KnownAgentIDs` inclui `iac` e o papel de tooling; `TestPreset_EveryPresetCoversExactlyKnownAgentIDs` verde nos 10 presets
- [ ] 10 skills técnicas presentes em `assets/skills/`, sem menção a stack específica
- [ ] CLAUDE.md gerado contém branch strategy, DoD, escopo negativo, exigências por estado e formato Waves+MLs
- [ ] Assinatura renderizada em EN, e com DisplayName quando há preset configurado
- [ ] `make quality` verde nos 3 CLIs
- [ ] `go build ./...`, `go test ./...` e `go vet ./...` sem erro

## Linked ADR

ADR: docs/adr/ADR-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/wip/ROADMAP-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw.md

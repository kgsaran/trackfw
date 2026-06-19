---
id: REQ-2026-06-19-analyzing-state-ml-status-rules
title: Estado "Analyzing" no kanban e regras de marcação de ML
status: approved
priority: high
type: feature
created: 2026-06-19
author: zeus
---

# REQ: Estado "Analyzing" no kanban e regras de marcação de ML

## Problema

1. **Ausência de regra de marcação de ML**: Os agentes não têm instrução explícita para marcar o status do ML como `🔄 Em andamento` ao iniciar e `✅ Concluído` ao terminar. O campo `active_ml` do board kanban fica sempre vazio durante execuções reais.

2. **Ausência do estado "Analyzing"**: Antes de mover um roadmap para `wip`, os agentes analisam e validam o roadmap (leitura, verificação de pré-requisitos, entendimento de contexto). Esse estado de análise é invisível no board — o roadmap aparece em `backlog` até que seja movido diretamente para `wip`.

## Requisitos

### R1 — Regra de marcação de status de ML (injetada via trackfw)
- O bloco de regras injetado por `init`, `discover --init` e `update` deve incluir:
  - Ao **iniciar** um ML: alterar `**Status:** ⬜ Pendente` → `**Status:** 🔄 Em andamento` no arquivo do roadmap
  - Ao **concluir** um ML: alterar `**Status:** 🔄 Em andamento` → `**Status:** ✅ Concluído` no arquivo do roadmap
  - O agente deve fazer commit do roadmap com a mudança de status junto com o commit do ML concluído
- Regra aplicável a todos os agentes que seguem o framework trackfw

### R2 — Novo estado "Analyzing" no kanban
- Ordem das colunas: `backlog → analyzing → wip → blocked → done → abandoned`
- O estado `analyzing` representa: "agente está lendo, validando e planejando execução do roadmap"
- `init` e `discover --init` devem criar o diretório `analyzing/` na estrutura de roadmaps
- Regra injetada: antes de mover roadmap para `wip`, mover para `analyzing/` e validar contexto
- Board kanban deve exibir a coluna "Analyzing" entre "Backlog" e "WIP"
- Paridade obrigatória: Go CLI, Node.js CLI, Python CLI para criação do diretório

## Critérios de Aceite

- [ ] Bloco de regras injetado contém instruções de marcação `🔄`/`✅` por ML
- [ ] `trackfw init` cria diretório `analyzing/` na estrutura de roadmaps
- [ ] `trackfw discover --init` cria diretório `analyzing/` na estrutura de roadmaps
- [ ] `trackfw update` reinjecta regras atualizadas (incluindo marcação de ML e uso de `analyzing/`)
- [ ] Board kanban exibe coluna "Analyzing" entre "Backlog" e "WIP"
- [ ] `boardStates` inclui `analyzing` na posição correta
- [ ] Frontend renderiza cards na coluna "Analyzing" com styling adequado
- [ ] Testes unitários cobrem o novo estado
- [ ] 3 CLIs (Go, Node.js, Python) criam `analyzing/` no init/discover

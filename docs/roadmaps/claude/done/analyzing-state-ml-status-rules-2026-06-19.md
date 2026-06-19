---
id: analyzing-state-ml-status-rules-2026-06-19
req: REQ-2026-06-19-analyzing-state-ml-status-rules
status: wip
created: 2026-06-19
---

# Roadmap: Estado "Analyzing" + Regras de marcação de ML

> Criado em: 2026-06-19 | Status: 🔄 WIP
> REQ: REQ-2026-06-19-analyzing-state-ml-status-rules

## Diagnóstico / Contexto

- Agentes não marcam ML como `🔄` ao iniciar nem `✅` ao concluir → `active_ml` sempre vazio no board
- Não existe estado "Analyzing" no kanban → validação pré-wip é invisível
- Regras injetadas pelo trackfw (`init`/`discover --init`/`update`) não cobrem ciclo de vida de ML
- Novo estado exige: diretório na estrutura, entrada em `boardStates`, coluna no frontend, regras nos 3 CLIs

---

## Wave 1 — Regras de agente (injeção nos 3 CLIs) (3 MLs em paralelo)
> Independente de Wave 2

### ML-1A — Atualizar template de regras Go + conteúdo injetado
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/rules.go` (ou arquivo equivalente com o conteúdo do bloco de regras)
**Ações:**
1. Localizar onde o conteúdo entre `<!-- trackfw:rules:start -->` e `<!-- trackfw:rules:end -->` é definido
2. Adicionar seção "Ciclo de vida de ML" com as regras:
   - Ao iniciar ML: `**Status:** ⬜ Pendente` → `**Status:** 🔄 Em andamento` + commit do roadmap
   - Ao concluir ML: `**Status:** 🔄 Em andamento` → `**Status:** ✅ Concluído` + commit junto com o ML
   - Ao mover roadmap para análise: mover arquivo para `analyzing/` antes de wip
   - Ao iniciar implementação: mover arquivo de `analyzing/` para `wip/`
3. Build e teste: `go build ./...` + `go test ./...`
**Critérios de aceite:**
- [ ] Bloco injetado contém as 4 regras de ciclo de vida
- [ ] `go build ./...` sem erros

### ML-1B — Atualizar template de regras Node.js
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `npm/src/generators/init.js` (função que gera o bloco de regras)
**Ações:**
1. Localizar a string/template do bloco de regras no gerador Node.js
2. Adicionar as mesmas 4 regras de ciclo de vida (idêntico ao ML-1A em conteúdo)
3. Verificar que `injectRulesDetected` e `generateClaudeCommandsForce` usam o conteúdo atualizado
**Critérios de aceite:**
- [ ] Bloco de regras Node.js contém as 4 regras de ciclo de vida
- [ ] `node npm/src/cli.js --help` executa sem erros

### ML-1C — Atualizar template de regras Python
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `pypi/trackfw/generators/init_gen.py` (função equivalente de injeção de regras)
**Ações:**
1. Localizar a string do bloco de regras no gerador Python
2. Adicionar as mesmas 4 regras de ciclo de vida
**Critérios de aceite:**
- [ ] Bloco de regras Python contém as 4 regras
- [ ] `python -m trackfw --help` executa sem erros

---

## Wave 2 — Estado "Analyzing" no board serve (Go) (2 MLs em paralelo)
> Independente de Wave 1

### ML-2A — boardStates + API: adicionar "analyzing"
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/serve/api_board.go`
- `internal/serve/api_board_test.go`
**Ações:**
1. Alterar `boardStates` de `[]string{"wip", "backlog", "blocked", "done", "abandoned"}` para:
   `[]string{"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}`
2. O novo estado `analyzing` é tratado automaticamente pelo `readStateDir` existente — sem lógica especial
3. Adicionar ao `TestBoardHandler_FlatMode`: criar `analyzing/` com um arquivo `.md` e verificar que aparece na resposta JSON
**Critérios de aceite:**
- [ ] API retorna coluna `analyzing` no JSON
- [ ] `go test ./internal/serve/...` verde

### ML-2B — Frontend: coluna "Analyzing" + badge CSS
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/serve/static/app.js`
- `internal/serve/static/index.html` (se houver CSS inline ou variáveis de cor)
**Ações:**
1. Verificar a função `stateLabel(state)` — adicionar case `"analyzing"` → `"Analisando"`
2. Verificar a função `renderBoard` — garantir que `analyzing` aparece entre `backlog` e `wip` na ordem de colunas
3. Adicionar classe CSS `.badge-analyzing` com cor amarela/âmbar (similar a `badge-blocked` mas tom diferente):
   ```css
   .badge-analyzing { background: #fef3c7; color: #92400e; }
   ```
4. Verificar se o header da coluna precisa de cor de fundo — adicionar se necessário
**Critérios de aceite:**
- [ ] Coluna "Analisando" aparece entre "Backlog" e "WIP" no board
- [ ] Badge exibe cor âmbar/amarelo distinto de "Bloqueado"

---

## Wave 3 — Criação de diretório `analyzing/` nos CLIs (3 MLs em paralelo)
> Independente das Waves 1 e 2

### ML-3A — Go: criar `analyzing/` em init + discover
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/scaffold.go` (ou arquivo que cria os diretórios de roadmap)
**Ações:**
1. Localizar onde os diretórios `wip/`, `backlog/`, `done/`, etc. são criados
2. Adicionar `analyzing/` na lista, na posição entre `backlog` e `wip`
3. Garantir que funciona tanto para layout flat quanto by_agent
4. Build e teste: `go build ./...`
**Critérios de aceite:**
- [ ] `trackfw init` cria `analyzing/` (flat e by_agent)
- [ ] `trackfw discover --init` cria `analyzing/`
- [ ] `go build ./...` sem erros

### ML-3B — Node.js: criar `analyzing/` em init + discover
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `npm/src/generators/init.js` (função que cria diretórios de roadmap)
- `npm/src/commands/discover.js` (se tiver criação de diretórios separada)
**Ações:**
1. Localizar onde os diretórios de roadmap são criados no gerador Node.js
2. Adicionar `analyzing` na lista de estados/diretórios
**Critérios de aceite:**
- [ ] `npx trackfw init` (ou equivalente) cria `analyzing/`

### ML-3C — Python: criar `analyzing/` em init + discover
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `pypi/trackfw/generators/init_gen.py`
- `pypi/trackfw/commands/discover.py`
**Ações:**
1. Localizar onde os diretórios de roadmap são criados
2. Adicionar `analyzing` na lista
**Critérios de aceite:**
- [ ] CLI Python cria `analyzing/` no init/discover

---

## Wave 4 — Integração final + commit do roadmap
> Depende de Waves 1, 2, 3 completas

### ML-4A — Commit, push e abertura de PR
**Status:** ⬜ Pendente
**Ações:**
1. `go build ./...` + `go test ./...` — verde
2. Commitar todos os arquivos modificados na branch `feat/analyzing-state-ml-status-rules`
3. Zeus abre PR para main
**Critérios de aceite:**
- [ ] Build verde
- [ ] Testes verdes
- [ ] PR aberto com descrição consolidando Waves 1–3

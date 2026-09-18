---
status: done
date: 2026-08-15
req: "docs/req/REQ-2026-08-15-agentes-especialistas-aceitam-contexto-de-convencoes-especifico-do-projeto.md"
squad: ""
---

# Roadmap: agentes especialistas aceitam contexto de convencoes especifico do projeto

> Created: 2026-08-15 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-08-15-agentes-especialistas-aceitam-contexto-de-convencoes-especifico-do-projeto.md -->
REQ: docs/req/REQ-2026-08-15-agentes-especialistas-aceitam-contexto-de-convencoes-especifico-do-projeto.md

Levantamento de código real (não suposição) confirmou o desenho a seguir:

- `internal/config/config.go`: `ProjectConfig.Update` (`UpdateConfig` struct, linhas ~68-76)
  já tem `Hooks`, `CI`, `Backend`, `Frontend`, `PkgManager` — todos `string`, lidos em
  `parse()` (linhas ~313-328) via `stringVal(m, "<chave>")` contra a raiz plana do YAML
  (não há tags `yaml:"..."`, o parser é `yaml.Node`-based com normalização própria — ver
  `docs/adr/ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-namespaces-tipados.md`,
  que É o precedente de design a seguir aqui: chave plana no YAML, campo tipado no struct
  em memória, uma única função `parse()` como fonte de verdade nos 3 CLIs).
- `internal/generators/scaffold.go`: `Config` struct (linhas 11-26) e `writeTrackfwConfig()`
  (linhas 626-668) geram o `trackfw.yaml` inicial via `fmt.Sprintf` de um template literal —
  **achado colateral, fora de escopo desta REQ**: `backend_framework` é escrito no template
  (linha ~646) mas nunca lido de volta por `config.go` (`ProjectConfig` não tem esse campo,
  `parse()` não tem `stringVal(m, "backend_framework")`) — não corrigir aqui, é bug
  pré-existente não relacionado, documentar como observação separada se notado de novo.
- `internal/generators/agentfiles.go`: `trackfwRulesBlock()` (linha 34, **sem parâmetros
  hoje**) monta o bloco de markdown entre os delimitadores `rulesStart`/`rulesEnd`
  (`<!-- trackfw:rules:start -->` / `<!-- trackfw:rules:end -->`, linhas 11-12), injetado de
  forma idempotente por `injectOrUpdateRules(filePath, headerIfNew string)` (linha 90) nos 7
  arquivos de `agentFiles` (CLAUDE.md, AGENTS.md, GEMINI.md, `.github/copilot-instructions.md`,
  `.windsurfrules`, `.amazonq/developer/guidelines.md`, `.cursor/rules/trackfw.mdc`) via
  `InjectRulesForTool(tool, cwd)` (linha 138) e `InjectRulesDetected(cwd)` (linha 151). Este é
  o mecanismo de injeção a reaproveitar (REQ pede explicitamente preferir isso a criar um
  segundo mecanismo).
- `internal/commands/discover.go`/`internal/discover/discover.go`: `discover.Scan()` hoje
  detecta ADR dirs, REQ dir, roadmap namespacing, hook framework (lefthook/husky/pre-commit)
  e CI system — **não detecta `backend`/`frontend`/`pkg_manager`/framework de teste**; esses
  só são preenchidos pelo wizard interativo `trackfw init` (`internal/commands/init.go`,
  prompts `huh` a partir da linha 138), sem nenhuma heurística de arquivo (sem sniff de
  `package.json`/`go.mod`/`requirements.txt`). Não existe hoje um precedente de heurística de
  detecção de convenção — a Wave 2 desta REQ constrói esse precedente do zero.
- Node.js: `npm/src/config/index.js` (`parse()` linha 194), `npm/src/generators/init.js`
  (`trackfwRulesBlock()` linha 401, injeção linha 463, export linha 1357).
- Python: `pypi/trackfw/config.py` (`defaults()` linha 74, `_parse()` linha 192, leitura de
  backend/frontend/pkg_manager linhas 284-289), `pypi/trackfw/generators/init_gen.py`
  (`_trackfw_rules_block()` linha 234, `_inject_or_update_rules()` linha 283).

**Decisão de design deste roadmap:** novo campo de texto livre `agent_conventions` (string,
YAML block scalar `|`, pode ser multi-linha) na raiz do `trackfw.yaml`, seguindo EXATAMENTE
o padrão de `backend`/`frontend`/`pkg_manager` (chave plana, campo string em
`UpdateConfig`/equivalente, sem schema rígido — texto livre, como a REQ pede). Não reaproveitar
`backend`/`frontend` (são nomes de stack, não descrição de convenção) — campo novo dedicado.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] Novo campo `agent_conventions` em `trackfw.yaml` (texto livre, opcional, vazio por
      padrão) nos 3 CLIs.
- [x] `trackfwRulesBlock()` (e equivalentes Node/Python) ganha seção "### Project
      Conventions" que só aparece quando `agent_conventions` não está vazio — injetada nos
      mesmos 7 arquivos de agente já cobertos por `InjectRulesForTool`/`InjectRulesDetected`.
- [x] `trackfw discover --init`/`trackfw init` propõe (não força) um valor inicial de
      framework de teste detectado por heurística de arquivo simples (best-effort).
- [x] Comportamento idêntico nos 3 CLIs (mesmo campo, mesmo texto injetado, byte-idêntico).
- [x] `make quality` passa sem novas divergências de paridade.
- [x] Documentação explícita (no próprio texto do bloco injetado E no roadmap) de que é
      convenção declarada pelo time, não inferência automática de "boas práticas".

## Wave 1 — Go (implementação de referência, 2 MLs)
> Dependências: nenhuma

### ML-1A — Campo `agent_conventions` (config + scaffold) + injeção no rules block
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/config/config.go` — `UpdateConfig` struct (~linha 68-76): adicionar
  `AgentConventions string // agent_conventions: free-text, multi-line (default: "")`;
  `parse()` (~linha 320-328): adicionar `if v, ok := stringVal(m, "agent_conventions"); ok { cfg.Update.AgentConventions = v }`
  logo após o bloco existente de `backend`/`frontend`/`pkg_manager`; nova função exportada
  `ReadAgentConventions(cwd string) string` que lê `filepath.Join(cwd, "trackfw.yaml")`
  diretamente (não usa o singleton `Load()`, mesmo padrão de isolamento de
  `ParseRulesFromContent`, ~linha 143) — arquivo ausente ou chave ausente retornam `""`
  silenciosamente, nunca erro.
- `internal/config/config_test.go` (ou novo `config_agent_conventions_test.go`) — testes de
  `parse()` com `agent_conventions:` presente/ausente/multi-linha, e de `ReadAgentConventions`
  com arquivo ausente/presente.
- `internal/generators/scaffold.go` — `Config` struct (~linha 11-26): adicionar
  `AgentConventions string`; `writeTrackfwConfig()` (~linha 626): adicionar ao template,
  condicional (só escreve a chave se `cfg.AgentConventions != ""`, para não poluir todo
  `trackfw.yaml` novo com uma chave vazia) — usar o MESMO padrão condicional já usado para
  `forge:` alguns parágrafos abaixo no mesmo template (`if cfg.Forge != "" { content += ... }`).
- `internal/generators/agentfiles.go` — `trackfwRulesBlock()` (linha 34) ganha parâmetro:
  `func trackfwRulesBlock(agentConventions string) string`; dentro do corpo, inserir um novo
  bloco condicional ANTES da seção `### Key Commands`:
  ```go
  conventionsSection := ""
  if strings.TrimSpace(agentConventions) != "" {
      conventionsSection = `

### Project Conventions
> Declared by the team in ` + "`trackfw.yaml`" + `'s ` + "`agent_conventions`" + ` field — NOT
> inferred automatically. trackfw does not impose an architectural standard; it only
> propagates what the project has already decided.

` + strings.TrimSpace(agentConventions) + `
`
  }
  ```
  e interpolar `conventionsSection` no ponto de junção (a string final continua terminando em
  `+ rulesEnd`, só muda o que vem imediatamente antes de `### Key Commands`). `injectOrUpdateRules`
  (linha 90) ganha parâmetro `cwd string`; sua única linha que chama `trackfwRulesBlock()`
  passa a ser `block := trackfwRulesBlock(config.ReadAgentConventions(cwd))` (import do pacote
  `internal/config` neste arquivo, hoje ausente — adicionar). `InjectRulesForTool`/
  `InjectRulesDetected` (já recebem `cwd`) passam esse `cwd` adiante para `injectOrUpdateRules`.
- `internal/generators/agentfiles_test.go` (ou teste equivalente já existente para
  `trackfwRulesBlock`/`injectOrUpdateRules`) — caso com `agent_conventions` vazio (seção
  ausente do bloco final, byte-idêntico ao comportamento pré-ML) e caso com conteúdo (seção
  presente, texto do team preservado verbatim).
**Ações:** implementar exatamente como especificado acima; rodar
`go build ./... && go test ./internal/config/... ./internal/generators/... && go vet ./...`.
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/config/... ./internal/generators/...` verde
- [ ] `go vet ./...` sem warnings
- [ ] Teste manual: `trackfw.yaml` com `agent_conventions: |\n  Use pytest, not unittest.\n  API REST, no GraphQL.`
      → `trackfw agents update` (ou `discover --init` num scaffold de teste) → `CLAUDE.md`
      contém a seção "### Project Conventions" com o texto exato; `trackfw.yaml` sem a chave
      → bloco injetado idêntico ao gerado antes deste ML (sem a seção)
**Comandos de validação:** `go build ./... && go test ./internal/config/... ./internal/generators/... && go vet ./...`

**Execução real (2026-08-15):** implementado por Apolo, auditado pelo orquestrador. Build/vet
verdes, `go test ./...` (suíte completa) sem regressões. Testes cobrem explicitamente o caso
crítico de não-regressão: `TestInjectOrUpdateRules_NoTrackfwYAML_NoRegression` prova saída
byte-idêntica ao comportamento pré-ML quando não há `agent_conventions`. Teste manual real via
`trackfw update` (entrypoint real de `InjectRulesDetected`) confirmou a seção "### Project
Conventions" presente/ausente conforme esperado.

### ML-1B — Heurística de sugestão de framework de teste em `trackfw discover`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/discover/discover.go` — `DiscoveryResult` struct (~linha 281-296): adicionar
  campo `SuggestedTestFramework string`; `Scan()` (~linha 299-407): adicionar heurística
  best-effort, mesmo estilo da detecção de hook framework já existente (linhas ~375-384) —
  checar presença de arquivo na raiz do projeto: `jest.config.js`/`jest.config.ts` →
  `"jest"`; `vitest.config.js`/`vitest.config.ts` → `"vitest"`; `pytest.ini`/`pyproject.toml`
  com seção `[tool.pytest...]`/`setup.cfg` com `[tool:pytest]` → `"pytest"`; `go.mod` presente
  E qualquer `*_test.go` no repo → `"go test"`; nenhum match → `""` (nunca erro, é sugestão).
- `internal/commands/discover.go` — no relatório impresso (texto, não `--json`), se
  `r.SuggestedTestFramework != ""`, imprimir uma linha
  `Suggested test framework: <valor> (add to trackfw.yaml as agent_conventions: if correct)`
  — é SUGESTÃO impressa, nunca escrita automaticamente em `trackfw.yaml` (REQ exige que a
  convenção seja SEMPRE declarada pelo time, não inferida/gravada automaticamente — este ML
  é só o `discover` --report opinando no texto, não o `--init` escrevendo o campo).
- Testes correspondentes em `internal/discover/discover_test.go` e
  `internal/commands/discover_test.go` (fixtures com/sem cada um dos arquivos-gatilho).
**Ações:** implementar a heurística e a linha de sugestão exatamente como especificado;
não escrever `agent_conventions` automaticamente em nenhum fluxo — isso violaria a AC de
"declarado pelo time, não inferido".
**Critérios de aceite:**
- [ ] `go build ./... && go test ./internal/discover/... ./internal/commands/...` verde
- [ ] Teste manual: rodar `trackfw discover` num fixture com `jest.config.js` → linha de
      sugestão aparece; fixture sem nenhum arquivo-gatilho → linha ausente,
      `trackfw.yaml` gerado por `--init` continua sem `agent_conventions`
**Comandos de validação:** `go build ./... && go test ./internal/discover/... ./internal/commands/...`

**Execução real (2026-08-15):** implementado por Apolo, auditado pelo orquestrador. Build/vet
verdes, `go test ./...` (repo inteiro) sem regressões. Heurística: jest → vitest → pytest
(`pytest.ini`/`pyproject.toml`/`setup.cfg`) → go test, `""` se nada bater. Teste manual real
confirmou a linha de sugestão presente/ausente conforme o fixture, e que
`trackfw discover --init` nunca escreve `agent_conventions` automaticamente (0 ocorrências no
`trackfw.yaml` gerado mesmo com a sugestão impressa).

## Wave 2 — Node.js e Python (2 MLs em paralelo — arquivos distintos por stack)
> Dependências: Wave 1 completa

### ML-2A — Node.js
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/config/index.js` — `parse()` (linha 194): ler `agent_conventions` do YAML root
  para `cfg.update.agent_conventions` (ou equivalente camelCase se for o padrão já usado
  neste arquivo para `backend`/`frontend` — confirmar convenção de nomeação real antes de
  escolher, não presumir); nova função exportada `readAgentConventions(cwd)` espelhando
  `ReadAgentConventions` do Go.
- `npm/src/generators/init.js` — `trackfwRulesBlock()` (linha 401) ganha parâmetro
  `agentConventions`; injeção da seção "### Project Conventions" idêntica (byte-a-byte) ao
  Go; `injectOrUpdateRules`-equivalente (linha 463) passa a chamar
  `readAgentConventions(cwd)`.
- `npm/src/commands/discover.js` — port do ML-1B: mesma heurística de arquivo-gatilho, mesma
  linha de sugestão impressa.
- Testes Node correspondentes (arquivo de teste já existente para rules-block/discover, ou
  novo `npm/tests/agent-conventions.test.js`).
**Ações:** replicar 1:1 a lógica das Waves 1A/1B em JS puro, lendo o Go real (já
implementado nesta branch) como fonte de verdade — mensagens/textos byte-idênticos.
**Critérios de aceite:**
- [ ] `cd npm && npm test` verde
- [ ] Seção "### Project Conventions" e linha de sugestão de `discover` byte-idênticas ao Go
**Comandos de validação:** `cd npm && npm test`

**Execução real (2026-08-15):** `npm test` → 550 passed, 0 failed (27 novos + suíte completa
sem regressão). Paridade byte-a-byte confirmada pelo orquestrador contra o binário Go nos 3
cenários (com/sem `agent_conventions`, sugestão de `discover`). Achado incidental, fora de
escopo: `roadmap_namespacing` diverge entre Go/Node num fixture de `--init` sem
`docs/roadmaps/` — não relacionado a esta REQ, registrado como observação, não investigado
aqui.

### ML-2B — Python
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/trackfw/config.py` — `defaults()` (linha 74): adicionar `"agent_conventions": ""`
  dentro do sub-dict `"update"`; `_parse()` (linha 192, leitura ~284-289): adicionar leitura
  de `agent_conventions`; nova função `read_agent_conventions(cwd)` espelhando o Go.
- `pypi/trackfw/generators/init_gen.py` — `_trackfw_rules_block()` (linha 234) ganha
  parâmetro `agent_conventions`; `_inject_or_update_rules()` (linha 283) passa a chamar
  `read_agent_conventions(cwd)`.
- `pypi/trackfw/commands/discover.py` — port do ML-1B: mesma heurística, mesma linha de
  sugestão.
- Testes Python correspondentes (`pypi/tests/test_agent_conventions.py`, novo, ou extensão
  dos testes já existentes de rules-block/discover).
**Ações:** replicar 1:1 a lógica das Waves 1A/1B em Python puro, lendo o Go real como fonte
de verdade.
**Critérios de aceite:**
- [ ] `python -m pytest pypi/tests -k agent_conventions` verde (e suíte completa sem
      regressão)
- [ ] Seção e linha de sugestão byte-idênticas ao Go
**Comandos de validação:** `python -m pytest pypi/tests -k agent_conventions`

**Execução real (2026-08-15):** `pytest -k agent_conventions` → 30 passed; suíte completa →
1172 passed, 8 subtests, 0 falhas. Paridade byte-a-byte confirmada pelo orquestrador contra o
binário Go nos 3 cenários (nota: a primeira tentativa de verificação deu falso-negativo por
`PYTHONPATH` relativo não sobreviver a `cd` entre diretórios — corrigido usando path absoluto;
o código em si estava correto desde a entrega do agente).

## Wave 3 — Validação cruzada e documentação (1 ML)
> Dependências: Wave 2 completa

### ML-3A — Paridade, teste manual end-to-end e documentação explícita
**Status:** ✅ Concluído
**Arquivos afetados:**
- `docs/cli-parity.md` — nova linha/seção documentando `agent_conventions` (chave de
  `trackfw.yaml`, não um comando — ver formato já usado para outras chaves de config no
  arquivo, se houver uma seção de config keys; senão, adicionar uma).
- Nenhum outro arquivo novo.
**Ações:**
1. Rodar `make quality` na raiz.
2. Fixture manual: `trackfw.yaml` com `agent_conventions: |\n  <texto de exemplo>` → rodar
   a injeção (`trackfw agents update` ou equivalente) nos 3 binários → `CLAUDE.md` (ou
   arquivo de agente equivalente) resultante idêntico byte-a-byte nos 3.
3. Fixture manual: `trackfw.yaml` sem `agent_conventions` → confirmar que o bloco injetado é
   IDÊNTICO ao gerado antes deste roadmap (sem a seção "### Project Conventions") — prova de
   não-regressão para todo projeto que já usa trackfw hoje.
4. Confirmar que em nenhum ponto do fluxo (`init`, `discover`, `discover --init`,
   `agents update`) o valor de `agent_conventions` é escrito automaticamente — só lido
   quando já presente no `trackfw.yaml` do usuário.
**Critérios de aceite:**
- [x] `make quality` verde
- [x] Os 2 cenários (com/sem `agent_conventions`) confirmados byte-idênticos nos 3 CLIs
- [x] Confirmado que nada escreve `agent_conventions` automaticamente em nenhum fluxo
**Comandos de validação:** `make quality`

**Execução real (2026-08-15):** `make quality` verde (build+vet+test Go, `npm test` 550
passed, `pytest` 1172 passed, 112 cenários de falsificação todos OK). Os 3 cenários (com/sem
`agent_conventions`, sugestão de `discover`) confirmados byte-idênticos entre os 3 binários
via `diff` direto pelo orquestrador — incluindo o caso Python, onde uma primeira checagem deu
falso-negativo por erro de `PYTHONPATH` relativo no script de teste manual (corrigido,
implementação em si estava correta). Nenhum fluxo (`init`, `discover`, `discover --init`,
`update`) escreve `agent_conventions` automaticamente — confirmado nos 3 CLIs. Documentada a
chave `agent_conventions` em `docs/cli-parity.md` (tabela de campos `Update`/`Sync` lidos
pelo config loader único, agora 12 campos). Achado incidental fora de escopo: divergência
`roadmap_namespacing` Go/Node em fixture `--init` sem `docs/roadmaps/` — registrado como
observação para investigação futura, não bloqueia esta REQ.

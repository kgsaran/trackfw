---
status: done
date: 2026-07-25
req: "REQ-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills"
squad: ""
---

# Roadmap: Escopo de instalação selecionável para agents e skills

> Criado em: 2026-07-25 | Status: done
REQ: `docs/req/REQ-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md`
ADR: `docs/adr/ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md`
Branch: `fix/escopo-de-instalacao-selecionavel-para-agents-e-skills`

## Critérios de Aceite

- [x] Em TTY, sem `--scope`, `agents|skills install|update|uninstall` perguntam o escopo
      com `global` pré-selecionado
- [x] O prompt dispara mesmo com `--targets` informado (gate independente)
- [x] Sem TTY e sem `--scope` → escopo `global` (exceto `uninstall`, que exige `--scope`
      explícito por D8)
- [x] `--scope` explícito é respeitado e não dispara prompt (detecção por *flag-set*)
- [x] `trackfw init` pergunta o escopo; sem TTY → `global`
- [x] Caminhos de destino impressos antes da gravação (fora de `--json`)
- [x] `make build && make test && make lint && make quality` verdes
- [x] `trackfw validate` sem violações (além das 2 pré-existentes de
      `REQ-2026-07-24-corrige-resolve-de-integrations-em-windows...`, fora do escopo
      desta REQ)

## Diagnóstico / Contexto

Ao rodar `trackfw agents install` (ou `skills install`), os artefatos são gravados
**silenciosamente no projeto atual** (`.claude/agents/`, `.claude/skills/`), sem que o
usuário tenha escolhido isso. O comportamento esperado é: instalar na pasta do usuário
(`~/.claude/...`) por padrão, ou **perguntar** ao usuário qual escopo deseja.

### Causa raiz (análise estática — 3 CLIs)

| CLI | Arquivo:linha | Evidência |
|---|---|---|
| Go | `internal/commands/integrations_flags.go:105` | `StringVar(&opts.scope, "scope", "project", ...)` |
| Node | `npm/src/commands/integrations.js:50` | `.option('--scope <scope>', ..., 'project')` |
| Python | `pypi/trackfw/integrations/command.py:94` | `default="project"` |
| Python | `pypi/trackfw/integrations/catalog.py:59` | `scope: str = "project"` (fallback secundário) |
| Go (init) | `internal/commands/init.go:358` | `Scope: "project"` hardcoded |

Nenhum dos 3 CLIs possui prompt de escopo. O único prompt existente
(`promptIntegrationSelection`, Go `integrations_flags.go:343`) pergunta apenas **CLIs alvo**
e **itens**, e só dispara quando `--targets` está vazio — ou seja, o caso mais comum
(`trackfw agents install --targets claude`) não passa por prompt algum.

Todos os 11 surfaces do catálogo (`internal/integrations/assets/catalog.json`) declaram
`"scopes": ["global", "project"]` — não há restrição técnica por target.

### Decisões travadas com o usuário (2026-07-25)

- **D1 — Não-interativo (sem TTY) e sem `--scope`:** default passa a ser **`global`**.
  Breaking change documentado no CHANGELOG.
- **D2 — Interativo (TTY) e sem `--scope`:** **perguntar** o escopo (`global` / `project`),
  com `global` como opção pré-selecionada.
- **D3 — `--scope` explícito:** sempre respeitado, nunca pergunta. Detecção obrigatória por
  *flag-set*, não por comparação de valor (ver Armadilha crítica abaixo).
- **D4 — `trackfw init`:** também passa a perguntar o escopo no wizard.
- **D5 — Confirmação:** nenhuma confirmação extra. Após a escolha, **imprimir os caminhos
  de destino resolvidos** antes de gravar.

### ⚠️ Armadilha crítica (leia antes de implementar)

Comparar `opts.scope == "project"` **não funciona** — é impossível distinguir um
`--scope project` explícito do valor default. Isso faria o CLI re-perguntar a usuários que
já escolheram `project`. A detecção deve ser:

- **Go:** `cmd.Flags().Changed("scope")`
- **Node:** remover o `'project'` de `.option(...)` e testar `options.scope === undefined`
- **Python:** `default=None` em `command.py:94`, resolver após o parse

### Fora de escopo (§11 — não expandir)

- `internal/generators/agents.go:16` (`InstallAgents`) — hardcoda `~/.claude/agents/`,
  **sem callers de produção** (só testes). Caminho morto: deixar como está, apenas registrar.
- `internal/generators/scaffold.go:95` (`ForceInstallSkills`, chamado por
  `internal/generators/update.go:122`) e `npm/src/generators/init.js:1043`
  (`installSkillsForce`) — instalam a **skill do próprio trackfw** em
  `~/.claude/skills/trackfw/`, que é global por natureza. Não é a skill da constelação.
  Não alterar.
- Nenhum refactor da camada `internal/integrations` além do necessário para o escopo.

---

## Wave 1 — Implementação por CLI (3 MLs em paralelo)
> Dependências: nenhuma. Os 3 MLs tocam árvores de arquivos disjuntas
> (`internal/` × `npm/src/` × `pypi/trackfw/`) — spawn simultâneo obrigatório.

### ML-1A — CLI Go: prompt e default de escopo
**Status:** ✅ Concluído (commit `fb33bbb`)
**Agente:** trackfw-backend
**Arquivos afetados (exclusivos deste ML):**
- `internal/commands/integrations_flags.go`
- `internal/commands/init.go`
- `internal/commands/amazonq.go`, `copilot.go`, `cursor.go`, `gemini.go`, `windsurf.go`
- `internal/commands/agents_skills_test.go`
- `internal/integrations/manager_test.go`

**Ações:**
1. `addIntegrationFlags` (`integrations_flags.go:105`): trocar o default de `"project"` por
   `""` (string vazia). Atualizar o texto de ajuda para
   `"installation scope: project or global (default: global; asks interactively)"`.
2. Criar `resolveScope(cmd *cobra.Command, opts *integrationOptions) error`:
   - Se `cmd.Flags().Changed("scope")` → validar contra `{project, global}` e retornar.
   - Senão, se `!integrationsStdinIsTTY()` → `opts.scope = "global"`.
   - Senão → `huh.NewSelect[string]()` com título
     `"Onde instalar os artefatos?"`, opções
     `"Pasta do usuário (~/.claude) — vale para todos os projetos"` → `global` e
     `"Este projeto (.claude) — apenas neste repositório"` → `project`,
     com `global` pré-selecionado. Gravar em `opts.scope`.
3. Chamar `resolveScope` em `executeIntegrationMutation` **como gate separado**,
   imediatamente após o `LoadCatalog()` e **antes** da checagem
   `if opts.scope != "project" && opts.scope != "global"` (que passa a ser redundante e
   deve ser removida, pois `resolveScope` já valida). Não colocar dentro do bloco
   `if len(opts.targets) == 0` — esse bloco não roda quando `--targets` é passado.
4. Mesmo tratamento em `executeIntegrationList` (`integrations_flags.go:245`): sem TTY ou
   sem flag → `global`. **Não** perguntar em `list` (comando de leitura); apenas usar o
   default `global`. Se o `list` continuar assumindo `project`, ele reportará deployments
   diferentes dos que o `install` gravou.
5. `runDeprecatedIntegrationAlias` (`integrations_flags.go:397`): os aliases passam scopes
   fixos e chamam `executeIntegrationMutation` com `opts.scope` já preenchido — garantir
   que `resolveScope` respeite isso. Como esses aliases não passam por `cmd.Flags()`,
   adicionar ao `integrationOptions` um campo `scopeExplicit bool` setado pelos aliases,
   e fazer `resolveScope` retornar cedo quando `opts.scopeExplicit || cmd.Flags().Changed("scope")`.
   Comportamento dos aliases deprecados não muda.
6. `init.go:358`: substituir `Scope: "project"` por uma variável resolvida. Em
   `installAITools`, aceitar um parâmetro `scope string`; no wizard de `init`, perguntar o
   escopo com o mesmo `huh.NewSelect` (extrair a construção do select para uma função
   compartilhada `promptInstallScope() (string, error)` para não duplicar strings).
   Sem TTY em `init` → `global`.
7. **D5 — imprimir destinos:** antes de `manager.Install(...)`, quando `!opts.json`,
   imprimir em `cmd.OutOrStdout()`:
   `"Destino (%s):\n"` seguido de uma linha `"  %s\n"` por `plan.Destination`.
8. Atualizar os testes que assumem `project`: `agents_skills_test.go` e
   `manager_test.go` (linhas 13, 30, 282, 315, 323). Em testes,
   `integrationsStdinIsTTY` deve ser stubado para `false` (já é uma var substituível).
9. **Novos testes obrigatórios:**
   - `--scope project` explícito **não** dispara prompt e grava em `.claude/`.
   - Sem TTY e sem `--scope` grava em `~/.claude/`.
   - `--targets claude` sem `--scope`, com TTY stubado, aciona o resolvedor de escopo.
   - `list` sem `--scope` reporta destinos globais.

**Critérios de aceite:**
- [x] `go build ./...` sem erros
- [x] `go vet ./...` limpo
- [x] `go test ./...` verde
- [x] `trackfw agents install --targets claude` (TTY) pergunta o escopo
- [x] `trackfw agents install --targets claude --scope project` **não** pergunta
- [x] Destinos resolvidos impressos antes da gravação

**Comandos de validação:**
```bash
go build ./... && go vet ./... && go test ./...
```

---

### ML-1B — CLI Node.js: prompt e default de escopo
**Status:** ✅ Concluído (commit `ac8b45b`)
**Agente:** trackfw-backend (segunda instância)
**Arquivos afetados (exclusivos deste ML):**
- `npm/src/commands/integrations.js`
- `npm/src/commands/init.js`
- testes correspondentes em `npm/`

**Ações:**
1. `integrations.js:50`: remover o terceiro argumento `'project'` de
   `.option('--scope <scope>', 'Installation scope: project or global')`.
2. Criar `resolveScope(options)` async:
   - `options.scope` definido → validar contra `['project','global']`, lançar
     `new Error(\`Unsupported scope: ${options.scope}\`)` se inválido (mantendo a mensagem
     atual da linha 68), e retornar.
   - `!process.stdin.isTTY` → `'global'`.
   - TTY → prompt de seleção com as mesmas duas opções e textos do ML-1A, `global`
     pré-selecionado. Usar o mesmo mecanismo de prompt já empregado pelo
     `identity-wizard.js` deste CLI (não introduzir dependência nova).
3. Chamar `resolveScope` como gate separado, antes de qualquer construção de plano e
   independentemente de `--targets` ter sido passado.
4. Substituir a validação da linha 68 pela validação interna de `resolveScope`.
5. `commands/init.js`: perguntar o escopo no wizard e propagá-lo ao build de planos;
   sem TTY → `global`.
6. **D5:** imprimir os destinos resolvidos antes de gravar, quando não estiver em `--json`.
7. **Não tocar** em `npm/src/generators/init.js:1043` (`installSkillsForce`) — fora de escopo.
8. Espelhar os 4 casos de teste listados no ML-1A.

**Critérios de aceite:**
- [x] `npm test` (workspace `npm/`) verde
- [x] `--scope project` explícito não dispara prompt
- [x] Sem TTY e sem `--scope` → destino `~/.claude/`
- [x] Mensagem de erro para escopo inválido idêntica à do CLI Go

**Comandos de validação:**
```bash
cd npm && npm test
```

---

### ML-1C — CLI Python: prompt e default de escopo
**Status:** ✅ Concluído (commit `5acf8f1`)
**Agente:** trackfw-backend (terceira instância)
**Arquivos afetados (exclusivos deste ML):**
- `pypi/trackfw/integrations/command.py`
- `pypi/trackfw/integrations/catalog.py`
- `pypi/trackfw/commands/init.py`
- testes correspondentes em `pypi/`

**Ações:**
1. `command.py:94`: trocar
   `child.add_argument("--scope", choices=("project","global"), default="project")`
   por `default=None`.
2. Criar `resolve_scope(args) -> str`:
   - `args.scope is not None` → retornar (o `choices` do argparse já validou).
   - `not sys.stdin.isatty()` → `"global"`.
   - TTY → prompt de seleção com as mesmas duas opções/textos do ML-1A,
     `global` como default (Enter vazio → `global`). Usar o mesmo mecanismo de prompt de
     `pypi/trackfw/commands/identity_wizard.py`, sem dependência nova.
3. Chamar `resolve_scope` como gate separado, antes da construção de planos e independente
   de `--targets`.
4. `catalog.py:59`: trocar `scope: str = "project"` por `scope: str` (parâmetro obrigatório)
   ou, se isso quebrar callers, manter a assinatura mas garantir que **nenhum** caller de
   produção dependa do default. Documentar a escolha em comentário.
5. `commands/init.py`: perguntar o escopo no wizard; sem TTY → `"global"`.
6. **D5:** imprimir os destinos resolvidos antes de gravar (exceto em modo JSON).
7. Espelhar os 4 casos de teste listados no ML-1A.

**Critérios de aceite:**
- [x] Suíte de testes Python verde
- [x] `--scope project` explícito não dispara prompt
- [x] Sem TTY e sem `--scope` → destino `~/.claude/`
- [x] `manager.py:35` continua resolvendo `home_dir` corretamente para `global`

**Comandos de validação:**
```bash
cd pypi && python -m pytest
```

---

## Wave 2 — Guarda de segurança do uninstall (barrier)
> Dependências: **aguardar ML-1A, ML-1B e ML-1C concluídos**.

### ML-2Z — `uninstall` não herda o default `global` sem TTY (ADR D8)
**Status:** ⬜ Pendente
**Agente:** trackfw-backend
**Arquivos afetados:** os 3 CLIs (Wave 1 encerrada, sem risco de colisão)

**Contexto — regressão de segurança detectada em auditoria pós-ML-1A:**
`trackfw agents uninstall --targets claude --json` sem TTY resolveu destinos como
`~/.claude/agents/trackfw-*.md`. Um script de CI que antes limpava o repositório passaria a
**apagar os agentes do home do usuário**. Ver ADR D8.

**Ações:** em `resolveScope` (e equivalentes Node/Python), quando a operação for `uninstall`
E não houver TTY E `--scope` não tiver sido informado → retornar erro exigindo `--scope`
explícito, no formato do precedente existente
(`"uninstall requires --scope in non-interactive mode"`). `install` e `update` mantêm o
default `global`. Em TTY, `uninstall` continua perguntando normalmente.

**Critérios de aceite:**
- [x] `uninstall --targets X` sem TTY e sem `--scope` falha com erro claro (teste nos 3 CLIs)
- [x] `uninstall --targets X --scope global` sem TTY funciona
- [x] `install`/`update` sem TTY e sem `--scope` continuam resolvendo `global`
- [x] Build/testes verdes nos 3 CLIs

---

## Wave 3 — Paridade, docs e changelog (barrier)
> Dependências: **aguardar ML-2Z concluído**.

### Divergências da Wave 1 a reconciliar no ML-2A

1. **`init` só pergunta o escopo quando há ferramentas de IA selecionadas** (`len(aiTools) > 0`) —
   decisão autônoma do ML-1A. Confirmar que Node e Python espelham essa condição.
   ✅ **Confirmado — sem divergência.** Go (`init.go:332`, `if len(aiTools) > 0`),
   Node (`init.js:193`, `aiTools.length ? await resolveScope(...) : 'global'`) e
   Python (`init.py:119`, `if ai_tools:`) usam exatamente a mesma condição. Nenhuma
   mudança necessária.
2. **Impressão de destinos (D5) no `init`:** Go imprime em `installAITools`; Node **não**
   imprime em `commands/init.js`. Uniformizar.
   ✅ **Divergência real, corrigida.** Python de fato já imprimia (`init.py:137-148`,
   "Destino:"); apenas o Node ficava silencioso, porque `installIntegrationTarget`
   (`npm/src/generators/init.js:1019`) chama `execute()` diretamente (camada baixa),
   sem passar pelo `printResolvedDestinations` que só existe em
   `commands/integrations.js`. Corrigido: `installIntegrationTarget` agora chama
   `buildPlans` (sem efeito colateral) para imprimir `"Destino (<scope>):"` e os
   caminhos únicos, antes do `execute()` real, no mesmo formato usado pelo restante
   do CLI Node. Teste novo:
   `npm/tests/identity-init.test.js::init --ai-tools claude sem TTY imprime destinos antes da gravação (D5)`.
3. **Caminhos que não passam pelo gate (Node):** `npm/src/commands/update.js` e
   `npm/src/generators/codex.js` chamam `buildPlans`/`execute` com `scope: 'project'` fixo.
   Avaliar se é divergência real ou caminho legítimo, e registrar a conclusão.
   ✅ **Caminho legítimo, não é divergência.** Confirmado que Go e Python têm os
   mesmos dois caminhos, fixos em `"project"` da mesma forma:
   - Gerador do próprio Codex (`internal/generators/codex.go:InstallCodex`,
     `npm/src/generators/codex.js:installCodex`,
     `pypi/trackfw/generators/codex.py:install_codex`): grava `AGENTS.md`/`.codex/`
     diretamente no repositório — é inerentemente project-scoped pelo próprio
     desenho do Codex, não uma escolha do usuário. Go/Python nem passam pelo
     `BuildPlans`/`plan_deployments` aqui; o Node passa, mas sempre com
     `scope: 'project'` fixo, produzindo o mesmo resultado.
   - Re-sincronização do Codex dentro de `trackfw update`
     (`internal/generators/update.go:updateDetectedCodexIntegrations`,
     `npm/src/commands/update.js` (ramo `AGENTS.md`/`.codex`),
     `pypi/trackfw/commands/update.py`): re-aplica artefatos **já instalados** no
     projeto atual — não é uma instalação nova para o usuário escolher destino,
     é uma atualização de arquivos que já vivem no repo. Os 3 CLIs fixam
     `scope: "project"` pela mesma razão. Documentado em `docs/cli-parity.md`
     ("Internal codex-sync paths fixed to `scope: "project"`").
4. **Aliases deprecados só existem no CLI Go** (`copilot`, `cursor`, `gemini`, `windsurf`,
   `amazonq`) — não há equivalente Node/Python. Registrar como exceção intencional em
   `docs/cli-parity.md`.
   ✅ **Já estava documentado** (`docs/cli-parity.md`, linha "gemini / cursor / copilot
   / windsurf / amazonq | yes | no | no | Historical Go-only compatibility aliases").
   Nenhuma mudança necessária.
5. **Pré-seleção `global` no prompt não tem teste** nos 3 CLIs (os testes stubam o runner).
   Go verificado manualmente (`scope := "global"` antes do `.Value(&scope)`). Verificar
   Node (`global` como default do `select`) e Python (Enter vazio → `global`).
   ✅ **Divergência real, corrigida em Go e Node; Python já estava coberto.**
   - Python: já existia `test_prompt_scope_defaults_to_global_on_bare_enter`
     (`pypi/tests/test_scope_resolution.py:119`), que estuba `builtins.input` para
     retornar `""` e chama `_prompt_scope()` de verdade. Nada a fazer.
   - Go: `promptInstallScope` foi refatorado extraindo a construção do
     `huh.Select` para `installScopeSelect(scope *string) *huh.Select[string]`
     (`internal/commands/integrations_flags.go`), permitindo exercitar o campo
     real via `Select.RunAccessible` (modo acessível do huh, lê um Enter em
     branco de um `io.Reader` falso — não é TTY nem stub do `promptInstallScopeRunner`).
     Novo teste: `TestInstallScopeSelectDefaultsToGlobal`
     (`internal/commands/agents_skills_test.go`).
   - Node: adicionado teste que captura o objeto de configuração passado ao
     `select()` real dentro de `resolveScope` (em vez de estubar `select` para
     sempre devolver `'global'` incondicionalmente) e verifica
     `capturedConfig.default === 'global'` e a ordem das choices. Novo teste:
     `resolveScope: TTY sem --scope monta o select real com "global" pré-selecionado`
     (`npm/tests/agents-skills.test.js`).
6. **Mensagem de `--targets` ausente diverge** (Go/Node: `"{operation} requires --targets in
   non-interactive mode"`; Python: `"--targets is required for non-interactive {action}"`).
   ✅ **Decisão: fora de escopo deste ML.** É uma divergência pré-existente (não
   introduzida por esta feature) e já coberta por testes próprios e independentes
   em cada CLI. Unificar a string é um fix pequeno e de baixo risco, mas ortogonal
   à reconciliação de escopo — misturaria neste commit uma mudança de contrato
   textual não relacionada a `--scope`. Registrado em `docs/cli-parity.md` como
   divergência conhecida e rastreada, para uma REQ futura dedicada.
7. **Exit code diverge no Python (2 em vez de 1)** para erro de escopo.
   ✅ **Confirmado como convenção pré-existente do CLI Python, agora documentada.**
   Não estava em `docs/cli-parity.md` antes deste ML; adicionada a seção
   "Non-zero exit codes for integration lifecycle errors" explicando que
   `pypi/trackfw/integrations/command.py` sempre saiu com `SystemExit(2)` para
   `IntegrationError | OSError | ValueError` no handler de `agents`/`skills`,
   independente desta feature.

### ML-2A — Contrato de paridade, documentação e CHANGELOG
**Status:** ✅ Concluído
**Agente:** trackfw-qa
**Arquivos afetados:**
- `docs/cli-parity.md`
- `CHANGELOG.md`
- `README.md` (se documentar `--scope`)
- testes de contrato de paridade

**Ações:**
1. Rodar `make quality` (Go + Node.js + Python + contratos de paridade) e corrigir
   divergências entre os 3 CLIs no comportamento de escopo.
2. Verificar manualmente a equivalência dos 3 CLIs nos 4 cenários:
   `--scope project` · `--scope global` · sem flag com TTY · sem flag sem TTY.
3. Atualizar `docs/cli-parity.md` registrando o novo comportamento de `--scope` como
   contrato compartilhado.
4. Inserir no topo do `CHANGELOG.md` a seção `## [Unreleased]` com:
   - `### Changed` — **BREAKING**: `agents|skills install|update|uninstall` sem `--scope`
     em modo não-interativo agora usa escopo `global` (antes: `project`). Pipelines de CI
     que dependiam do comportamento anterior devem passar `--scope project` explicitamente.
   - `### Added` — prompt interativo de escopo em `agents`, `skills` e `init`;
     impressão dos caminhos de destino antes da gravação.
5. Atualizar `README.md` / docs de uso onde `--scope` for mencionado.

**Critérios de aceite:**
- [x] `make quality` passa (inclui contratos de paridade dos 3 CLIs)
- [x] `make test` verde
- [x] `docs/cli-parity.md` reflete o novo contrato
- [x] `CHANGELOG.md` documenta o breaking change explicitamente
- [x] Os 3 CLIs produzem comportamento idêntico nos 4 cenários

**Comandos de validação:**
```bash
make build && make test && make lint && make quality
trackfw validate
```

---

## Registro de decisões autônomas

- `global` é a opção **pré-selecionada** no prompt (não apenas disponível), refletindo a
  expectativa declarada pelo usuário de que a instalação padrão seja na pasta do usuário.
- `list` não pergunta escopo (comando de leitura, não deve bloquear em prompt); apenas
  adota o mesmo default `global` para permanecer consistente com o `install`.
- Aliases deprecados (`copilot`, `cursor`, `gemini`, `windsurf`, `amazonq`) mantêm os
  scopes fixos que já passam hoje — mudá-los seria expansão de escopo não solicitada.

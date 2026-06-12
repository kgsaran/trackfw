# agents-working-context.md

> Arquivo de handoff entre sessões. Atualizar ao iniciar e ao encerrar cada ciclo de trabalho.

---

## Sessão 2026-06-11 — Sessão inaugural

### O que foi decidido e construído

**Nome:** `trackfw` — nos três artefatos: repositório GitHub, CLI e pacote npm/PyPI.

**Conceito validado:**
- Framework de governança de entrega de software: `ADR → REQ → ROADMAP → backlog/wip/blocked/done/abandoned`
- CLI stack-agnostic com `trackfw init` interativo que detecta a stack e gera gates/hooks por projeto
- Modelo de plugin para generators comunitários (padrão Prettier/ESLint)
- Distribuição como Go binary único + wrappers finos npm/PyPI/Homebrew (padrão esbuild/Biome/Turbo)
- Nome do pacote npm e PyPI `trackfw` — **confirmado disponível** em ambos os registros

**O que foi implementado (v0.1 — Foundation):**
- `cmd/trackfw/main.go` — entry point
- `internal/commands/` — root, init, adr, req, roadmap, status, validate
- `internal/generators/` — scaffold, adr, req, roadmap (com move entre estados)
- `internal/validator/` — validate (consistência ADR↔REQ↔ROADMAP) + status
- `scripts/install.sh` — `curl | sh` para instalação direta
- `Makefile` — build, test, lint, install, clean
- `docs/visao-projeto/VISION.md` — visão completa do projeto
- Go module: `github.com/trackfw/trackfw`
- Dependências: `cobra` (CLI), `huh` (wizard interativo), `lipgloss` (styling)
- Build verde ✅ | CLI `--help` funcionando ✅ | 2 commits na `main`

---

## Próxima sessão — O que fazer primeiro

### ✅ Prioridade 1 — Criar repositório no GitHub (CONCLUÍDO)
- Repo: https://github.com/kgsaran/trackfw (privado, conta pessoal kgsaran)
- Module path atualizado para `github.com/kgsaran/trackfw`
- 3 commits na main, código em sincronia com o remoto

### Prioridade 2 — GoReleaser (distribuição de binários)
- ✅ ML-1A CONCLUÍDO (Ares, 2026-06-11): `.goreleaser.yaml` criado na raiz — v2 syntax, 5 targets (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64), archives tar.gz/zip, checksum sha256
- ✅ ML-2A CONCLUÍDO (Ares, 2026-06-11): `scripts/install.sh` reescrito — detecta OS/ARCH via uname, busca versao mais recente via API GitHub, suporta curl+wget, sudo quando necessario, verificacao de PATH, idempotente
- Criar GitHub Actions workflow: `.github/workflows/release.yml` (trigger: `push tag v*`)
- Testar release local: `goreleaser release --snapshot --clean`

### Prioridade 3 — Wrapper npm
- ✅ CONCLUIDO (Afrodite, 2026-06-11): `npm/package.json` criado com conteudo exato, JSON valido
- ✅ CONCLUIDO (Afrodite, 2026-06-11): `npm/bin/.gitkeep` e `npm/scripts/.gitkeep` criados
- Pendente: `npm/scripts/postinstall.js` — baixa o binario correto para a plataforma
- Pendente: Publicar no npm como `trackfw`

### Prioridade 4 — Wrapper PyPI
- Criar `pypi/` com `setup.py` / `pyproject.toml`
- Script de instalação que baixa o binário
- Publicar no PyPI como `trackfw`

---

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Criar `npm/bin/trackfw` — wrapper JS que o npm registra como comando no PATH do usuário.

**Entregue:**
- `npm/bin/trackfw` criado com shebang `#!/usr/bin/env node`, detecção de Windows (`.exe`), `spawnSync` com `stdio: 'inherit'` e `process.argv.slice(2)`, saída de erro amigável se binário ausente.
- `chmod +x` aplicado — permissão `-rwxr-xr-x` confirmada.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Criar `npm/scripts/postinstall.js` — script que baixa o binário Go correto das GitHub Releases durante `npm install trackfw`.

**Entregue:**
- `npm/scripts/postinstall.js` criado — sem dependências externas, Node >= 14, segue redirects HTTPS 301/302, suporte a `tar.gz` (Linux/macOS) via `tar -xzf` e `.zip` (Windows) via PowerShell `Expand-Archive`, `chmod 755` no Unix, `exit(0)` em plataforma/arch não suportada ou erro (não bloqueia CIs).
- Versão lida do `npm/package.json` em tempo de execução.
- Sintaxe validada com `node --check`.

---

## Decisões técnicas registradas

| Decisão | Escolha | Motivo |
|---|---|---|
| Linguagem do CLI | Go | Binário único sem runtime, cross-platform, startup rápido |
| Distribuição | Binary + wrappers | Padrão esbuild/Biome/Turbo — agnóstico de runtime |
| CLI framework | cobra | Padrão da comunidade Go para CLIs |
| Wizard interativo | huh (charmbracelet) | TUI elegante, bem mantido |
| Estado do roadmap | Pasta = fonte de verdade | Sem DB, sem SaaS, portável |
| Extensibilidade | Plugin model (generators) | Comunidade contribui sem tocar core |

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Criar pacote Python `pypi/trackfw/` — módulo Python do wrapper PyPI.

**Entregue:**
- `pypi/trackfw/__init__.py` criado (arquivo vazio — declara o pacote Python).
- `pypi/trackfw/_cli.py` criado — entry point PyPI sem dependências externas, Python 3.6+, detecta OS/ARCH, baixa binário Go das GitHub Releases (`tar.gz` Linux/macOS, `.zip` Windows), `os.execv` no Unix / `subprocess.run` no Windows, armazena binário como `trackfw-bin` dentro do pacote.
- Sintaxe validada com `python3 -m py_compile` — OK.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Corrigir Bug 1 (URL hardcoded org errada em `scaffold.go`) e Bug 2 (`containsIgnoreCase` não case-insensitive em `roadmap.go`).

**Entregue:**
- `internal/generators/scaffold.go`: substituídas 2 ocorrências de `https://raw.githubusercontent.com/trackfw/trackfw/main/scripts/install.sh` por `https://github.com/kgsaran/trackfw/releases/latest/download/install.sh` (linha GitHub Actions e linha GitLab CI).
- `internal/generators/roadmap.go`: adicionado import `"strings"`, substituídas `containsIgnoreCase` + `containsRune` por implementação correta via `strings.ToLower` + `strings.Contains`.
- `go build ./...` passou sem erros.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Adicionar comando `trackfw version`.

**Entregue:**
- `internal/version/version.go` criado — variável `Version = "dev"` injetável via ldflags em tempo de build.
- `internal/commands/version.go` criado — comando cobra `version` que imprime `trackfw <Version>`.
- `internal/commands/root.go` atualizado — `newVersionCmd()` registrado na lista de subcomandos.
- `.goreleaser.yaml` atualizado — ldflags com `-X 'github.com/kgsaran/trackfw/internal/version.Version={{.Version}}'`.
- `go build ./...` sem erros; `go run ./cmd/trackfw version` imprime `trackfw dev`.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Adicionar Regras 3, 4 e 5 ao `internal/validator/validator.go`.

**Entregue:**
- `validateBlockedHasREQ()` — verifica roadmaps em `docs/roadmaps/blocked/` sem campo `REQ:` preenchido.
- `validateREQsHaveRoadmap()` — verifica REQs em `docs/req/` sem campo `Roadmap:` preenchido.
- `validateADRsAreReferenced()` — verifica ADRs em `docs/adr/` não referenciados em nenhum REQ (campo `ADR:` dos REQs).
- As três funções registradas em `Validate()` após as chamadas existentes.
- `go build ./...` e `go vet ./...` passaram sem erros.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Configurar distribuição Homebrew para trackfw.

**Entregue:**
- Repositório `kgsaran/homebrew-trackfw` criado no GitHub (público) com `Formula/trackfw.rb` placeholder.
- `.goreleaser.yaml` — seção `brews:` adicionada ao final: aponta para `kgsaran/homebrew-trackfw`, diretório `Formula`, token via `HOMEBREW_TAP_GITHUB_TOKEN`, `skip_upload: auto`.
- `.github/workflows/release.yml` — `HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}` adicionado ao `env:` do step goreleaser.
- `goreleaser check` confirma `configuration is valid` (aviso de deprecação esperado: `brews` é a chave correta para CLI formulas em v2.16.0; `homebrew_casks` é para apps GUI).

**Pendente (ação do usuário):**
- Criar PAT com scope `repo` (para push no tap) e cadastrar como secret `HOMEBREW_TAP_GITHUB_TOKEN` no repo `kgsaran/trackfw` (Settings > Secrets > Actions).

---

## Sessão 2026-06-11 — Artemis (CONCLUÍDO)

**Tarefa:** Escrever testes unitários Go para `internal/validator` e `internal/generators`.

**Entregue:**
- `internal/validator/validator_test.go` — 7 testes: Clean, WIPMissingREQ, WIPMissingAcceptanceCriteria, MultipleWIP, REQMissingADR, BlockedMissingREQ, GetStatus_Empty
- `internal/generators/roadmap_test.go` — 5 testes: NewRoadmap_CreatesFile, MoveRoadmap_Valid, MoveRoadmap_InvalidState, MoveRoadmap_NotFound, ContainsIgnoreCase
- `internal/generators/adr_test.go` — 2 testes: NewADR_CreatesFile, NewADR_SlugInFilename

**Resultado:** 14/14 testes passaram. `go test ./internal/validator/... ./internal/generators/... -v` OK.

**Decisoes tecnicas:**
- Fixtures construidas para satisfazer regras irmas e isolar uma violacao por teste (ex: WIPMissingREQ inclui bloco Acceptance Criteria; REQMissingADR inclui Roadmap preenchido)
- MkdirAll de todos os diretorios de estados validos em TestMoveRoadmap_Valid antes da chamada (os.Rename requer destino existente)
- Localizacao de arquivos gerados via filepath.Glob (filename embute time.Now — data do dia)
- Package white-box (sem prefixo de pacote) para acesso direto a containsIgnoreCase e validStates

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** Refatorar `trackfw init` — wizard condicional por tipo de projeto, geração de `CLAUDE.md`, e correção do validate script para Python.

**Entregue:**
- `internal/generators/scaffold.go` — `Config` estendido com `ProjectType` e `ProjectName`; case `python` adicionado em `buildValidateScript`; chamada a `generateClaudeMD(cfg)` adicionada ao final de `Scaffold()`.
- `internal/generators/claudemd.go` — arquivo novo; `generateClaudeMD(cfg Config) error` gera `CLAUDE.md` com seções de governança, frontend/backend condicionais, pre-commit checklist, git hooks e CI gate; `backendCommands()` mapeia build/test/lint por stack (go, java, node, python).
- `internal/commands/init.go` — wizard reescrito com 4 grupos: Grupo 1 (sempre, nome + tipo), Grupo 2 (frontend+pkgmanager, hidden se backend/governance), Grupo 3 (backend, hidden se frontend/governance), Grupo 4 (sempre, hooks+ci).
- `go build ./...` — sem erros.
- `go vet ./...` — sem erros.
- `go test ./internal/validator/... ./internal/generators/... -v` — 14/14 testes passando.

**Observação:** projetos `backend=node` em modo `backend-only` não recebem pergunta sobre `pkgManager` (fica em `""`). A função `backendCommands` faz fallback para `npm` nesses casos — comportamento documentado e alinhado ao spec.

---

## Sessão 2026-06-11 — Apolo (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap `roadmap-adr-wizard-e-list-2026-06-11` — wizard interativo `adr new` + subcomando `adr list`.

**Entregue:**
- `internal/generators/adr.go` — struct `ADRContent{Title, Context, Decision, Consequences, Alternatives}`; `NewADR(ADRContent)` puro (sem I/O de UI); campos preenchidos inseridos diretamente, campos vazios mantêm placeholder HTML; nova função `ListADRs(dir)` (glob + print tabular); `parseADRMeta` extrai título e status do markdown.
- `internal/commands/adr.go` — `newADRNewCmd()` detecta TTY via `charmbracelet/x/term.IsTerminal`; wizard huh 4 campos em TTY, fallback silencioso em CI/não-TTY; `newADRListCmd()` registrado no grupo `adr`.
- `internal/generators/adr_test.go` — 7 testes: `CreatesFile`, `SlugInFilename`, `WithContent`, `EmptyFields`, `ListADRs_Empty`, `ListADRs_WithFiles`, `ListADRs_ParsesMeta`.
- `go build ./...` sem erros | `go vet ./...` limpo | 20/20 testes verdes.
- Commit `e4a69d8` na branch `feat/adr-wizard-e-list` | push para remoto.

**Decisões técnicas:**
- Usado `charmbracelet/x/term` (já no go.mod) ao invés de `golang.org/x/term` — evita nova dependência.
- `ListADRs` e `parseADRMeta` ficam em `generators` para permitir teste direto sem cobra.
- Wizard só ativa em TTY — em CI o comando ainda funciona gerando ADR com placeholders.

---

## Sessão 2026-06-11 — Apolo (CONCLUIDO)

**Tarefa:** ML-1A do roadmap `roadmap-req-wizard-e-list-2026-06-11` — wizard interativo `req new` + subcomando `req list`.

**Entregue:**
- `internal/generators/req.go` — struct `REQContent{Title, Motivation, Criteria, LinkedADR, LinkedRoadmap}`; `NewREQ(REQContent)` puro sem I/O de UI; campos preenchidos inseridos diretamente, campos vazios mantêm placeholder HTML/markdown; `ListREQs(dir)` (glob + print tabular); `parseREQMeta` extrai título e status do markdown.
- `internal/commands/req.go` — `newReqNewCmd()` detecta TTY via `charmbracelet/x/term.IsTerminal`; wizard huh 4 campos em TTY (Motivation, Criteria, LinkedADR, LinkedRoadmap), fallback silencioso em CI/não-TTY; `newReqListCmd()` registrado no grupo `req`.
- `internal/generators/req_test.go` — 7 testes: `CreatesFile`, `SlugInFilename`, `WithContent`, `EmptyFields`, `ListREQs_Empty`, `ListREQs_WithFiles`, `ListREQs_ParsesMeta`.
- `go build ./...` sem erros | `go vet ./...` limpo | 26/26 testes verdes.
- Commit `0db0864` na branch `feat/req-wizard-e-list` | push para remoto.

---

---

## Sessão 2026-06-11 — Apolo (CONCLUIDO)

**Tarefa:** Implementar geração de roadmap por IA no `trackfw roadmap new` (branch `feat/roadmap-ai-generation`).

**Entregue:**
- `internal/ai/` — Client interface, AnthropicClient (SDK v1.50.1 — API v1.x sem `anthropic.F()`), OpenAIClient (stdlib), FakeClient, ReadConfig (parser YAML simples sem dependência de yaml.v3)
- `internal/generators/roadmap.go` — struct RoadmapContent + NewRoadmapFromContent; NewRoadmap refatorado para delegar
- `internal/commands/roadmap.go` — reescrito: wizard huh.Select lista docs/req/*.md, lê conteúdo da REQ, chama IA se configurada, fallback template vazio
- `internal/generators/scaffold.go` — Config.AIProvider/AIApiKey; writeTrackfwConfig escreve ai_provider/ai_model/ai_api_key
- `internal/commands/init.go` — Grupo 5 no wizard (provider + api key)
- Commit `7656a4b` | push para `feat/roadmap-ai-generation`

**Resultado:** 29/29 testes verdes | `go build ./...` limpo | `go vet ./...` limpo

**Decisoes tecnicas:**
- SDK Anthropic v1.50.1: `Messages []MessageParam` (sem wrapper F()), `NewUserMessage(NewTextBlock(prompt))` como helper, `msg.Content[0].Text` para acessar texto
- OpenAI implementado com stdlib pura (sem dependência adicional)
- ai_model: escrita sem valor no YAML (campo livre editável manualmente) — sem verb Sprintf para evitar corrupção silenciosa

---

## Sessão 2026-06-11 — Zeus + Apolo (CONCLUÍDO)

**Tarefa:** Geração de roadmap por IA — `trackfw roadmap new` com wizard interativo + integração Anthropic/OpenAI + fallback template vazio.

**Entregue:**
- `internal/ai/client.go` — interface `Client{Generate}` + factory `NewClient(provider, model, apiKey)`
- `internal/ai/anthropic.go` — struct `anthropicClient` via `github.com/anthropics/anthropic-sdk-go` v1.50.1
- `internal/ai/openai.go` — struct `openAIClient` via stdlib `net/http` + `encoding/json`
- `internal/ai/fake.go` — `FakeClient{Response string}` para testes
- `internal/ai/config.go` — `ReadConfig(path)` lê `ai_provider`, `ai_model`, `ai_api_key` de YAML flat sem yaml.v3
- `internal/ai/client_test.go` — 3 testes: `ReadConfig_Empty`, `ReadConfig_WithValues`, `FakeClient_Generate`
- `internal/generators/roadmap.go` — `RoadmapContent{Title, REQPath, Body}` + `NewRoadmapFromContent`; `NewRoadmap` refatorado para delegar
- `internal/generators/roadmap_test.go` — 2 novos testes: `NewRoadmapFromContent_CreatesFile`, `NewRoadmapFromContent_EmptyBody`
- `internal/generators/scaffold.go` — `Config.AIProvider`, `Config.AIApiKey`; `writeTrackfwConfig` gera `ai_provider`/`ai_model`/`ai_api_key` no YAML
- `internal/commands/init.go` — Grupo 5 AI no wizard (`huh.Select` provider + `huh.Input` api key)
- `internal/commands/roadmap.go` — reescrito: `huh.Select` lista `docs/req/*.md`, lê config AI, chama IA ou fallback template, `Args: cobra.MaximumNArgs(1)`
- `go.mod` — `github.com/anthropics/anthropic-sdk-go v1.50.1` adicionado

**Resultado:** 29/29 testes verdes | `go build ./...` limpo | `go vet ./...` limpo | commit `7656a4b` na branch `feat/roadmap-ai-generation`

---

## Sessão 2026-06-11 — Apolo (CONCLUIDO)

**Tarefa:** Reverter integração de IA do binário Go — remover `internal/ai/`, campos AI do Config, Grupo 5 do wizard init, lógica AI do roadmap.go, e rodar `go mod tidy`.

**Entregue:**
- `internal/ai/` deletado integralmente (6 arquivos: client.go, anthropic.go, openai.go, fake.go, config.go, client_test.go)
- `internal/generators/scaffold.go` — campos `AIProvider`/`AIApiKey` removidos de `Config`; template `writeTrackfwConfig` sem `ai_provider`/`ai_model`/`ai_api_key`
- `internal/commands/init.go` — variáveis `aiProvider`/`aiApiKey` e Grupo 5 do wizard removidos; struct `cfg` sem campos AI
- `internal/commands/roadmap.go` — reescrito: sem imports `context`/`time`/`ai`, sem `roadmapPromptTemplate`, sem lógica AI; `roadmap new` simplificado com `body: ""` implícito via `RoadmapContent` sem campo Body
- `go mod tidy` — `github.com/anthropics/anthropic-sdk-go` e deps transitivas removidos
- 26/26 testes verdes | `go build ./...` limpo | `go vet ./...` limpo
- Commit `2557fef` | push para `feat/roadmap-ai-generation`

---

## Estrutura atual do projeto

```
trackfw/
├── cmd/trackfw/main.go
├── internal/
│   ├── commands/        # init, adr, req, roadmap, status, validate
│   ├── generators/      # scaffold, adr, req, roadmap
│   └── validator/       # validate + status
├── docs/
│   ├── visao-projeto/VISION.md
│   └── agents-working-context.md  ← este arquivo
├── scripts/install.sh
├── Makefile
├── go.mod               # module github.com/trackfw/trackfw
├── go.sum
└── .gitignore
```

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap `feat/req-driven-adr-discovery` — catálogo de probes e detecção de domínio.

**Entregue:**
- `internal/generators/probes.go` — tipos `Probe`, `Question`, `ProbeOption`; `ProbesCatalog` com 6 domínios (authentication, ui, persistence, api, deploy, events); `DetectDomains(intention string) []Probe` — busca case-insensitive por substring nos keywords.
- `internal/generators/probes_test.go` — 5 testes: `Authentication`, `UI`, `NoMatch`, `MultiDomain`, `CaseInsensitive`.
- `go build ./...` limpo | 5/5 testes verdes | commit `2cb3976` | push para `feat/req-driven-adr-discovery`.

---

## Sessão 2026-06-11 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-1A — Criar templates Gemini CLI: GEMINI.md global, GEMINI-project.md, 10 SKILL.md (architect/backend/frontend/qa/infra/security/code-quality/dba/ux/data), 3 commands TOML (trackfw-adr/req/roadmap).

---

## Sessão 2026-06-11 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-1C — Criar templates GitHub Copilot: 1 `copilot-instructions.md` consolidado + 10 `.instructions.md` + 10 `.prompt.md` = 21 arquivos.

---

## Sessão 2026-06-11 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-1D e ML-1E — Criar templates Windsurf (10 rules + 3 workflows + 1 global_rules_append = 14 arquivos) e Amazon Q (10 rules = 10 arquivos). Total: 24 arquivos.

**Destino:** `internal/generators/templates/copilot/`

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-1B do roadmap `feat/req-driven-adr-discovery` — Adicionar `NewADRDraft` em `internal/generators/adr.go`.

**O que foi feito:**
- Adicionadas funções `slugToTitle` e `NewADRDraft` ao final de `internal/generators/adr.go`
- `NewADRDraft` cria ADR com `Status: Draft`, é idempotente via glob por slug, e deriva o título do slug via title case
- Adicionados 4 testes em `internal/generators/adr_test.go`: `TestNewADRDraft_CriaArquivo`, `TestNewADRDraft_StatusDraft`, `TestNewADRDraft_Idempotente`, `TestNewADRDraft_TituloDerivado`
- Build e testes passando: `go build ./...` ok, 4/4 testes verdes
- Commit `7510a64` pushado para branch `feat/req-driven-adr-discovery`

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-2A do roadmap `feat/req-driven-adr-discovery` — Estender `REQContent` com `DependsOnADRs []string` e gerar seção "Blocked by ADRs" no arquivo REQ.

**Entregue:**
- `internal/generators/req.go` — campo `DependsOnADRs []string` adicionado em `REQContent`; `NewREQ` gera cabeçalho com `| Blocked by ADRs: N` quando há ADRs vinculados; nova seção `## Blocked by ADRs` inserida entre `Linked ADR` e `Linked Roadmap`; `parseREQMeta` corrigido para extrair status antes do próximo pipe (evita capturar "Blocked by ADRs: 2" como parte do status).
- `internal/generators/req_test.go` — 3 novos testes: `TestNewREQ_ComADRsVinculados`, `TestNewREQ_SemADRsVinculados`, `TestNewREQ_ContadorNoStatus`.
- `go build ./...` limpo | 10/10 testes `TestNewREQ` verdes | suite completa OK.
- Commit `7e2a069` | push para `feat/req-driven-adr-discovery`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-2B do roadmap `feat/req-driven-adr-discovery` — Wizard `req new` com etapa de probes contextuais.

**Entregue:**
- `internal/commands/req.go` — `runReqNew` refatorado com dois forms em sequência:
  - Form 1: coleta `Title` + `Motivation` em grupo único.
  - Detecção automática via `generators.DetectDomains(title + motivation)`.
  - Form 2: grupos de `Criteria`, `LinkedADR`/`LinkedRoadmap` + um `huh.Select` por question de cada probe detectada.
  - Respostas processadas: ADRSlug não-vazio gera ADR Draft via `generators.NewADRDraft`; resultado salvo em `content.DependsOnADRs` (deduplicado via `uniqueStrings`).
  - Mensagem final lista ADR drafts criados e orienta a resolvê-los antes do roadmap.
- Helper `uniqueStrings` adicionado no mesmo arquivo.
- Em modo não-TTY (CI): fluxo direto sem wizard/probes — comportamento inalterado.
- `go build ./...` limpo.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-3A do roadmap `feat/req-driven-adr-discovery` — Adicionar regra de validação em `internal/validator/validator.go` que detecta REQs Open bloqueadas por ADRs com Status: Draft.

**Entregue:**
- `validateREQsNotBlockedByDraftADRs()` — percorre `docs/req/*.md`, filtra REQs com `Status: Open`, extrai ADRs da seção `## Blocked by ADRs` via `parseBlockedADRs()`, verifica `Status: Draft` via `adrIsDraft()`, emite violation `"REQ X is blocked by Draft ADR: Y"`.
- `parseBlockedADRs(path)` — parser de seção markdown: lê de `## Blocked by ADRs` até próximo `##`, extrai basename `.md` de cada linha `- `.
- `adrIsDraft(adrBasename)` — lê `docs/adr/<basename>` e verifica presença de `"Status: Draft"`.
- `blockedREQs()` — retorna `map[string][]string` (req → adrs Draft) para uso em `GetStatus()`.
- Integrada em `Validate()` após `validateSingleWIP()`.
- Integrada em `GetStatus()` com seção "REQs blocked by Draft ADRs" (adicionada externamente antes da conclusão desta sessão).
- 3 testes novos: `TestValidateREQsNotBlockedByDraftADRs_Violação`, `_SemViolação`, `_Retrocompatível`.
- `go build ./...` limpo | todos os testes verdes | commit `36d582b` | push para `feat/req-driven-adr-discovery`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-3B do roadmap `feat/req-driven-adr-discovery` — Adicionar seção `⏳ REQs blocked by Draft ADRs` ao `GetStatus()`.

**Entregue:**
- `internal/validator/validator.go` — função `blockedREQs() (map[string][]string, error)` que reutiliza `parseBlockedADRs` e `adrIsDraft` do ML-3A; seção adicionada em `GetStatus()` entre `❌ Blocked` e `✅ Done`, emitida apenas quando há REQs bloqueadas.
- `internal/validator/validator_test.go` — 2 novos testes: `TestGetStatus_REQsBloqueadas` (verifica presença da seção e do ADR listado) e `TestGetStatus_SemREQsBloqueadas` (verifica ausência quando não há bloqueios). Padrão de fixture igual ao existente (`t.TempDir()` + `chdir`).
- `go build ./...` limpo | 12/12 testes verdes | commit `85b0ba1` | push para `feat/req-driven-adr-discovery`.

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

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Wave 3+4B do roadmap `v1.1.0-i18n-wizard-java` — infraestrutura i18n Go + wiring nos comandos.

**Entregue:**
- `internal/i18n/i18n.go` — pacote i18n com `DetectLocale()` (LANG/LC_ALL/LANGUAGE), `load()` com `sync.Once`, `T(key string, vars ...string) string` (dot-path + interpolação `{{var}}`), `Locale() string`. Embed de 3 arquivos JSON via `//go:embed`.
- `internal/i18n/locales/en-US.json` — traduções em inglês (init, adr, req, roadmap, validate, status, log, plugins, errors).
- `internal/i18n/locales/pt-BR.json` — traduções em português brasileiro.
- `internal/i18n/locales/es-ES.json` — traduções em espanhol.
- `internal/commands/init.go` — `newInitCmd().Short` usa `i18n.T("init.description")`; títulos dos prompts huh via variáveis intermediárias com `i18n.T("init.prompt.*")`; `fmt.Println(i18n.T("init.success"))`.
- `internal/commands/validate.go` — `Short`, mensagens de ok/violations/warnings via `i18n.T()`.
- `internal/commands/log.go` — `Short`, flag `--tail` description, mensagem "No transitions" via `i18n.T()`.
- `go build ./...` limpo | `go test ./...` 100% verde | `LANG=pt_BR.UTF-8 bin/trackfw --help` exibe comandos traduzidos.

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

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Detectar roadmaps em WIP stale (> 7 dias sem modificação) na branch `feat/v1-remaining-features`.

**Entregue:**
- `internal/validator/validator.go` — constante `staleWIPDays = 7`; função `validateStaleWIP()` que usa `filepath.Glob` + `os.Stat` para calcular idade por `ModTime`; integrada em `Validate()` após `validateSingleWIP()`; seção `⚠  Stale WIP` adicionada em `GetStatus()` entre `❌ Blocked` e `⏳ REQs blocked by Draft ADRs`.
- Import `"time"` adicionado.
- `go build ./...` limpo | `go test ./...` 100% verde | `go vet ./...` limpo | commit `406ebcf` na branch `feat/v1-remaining-features`.

---

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

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Implementar `trackfw log` e registro automático de transições de estado (branch `feat/v1-remaining-features`).

**Entregue:**
- `internal/generators/roadmap.go` — `appendTransitionLog(basename, fromState, toState)` grava em `docs/roadmaps/.trackfw-log` no formato `YYYY-MM-DD HH:MM  <basename padded 50>  <from> → <to>`; `MoveRoadmap` extrai `fromState` via `filepath.Base(filepath.Dir(src))` e chama `appendTransitionLog` após `os.Rename` bem-sucedido.
- `internal/commands/log.go` — comando cobra `log` com flag `--tail N` (default 20); lê `.trackfw-log`, seleciona as últimas N linhas e imprime com cabeçalho; mensagem amigável se arquivo inexistente.
- `internal/commands/root.go` — `newLogCmd()` registrado na lista de subcomandos.
- `go build ./...` limpo | testes verdes | `go vet ./...` limpo | commit `138b4e8` na branch `feat/v1-remaining-features`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Implementar sistema de plugins do trackfw (branch `feat/v1-remaining-features`).

**Entregue:**
- `internal/plugins/plugins.go` — pacote novo; `Dir()` retorna `~/.trackfw/plugins`; `List()` lista binários instalados; `Install(repo)` baixa asset das GitHub Releases (formato `user/name[@tag]`, detecta GOOS/GOARCH); `Remove(name)` remove plugin pelo nome.
- `internal/commands/plugins.go` — comando cobra `plugins` com subcomandos `list`, `add` e `remove`; `RunPlugin(name, args)` executa plugin instalado passando stdin/stdout/stderr.
- `internal/commands/root.go` — `newPluginsCmd()` registrado; `rootCmd.Args = cobra.ArbitraryArgs` + `rootCmd.RunE` configurados para dispatch automático de comandos desconhecidos para plugins.
- `go build ./...` limpo | `go test ./...` verde | `go vet ./...` limpo | commit `d201b45` na branch `feat/v1-remaining-features`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Adicionar subcomando `trackfw roadmap show <name>` com busca parcial por nome.

**Entregue:**
- `internal/generators/roadmap.go` — função `ShowRoadmap(name string) error` adicionada: busca via `filepath.Glob` em todos os estados (`docs/roadmaps/*/*name*.md`), exibe cabeçalho com basename e estado em maiúsculas, conteúdo completo do arquivo e path.
- `internal/commands/roadmap.go` — função `newRoadmapShowCmd()` adicionada e registrada em `newRoadmapCmd()`.
- `go build ./...` limpo | `go test ./...` verde | `go vet ./...` limpo.
- Commit `6d4cc19` na branch `feat/v1-remaining-features`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap de reescrita do pacote npm em Node.js puro (branch `feat/npm-nodejs-rewrite`) — Atualizar package.json e entry point.

**Entregue:**
- `npm/package.json` — reescrito: removidos campos `os`/`cpu`, adicionados `main`, `files` com `src/`, `dependencies` (`commander ^12.0.0`, `@inquirer/prompts ^5.0.0`), `engines.node` atualizado para `>=18`.
- `npm/bin/trackfw` — reescrito: sem mais fat-package/spawnSync de binário Go; entry point Node puro que chama `createProgram().parseAsync(process.argv)`.
- `npm/bin/.gitkeep` — removido.
- `npm/src/commands/index.js` — criado: stub commander com `name/description/version`; exporta `createProgram()`.
- `npm/package-lock.json` — gerado via `npm install` (41 pacotes: commander + @inquirer/prompts + transitivos).
- Critério de aceite: `node npm/bin/trackfw --help` imprime usage sem erro. Passou.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-1B do roadmap de reescrita do pacote npm em Node.js puro (branch `feat/npm-nodejs-rewrite`) — Criar estrutura src/ com stubs.

**Entregue:**
- `npm/src/commands/index.js` — reescrito: `createProgram()` lê version do `package.json`, registra 8 subcomandos via `addCommand`, hook `preSubcommand` vazio para futura dispatch de plugins.
- `npm/src/commands/init.js` — stub: `trackfw init` → `TODO: init`.
- `npm/src/commands/adr.js` — stub com subcomandos `new <title>` e `list`.
- `npm/src/commands/req.js` — stub com subcomandos `new <title>` e `list`.
- `npm/src/commands/roadmap.js` — stub com subcomandos `new`, `list`, `show <name>`, `move <name> <state>`.
- `npm/src/commands/validate.js` — stub: `trackfw validate` → `TODO: validate`.
- `npm/src/commands/status.js` — stub: `trackfw status` → `TODO: status`.
- `npm/src/commands/log.js` — stub com flag `--tail <n>` (default 20).
- `npm/src/commands/plugins.js` — stub com subcomandos `list`, `add <repo>`, `remove <name>`.
- `npm/src/generators/{adr,req,roadmap,init}.js` — stubs `module.exports = {}`.
- `npm/src/validator/index.js` — stub `module.exports = {}`.
- Critério de aceite: `node -e "const {createProgram}=require('./npm/src/commands/index.js'); const p=createProgram(); console.log(p.commands.map(c=>c.name()))"` retorna todos os 8 subcomandos. Passou.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-2A do roadmap de reescrita do pacote npm em Node.js puro (branch `feat/npm-nodejs-rewrite`) — Implementar `npm/src/generators/adr.js` e `npm/src/commands/adr.js`.

**Entregue:**
- `npm/src/generators/adr.js` — funções `newADR(content)`, `listADRs(dir)`, `newADRDraft(slug)`, `toSlug(s)` portadas do Go; placeholders HTML idênticos; `newADRDraft` idempotente via regex sobre `readdirSync`; coluna 60 chars no `list`; helper `parseADRStatus` extrai status da linha `| Status: `.
- `npm/src/commands/adr.js` — implementação real (não mais stub); subcomando `new <title>` com wizard `@inquirer/prompts` em TTY + fallback silencioso em não-TTY; subcomando `list` delega para `generators.listADRs('docs/adr')`.
- Critérios de aceite validados manualmente em `/tmp/trackfw-test-node`:
  - `adr list` (diretório vazio) → `No ADRs found in docs/adr` ✅
  - `adr new "Test Decision" < /dev/null` → `created docs/adr/ADR-2026-06-12-test-decision.md` ✅
  - `adr list` (após criação) → linha com arquivo e status `Proposed` em coluna 60 ✅
  - Conteúdo do arquivo com template e placeholders idênticos ao gerador Go ✅

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-2C do roadmap de reescrita npm Node.js — Implementar `npm/src/commands/log.js` com leitura real do `.trackfw-log`.

**Entregue:**
- `npm/src/commands/log.js` — implementação real: lê `docs/roadmaps/.trackfw-log`, filtra linhas vazias, aplica `--tail N` (default 20), imprime cabeçalho + linhas; mensagem amigável se arquivo inexistente.
- Critérios de aceite validados: sem log → "No transitions recorded yet." | com log → cabeçalho + linha impressos | `--version` → "0.1.0".

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-2B do roadmap de reescrita npm Node.js — portar `listREQs`, `listRoadmaps`, `showRoadmap`, `moveRoadmap`, `appendTransitionLog`, `newRoadmap` para Node.js puro + atualizar commands.

**Entregue:**
- `npm/src/generators/req.js` — `listREQs(dir)`: glob `.md`, extrai status da linha `| Status: ...`, padding 60 chars, fallback `No REQs found in <dir>`.
- `npm/src/generators/roadmap.js` — `VALID_STATES`, `listRoadmaps()`, `showRoadmap(name)`, `moveRoadmap(name, state)`, `appendTransitionLog(basename, from, to)`, `newRoadmap(title, reqPath)`, helpers `findRoadmapMatches` e `toSlug`. Zero dependências externas.
- `npm/src/commands/req.js` — `req list` delegando a `listREQs('docs/req')`.
- `npm/src/commands/roadmap.js` — todos os 4 subcomandos (`new`, `list`, `show`, `move`) delegando aos generators.

**Critérios de aceite validados:**
- `roadmap list` vazio → mensagem orientando usuário ✅
- `roadmap list` com arquivo em backlog → lista `[backlog]` ✅
- `roadmap move test wip` → `✓ moved ROADMAP-2026-06-12-test.md → docs/roadmaps/wip` + log gravado ✅
- `roadmap show test` → cabeçalho `── BASENAME ── [WIP] ──────────...` + conteúdo + `Location:` ✅
- `req list` vazio → `No REQs found in docs/req` ✅
- `req list` com arquivo → `REQ-...md                    Open` ✅

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-3A do roadmap de reescrita npm Node.js — Implementar `npm/src/validator/index.js` (porte completo do validador Go) + `npm/src/commands/validate.js` + `npm/src/commands/status.js`.

**Entregue:**
- `npm/src/validator/index.js` — porte completo do `internal/validator/validator.go`: 9 funções de validação + auxiliares `parseBlockedADRs`, `adrIsDraft`, `listDir`, `blockedREQs`, função principal `validate()` e `getStatus()`. Zero dependências externas.
- `npm/src/commands/validate.js` — saída `✓ No violations found.` / listagem de violations e warnings / `process.exit(1)` em violações.
- `npm/src/commands/status.js` — delegando para `getStatus()`.

**Critérios de aceite:** diretório vazio → `✓ No violations found.` ✅ | `status` → seções formatadas ✅ | `node --check` limpo ✅

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-3B do roadmap de reescrita npm Node.js — Portar `newREQ`, `PROBES_CATALOG`, `detectDomains` para `npm/src/generators/req.js` e reescrever wizard `req new` em `npm/src/commands/req.js`.

**Entregue:**
- `npm/src/generators/req.js` — funções `newREQ(content)`, `PROBES_CATALOG` (6 domínios: authentication, ui, persistence, api, deploy, events — porte exato do Go), `detectDomains(intention)` adicionadas sem remover `listREQs`/`parseREQStatus` existentes; helper `toSlug` local; template idêntico ao Go com seção `## Blocked by ADRs`, linha de status com contador `| Blocked by ADRs: N`.
- `npm/src/commands/req.js` — `req new` reescrito com wizard `@inquirer/prompts` em dois passos (TTY) + fallback silencioso (não-TTY); perguntas dinâmicas por probe via `select`; ADR drafts gerados via `adrGenerators.newADRDraft`; deduplicação via `Set`; mensagem final lista ADR drafts criados.
- Critérios de aceite validados:
  - `req new "OAuth login" < /dev/null` → `created docs/req/REQ-2026-06-12-oauth-login.md` com template correto e `Status: Open` ✅
  - `req list` → `REQ-2026-06-12-oauth-login.md   Open` ✅
  - `detectDomains("OAuth login via SSO provider")` → `['authentication']` ✅
  - `newREQ` com `dependsOnADRs` → seção `## Blocked by ADRs` populada + status `| Blocked by ADRs: 2` ✅

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** ML-3C do roadmap de reescrita npm Node.js — Implementar `npm/src/generators/init.js` (scaffold completo) e `npm/src/commands/init.js` (wizard com @inquirer/prompts).

**Entregue:**
- `npm/src/generators/init.js` — `GOV_DIRS` (7 entradas), `scaffold(cfg)`, `writeTrackfwConfig`, `generateValidateScript` + `buildValidateScript` (go/java/node/python + frontend), `generateCIWorkflow` (github-actions/gitlab-ci), `generateGitHooks` (husky/lefthook), `generateClaudeMD` (seções frontend/backend/pre-commit/hooks/CI), `generateClaudeCommands` (7 slash commands idempotentes), stubs `installAgents/Gemini/Cursor/Copilot/Windsurf/AmazonQ` com mensagem orientativa.
- `npm/src/commands/init.js` — wizard completo com `@inquirer/prompts` (input/select/checkbox), guard `!process.stdin.isTTY` com defaults, try/catch para fallback em stdin inesperadamente fechado, dispatch para instaladores de AI tools.
- Critério de aceite validado: `echo "" | node npm/bin/trackfw init` cria os 7 diretórios de governança + trackfw.yaml + scripts/trackfw-validate.sh + CLAUDE.md + .claude/commands/trackfw (7 slash commands). Sintaxe validada com `node --check`.

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Criar artefatos de governança para v1.1.0 — REQ e Roadmap de i18n, wizard init fixes e scaffold Java.

**Entregue:**
- `docs/requisições/claude/REQ-2026-06-12-i18n-wizard-java-scaffold.md` — REQ com motivação (4 melhorias pós validação em ambiente Windows corporativo) e 9 critérios de aceite mensuráveis.
- `docs/roadmaps/claude/backlog/v1.1.0-i18n-wizard-java-2026-06-12.md` — Roadmap com 4 waves, 9 MLs detalhados (Go binary + npm em paridade): Wave 1 (wizard fixes), Wave 2 (Java pom.xml), Wave 3 (i18n infra), Wave 4 (i18n wiring + templates).

---

## Sessão 2026-06-12 — Apolo (CONCLUÍDO)

**Tarefa:** Wave 1+2 do roadmap `v1.1.0-i18n-wizard-java` — adicionar pergunta de framework de backend ao wizard `trackfw init` (Go) e gerar `pom.xml` Spring Boot 3.3 quando backend=java.

**Entregue:**
- `internal/commands/init.go` — variável `backendFramework string` adicionada; title "Backend stack?" renomeado para "Backend language?"; segundo form `frameworkForm` executado após o form principal quando `backend != ""`; opções condicionais por linguagem (go: 4, java: 3, node: 4, python: 3); `cfg.BackendFramework` passado ao Config.
- `internal/generators/scaffold.go` — campo `BackendFramework string` adicionado em `Config`; `writeTrackfwConfig` gera linha `backend_framework: <valor>` no YAML; chamada `GeneratePomXML(cfg)` adicionada ao final de `Scaffold` com guard `cfg.Backend == "java"`.
- `internal/generators/java.go` — arquivo novo; `GeneratePomXML(cfg Config) error` gera `pom.xml` Spring Boot 3.3 / Java 21 com starter-web, starter-actuator e starter-test; reutiliza `toSlug` de `adr.go` (sem redefinição).
- `go build ./...` — sem erros | `make test` — todos os testes verdes.

**Observação:** `toSlug` já existia em `internal/generators/adr.go` — não foi redefinida em `java.go`.

---

## Sessão 2026-06-12 — Afrodite (CONCLUÍDO)

**Tarefa:** Criar infraestrutura i18n para o pacote npm do trackfw (branch `feat/v1.1.0-i18n-wizard-java`).

**Status:** CONCLUIDO

**Entregue:**
- `npm/src/i18n/index.js` — módulo de detecção de locale (LANG/LC_ALL/LANGUAGE + fallback Intl) e função `t(key, vars)` com interpolação `{{var}}`
- `npm/src/i18n/locales/en-US.json` — todas as strings do CLI em inglês
- `npm/src/i18n/locales/pt-BR.json` — tradução completa para português do Brasil
- `npm/src/i18n/locales/es-ES.json` — tradução completa para espanhol
- `npm/src/commands/validate.js` — wired com `t()`
- `npm/src/commands/status.js` — wired com `t()`
- `npm/src/commands/log.js` — wired com `t()`
- `npm/src/commands/roadmap.js` — wired com `t()`
- `npm/src/commands/plugins.js` — wired com `t()`; erros de download/plugin via `t()`
- `npm/src/commands/adr.js` — wired com `t()`; prompts do wizard i18n
- `npm/src/commands/req.js` — wired com `t()`; prompts do wizard i18n
- `npm/src/commands/init.js` — wired com `t()`; todos os prompts e messages do wizard i18n

**Validacao:**
- `node npm/bin/trackfw --help` — strings em EN-US (padrao) OK
- `LANG=pt_BR.UTF-8 node npm/bin/trackfw --help` — strings em PT-BR OK
- `LANG=es_ES.UTF-8 node npm/bin/trackfw --help` — strings em ES-ES OK
- `LANG=pt_BR.UTF-8 node npm/bin/trackfw validate` — "Nenhuma violacao encontrada." OK

---

## Sessão 2026-06-13 — Apolo ML-1A (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap `feat/v2.0-gaps` — implementar `trackfw serve` (servidor HTTP local de visualização ADR→REQ→ROADMAP).

**Arquivos criados/modificados:**
- `internal/server/server.go` (novo) — handlers HTTP, parse de markdown, template HTML
- `internal/commands/serve.go` (novo) — comando cobra serve com flag --port
- `internal/commands/root.go` — newServeCmd() registrado
- `internal/i18n/locales/en-US.json`, `pt-BR.json`, `es-ES.json` — chave serve.description adicionada

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | `go test ./...` verde | `trackfw serve --help` mostra flag --port | `/api/data` retorna JSON válido | HTML com 3 seções (traceability, timeline, kanban) | commit `b0f27b8` | push para `feat/v2.0-gaps`.

---

## Sessão 2026-06-13 — Apolo ML-1B (CONCLUÍDO)

**Tarefa:** ML-1B do roadmap `feat/v2.0-gaps` — implementar `trackfw metrics` (cycle time, throughput e WIP age a partir do `.trackfw-log`).

**Arquivos a criar/modificar:**
- `internal/metrics/metrics.go` (novo) — ParseLog, Filter, Calculate, ExportCSV
- `internal/metrics/metrics_test.go` (novo) — testes unitários
- `internal/commands/metrics.go` (novo) — comando cobra metrics com --since e --export
- `internal/commands/root.go` — newMetricsCmd() registrado
- `internal/i18n/locales/*.json` — chave metrics.* nos 3 locales
- `npm/src/commands/metrics.js` (novo) — porte Node.js puro
- `npm/src/commands/index.js` — registrar command metrics

**Resultado:**
- `go build ./...` limpo | `go vet ./...` limpo | `go test ./internal/metrics/...` 8/8 verde
- `node --check npm/src/commands/metrics.js` OK
- Commit `a2fc979` | push para `feat/v2.0-gaps`
- `trackfw metrics --help` disponível com flags --since e --export

---

## Sessão 2026-06-13 — Apolo ML-2B (CONCLUÍDO)

**Tarefa:** ML-2B do roadmap `feat/v2.0-gaps` — WIP Limit configurável por squad via `trackfw.yaml`.

**Entregue:**
- `internal/generators/scaffold.go` — `Config.WipLimit int` e `Config.WipBySquad bool` adicionados; `writeTrackfwConfig` gera `wip_limit: 1` e `wip_by_squad: false` no YAML (com defaults quando campos zero).
- `internal/generators/roadmap.go` — campo `squad:` adicionado ao template de novo roadmap no frontmatter (após REQ:, vazio para preenchimento manual).
- `internal/validator/validator.go` — `WIPConfig{Limit, BySquad}` + `readWIPConfig()` (parser YAML flat, sem yaml.v3); `parseSquadFromFrontmatter(path)` extrai campo `squad:` do markdown; `validateWIPLimit()` substitui `validateSingleWIP()` — modo global conta todos os WIPs contra o limite, modo squad agrupa por squad e valida por grupo; `GetStatus()` exibe seção `⚙ WIP by Squad` com count e indicador ⚠/✓ quando `wip_by_squad: true`.
- `internal/validator/validator_test.go` — 5 novos testes: `Global_OK`, `Global_Exceed`, `Global_HighLimit`, `BySquad_OK`, `BySquad_Exceed`. Todos os 17 testes do pacote passando.
- `npm/src/validator/index.js` — paridade Node.js: `readWIPConfig()`, `parseSquadFromFrontmatter()`, `validateWIPLimit()` (retorna `{violations, warnings}`); `validate()` usa `validateWIPLimit` no lugar de `validateSingleWIP`; `getStatus()` exibe seção squad quando `bySquad: true`; novos exports adicionados.

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | 17/17 testes verdes | `node --check` OK | commit `0b39e3d` | push para `feat/v2.0-gaps`.

---

## Sessão 2026-06-13 — Apolo ML-2A (CONCLUÍDO)

**Tarefa:** ML-2A do roadmap `feat/v2.0-gaps` — `trackfw init --brownfield` modo lenient de governança.

**Arquivos criados/modificados:**
- `internal/generators/scaffold.go` — campos `BrownfieldMode bool` e `LenientUntil time.Time` adicionados em `Config`; `writeTrackfwConfig` escreve `governance_mode: lenient` e `lenient_until: YYYY-MM-DD` condicionalmente.
- `internal/commands/init.go` — flag `--brownfield` registrada em `newInitCmd()`; import `"time"` adicionado; `cfg.BrownfieldMode=true` e `cfg.LenientUntil=time.Now().AddDate(0,0,30)` quando flag ativa.
- `internal/validator/validator.go` — structs `GovernanceMode`, funções `readGovernanceMode()`, `IsLenient()`, `LenientUntilDate()` (exportadas) adicionadas; `Validate()` move violations para warnings quando `IsLenient()`.
- `internal/commands/validate.go` — imprime `[LENIENT MODE]` + `i18n.T("validate.lenient_mode", "date", until)` quando em modo lenient.
- `internal/i18n/locales/{en-US,pt-BR,es-ES}.json` — chave `validate.lenient_mode` adicionada nos 3 locales.
- `npm/src/generators/init.js` — `writeTrackfwConfig` escreve linhas lenient quando `cfg.brownfieldMode`.
- `npm/src/validator/index.js` — funções `readGovernanceMode()`, `isLenient()`, `lenientUntilDate()` adicionadas; `validate()` move violations para warnings quando lenient; exports atualizados.
- `npm/src/commands/validate.js` — imprime `[LENIENT MODE]` quando em modo lenient.
- `npm/src/i18n/locales/{en-US,pt-BR,es-ES}.json` — chave `validate.lenient_mode` adicionada nos 3 locales.

**Resultado:**
- `go build ./...` limpo | `go vet ./...` limpo | todos os testes verdes
- Teste integração: `trackfw validate` em projeto lenient → `[LENIENT MODE]`, `⚠ violation`, exit 0
- Teste integração: `trackfw validate` em projeto strict → `✗ violation`, exit 1 (inalterado)
- `node --check` limpo nos 3 arquivos npm modificados

---

## Sessão 2026-06-13 — Apolo ML-3A (CONCLUÍDO)

**Tarefa:** ML-3A do roadmap `feat/v2.0-gaps` — Plugin Registry: `trackfw plugins search` e resolução de nomes via registry `kgsaran/trackfw-plugins`.

**Entregue:**
- `internal/plugins/plugins.go` — `RegistryURL`, `RegistryEntry`, `parseRegistryYAML` (parser YAML lista-de-maps linha a linha, sem yaml.v3), `matchesKeyword` (name+description+tags), `Search` (GET registry + filter), `ResolveRepo` (sem `/` → busca no registry; com `/` → retorna direto sem rede); `Install` modificado para chamar `ResolveRepo` antes de baixar.
- `internal/plugins/plugins_test.go` — 6 testes sem rede: `ParseRegistryYAML_Empty`, `ParseRegistryYAML_OneEntry`, `MatchesKeyword_Name`, `MatchesKeyword_Tag`, `MatchesKeyword_NoMatch`, `ResolveRepo_WithSlash`.
- `internal/commands/plugins.go` — subcomando `search <keyword>` registrado; exit 0 em offline (mensagem amigável) e em sem matches.
- `npm/src/commands/plugins.js` — `fetchRegistry`, `parseRegistryYAML`, `matchesKeyword` e subcomando `search` com saída tabular e exit 0 em offline/sem matches.

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | 6/6 testes verdes | `node --check` OK | commit `26275dc` | push para `feat/v2.0-gaps`.

---

## Sessão 2026-06-13 — Apolo ML-3B (CONCLUÍDO)

**Tarefa:** ML-3B do roadmap `feat/v2.0-gaps` — `trackfw sync --to=linear` e `--to=jira`.

**Entregue:**
- `internal/sync/linear.go` — LinearClient: credenciais via trackfw.yaml ou env vars (LINEAR_API_KEY, LINEAR_TEAM_ID); CreateIssue via GraphQL mutation; readConfigField (parser YAML linha a linha sem yaml.v3).
- `internal/sync/jira.go` — JiraClient: credenciais via trackfw.yaml ou env vars (JIRA_BASE_URL, JIRA_EMAIL, JIRA_TOKEN, JIRA_PROJECT); CreateIssue via REST API v3 com Basic Auth (base64 email:token).
- `internal/sync/sync.go` — SyncToLinear, SyncToJira, syncToProvider: percorre docs/req/*.md, pula não-Open e já sincronizados, chama create, injeta campo no frontmatter; helpers extractTitle, extractMotivation, extractField, injectField, isStatusOpen.
- `internal/sync/sync_test.go` — 8 testes sem rede: SkipsNonOpen, SkipsAlreadySynced, InjectsField, ExtractTitle (3 casos), InjectField, InjectField_UpdatesExisting, ReadConfigField, ExtractMotivation. Todos 8/8 verdes.
- `internal/commands/sync.go` — cobra command `sync` com flag `--to` obrigatória; saída tabular REQ/ISSUE; mensagens de erro claras.
- `internal/commands/root.go` — newSyncCmd() registrado.
- `internal/generators/req.go` — campos `| Linear Issue:` e `| Jira Issue:` adicionados no template de REQ.
- `npm/src/commands/sync.js` — paridade Node.js com https stdlib; linearCreateIssue (GraphQL), jiraCreateIssue (REST v3), syncToProvider, syncToLinear, syncToJira; commander command com --to.
- `npm/src/commands/index.js` — sync registrado no createProgram().

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | 8/8 testes sync verdes | suite completa verde | `node --check` OK | commit `dfa58aa` | push para `feat/v2.0-gaps`.

---

## Sessão 2026-06-13 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-4A do roadmap v2.0-gaps — Hook `commit-msg` com validação de REQ em branches feat/fix.

**Branch:** `feat/v2.0-gaps`

**Entregue:**
- `internal/generators/scaffold.go` — campo `RequireReqInCommit bool` em `Config`; função `generateCommitMsgHook` (husky: `.husky/commit-msg`; lefthook: `lefthook.yml` + `.lefthook/commit-msg/trackfw-req-check.sh`); campo `require_req_in_commit` no `trackfw.yaml`
- `internal/commands/init.go` — segundo form condicional pós-wizard perguntando `require_req_in_commit` quando hooks != "none"; campo passado para `Config`
- `internal/generators/commitmsghook_test.go` — 3 testes: `TestGenerateCommitMsgHook_Husky`, `TestGenerateCommitMsgHook_Disabled`, `TestGenerateCommitMsgHook_Lefthook` — todos 3/3 verdes
- i18n locales Go (en-US, pt-BR, es-ES) — chave `init.prompt.require_req_in_commit`
- `npm/src/generators/init.js` — função `generateCommitMsgHook` + chamada em `scaffold()` + campo no `writeTrackfwConfig`
- `npm/src/commands/init.js` — pergunta condicional com `@inquirer/prompts` confirm; `requireReqInCommit` no cfg
- `npm/src/i18n/locales/` — chave `require_req_in_commit` nos 3 locales

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | suite completa verde | `node --check` OK | commit `add41a6` | push para `feat/v2.0-gaps`.

---

## Sessão 2026-06-13 — Apolo Wave 1 feat/v2.3-ai-agent-rail (CONCLUÍDO)

**Tarefa:** Wave 1 do roadmap `trackfw-ai-agent-rail` — ML-1A (frontmatter YAML em templates) e ML-1B (comando `trackfw context`).

**Branch:** `feat/v2.3-ai-agent-rail`

**ML-1A — Frontmatter YAML em templates (Go + npm):**
- `internal/generators/adr.go` — `NewADR()` e `NewADRDraft()` agora geram bloco `---` com `status`/`date`/`author`
- `internal/generators/req.go` — `NewREQ()` agora gera bloco `---` com `status`/`date`/`author`/`adr`/`roadmap`
- `internal/generators/roadmap.go` — template padrão (quando `content.Body == ""`) agora gera bloco `---` com `status`/`date`/`req`/`squad`
- `npm/src/generators/adr.js` — paridade: `newADR()` e `newADRDraft()` com frontmatter
- `npm/src/generators/req.js` — paridade: `newREQ()` com frontmatter
- `npm/src/generators/roadmap.js` — paridade: `newRoadmap()` com frontmatter

**ML-1B — Comando `trackfw context` (Go + npm):**
- `internal/generators/context.go` — `GetContext(format string) error`: coleta ADRs/REQs/Roadmaps via config, chama `validator.Validate()`, computa score (20pts/categoria + 40pts validate limpo), imprime em md ou json; `extractFrontmatterField()` e `extractInlineStatus()` como helpers
- `internal/commands/context.go` — cobra command `context` com flag `--format` (md|json)
- `internal/commands/root.go` — `newContextCmd()` registrado
- `npm/src/commands/context.js` — paridade Node.js puro: mesma lógica de coleta, score e formatação
- `npm/src/commands/index.js` — `require('./context')` registrado

**Resultado:** `go build ./...` limpo | `go test ./...` 100% verde | `node --check` OK em todos os arquivos npm
- Commit `66b5a8f` (ML-1A) | Commit `4f8b504` (ML-1B) | Push para `feat/v2.3-ai-agent-rail`

---

## Sessão 2026-06-13 — Apolo ML-3A (CONCLUÍDO)

**Tarefa:** ML-3A do roadmap `trackfw-ai-agent-rail` — JSON Schema para ADR/REQ/ROADMAP + `validateFrontmatterPresence` em Go e npm.

**Branch:** `feat/v2.3-ai-agent-rail`

**Entregue:**
- `docs/schema/adr.schema.json` — JSON Schema Draft-07; `required: ["status", "date"]`; `status` enum `["Draft","Proposed","Accepted","Deprecated","Superseded"]`; `date` pattern `^[0-9]{4}-[0-9]{2}-[0-9]{2}$`; campos opcionais `author`, `superseded_by`.
- `docs/schema/req.schema.json` — JSON Schema Draft-07; `required: ["status", "date"]`; `status` enum `["Open","Closed","Blocked"]`; campos opcionais `author`, `adr`, `roadmap`.
- `docs/schema/roadmap.schema.json` — JSON Schema Draft-07; `required: ["status", "date"]`; `status` enum `["backlog","wip","blocked","done","abandoned"]`; campos opcionais `req`, `squad`.
- `internal/validator/validator.go` — `extractFrontmatterField(content, field)` + `validateFrontmatterPresence()`: verifica ADRs e REQs sem bloco `---` de frontmatter; registrada em `Validate()` após `validateREQsNotBlockedByDraftADRs`.
- `npm/src/validator/index.js` — `validateFrontmatterPresence()` portada em Node.js puro; integrada em `validate()` e exportada em `module.exports`.

**Resultado:** `go build ./...` limpo | `go test ./...` 100% verde | `node --check npm/src/validator/index.js` OK | commit `f7ab22c` | push para `feat/v2.3-ai-agent-rail`.

---

## Sessão 2026-06-13 — Afrodite (CONCLUIDO)

**Tarefa:** Criar site de documentação VitePress bilíngue pt-BR/en-US + GitHub Actions deploy (branch `feat/v2.4-docs-site`)

**Branch:** `feat/v2.4-docs-site`

**Entregue:**
- `site/package.json` + `site/.gitignore` — configuração base VitePress 1.6.4
- `site/.vitepress/config.mts` — config bilíngue (root=pt-BR, /en=en-US), base=/trackfw/, search local, social links
- `site/index.md` + `site/en/index.md` — landing pages hero com features, instalação e quickstart
- `site/guide/getting-started.md` + `site/en/guide/getting-started.md` — guia completo (init, adr, req, roadmap, status, validate)
- `site/guide/commands.md` + `site/en/guide/commands.md` — referência de todos os comandos com flags e exemplos
- `site/guide/ai-agents.md` + `site/en/guide/ai-agents.md` — integração com Claude Code, Gemini CLI, Cursor, JSON Schema, prompts
- `.github/workflows/deploy-docs.yml` — build + deploy automático no GitHub Pages em push na main

**Resultado:** `npm run build` limpo | 9 HTMLs gerados em `.vitepress/dist/` | commit `d252e92` | push para `feat/v2.4-docs-site`

---

## Sessão 2026-06-13 — Apolo ML-1A Python CLI (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap Python CLI nativo — `config.py` singleton + `__main__` entry point.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/__init__.py` — `__version__ = "2.2.0"`.
- `pypi/trackfw/__main__.py` — entry point `from trackfw.cli import main; main()`.
- `pypi/trackfw/config.py` — funções `defaults()`, `load(cwd=None)`, `reset()`, `_parse(content, cfg)`; singleton `_instance`; parse YAML linha a linha sem dependência externa; constantes `NAMESPACING_FLAT` e `NAMESPACING_BY_AGENT`; paridade exata com `npm/src/config/index.js`.
- `pypi/tests/__init__.py` — vazio (declara pacote de testes).
- `pypi/tests/test_config.py` — 5 testes unittest: `test_defaults_sem_yaml`, `test_le_campos_escalares`, `test_le_adr_dirs`, `test_singleton`, `test_reset`.

**Resultado:** 5/5 testes verdes | commit `633016d` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-1B Python CLI (CONCLUÍDO)

**Tarefa:** ML-1B do roadmap Python CLI nativo — módulo i18n Python com suporte pt-BR/en-US/es-ES.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/i18n/__init__.py` — detecção de locale via `TRACKFW_LANG`/`LANG`/`LANGUAGE`/`LC_ALL`; normalização `pt_BR*→pt-BR`, `es_*→es-ES`, qualquer outro→`en-US`; função `t(key, **vars)` com suporte a chaves aninhadas com `.` e interpolação `{{var}}`; fallback en-US e fallback para a própria chave; cache lazy com `reset()` para testes.
- `pypi/trackfw/i18n/locales/{pt-BR,en-US,es-ES}.json` — copiados de `npm/src/i18n/locales/`
- `pypi/tests/test_i18n.py` — 11 testes unittest: fallback en-US, pt-BR, es-ES, normalização LANG Unix, chave inexistente, chaves aninhadas, interpolação, detecção de locale, fallback de chave ausente.

**Resultado:** 11/11 testes verdes | sintaxe validada com `py_compile` | commit `e3087d1` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo (CONCLUIDO)

**Tarefa:** ML-1C do roadmap Python CLI nativo — `validator.py` com wip-limit, stale-wip, req-adr em paridade com `npm/src/validator/index.js`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/validator.py` — espelho completo do validator JS: list_dir, resolve_wip_dirs, parse_frontmatter, validate_wip_has_req, validate_reqs_have_adr, validate_blocked_has_req, validate_reqs_have_roadmap, validate_adrs_are_referenced, validate_wip_has_acceptance_criteria, validate_wip_limit (flat/by_agent/by_squad), validate_stale_wip, validate_reqs_not_blocked_by_draft_adrs, validate_frontmatter_presence, validate(), modo lenient.
- `pypi/tests/test_validator.py` — 22 testes unittest passando (100%).
- Commit `a2a0407` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-2A Python CLI (CONCLUÍDO)

**Tarefa:** ML-2A do roadmap Python CLI nativo — `generators/__init__.py` + `generators/adr.py` + `tests/test_generators_adr.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/generators/__init__.py` — pacote vazio (declara o subpacote generators).
- `pypi/trackfw/generators/adr.py` — três funções: `next_adr_number(adr_dir)` escaneia ADR-NNN-*.md e retorna max+1; `slugify(title)` via unicodedata NFKD + encode ascii ignore, espaços→hífen, remove não-alfanuméricos; `generate_adr(title, status, adr_dirs, cwd)` cria arquivo ADR com frontmatter YAML e template markdown, numeração sequencial automática.
- `pypi/tests/test_generators_adr.py` — 13 testes unittest: TestNextAdrNumber (4 casos), TestSlugify (5 casos), TestGenerateAdr (4 casos). Todos 13/13 verdes.
- Commit `b9003b6` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-2B Python CLI (CONCLUÍDO)

**Tarefa:** ML-2B do roadmap Python CLI nativo — `generators/req.py` + `tests/test_generators_req.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/generators/req.py` — `slugify(title)` via `unicodedata.NFKD + ascii ignore`; `generate_req(title, req_dir, cwd)` cria `REQ-YYYY-MM-DD-<slug>.md` com frontmatter completo (name, title, status: Open, linked_adr: —, created, author) e seções Motivação, Critérios de Aceite, Fora de Escopo; cria `req_dir` automaticamente via `os.makedirs(exist_ok=True)`; retorna path absoluto.
- `pypi/tests/test_generators_req.py` — 8 testes unittest: `test_generate_req_cria_arquivo`, `test_frontmatter_correto`, `test_slugify_com_acentos`, `test_cria_req_dir_se_nao_existir`, `test_retorna_path_absoluto`, `test_conteudo_template`, `test_slugify_lowercase`, `test_slugify_sem_acentos`.

**Resultado:** 8/8 testes verdes | commit `bf64f67` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-2D Python CLI (CONCLUÍDO)

**Tarefa:** ML-2D do roadmap Python CLI nativo — `generators/init_gen.py` (scaffold flat/by_agent) + `tests/test_generators_init.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/generators/init_gen.py` — espelho de `npm/src/generators/init.js` em Python puro (stdlib apenas): `scaffold(cwd, opts)`, `_gov_dirs_by_agent(agents)`, `_write_trackfw_yaml(cwd, opts)`, `_write_example_adr(cwd, opts)`; constantes `GOV_DIRS_FLAT` e `ROADMAP_STATES`; ADR exemplo idempotente (não sobrescreve se já existir).
- `pypi/tests/test_generators_init.py` — 12 testes unittest distribuídos em 5 classes: `TestScaffoldFlat` (2), `TestScaffoldByAgent` (2), `TestTrackfwYamlGerado` (3), `TestIdempotente` (2), `TestExemploADR` (3).
- Suite completa: 82/82 testes verdes | `py_compile` OK | commit `591d4df` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-2C Python CLI (CONCLUÍDO)

**Tarefa:** ML-2C do roadmap Python CLI nativo — `generators/roadmap.py` + `tests/test_generators_roadmap.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/generators/roadmap.py` — espelho de `npm/src/generators/roadmap.js`: `slugify()`, `generate_roadmap()` (modo flat e by_agent), `move_roadmap()` (busca em todos os estados/agentes, atualiza `status:` no frontmatter, grava `.trackfw-log`); helpers `_state_dir`, `_agent_state_dir`, `_find_roadmap_matches`, `_append_transition_log`, `_roadmap_template`.
- `pypi/tests/test_generators_roadmap.py` — 11 testes unittest: `TestSlugify` (3 casos), `TestGenerateFlat` (gera em `backlog/`), `TestGenerateByAgent` (gera em `zeus/backlog/`, fallback primeiro agente), `TestMoveBacklogParaWip` (move arquivo, atualiza frontmatter, grava log, levanta erros), `TestMoveBuscaEmTodosAgentes` (by_agent sem especificar agente).

**Resultado:** 11/11 testes verdes | commit `3b3d3cb` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-3A Python CLI (CONCLUÍDO)

**Tarefa:** ML-3A do roadmap Python CLI nativo — Wave 3 comandos CLI: `cli.py` (entry point argparse), `commands/adr.py`, `commands/req.py`, `commands/log.py`, `commands/__init__.py`, `tests/test_commands_basic.py` + atualizar `pyproject.toml`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/commands/__init__.py` — declara pacote de subcomandos.
- `pypi/trackfw/cli.py` — entry point argparse com 11 subcomandos: `adr` e `req` e `log` com implementação real; `init`, `roadmap`, `validate`, `status`, `discover`, `metrics`, `context`, `sync`, `plugins` como stubs ("Not implemented yet", exit 0). Flag `--version` via argparse.
- `pypi/trackfw/commands/adr.py` — `register(subparsers)` + `adr new <title> [--status] [--dir]`; chama `generate_adr()`, imprime path criado.
- `pypi/trackfw/commands/req.py` — `register(subparsers)` + `req new [<title>]`; `input()` quando título ausente; chama `generate_req()`, imprime path criado.
- `pypi/trackfw/commands/log.py` — `register(subparsers)` + `log <message>`; append em `.trackfw-log` na raiz do projeto com timestamp `YYYY-MM-DD HH:MM`.
- `pypi/pyproject.toml` — entry point atualizado de `trackfw._cli:main` para `trackfw.cli:main`.
- `pypi/tests/test_commands_basic.py` — 11 testes de integração via `subprocess.run` com `PYTHONPATH=PYPI_DIR`; cobre `--version`, `adr new` (3 variações), `log` (3 variações) e 4 stubs.

**Resultado:** 93/93 testes verdes | commit `1f83956` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-14 — Athena (IMPLEMENTANDO)

**Tarefa:** Análise de mercado aprofundada e completa — trackfw vs. concorrentes em 6 segmentos: ADR Management, Spec/REQ Management, Roadmap, Platform Engineering/IDP, Engineering Metrics/DORA, AI-native Governance. WebSearch ativo para 20+ ferramentas. Entrega do relatório completo em markdown.

**Status:** CONCLUÍDO — relatório completo entregue. Cobertura: 6 segmentos, 25+ ferramentas analisadas via WebSearch. Posicionamento, diferenciadores únicos, gaps, ameaças, oportunidades e 9 recomendações estratégicas.

---

## Sessão 2026-06-13 — Apolo ML-3B Python CLI (CONCLUÍDO)

**Tarefa:** ML-3B do roadmap Python CLI nativo — `commands/validate.py` + `commands/status.py` + `tests/test_commands_validate_status.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/commands/validate.py` — `register(subparsers)` e `run(args)`: chama `validator.validate()`, imprime violations como `✗ <msg>` (vermelho ANSI se terminal suportar), warnings como `⚠ <msg>`, `✓ Governance OK` se tudo limpo; exit code 1 se violations; informa usuario sobre modo lenient.
- `pypi/trackfw/commands/status.py` — `register(subparsers)`, `run(args)`, `get_status(cwd)`: dashboard com contagem de ADRs, REQs (breakdown Open/Closed) e Roadmaps por estado; suporta modo `flat` e `by_agent` (totais agregados + seção "Roadmaps (by agent)" com contagens por agente); helper `_resolve(base, path)` garante paths relativos resolvidos ao `cwd` passado.
- `pypi/tests/test_commands_validate_status.py` — 10 testes unittest (sem subprocess, `tempfile.mkdtemp()`): `TestValidateSemViolations`, `TestValidateComViolation`, `TestValidateLenientExitZero`, `TestStatusFlat` (3 asserts), `TestStatusByAgent` (4 asserts).

**Resultado:** 10/10 testes novos verdes | suite completa 103/103 | commit `7e989a6` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-3C Python CLI (CONCLUÍDO)

**Tarefa:** ML-3C do roadmap Python CLI nativo — `commands/roadmap.py` + `commands/discover.py` + `tests/test_commands_roadmap_discover.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/commands/roadmap.py` — `register(subparsers)` com 4 subcomandos:
  - `roadmap new <title> [--agent]`: chama `generate_roadmap()`, imprime path criado.
  - `roadmap move <filename> <state>`: chama `move_roadmap()`, imprime novo path.
  - `roadmap list [--state]`: lista roadmaps por estado; modo flat agrupa por estado, modo by_agent agrupa por agente.
  - `roadmap show <filename>`: busca por nome exato ou parcial (case-insensitive), imprime conteúdo.
  - Helpers internos: `_list_flat`, `_list_by_agent`, `_find_file`.
- `pypi/trackfw/commands/discover.py` — `register(subparsers)` com flags `--init` e `--bootstrap-log`:
  - `scan(root_dir)`: detecta adr_dirs, req_dir, roadmap_dir, namespacing, agents, counts, score 0-100; espelha `internal/discover/discover.go` e `npm/src/commands/discover.js`.
  - `generate_yaml(result)`: gera conteúdo do trackfw.yaml.
  - `generate_bootstrap_log(result, root_dir)`: entradas retroativas baseadas em mtime dos arquivos em done/.
  - `install_gates(result, root_dir)`: instala validate script, hook entry e CI workflow.
  - `_cmd_discover(args)`: imprime relatório com score e executa --init/--bootstrap-log conforme flags.
- `pypi/tests/test_commands_roadmap_discover.py` — 26 testes unittest:
  - `TestRoadmapNew` (3 casos): flat, by_agent com agent, by_agent sem agent.
  - `TestRoadmapMove` (3 casos): move válido, estado inválido, arquivo não encontrado.
  - `TestRoadmapList` (3 casos): flat, by_agent, filtro por estado.
  - `TestRoadmapShow` (3 casos): exato, parcial, não encontrado.
  - `TestDiscoverScan` (6 casos): flat, by_agent, score 0, score parcial, github-actions, lefthook.
  - `TestDiscoverInit` (2 casos): arquivo criado, conteúdo correto.
  - `TestDiscoverBootstrapLog` (3 casos): flat, by_agent, sem done/.
  - `TestRegister` (3 casos): argparse de roadmap e discover.

**Resultado:** 26/26 testes novos verdes | suite completa 129/129 | commit `2fcbe02` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessão 2026-06-13 — Apolo ML-3D Python CLI (CONCLUÍDO)

**Tarefa:** ML-3D do roadmap Python CLI nativo — Wave 3 comandos extras: `commands/metrics.py`, `commands/context.py`, `commands/sync.py`, `commands/plugins.py`, `tests/test_commands_extras.py`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Entregue:**
- `pypi/trackfw/commands/metrics.py` — `register(subparsers)` com flags `--days`, `--since`, `--export`; `_parse_log()` via regex LINE_RE (espelha JS); `_calculate()` (cycle time médio, throughput por semana, WIP age); `_print_metrics()` (tabela ASCII); `_export_csv()`; `_filter()` por datetime; `_format_duration()`.
- `pypi/trackfw/commands/context.py` — `register(subparsers)` com flags `--format` e `--output`; `_get_context()` coleta ADRs/REQs/Roadmaps via config, chama `validator.validate()`, computa score, saída em markdown ou JSON; suporte a `--output FILE`.
- `pypi/trackfw/commands/sync.py` — `register(subparsers)` com flag `--to` obrigatória (linear|jira); `_sync_to_linear()` e `_sync_to_jira()` via `urllib.request` (stdlib pura); helpers `_read_config_field`, `_extract_title`, `_extract_motivation`, `_inject_field`, `_is_status_open`; `_sync_to_provider()` percorre `docs/req/*.md`, pula não-Open e já sincronizados; saída tabular REQ/ISSUE.
- `pypi/trackfw/commands/plugins.py` — `register(subparsers)` com sub-subcomandos `list` e `run`; `_find_plugins_in_path()` busca executáveis `trackfw-*` no PATH via `os.listdir` + `os.access`; `_cmd_run()` executa via `subprocess.run()`, repassa args e exit code.
- `pypi/tests/test_commands_extras.py` — 17 testes unittest: TestMetrics (6), TestContext (6), TestPlugins (5). Todos 17/17 verdes.

**Resultado:** 17/17 testes verdes | suite completa 146/146 | commit `09b54c5` | push para `feat/v2.2-python-cli-nativo`.

---

## Sessao 2026-06-13 — Artemis ML-4A Python CLI QA (CONCLUIDO)

**Tarefa:** ML-4A do roadmap Python CLI nativo — auditoria e validacao da suite de testes Python completa.

**Branch:** `feat/v2.2-python-cli-nativo`

**Resultado da auditoria:**
- **146/146 testes verdes** (0 failures, 0 errors)
- Suite completa em 0.688s
- Working tree limpo — todos os testes ja estavam commitados junto com cada ML de implementacao
- Nenhum teste faz chamada de rede (urllib/requests/http/socket ausentes nos arquivos de teste)
- Nenhum arquivo temporario deixado em `pypi/` apos execucao
- Cobertura verificada: config sem trackfw.yaml (test_defaults_sem_yaml), modo lenient (test_lenient_mode_violations_viram_warnings, test_validate_lenient_violations_viram_warnings), roadmap move (test_roadmap_move, test_roadmap_move_estado_invalido, test_roadmap_move_arquivo_nao_encontrado)
- Total >= 100 testes: 146 (criterio atendido com folga)

**Distribuicao por arquivo:**
- test_config.py: 5 | test_i18n.py: 11 | test_validator.py: 22
- test_generators_adr.py: 13 | test_generators_req.py: 8 | test_generators_roadmap.py: 11
- test_generators_init.py: 12 | test_commands_basic.py: 11
- test_commands_validate_status.py: 10 | test_commands_roadmap_discover.py: 26
- test_commands_extras.py: 17

**Agente:** Artemis | Status: CONCLUIDO

---

## Sessão 2026-06-13 — Zeus ML-4B + Fechamento v2.2 Python CLI (CONCLUÍDO)

**Tarefa:** ML-4B (remoção do wrapper `_cli.py`) + fechamento do roadmap v2.2.

**Branch:** `feat/v2.2-python-cli-nativo`

**ML-4B resultado:**
- `pypi/trackfw/_cli.py` (wrapper Go binary) removido
- Nenhuma referência residual a `_cli` nos arquivos Python/TOML
- `pip install -e pypi/` sem warnings
- `trackfw --version` → `trackfw 2.2.0`
- `python3 -m trackfw --help` funcional
- Commit `b2121dd` | push OK

**Fechamento do roadmap:**
- Roadmap movido de `wip/` para `done/`
- Todos os 11 MLs marcados ✅ Concluído
- Total: 146 testes, 12 comandos, zero dependências externas, Python 3.8+

**Próximos passos:** criar PR para `feat/v2.2-python-cli-nativo` → `main` e gerar tag v2.2.0 após merge.

**Agente:** Zeus | Status: CONCLUÍDO

---

## Sessão 2026-06-13 — Apolo ML-1A v2.3 Validator Improvements (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap v2.3 — melhorias no validador Go do trackfw (5 mudanças).

**Branch:** `feat/v2.3-validator-improvements`

**Entregue:**

B1 — adr_dirs recursivo:
- `walkADRFiles(adrDir)` — WalkDir recursivo, retorna basenames de todos `.md`.
- `findADRFile(basename, adrDirs)` — busca o caminho completo recursivamente; usa `fs.SkipAll` ao encontrar.
- `validateADRsAreReferenced`, `validateFrontmatterPresence` e `adrIsDraft` migrados para busca recursiva.

B2 — stale WIP por git log:
- `gitLastModifiedTime(path)` — `git log -1 --format=%ct` com fallback para mtime do filesystem.
- `validateStaleWIP()` — usa timestamp do último commit quando disponível.

M3 — verificar existência de referências:
- `extractRefPath(content, field)` — extrai caminho `.md`; ignora vazios/traços.
- `validateRefTargetsExist()` — warnings para REQ:/ADR:/Roadmap: que não existem no filesystem.

M4 — coerência pasta × status:
- `validateFolderStatusCoherence()` — warning quando frontmatter `status:` diverge da pasta (flat e by_agent).

M5 — unicidade de filename entre estados:
- `validateFilenameUniqueness()` — violation quando mesmo filename aparece em múltiplos estados.

Testes (7 novos em `internal/validator/validator_improvements_test.go`):
- TestWalkADRFiles, TestADRDirsRecursiveInValidate, TestValidateStaleWIPFallback
- TestExtractRefPath (7 sub-casos), TestRefTargetsExistWarning, TestFolderStatusCoherence, TestFilenameUniqueness

**Resultado:** `go build ./...` limpo | 24/24 testes verdes | commit `a3a3697` | push para `feat/v2.3-validator-improvements`

---

## Sessão 2026-06-13 — Apolo ML-1B validator-improvements (CONCLUÍDO)

**Tarefa:** ML-1B do roadmap `feat/v2.3-validator-improvements` — Melhorias no validador Node.js.

**Branch:** `feat/v2.3-validator-improvements`

**Entregue:**
- `npm/src/validator/index.js` — walkDirMd, findAdrFile, gitLastModifiedTime adicionados; adrIsDraft, validateADRsAreReferenced, validateFrontmatterPresence e validateStaleWIP atualizados para busca recursiva; extractRefPath, validateRefTargetsExist, validateFolderStatusCoherence, validateFilenameUniqueness + FOLDER_TO_STATUS implementados; validate() inclui novas validações; module.exports expandido.
- `npm/tests/validator.test.js` — criado: 12/12 testes passando (sem framework externo).

**Resultado:** `node --check` OK | 12/12 testes verdes | `validate()` OK | commit `c1b236b` | push para `feat/v2.3-validator-improvements`.

**Agente:** Apolo | Status: CONCLUÍDO

---

## Sessão 2026-06-13 — Apolo ML-1C validator-improvements Python (IMPLEMENTANDO)

**Tarefa:** ML-1C do roadmap `feat/v2.3-validator-improvements` — Melhorias no validador Python (`pypi/trackfw/validator.py`).

**Branch:** `feat/v2.3-validator-improvements`

**Mudanças a implementar:**
- B1: `_walk_dir_md` + `_find_adr_file` (ADR dirs recursivo)
- B2: `_git_last_modified_time` + `subprocess` (stale WIP por git log)
- M3: `_extract_ref_path` + `validate_ref_targets_exist` (verificar existência de referências)
- M4: `_FOLDER_TO_STATUS` + `validate_folder_status_coherence` (coerência pasta×status)
- M5: `validate_filename_uniqueness` (unicidade de filename entre estados)
- Novos testes: classe `TestValidatorImprovements` em `pypi/tests/test_validator.py`

**Entregue:**
- `pypi/trackfw/validator.py` — `import subprocess` adicionado; `_walk_dir_md`, `_find_adr_file`, `_git_last_modified_time`, `_extract_ref_path` adicionados; `_adr_is_draft` usa `_find_adr_file`; `validate_adrs_are_referenced` usa `_walk_dir_md`; `validate_frontmatter_presence` usa `_walk_dir_md` + `_find_adr_file`; `validate_stale_wip` usa `_git_last_modified_time` com fallback para `st_mtime`; `validate_ref_targets_exist`, `_FOLDER_TO_STATUS`, `validate_folder_status_coherence`, `validate_filename_uniqueness` implementados; `validate()` inclui novas validações.
- `pypi/tests/test_validator.py` — classe `TestValidatorImprovements` com 11 novos testes adicionada.

**Resultado:** 157/157 testes verdes (11 novos) | commit `12d1009` | push para `feat/v2.3-validator-improvements`

**Agente:** Apolo | Status: CONCLUÍDO

---

## Sessão 2026-06-13 — Backend (config evolution ML-1A)

**Agente:** Backend | Status: CONCLUIDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-1A — estender `internal/config/config.go` com novos campos (`LinkFieldsReq`, `LinkFieldsADR`, `LinkFieldsRoadmap`, `AcceptanceMarkers`, `Rules`) e parser de blocos aninhados de 1 nível. Criar `internal/config/config_evolution_test.go` com 6 testes cobrindo defaults, parsing e retrocompatibilidade.

**Entregue:**
- `internal/config/config.go` — struct `ProjectConfig` estendida com 5 novos campos v2.4; `defaults()` atualizado com defaults para todos; `parse()` reescrito com suporte a blocos aninhados de 1 nível (link_fields com sub-chaves req/adr/roadmap, acceptance_markers como lista, rules como mapa chave/valor).
- `internal/config/config_evolution_test.go` — 6 testes: `TestConfigDefaults_NewFields`, `TestConfigLinkFields`, `TestConfigAcceptanceMarkers`, `TestConfigRules`, `TestConfigSparse_NewFields`, `TestConfigRetrocompat`.

**Resultado:** 12/12 testes verdes em `internal/config` | `go build ./...` verde | commit `c676d45` | push para `feat/v2.4-config-evolution`

**Obs:** `TestMoveRoadmap_ByAgent` em `internal/generators` falha — pré-existente no commit `84eeff0`, fora do escopo do ML-1A.

---

## Sessão 2026-06-13 — Backend (config evolution ML-1B npm)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-1B — estender `npm/src/config/index.js` com novos campos (`linkFields`, `acceptanceMarkers`, `rules`) e parser de blocos aninhados de 1 nível. Criar `npm/tests/config.test.js` com 6 testes.

**Entregue:**
- `npm/src/config/index.js` — `defaults()` estendida com `linkFields` (req/adr/roadmap), `acceptanceMarkers` e `rules` (9 regras com severidade); `parse()` reescrita com estados `inLinkFields`/`inAcceptanceMarkers`/`inRules` e função `flushBlocks()` para flush ao mudar de bloco ou no EOF; parser distingue indent via `rawLine[0]` (espaço/tab); sub-chaves de `link_fields` (req/adr/roadmap) resolvidas por nome.
- `npm/tests/config.test.js` — 6 testes sem framework externo (assert nativo): defaults, link_fields customizado, acceptance_markers customizado, rules parcial com merge, sparse, retrocompatibilidade v2.3.

**Resultado:** 6/6 testes `config.test.js` verdes | 12/12 testes `validator.test.js` inalterados | commit `84eeff0` | push para `feat/v2.4-config-evolution`.

---

## Sessão 2026-06-13 — Backend (config evolution ML-1C Python)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-1C — estender `pypi/trackfw/config.py` com novos campos (`link_fields`, `acceptance_markers`, `rules`) e parser de blocos aninhados de 1 nível. Adicionar classe `TestConfigEvolution` em `pypi/tests/test_config.py` com 6 novos testes.

**Entregue:**
- `pypi/trackfw/config.py` — `defaults()` estendida com `link_fields` (req/adr/roadmap), `acceptance_markers` e `rules` (9 regras); `_parse()` reescrita com suporte a blocos aninhados: detecta indentação via `raw_line[0]`, aceita itens de lista com e sem indentação (compatibilidade com yamls existentes onde `- item` vem sem indent após a chave), função interna `flush_blocks()` com `nonlocal` para flush ao trocar de bloco ou no EOF; sub-chaves de `link_fields` resolvidas por nome.
- `pypi/tests/test_config.py` — classe `TestConfigEvolution` com 6 testes: `test_defaults_novos_campos`, `test_link_fields_customizado`, `test_acceptance_markers_customizado`, `test_rules_parcial_merge_com_defaults`, `test_sparse_novos_campos_usam_defaults`, `test_retrocompat_yaml_v23`.

**Decisão técnica:** o parser original aceitava itens de lista sem indentação (`- zeus` direto após `agents:`) — a nova implementação preserva esse comportamento detectando `line.startswith("- ")` independente do `raw_line[0]`, garantindo retrocompatibilidade total com yamls v2.3.

**Resultado:** 163/163 testes verdes (6 novos) | commit `201e748` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (config evolution ML-2A validator)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-2A — fazer o validator Go consumir os novos campos de config (`LinkFieldsReq`, `LinkFieldsADR`, `LinkFieldsRoadmap`, `AcceptanceMarkers`, `Rules`) em vez de strings hardcoded. F2 (field mapping) + F3 (severity per rule).

**Entregue:**
- `internal/validator/validator.go` — helper `contentHasMarker` substitui todas as comparações hardcoded `strings.Contains(content, "REQ:")` por loops sobre `cfg.LinkFieldsReq/ADR/Roadmap` e `cfg.AcceptanceMarkers`; helpers `ruleSeverity` e `applyRule` adicionados; `Validate()` refatorada para usar `applyRule` em todas as regras configuráveis (wip_has_req, adr_orphan, wip_acceptance, wip_limit, stale_wip, blocked_by_draft_adr, ref_targets_exist, folder_status, filename_uniqueness); regras sem entrada em `Rules` (validateREQsHaveADR, validateBlockedHasREQ, validateREQsHaveRoadmap, validateFrontmatterPresence) mantêm append direto em violations.
- `internal/validator/validator_evolution_test.go` — 4 testes: `TestFieldMapping_ReqId_SatisfiesWipHasREQ`, `TestRuleSeverity_Off_AdrOrphan`, `TestRuleSeverity_Warning_WipHasReq`, `TestAcceptanceMarkersCustom`.

**Resultado:** go build ./... verde | 4/4 novos testes verdes | todos os testes anteriores mantidos verdes | commit `0b0e47a` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (config evolution ML-2B Node.js)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-2B — fazer o validator Node.js (`npm/src/validator/index.js`) consumir os novos campos de config (`linkFields`, `acceptanceMarkers`, `rules`) em vez de strings hardcoded.

**Entregue:**
- `npm/src/validator/index.js` — adicionado `contentHasMarker(content, markers)` que substitui checks hardcoded de `'REQ:'`/`'ADR:'`/`'Roadmap:'` por loops sobre `cfg.linkFields.*`; adicionado `ruleSeverity(name)` e `applyRule(ruleName, msgs, violations, warnings)` para rotear msgs conforme `cfg.rules[name]` (error→violations, warning→warnings, off→descarta); função `validate()` refatorada usando `applyRule` para 9 regras configuráveis; regras sem configuração de severidade (validateREQsHaveADR, validateBlockedHasREQ, validateREQsHaveRoadmap, validateFrontmatterPresence) mantidas como violations diretas; `contentHasMarker`, `ruleSeverity`, `applyRule` exportadas no `module.exports`.
- `npm/tests/validator.test.js` — 4 novos testes: field mapping `req_id` satisfaz `wip_has_req`, severity `off` suprime `adr_orphan`, severity `warning` roteia `wip_has_req` para warnings, `acceptance_markers` customizado satisfaz verificação.

**Decisão técnica:** os testes de severity chamam diretamente `applyRule` + a sub-função de validação em vez de chamar `validate()` completo — evita efeitos colaterais de outras regras no ambiente de teste isolado.

**Resultado:** 16/16 testes `validator.test.js` verdes (12 existentes + 4 novos) | comportamento default idêntico à v2.3 | commit `6ed3ed5` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (config evolution ML-2C Python)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-2C — fazer o validator Python (`pypi/trackfw/validator.py`) consumir os novos campos de config (`link_fields`, `acceptance_markers`, `rules`) em vez de strings hardcoded (F2 field mapping + F3 severity per rule).

**Entregue:**
- `pypi/trackfw/validator.py` — adicionado `_content_has_marker(content, markers)` que substitui checks hardcoded de `"REQ:"`/`"ADR:"`/`"Roadmap:"` em `validate_wip_has_req`, `validate_reqs_have_adr`, `validate_blocked_has_req`, `validate_reqs_have_roadmap` por loops sobre `cfg["link_fields"][*]`; `validate_wip_has_acceptance_criteria` refatorado para usar `cfg["acceptance_markers"]` substituindo os 4 checks hardcoded; adicionado `_rule_severity(name, cfg)` e `_apply_rule(rule_name, msgs, violations, warnings, cfg)` para rotear msgs conforme `cfg["rules"]`; função `validate()` refatorada usando `_apply_rule` para 8 regras configuráveis (wip_has_req, adr_orphan, wip_acceptance, blocked_by_draft_adr, filename_uniqueness, ref_targets_exist, folder_status, stale_wip, wip_limit); regras sem configuração de severidade (validate_reqs_have_adr, validate_blocked_has_req, validate_reqs_have_roadmap, validate_frontmatter_presence) mantidas como violations diretas.
- `pypi/tests/test_validator.py` — nova classe `TestValidatorEvolution` com 4 testes: field mapping `req_id` satisfaz `wip_has_req`, severity `off` suprime `adr_orphan`, severity `warning` roteia `wip_has_req` para warnings, `acceptance_markers` customizado `## Done When` satisfaz verificação.

**Decisão técnica:** violations/warnings no Python validator são dicts `{"type": "...", "message": "..."}` (não strings simples) — `_apply_rule` e `_violations_messages` no teste tratam ambos os formatos.

**Resultado:** 167/167 testes verdes (todos os anteriores + 4 novos) | comportamento default idêntico à v2.3 | commit `86c133a` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (baseline ML-3A Go)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-3A — implementar `trackfw baseline` e mecanismo de ratchet em `trackfw validate` (Go).

**Entregue:**
- `internal/validator/validator.go` — adicionado `BaselineFile` struct, `baselineFileName`, `LoadBaseline()`, `SaveBaseline()`; `Validate()` renomeada para `ValidateUnfiltered()` (sem filtros); nova `Validate()` chama `ValidateUnfiltered()`, aplica ratchet de baseline (filtra violations presentes no baseline) e depois aplica modo lenient; import `encoding/json` adicionado.
- `internal/commands/baseline.go` — novo arquivo com `newBaselineCmd()`: chama `ValidateUnfiltered()`, persiste resultado via `SaveBaseline()`, imprime contagem.
- `internal/commands/root.go` — `newBaselineCmd()` registrado após `newValidateCmd()`.
- `internal/validator/validator_baseline_test.go` — 3 testes: `TestBaselineCreation` (cria baseline com violation), `TestBaselineFiltersOldViolations` (Validate() filtra violation do baseline), `TestBaselineNetNewViolation` (Validate() reporta violation não no baseline).

**Resultado:** `go build ./...` verde | 34/34 testes validator verdes (31 existentes + 3 novos) | commit `88456fd` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (baseline ML-3B Node.js)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-3B — implementar `trackfw baseline` e mecanismo de ratchet em `trackfw validate` (Node.js).

**Entregue:**
- `npm/src/validator/index.js` — adicionado `BASELINE_FILE`, `loadBaseline()`, `saveBaseline()`; função `validate()` renomeada para `validateUnfiltered()` (lógica inalterada, sem ratchet); nova `validate()` chama `validateUnfiltered()`, aplica ratchet (filtra violations já no baseline via Set de strings) e depois aplica modo lenient; todas as 4 funções novas exportadas em `module.exports`.
- `npm/src/commands/baseline.js` — novo arquivo; comando `trackfw baseline` chama `validateUnfiltered()` (async), persiste via `saveBaseline()`, imprime contagem.
- `npm/src/commands/index.js` — `require('./baseline')` registrado em `createProgram()`.
- `npm/tests/baseline.test.js` — 4 testes async: `saveBaseline cria .trackfw-baseline.json`, `loadBaseline retorna null se arquivo não existe`, `validate filtra violations do baseline`, `validate reporta violations novas (não no baseline)`.

**Resultado:** 4/4 testes `baseline.test.js` verdes | 16/16 testes `validator.test.js` inalterados | commit `77b8f8a` | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Backend (baseline ML-3C Python)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.4-config-evolution`

**Tarefa:** ML-3C — implementar `trackfw baseline` e mecanismo de ratchet em `trackfw validate` (Python).

**Entregue:**
- `pypi/trackfw/validator.py` — adicionado `import json`; constante `_BASELINE_FILE`; funções `_extract_messages()`, `load_baseline()`, `save_baseline()`; função `validate()` renomeada para `validate_unfiltered()` (sem ratchet, sem lenient); nova `validate()` chama `validate_unfiltered()`, aplica ratchet (filtra violations já no baseline via set de strings extraídas por `_extract_messages`) e depois aplica modo lenient; usa `datetime.now(timezone.utc)` (API moderna, sem DeprecationWarning).
- `pypi/trackfw/commands/baseline.py` — novo arquivo; comando `trackfw baseline` chama `validate_unfiltered()`, persiste via `save_baseline()`, imprime contagem.
- `pypi/trackfw/cli.py` — `baseline_cmd.register(subparsers)` registrado após `log_cmd`.
- `pypi/tests/test_baseline.py` — 4 testes: `test_save_baseline_cria_arquivo`, `test_load_baseline_retorna_none_se_nao_existe`, `test_validate_filtra_violations_do_baseline`, `test_validate_reporta_violations_novas`.

**Resultado:** 4/4 testes `test_baseline*` verdes | 171/171 testes totais verdes | `trackfw baseline` CLI funcional | commit a seguir | push para `feat/v2.4-config-evolution`

---

## Sessão 2026-06-13 — Apolo (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `fix/v2.4.1-baseline-ratchet-warnings`

**Tarefa:** ML-2C — corrigir parser de `trackfw.yaml` em Python: trim de aspas envolventes nos valores do bloco `rules:` e nos escalares top-level.

**Entregue:**
- `pypi/trackfw/config.py` — `_parse()`: valor de sub-chaves de `rules:` agora usa `.strip().strip("\"'")` (linha do bloco `in_rules`); valores escalares top-level (`req_dir`, `roadmap_dir`, `roadmap_namespacing`, `governance_mode`, `lenient_until`) também recebem `.strip("\"'")`.
- `pypi/tests/test_config.py` — 2 novos testes adicionados em `TestConfigEvolution`: `test_rules_value_with_double_quotes` e `test_rules_value_with_single_quotes`.

**Resultado:** 187/187 testes verdes | commit `3f4becf` | push para `fix/v2.4.1-baseline-ratchet-warnings`

---

## Sessão 2026-06-13 — Apolo ML-2A Go (CONCLUÍDO)

**Agente:** Apolo | Status: CONCLUÍDO

**Branch:** `fix/v2.4.1-baseline-ratchet-warnings`

**Tarefa:** ML-2A — corrigir parser de `trackfw.yaml` em Go: trim de aspas envolventes em valores YAML (bloco `rules:` e escalares top-level).

**Entregue:**
- `internal/config/config.go` — `splitKV()` agora aplica `strings.Trim(val, "\"'")` após o `TrimSpace`, removendo aspas simples e duplas de qualquer valor extraído — cobre sub-chaves de `rules:`, `link_fields:` e escalares top-level em uma única mudança centralizada.
- `internal/config/config_evolution_test.go` — 2 novos testes adicionados: `TestRulesValueWithDoubleQuotes` (`adr_orphan: "off"` → `"off"` sem aspas) e `TestRulesValueWithSingleQuotes` (`stale_wip: 'warning'` → `"warning"` sem aspas).

**Resultado:** `go build ./...` verde | 14/14 testes `internal/config` verdes | commit `e6b8b39` | push para `fix/v2.4.1-baseline-ratchet-warnings`

---

## Sessão 2026-06-13 — Backend ML-1B Node.js (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-1B — flag `--json` no `trackfw validate` para o CLI Node.js.

**Arquivos criados/modificados:**
- `npm/src/commands/validate.js` — opção `--json` adicionada ao commander; quando ativa, monta e imprime `JSON.stringify({summary, violations, warnings}, null, 2)` onde `summary = {violations: N, warnings: N, mode: "strict"|"lenient", exit_code: 0|1}`; comportamento texto completamente inalterado sem a flag.
- `npm/tests/validate_json.test.js` (novo) — 12 testes cobrindo: JSON válido, campos summary/violations/warnings presentes, contagem correta, exit_code consistente entre texto e JSON, mode válido, e comportamento texto inalterado sem --json.

**Resultado:** 12/12 validate_json.test.js verdes | 45/45 testes existentes (validator + config + help + baseline) sem regressões | commit e push para `feat/v2.5-discovery-json-traceid`

---

## Sessão 2026-06-13 — Backend ML-2B Node.js paths configuráveis (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-2B — paths configuráveis `adr_dirs`, `req_dir`, `roadmap_dir` no CLI Node.js.

**Diagnóstico:** `npm/src/config/index.js` e `npm/src/validator/index.js` já tinham os campos implementados. Faltava: strip de aspas em `req_dir` e `roadmap_dir` (parser atribuía val direto) e testes dos novos campos.

**Arquivos modificados:**
- `npm/src/config/index.js` — fix: `req_dir` e `roadmap_dir` agora removem aspas envolventes com `.replace(/^["']|["']$/g, '')`.
- `npm/tests/config.test.js` — 4 novos testes ML-2B: `adr_dirs` com 2 itens, `req_dir` customizado, `roadmap_dir` customizado, defaults quando campos ausentes.

**Resultado:** 12/12 config.test.js verdes (8 anteriores + 4 novos) | 0 falhas

---

## Sessão 2026-06-13 — Backend ML-2C Python (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-2C — paths configuráveis `adr_dirs`, `req_dir`, `roadmap_dir` no CLI Python.

**Diagnóstico:** `config.py` e `validator.py` já estavam totalmente parametrizados com os campos `adr_dirs`, `req_dir`, `roadmap_dir` (defaults e parse implementados em versões anteriores). Nenhuma alteração necessária nesses arquivos.

**Arquivos modificados:**
- `pypi/tests/test_config.py` — classe `TestConfigPaths` adicionada com 4 testes: `test_config_adr_dirs_list`, `test_config_req_dir_custom` (UTF-8), `test_config_roadmap_dir_custom`, `test_config_paths_defaults`.

**Resultado:** 17/17 test_config.py verdes | 191/191 testes pypi completos sem regressões | commit `41822c2` | push para `feat/v2.5-discovery-json-traceid`

---

## Sessão 2026-06-13 — Backend ML-2A v2.5 Go paths configuráveis (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-2A — paths configuráveis `adr_dirs`/`req_dir`/`roadmap_dir` no CLI Go.

**Análise:** Campos `ADRDirs`, `REQDir`, `RoadmapDir` e o parser YAML já estavam implementados em `internal/config/config.go`. Os 4 testes nomeados no ML-2A não existiam — criados em `internal/config/config_paths_test.go`.

**Paths hardcoded em `discover.go`:** pertencem ao scanner de discovery brownfield (candidatos de autodetecção), não à camada de config — mantidos intencionalmente.

**Entregue:**
- `internal/config/config_paths_test.go` — 4 testes: `TestConfigAdrDirsList`, `TestConfigReqDirCustom` (UTF-8 docs/requisições), `TestConfigRoadmapDirCustom`, `TestConfigPathsDefaults`.

**Resultado:** 18/18 testes `internal/config` verdes | `make build` limpo | sem regressões novas | commit `d8ad96d` | push para `feat/v2.5-discovery-json-traceid`

---

## Sessão 2026-06-13 — Backend (IMPLEMENTANDO)

**Agente:** Backend | Status: CONCLUIDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-1A — flag `--json` no `trackfw validate` (CLI Go).

**Entregue:**
- `internal/validator/result.go` — structs `RuleItem`, `ValidateSummary`, `ValidateResult` e builder `BuildResult`; slices inicializados como `[]RuleItem{}` para serializar como `[]` e não `null`.
- `internal/commands/validate.go` — flag `--json bool` adicionada ao cobra command; modo JSON usa `cmd.SilenceErrors = true` para saída JSON pura no stdout; exit code idêntico ao modo texto.
- `internal/commands/validate_json_test.go` — 3 testes: `TestValidateJSONFlag` (JSON válido + campos obrigatórios), `TestValidateJSONExitCode` (paridade de exit code), `TestValidateTextUnchanged` (modo texto inalterado).
- `make build` sem erros | 6/6 testes de commands verdes | todos os testes de validator verdes | sem regressões nos pacotes afetados.

---

## Sessão 2026-06-13 — Backend ML-1C v2.5 flag --json no validate Python (IMPLEMENTANDO)

**Agente:** Backend | Status: IMPLEMENTANDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-1C — flag `--json` no `trackfw validate` para o CLI Python.

**Análise:**
- `pypi/trackfw/commands/validate.py` já é implementação completa (não stub)
- `pypi/trackfw/validator.py` retorna dicts `{"type": ..., "message": ...}` — sem campos `rule` e `file`
- Node JS mirror já tem `--json` com estrutura `{summary, violations: [{message}], warnings: [{message}]}`
- Estratégia: adicionar `--json` ao parser; no branch JSON, suprimir toda saída textual e emitir JSON puro; campos `rule`/`file` extraídos do dict se presentes (null se ausentes); testes pytest isolados com tmpdir + os.chdir

**Resultado:** 15/15 test_validate_json.py verdes | 206/206 testes pypi completos sem regressões | commits e2ed388 + b006205 | push para `feat/v2.5-discovery-json-traceid`

**Status final:** CONCLUIDO

**Arquivos modificados:**
- `pypi/trackfw/commands/validate.py` — argumento `--json` adicionado ao parser; branch JSON emite JSON estruturado puro suprimindo saída textual; modo texto inalterado
- `pypi/tests/test_validate_json.py` — 15 testes cobrindo: JSON válido, campos corretos, exit code paridade, modo lenient

---

## Sessão 2026-06-13 — ML-3C: namespacing by_agent — Python CLI

**Agente:** Backend | Status: IMPLEMENTANDO

**Branch:** `feat/v2.5-discovery-json-traceid`

**Tarefa:** ML-3C — `roadmap_namespacing: by_agent` no CLI Python.

**Análise:**
- `pypi/trackfw/config.py` já tem `NAMESPACING_BY_AGENT`, parse de `roadmap_namespacing` e `agents`
- `pypi/trackfw/validator.py` já tem `resolve_wip_dirs`, `validate_wip_limit` e `validate_folder_status_coherence` com suporte by_agent
- `pypi/trackfw/commands/status.py` já tem breakdown por agente
- Falta apenas: `pypi/tests/test_namespacing.py` com 3 testes obrigatórios

---

## 2026-06-13 — ML-3B Node.js namespacing by_agent (CONCLUÍDO)

**Agente:** Backend
**Branch:** `feat/v2.5-discovery-json-traceid`

### O que foi implementado

`npm/tests/namespacing.test.js` criado com 15 testes cobrindo:
- Parse de `roadmap_namespacing: by_agent` e `agents: [zeus, apolo]` no config
- `resolveWIPDirs` retornando hierarquia `<roadmapDir>/<agente>/wip/` no modo by_agent
- `validateWIPHasREQ`, `validateWIPHasAcceptanceCriteria` e `validateWIPLimit` varrendo dois agentes independentemente
- Comportamento flat inalterado (sem regressão)
- `getStatus` exibindo breakdown por agente
- Exportação correta de `NAMESPACING_FLAT` e `NAMESPACING_BY_AGENT`

**Resultado:** 15/15 passando; config.test.js (12) e validator.test.js (16) sem regressão.
**Commit:** `4777f80` — push em `feat/v2.5-discovery-json-traceid`

**Nota:** `config/index.js` e `validator/index.js` já tinham suporte completo a `by_agent` implementado em MLs anteriores. O ML-3B Node.js consistiu exclusivamente em criar a cobertura de testes.

**Resultado:** 9/9 test_namespacing.py verdes | 215/215 testes pypi completos sem regressões | commit 265caa4 | push para `feat/v2.5-discovery-json-traceid`

**Status final:** CONCLUIDO

**Arquivos modificados:**
- `pypi/tests/test_namespacing.py` — 9 testes cobrindo: parse config by_agent, wip_limit por agente, autodiscover de agentes, resolve_wip_dirs, comportamento flat inalterado

**Nota:** config.py, validator.py e status.py já tinham implementação completa de by_agent. Apenas os testes de namespacing estavam ausentes.

---

## 2026-06-13 — ML-5C: req_id bidirecional no CLI Python (Backend)

**Status:** CONCLUIDO
**Branch:** `feat/v2.5-discovery-json-traceid`
**Commit:** `7249687`

**O que foi implementado:**
- `pypi/trackfw/config.py`: campo `trace_id_field` adicionado ao defaults (default `""` — desativado) com parse no `_parse`
- `pypi/trackfw/traceid.py`: novo módulo com `check_traceid(cfg)` — indexa REQs e Roadmaps pelo campo de frontmatter configurado e emite 5 tipos de violations: `traceid_orphan_roadmap`, `traceid_orphan_req`, `traceid_state_mismatch`, `traceid_duplicate_req`, `traceid_duplicate_roadmap`. Parse de frontmatter duplicado localmente para evitar importação circular com `validator.py`
- `pypi/trackfw/validator.py`: integra `check_traceid(cfg)` em `validate_unfiltered()`
- `pypi/tests/test_traceid.py`: 6 testes pytest cobrindo todos os cenários (orphan roadmap, orphan req, state mismatch, duplicate req, par válido sem violation, desativado sem trace_id_field)

**Resultado:** 6/6 test_traceid.py verdes | 221/221 testes pypi completos sem regressões

---

## 2026-06-13 — ML-5A: req_id bidirecional no CLI Go (Backend)

**Status:** CONCLUIDO
**Branch:** `feat/v2.5-discovery-json-traceid`

### O que foi implementado

- `internal/config/config.go`: campo `TraceIdField string` adicionado ao struct `ProjectConfig` + case `trace_id_field` no parser `parse()`.
- `internal/validator/validator_traceid.go`: módulo com `validateTraceId(cfg ProjectConfig)` — 5 verificações: `traceid_orphan_roadmap`, `traceid_orphan_req`, `traceid_state_mismatch`, `traceid_duplicate_req`, `traceid_duplicate_roadmap`. Indexação por estado via subpastas (wip/, done/ etc.) + flat para REQs.
- `internal/validator/validator.go`: `ValidateUnfiltered()` atualizado — carrega `cfg := config.Load()` e chama `validateTraceId(cfg)` ao final.
- `internal/validator/validator_traceid_test.go`: 6 testes (`TestTraceIdOrphanRoadmap`, `TestTraceIdOrphanReq`, `TestTraceIdStateMismatch`, `TestTraceIdDuplicateReq`, `TestTraceIdValidPair`, `TestTraceIdDisabled`) — 6/6 verdes.

**Resultado:** `make build` sem erros | `go test ./internal/validator/ -run TestTraceId -v` 6/6 verdes | `go test ./...` sem novas regressões (falha pré-existente `TestMoveRoadmap_ByAgent` inalterada).

---

## 2026-06-13 — ML-5B: req_id bidirecional no CLI Node.js (Backend)

**Status:** IMPLEMENTANDO
**Branch:** `feat/v2.5-discovery-json-traceid`

**O que está sendo implementado:**
- `npm/src/config/index.js`: campo `traceIdField` no defaults + parse de `trace_id_field` no YAML
- `npm/src/validator/traceid.js`: módulo puro `checkTraceIds(reqDir, roadmapDir, fieldName)` com 5 violations
- `npm/src/validator/index.js`: integração da verificação via `validateUnfiltered()`
- `npm/tests/traceid.test.js`: testes com dirs temporários (mkdtempSync)

---

## 2026-06-13 — ML-3A: namespacing by_agent — testes Go (Backend)

**Status:** IMPLEMENTANDO → CONCLUIDO
**Branch:** `feat/v2.5-discovery-json-traceid`

**O que foi implementado:**
- `internal/validator/validator_namespacing_test.go`: 3 testes novos
  - `TestByAgentNamespacingWIPLimit`: limiar discriminante (zeus=3, apolo=3, limit=5 → total=6 violaria check global mas por agente passa sem warning)
  - `TestByAgentNamespacingWIPLimitExceeded`: agente zeus com 3 WIPs ultrapassa limit=2 → warning somente para zeus
  - `TestByAgentNamespacingFlat`: sem namespacing, comportamento flat — 2 WIPs com limit=1 emite warning global
- `internal/config/config_namespacing_test.go`: 1 teste novo
  - `TestConfigByAgentParsed`: YAML block-style `roadmap_namespacing: by_agent` + `agents: [zeus, apolo]` → struct correto

**Nota:** implementação de config.go, validator.go e generators/roadmap.go estava completa em MLs anteriores. Este ML consistiu exclusivamente em criar os testes de verificação.

**Falha pré-existente (não é responsabilidade do ML-3A):** `TestMoveRoadmap_ByAgent` em `internal/generators/` — ausência de `config.Reset()` faz o singleton retornar flat e `findRoadmap` falha. Confirmado anterior a este ML.

**Resultado:** `go test ./internal/validator/ -run TestByAgent -v` → 3/3 PASS | `go test ./internal/config/ -run TestConfigByAgent -v` → 1/1 PASS | `make build` → sem erros

**Status:** CONCLUIDO
**Commit:** `10119cb`

**Arquivos modificados:**
- `npm/src/config/index.js`: campo `traceIdField: ''` no defaults + case `trace_id_field` no parse YAML
- `npm/src/validator/traceid.js`: módulo puro `checkTraceIds(reqDir, roadmapDir, fieldName)` — indexa REQs e Roadmaps pelo campo de frontmatter e emite 5 violations; state derivado da pasta do arquivo (não do frontmatter)
- `npm/src/validator/index.js`: importa `checkTraceIds` e integra em `validateUnfiltered()` com guard `if (cfg.traceIdField)`
- `npm/tests/traceid.test.js`: 6 testes com mkdtempSync cobrindo todos os cenários

---

## Sessão 2026-06-13 — Backend (IMPLEMENTANDO)

**Tarefa:** ML-1C do roadmap v2.5.1 — popular `rule` e `file` no `--json` + adicionar `trace_id_field` e `rules.traceid_*` ao `trackfw help` no CLI Python.

**Branch:** `fix/v2.5.1-json-rule-file-help-traceid`

**Arquivos modificados:**
- `pypi/trackfw/validator.py`: adicionado `import re`; funções `_extract_file(msg)` e `_enrich_items(items, rule_name)` novas; `_apply_rule` passa por `_enrich_items` antes de distribuir; regras sem `_apply_rule` (diretas) também enriquecidas via `_enrich_items` em `validate_unfiltered`.
- `pypi/trackfw/commands/help_cmd.py`: adicionadas entradas `trace_id_field` + 5 regras `rules.traceid_*` ao `CONFIG_DOCS`.
- `pypi/tests/test_validate_json.py`: novo teste `test_json_violations_tem_campos_rule_e_file` verifica que `rule` e `file` são preenchidos.
- `pypi/tests/test_help.py`: 4 novos testes para `trace_id_field` e `rules.traceid_*`.

**Resultado:** 230/230 testes verdes | Sem regressões

**Status:** CONCLUIDO
**Commit:** `b572ee7`

**Resultado:** 6/6 traceid.test.js verdes | 12/12 config.test.js sem regressões | 12/12 validate_json.test.js sem regressões

---

## Sessão 2026-06-13 — Backend (CONCLUIDO)

**Tarefa:** ML-1B do roadmap v2.5.1 — popular `rule` e `file` no `--json` + adicionar `trace_id_field` e `rules.traceid_*` ao `trackfw help` no CLI Node.js.

**Branch:** `fix/v2.5.1-json-rule-file-help-traceid`

**Arquivos modificados:**
- `npm/src/validator/index.js`: adicionado `_itemMeta` Map com funções `_setMeta`, `getItemMeta` e `resetMeta`; `applyRule` popula o map na fonte; pushs diretos (`req_has_adr`, `blocked_has_req`, `req_has_roadmap`, `frontmatter_presence`, `wip_limit`, `traceid_*`) também populam com nome de regra explícito. Exporta `getItemMeta` e `resetMeta` sem alterar representação interna (strings — baseline e tests inalterados).
- `npm/src/commands/validate.js`: ao montar `--json`, enriquece cada item com `rule`/`file` via `getItemMeta()`.
- `npm/src/commands/help.js`: adicionadas 6 entradas (`trace_id_field` + `rules.traceid_{orphan_roadmap, orphan_req, state_mismatch, duplicate_req, duplicate_roadmap}`) ao `configDocs` com todos os campos obrigatórios.
- `npm/tests/validate_json.test.js`: dois novos testes com fixtures isoladas garantindo violations/warnings reais e verificando `rule`/`file`.
- `npm/tests/help.test.js`: dez novos testes cobrindo `listKeys` e `describeKey` para todas as entradas traceid.

**Resultado:** 14/14 validate_json.test.js | 20/20 help.test.js | 12/12 config.test.js | 6/6 baseline.test.js | 16/16 validator.test.js | 6/6 traceid.test.js | 15/15 namespacing.test.js | 13/13 discover.test.js — todos verdes, zero regressões.

**Status:** CONCLUIDO
**Commit:** `8536b7a`

---

## Sessão 2026-06-13 — Backend ML-1A v2.5.1 — auditoria Go (CONCLUÍDO)

**Agente:** Backend | Status: CONCLUÍDO

**Branch:** `fix/v2.5.1-json-rule-file-help-traceid`

**Tarefa:** Auditoria e verificação do ML-1A do roadmap v2.5.1 — popular `rule` e `file` no `--json` + adicionar `trace_id_field` e `rules.traceid_*` ao `trackfw help` (CLI Go).

**Resultado da auditoria:**
- `internal/validator/result.go` — `TaggedMsg{Rule, Msg}`, `extractFile()`, `BuildResultTagged()` implementados; `BuildResult()` mantido para compatibilidade com assinatura original.
- `internal/validator/validator.go` — `applyRuleTagged()`, `validateUnfilteredTagged()`, `extractRulePrefix()` e `ValidateTagged()` implementados; assinaturas públicas `Validate()`/`ValidateUnfiltered()`/`SaveBaseline()` inalteradas; filtro de baseline e modo lenient preservados em `ValidateTagged`.
- `internal/commands/validate.go` — modo `--json` usa `ValidateTagged()` + `BuildResultTagged()`; modo texto usa `Validate()` original sem alteração.
- `internal/commands/help.go` — 6 entradas adicionadas: `trace_id_field` + `rules.traceid_{orphan_roadmap,orphan_req,state_mismatch,duplicate_req,duplicate_roadmap}`.
- `internal/commands/validate_json_test.go` — asserção `rule='wip_has_req'` e `file='ROADMAP-sem-req.md'` adicionada ao `TestValidateJSONExitCode`.
- `internal/commands/help_test.go` — asserções `trace_id_field` e `rules.traceid_orphan_roadmap` adicionadas ao `TestHelpNoArgs`.

**Testes verificados:**
- `go test ./internal/commands/ -run 'TestValidateJSON|TestHelp' -v` — todos PASS
- `go test ./...` — sem novas regressões; `TestMoveRoadmap_ByAgent` falha pré-existente inalterada
- `make build` — limpo

**Observação:** os arquivos Go já estavam commitados no branch (provavelmente por sessão anterior). A implementação desta auditoria reproduziu o mesmo código já presente no HEAD — confirmando que o ML-1A Go estava correto e completo.

---

## Sessão 2026-06-13 — Apolo (CONCLUÍDO)

**Tarefa:** fix(traceid) ML-1A — suporte a `roadmap_namespacing: by_agent` na função `validateTraceId` + salvaguarda de zero entradas.

**Branch:** `fix/v2.5.2-traceid-by-agent`

**Problema corrigido:** Em projetos com `roadmap_namespacing: by_agent`, os 5 checks `traceid_*` nunca disparavam porque `collectTraceIdEntries` só varria `rootDir/<estado>/`, mas em `by_agent` a estrutura é `rootDir/<agente>/<estado>/`.

**Arquivos modificados:**
- `internal/validator/validator_traceid.go` — nova função `collectTraceIdEntriesByAgent` (varre `rootDir/<agente>/<estado>/*.md`; usa `cfg.Agents` ou descobre agentes via `os.ReadDir`); `validateTraceId` agora escolhe entre `collectTraceIdEntries` e `collectTraceIdEntriesByAgent` com base em `cfg.RoadmapNamespacing`; salvaguarda de zero entradas emite warning descritivo.
- `internal/validator/validator_traceid_test.go` — 2 novos testes: `TestTraceIdByAgent` (valida `traceid_orphan_req` e `traceid_orphan_roadmap` em estrutura by_agent) e `TestTraceIdZeroEntriesSalvaguarda` (valida warning quando diretórios estão vazios).

**Resultado:** `make build` limpo | 8/8 testes TraceId verdes | suite `internal/validator` 100% verde | commit `c7e61b9` | push para `fix/v2.5.2-traceid-by-agent`.


---

## Sessão 2026-06-13 — ML-1A: REQ indexing by_agent (v2.5.3)

**Agente:** Apolo
**Status:** IMPLEMENTANDO
**Branch:** fix/v2.5.3-req-indexing-by-agent

**Objetivo:** corrigir scanner de REQs para suportar req_dir/<agente>/<estado>/ quando roadmap_namespacing: by_agent — adicionar resolveREQFiles, substituir coletas planas em validator.go, fix em validator_traceid.go e salvaguarda one-sided.

---

## Sessão 2026-06-13 — ML-1B: context REQ by_agent (v2.5.4) — Apolo (CONCLUÍDO)

**Tarefa:** fix(npm): `trackfw context` exibia `## REQs (0)` em projetos com `roadmap_namespacing: by_agent`.

**Branch:** `fix/v2.5.4-context-req-by-agent`

**Problema corrigido:** `npm/src/commands/context.js` linha ~102 usava `collectEntries` plana para REQs, sem iterar agentes/estados como já era feito para Roadmaps.

**Arquivos modificados:**
- `npm/src/commands/context.js` — substituído `const reqs = collectEntries(cfg.reqDir || 'docs/req', 'REQ')` por lógica by_agent-aware que descobre agentes via `fs.readdirSync` e itera os 5 estados kanban; fallback para flat quando não é by_agent.
- `npm/tests/context_req_by_agent.test.js` — 2 testes: by_agent encontra REQ em `claude/wip/`; flat sem by_agent não regride.

**Resultado:** 2/2 testes novos verdes | testes `req_by_agent` e `validate_json` sem regressão | commit `5ab2532` | push para `fix/v2.5.4-context-req-by-agent`.

---

## Sessão 2026-06-13 — ML-1C: context REQ by_agent Python (v2.5.4) — Apolo (CONCLUÍDO)

**Tarefa:** fix(python): `trackfw context` exibia `## REQs (0)` em projetos com `roadmap_namespacing: by_agent` no CLI Python.

**Branch:** `fix/v2.5.4-context-req-by-agent`

**Problema corrigido:** `pypi/trackfw/commands/context.py` linha 108 usava `_collect_entries` plana para REQs, sem iterar agentes/estados como já era feito para Roadmaps no mesmo arquivo.

**Arquivos modificados:**
- `pypi/trackfw/commands/context.py` — substituído `reqs = _collect_entries(cfg.get("req_dir", "docs/req"), "REQ")` por lógica by_agent-aware que descobre agentes via `os.listdir` e itera os 5 estados kanban; fallback para flat quando não é by_agent.
- `pypi/tests/test_context_req_by_agent.py` — 2 testes pytest: `test_context_req_by_agent` (REQ em `claude/wip/` encontrada), `test_context_req_flat_no_regression` (modo flat sem regressão).

**Resultado:** 2/2 testes novos verdes | 238/238 testes totais passando | commit `6d10bf3` | push para `fix/v2.5.4-context-req-by-agent`.

---

## Sessão 2026-06-13 — ML-1A: context REQ by_agent Go (v2.5.4) — Apolo (CONCLUÍDO)

**Tarefa:** fix(go): `trackfw context` exibia `## REQs (0)` em projetos com `roadmap_namespacing: by_agent` no CLI Go. Adicionalmente, `validateADRsAreReferenced` usava `os.ReadDir` flat ignorando estrutura by_agent.

**Branch:** `fix/v2.5.4-context-req-by-agent`

**Problemas corrigidos:**
- `internal/generators/context.go` — bloco flat de REQs substituído por lógica by_agent-aware: quando `cfg.RoadmapNamespacing == config.NamespacingByAgent`, descobre agentes via `cfg.Agents` ou `os.ReadDir(cfg.REQDir)` (filtrando dirs) e itera os 5 estados kanban. Fallback flat preservado.
- `internal/validator/validator.go` — `validateADRsAreReferenced` substituiu `os.ReadDir(cfg.REQDir)` flat por `resolveREQFiles(cfg)` (já existia desde v2.5.3), tornando a validação de ADRs órfãos by_agent-aware.

**Testes adicionados:**
- `internal/generators/context_test.go` — `TestContextREQByAgent`: verifica que a lógica by_agent encontra REQ em `req/claude/wip/` com status correto extraído do frontmatter.
- `internal/validator/validator_test.go` — `TestValidateADRsAreReferencedByAgent`: verifica que ADR referenciado em REQ by_agent não gera violation de orphan.

**Resultado:** 2/2 testes novos verdes | `go test ./internal/validator/... ok` | commit `ac0c0de` | push para `fix/v2.5.4-context-req-by-agent`.

---

## Sessão 2026-06-14 — Apolo ML-1A Go (v2.6.0-rules-req-configuraveis) (CONCLUÍDO)

**Tarefa:** ML-1A do roadmap `feat/v2.6.0-rules-req-configuraveis` — tornar `req_has_adr`, `blocked_has_req` e `req_has_roadmap` controláveis via `rules.<nome>: off/warning/error` no `trackfw.yaml`.

**Branch:** `feat/v2.6.0-rules-req-configuraveis`

**Arquivos modificados:**
- `internal/validator/validator.go` — em `ValidateUnfiltered`: substituídos 3 `violations = append(violations, ...)` diretos por `applyRule("req_has_adr", ...)`, `applyRule("blocked_has_req", ...)` e `applyRule("req_has_roadmap", ...)`; em `validateUnfilteredTagged`: substituídos 3 loops `for _, m := range ... { violations = append(..., TaggedMsg{Rule: "", Msg: m}) }` por `applyRuleTagged("req_has_adr", ...)`, `applyRuleTagged("blocked_has_req", ...)` e `applyRuleTagged("req_has_roadmap", ...)`.
- `internal/validator/validator_test.go` — 3 novos testes com 3 sub-testes cada (warning/off/default_error): `TestReqHasADRConfiguravel`, `TestBlockedHasREQConfiguravel`, `TestReqHasRoadmapConfiguravel`. Seguem o padrão `t.TempDir()` + `chdir` + `config.Reset` + `t.Cleanup(config.Reset)`.

**Resultado:** `go build ./...` limpo | 11/11 pacotes de teste verdes (todos) | commit `f94dac9` | push para `feat/v2.6.0-rules-req-configuraveis`.

---

## 2026-06-14 — Apolo — ML-1C (Python) — CONCLUIDO

**Tarefa:** ML-1C do roadmap `feat/v2.6.0-rules-req-configuraveis` — tornar `req_has_adr`, `blocked_has_req` e `req_has_roadmap` configuráveis via `_apply_rule` no CLI Python.

**Branch:** `feat/v2.6.0-rules-req-configuraveis`

**Arquivos modificados:**
- `pypi/trackfw/validator.py` — em `validate_unfiltered`: substituídas 3 linhas `violations += _enrich_items(...)` por `_apply_rule("req_has_adr", ...)`, `_apply_rule("blocked_has_req", ...)` e `_apply_rule("req_has_roadmap", ...)`; renomeada chave `reqs_have_adr` → `req_has_adr` (sem "s") para alinhar cross-CLI.
- `pypi/tests/test_rules_req_configuraveis.py` — 9 testes novos (3 regras × 3 cenários: warning/off/default-error) usando `monkeypatch` para injetar config sem `trackfw.yaml`.

**Resultado:** 9/9 testes do arquivo novo verdes | 247/247 testes da suite completa verdes (sem regressão) | commit `80cf580` | push para `feat/v2.6.0-rules-req-configuraveis`.

---

## 2026-06-14 — Apolo — ML-1B (Node.js) — CONCLUIDO

**Tarefa:** ML-1B do roadmap `feat/v2.6.0-rules-req-configuraveis` — tornar `req_has_adr`, `blocked_has_req` e `req_has_roadmap` configuráveis via `applyRule` no CLI Node.js.

**Branch:** `feat/v2.6.0-rules-req-configuraveis`

**Arquivos modificados:**
- `npm/src/validator/index.js` — em `validateUnfiltered`: substituídos 3 loops `for (const msg of ...)` com push direto em violations por `applyRule('req_has_adr', ...)`, `applyRule('blocked_has_req', ...)` e `applyRule('req_has_roadmap', ...)`. `applyRule` já chama `_setMeta` internamente.
- `npm/tests/rules_req_configuraveis.test.js` — 9 testes novos (3 regras × 3 cenários: warning/off/default-error) usando `process.chdir` + `config.reset()` + dirs temporários.

**Resultado:** 9/9 testes novos verdes | `validate_json.test.js` 14/14 verdes (sem regressão) | `req_by_agent.test.js` 4/4 verdes (sem regressão) | alterações já presentes no commit `80cf580` (commit conjunto com Python) | branch atualizada no remoto.

---

## 2026-06-14 — Athena — Análise de Mercado trackfw v2.6.0 (CONCLUÍDO)

**Tarefa:** Pesquisa via WebSearch de 25+ concorrentes e geração de relatório completo de análise de mercado.

**Entregue:**
- `/tmp/trackfw-market-analysis.md` — relatório completo com 7 seções: mapa de mercado, análise por segmento (ADR tools, Spec/REQ, Roadmap, Platform Engineering, Engineering Metrics, AI-native Governance), posicionamento, pontos fortes/fracos, ameaças/oportunidades e recomendações estratégicas.

**Concorrentes pesquisados:** log4brains, adr-tools (npryce), MADR, pyadr, adr-log, arc-kit, Linear, Shortcut, GitHub Projects, GitLab Requirements, Productboard, Aha!, Backstage, Port.io, Cortex.io, OpsLevel, LinearB, Sleuth, Swarmia, Faros AI, GitHub Copilot Workspace, Cursor Rules/Organizations.

**Insights chave:**
- trackfw ocupa quadrante único: offline-first + CLI-centric + cadeia completa ADR→REQ→ROADMAP com CI gate.
- `roadmap_namespacing: by_agent` e `trace_id_field` são diferenciadores sem equivalente no mercado em jun/2026.
- Maior ameaça: GitHub Copilot Workspace + arc-kit evoluindo para CI gate. Maior oportunidade: SaaS fatigue + AI agents como atores de delivery.

**Agente:** Athena | Status: CONCLUÍDO

---

## 2026-06-14 — Apolo — ML-1A v2.7.0 trackfw serve UI (IMPLEMENTANDO)

**Tarefa:** ML-1A do roadmap `v2.7.0-trackfw-serve-ui` — criar pacote `internal/serve/` com `embed.FS` e placeholder `index.html`; atualizar `commands/serve.go` para usar `serve.Start(port)`.

**Branch:** `feat/v2.7.0-trackfw-serve-ui`

**Arquivos criados/modificados:**
- `internal/serve/serve.go` (novo) — pacote serve com `//go:embed static`, `Start(port int)`, rotas `/` e `/static/*`
- `internal/serve/static/index.html` (novo) — placeholder HTML inicial
- `internal/commands/serve.go` — import trocado de `internal/server` para `internal/serve`

**Resultado:** `go build ./...` limpo | `go test ./...` 100% verde | commit `648af62` | push para `feat/v2.7.0-trackfw-serve-ui`

**Observação:** `internal/server` permanece no projeto (não foi deletado) — será removido/migrado em wave posterior quando os endpoints API forem portados para `internal/serve/api_*.go`.

**Agente:** Apolo | Status: CONCLUÍDO

---

## 2026-06-14 — Apolo — Atualização VISION.md v2.6.0 (CONCLUÍDO)

**Tarefa:** Atualizar `docs/visao-projeto/VISION.md` para refletir o estado atual do projeto (v2.6.0) e posicionamento de mercado.

**Arquivo modificado:**
- `docs/visao-projeto/VISION.md` — header atualizado (v2.6.0 / 2026-06-14); comandos atuais adicionados (`context`, `validate --json`, `serve`, `traceid`); seção `trackfw validate` expandida com `governance_mode`, 15+ regras configuráveis e `trace_id_field` (5 checks automáticos); nova seção "AI-native Governance" com `roadmap_namespacing: by_agent`; seção Distribution atualizada para CLIs nativos (Go + Node.js + Python); 2 novos Design Principles (Configurable by design, AI-agent aware); roadmap antigo substituído por tabela "Current State (v2.6.0)"; seção "What trackfw Is NOT" ajustada para mencionar `trackfw serve`.

---

## 2026-06-14 — Afrodite — ML-0A assets dashboard trackfw serve (CONCLUÍDO)

**Branch:** `feat/v2.7.0-trackfw-serve-ui`

**Tarefa:** Implementar os 3 assets estáticos do dashboard `trackfw serve` (sem bundler, CDN apenas).

**Arquivos criados/modificados:**
- `internal/serve/static/index.html` — substituiu placeholder; layout completo com header/nav (Board/Chain/Metrics), 3 views, drawer lateral com overlay
- `internal/serve/static/style.css` — animacao slideIn do drawer, tab ativa, badge de estado, kanban cards com hover/focus, estilos prose para markdown, frontmatter table, D3 node labels, responsivo mobile (drawer 100% width < 768px)
- `internal/serve/static/app.js` — JS vanilla: loadBoard (kanban com cache, filtro agente), loadChain (D3 force-directed com zoom/pan/drag, setas, coloracao por tipo/estado), loadMetrics (Chart.js donut + burndown line), openDrawer/closeDrawer (fetch /api/file, parseFrontmatter, marked.parse, intercept links .md internos), switchView, filterByAgent, escapeHtml

**Resultado:** `go build ./...` limpo (embed.FS continua funcionando) | 3 arquivos criados

**Agente:** Afrodite | Status: CONCLUÍDO

**Agente:** Apolo | Status: CONCLUÍDO

---

## 2026-06-14 — Apolo — ML-1B→1E v2.7.0 trackfw serve endpoints (IMPLEMENTANDO)

**Tarefa:** Implementar os 4 endpoints da Wave 1 do `trackfw serve`:
- ML-1B: `GET /api/board` — kanban de roadmaps
- ML-1C: `GET /api/chain` — grafo ADR→REQ→ROADMAP
- ML-1D: `GET /api/metrics` — métricas de fluxo (log parser + cálculos)
- ML-1E: `GET /api/file` — leitura segura de arquivos (anti path traversal)

**Branch:** `feat/v2.7.0-trackfw-serve-ui`

**Arquivos a criar:**
- `internal/serve/api_board.go`
- `internal/serve/api_chain.go`
- `internal/serve/api_metrics.go`
- `internal/serve/metrics_log.go`
- `internal/serve/api_file.go`
- Atualizar `internal/serve/serve.go` para registrar os handlers

**Resultado:** `go build ./...` limpo | `go test ./...` 100% verde | commit `8a5dce3` | push para `feat/v2.7.0-trackfw-serve-ui`

**Decisoes tecnicas:**
- `setCORSHeaders` centralizado em `api_board.go` com `Access-Control-Allow-Origin: *` (dev-only)
- `parseFrontmatter` em `api_chain.go` puro sem dependência externa (evita yaml.v3)
- `fileHandler` usa prefixo com separador para evitar falsos positivos (ex: `docs/adr2` vs `docs/adr`)
- `calcBurndown` usa boundary semanal: para cada semana, aplica todos os eventos até o fim da semana para determinar o estado de cada roadmap
- `ParseLog` retorna slice vazia (não nil) quando o arquivo não existe — compatível com o frontend

**Agente:** Apolo | Status: CONCLUÍDO

---

## 2026-06-14 — Apolo — ML-3A: trackfw serve Python

**Status:** CONCLUIDO

**Tarefa:** Implementar `trackfw serve` para o CLI Python — servidor HTTP stdlib com dashboard web (kanban board, chain, metrics, file API).

**Branch:** `feat/v2.7.0-trackfw-serve-ui`

**Resultado:** 247 testes passando | commit `10e1a23` | push para `feat/v2.7.0-trackfw-serve-ui`

**Decisoes tecnicas:**
- `functools.partial` para injetar `cfg` no `BaseHTTPRequestHandler` sem variável global
- `os.path.realpath` + sufixo `os.sep` para evitar falsos positivos em path traversal (ex: `/docs/adr` vs `/docs/adr2`)
- `_parse_log` de `commands/metrics.py` reutilizado diretamente — sem duplicação
- Assets estáticos copiados de `internal/serve/static/` e declarados em `pyproject.toml` via `[tool.setuptools.package-data]`
- Detecção automática de agentes por subdiretórios quando `roadmap_namespacing == "by_agent"` e `agents: []`

**Agente:** Apolo | Status: CONCLUIDO

---

## 2026-06-14 — Apolo — ML-2A: trackfw serve Node.js

**Status:** CONCLUÍDO

**Tarefa:** Implementar `trackfw serve` para o CLI Node.js — servidor HTTP nativo (sem Express) com dashboard web.

**Arquivos criados/modificados:**
- `npm/src/commands/serve.js` — comando CLI + createServer com roteamento HTTP
- `npm/src/serve/api_board.js` — scan kanban (flat + by_agent)
- `npm/src/serve/api_chain.js` — grafo ADR→REQ→ROADMAP com parseFrontmatter nativo
- `npm/src/serve/api_metrics.js` — reutiliza parseLog/calculate de metrics.js
- `npm/src/serve/api_file.js` — segurança path traversal (resolve + allowedDirs)
- `npm/src/serve/static/` — cópia dos assets de internal/serve/static/
- `npm/src/commands/metrics.js` — exporta parseLog e calculate além do cmd
- `npm/src/commands/index.js` — registra createServeCommand()

**Critérios de aceite verificados:**
- `node npm/bin/trackfw serve --no-open --port 9191` sobe sem erro
- `/api/board` retorna JSON válido com columns e agents
- `/api/metrics` retorna JSON com lead_time, cycle_time, abandonment_rate, state_distribution, burndown
- `/api/chain` retorna JSON com nodes e edges
- `/api/file?path=../../../etc/passwd` retorna 403
- `/static/app.js` retorna 200

**Commit:** `8ea11ee` | **Push:** `feat/v2.7.0-trackfw-serve-ui`

**Observação:** O ambiente tem processos Go `main` ouvindo em várias portas (8080, 8081, etc.) que interceptam requisições com autenticação. Os testes foram realizados na porta 9191.

**Agente:** Apolo | Status: CONCLUÍDO

---

## Sessao 2026-06-14 — ML-4B Testes Node.js serve APIs

**Agente:** Artemis | Status: CONCLUIDO
**Branch:** feat/v2.7.0-trackfw-serve-ui
**REQ:** docs/requisicoes/artemis/done/REQ-2026-06-14-serve-api-tests-nodejs.md
**ROADMAP:** docs/roadmap/artemis/done/ROADMAP-2026-06-14-serve-api-tests-nodejs.md

**Arquivo criado:** `npm/tests/serve_api.test.js`
**Resultado:** 8/8 testes passaram | 0 regressoes nos 130 testes existentes
**Cobertura:**
- api_board: flat mode (columns + agents), by_agent mode (agent no card), board vazio
- api_file: path valido (200), path traversal (403), path fora dos dirs (403)
- api_metrics: sem log (zeros), com log valido (cycle_time_avg_days calculado)

---

## Sessao 2026-06-14 — ML-4C Testes Python serve APIs

**Agente:** Artemis | Status: CONCLUIDO
**Branch:** feat/v2.7.0-trackfw-serve-ui

**Objetivo:** Implementar `pypi/tests/test_serve_api.py` cobrindo api_board, api_file e api_metrics.

**Resultados:**
- 14 testes implementados e passando (pytest)
- Suite completa: 261/261 PASSED, sem regressoes
- Cobertura: api_board (flat, by_agent, autodetect, vazio), api_file (200, 403 traversal, 403 outside, _is_safe_path unit), api_metrics (sem log zeros, com log cycle_time, abandonment_rate, _calc_cycle_time direto)
- Path traversal bloqueado e testado com `../../../etc/passwd` → 403

**Commit:** `80e2492` | **Push:** `feat/v2.7.0-trackfw-serve-ui`

**Agente:** Artemis | Status: CONCLUIDO

---

## Sessao 2026-06-15 — ML-1A discover auto-install hook framework

**Agente:** Backend | Status: CONCLUIDO
**Branch:** `feat/discover-init-hook-autoinstall`
**Commit:** `0df8b6f`

**Objetivo:** `trackfw discover --init` sem framework detectado agora auto-instala lefthook ou husky em vez de apenas imprimir aviso.

**Mudancas implementadas:**
- `internal/discover/discover.go`:
  - `InstallGates` e `installHook` agora recebem `io.Writer` — corrige vazamento de `fmt.Println` para stdout
  - `case default` em `installHook`: detecta `package.json` → chama `installHusky`; ausente → chama `installLefthook`
  - `installLefthook`: cria `lefthook.yml` com entrada trackfw-validate; idempotente; tenta `lefthook install` se disponivel no PATH
  - `installHusky`: executa `npm install --save-dev husky` + `npx husky init`; cria `.husky/pre-commit` com `MkdirAll`; erros de exec sao warn, nao bloqueantes
- `internal/commands/discover.go`: repassa `out` (cobra writer) para `InstallGates`
- `internal/discover/discover_test.go`:
  - Testes existentes atualizados para nova assinatura (`io.Discard`)
  - 5 novos testes: sem package.json → lefthook.yml criado; com package.json → .husky/pre-commit criado; idempotencia lefthook; default sem/com package.json

**Resultado:** `make build`, `make test`, `make lint` — todos verdes

---

## Sessão 2026-06-17 — Apolo (CONCLUÍDO)

**Tarefa:** Feature de progresso de Wave/ML em `internal/serve/api_board.go`.

**Entregue:**
- `boardItem` — 3 campos novos: `MLTotal int`, `MLDone int`, `ActiveML string` (JSON: `ml_total`, `ml_done`, `active_ml`).
- `parseMLProgress(path string) (total, done int, activeML string)` — lê o arquivo de roadmap linha a linha; detecta linhas `## ... Wave` (captura título da wave atual), `### ML-*` (incrementa total, salva mlTitle), `**Status:**` com `✅` (incrementa done) ou `🔄` (preenche `activeML` como `"<wave> · <ml>"`). Tolerante a roadmaps sem waves (activeML usa somente o título do ML).
- `readStateDir` — chama `parseMLProgress(fullPath)` para cada card e popula os 3 campos novos no `boardItem`.

**Resultado:** `make build` limpo | `make test` 100% verde (todos os pacotes, incluindo `internal/serve`)

---

## Sessão 2026-06-18 — Zeus (CONCLUÍDO)

**Tarefa:** Implementar `trackfw update` nos 3 CLIs (Go + Node.js + Python).

**Entregue:**
- `internal/generators/update.go` — `Update(cwd)`, `ReadUpdateConfig(cwd)`, `updateHooksSurgical(cfg)`.
- `internal/generators/scaffold.go` — `ForceGenerateClaudeCommands()`, `ForceInstallSkills()`, variantes internas `force bool`.
- `internal/commands/update.go` — comando cobra `trackfw update`.
- `npm/src/commands/update.js` — comando Node.js com mesma lógica.
- `npm/src/generators/init.js` — `generateClaudeCommandsForce(rootDir)`, `installSkillsForce()`.
- `npm/src/commands/discover.js` — `writeCIWorkflowForce(rootDir)`, exports de `writeValidateScript/writeCIWorkflow`.
- `pypi/trackfw/commands/update.py` — escopo reduzido: apenas regras de agente.
- REQ: `docs/requisições/claude/REQ-2026-06-18-trackfw-update-command.md`.

**Comportamento:** 3 categorias de update — (1) marker-delimited via InjectRulesDetected, (2) trackfw-owned force overwrite, (3) shared hooks com inject cirúrgico.
**Branch:** `feat/kanban-roadmap-progress` | Roadmap: `done/trackfw-update-command-2026-06-18.md`

---

## Sessão 2026-06-20 — Apolo (CONCLUÍDO)

**Tarefa:** Implementar sistema de attention hooks do trackfw no CLI Go.

**Entregue:**
- `internal/generators/hooks.go` (novo) — `InjectHooksDetected(cwd)` detecta CLIs presentes e injeta hooks; injetores por CLI: `injectClaudeHooks` (merge idempotente em `.claude/settings.json`), `injectCodexHooks` (`.codex/hooks.json`), `injectGeminiHooks` (`.gemini/settings.json`), `injectKiroHooks` (`.kiro/hooks/trackfw-attention.json` — arquivo dedicado), `injectCopilotHooks` (`.github/hooks/trackfw-attention.json` — arquivo dedicado), `injectCursorHooks` (`.cursor/hooks.json`); helpers `mergeClaudeHookArray` e `mergeSimpleCommandArray` para deduplicação por command.
- `internal/generators/scaffold.go` — função `generateAttentionScripts()` gera `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh` (permissão 0755); chamada adicionada em `Scaffold()` após `generateValidateScript`.
- `internal/generators/update.go` — passo 1b (`InjectHooksDetected`) adicionado após passo 1; `generateAttentionScripts()` chamada junto com validate script.
- `internal/commands/discover.go` — `generators.InjectHooksDetected(cwd)` invocado após `InjectRulesDetected` no fluxo `--init`.
- `internal/generators/agentfiles.go` — nota Windsurf adicionada na seção `### Attention Signal` do rules block.

**Resultado:** `go build ./...` limpo | `go vet ./...` limpo | `go test ./...` 100% verde.
**Branch:** `feat/attention-hooks-agent-clis`

---

## Sessão 2026-06-24 — Estabilização de qualidade (CONCLUÍDA)

**Branch:** `fix/repository-quality-gates`

**Objetivo:** corrigir a paridade do entrypoint Python, tornar os testes herméticos,
adicionar quality gates de CI/release e formalizar o contrato entre Go, Node.js e Python.

### Encerramento

**Status:** CONCLUÍDO

**Entregue:**
- Entry point Python conectado aos handlers reais, incluindo novo `init` não interativo.
- `version` e `--version` disponíveis nos três CLIs.
- Testes Go sem instalações externas reais e processos de discovery com timeout.
- CI de PR/push e gate obrigatório no workflow de release.
- Contratos automatizados de comandos, JSON de `validate` e assets do dashboard.
- `/api/attention` implementado no dashboard Node.js e Python.
- Build e smoke test dos pacotes npm e wheel Python.
- Downloads de plugins Go/Node com timeout, limite de tamanho e substituição atômica.
- Runtime mínimo alinhado: Go 1.25+, Node.js 18+, Python 3.10+.

---

## Sessão 2026-06-24 — Paridade documental de agentes (CONCLUÍDO)

**Tarefa:** alinhar a documentação visível e o log interno com a cobertura real de agentes e hooks.

**Entregue:**
- `site/guide/ai-agents.md` e `site/en/guide/ai-agents.md` — intro atualizada para listar Codex, Claude Code, Gemini CLI, Cursor, GitHub Copilot, Windsurf e Amazon Q.
- `site/index.md` e `site/en/index.md` — teaser de home alinhado à lista atual de agentes suportados.
- `site/guide/getting-started.md` e `site/en/guide/getting-started.md` — bullets de onboarding atualizados.
- `docs/agents-working-context.md` — sessão registrada com a fase de paridade documental.

**Validação:** `trackfw validate --json` manteve `violations=0` e `warnings=0`; `go test ./...`, `npm test` e `pytest pypi/tests` permaneceram verdes na fase anterior.

**Branch:** `feat/codex-agent-integrations`

**Validação:**
- `make quality` verde.
- Go: `go test`, `go vet` e `go build` verdes.
- Node.js: 13 arquivos de teste verdes.
- Python: 265 testes verdes.
- Wheel e tarball npm construídos e executados com sucesso.

---

## Sessão 2026-07-18 — Agents/skills lifecycle multi-CLI (CONCLUÍDO)

**Branch:** `feat/agents-skills-lifecycle-multi-cli`

**Objetivo:** substituir os instaladores fragmentados por um catálogo canônico e
adapters nativos, expondo `list`, `install`, `uninstall` e `update` para `agents` e
`skills` com paridade Go, Node.js e Python.

**Governança:**
- ADR: `docs/adr/ADR-2026-07-18-catalogo-canonico-e-adapters-para-integracoes-de-agentes.md`
- REQ: `docs/req/REQ-2026-07-18-agents-skills-lifecycle-multi-cli.md`
- Roadmap: `docs/roadmaps/done/ROADMAP-2026-07-18-agents-skills-lifecycle-multi-cli.md`

**Matriz entregue:** Claude, Codex, Gemini, Antigravity, Cursor, Copilot,
Windsurf, Amazon Q e Kiro, com formatos nativos ou fallback declarado.

### Progresso em 2026-07-18

- Waves 1 e 2 concluídas: catálogo canônico, manifesto de ownership e os quatro
  subcomandos de lifecycle estão implementados em Go, Node.js e Python.
- Os três runtimes compartilham o schema de manifesto v1, os estados
  `not-installed/current/outdated/modified` e as proteções de update/uninstall.
- O `list` exibe todos os itens e todas as surfaces compatíveis por target; uma
  surface específica pode ser escolhida com `--surface target=surface`.
- Testes focados Go, npm e Python estão verdes; assets dos três runtimes estão
  byte-idênticos. A Wave 3 iniciou os gates de empacotamento e migração legada.

### Encerramento

- Lifecycle `list/install/uninstall/update` entregue com JSON semanticamente
  idêntico em Go/Homebrew, npm e PyPI.
- Migração byte-exata cobre instalações Claude/Codex anteriores dos três
  pacotes, preservando conteúdo desconhecido e customizações.
- Sync/check de assets, smoke real do tarball npm e do wheel Python e matriz
  hermética dos nove targets foram aprovados.
- `make quality`: verde; Python: 300 testes; Node: 40 testes top-level;
  `trackfw validate --json`: zero violações e zero warnings.

---

## Sessão 2026-07-19 — Validação de Escrita de Arquivos (CONCLUÍDO)

**Branch:** `main`

**Objetivo:** Validar o acesso de escrita a arquivos no repositório.

**Progresso:**
- Acesso de escrita validado com sucesso.
- O arquivo `docs/valida-escrita.md` foi criado e persistido no repositório.
- Atualização do arquivo de contexto de agentes realizada com sucesso.

---

## Sessão 2026-07-19 — Suporte a ADRs Globais e Diretivas de IA (CONCLUÍDO)

**Branch:** `feat/global-adrs-governance`

**Objetivo:** Criar a especificação (ADR e REQ) para o suporte a ADRs globais compartilhados e diretivas de IA.

**Progresso:**
- Arquivo ADR `docs/adr/ADR-2026-07-19-global-adrs-governance.md` criado com sucesso.
- Arquivo REQ `docs/req/REQ-2026-07-19-global-adrs-governance.md` criado com sucesso.
- Contexto de trabalho atualizado.

---

## Sessão 2026-07-19 — Customização da Statusline (CONCLUÍDO)

**Branch:** `feat/global-adrs-governance` (sem alterações de código no repositório)

**Objetivo:** Configurar a statusline do Antigravity CLI com o layout Powerline personalizado.

**Progresso:**
- Criado o script Python em `~/.gemini/antigravity-cli/statusline.py` para receber o payload do CLI e formatá-lo com cores e setas Powerline.
- Atualizado o arquivo de configuração `~/.gemini/antigravity-cli/settings.json` para apontar para o novo script.

---

## Sessão 2026-07-19 — Apolo ML-1C (CONCLUÍDO)

**Tarefa:** ML-1C do roadmap `ROADMAP-2026-07-19-antigravity-agent-tools.md` — Implementar renderer `agent-directory` no CLI Python.

**Arquivos alterados:**
- `pypi/trackfw/integrations/renderers.py` — novo branch para `kind == "agents" and target == "antigravity" and surface == "current"`: reconstrói frontmatter com mapeamento de model (opus→pro, sonnet→flash) e injeção de tools (SET_IMPL 10 / SET_ARCH 14). Helpers: `_map_model`, `_agent_tools`, constantes `_MODEL_MAP`, `_SET_IMPL`, `_SET_ARCH`.
- `pypi/tests/test_agents_skills.py` — novo teste `test_antigravity_current_surface_renders_agent_directory`: valida architect (14 tools, model: pro, sem opus) e backend (10 tools, model: flash, sem define_subagent), ambos sem IDs proibidos.

**Resultado:** 31/31 testes verdes. Paridade byte-a-byte com implementação Go (`internal/integrations/render.go`).

---

## Sessão 2026-07-19 — ML-1A: Render agent-directory para Antigravity (IMPLEMENTANDO)

**Agente:** Apolo (Backend Specialist)
**Branch:** feat/antigravity-agent-tools (criada por Zeus)

**Objetivo:** Adicionar `case "agent-directory"` em `internal/integrations/render.go` para reconstruir frontmatter sem `model: opus|sonnet` e com `tools:` (SET_IMPL / SET_ARCH).

**Progresso:**
- Estendeu `markdownParts` para retornar 4º valor `model string`.
- Adicionou `case "agent-directory"` no switch de `Render` com reconstrução de frontmatter.
- Implementou helpers `mapModel` (opus→pro, sonnet→flash, passthrough para flash_lite/flash/pro) e `agentTools` (SET_IMPL 10 tools / SET_ARCH 14 tools).
- Adicionou `TestRenderAgentDirectory` com subtestes architect e backend.
- `go test ./internal/integrations/...` verde.
- `make build` sem erros.
- Nenhum asset alterado.

**Status: CONCLUIDO**

---

## Sessão 2026-07-19 — ML-1B: Render agent-directory para Antigravity no CLI Node.js (CONCLUÍDO)

**Agente:** Apolo (Backend Specialist)
**Branch:** feat/antigravity-agent-tools (criada por Zeus)

**Objetivo:** Adaptar `npm/src/integrations/render.js` para a representação `agent-directory` com mapa de model e injeção de tools; adicionar teste golden em `npm/tests/agents-skills.test.js`.

**Entregue:**
- `markdownParts` estendido para capturar campo `model` do frontmatter.
- Helpers `resolveModel` (opus→pro, sonnet→flash, passthrough flash_lite/flash/pro, '' para ausente/não-mapeável) e `toolsFor` (SET_ARCH 14 tools para nomes terminando em "architect", SET_IMPL 10 tools para demais).
- Constantes `SET_IMPL` e `SET_ARCH` locais; IDs proibidos nunca incluídos.
- Branch `if (capability.representation === 'agent-directory')` que reconstrói frontmatter e preserva body.
- Formato byte-equivalente ao Go (ML-1A): `---\nname/description/model(opcional)/tools---\nbody\n`.
- Teste golden `'Antigravity agent-directory renderer é byte-equivalente ao contrato Go/Python'` com `assert.equal` de string completa para architect e backend + asserts de ausência de IDs proibidos.
- `node --test npm/tests/agents-skills.test.js`: 21/21 testes passando.
- Nenhum asset em `npm/src/integrations/assets/agents/` alterado.

---

## 2026-07-19 — Apolo | Housekeeping: sincronização de version files → 2.14.0

**Status:** CONCLUIDO
**Branch:** `chore/sync-version-files-2.14.0`
**Agente:** Apolo (Backend Senior Specialist)

### O que foi feito
- Bump de `2.12.4` → `2.14.0` nos 5 version files: `internal/version/version.go`, `npm/package.json`, `pypi/pyproject.toml`, `pypi/trackfw/__init__.py`, `docs/visao-projeto/VISION.md`.
- Build validado: `make build` sem erros.
- Binário confirmado: `./bin/trackfw version` → `trackfw v2.14.0`.
- Testes verdes: `go test ./internal/version/... ./internal/integrations/...`.
- grep de residual `2.12.4` nos 5 arquivos: vazio.
- Commit: `2ed0874` — apenas os 5 arquivos, sem push (Zeus faz o push).

---

## Sessão 2026-07-20 — Zeus (CONCLUÍDO)

**Tarefa:** Verificação e consolidação do backlog para codar no projeto.
**Agente:** 🌩️ Zeus - Principal Software Architect

**Ações:**
- Inspecionados diretórios `docs/req/`, `docs/roadmaps/`, `docs/requisições/` e `docs/adr/`.
- Mapeadas 4 demandas pendentes/backlog.

---

## Sessão 2026-07-20 — Zeus (IMPLEMENTANDO)

**Tarefa:** Orquestração e disparo da Wave 1 do ROADMAP-2026-07-19-global-adrs-governance.md.
**Branch:** `feat/global-adrs-governance`
**Agente:** 🌩️ Zeus - Principal Software Architect

**Ações:**
- Criada branch `feat/global-adrs-governance`.
- Gerado `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` detalhado com 4 waves e paralelização de microlotes.
- Atualizado vínculo no `REQ-2026-07-19-global-adrs-governance.md`.
- Commit de docs realizado (`d6f649b`).
2202: - Disparados 3 subagentes paralelos para Wave 1 (ML-1A Go, ML-1B Node, ML-1C Python).

---

## Sessão 2026-07-20 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-1C do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Expansão de tilde (`~` / `~/`) no CLI Python (`config.py` e `validator.py`).
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Ações:**
- Iniciando implementação da expansão de til em `pypi/trackfw/config.py` e `pypi/trackfw/validator.py`.
- Adição de testes em `pypi/tests/test_config.py` e `pypi/tests/test_validator.py`.


---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-1A)

**Tarefa:** ML-1A - Suporte à expansão de til (`~` ou `~/`) no carregamento de `adr_dirs` no CLI Go (`internal/config/config.go` e `internal/validator/validator.go`).
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `internal/config/config.go`: adicionada função exportada `ExpandPath(p string) string` utilizando `os.UserHomeDir()` e `filepath.Join()`. Atualizado o parser `parse()` de `trackfw.yaml` para expandir caminhos em `adr_dirs`.
- `internal/validator/validator.go`: atualizadas funções `walkADRFiles`, `findADRFile` e `referenceExists` para expandir caminhos com `config.ExpandPath()`.
- `internal/config/config_paths_test.go`: adicionados testes `TestExpandPath` e `TestConfigTildeExpansionInAdrDirs`.
- `internal/validator/validator_test.go`: adicionado teste `TestValidate_WithTildeInADRDirs`.
- Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md`: ML-1A marcado como `✅ Concluído`.

## Sessão 2026-07-20 — Zeus (IMPLEMENTANDO)

**Tarefa:** Orquestração e disparo do ROADMAP-2026-06-20-attention-hooks-agent-clis.md (Backlog #2).
**Branch:** `feat/attention-hooks-agent-clis`
**Agente:** 🌩️ Zeus - Principal Software Architect

**Ações:**
- Feito `git checkout main`, `git pull origin main` (PR #56 mesclado).
- Alternado para a branch isolada `feat/attention-hooks-agent-clis` (rebased na `main`).
- Movido e configurado roadmap `docs/roadmaps/ROADMAP-2026-06-20-attention-hooks-agent-clis.md` (status `wip`).
- Disparado subagente para Wave 1 (ML-1A: Scripts shell `trackfw-attention-signal.sh` e `trackfw-attention-cleanup.sh`).





## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-1C)

**Tarefa:** ML-1C do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Expansão de tilde (`~` / `~/`) no CLI Python (`config.py` e `validator.py`).
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `pypi/trackfw/config.py`: adr_dirs utiliza `os.path.expanduser` durante a leitura/parse de listas YAML.
- `pypi/trackfw/validator.py`: `_find_adr_file`, `_adr_is_draft`, `validate_adrs_are_referenced`, `validate_frontmatter_presence` e `validate_ref_targets_exist` utilizam `os.path.expanduser` em cada `adr_dir`.
- `pypi/tests/test_config.py`: adicionado `test_config_adr_dirs_tilde_expansion` testando o parse de `~/...`.
- `pypi/tests/test_validator.py`: adicionada classe `TestExpandTildeAdrDirs` com `test_find_adr_file_com_tilde` e `test_validate_adrs_are_referenced_com_tilde`.
- Status do ML-1C no roadmap atualizado para `✅ Concluído`.


---

## Sessão 2026-07-20 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-2A do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Suporte a `strict_ci_paths` (default `false`), Warning para `adr_dirs` inexistentes e isenção de `adr_orphan` para ADRs fora do `cwd` no Go CLI.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Ações:**
- Iniciando implementação de `strict_ci_paths` em `internal/config/config.go`.
- Ajustando validações em `internal/validator/validator.go`.
- Adicionando testes unitários em `internal/validator/validator_test.go`.

---

## Sessão 2026-07-20 — Afrodite (CONCLUÍDO ML-2B)

**Tarefa:** ML-2B do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Suporte a `strict_ci_paths` (default `false`), `Warning` para diretórios `adr_dirs` inexistentes e isenção de `adr_orphan` para arquivos fora de `cwd` no CLI Node.js.
**Agente:** 💖 Afrodite — Frontend i18n Senior Specialist

**Entregue:**
- `npm/src/config/index.js`: adicionada opção `strictCiPaths` no `defaults()` (default `false`) e parse de `strict_ci_paths` no parser YAML.
- `npm/src/validator/index.js`:
  - Criados helpers `isInsideDir` e `walkDirMdWithPaths`.
  - Criada função `validateADRDirsExist` que retorna `warnings` se `strictCiPaths: false` (default) ou `violations` se `strictCiPaths: true` para diretórios `adr_dirs` inexistentes.
  - Atualizada `validateADRsAreReferenced` para isentar diretórios e arquivos de ADR externos à raiz do projeto (`cwd`) da verificação de `adr_orphan`.
- `npm/tests/config.test.js`: adicionado teste de `strict_ci_paths`.
- `npm/tests/validator.test.js`: adicionados testes unitários validando warning/violation para dir inexistente e isenção de `adr_orphan` para ADRs externos.
- Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md`: ML-2B marcado como `✅ Concluído`.


---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-2C)

**Tarefa:** ML-2C do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Suporte a `strict_ci_paths` (default `False`), `Warning` para diretórios `adr_dirs` não encontrados e isenção de `adr_orphan` para arquivos fora de `cwd` no CLI Python.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `pypi/trackfw/config.py`: `strict_ci_paths` adicionado aos `defaults()` (default `False`) e parseado a partir de `trackfw.yaml`.
- `pypi/trackfw/validator.py`:
  - Helper `_is_subpath` criado para identificar arquivos/diretórios contidos em `cwd`.
  - `validate_adr_dirs_exist` verifica se os diretórios em `adr_dirs` existem, emitindo `Warning` se `strict_ci_paths` for `False` e `violation` se `strict_ci_paths` for `True`.
  - `validate_adrs_are_referenced` isenta caminhos fora de `cwd` da regra `adr_orphan`.
- `pypi/tests/test_config.py`: teste `test_config_strict_ci_paths` adicionado.
- `pypi/tests/test_validator.py`: classes `TestStrictCIPathsAndInexistentAdrDirs` e `TestAdrOrphanExemptOutsideCwd` adicionadas.
- Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md`: ML-2C marcado como `✅ Concluído`.


---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-2A)

**Tarefa:** ML-2A do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Suporte a `strict_ci_paths` (default `false`), `Warning` para diretórios `adr_dirs` inexistentes e isenção de `adr_orphan` para arquivos fora do `cwd` no Go CLI.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `internal/config/config.go`: adicionado campo `StrictCIPaths bool` em `ProjectConfig` (default `false`) e parse de `strict_ci_paths` a partir do YAML.
- `internal/config/config_paths_test.go`: adicionado `TestConfigStrictCIPaths` cobrindo o default `false` e parse quando `true`.
- `internal/validator/validator.go`:
  - `validateADRDirsExist`: verifica se cada diretório em `adr_dirs` existe; se não existir, gera `Warning` (se `StrictCIPaths == false`) ou `Error` violation (se `StrictCIPaths == true`).
  - `isOutsideCWD`: helper que determina se um caminho está fora da raiz do projeto local (`cwd`).
  - `validateADRsAreReferenced`: isenta arquivos ADR localizados fora do `cwd` da verificação `adr_orphan`.
- `internal/validator/validator_test.go`: adicionados testes `TestValidate_NonExistentADRDirs_WarningByDefault`, `TestValidate_NonExistentADRDirs_StrictCIPathsError` e `TestValidate_ExternalADROrphanExemption`.


---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-3B)

**Tarefa:** ML-3B do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Injetar a diretiva obrigatória de leitura dos ADRs globais no gerador de regras de agente para Python.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `pypi/trackfw/generators/init_gen.py`: inclusão da diretiva `"- Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura."` na seção `Architecture Directives (mandatory)` de `_trackfw_rules_block()`.
- `pypi/tests/test_generators_init.py`: adicionada a classe `TestGlobalADRsRuleDirective` com teste `test_rules_block_contains_global_adrs_directive` validando a presença da nova diretiva no bloco gerado e na injeção em arquivos de agentes.
- `pypi/tests/test_rules_agents.py`: atualizado para asserção do snippet da diretiva em múltiplos assistentes.
- Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md`: ML-3B marcado como `✅ Concluído`.

---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-3A)

**Tarefa:** ML-3A do Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md` — Injetar a diretiva obrigatória de leitura dos ADRs globais nos geradores de regras de agente para Go e Node.js.
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `internal/generators/claudemd.go`: inclusão da diretiva `"8. **Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura.**"` no bloco de Agent rules.
- `internal/generators/scaffold.go`: inclusão da mesma diretiva nas Regras invioláveis de `installGlobalSkillInner`.
- `internal/generators/agentfiles.go`: inclusão da diretiva no Agent Protocol de `trackfwRulesBlock()`.
- `internal/generators/claudemd_test.go`: criação de suíte de testes unitários Go cobrindo a presença da diretiva em `CLAUDE.md`, `trackfwRulesBlock` e na skill global `SKILL.md`.
- `npm/src/generators/init.js`: inclusão da diretiva em `trackfwRulesBlock()` e `generateClaudeMD()`.
- `npm/tests/generators.test.js`: criação de suíte de testes unitários Node.js validando a inclusão da diretiva no bloco de regras e em arquivos gerados.
- Roadmap `docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md`: ML-3A marcado como `✅ Concluído`.

---

## Sessão 2026-07-20 — Apolo (CONCLUÍDO ML-1A)

**Tarefa:** ML-1A do Roadmap `docs/roadmaps/ROADMAP-2026-06-20-attention-hooks-agent-clis.md` — Geração dos scripts `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh` nos 3 geradores de scaffold/init (Go, Node.js, Python).
**Agente:** ☀️ Apolo — Backend Senior Specialist

**Entregue:**
- `internal/generators/scaffold.go`: atualizada a função `generateAttentionScripts()` para gerar `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh` com o conteúdo exato exigido e permissão `0755`.
- `internal/generators/scaffold_test.go`: adicionado o teste `TestGenerateAttentionScripts` garantindo a criação dos dois scripts, permissões executáveis e validação do cabeçalho do conteúdo.
- `npm/src/generators/hooks.js`: atualizadas as constantes `SIGNAL_SCRIPT` e `CLEANUP_SCRIPT` para gerar os scripts com o conteúdo exato exigido.
- `npm/tests/generators.test.js`: adicionado o teste `scaffold generates attention scripts with execution permissions and expected headers`.
- `pypi/trackfw/generators/init_gen.py`: atualizadas as constantes `_ATTENTION_SIGNAL_SH` e `_ATTENTION_CLEANUP_SH` para o conteúdo exato exigido.
- `pypi/tests/test_generators_init.py`: adicionada a classe de testes `TestAttentionScripts` validando existência, permissões executáveis no POSIX e cabeçalhos dos scripts.
- `docs/roadmaps/ROADMAP-2026-06-20-attention-hooks-agent-clis.md`: ML-1A marcado como `✅ Concluído`.




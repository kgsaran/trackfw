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

## Sessão 2026-06-13 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-1B do roadmap Python CLI nativo — módulo i18n Python com suporte pt-BR/en-US/es-ES.

**Branch:** `feat/v2.2-python-cli-nativo`

**Arquivos a criar:**
- `pypi/trackfw/i18n/__init__.py` — função `t(key)`, detecção de locale via env vars, suporte a chaves aninhadas com `.`
- `pypi/trackfw/i18n/locales/{pt-BR,en-US,es-ES}.json` — cópia dos JSONs do npm
- `pypi/tests/test_i18n.py` — testes unittest

---

## Sessão 2026-06-13 — Apolo (IMPLEMENTANDO)

**Tarefa:** ML-1C do roadmap Python CLI nativo — `validator.py` com wip-limit, stale-wip, req-adr em paridade com `npm/src/validator/index.js`.

**Branch:** `feat/v2.2-python-cli-nativo`

**Arquivos a criar:**
- `pypi/trackfw/validator.py` — espelho Python do validator JS (612 linhas)
- `pypi/tests/test_validator.py` — testes unittest

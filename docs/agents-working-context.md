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

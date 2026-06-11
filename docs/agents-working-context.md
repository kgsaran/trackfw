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
- Instalar GoReleaser: `brew install goreleaser`
- Criar `.goreleaser.yaml` com targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- Criar GitHub Actions workflow: `.github/workflows/release.yml` (trigger: `push tag v*`)
- Testar release local: `goreleaser release --snapshot --clean`

### Prioridade 3 — Wrapper npm
- Criar `npm/package.json` com `bin: { trackfw: "./bin/trackfw" }`
- Script `postinstall` que baixa o binário correto para a plataforma
- Publicar no npm como `trackfw`

### Prioridade 4 — Wrapper PyPI
- Criar `pypi/` com `setup.py` / `pyproject.toml`
- Script de instalação que baixa o binário
- Publicar no PyPI como `trackfw`

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

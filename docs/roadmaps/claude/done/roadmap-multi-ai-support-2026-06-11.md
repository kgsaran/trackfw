# Roadmap: Suporte Multi-AI — Subcomandos por Ferramenta

> Criado em: 2026-06-11 | Status: ✅ Done
> REQ: `docs/requisições/claude/backlog/REQ-multi-ai-support-2026-06-11.md`

## Diagnóstico / Contexto

O trackfw já suporta Claude Code (`trackfw agents`, `trackfw skills`) com os 10 agentes especializados. O usuário quer a mesma funcionalidade para Gemini CLI, Cursor, GitHub Copilot, Windsurf e Amazon Q Developer.

Formatos confirmados via documentação oficial:
- **Gemini CLI**: `~/.gemini/GEMINI.md` (global) + `GEMINI.md` (projeto) + `~/.gemini/skills/*/SKILL.md` + `~/.gemini/commands/*.toml`
- **Cursor**: `.cursor/rules/*.mdc` com frontmatter `alwaysApply/globs/description`
- **GitHub Copilot**: `.github/copilot-instructions.md` + `.github/instructions/*.instructions.md` (com `applyTo`) + `.github/prompts/*.prompt.md`
- **Windsurf**: `.windsurf/rules/*.md` com frontmatter `trigger` + `.windsurf/workflows/*.md` + global `~/.codeium/windsurf/memories/global_rules.md`
- **Amazon Q**: `.amazonq/rules/*.md` (Markdown puro, sem frontmatter)

---

## Wave 1 — Templates de conteúdo (5 MLs em paralelo)

> Dependências: Independente — podem ser escritos em paralelo

### ML-1A — Templates Gemini CLI
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/templates/gemini/GEMINI.md`
- `internal/generators/templates/gemini/GEMINI-project.md`
- `internal/generators/templates/gemini/skills/trackfw-architect/SKILL.md`
- `internal/generators/templates/gemini/skills/trackfw-backend/SKILL.md`
- `internal/generators/templates/gemini/skills/trackfw-frontend/SKILL.md`
- `internal/generators/templates/gemini/skills/trackfw-qa/SKILL.md`
- `internal/generators/templates/gemini/skills/trackfw-infra/SKILL.md`
- `internal/generators/templates/gemini/skills/trackfw-security/SKILL.md`
- `internal/generators/templates/gemini/skills/trackfw-code-quality/SKILL.md`
- `internal/generators/templates/gemini/skills/trackfw-dba/SKILL.md`
- `internal/generators/templates/gemini/skills/trackfw-ux/SKILL.md`
- `internal/generators/templates/gemini/skills/trackfw-data/SKILL.md`
- `internal/generators/templates/gemini/commands/trackfw-adr.toml`
- `internal/generators/templates/gemini/commands/trackfw-req.toml`
- `internal/generators/templates/gemini/commands/trackfw-roadmap.toml`

**Ações:**
- `GEMINI.md` global: governança trackfw (ADR→REQ→ROADMAP→backlog/wip/done), stack, comandos, regras de branch
- `GEMINI-project.md`: versão compacta para projeto (sem regras globais de workflow)
- `SKILL.md` de cada role: frontmatter `name`, `description`, `signature`, `capabilities`, `tools`; conteúdo baseado nos agents Claude equivalentes, limpo de refs CMDB/pessoais
- `commands/*.toml`: campos `prompt` e `description`; ex: `trackfw-adr.toml` cria ADR via `/trackfw-adr`

**Critérios de aceite:**
- [ ] Todos os SKILL.md começam com frontmatter YAML válido
- [ ] Nenhum arquivo contém "CMDB", "KG", "Kleber", nomes mitológicos
- [ ] TOML commands têm campos `prompt` e `description`

---

### ML-1B — Templates Cursor
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/templates/cursor/trackfw-architect.mdc`
- `internal/generators/templates/cursor/trackfw-backend.mdc`
- `internal/generators/templates/cursor/trackfw-frontend.mdc`
- `internal/generators/templates/cursor/trackfw-qa.mdc`
- `internal/generators/templates/cursor/trackfw-infra.mdc`
- `internal/generators/templates/cursor/trackfw-security.mdc`
- `internal/generators/templates/cursor/trackfw-code-quality.mdc`
- `internal/generators/templates/cursor/trackfw-dba.mdc`
- `internal/generators/templates/cursor/trackfw-ux.mdc`
- `internal/generators/templates/cursor/trackfw-data.mdc`

**Ações:**
- Cada arquivo `.mdc` começa com frontmatter:
  ```
  ---
  description: "<Role> — <especialidade curta>"
  alwaysApply: false
  ---
  ```
- Corpo: especialidade, responsabilidades, ferramentas preferidas, critérios de qualidade
- Baseado nos agents Claude equivalentes, adaptado para Cursor (sem `name:`, sem `model:`, sem `tools:` list)

**Critérios de aceite:**
- [ ] Todos começam com `---` (frontmatter válido)
- [ ] `alwaysApply: false` em todos (modo Agent Requested)
- [ ] `description:` presente em todos
- [ ] Sem refs CMDB/pessoais/mitológicas

---

### ML-1C — Templates GitHub Copilot
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/templates/copilot/copilot-instructions.md`
- `internal/generators/templates/copilot/instructions/trackfw-architect.instructions.md`
- (+ 9 outros arquivos `.instructions.md`)
- `internal/generators/templates/copilot/prompts/trackfw-architect.prompt.md`
- (+ 9 outros arquivos `.prompt.md`)

**Ações:**
- `copilot-instructions.md`: visão geral de governança trackfw, cadeia ADR→REQ→ROADMAP, stack e padrões do projeto; ≤2 páginas (limitação do Copilot)
- Cada `.instructions.md`: frontmatter `applyTo: "**"` + conteúdo da especialidade
- Cada `.prompt.md`: frontmatter `agent: 'agent'` + `description: ...` + prompt detalhado da especialidade

**Critérios de aceite:**
- [ ] `copilot-instructions.md` sem frontmatter (plain markdown)
- [ ] Cada `.instructions.md` tem `applyTo: "**"` no frontmatter
- [ ] Cada `.prompt.md` tem `agent: 'agent'` e `description:` no frontmatter
- [ ] Sem refs proibidas

---

### ML-1D — Templates Windsurf
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/templates/windsurf/rules/trackfw-architect.md`
- (+ 9 outros `.md` files para regras)
- `internal/generators/templates/windsurf/workflows/trackfw-adr.md`
- `internal/generators/templates/windsurf/workflows/trackfw-req.md`
- `internal/generators/templates/windsurf/workflows/trackfw-implement.md`
- `internal/generators/templates/windsurf/global_rules_append.md`

**Ações:**
- Cada rule tem frontmatter:
  ```
  ---
  trigger: model_decision
  ---
  ```
- `global_rules_append.md`: trecho para append no arquivo global do Windsurf (separado por `---`)
- Workflows: formato Markdown com título, descrição e passos sequenciais; invocados via `/trackfw-adr`

**Critérios de aceite:**
- [ ] Todos os rules têm `trigger: model_decision` no frontmatter
- [ ] Workflows têm título H1 e passos numerados
- [ ] `global_rules_append.md` existe e é compacto (≤500 chars)

---

### ML-1E — Templates Amazon Q
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/templates/amazonq/trackfw-architect.md`
- (+ 9 outros `.md` files)

**Ações:**
- Markdown puro, sem frontmatter (requisito Amazon Q)
- Conteúdo: especialidade, responsabilidades, padrões técnicos; baseado nos agents Claude equivalentes

**Critérios de aceite:**
- [ ] Nenhum arquivo começa com `---` (sem frontmatter)
- [ ] Sem refs proibidas

---

## Wave 2 — Generators Go (5 MLs em paralelo, dependem da Wave 1)

> Dependências: Todos os templates da Wave 1

### ML-2A — Generator Gemini (`internal/generators/gemini.go`)
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/gemini.go` (novo)
- `internal/generators/gemini_test.go` (novo)

**Ações:**
- Struct `GeminiOptions` (se necessário)
- `InstallGemini() error`:
  - Instala `~/.gemini/GEMINI.md` (se não existir)
  - Instala `GEMINI.md` no `$PWD` (se não existir)
  - Instala cada skill em `~/.gemini/skills/trackfw-<role>/SKILL.md`
  - Instala cada command em `~/.gemini/commands/trackfw-*.toml`
- `//go:embed templates/gemini` + `embed.FS`
- Mesmo padrão idempotente de `agents.go`

**Testes:**
- `TestInstallGemini_CriaArquivos`: verifica existência de SKILL.md e commands
- `TestInstallGemini_Idempotente`: segunda chamada não sobrescreve arquivo customizado
- `TestInstallGemini_SkillsTemFrontmatter`: cada SKILL.md começa com frontmatter válido

**Critérios de aceite:**
- [ ] `go test ./internal/generators/ -run TestInstallGemini` verde
- [ ] `go build ./...` sem erros

---

### ML-2B — Generator Cursor (`internal/generators/cursor.go`)
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/cursor.go` (novo)
- `internal/generators/cursor_test.go` (novo)

**Ações:**
- `InstallCursor() error`:
  - Cria `.cursor/rules/` no `$PWD` se não existir
  - Instala cada `.mdc` em `.cursor/rules/trackfw-<role>.mdc`
  - Idempotente
- `//go:embed templates/cursor`

**Testes:**
- `TestInstallCursor_CriaArquivos`
- `TestInstallCursor_Idempotente`
- `TestInstallCursor_ConteudoComFrontmatter`

**Critérios de aceite:**
- [ ] `go test ./internal/generators/ -run TestInstallCursor` verde

---

### ML-2C — Generator Copilot (`internal/generators/copilot.go`)
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/copilot.go` (novo)
- `internal/generators/copilot_test.go` (novo)

**Ações:**
- `InstallCopilot() error`:
  - Cria `.github/` se não existir
  - Instala `copilot-instructions.md`
  - Instala cada `.instructions.md` em `.github/instructions/`
  - Instala cada `.prompt.md` em `.github/prompts/`
  - Idempotente
- `//go:embed templates/copilot`

**Testes:** mesma tripla de testes (cria/idempotente/frontmatter)

**Critérios de aceite:**
- [ ] `go test ./internal/generators/ -run TestInstallCopilot` verde

---

### ML-2D — Generator Windsurf (`internal/generators/windsurf.go`)
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/windsurf.go` (novo)
- `internal/generators/windsurf_test.go` (novo)

**Ações:**
- `InstallWindsurf() error`:
  - Instala rules em `.windsurf/rules/`
  - Instala workflows em `.windsurf/workflows/`
  - Para o global rules: append de `global_rules_append.md` em `~/.codeium/windsurf/memories/global_rules.md` SE não contiver `trackfw` ainda (não sobrescreve)
- `//go:embed templates/windsurf`

**Critérios de aceite:**
- [ ] `go test ./internal/generators/ -run TestInstallWindsurf` verde
- [ ] Segunda execução não duplica conteúdo em `global_rules.md`

---

### ML-2E — Generator Amazon Q (`internal/generators/amazonq.go`)
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/amazonq.go` (novo)
- `internal/generators/amazonq_test.go` (novo)

**Ações:**
- `InstallAmazonQ() error`:
  - Cria `.amazonq/rules/` no `$PWD`
  - Instala cada `.md` idempotente
- `//go:embed templates/amazonq`

**Critérios de aceite:**
- [ ] `go test ./internal/generators/ -run TestInstallAmazonQ` verde

---

## Wave 3 — Comandos CLI (dependem da Wave 2)

> Dependências: Todos os generators da Wave 2

### ML-3A — Comandos CLI para as 5 ferramentas
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/commands/gemini.go` (novo)
- `internal/commands/cursor.go` (novo)
- `internal/commands/copilot.go` (novo)
- `internal/commands/windsurf.go` (novo)
- `internal/commands/amazonq.go` (novo)
- `internal/commands/root.go` (modificar — adicionar os 5 novos comandos)

**Ações por arquivo:**
- Padrão idêntico ao `internal/commands/agents.go`
- `Use`, `Short`, `Long` descrevendo o que instala e onde
- `RunE` chama o generator correspondente
- Em `root.go`: adicionar `newGeminiCmd()`, `newCursorCmd()`, `newCopilotCmd()`, `newWindsurfCmd()`, `newAmazonQCmd()` ao `AddCommand`

**Critérios de aceite:**
- [ ] `trackfw gemini --help` mostra o Long description correto
- [ ] `trackfw cursor --help` idem
- [ ] `go build ./...` sem erros

---

## Wave 4 — Extensão do init wizard (depende da Wave 3)

> Dependências: Wave 3 completa

### ML-4A — init wizard: seleção de ferramentas de IA
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/commands/init.go` (modificar)
- `internal/generators/scaffold.go` (modificar se necessário)

**Ações:**
- Adicionar etapa no wizard `huh` após as perguntas atuais:
  - Campo `huh.NewMultiSelect` com título "Which AI assistants do you use?"
  - Opções: Claude Code (padrão selecionado), Gemini CLI, Cursor, GitHub Copilot, Windsurf, Amazon Q
- Para cada ferramenta selecionada, chamar o generator correspondente
- Claude Code chama `generators.InstallAgents()` (já existente)
- Retrocompatível: se nenhuma ferramenta selecionada além do Claude, comportamento atual mantido

**Critérios de aceite:**
- [ ] `trackfw init` continua funcionando sem selecionar nada
- [ ] Selecionar Cursor gera `.cursor/rules/*.mdc` no diretório atual
- [ ] `go build ./...` sem erros

---

## Comandos de validação global

```bash
go build ./...
go test ./...
trackfw --help   # deve listar gemini, cursor, copilot, windsurf, amazonq
```

## Legenda

- ⬜ Pendente
- 🔄 Em andamento
- ✅ Concluído
- ❌ Bloqueado

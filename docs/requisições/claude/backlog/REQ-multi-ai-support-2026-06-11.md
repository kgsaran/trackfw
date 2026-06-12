# REQ: Suporte Multi-AI — Subcomandos por Ferramenta

> Criado em: 2026-06-11 | Status: Backlog | Agente: Zeus (Arquiteto)

## Solicitação

Estender o trackfw para suportar as principais ferramentas de IA além do Claude Code, com subcomandos dedicados por ferramenta. O objetivo é instalar os equivalentes dos 10 agentes especializados (architect, backend, frontend, qa, infra, security, code-quality, dba, ux, data) no formato nativo de cada ferramenta, além de adaptar o `trackfw init` para gerar o arquivo de instruções de cada ferramenta.

## Ferramentas alvo

| Ferramenta | Subcomando | Arquivo principal | Formato |
|---|---|---|---|
| Gemini CLI | `trackfw gemini` | `~/.gemini/GEMINI.md` + `GEMINI.md` (projeto) + `~/.gemini/skills/` | Markdown puro + TOML commands |
| Cursor | `trackfw cursor` | `.cursor/rules/*.mdc` | Markdown + frontmatter YAML |
| GitHub Copilot | `trackfw copilot` | `.github/copilot-instructions.md` + `.github/instructions/*.instructions.md` + `.github/prompts/*.prompt.md` | Markdown puro |
| Windsurf | `trackfw windsurf` | `.windsurf/rules/*.md` + `.windsurf/workflows/*.md` | Markdown + frontmatter YAML |
| Amazon Q | `trackfw amazonq` | `.amazonq/rules/*.md` | Markdown puro |

## Escopo detalhado por ferramenta

### trackfw gemini
- Instala `~/.gemini/GEMINI.md` com instruções de governança (se não existir)
- Instala `GEMINI.md` no projeto corrente (se não existir)
- Instala 10 skills em `~/.gemini/skills/trackfw-<role>/SKILL.md` seguindo o padrão agentskills.io
- Instala custom commands em `~/.gemini/commands/trackfw-*.toml` com prompts de governança
- Idempotente: arquivos existentes não são sobrescritos

### trackfw cursor
- Instala 10 arquivos `.cursor/rules/trackfw-<role>.mdc` no projeto corrente
- Frontmatter: `alwaysApply: false` + `description: <papel>` (modo "Agent Requested")
- Idempotente

### trackfw copilot
- Instala `.github/copilot-instructions.md` com instruções consolidadas de governança
- Instala 10 arquivos `.github/instructions/trackfw-<role>.instructions.md` com `applyTo: "**"`
- Instala 10 arquivos `.github/prompts/trackfw-<role>.prompt.md` para uso manual
- Idempotente

### trackfw windsurf
- Instala 10 arquivos `.windsurf/rules/trackfw-<role>.md` com frontmatter `trigger: model_decision`
- Instala workflows de governança em `.windsurf/workflows/trackfw-*.md`
- Instala `~/.codeium/windsurf/memories/global_rules.md` (append se já existir, não sobrescreve)
- Idempotente

### trackfw amazonq
- Instala 10 arquivos `.amazonq/rules/trackfw-<role>.md` (Markdown puro, sem frontmatter)
- Idempotente

## Extensão do trackfw init

O wizard `trackfw init` deve ganhar uma etapa adicional:
- "Which AI assistants do you use?" (multi-select)
- Para cada ferramenta selecionada, chamar o generator correspondente
- Continua funcionando sem selecionar nenhuma (backwards-compatible)

## Extensão do trackfw validate / status

- `trackfw validate` deve verificar presença dos arquivos de instrução das ferramentas detectadas no projeto
- `trackfw status` deve mostrar quais ferramentas estão configuradas

## Restrições

- **Idempotência obrigatória** em todos os instaladores (mesmo padrão de `trackfw agents`)
- **Sem sobrescrita de customizações do usuário** — se o arquivo existe, skip
- **Exceção Windsurf global rules**: fazer append com separador `---` em vez de sobrescrever
- Comandos operam em `$PWD` para arquivos de projeto (`.cursor/`, `.github/`, etc.)
- Conteúdo dos templates: limpo de referências CMDB, pessoais e mitológicas (mesmo padrão dos agentes Claude)

## Critérios de aceite

- [ ] `trackfw gemini` instala GEMINI.md + 10 skills + commands; roda 2x sem erro
- [ ] `trackfw cursor` instala 10 `.mdc` files no `.cursor/rules/` do diretório corrente
- [ ] `trackfw copilot` instala copilot-instructions.md + 10 instructions + 10 prompts
- [ ] `trackfw windsurf` instala 10 rules + workflows; não sobrescreve global_rules.md
- [ ] `trackfw amazonq` instala 10 rules em `.amazonq/rules/`
- [ ] `trackfw init` wizard oferece seleção de ferramentas
- [ ] `go test ./...` verde para todos os novos generators
- [ ] `go build ./...` sem erros

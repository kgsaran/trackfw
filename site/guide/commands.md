# Referência de Comandos

Referência completa de todos os comandos do `trackfw`.

---

## `trackfw init`

Inicializa a estrutura de governança no projeto atual via wizard interativo.

```bash
trackfw init [--brownfield] [--ai-tools codex,...]
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--brownfield` | Ativa modo lenient por 30 dias (violações viram warnings) |
| `--ai-tools` | Configura os nove targets de IA em modo não interativo nos três runtimes |

### O que é gerado

- `docs/adr/`, `docs/req/`, `docs/roadmaps/{backlog,wip,blocked,done,abandoned}/`
- `trackfw.yaml` — configuração do projeto
- `scripts/trackfw-validate.sh` — script de validação para CI
- `CLAUDE.md` — contexto para Claude Code (se selecionado)
- `.claude/commands/` — 7 slash commands para Claude Code
- `AGENTS.md`, `.agents/skills/`, `.codex/agents/` e `.codex/hooks.json` — integração Codex (se selecionado)
- `.husky/` ou `lefthook.yml` — git hooks (se selecionado)
- `.github/workflows/trackfw.yml` ou `.gitlab-ci.yml` (se selecionado)
- `pom.xml` Spring Boot 3.3 (se backend=java)

### Exemplo

```bash
$ trackfw init
? Project name: meu-projeto
? Project type: fullstack
? Backend language: java
? Backend framework: Spring Boot
? Git hooks: husky
? CI/CD: GitHub Actions
? AI assistants: Claude Code

✓ Governance structure initialized.
```

---

## `trackfw agents` e `trackfw skills`

Gerenciam agents especialistas e skills de governança com o mesmo contrato nos
CLIs Go/Homebrew, npm e PyPI.

```bash
trackfw agents list|install|uninstall|update [flags]
trackfw skills list|install|uninstall|update [flags]
```

Targets suportados: `claude`, `codex`, `gemini`, `antigravity`, `cursor`,
`copilot`, `windsurf`, `amazonq` e `kiro`.

### Flags

| Flag | Descrição |
|---|---|
| `--targets <csv>` | CLIs de destino; obrigatório para mutações sem TTY |
| `--items <csv>` | IDs do catálogo; o padrão é todos os items |
| `--scope project\|global` | Instala no projeto ou no diretório do usuário |
| `--surface target=surface` | Seleciona uma surface específica; pode ser repetido |
| `--json` | Emite catálogo e deployments em formato determinístico |
| `--force` | Permite substituir/remover conteúdo gerenciado modificado |

Em TTY, `install`, `update` e `uninstall` sem `--targets` abrem uma seleção
interativa. Em CI ou outro ambiente não interativo, omitir `--targets` é erro.

### Exemplos

```bash
# Lista catálogo, formato nativo/fallback e estado; inclui surfaces legadas
trackfw agents list --json

# Instala agents e skills selecionados no projeto
trackfw agents install --targets claude,codex --items architect,backend --scope project
trackfw skills install --targets gemini,kiro --items governance,implement --scope project

# Instala globalmente e seleciona a surface CLI do Kiro
trackfw agents install --targets kiro --scope global --surface kiro=cli

# Inspeciona a surface antiga do Antigravity explicitamente
trackfw agents list --targets antigravity --surface antigravity=legacy-cli

# Atualiza ou remove apenas deployments selecionados
trackfw skills update --targets codex,gemini
trackfw agents uninstall --targets claude --items backend
```

Os estados são `not-installed`, `current`, `outdated` e `modified`. O manifesto
`.trackfw/integrations-manifest.json`, separado por scope, registra ownership,
versão, SHA-256 e claims compartilhados. Arquivos `modified` são preservados por
`update` e `uninstall` até o uso explícito de `--force`. Uninstall nunca remove
arquivo unmanaged nem artefato ainda compartilhado. Uma instalação histórica
com hash conhecido é adotada sem overwrite e aparece como `outdated`; `update`
faz a migração. Conteúdo unmanaged desconhecido nunca é adotado por update,
mesmo com `--force`.

Os comandos standalone `gemini`, `cursor`, `copilot`, `windsurf` e `amazonq`
permanecem apenas como aliases de compatibilidade e delegam a este lifecycle.

---

## `trackfw adr new`

Cria um novo Architecture Decision Record via wizard interativo.

```bash
trackfw adr new
```

### Saída esperada

```
created docs/adr/ADR-2026-06-13-titulo-da-decisao.md
```

---

## `trackfw adr list`

Lista todos os ADRs do projeto com status.

```bash
trackfw adr list
```

### Saída esperada

```
ADR-2026-06-13-usar-postgresql.md         Proposed
ADR-2026-06-10-arquitetura-monolito.md    Accepted
ADR-2026-06-01-provider-oauth.md          Draft
```

---

## `trackfw req new`

Cria um novo requisito via wizard interativo com probes contextuais.

```bash
trackfw req new
```

O wizard detecta domínios (autenticação, UI, persistência, API, deploy, eventos) com base no título e motivação e apresenta perguntas específicas por domínio. ADR Drafts são criados automaticamente quando a resposta indica decisão arquitetural pendente.

### Saída esperada

```
created docs/req/REQ-2026-06-13-login-via-oauth.md
created docs/adr/ADR-2026-06-13-provider-oauth.md (Draft)
```

---

## `trackfw req list`

Lista todos os requisitos com status.

```bash
trackfw req list
```

### Saída esperada

```
REQ-2026-06-13-login-via-oauth.md      Open
REQ-2026-06-10-exportar-relatorio.md   Closed
```

---

## `trackfw roadmap new`

Cria um novo roadmap de implementação.

```bash
trackfw roadmap new [--title "Título"] [--req docs/req/REQ-*.md] [--from-req docs/req/REQ-*.md]
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--title "Título"` | Define título sem wizard |
| `--req <path>` | Vincula REQ ao roadmap |
| `--from-req <path>` | Cria roadmap já vinculado à REQ (shorthand) |

### Exemplos

```bash
# Wizard interativo
trackfw roadmap new

# Com título e REQ definidos
trackfw roadmap new --title "Implementar OAuth" --req docs/req/REQ-2026-06-13-login-via-oauth.md

# Shorthand
trackfw roadmap new --from-req docs/req/REQ-2026-06-13-login-via-oauth.md
```

---

## `trackfw roadmap list`

Lista todos os roadmaps agrupados por estado.

```bash
trackfw roadmap list
```

### Saída esperada

```
[backlog]  ROADMAP-2026-06-13-implementar-oauth.md
[wip]      ROADMAP-2026-06-10-refactor-db.md
[done]     ROADMAP-2026-06-01-setup-ci.md
```

---

## `trackfw roadmap move`

Move um roadmap entre estados do kanban.

```bash
trackfw roadmap move <nome-parcial> <estado>
```

Estados válidos: `backlog`, `wip`, `blocked`, `done`, `abandoned`

### Exemplo

```bash
trackfw roadmap move oauth wip
# ✓ moved ROADMAP-2026-06-13-implementar-oauth.md → docs/roadmaps/wip
```

A transição é registrada automaticamente em `docs/roadmaps/.trackfw-log`.

---

## `trackfw roadmap show`

Exibe o conteúdo completo de um roadmap com busca parcial por nome.

```bash
trackfw roadmap show <nome-parcial>
```

### Exemplo

```bash
trackfw roadmap show oauth
```

```
─────────────────────────────────────────
ROADMAP-2026-06-13-implementar-oauth.md — [WIP]
─────────────────────────────────────────

---
status: wip
date: 2026-06-13
req: docs/req/REQ-2026-06-13-login-via-oauth.md
squad: ""
---

# Roadmap: Implementar OAuth
...

Location: docs/roadmaps/wip/ROADMAP-2026-06-13-implementar-oauth.md
```

---

## `trackfw validate`

Valida a consistência entre ADRs, REQs e Roadmaps do projeto.

```bash
trackfw validate
```

### Regras validadas

1. Roadmaps em WIP devem ter campo REQ preenchido
2. Roadmaps em WIP devem ter critérios de aceite
3. Somente um roadmap pode estar em WIP por vez (configurável por squad)
4. Roadmaps em Blocked devem ter campo REQ preenchido
5. REQs devem ter Roadmap vinculado
6. ADRs devem ser referenciados em pelo menos uma REQ
7. REQs Open não podem estar bloqueadas por ADRs em Draft
8. Roadmaps em WIP há mais de 7 dias são marcados como stale
9. ADRs e REQs devem ter frontmatter YAML válido

### Saída esperada — sem violações

```
✓ No violations found.
```

### Saída esperada — com problemas

```
✗ 2 violation(s) found:

  [violation] ROADMAP-2026-06-13-implementar-oauth.md missing REQ field
  [violation] REQ-2026-06-13-login-via-oauth.md is blocked by Draft ADR: ADR-2026-06-13-provider-oauth.md

⚠  1 warning(s):

  [warning] ROADMAP-2026-06-10-refactor-db.md in WIP for 9 days (stale)
```

### Modo lenient (brownfield)

```
[LENIENT MODE until 2026-07-13]

⚠  1 violation treated as warning:
  [warning] ROADMAP-2026-06-13-implementar-oauth.md missing REQ field
```

---

## `trackfw status`

Exibe visão geral do estado atual do projeto.

```bash
trackfw status
```

### Saída esperada

```
trackfw — project status

📋 Backlog       2 roadmaps
🔄 WIP           1 roadmap
⚠  Stale WIP     0 roadmaps
❌ Blocked       0 roadmaps
✅ Done          3 roadmaps

📄 ADRs          4   (Proposed: 2, Accepted: 1, Draft: 1)
📝 REQs          3   (Open: 2, Closed: 1)

⏳ REQs blocked by Draft ADRs:
  REQ-2026-06-13-login-via-oauth.md → ADR-2026-06-13-provider-oauth.md
```

---

## `trackfw context`

Emite o contexto de governança do projeto para consumo por LLMs e agentes de IA.

```bash
trackfw context [--format=md|json]
```

### Flags

| Flag | Descrição | Padrão |
|------|-----------|--------|
| `--format` | Formato de saída: `md` ou `json` | `md` |

### Exemplo — formato JSON

```bash
trackfw context --format=json
```

```json
{
  "project": "meu-projeto",
  "governance_score": 80,
  "adrs": [
    { "file": "ADR-2026-06-13-usar-postgresql.md", "status": "Accepted" }
  ],
  "reqs": [
    { "file": "REQ-2026-06-13-login-via-oauth.md", "status": "Open" }
  ],
  "roadmaps": {
    "wip": ["ROADMAP-2026-06-13-implementar-oauth.md"]
  },
  "violations": [],
  "warnings": []
}
```

---

## `trackfw serve`

Inicia um servidor HTTP local com visualização web da cadeia ADR → REQ → ROADMAP.

```bash
trackfw serve [--port 8080]
```

### Flags

| Flag | Descrição | Padrão |
|------|-----------|--------|
| `--port` | Porta do servidor | `8080` |

Acesse `http://localhost:8080` para ver:
- **Traceability** — mapa de rastreabilidade ADR → REQ → ROADMAP
- **Timeline** — linha do tempo de transições
- **Kanban** — board visual dos roadmaps por estado

---

## `trackfw metrics`

Calcula métricas de fluxo a partir do histórico de transições (`.trackfw-log`).

```bash
trackfw metrics [--since YYYY-MM-DD] [--export relatorio.csv]
```

### Flags

| Flag | Descrição |
|------|-----------|
| `--since` | Data de início do período (ex: `2026-01-01`) |
| `--export` | Exporta métricas para CSV |

### Métricas calculadas

- **Cycle time** — tempo médio de backlog → done
- **Throughput** — roadmaps concluídos por semana
- **WIP age** — tempo médio dos roadmaps em WIP

### Saída esperada

```
Metrics (2026-01-01 → 2026-06-13)

Cycle time:    4.2 days (avg)
Throughput:    2.1 roadmaps/week
WIP age:       3 days (avg)
```

---

## `trackfw sync`

Sincroniza REQs abertas com ferramentas de issue tracking externas.

```bash
trackfw sync --to=linear
trackfw sync --to=jira
```

### Flags

| Flag | Descrição | Valores |
|------|-----------|---------|
| `--to` | Destino da sincronização | `linear`, `jira` |

### Configuração — Linear

Em `trackfw.yaml` ou via variáveis de ambiente:

```yaml
linear_api_key: "lin_api_..."
linear_team_id: "TEAM_ID"
```

Ou:
```bash
export LINEAR_API_KEY="lin_api_..."
export LINEAR_TEAM_ID="TEAM_ID"
```

### Configuração — Jira

```yaml
jira_base_url: "https://empresa.atlassian.net"
jira_email: "usuario@empresa.com"
jira_token: "ATATT..."
jira_project: "PROJ"
```

Ou:
```bash
export JIRA_BASE_URL="https://empresa.atlassian.net"
export JIRA_EMAIL="usuario@empresa.com"
export JIRA_TOKEN="ATATT..."
export JIRA_PROJECT="PROJ"
```

### Saída esperada

```
REQ-2026-06-13-login-via-oauth.md → LIN-42 (created)
REQ-2026-06-10-exportar-relatorio.md → already synced (skipped)
```

---

## `trackfw log`

Exibe o histórico de transições de estado dos roadmaps.

```bash
trackfw log [--tail N]
```

### Flags

| Flag | Descrição | Padrão |
|------|-----------|--------|
| `--tail` | Número de entradas a exibir | `20` |

### Saída esperada

```
Date                 Roadmap                                            From       To
2026-06-13 14:32     ROADMAP-2026-06-13-implementar-oauth.md           backlog  → wip
2026-06-12 09:15     ROADMAP-2026-06-10-refactor-db.md                 wip      → done
```

---

## `trackfw plugins`

Gerencia plugins do trackfw.

```bash
trackfw plugins list
trackfw plugins add <repo>
trackfw plugins remove <nome>
trackfw plugins search <keyword>
```

### Subcomandos

| Subcomando | Descrição |
|------------|-----------|
| `list` | Lista plugins instalados em `~/.trackfw/plugins/` |
| `add <repo>` | Instala plugin das GitHub Releases (formato `user/name[@tag]`) |
| `remove <nome>` | Remove plugin instalado |
| `search <keyword>` | Busca plugins no registry oficial |

### Exemplo

```bash
# Buscar plugins disponíveis
trackfw plugins search metrics

# Instalar plugin
trackfw plugins add kgsaran/trackfw-metrics

# Usar plugin instalado
trackfw metrics-extended --since 2026-01-01
```

---

## `trackfw version`

Exibe a versão instalada do trackfw.

```bash
trackfw version
# trackfw v2.1.0
```

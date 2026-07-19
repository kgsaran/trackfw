# trackfw para Agentes de IA

O `trackfw` foi desenhado desde o início para ser nativo ao fluxo de trabalho com agentes de IA. Esta página explica como aproveitar essa integração ao máximo.

---

## Por que agentes de IA precisam de governança estruturada

Agentes de IA como Claude Code, OpenAI Codex, Gemini CLI, Antigravity, Cursor, GitHub Copilot, Windsurf, Amazon Q e Kiro operam em sessões independentes. Sem um registro persistente de decisões e contexto, cada sessão começa do zero — o agente não sabe:

- Quais decisões arquiteturais já foram tomadas (e por quê)
- Quais requisitos estão em andamento
- Quais roadmaps estão em execução e em que estado
- O que foi discutido e decidido em sessões anteriores

O resultado é redundância, inconsistência e regressões: o agente refaz investigações já feitas, toma decisões que contradizem ADRs existentes, ou implementa features que conflitam com roadmaps ativos.

O `trackfw` resolve isso com artefatos Markdown estruturados que qualquer agente pode ler, interpretar e atualizar em tempo de execução.

---

## Frontmatter YAML — parseable por LLMs

Todos os artefatos gerados pelo `trackfw` incluem um bloco de frontmatter YAML no início do arquivo:

### ADR (Architecture Decision Record)

```markdown
---
status: Accepted
date: 2026-06-13
author: "Kleber Saran"
---

# ADR: Usar PostgreSQL como banco principal
...
```

### REQ (Requirement)

```markdown
---
status: Open
date: 2026-06-13
author: ""
adr: ADR-2026-06-13-usar-postgresql.md
roadmap: ""
---

# REQ: Login via OAuth
...
```

### ROADMAP

```markdown
---
status: wip
date: 2026-06-13
req: docs/req/REQ-2026-06-13-login-via-oauth.md
squad: backend
---

# Roadmap: Implementar OAuth
...
```

O frontmatter é validado contra JSON Schemas em `docs/schema/` — ADRs, REQs e Roadmaps com campos obrigatórios ausentes geram violações no `trackfw validate`.

---

## `trackfw context --format=json`

O comando `context` agrega todo o estado de governança do projeto em um único output estruturado, otimizado para ser incluído em prompts de LLM.

```bash
trackfw context --format=json
```

### Output de exemplo

```json
{
  "project": "meu-projeto",
  "governance_score": 80,
  "adrs": [
    {
      "file": "ADR-2026-06-13-usar-postgresql.md",
      "status": "Accepted"
    },
    {
      "file": "ADR-2026-06-13-provider-oauth.md",
      "status": "Draft"
    }
  ],
  "reqs": [
    {
      "file": "REQ-2026-06-13-login-via-oauth.md",
      "status": "Open"
    }
  ],
  "roadmaps": {
    "backlog": [],
    "wip": ["ROADMAP-2026-06-13-implementar-oauth.md"],
    "blocked": [],
    "done": ["ROADMAP-2026-06-01-setup-ci.md"]
  },
  "violations": [],
  "warnings": [
    "ADR-2026-06-13-provider-oauth.md is Draft — REQ may be blocked"
  ]
}
```

### Como usar em prompts

```bash
# Inclua o contexto diretamente no prompt do agente
CONTEXT=$(trackfw context --format=json)

# Exemplo com curl para API Anthropic
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d "{
    \"model\": \"claude-opus-4-5\",
    \"max_tokens\": 4096,
    \"messages\": [{
      \"role\": \"user\",
      \"content\": \"Contexto de governança do projeto: $CONTEXT\n\nCrie um novo roadmap para implementar o login OAuth.\"
    }]
  }"
```

### Formato Markdown (para leitura humana)

```bash
trackfw context --format=md
```

```markdown
# Contexto de Governança — meu-projeto

**Governance Score:** 80/100

## ADRs (2)
- ADR-2026-06-13-usar-postgresql.md — Accepted
- ADR-2026-06-13-provider-oauth.md — Draft

## REQs (1)
- REQ-2026-06-13-login-via-oauth.md — Open

## Roadmaps
- **WIP:** ROADMAP-2026-06-13-implementar-oauth.md
- **Done:** ROADMAP-2026-06-01-setup-ci.md

## Warnings
- ADR-2026-06-13-provider-oauth.md is Draft — REQ may be blocked
```

---

## `trackfw roadmap new --from-req` — geração assistida

O flag `--from-req` permite que um agente crie um roadmap diretamente vinculado a uma REQ existente, sem precisar do wizard interativo:

```bash
trackfw roadmap new --from-req docs/req/REQ-2026-06-13-login-via-oauth.md
```

O conteúdo da REQ é injetado no template do roadmap, dando ao agente o contexto necessário para preencher os microlotes de implementação.

**Fluxo típico com agente:**

```bash
# 1. Agente lê o contexto do projeto
trackfw context --format=json > /tmp/context.json

# 2. Agente cria um roadmap vinculado à REQ aberta
trackfw roadmap new --from-req docs/req/REQ-2026-06-13-login-via-oauth.md

# 3. Agente edita o roadmap gerado com os microlotes de implementação
# (o arquivo está em docs/roadmaps/backlog/)

# 4. Agente move para WIP ao iniciar implementação
trackfw roadmap move login-via-oauth wip

# 5. Agente valida ao final
trackfw validate
```

---

## JSON Schema em `docs/schema/`

O `trackfw init` gera schemas JSON para validação de artefatos em `docs/schema/`:

```
docs/schema/
├── adr.schema.json
├── req.schema.json
└── roadmap.schema.json
```

Agentes externos podem validar artefatos contra esses schemas antes de fazer commits:

```bash
# Com ajv-cli
npx ajv validate -s docs/schema/adr.schema.json -d docs/adr/ADR-2026-06-13-usar-postgresql.md

# Com jsonschema (Python)
python3 -m jsonschema -i docs/adr/ADR-2026-06-13-usar-postgresql.md docs/schema/adr.schema.json
```

### Schema ADR (`docs/schema/adr.schema.json`)

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["status", "date"],
  "properties": {
    "status": {
      "type": "string",
      "enum": ["Draft", "Proposed", "Accepted", "Deprecated", "Superseded"]
    },
    "date": {
      "type": "string",
      "pattern": "^[0-9]{4}-[0-9]{2}-[0-9]{2}$"
    },
    "author": { "type": "string" },
    "superseded_by": { "type": "string" }
  }
}
```

---

## Exemplo de prompt completo para Claude Code

Use o output de `trackfw context` como prefixo de sistema para garantir que o agente opere dentro das restrições de governança do projeto:

```markdown
Você é um agente de implementação especialista.

## Contexto de governança do projeto

[OUTPUT DE: trackfw context --format=md]

## Instruções

- Antes de implementar qualquer feature, verifique se existe uma REQ aberta cobrindo o escopo.
- Se não existir, crie primeiro com `trackfw req new`.
- Ao iniciar a implementação, mova o roadmap para WIP: `trackfw roadmap move <nome> wip`.
- Ao concluir cada microlote, atualize o roadmap e execute `trackfw validate`.
- Ao concluir toda a implementação, mova para Done: `trackfw roadmap move <nome> done`.
- Nunca tome decisões arquiteturais sem criar ou referenciar um ADR.

## Tarefa

[DESCRIÇÃO DA TAREFA]
```

---

## Integração com Claude Code via slash commands

Quando você seleciona **Claude Code** durante o `trackfw init`, os seguintes slash commands são gerados em `.claude/commands/`:

| Comando | Ação |
|---------|------|
| `/trackfw-context` | Emite contexto JSON atual e instrui o agente |
| `/trackfw-adr-new` | Wizard de novo ADR via agente |
| `/trackfw-req-new` | Wizard de nova REQ via agente |
| `/trackfw-roadmap-new` | Cria roadmap vinculado à REQ atual |
| `/trackfw-roadmap-wip` | Move roadmap atual para WIP |
| `/trackfw-roadmap-done` | Finaliza roadmap atual |
| `/trackfw-validate` | Valida e exibe violações |

### Uso no Claude Code

```
/trackfw-context
```

O agente lê automaticamente `trackfw context --format=json` e inclui o resultado no contexto antes de responder.

---

## Instalação e lifecycle multi-CLI

Agents e skills usam o mesmo contrato nos pacotes Go/Homebrew, npm e PyPI:

```bash
trackfw agents list
trackfw agents install --targets claude,codex --items architect,backend --scope project
trackfw agents update --targets claude,codex
trackfw agents uninstall --targets claude,codex

trackfw skills list --json
trackfw skills install --targets gemini,antigravity --items governance,implement
trackfw skills update --targets gemini,antigravity
trackfw skills uninstall --targets gemini,antigravity
```

Os targets são Claude Code, Codex, Gemini CLI, Antigravity, Cursor, GitHub
Copilot, Windsurf, Amazon Q e Kiro. As flags compartilhadas são:

| Flag | Uso |
|---|---|
| `--targets <csv>` | CLIs de destino; obrigatório em mutações sem TTY |
| `--items <csv>` | Agents ou skills específicos; o padrão é todos |
| `--scope project\|global` | Instalação no repositório ou no diretório do usuário |
| `--surface target=surface` | Seleciona uma surface, por exemplo `kiro=cli` |
| `--json` | Retorna catálogo, deployments, suporte, formato e estado |
| `--force` | Autoriza substituir/remover apenas conteúdo gerenciado modificado |

Em um terminal interativo, mutações sem `--targets` exibem uma seleção e targets
com mais de uma surface atual pedem a surface desejada. `list` inspeciona também
surfaces legadas. Para escolher a compatibilidade antiga explicitamente:

```bash
trackfw agents list --targets antigravity --surface antigravity=legacy-cli
```

Cada deployment fica em `not-installed`, `current`, `outdated` ou `modified`.
O manifesto `.trackfw/integrations-manifest.json` registra ownership, versão,
SHA-256 e claims compartilhados por escopo. `update` e `uninstall` preservam
customizações sem `--force`; uninstall nunca remove arquivo unmanaged. Templates
históricos conhecidos podem ser adotados sem overwrite e migrados com `update`.
Conteúdo desconhecido não é adotado por update, nem mesmo com `--force`.

---

## Gemini CLI

```bash
# Exportar contexto para uso com Gemini
trackfw context --format=json | gemini --model gemini-2.0-flash "Analise o estado de governança e sugira próximos passos."
```

---

## OpenAI Codex

Selecione **OpenAI Codex** no `trackfw init` ou use o lifecycle:

```bash
trackfw agents install --targets codex --scope project
trackfw skills install --targets codex --scope project
```

O adapter gera dez agentes especialistas TOML em `.codex/agents/` e cinco Agent
Skills em `.agents/skills/`, com ownership seguro e atualização conservadora.

---

## Cursor

Adicione ao `.cursorrules` do projeto:

```
Sempre execute `trackfw context --format=json` no início de cada sessão.
Nunca implemente features sem REQ aberta correspondente.
Atualize o roadmap ativo após cada microlote concluído.
Execute `trackfw validate` antes de cada commit.
```

---

## Integração com CI/CD

O `trackfw validate` retorna exit code 1 em violações — use como gate no CI:

### GitHub Actions

```yaml
- name: trackfw validate
  run: trackfw validate
```

### GitLab CI

```yaml
trackfw:validate:
  stage: test
  script:
    - trackfw validate
```

O script `scripts/trackfw-validate.sh` gerado pelo `trackfw init` já contém a configuração correta para a stack do seu projeto.

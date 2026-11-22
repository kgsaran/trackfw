# Início Rápido

Este guia cobre a instalação do `trackfw` e os primeiros passos para configurar governança de entrega no seu projeto.

## Instalação

### Homebrew (macOS / Linux)

```bash
brew tap kgsaran/trackfw
brew install trackfw
```

### npm

```bash
npm install -g trackfw
```

### pip

```bash
pip install trackfw
```

### Script shell (macOS / Linux / Git Bash no Windows)

```bash
curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
```

### PowerShell (Windows)

```powershell
irm https://github.com/kgsaran/trackfw/releases/latest/download/install.ps1 | iex
```

> Nota: o `install.ps1` é publicado como asset de release a partir da release após a 11.1.0. Para instalação no Windows hoje, use npm (`npm install -g trackfw`), pip (`pip install trackfw`) ou o script shell via Git Bash.

### Verificar instalação

```bash
trackfw version
```

Exibe a versão instalada (por exemplo, `trackfw 11.8.1`).

---

## Quick start

Os cinco comandos abaixo configuram um projeto com governança completa do zero:

```bash
trackfw init --ai-tools claude --forge github
trackfw adr new "User Authentication Strategy"
trackfw req new "User Authentication" --adr ADR-$(date +%F)-user-authentication-strategy.md
trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip
trackfw validate
```

Após executar esses comandos, `trackfw validate` termina com rc=0 e exibe `2 warning(s)`:

```
⚠  adr_req_has_content: "docs/adr/ADR-...-user-authentication-strategy.md" has mandatory
   section(s) with only model content: ## Context, ## Decision, ## Consequences,
   ## Alternatives Considered — fill in or set rules.adr_req_has_content: off
⚠  adr_req_has_content: "docs/req/REQ-...-user-authentication.md" has mandatory
   section(s) with only model content: ## Motivation, ## Acceptance Criteria — fill in or
   set rules.adr_req_has_content: off
2 warning(s)
```

Os warnings aparecem porque as seções do ADR e da REQ contêm apenas conteúdo de template. Em uma sessão com TTY, `adr new` e `req new` abrem wizards — se você preencher todas as seções, `validate` produz `✓ No violations found.` em vez disso. Você também pode desativar a regra com `rules.adr_req_has_content: off` no `trackfw.yaml`.

---

## 1. Inicializar um projeto: `trackfw init`

O comando `init` executa um wizard interativo que detecta a stack do projeto e gera toda a estrutura de governança.

```bash
trackfw init
```

Para o Quick start, passe as flags diretamente:

```bash
trackfw init --ai-tools claude --forge github
```

O wizard (ou as flags) configura:

- **Nome do projeto** — usado nos templates gerados
- **Tipo de projeto** — frontend, backend ou fullstack
- **Stack backend** — Go, Java, Node.js ou Python
- **Framework backend** — Spring Boot, Gin, Express, FastAPI, etc.
- **Git hooks** — husky, lefthook ou nenhum
- **CI/CD** — GitHub Actions ou GitLab CI
- **Agentes de IA** — Claude Code, OpenAI Codex, Gemini CLI, Google Antigravity, Cursor, GitHub Copilot, Windsurf, Amazon Q, OpenCode e Kiro (10 targets).

### O que é gerado

Após o `init`, a seguinte estrutura é criada no repositório:

```
docs/
├── adr/              ← Architecture Decision Records
├── req/              ← Requirements
└── roadmaps/
    ├── backlog/
    ├── analyzing/
    ├── wip/
    ├── blocked/
    ├── done/
    └── abandoned/

vault/
└── notes/
    └── index.md      ← índice do vault de conhecimento

trackfw.yaml          ← configuração do projeto
scripts/
├── trackfw-validate.sh
├── trackfw-attention-signal.sh
├── trackfw-attention-cleanup.sh
├── trackfw-credential-guard.sh
└── trackfw-git-branch-guard.sh

CLAUDE.md             ← contexto para Claude Code (se selecionado)
.claude/
├── commands/
│   └── trackfw/      ← 9 slash commands para Claude Code
└── settings.json
AGENTS.md             ← instruções persistentes para Codex (se selecionado)
.agents/skills/       ← workflows de governança para Codex
.codex/agents/        ← subagentes especialistas do Codex
```

> **Escrita global:** Quando `--ai-tools claude` é selecionado com o `--scope global` padrão, o `init` também escreve 12 agents em `~/.claude/agents/` e 17 skills em `~/.claude/skills/`. Use `--scope project` para manter todos os arquivos dentro do repositório.

### Modo brownfield (projeto legado)

Para projetos que já existem sem governança estruturada, use o flag `--brownfield`:

```bash
trackfw init --brownfield
```

**Sem `trackfw.yaml` existente:** o `init` grava `governance_mode: lenient` e `lenient_until` (hoje + 30 dias) no `trackfw.yaml` gerado. Durante o período lenient, violações de governança são reportadas como warnings em vez de erros.

**Com `trackfw.yaml` existente:** o `init` avisa `governance keys not modified — edit trackfw.yaml directly to enable lenient mode` e não altera o arquivo existente. Para ativar o modo lenient manualmente, adicione ao `trackfw.yaml`:

```yaml
governance_mode: lenient
lenient_until: YYYY-MM-DD   # 30 dias a partir de hoje
```

---

## 2. Primeira decisão arquitetural: `trackfw adr new`

ADRs (Architecture Decision Records) documentam decisões técnicas significativas do projeto.

```bash
trackfw adr new "User Authentication Strategy"
```

Passe o título como argumento. Em um TTY, omitir o título abre o wizard.

O wizard (ou o argumento) solicita:
- **Título** da decisão
- **Context** — por que essa decisão foi necessária
- **Decision** — o que foi decidido
- **Consequences** — impactos positivos e negativos
- **Alternatives Considered** — outras opções avaliadas

### Saída esperada

```
created docs/adr/ADR-2026-06-13-user-authentication-strategy.md
```

### Arquivo gerado

```markdown
---
status: Proposed
date: 2026-06-13
author: ""
---

# ADR: User Authentication Strategy

| Status: Proposed | Date: 2026-06-13 |

## Context
<!-- Why this decision was needed -->

## Decision
<!-- What was decided -->

## Consequences
<!-- Positive and negative impacts -->

## Alternatives Considered
<!-- Other options evaluated -->
```

> **Atenção:** As seções do arquivo gerado (`## Context`, `## Decision`, `## Consequences`, `## Alternatives Considered`) estão em inglês — são marcadores literais usados pelo `trackfw validate`. Se o validate exibir warnings sobre essas seções, preencha-as (ou desative com `rules.adr_req_has_content: off`).

### Listar ADRs

```bash
trackfw adr list
```

```
ADR-2026-06-13-user-authentication-strategy.md    Proposed
```

---

## 3. Primeiro requisito: `trackfw req new`

REQs (Requirements) documentam necessidades de negócio e técnicas. O wizard inclui **probes contextuais** — perguntas específicas por domínio (autenticação, UI, persistência, etc.) que geram ADR Drafts automaticamente quando necessário.

```bash
trackfw req new "User Authentication" --adr ADR-$(date +%F)-user-authentication-strategy.md
```

Passe o título como argumento. Em um TTY, omitir o título abre o wizard.

O wizard (ou o argumento) solicita:
- **Título** do requisito
- **Motivation** — por que este requisito existe
- **Acceptance Criteria** — como saber que está pronto
- **ADR vinculado** — decisão arquitetural relacionada

### Roadmap criado automaticamente

`req new` cria automaticamente um roadmap em `docs/roadmaps/backlog/`. Não é necessário executar `roadmap new` separadamente — ele só é necessário se você usar `req new --no-roadmap`.

### Saída esperada

```
created docs/req/REQ-2026-06-13-user-authentication.md
✓ created docs/roadmaps/backlog/ROADMAP-2026-06-13-user-authentication.md
✓ linked REQ-2026-06-13-user-authentication.md → docs/roadmaps/backlog/ROADMAP-...
```

> **Atenção:** As seções do arquivo gerado (`## Motivation`, `## Acceptance Criteria`) estão em inglês — são marcadores literais usados pelo `trackfw validate`.

### Listar REQs

```bash
trackfw req list
```

```
REQ-2026-06-13-user-authentication.md    Open
```

---

## 4. Mover roadmap para WIP: `trackfw roadmap move`

O roadmap foi criado em `backlog/` pelo `req new`. Para iniciar a implementação, mova-o para `wip/`:

```bash
trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip
```

Use o prefixo `ROADMAP-` com a extensão `.md`, não o nome da REQ.

### Saída esperada

```
✓ moved ROADMAP-2026-06-13-user-authentication.md → docs/roadmaps/wip
✓ synced REQ-2026-06-13-user-authentication.md → docs/roadmaps/wip/ROADMAP-...
```

### Estrutura do roadmap gerado

O roadmap gerado pelo `req new` inclui uma Wave 0 de threat model:

```markdown
---
status: wip
date: 2026-06-13
req: "docs/req/REQ-2026-06-13-user-authentication.md"
squad: ""
---

# Roadmap: User Authentication

> Created: 2026-06-13 | Status: wip

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pending
...

## Wave 1 — ...
```

> **Nota:** o `ML-0A` precisa ser marcado `✅ Done` antes que `roadmap move ... done` seja bem-sucedido. Mover para `done` com ML-0A pendente produz: `Error: cannot move "..." to done: 1 blocker(s): ML-0A`.

### Estados disponíveis

`backlog` → `analyzing` → `wip` → `done` (ou `blocked` / `abandoned`)

---

## 5. Verificar consistência: `trackfw validate`

```bash
trackfw validate
```

Após o Quick start, `validate` termina com rc=0 e exibe `2 warning(s)` (veja a seção Quick start acima para a saída completa).

### Se houver violações

```
✗ roadmap "ROADMAP-2026-06-13-implement-oauth.md" is in wip but has no linked REQ
✗ REQ-2026-06-13-oauth-login.md is blocked by Draft ADR: ADR-2026-06-13-oauth-provider.md
Error: 2 violation(s) found
```

Cada violação é prefixada com `✗`. Warnings usam `⚠`.

---

## 6. Visão geral do projeto: `trackfw status`

```bash
trackfw status
```

Exibe um inventário de ADRs, REQs e roadmaps por estado, com roadmaps em WIP e blocked listados por nome. O formato da saída e os contadores refletem o estado atual do projeto.

---

---

## Windows

Para configuração detalhada no Windows — caminhos de instalação, shells suportados (PowerShell 5.1, Git Bash, pwsh 7), matriz de suporte medida e limitações conhecidas — veja a seção **Windows support (partial)** no [README do trackfw no GitHub](https://github.com/kgsaran/trackfw#windows-support-partial).

---

## Próximos passos

- [Referência completa de comandos](/pt/guide/commands)
- [Usando trackfw com agentes de IA](/pt/guide/ai-agents)

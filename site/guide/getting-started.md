# Início Rápido

Este guia cobre a instalação do `trackfw` e os primeiros passos para configurar governança de entrega no seu projeto.

## Instalação

### Homebrew (macOS / Linux)

```bash
brew tap kgsaran/trackfw
brew install trackfw
```

### npm (Node.js >= 18)

```bash
npm install -g trackfw
```

### Go (requer Go >= 1.21)

```bash
go install github.com/kgsaran/trackfw/cmd/trackfw@latest
```

### Verificar instalação

```bash
trackfw version
# trackfw v2.1.0
```

---

## 1. Inicializar um projeto: `trackfw init`

O comando `init` executa um wizard interativo que detecta a stack do projeto e gera toda a estrutura de governança.

```bash
trackfw init
```

O wizard faz perguntas sobre:

- **Nome do projeto** — usado nos templates gerados
- **Tipo de projeto** — frontend, backend ou fullstack
- **Stack backend** — Go, Java, Node.js ou Python
- **Framework backend** — Spring Boot, Gin, Express, FastAPI, etc.
- **Git hooks** — husky, lefthook ou nenhum
- **CI/CD** — GitHub Actions ou GitLab CI
- **Agentes de IA** — Claude Code, Gemini CLI, Cursor, etc.

### O que é gerado

Após o `init`, a seguinte estrutura é criada no repositório:

```
docs/
├── adr/              ← Architecture Decision Records
├── req/              ← Requirements
├── roadmaps/
│   ├── backlog/
│   ├── wip/
│   ├── blocked/
│   ├── done/
│   └── abandoned/
├── visao-projeto/
└── agents-working-context.md

trackfw.yaml          ← configuração do projeto
scripts/
└── trackfw-validate.sh
CLAUDE.md             ← contexto para Claude Code (se selecionado)
.claude/commands/     ← slash commands para Claude Code
```

### Modo brownfield (projeto legado)

Para projetos que já existem sem governança estruturada, use o flag `--brownfield`:

```bash
trackfw init --brownfield
```

Isso ativa o **modo lenient** por 30 dias: violações de governança são reportadas como avisos (warnings) em vez de erros, dando tempo para o time adaptar os processos sem bloquear CIs existentes.

---

## 2. Primeira decisão arquitetural: `trackfw adr new`

ADRs (Architecture Decision Records) documentam decisões técnicas significativas do projeto.

```bash
trackfw adr new
```

O wizard solicita:
- **Título** da decisão
- **Contexto** — por que essa decisão foi necessária
- **Decisão** — o que foi decidido
- **Consequências** — impactos positivos e negativos
- **Alternativas** consideradas

### Saída esperada

```
created docs/adr/ADR-2026-06-13-usar-postgresql-como-banco-principal.md
```

### Arquivo gerado

```markdown
---
status: Proposed
date: 2026-06-13
author: ""
---

# ADR: Usar PostgreSQL como banco principal

| Status: Proposed | Date: 2026-06-13 |

## Contexto
<!-- Por que essa decisão foi necessária -->

## Decisão
<!-- O que foi decidido -->

## Consequências
<!-- Impactos positivos e negativos -->

## Alternativas consideradas
<!-- Outras opções avaliadas -->
```

### Listar ADRs

```bash
trackfw adr list
```

```
ADR-2026-06-13-usar-postgresql-como-banco-principal.md    Proposed
```

---

## 3. Primeiro requisito: `trackfw req new`

REQs (Requirements) documentam necessidades de negócio e técnicas. O wizard inclui **probes contextuais** — perguntas específicas por domínio (autenticação, UI, persistência, etc.) que geram ADR Drafts automaticamente quando necessário.

```bash
trackfw req new
```

O wizard solicita:
- **Título** do requisito
- **Motivação** — por que este requisito existe
- **Critérios de aceite** — como saber que está pronto
- **ADR vinculado** — decisão arquitetural relacionada
- **Roadmap vinculado** — plano de implementação

Se o título ou motivação mencionar domínios conhecidos (autenticação, banco de dados, deploy, etc.), o wizard apresenta perguntas adicionais específicas e pode criar ADR Drafts automaticamente.

### Saída esperada

```
created docs/req/REQ-2026-06-13-login-via-oauth.md
created docs/adr/ADR-2026-06-13-provider-oauth.md (Draft)

Next step: resolve Draft ADRs before creating the roadmap.
```

### Listar REQs

```bash
trackfw req list
```

```
REQ-2026-06-13-login-via-oauth.md    Open
```

---

## 4. Primeiro roadmap: `trackfw roadmap new`

Roadmaps detalham o plano de implementação de uma REQ em microlotes (MLs).

```bash
trackfw roadmap new
# ou vinculando diretamente a uma REQ:
trackfw roadmap new --from-req docs/req/REQ-2026-06-13-login-via-oauth.md
```

### Flags disponíveis

| Flag | Descrição |
|------|-----------|
| `--title "Título"` | Título do roadmap (sem wizard) |
| `--req docs/req/REQ-*.md` | Caminho da REQ vinculada |
| `--from-req docs/req/REQ-*.md` | Cria roadmap já vinculado à REQ |

### Arquivo gerado

```markdown
---
status: backlog
date: 2026-06-13
req: ""
squad: ""
---

# Roadmap: Login via OAuth

> Criado em: 2026-06-13 | Status: backlog

## REQ: docs/req/REQ-2026-06-13-login-via-oauth.md

## Wave 1 — ...
```

---

## 5. Mover roadmap entre estados

```bash
# Iniciar implementação
trackfw roadmap move REQ-2026-06-13-login-via-oauth wip

# Concluir
trackfw roadmap move REQ-2026-06-13-login-via-oauth done
```

Estados disponíveis: `backlog` → `wip` → `done` (ou `blocked` / `abandoned`)

---

## 6. Visão geral: `trackfw status` e `trackfw validate`

### Status do projeto

```bash
trackfw status
```

```
trackfw — project status

📋 Backlog       2 roadmaps
🔄 WIP           1 roadmap
❌ Blocked       0 roadmaps
✅ Done          3 roadmaps

📄 ADRs          4   (Proposed: 2, Accepted: 1, Draft: 1)
📝 REQs          3   (Open: 2, Closed: 1)
```

### Validação de consistência

```bash
trackfw validate
```

```
✓ No violations found.
```

Se houver problemas:

```
✗ 1 violation(s) found:

  [violation] REQ-2026-06-13-login-via-oauth.md is blocked by Draft ADR: ADR-2026-06-13-provider-oauth.md

⚠  1 warning(s):

  [warning] ROADMAP-2026-06-10-refactor-db.md in WIP for 9 days (stale)
```

---

## Próximos passos

- [Referência completa de comandos](/guide/commands)
- [Usando trackfw com agentes de IA](/guide/ai-agents)

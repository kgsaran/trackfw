---
status: wip
date: 2026-06-19
req: "docs/requisições/claude/REQ-2026-06-19-architect-command-guidelines.md"
branch: "feat/architect-command-guidelines"
---

# Roadmap: Slash command /trackfw:architect + diretrizes de arquitetura

> Criado em: 2026-06-19 | Status: 🔄 WIP
> REQ: `docs/requisições/claude/REQ-2026-06-19-architect-command-guidelines.md`

## Diagnóstico / Contexto

Times não técnicos que usam o trackfw não têm orientação sobre stack e arquitetura. Os agentes tomam decisões técnicas arbitrárias. A solução é um slash command guia-ativo que faz perguntas de negócio, recomenda uma das 3 stacks pré-validadas, explica arquitetura em camadas com metáforas simples e gera o ADR automaticamente. As diretrizes de arquitetura também precisam ser resumidas no bloco de regras injetado (≤15 linhas).

## Wave 1 — Implementação em paridade nos 3 CLIs (3 MLs em paralelo)
> Dependências: Independente (arquivos distintos por CLI)

### ML-1A — Go: architect.md + regras de arquitetura no rules block
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/generators/scaffold.go` (função `generateClaudeCommandsInner`, mapa `commands`)
- `internal/generators/claudemd.go` (função `generateClaudeMD` — seção "Agent rules")
**Ações:**
1. Em `generateClaudeCommandsInner` (linha ~221), adicionar entrada `"architect.md"` ao mapa `commands` com o conteúdo completo do slash command (ver spec abaixo)
2. Em `generateClaudeMD` em `claudemd.go`, adicionar linha 6a nas "Agent rules":
   `6a. **Usar `/trackfw:architect` para definir stack e arquitetura antes da primeira REQ.**`
   E adicionar nova seção `## Architecture Directives` com as 8 diretrizes obrigatórias
3. Adicionar `| \`/trackfw:architect\` | Guide stack and architecture decisions |` na tabela de slash commands
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` sem erros
- [ ] Arquivo `architect.md` gerado em `.claude/commands/trackfw/` após `trackfw init` (manual check)
**Comandos de validação:** `cd /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw && go build ./... && go test ./...`

### ML-1B — Node.js: architect.md + regras de arquitetura no rules block
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `npm/src/generators/init.js` (funções `generateClaudeCommands` e `generateClaudeCommandsForce`, e `trackfwRulesBlock`)
**Ações:**
1. Em `generateClaudeCommands()` (linha ~585), adicionar `'architect.md'` ao mapa `commands`
2. Em `generateClaudeCommandsForce()` (linha ~895+), adicionar `'architect.md'` ao mapa `commands`
3. Em `trackfwRulesBlock()` (linha ~331), adicionar seção `### Architecture Directives` com as 8 diretrizes
**Critérios de aceite:**
- [ ] `node npm/src/cli.js --version` sem erros
- [ ] Arquivo `architect.md` gerado após chamada de `generateClaudeCommands()`
**Comandos de validação:** `cd /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw && node -e "const g = require('./npm/src/generators/init.js'); console.log(typeof g.generateClaudeCommandsForce)"`

### ML-1C — Python: generate_claude_commands + architect.md + regras de arquitetura
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `pypi/trackfw/generators/init_gen.py` (nova função `generate_claude_commands`, atualização de `_trackfw_rules_block`, chamada em `scaffold`)
- `pypi/trackfw/commands/discover.py` (chamar `generate_claude_commands` no `--init`)
**Ações:**
1. Criar função `generate_claude_commands(cwd: str) -> None` em `init_gen.py` com todos os slash commands (adr, req, validate, status, move, roadmap, implement, **architect**)
2. Chamar `generate_claude_commands(cwd)` ao final de `scaffold()` em `init_gen.py`
3. Chamar `generate_claude_commands(cwd)` no bloco `--init` de `discover.py` (após `inject_rules_detected`)
4. Atualizar `_trackfw_rules_block()` para incluir seção `### Architecture Directives` com as 8 diretrizes
**Critérios de aceite:**
- [ ] `python -m pytest pypi/tests/ -x -q` sem erros
- [ ] Função `generate_claude_commands` exportada e chamável
**Comandos de validação:** `cd /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw && python -m pytest pypi/tests/ -x -q 2>&1 | tail -5`

---

## Conteúdo do slash command architect.md

```markdown
Você é o guia de arquitetura do trackfw. Ajude o usuário a escolher a stack correta e arquitetar a aplicação em linguagem simples, acessível para times não técnicos.

## Passo 1 — Descoberta de Negócio

Faça ao usuário as seguintes perguntas em linguagem simples, uma por vez:

1. "O que sua aplicação vai fazer? Descreva em 2-3 frases como se fosse explicar para alguém de fora da TI."
2. "Quantas pessoas vão usar esse sistema simultaneamente? (< 10 pessoas / 10-100 pessoas / > 100 pessoas)"
3. "Esse sistema vai para produção de verdade ou é um protótipo para validar uma ideia?"
4. "Você precisa de login/autenticação de usuários? (Sim / Não / Não sei)"
5. "Tem alguma restrição de tecnologia ou preferência da empresa? (ex: só Java, só Microsoft, etc.)"

---

## Passo 2 — Recomendação de Stack

Com base nas respostas, escolha **UM** dos combos pré-validados:

### Combo A — Protótipo Rápido
**Quando usar:** prototipagem, validação de ideia, até ~10 usuários, sem pressão de produção.
- **Frontend:** React + Vite
- **Backend:** FastAPI (Python) ou Express (Node.js)
- **Banco:** SQLite + SQLAlchemy / Prisma
- **Auth:** JWT simples quando necessário
- **Docker:** Dockerfile básico para o backend

### Combo B — Sistema Pequeno/Médio em Produção
**Quando usar:** sistema real, 10-100 usuários, robustez e manutenibilidade.
- **Frontend:** Next.js (SSR + rotas prontas)
- **Backend:** FastAPI (Python) ou NestJS (Node.js)
- **Banco:** PostgreSQL + ORM (SQLAlchemy / Prisma / TypeORM)
- **Auth:** OAuth2 com JWT (Supabase Auth ou Auth0)
- **Docker:** docker-compose com frontend + backend + banco

### Combo C — Enterprise / Java
**Quando usar:** integração com sistemas corporativos, > 100 usuários, exigência de Java.
- **Frontend:** Angular
- **Backend:** Spring Boot
- **Banco:** PostgreSQL + Hibernate
- **Auth:** Spring Security + OAuth2 (Keycloak ou Azure AD)
- **Docker:** docker-compose com todos os serviços

Apresente o combo recomendado com explicação simples do motivo.

---

## Passo 3 — Arquitetura em Camadas (explicação simples)

Explique a arquitetura com uma metáfora de negócio:

"Pense na aplicação como um restaurante:
- **Frontend** = o salão: o que o cliente vê e interage
- **Backend** = a cozinha: onde as regras de negócio acontecem, nunca exposta diretamente
- **Banco de dados** = a despensa: onde os dados ficam guardados, acessada só pela cozinha"

Reforce as regras obrigatórias:
- **Nunca dados em memória** (arrays, variáveis globais): sempre banco + ORM
- **Docker + .env desde o dia 1**: facilita deploys e evita problemas de ambiente
- **Auth desde o início**: nunca deixe para depois — refatorar auth é muito custoso
- **Validação em 2 camadas**: frontend (UX) + backend (segurança)
- **API-first**: defina o contrato OpenAPI antes de codar frontend e backend juntos
- **Red team wave**: reserve 1 wave de segurança no roadmap para revisar vulnerabilidades
- **Cobertura mínima de testes**: 60% (protótipo) / 80% (produção)

---

## Passo 4 — Gerar o ADR de Stack

Execute `/trackfw:adr` com o título: `"Stack e arquitetura em camadas — [nome do projeto]"`

O ADR deve registrar a stack escolhida (combo e componentes), motivação baseada nas respostas, alternativas descartadas e princípios de arquitetura adotados.

---

## Passo 5 — Próximos Passos

Oriente o usuário:

```
✅ Stack definida. Próximos passos:

1. Crie a REQ da primeira feature com /trackfw:req
2. Gere o roadmap em microlotes com /trackfw:roadmap
3. Inicie a implementação com /trackfw:implement
```
```

## Diretrizes de arquitetura para o rules block (≤15 linhas)

```
### Architecture Directives (mandatory)
- **3-layer separation:** frontend / backend / database — never mix concerns
- **No in-memory data:** always database + ORM (never arrays/globals for persistence)
- **Auth from day 1:** never defer — refactoring auth later is very costly
- **Docker + .env from day 1:** containerize early; all config via env vars
- **2-layer validation:** frontend (UX) + backend (security) — never only one
- **API-first:** define OpenAPI contract before coding frontend/backend integration
- **Security wave:** include a red-team review wave in every feature roadmap
- **Test coverage:** TDD for critical logic; min 60% (prototype) / 80% (production)
- Use `/trackfw:architect` to define stack before the first REQ
```

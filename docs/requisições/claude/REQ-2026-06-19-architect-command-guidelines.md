---
id: REQ-2026-06-19-architect-command-guidelines
title: Slash command /trackfw:architect — guia de arquitetura para times não técnicos
status: approved
priority: high
type: feature
created: 2026-06-19
author: zeus
---

# REQ: Slash command /trackfw:architect + diretrizes de arquitetura

## Problema

Usuários não técnicos (times de negócio) que usam o trackfw para criar protótipos funcionais não têm orientação sobre como estruturar a aplicação corretamente. Os agentes de IA tomam decisões técnicas arbitrárias sem considerar boas práticas de arquitetura em camadas, segurança, testabilidade e deployment.

## Requisitos

### R1 — Slash command `/trackfw:architect`
Instalado em `.claude/commands/trackfw/architect.md` via `init`/`discover --init`/`update`.

**Comportamento (guia ativo):**
1. Faz perguntas de negócio em linguagem simples (não técnica)
2. Com base nas respostas, recomenda UMA stack validada (combo frontend + backend + banco + auth)
3. Explica a arquitetura em camadas com metáforas de negócio
4. Gera o ADR de stack e arquitetura
5. Aponta próximos passos (REQ → ROADMAP)

**Stacks pré-validadas:**
- **Combo A — Protótipo rápido**: React + Vite + FastAPI/Express + SQLite + SQLAlchemy/Prisma
- **Combo B — Sistema pequeno/médio em produção**: Next.js + FastAPI/NestJS + PostgreSQL + ORM + OAuth2
- **Combo C — Enterprise/Java**: Angular + Spring Boot + PostgreSQL + Hibernate + Spring Security

### R2 — Diretrizes obrigatórias de arquitetura (boas práticas injetadas)
Adicionadas ao bloco de regras (`<!-- trackfw:rules:start -->`) de forma concisa:
- Separação obrigatória em 3 camadas: frontend / backend / banco de dados
- Nunca dados mockados in-memory (sempre banco de dados + ORM)
- Auth desde o início (nunca deixar para depois)
- Docker + `.env` desde o dia 1
- Validação em 2 camadas: frontend (UX) + backend (segurança)
- Wave de segurança (red team) obrigatória ao final de cada roadmap de feature
- Testes obrigatórios: TDD para lógica de negócio crítica, test-after para o resto (cobertura mínima 60% protótipo / 80% produção)
- API-first: contrato OpenAPI antes de qualquer código de integração frontend/backend

### R3 — Paridade 3 CLIs
Go, Node.js e Python devem instalar o `architect.md` via `generateClaudeCommands()`.

## Critérios de Aceite

- [ ] `architect.md` gerado por `trackfw init` em `.claude/commands/trackfw/`
- [ ] Comando cobre: discovery → stack recommendation → ADR → next steps
- [ ] Bloco de regras injetado contém resumo das diretrizes arquiteturais (≤ 15 linhas)
- [ ] 3 CLIs (Go, Node.js, Python) instalam o comando
- [ ] `trackfw update` reinjecta o comando atualizado
- [ ] Build e testes verdes

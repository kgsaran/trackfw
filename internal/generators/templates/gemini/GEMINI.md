# trackfw — Instruções de Governança

O trackfw é um framework de entrega de software que impõe uma cadeia de rastreabilidade:

**ADR → REQ → ROADMAP → backlog/wip/done**

## Estrutura de artefatos

- `docs/adr/` — Architecture Decision Records: decisões arquiteturais versionadas.
- `docs/requisições/` — REQs: requisições detalhadas antes de qualquer implementação não trivial.
- `docs/roadmaps/` — Roadmaps em microlotes. Estados pela pasta: `backlog/` → `wip/` → `done/`.

## Regras principais

- Sempre criar REQ + roadmap antes de features, refactors ou mudanças de contrato de API.
- Criar branch antes de qualquer implementação: `feat/<descricao>`, `fix/<descricao>`.
- Nunca commitar direto na `main`.
- Cada ML do roadmap: build + testes + commit + push.
- Iniciar qualquer tarefa lendo o codebase atual antes de propor mudanças.

## Quando NÃO criar REQ/roadmap

- Correção de typo/renomear variável local.
- Mudança doc-only (markdown, comentários).
- Ajuste de config sem efeito em runtime.
- Revert direto de commit anterior.

## Papéis disponíveis

Dez skills especializadas estão disponíveis em `~/.gemini/skills/`:

- `trackfw-architect`: decisões arquiteturais, ADRs, orquestração
- `trackfw-backend`: Go/Java, APIs REST/gRPC/GraphQL
- `trackfw-frontend`: React/Next.js, i18n, WCAG 2.2, MFE
- `trackfw-qa`: Playwright, Vitest, contract testing
- `trackfw-infra`: Kubernetes, AWS/GCP/Azure, GitOps, Terraform
- `trackfw-security`: SAST/DAST, Zero Trust, OWASP
- `trackfw-code-quality`: SonarQube, linting, fitness functions
- `trackfw-dba`: PostgreSQL, ArangoDB, pgvector, bancos vetoriais
- `trackfw-ux`: Design System, Figma, WCAG 2.2
- `trackfw-data`: Airflow/Kafka/dbt, MLOps, LangChain/RAG

## Comandos disponíveis

- `/trackfw-adr [titulo]` — criar novo ADR
- `/trackfw-req [titulo]` — criar nova REQ
- `/trackfw-roadmap [titulo]` — criar novo roadmap

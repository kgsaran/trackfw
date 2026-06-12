# trackfw — Instruções de Governança

O trackfw é um framework de entrega de software que impõe uma cadeia de rastreabilidade:

**ADR → REQ → ROADMAP → backlog/wip/done**

## Estrutura de artefatos

- `docs/adr/` — Architecture Decision Records: decisões arquiteturais versionadas.
- `docs/requisições/` — REQs: requisições detalhadas antes de qualquer implementação não trivial.
- `docs/roadmaps/` — Roadmaps em microlotes: cada ML com arquivos, ações e critérios de aceite.
- Estados do roadmap pela pasta: `backlog/` → `wip/` → `done/` (ou `blocked/`, `abandoned/`).

## Regras principais

- Sempre criar REQ + roadmap antes de features, refactors ou mudanças de contrato.
- Criar branch antes de qualquer implementação: `feat/<descricao>`, `fix/<descricao>`, `refactor/<descricao>`.
- Nunca commitar direto na `main`.
- Cada ML do roadmap: build + testes + gate + commit + push.

## Stack típica (adaptar ao projeto)

- Backend: Go (Gin/Fx/Clean Arch) ou Java Spring Boot 3.x
- Frontend: React 19 / Next.js 15 App Router, Tailwind, i18next
- Banco: PostgreSQL, ArangoDB, Redis, pgvector
- Infra: Kubernetes, GitHub Actions, ArgoCD, Terraform

## Papéis disponíveis

Dez papéis especializados estão disponíveis via arquivos de instruções:

- **trackfw-architect**: decisões arquiteturais, ADRs, orquestração
- **trackfw-backend**: Go/Java, APIs REST/gRPC/GraphQL, microserviços
- **trackfw-frontend**: React/Next.js, i18n, WCAG 2.2, Module Federation
- **trackfw-qa**: Playwright, Vitest, contract testing, CI quality gates
- **trackfw-infra**: Kubernetes, AWS/GCP/Azure, GitOps, Terraform
- **trackfw-security**: SAST/DAST/SCA, Zero Trust, OWASP, secrets
- **trackfw-code-quality**: SonarQube, linting, Architecture Fitness Functions
- **trackfw-dba**: PostgreSQL, ArangoDB, pgvector, bancos vetoriais
- **trackfw-ux**: Design System, Figma, WCAG 2.2, User Research
- **trackfw-data**: Airflow/Kafka/dbt, MLOps, LangChain/RAG

---
status: Open
date: 2026-08-07
author: "kg.saran@gmail.com"
adr: ""
roadmap: ""
---

# REQ: atualiza actions do github para versoes com node24 nativo

> Date: 2026-08-07 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

O workflow de Release da tag `v6.5.0` (run 31170005222) emitiu avisos de depreciação em todos os 4
jobs: `actions/checkout@v4`, `actions/setup-go@v5`, `actions/setup-node@v4`, `actions/setup-python@v5`
e `goreleaser/goreleaser-action@v6` ainda declaram runtime `node20` no `action.yml`, e o GitHub está
forçando essas execuções a rodar em `node24` como fallback temporário:

> "Node.js 20 is deprecated. The following actions target Node.js 20 but are being forced to run on
> Node.js 24: actions/checkout@v4, actions/setup-go@v5, actions/setup-node@v4, actions/setup-python@v5.
> For more information see: https://github.blog/changelog/2025-09-19-deprecation-of-node-20-on-github-actions-runners/"

O fallback é temporário — quando o GitHub descontinuar de vez o runtime `node20`, esses workflows
podem parar de rodar sem aviso prévio adicional. Todos os 5 workflows do repositório
(`quality.yml`, `trackfw-validate.yml`, `trackfw-gate.yml`, `release.yml`, `deploy-docs.yml`) usam
pelo menos uma dessas actions.

## Acceptance Criteria
- [ ] `actions/checkout@v4` → `@v7` (ou major mais recente confirmado com runtime `node24` nativo) em
      todos os workflows
- [ ] `actions/setup-go@v5` → `@v7` em todos os workflows
- [ ] `actions/setup-node@v4` → `@v7` em todos os workflows
- [ ] `actions/setup-python@v5` → `@v7` em todos os workflows
- [ ] `goreleaser/goreleaser-action@v6` → `@v7` em `release.yml`
- [ ] `actions/upload-pages-artifact@v3`/`actions/deploy-pages@v4` (usadas só em `deploy-docs.yml`,
      não apareceram no aviso de depreciação, mas avaliar se também merece bump preventivo para
      `@v5` durante o mesmo ML, já que estão no mesmo arquivo)
- [ ] Nenhum comportamento funcional dos workflows muda (só a versão pinada da action) — sem
      breaking changes conhecidas nas notas de release das novas majors consultadas
- [ ] Confirmado via execução real de CI (push/PR de teste) que os workflows atualizados rodam sem o
      aviso de depreciação e sem falhar
- [ ] `trackfw validate` sem violações novas

## Linked ADR
<!-- mudança mecânica de versão pinada, sem decisão de arquitetura — ADR não necessária -->
ADR: N/A

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-07-atualiza-actions-do-github-para-versoes-com-node24-nativo.md`

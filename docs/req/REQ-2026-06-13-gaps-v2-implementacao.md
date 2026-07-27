---
status: Done
date: 2026-06-13
author: zeus
adr: ""
roadmap: v2.0-gaps-implementacao-2026-06-13.md
---

# REQ: Implementação dos Gaps v2.0 — trackfw

> Date: 2026-06-13 | Status: Done

## Motivação

A análise comparativa do trackfw frente ao mercado (adr-tools, log4brains, Backstage, Linear, Cortex.io) identificou 7 gaps que limitam a adoção em times reais. Os dois gaps mais críticos (P0) representam o salto do trackfw de ferramenta de governança para **ferramenta de inteligência de delivery**: visualização navegável da cadeia ADR→REQ→ROADMAP e métricas de flow baseadas no `.trackfw-log`.

Os gaps P1 resolvem barreiras de adoção em times existentes (brownfield e monorepos multi-squad). Os gaps P2/P3 consolidam o ecossistema de plugins e integração.

---

## Critérios de Aceite

### P0 — trackfw serve (visualização)
- [ ] `trackfw serve` sobe servidor HTTP local na porta 4080 (configurável)
- [ ] Página inicial exibe grafo navegável ADR→REQ→ROADMAP
- [ ] Timeline cronológica de decisões (ADRs ordenados por data)
- [ ] Kanban visual dos roadmaps por estado (backlog/wip/blocked/done)
- [ ] Renderiza markdown para HTML (sem dependência de runtime JS no servidor)
- [ ] Zero dependências externas além da stdlib Go

### P0 — trackfw metrics
- [ ] `trackfw metrics` exibe cycle time médio (backlog→done), throughput (roadmaps/semana), WIP age atual
- [ ] Flag `--since <Nd>` filtra por período (ex: `--since 30d`)
- [ ] Flag `--export csv` gera arquivo `trackfw-metrics-YYYY-MM-DD.csv`
- [ ] Baseado exclusivamente no `.trackfw-log` existente (sem nova fonte de dados)
- [ ] Paridade npm: `node npm/bin/trackfw metrics` com saída idêntica

### P1 — Brownfield onboarding
- [ ] `trackfw init --brownfield` cria estrutura de governança com validate em modo `warn` (não quebra CI)
- [ ] Arquivo `trackfw.yaml` gerado com `governance_mode: lenient` e `lenient_until: YYYY-MM-DD` (30 dias)
- [ ] `trackfw validate` lê `governance_mode` e emite `[WARN]` em vez de `[ERROR]` quando `lenient`
- [ ] Após `lenient_until`, validate retorna automaticamente ao modo estrito

### P1 — WIP limit configurável por squad
- [ ] `trackfw.yaml` aceita `wip_limit: N` (default: 1) e `wip_by_squad: true/false`
- [ ] Frontmatter dos roadmaps aceita campo `squad: <nome>`
- [ ] `trackfw validate` respeita WIP limit por squad quando `wip_by_squad: true`
- [ ] `trackfw status` exibe breakdown por squad

### P2 — Plugin registry
- [ ] `trackfw plugins search <keyword>` consulta registry central (YAML no GitHub kgsaran/trackfw-plugins)
- [ ] Registry lista name, repo, description, version, installs
- [ ] `trackfw plugins add` aceita nome do registry (além de `user/repo`)

### P2 — Integração PM (Linear/Jira sync)
- [ ] `trackfw sync --to=linear` cria Issues no Linear para cada REQ Open sem issue vinculado
- [ ] Frontmatter de REQ aceita `linear_issue: <id>` (preenchido após sync)
- [ ] `trackfw sync --to=jira` equivalente para Jira Cloud (API token via trackfw.yaml ou env var)

### P3 — Commit message validation hook
- [ ] `trackfw init` gera hook `commit-msg` que verifica presença de `REQ:` no corpo quando branch começa com `feat/` ou `fix/`
- [ ] Hook configurável: `require_req_in_commit: true/false` em `trackfw.yaml`
- [ ] Mensagem de erro clara quando REQ não encontrada

---

## Fora de Escopo

- UI web hospedada em SaaS (trackfw serve é sempre local)
- Suporte a GitLab/Azure DevOps no sync PM (apenas GitHub Actions nesta REQ)
- Plugin registry com autenticação/publicação (apenas leitura pública)

## Linked ADR

ADR:

## Linked Roadmap

Roadmap: v2.0-gaps-implementacao-2026-06-13.md

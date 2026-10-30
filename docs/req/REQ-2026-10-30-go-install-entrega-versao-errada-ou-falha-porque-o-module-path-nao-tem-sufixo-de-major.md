---
status: Open
date: 2026-10-30
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-30-go-install-nao-e-canal-de-instalacao-do-trackfw.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md"
---

# REQ: go install entrega versao errada ou falha porque o module path nao tem sufixo de major

> Date: 2026-10-30 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Issue #637 (KG, medido no CMDB): o `trackfw-validate.yml` que o gerador entrega a consumidores
instala com `go install github.com/kgsaran/trackfw/cmd/trackfw@v<versão>`, e o Go recusa:
`module path must match major version ("github.com/kgsaran/trackfw/v11")`. O step falha antes do
`trackfw validate`; o gate de governança do consumidor nunca roda. Isso trava o desenvolvimento do
CMDB.

**Mesma causa, medida pelo arquiteto em 2026-10-30:** `go list -m github.com/kgsaran/trackfw@latest`
→ `v1.1.0`. O `go install ...@latest` que `README.md`, `site/index.md`,
`site/guide/getting-started.md`, `site/en/index.md` e `site/en/guide/getting-started.md` oferecem
instala a v1.1.0 **sem erro**. É pior que o relatado: falha silenciosa.

Sítios (enumerados com `grep -rn "go install\|cmd/trackfw@"`):

| sítio | efeito |
|---|---|
| `internal/generators/scaffold_doctor.go:152` (template de consumidor do `trackfw-validate.yml`) | falha rc=1 |
| `README.md:106`, `site/index.md:53`, `site/guide/getting-started.md:43`, `site/en/index.md:53`, `site/en/guide/getting-started.md:43` | instala v1.1.0 em silêncio |
| `docs/cli-parity.md:7550` (contrato do template) | descreve o mecanismo quebrado |

O `trackfw-gate.yml` canônico já instala pelo `install.sh` com `TRACKFW_VERSION` pinado; mas baixa o
**script** de `releases/latest/download/install.sh` (`scaffold.go:1801`, `:1850`), não da tag.

Decisão de direção: ADR-2026-10-30 (`go install` não é canal; module path não muda).

## Acceptance Criteria

- [ ] **AC1** — o `trackfw-validate.yml` de consumidor instala por `install.sh` com
      `TRACKFW_VERSION` = `internal/version.Version`, como o `trackfw-gate.yml`; job id
      `governance-go-install` mantido; nenhum `go install` em template de consumidor.
- [ ] **AC2** — teste do gerador que falha se qualquer template de consumidor emitir `go install`
      de `github.com/kgsaran/trackfw` sem sufixo de major (o critério que o issue pede: algo exercita
      o comando emitido).
- [ ] **AC3** — o script de instalação baixado pelos templates de consumidor (`trackfw-gate.yml`,
      `.gitlab-ci-trackfw.yml`, `trackfw-validate.yml`) vem da tag pinada, não de `releases/latest`.
      Viável: o `install.sh` é asset de cada release (`.goreleaser.yaml:44`; conferido na v11.2.0).
- [ ] **AC4** — `README.md` e as quatro páginas do site deixam de oferecer `go install`; oferecem os
      canais da ADR. `docs/cli-parity.md` descreve o mecanismo novo.
- [ ] **AC5** — `trackfw doctor` num consumidor com o `trackfw-validate.yml` antigo aponta a
      divergência, e `trackfw update` a corrige (contra-braço: arquivo atual não gera achado).
- [ ] **AC6** — o template do produtor (este repositório, `isProducer=true`) não muda: continua
      compilando do código-fonte.

## Negative Scope

- Não muda o module path nem imports.
- Não renomeia job ids.
- Não muda os scripts de hook que usam `releases/latest/download/install.sh` como fallback local
  (`scaffold.go:378–431`): não são templates de CI e não pinam versão por desenho.
- Não toca `.github/workflows/` deste repositório.

## Linked ADR
ADR: docs/adr/ADR-2026-10-30-go-install-nao-e-canal-de-instalacao-do-trackfw.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md

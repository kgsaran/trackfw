---
status: Accepted
date: 2026-10-30
author: "zeus-tf"
---

# ADR: go install nao e canal de instalacao do trackfw

> Date: 2026-10-30 | Status: Accepted

## Context

O `go.mod` declara `module github.com/kgsaran/trackfw`, sem sufixo de major. Para tags `>= v2`, o
toolchain do Go exige o sufixo (`/v11`) quando o módulo tem `go.mod`. Medido em 2026-10-30:

| comando | resultado |
|---|---|
| `go install github.com/kgsaran/trackfw/cmd/trackfw@v11.2.0` | `invalid version: ... module path must match major version ("github.com/kgsaran/trackfw/v11")`, rc=1 |
| `go list -m -versions github.com/kgsaran/trackfw` | `v0.1.0 … v1.0.4 v1.1.0` — nenhuma tag v2+ é visível ao toolchain |
| `go list -m github.com/kgsaran/trackfw@latest` | `v1.1.0` — `@latest` instala, **sem erro**, uma versão de muitos meses atrás |

Dois sítios dependiam disso:

1. O `trackfw-validate.yml` que o gerador entrega a consumidores (job `governance-go-install`)
   instala com `go install ...@v<versão>` e falha antes do `trackfw validate` (issue #637, medido no
   CMDB). O gate de governança do consumidor nunca roda.
2. `README.md`, `site/index.md`, `site/guide/getting-started.md` e as versões em inglês oferecem
   `go install ...@latest`, que entrega a v1.1.0 em silêncio.

O `trackfw-gate.yml` (canônico, ADR-2026-09-29 D1) já instala pelo `scripts/install.sh` com
`TRACKFW_VERSION` pinado e verificação SHA-256 contra o `checksums.txt` da tag.

## Decision

1. **`go install` não é canal de instalação do trackfw.** Os canais são os da Regra Dura do
   `CLAUDE.md`: `install.sh`/`install.ps1` e Homebrew (GitHub), `npm` e `PyPI`. A documentação de
   usuário deixa de oferecer `go install`. Compilar do código-fonte (`go build ./cmd/trackfw` num
   clone) continua possível e não é canal.
2. **O `trackfw-validate.yml` de consumidor instala pelo mesmo mecanismo do `trackfw-gate.yml`:**
   `install.sh` com `TRACKFW_VERSION` pinado em `internal/version.Version`.
3. **O job id `governance-go-install` é mantido.** É contrato de `required_status_checks` do
   consumidor (já registrado no comentário do gerador e na ADR-2026-09-29 D3); renomear deixaria
   PRs pendentes para sempre. O nome deixa de descrever o método, e isso fica documentado.
4. **O module path não muda.** Ver alternativas.

## Consequences

- Positivas: o gate do consumidor volta a rodar; ninguém instala a v1.1.0 por engano a partir da
  documentação; um único mecanismo de instalação nos workflows gerados.
- Negativas: quem usava `go install` precisa trocar de canal; o job id fica com nome que não
  descreve o passo.
- Consumidores com `trackfw-validate.yml` antigo recebem a correção com `trackfw update` (o alvo
  `ci-workflow` refresca o arquivo quando ele existe) depois de instalar a release com a correção.

## Alternatives Considered

- **Module path `github.com/kgsaran/trackfw/v11`.** Faz o `go install` funcionar, mas toda major
  exige trocar o path e todos os imports internos; foram três majors (v8 → v11) em cerca de dois
  meses. Rejeitada pelo custo recorrente, por decisão do KG em 2026-10-30.
- **Manter `go install` com pseudo-versão por commit.** Também é barrada pela regra de major
  (a pseudo-versão herda a major da tag mais próxima). Rejeitada.

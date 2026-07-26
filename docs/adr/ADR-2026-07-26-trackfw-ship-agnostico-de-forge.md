---
status: Accepted
date: 2026-07-26
author: "KG"
---

# ADR: trackfw ship agnostico de forge

> Date: 2026-07-26 | Status: Accepted

## Context

O harness pessoal do KG possui uma skill `git-ship` (122 linhas) que é o **único fluxo git
internamente coerente** entre os artefatos pessoais: valida a branch, detecta squash-merges pendentes,
revisa o que está staged, faz commit em Conventional Commits, dá push e abre o Pull Request.

Ela é a última skill de harness sem destino na convergência descrita no
`ADR-2026-07-26-convergencia-do-harness-pessoal-para-o-trackfw` (decisão D13), e fecharia o ciclo do
produto: hoje o trackfw sabe **validar** (`trackfw validate`) mas não sabe **entregar**.

O problema é que `git-ship` termina em `gh pr create`, amarrado ao GitHub. O trackfw é um produto
open-source e não pode assumir a forge do usuário. Além disso:

- **GitLab usa outro substantivo** — Merge Request, não Pull Request.
- **Instalações self-hosted** de GitLab e Bitbucket têm host arbitrário (`git.empresa.com.br`),
  portanto o parse do remote **não** identifica a forge sozinho.
- O `discover` já detecta o sistema de CI (`.github/workflows` → `github-actions`;
  `.gitlab-ci.yml` → `gitlab`), o que é um **proxy** da forge, mas não prova.
- `config.ProjectConfig` (lido de `trackfw.yaml`) é o local natural para persistir a escolha, e o
  `discover` já possui o helper `externalCommandAvailable` baseado em `exec.LookPath`.

## Decision

Criar o comando `trackfw ship`, portando o fluxo do `git-ship` com abertura de PR/MR **agnóstica de
forge**.

### Resolução da forge, por ordem de precedência

| # | Fonte | Observação |
|:-:|---|---|
| 1 | Flag `--forge github\|gitlab\|bitbucket\|azure` | Override pontual |
| 2 | Campo `forge:` em `trackfw.yaml` | Novo campo em `config.ProjectConfig`; perguntado no `init` e preenchido pelo `discover` |
| 3 | Host de `git remote get-url origin` | Fonte mais autoritativa quando o host é conhecido |
| 4 | Sistema de CI detectado pelo `discover` | Desempate para self-hosted |
| 5 | Nada resolvido | **Modo manual**: faz o push e imprime a URL de criação, sem falhar |

### Mapeamento por forge

| Forge | CLI | Comando | Substantivo na saída |
|---|---|---|---|
| `github` | `gh` | `gh pr create` | Pull Request |
| `gitlab` | `glab` | `glab mr create` | **Merge Request** |
| `azure` | `az` | `az repos pr create` | Pull Request |
| `bitbucket` | — (sem CLI oficial estável) | fallback para URL | Pull Request |

### Fluxo do comando

1. Valida que a branch não é `main`/`master` e segue `feat|fix|refactor/<slug>`
2. Valida governança: REQ e roadmap em `wip` (reaproveita `trackfw validate`)
3. Detecta squash-merges pendentes em outras branches
4. Revisa o que está staged — nunca `git add .`
5. Commit em Conventional Commits, sem sufixo de agente e sem trailer de modelo hardcoded
6. `git push origin <branch>`
7. Abre PR/MR conforme a forge, com corpo referenciando REQ, roadmap e critérios de aceite

### Degradação graciosa
Se o CLI da forge não estiver instalado, o comando **não falha**: conclui o push e imprime a URL de
criação. Verificação via `exec.LookPath`, reaproveitando o padrão de `externalCommandAvailable`.

## Consequences

### Positivas
- Fecha o ciclo `validate → ship`, tornando a entrega parte da governança e não um passo manual.
- O passo 2 amarra o ship à cadeia ADR→REQ→ROADMAP: sem roadmap em `wip`, não há entrega.
- Absorve a última skill de harness pessoal sem destino.
- Suporta GitLab e Azure DevOps, ampliando o público do produto para além do GitHub.

### Negativas e custos
- Comando novo nos 3 CLIs (Go, Node.js, Python), com paridade obrigatória.
- Novo campo em `config.ProjectConfig`, nova pergunta no wizard do `init` e nova detecção no `discover`.
- Superfície de teste maior: 4 forges × (CLI presente / ausente) × (host conhecido / self-hosted).
- Dependência de CLIs externos que o trackfw não controla (`gh`, `glab`, `az`), mitigada pela
  degradação graciosa.

## Alternatives Considered

- **Manter `gh pr create` fixo** — rejeitada: amarra um produto open-source ao GitHub.
- **Detectar a forge apenas pelo remote** — rejeitada: não resolve GitLab e Bitbucket self-hosted,
  que têm host arbitrário.
- **Apenas imprimir a URL, sem integrar CLIs** — rejeitada: perde a automação que dá valor ao fluxo,
  embora seja mantida como fallback.
- **Deixar o fluxo como skill em vez de comando** — rejeitada: skill não pode validar governança nem
  garantir paridade entre os 3 CLIs; o valor está justamente em ser gate executável.

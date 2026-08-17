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

### Governança no `ship` é gate duro, independente da configuração

Decisão tomada em 2026-07-26, durante a execução do ML-2A, e não prevista no desenho original.

O passo 2 (validação de REQ + roadmap em `wip`) **ignora** o baseline, o modo `lenient` e a
severidade por regra configurada em `rules:`. O `ship` é um gate de **entrega**: entregar sem
governança derrota o propósito do comando.

**Consequência assumida:** o `ship` é mais rígido que o `validate`. Um projeto em modo `lenient` pode
ter `trackfw validate` verde e `trackfw ship` abortando. Isso é inconsistência **visível e
deliberada**, não acidente — e precisa estar explícita:

- no texto de `trackfw ship --help`;
- em `docs/cli-parity.md`;
- na mensagem de erro do próprio comando, que deve dizer que a exigência não é afetada por `lenient`.

Projetos brownfield em adoção gradual continuam usando `git` diretamente até estarem conformes.

Implementação: `validator.CheckShipGovernance()`, função exportada que compõe
`validateBranchHasWIPRoadmap` e `validateWIPHasREQ` sem passar pelo `applyRule` — evitando duplicar a
lógica de busca de artefatos.

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

---

## Emenda 1 — 2026-08-16: o passo 1 e o gate de governança mudaram desde a decisão original

**Motivo:** o ML-3A da `ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0`
encontrou o passo 1 da sequência descrevendo um vocabulário que o comando não usa mais. O ADR é
**aceito** — isto é emenda, não reescrita: o texto original acima fica como registro do que foi
decidido em 2026-07-26.

Tudo abaixo foi **medido no binário atual**, não inferido do histórico.

### O que mudou

**1. Tipos de branch aceitos.** O passo 1 dizia `feat|fix|refactor/<slug>`. Hoje é
`feat|fix|refactor|chore|docs/<slug>` — `chore` e `docs` entraram em #177 (`branch new`) e #178
(`ship`), porque o fluxo de release e o de housekeeping não têm REQ e estavam sendo bloqueados por
um gate desenhado para mudança de produto.

**2. O gate de governança tem duas isenções que o ADR não previa.** O passo 2 é pulado quando:
- a branch é `chore/` ou `docs/` — `Governance: skipped (chore/docs branch)`;
- **todos** os arquivos staged são doc-only (`docs/`, `vault/`, `*.md`) — `Governance: skipped
  (doc-only change)`, espelhando a exceção de trivialidade do `CLAUDE.md` §7.

**3. O gate aceita roadmap em `wip/` OU `done/`.** `CheckShipGovernance` delega a
`validateBranchHasWIPRoadmap`, que casa o slug da branch contra `resolveWIPDirs` **e**
`resolveDoneDirs`. Confirmado por execução: com o roadmap já movido para `done/`, o `ship` imprime
`Governance: OK`.

> 🔴 **Divergência de mensagem, registrada e não corrigida aqui.** O texto do `--help` e o da
> mensagem de erro do passo 2 afirmam *"ship always requires REQ + roadmap in **wip/**"*. O
> comportamento real aceita `done/` também. A mensagem está mais estrita que o código — quem ler o
> erro pode mover um roadmap concluído de volta para `wip/` sem necessidade. Correção é mudança de
> string de usuário nos 3 CLIs, fora do escopo deste ML de documentação.

**4. `--no-pr`.** Existe uma flag para pular a criação do PR/MR, mantendo commit + push. Não constava
do desenho original, que tratava a abertura do PR como passo sempre executado (ou degradado para
impressão de URL). Torna o `ship` utilizável como *push governado* puro.

**5. O passo 4 é bloqueante.** "Revisa o que está staged" é, na prática, um erro duro: sem nada
staged o comando aborta com `nothing is staged — stage your files explicitly before running ship`.
O `ship` nunca executa `git add .` nem `git add -A`, por decisão explícita.

### Consequência operacional descoberta em uso

Como o `ship` **acopla** commit e push e exige algo staged, não existe caminho para "empurrar o que
já foi commitado" — e o push bruto é bloqueado pelo hook de guarda. Para empurrar trabalho já
commitado, o fluxo é desfazer o último commit com `git reset --soft HEAD~1` e deixar o `ship`
refazê-lo. Isso funciona, mas é contorno, não desenho.

**Não decidido aqui:** se o `ship` deve ganhar um modo push-only. Fica registrado como questão
aberta — vira REQ própria se o contorno incomodar.

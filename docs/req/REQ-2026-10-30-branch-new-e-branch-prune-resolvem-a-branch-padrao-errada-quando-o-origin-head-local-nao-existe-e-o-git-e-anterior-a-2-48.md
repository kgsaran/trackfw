---
status: Open
date: 2026-10-30
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-30-resolucao-da-branch-padrao-consulta-o-remoto-quando-o-symref-local-falta.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-30-branch-new-e-branch-prune-resolvem-a-branch-padrao-errada-quando-o-origin-head-local-nao-existe-e-o-git-e-anterior-a-2-48.md"
---

# REQ: branch new e branch prune resolvem a branch padrao errada quando o origin HEAD local nao existe e o git e anterior a 2.48

> Date: 2026-10-30 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Origem

Issue #633, de `lourivalgarciajunior`, medido em git 2.45.1 (Windows): `TestBranchNew_DryRunFetchesMasterDefault`
reprova em 2 de 2 corridas; passa no `windows-latest` (git 2.56). A medição e a sonda isolada do git
estão no issue e não são refeitas aqui.

## Motivation

`resolveBranchBase` (`internal/commands/branch.go:338–350`) faz `git fetch origin` e conta com ele
para criar `refs/remotes/origin/HEAD`. O git só faz isso a partir da 2.48. Abaixo disso, sem forge
respondendo, `resolveDefaultBranchWithSource` (`branch_prune_forge.go:22`) cai no literal `"main"`,
e um repositório `master` recebe `origin/main` como base.

**Sítios da mesma causa** (enumerados por `grep -rn "origin/HEAD\|resolveDefaultBranch" internal`):

| sítio | depende do symref | efeito sem symref |
|---|---|---|
| `branch.go:350` (`branch new`) | sim, depois do fetch | base `origin/main` inexistente → recusa |
| `branch_prune.go:423` (`branch prune`) | sim, depois de `fetch --prune` | padrão errada no prune local |
| `ship.go:765` | sim (resolver compartilhado) | a medir no ML-1A |
| `ship.go:477` e `release.go:275` (`defaultBaseBranch`, cadeia própria: symref → `"main"`) | sim | a medir no ML-1A; no `release` o valor é só cross-check (a forge decide) |

Contexto: o `cli-parity.md` já cita `git ≥2.48` no cenário 11 do `check-release-tag-parity.sh`, que
usa `remote.origin.followRemoteHEAD never` para **impedir** a auto-cura. É a mesma propriedade do
git, conferida só numa direção.

## Decisão

ADR-2026-10-30: nova fonte `git ls-remote --symref origin HEAD` entre o symref local e o literal.

## Acceptance Criteria

- [ ] **AC1** — `TestBranchNew_DryRunFetchesMasterDefault` reproduz o comportamento anterior à 2.48 em
      qualquer git (`git config remote.origin.followRemoteHEAD never` no fixture) e passa; o comentário
      "after fetch, origin/HEAD is updated" deixa de afirmar a premissa.
- [ ] **AC2** — contra-braço: com a fonte nova removida, o AC1 reprova no git 2.54 desta máquina.
- [ ] **AC3** — teste equivalente para o `branch prune` num repositório `master` sem symref.
- [ ] **AC4** — testes unitários do parse: aceita só `ref: refs/heads/<nome>\tHEAD`; saída vazia, erro
      e formato estranho caem no literal; a fonte nova devolve `fromForge=false`.
- [ ] **AC5** — `ship` e `defaultBaseBranch` medidos num repositório `master` sem symref; se o resultado
      estiver errado, corrigidos pela mesma fonte (mesma causa); se não, a medição fica escrita aqui.
- [ ] **AC6** — `go test ./internal/commands/ -count=1` verde (os fakes de `gitExec` dos testes
      existentes não podem passar a resolver errado com a chamada nova).

## Negative Scope

- Não muda a ordem forge → symref, nem a regra de não comparar forge com symref.
- Não muda as pré-condições de remoção remota do `branch prune` (ADR-2026-10-12 D4).
- Não declara versão mínima de git.
- Não usa `git remote set-head --auto` (escreveria no repositório do usuário).

## Linked ADR
ADR: docs/adr/ADR-2026-10-30-resolucao-da-branch-padrao-consulta-o-remoto-quando-o-symref-local-falta.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-30-branch-new-e-branch-prune-resolvem-a-branch-padrao-errada-quando-o-origin-head-local-nao-existe-e-o-git-e-anterior-a-2-48.md

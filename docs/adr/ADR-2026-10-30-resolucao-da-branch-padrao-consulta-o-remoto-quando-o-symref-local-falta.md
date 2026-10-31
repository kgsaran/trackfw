---
status: Accepted
date: 2026-10-30
author: "zeus-tf"
---

# ADR: resolução da branch padrão consulta o remoto quando o symref local falta

> Date: 2026-10-30 | Status: Accepted

Estende: `ADR-2026-10-12-branch-prune-apaga-a-branch-remota-so-pelo-sinal-do-forge-e-condicionada-ao-tip-avaliado.md` (A7)

## Context

`resolveDefaultBranchWithSource` (`internal/commands/branch_prune_forge.go`) resolve a branch padrão
em três fontes: forge (`gh api`), symref local `refs/remotes/origin/HEAD` e o literal `"main"`.
Usam essa cadeia o `branch new`, o `branch prune` e o `ship`.

O `branch new` e o `branch prune` fazem `git fetch origin` antes e contam com o fetch para criar o
symref. Esse comportamento só existe a partir do git 2.48 (issue #633, medido pelo autor em git
2.45.1). Abaixo disso, num remoto sem `origin/HEAD` local e sem forge respondendo, a cadeia cai no
literal `"main"` e falha num repositório cuja branch padrão é outra.

## Decision

1. **Nova fonte entre o symref local e o literal:** `git ls-remote --symref origin HEAD`. Ela pergunta
   ao remoto qual é o HEAD dele, não depende da versão do git (existe desde a 2.8) e não escreve nada
   no repositório do usuário.
2. **A ordem das fontes anteriores não muda.** A forge continua autoritativa e sem cross-check
   (nota do vault `forge-anchoring-branch-name-must-be-unconditional-not-cross-checked-2026-08-19`);
   o symref local continua antes do remoto.
3. **A fonte nova não conta como forge** (`fromForge=false`). As pré-condições de remoção remota do
   `branch prune` (ADR-2026-10-12 D4) não mudam.
4. **Só a linha `ref: refs/heads/<nome>\tHEAD` é aceita.** Saída vazia, erro, estouro de prazo ou
   formato diferente caem no literal, como hoje. O `ls-remote` já tem prazo de rede
   (`networkSubcmds`, ADR-2026-10-12 de operação de rede).

## Consequences

- Positivas: a cadeia deixa de depender da versão do git; uma versão mínima de git não precisa ser
  declarada por causa disso.
- Negativas: quando o symref local falta e a forge não responde, há uma chamada de rede a mais.
  Offline, ela falha dentro do prazo e o resultado é o mesmo de hoje.

## Alternatives Considered

- **`git remote set-head origin --auto` antes de resolver.** Também independe de versão, mas escreve
  uma ref no repositório do usuário, inclusive em `--dry-run`. Rejeitada.
- **Declarar git ≥ 2.48 como requisito.** Transfere ao usuário um defeito que o produto consegue
  evitar. Rejeitada.

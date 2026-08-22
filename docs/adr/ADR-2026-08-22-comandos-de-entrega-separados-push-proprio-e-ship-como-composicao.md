---
status: Accepted
date: 2026-08-22
author: "Zeus (Arquiteto)"
---

# ADR: Comandos de entrega separados — `push` próprio e `ship` como composição

> Date: 2026-08-22 | Status: Accepted

## Context

O `git-branch-guard` bloqueia `git push` bruto e manda o usuário para `trackfw ship`. Existem hoje
dois comandos de escrita no trilho:

- `trackfw commit -m` — commita, **não** empurra. Anuncia-se no próprio help como *"the missing
  intermediate step between raw `git commit` and `trackfw ship`"*.
- `trackfw ship -m` — commita, empurra e abre PR, nos 7 passos documentados em `docs/cli-parity.md`.

**Quem usa `trackfw commit` fica sem saída sancionada.** Medido em 2026-08-22, durante o
encerramento da REQ do hook relativo:

```
trackfw commit -m "..."   → commit criado                                     ok
git push                  → bloqueado pelo guard: "Use `trackfw ship`"
trackfw ship --no-pr      → "nothing is staged — stage your files explicitly"
```

O caminho sem `-m` do `ship` existe (`pushOnly`, `internal/commands/ship.go:397`), mas está
condicionado a `--force-with-lease`, que por sua vez exige **PR já aberto** (passo 2.5,
`ship.go:318-364`). Para um commit local ainda não empurrado, nenhuma das portas abre.

A saída que restou foi `git reset --soft HEAD~1` + `ship`, ou seja: **desfazer trabalho correto para
caber na ferramenta**. Isso é o sintoma que a decisão precisa eliminar — e o `reset` só foi seguro
porque o push havia sido bloqueado antes de qualquer efeito remoto. Se o commit já estivesse
publicado, a mesma manobra desincronizaria a branch.

### O que o guard ensina é parte do problema

A REASON do ramo `push` cita literalmente `trackfw ship`. O guard é a superfície que ensina o
caminho certo no momento exato do erro; apontar para o comando que **não resolve** o caso do usuário
recria o beco sem saída a cada tentativa.

## Decision

**1. `trackfw push` passa a existir nos 3 CLIs**, como comando de primeira classe: empurra commits
já criados, **nunca commita** e **nunca abre PR**.

Ele reusa, sem duplicar, os passos 1–3 do `ship`:

| Passo | Reuso | Go | Node | Python |
|---|---|---|---|---|
| Nome da branch (bloqueio em `main`/`master`; padrão `feat\|fix\|refactor\|chore\|docs/<slug>`) | `isShipBranch` / `isGatedShipBranch` | `ship.go:729,736` | `runner.js:101,112` | `runner.py:99,110` |
| Governança (roadmap em `wip/` **ou** `done/`; `chore`/`docs` isentos) | `CheckShipGovernance` | `validator.go:2292` | `runner.js:156` | `runner.py:165` |
| Squash-merge pendente (advisory) | `detectPendingSquashMerges` | `ship.go:771` | `runner.js:211` | `runner.py:189` |
| Montagem do push (`-u` quando não há upstream) | `buildPushArgs` | `ship.go:795` | `runner.js:234` | `runner.py:225` |
| `--force-with-lease` (exige PR/MR aberto) | passo 2.5 | `ship.go:318` | `runner.js:516` | `runner.py:524` |

**2. O vocabulário de entrega passa a ser composicional**, e é assim que a documentação o descreve:

```
trackfw commit -m "..."      commita
trackfw push                 empurra
trackfw ship -m "..."        commit + push + PR   (composição das duas anteriores + forge)
```

**3. A REASON do guard, no ramo `push`, passa a citar `trackfw push`** — o comando mínimo suficiente
para a ação bloqueada. `ship` continua nomeado onde a ação bloqueada é `commit`.

**4. `ship` não muda de contrato.** O `-m` continua obrigatório no seu caminho normal, e a exceção
`pushOnly` continua atrelada a `--force-with-lease`. Nenhuma flag do `ship` é relaxada.

## Consequences

**Positivas**
- O ciclo `commit → push` fecha sem `reset`, e sem publicar PR antes da hora.
- O guard passa a ensinar o comando que resolve, no momento do bloqueio.
- `ship` fica descrito como o que é — composição —, o que torna o help dos três comandos coerente
  entre si.
- Cobre casos que `commit --push` não cobriria: commit feito em invocação anterior, reempurrar após
  rebase local, empurrar trabalho de outro agente já auditado.

**Negativas e riscos aceitos**
- **Quarto comando de escrita** na superfície do CLI. Mitigado por não introduzir gate novo: `push`
  não tem regra própria, só reuso.
- **A REASON do guard está duplicada em 5 arquivos** (script canônico, referência do validator e os
  3 geradores) e a sincronia é cobrada por 4 gates de paridade. Mudar a string exige tocar os cinco
  no mesmo lote — é o ML de maior risco desta entrega.
- **Harnesses instalados ficam defasados** até o usuário rodar `trackfw update`. A mensagem antiga
  continua correta (o `ship` funciona), só não é a mais útil. Não é regressão.
- Em Python, `_build_push_args`, `_detect_pending_squash_merges` e `_all_doc_only` são privados — é o
  único stack onde o reuso exige renomear ou importar símbolo privado. Decisão de implementação
  registrada no roadmap, não aqui.

## Alternatives Considered

**Relaxar o `ship` para aceitar push-only sem `--force-with-lease`.** Diff menor e nenhum comando
novo. Rejeitada: sobrecarrega um comando cujo contrato publicado diz que `-m` é obrigatório, e
mantém o nome errado para a ação — "ship" carrega a semântica de abrir PR, que é justamente o que o
usuário **não** quer nesse momento. Também não conserta o que o guard ensina.

**`trackfw commit --push`.** Resolve o fluxo feliz e nada além dele: um commit criado em invocação
anterior — exatamente o caso medido — continua sem saída. Rejeitada por não fechar a classe.

**Allowlist no guard para `git push` do próprio usuário.** Rejeitada de saída: o guard não tem, e não
deve ter, escape hatch. A verificação feita nesta análise confirma que o CLI **não precisa** de
bypass — o `git push` que ele executa é subprocesso e nunca atravessa o hook, que só inspeciona o
comando da ferramenta Bash do agente. É por isso que `ship` funciona hoje, e `push` funcionará pelo
mesmo mecanismo.

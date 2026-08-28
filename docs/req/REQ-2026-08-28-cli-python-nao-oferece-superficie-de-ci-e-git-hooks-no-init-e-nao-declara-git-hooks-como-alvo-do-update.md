---
status: Open
date: 2026-08-28
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: CLI Python não oferece superfície de CI e git hooks no `init`, e não declara `git-hooks` como alvo do `update`

> Date: 2026-08-28 | Status: Open

## Motivation

A regra dura de paridade do projeto é que os 3 CLIs tenham **as mesmas funcionalidades, funcionando
exatamente igual**. O `init` do CLI Python viola isso em dois eixos, e o `update` num terceiro.

**Medido em 2026-08-28:**

| superfície | Go | Node.js | Python |
|---|---|---|---|
| `init` pergunta/aceita **CI** (`github-actions`, `gitlab-ci`, `none`) | sim (`internal/commands/init.go:231-236`) | sim (`npm/src/commands/init.js:183-184`) | **não** — nenhuma flag, nenhum prompt |
| `init` pergunta/aceita **git hooks** (`husky`, `lefthook`, `none`) | sim (`init.go:222-227`) | sim (`init.js:174-175`) | **não** |
| `update` **executa** injeção de hook | sim | sim | **sim** — `_update_hooks_surgical`, chamado em `pypi/trackfw/commands/update.py:315` |
| `update` **declara** `git-hooks` em `PROJECT_TARGET_IDS` | sim | sim | **não** (`update.py:107`) |

O `init` do Python (`pypi/trackfw/commands/init.py:22-45`) expõe `--project-name`,
`--namespacing`, `--agents`, `--wip-limit`, `--ai-tools`, `--identity-preset` e `--forge`. Não há
`--ci` nem `--hooks`. Consequência direta: **um `trackfw.yaml` escrito pelo `init` do Python nasce
sem `hooks:` e sem `ci:` preenchidos**, então o projeto não tem gate de governança no CI nem
validação no pre-commit — e o usuário não é perguntado, nem avisado.

O caso do `git-hooks` é o mais traiçoeiro dos três, e é o padrão que a release 7.3.0 inteira
combateu: **o trabalho é feito, mas o controle não aparece no relatório.** `_update_hooks_surgical`
existe, espelha Go e Node linha a linha, e roda de verdade — mas como `git-hooks` não está em
`PROJECT_TARGET_IDS`, o alvo nunca é listado na saída do `update`. Quem lê o relatório do Python
conclui que hooks não são gerenciados; quem lê o código vê que são. As duas leituras discordam, e a
saída do comando é a que o usuário usa para decidir.

## Acceptance Criteria

- [ ] **AC1** — `trackfw init` no CLI Python aceita `--ci` com os mesmos 3 valores de Go/Node
      (`github-actions`, `gitlab-ci`, `none`) e escreve a chave `ci:` no `trackfw.yaml`.
- [ ] **AC2** — `trackfw init` no CLI Python aceita `--hooks` com os mesmos 3 valores
      (`husky`, `lefthook`, `none`) e escreve a chave `hooks:` no `trackfw.yaml`.
- [ ] **AC3** — Valor inválido em qualquer das duas flags falha **alto** (exit não-zero, mensagem
      nomeando os valores aceitos), não silenciosamente. É o mesmo tratamento que `--identity-preset`
      já recebe em Go (`internal/commands/init.go:69,86`) — o comentário lá diz explicitamente que a
      falha alta existe para não virar no-op silencioso em CI.
- [ ] **AC4** — `trackfw.yaml` gerado pelos 3 CLIs com as mesmas flags é **byte-idêntico** nas chaves
      `ci:` e `hooks:`. Gate falsificável cobre.
- [ ] **AC5** — `git-hooks` passa a constar em `PROJECT_TARGET_IDS` do Python, na **mesma posição**
      da lista de Go e Node, e o alvo é reportado com estado honesto (`updated`/`skipped`/`missing`)
      pelo mesmo mecanismo dos demais.
- [ ] **AC6** — O docstring de módulo de `pypi/trackfw/commands/update.py:18-32` e o comentário de
      `PROJECT_TARGET_IDS` (`:102-106`), que hoje afirmam que `ci-workflow` e `git-hooks` são
      "Go/Node.js-only" porque "this runtime has no CLI surface to configure a CI system or a
      git-hooks framework at `init` time", são corrigidos — a premissa deixa de valer com AC1/AC2.
- [ ] **AC7** — A seção de `docs/cli-parity.md` que declara a lista fixa de alvos de projeto passa a
      registrar os 3 CLIs com a mesma lista, sem exceção por runtime, anotada com `gate=`.
- [ ] **AC8** — Nenhuma regressão no `init` do Python para quem **não** passa as flags novas: o
      comportamento atual (chaves vazias) é preservado como default, ou o default é declarado
      explicitamente na REQ da implementação. Gate cobre as duas direções.
- [ ] **AC9** — `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` → exit 0.

## Negative Scope

- **Não** implementar o gerador de workflow de CI no Python nem o alvo `ci-workflow` no `update` —
  isso é entregue por
  `REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-nao-ha-como-pinar`
  (ML-2C). **Esta REQ depende daquela**: sem o gerador, `--ci` não tem o que gerar.
- **Não** portar o wizard interativo (`huh` em Go, prompts em Node) para o Python. O escopo é a
  superfície não-interativa por flag, que é a que CI e automação usam. Se o Python deve ganhar
  wizard interativo é decisão de produto separada.
- **Não** mexer em `--forge`, `--ai-tools`, `--identity-preset` ou qualquer outra flag existente.
- **Não** migrar `trackfw.yaml` de projetos já criados pelo `init` do Python sem as chaves.

## Observação de higiene (fora de escopo, registrado)

`pypi/build/lib/trackfw/` contém uma cópia velha do CLI Python (com `_update_hooks_surgical` numa
versão anterior, em `:131`). Não é rastreada pelo git, mas polui todo `grep -r` na árvore e já
produziu contagem divergente em auditoria. Vale um `make clean` ou entrada de `.gitignore` mais
agressiva — não nesta REQ.

## Linked ADR
<!-- Nenhum ADR novo: a decisão que governa esta REQ é a regra dura de paridade dos 3 CLIs, já
     estabelecida em CLAUDE.md e docs/cli-parity.md. Esta REQ fecha uma violação dela, não decide
     nada novo. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:

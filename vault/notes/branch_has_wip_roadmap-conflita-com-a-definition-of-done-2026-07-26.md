---
title: "branch_has_wip_roadmap conflita com a Definition of Done"
tags: [validator, governanca, kanban, gate]
date: 2026-07-26
related: []
---

# branch_has_wip_roadmap conflita com a Definition of Done

## Problem

Ao encerrar a REQ de convergência do harness, o roadmap foi movido de `wip/` para `done/` **ainda na
branch de feature**, como manda a Definition of Done. Imediatamente `trackfw validate` passou a
reprovar:

```
✗ branch "feat/convergencia-do-harness-pessoal-para-o-trackfw" is a feat/fix/refactor branch
  but no roadmap is in wip/ — create governance artifacts first
```

Impasse no momento do PR:

- Mover o roadmap para `done/` na branch → `validate` reprova localmente.
- Deixar em `wip/` para ficar verde → **viola a Definition of Done**, que o próprio produto passou a
  pregar ("build e testes verdes não encerram o trabalho; o fluxo Kanban encerra").

O gate pune exatamente o comportamento que a documentação gerada exige.

**Quem é afetado — verificado, não presumido.** O CI **deste repositório** (`quality.yml:71-74`) roda
`trackfw validate --json` dentro de um `mktemp -d`, ou seja, **não valida o estado de governança do
próprio repo** — logo o PR desta REQ não é bloqueado.

O impasse atinge os **projetos usuários**: o workflow que o trackfw gera
(`writeCIWorkflow` → `.github/workflows/trackfw-gate.yml`) roda `trackfw validate` **na raiz do
projeto** a cada pull request. Nesses repositórios, encerrar o roadmap conforme a DoD reprova o PR.
Ou seja: o defeito é invisível para quem desenvolve o trackfw e bloqueante para quem o usa — a pior
combinação possível para ser descoberto tarde.

## Root cause

`internal/validator/validator.go`, regra `branch_has_wip_roadmap` (~linha 1506):

```go
if !strings.HasPrefix(branch, "feat/") && !strings.HasPrefix(branch, "fix/") && !strings.HasPrefix(branch, "refactor/") {
    return ...
}
wipDirs := resolveWIPDirs(cfg)
// ... percorre apenas wip/
if len(wipFiles) == 0 { → violação }
```

A regra só enxerga `wip/`. Não tem noção de "o trabalho desta branch terminou e o roadmap já foi para
`done/`". O `branchSlug` é comparado apenas contra os arquivos em `wip/`.

Ela foi desenhada para pegar o anti-padrão "codar sem governança" — branch de feature aberta sem
roadmap nenhum. Não distingue esse caso de "roadmap concluído nesta mesma branch".

## Solution

Nenhuma correção aplicada — é decisão de produto, registrada para decisão do KG.

1. **Aceitar roadmap em `done/` cujo slug casa com a branch.** A regra procuraria o slug em `wip/`
   **e** `done/`, reprovando só quando não houver roadmap correspondente em nenhum dos dois.
   Preserva a intenção original e destrava o encerramento. Risco: mover para `done/` cedo demais e
   seguir codando sem gate.
2. **Só mover para `done/` depois do merge.** Mantém a regra, mas contraria a DoD e exige passo
   manual pós-merge — justamente o "rabo" que o kanban existe para evitar.
3. **Rebaixar a `warning` quando existe roadmap em `done/` com o slug da branch.** Sinaliza sem
   bloquear o CI.

Recomendação: opção 1, reaproveitando `normalizeBranchSlug` já existente, aplicada também a `done/`.

**Workaround imediato:** nesta REQ o roadmap fica em `done/` e a violação é conhecida e documentada
aqui.

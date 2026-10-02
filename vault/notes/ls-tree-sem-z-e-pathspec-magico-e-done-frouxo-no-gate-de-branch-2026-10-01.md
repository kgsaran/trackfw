# Ler árvore do git para decidir governança: `-z`, `--literal-pathspecs`, e por que `done/` não pode governar por inferência

> 2026-10-01 | REQ-2026-10-01 (estado que governa a branch) | issues #494, #490 | zeus-tf

## O problema

O gate branch↔roadmap passou a perguntar "este roadmap de `done/` já estava em `done/` na base?",
lendo a árvore de `origin/main` com `git ls-tree`. Há três armadilhas, e as três erram na direção
**frouxa**: o roadmap parece ausente da base e o gate aceita.

1. **Sem `-z`, nome não-ASCII sai citado.** Com `core.quotepath=true` (o padrão), `ls-tree
   --name-only` emite `"docs/a\303\247\303\243o.md"` entre aspas. `filepath.Base` sobre isso não
   casa o nome real. Medido em `auditsurface.gitLsTree` e `mdBasenamesInGitTree`, que já existiam e
   tinham o defeito antes desta REQ. Correção: `-z` + `bytes.Split(out, []byte{0})`.
2. **Sem `--literal-pathspecs`, o `roadmap_dir` vira pathspec mágico.** `:(exclude)…`/`:(icase)…`
   vindos do `trackfw.yaml` são interpretados pelo git. A medição divergiu entre agentes: o Hades viu
   rc=0 com `fatal` no stderr, e o teste final viu rc=128. Os dois comportamentos são ruins: o
   primeiro é silencioso, e o segundo força a degradação do D3. Correção: `git --literal-pathspecs
   ls-tree …` (a opção global vem **antes** do subcomando).
   ⚠️ **`:` é válido em nome de diretório no macOS** (`mkdir ":(exclude)rm"` funciona e o git
   versiona). Um agente afirmou o contrário e escreveu um teste que não mordia em cima disso.
3. **A inferência contra `done/` é frouxa nas duas pernas.** Das 217 branches `feat|fix|refactor`
   históricas, 22 casam `done/` só por sobreposição de tokens, e **126 casam mais de um** roadmap
   concluído. O "contém" de slug curto também casa assunto alheio. Apertar só a sobreposição não
   resolve; o que resolve é perguntar outra coisa ("a própria branch moveu este roadmap?").

## Por que a ponta da base e não o `merge-base`

O checkout raso do CI (profundidade 1 nas duas pontas) não tem ancestral comum. A árvore da ponta
só exige o fetch de 1 commit, que o `trackfw-gate.yml` gerado já faz para a âncora de severidade.

## Como medir sem tocar a árvore

`go test -overlay <json>` substitui um arquivo do pacote por outro, sem mexer no checkout. Serve para:
- rodar uma medição com funções não exportadas (um `_test.go` temporário via overlay);
- provar que um teste **morde**: troque o arquivo de produção por uma cópia sabotada (por exemplo,
  sem a flag) e exija FAIL.

## Onde está

`internal/validator/branchlink.go` (`ResolveBranchRoadmapForExisting`) ·
`internal/validator/validator.go` (`mdBasenamesInGitTreeWithError`) ·
`internal/auditsurface/auditsurface.go` (`gitLsTree`) · ADR-2026-10-01-estado-que-governa-a-branch-…

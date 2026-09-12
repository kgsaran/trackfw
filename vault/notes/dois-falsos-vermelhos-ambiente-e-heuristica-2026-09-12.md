# Dois falsos vermelhos: um de ambiente, um de heurística

> 2026-09-12 · descoberto ao rodar `make quality` e `trackfw push` no fechamento da release v7.6.0

Ambos custam o mesmo: **treinam quem lê a ignorar o aviso.** Nenhum dos dois quebra produto, e é
exatamente por isso que sobrevivem.

## 1. `check-branch-new-parity` reprova se a shell tiver `FORCE_COLOR`

**Sintoma.** `make quality` reprova com 3 cenários `go-vs-node/err` divergindo, e o diff é:

```
+(node:16899) Warning: The 'NO_COLOR' env is ignored due to the 'FORCE_COLOR' env being set.
+(Use `node --trace-warnings ...` to show where the warning was created)
```

**Causa.** O gate compara stderr **byte a byte** entre Go e Node. O Node emite esse warning quando
`FORCE_COLOR` está setado no ambiente herdado. O gate não sanea o ambiente antes de comparar.

**Prova.** `env -u FORCE_COLOR GO_BIN=bin/trackfw scripts/check-branch-new-parity.sh` → todos OK.

🔴 **Por que importa mais do que parece:** o CI não seta `FORCE_COLOR`, então o gate é verde lá — e
vermelho na máquina de qualquer contribuidor cujo terminal seta a variável. **O Claude Code seta
`FORCE_COLOR=3`.** Ou seja: o público que mais roda `make quality` localmente é justamente o que vê
o falso vermelho. Um gate que só é confiável no CI ensina a rodar menos o gate.

**Correção sugerida (não feita):** o gate deve invocar os runtimes com ambiente saneado — `env -u
FORCE_COLOR -u NO_COLOR` — ou filtrar linhas `(node:NNNN) Warning:` do stderr antes de comparar. A
primeira é melhor: filtrar esconde warnings legítimos.

## 2. O aviso de branch não-mergeada do `push`/`ship` é vácuo neste projeto

**Sintoma.** `trackfw push` avisa `branch "X" appears to have unmerged changes vs origin/main` para
uma branch que está integrada.

**Causa.** `internal/commands/ship.go:778` (e os espelhos em `npm/src/ship/runner.js:223`,
`pypi/trackfw/ship/runner.py:222`) medem com:

```go
gitExec("branch", "-r", "--no-merged", "origin/main")
```

🔴 **`--no-merged` não detecta squash-merge** — e squash é a estratégia padrão deste repositório.
O `CLAUDE.md` global documenta isso explicitamente no protocolo de branch única, e prescreve o teste
correto: comparar os **arquivos tocados** contra o tip da main.

**Prova.** `origin/fix/fechar-os-grupos-de-falha-de-windows-por-causa-raiz` dispara o aviso. Pelo
teste de arquivos-tocados:

```bash
mb=$(git merge-base origin/main "$b")
touched=$(git diff --name-only "$mb" "$b")
git diff --name-only origin/main "$b" -- $touched   # → vazio ⇒ integrada
```

**Consequência.** O aviso é falso **por construção** para a estratégia de merge do próprio projeto.
Ele não distingue nada: dispara para toda branch integrada por squash que ainda exista no remoto.

**Correção sugerida (não feita):** trocar a heurística pelo teste de arquivos-tocados que o
`CLAUDE.md` já prescreve, nos 3 runtimes. Enquanto não for trocado, o aviso deve ser lido como
"existe branch remota antiga", não como "existe trabalho pendente".

## Padrão comum

Os dois são a mesma família dos quatro gates que **estavam corretos no dia 1 e pararam de medir**:
o artefato existe, roda, e produz um veredito que não corresponde à pergunta que ele afirma
responder. A diferença é a direção do erro — aqui é falso **positivo**, não falso negativo — e o
falso positivo é o mais barato de conviver e o mais caro no acumulado, porque erode a atenção que
faz o falso negativo ser pego.

---
name: cenario18-enumera-gates-com-go-bin
description: Todo gate novo que cite GO_BIN entra automaticamente no Cenário 18 do check-gates-falsify (proibição de mutar a árvore)
metadata:
  type: project
---

O Cenário 18 (`no-repo-mutation`) do `scripts/check-gates-falsify.sh` **enumera em tempo de execução**
todo `scripts/check-*.sh` cujo fonte contenha `GO_BIN` ou `bin/trackfw`, roda cada um numa cópia limpa
do repositório e compara o OID da árvore antes/depois. Gate novo entra na cobertura **por construção**
— não há lista a editar.

**Why:** foi desenhado assim depois de um gate ficar fora de uma lista fixa de 4 e ser justamente o
que mutava a árvore. Consequência prática para quem escreve gate novo: ele **não pode** escrever nada
dentro do repositório (usar `mktemp -d`), e a cópia inclui arquivos **não commitados**, então a
fixture não precisa estar no índice para o cenário achá-la.

**How to apply:** ao entregar gate que invoque o binário, confirme que toda escrita vai para `$WORK`
de `mktemp`, e conte com o cenário 18 rodando o seu gate uma vez por `make quality` (custo de parede
soma no chunk 3).

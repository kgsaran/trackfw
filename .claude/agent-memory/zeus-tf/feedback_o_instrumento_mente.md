---
name: o-instrumento-mente
description: Antes de acreditar num resultado surpreendente, verifique o instrumento — zsh sem word-splitting, $? do pipe e ls com alias já produziram dado falso neste projeto
metadata:
  type: feedback
---

**Resultado surpreendente ⇒ suspeite primeiro do instrumento, não do dado.** Shell, pipe e alias
contaminam a medição em silêncio: o comando sai com exit 0 e devolve algo **plausível**.

**Why:** três ocorrências em um único dia (2026-09-16), todas minhas, todas quase virando conclusão:

| o que medi | instrumento | devolveu | era |
|---|---|---|---|
| artefatos de 18 issues presentes na árvore | `for a in $arts` em **zsh** | "ausente" em **tudo** | metade presente |
| exit code do `install.sh` recusando Windows | `sh x.sh \| tail` | `0` | **1** |
| nome do REQ vs. arquivo em disco | `ls` com alias de ícone | "diferentes" | **idênticos** |

O `zsh` **não** faz word-splitting de variável não citada: a lista inteira virou um pathspec só, nada
casou, e tudo saiu "ausente". O `$?` depois de um pipe é do **último** comando, não do script. E o
`ls` desta máquina tem alias que injeta um caractere antes do nome.

🔴 O sinal comum é **unanimidade**: quando todo item de um corpus recebe o mesmo veredito, o teste
provavelmente está quebrado. Foi assim nos três. Em 2026-09-12 essa mesma classe (o `zsh`) apagou uma
branch com trabalho não integrado.

**How to apply:**

- Varredura com lista ⇒ `bash -c` com `mapfile`/array citado, **nunca** `zsh` com `$var` solto.
- Exit code ⇒ meça **sem pipe**: `cmd > out.txt 2>&1; echo $?`. Se precisar do pipe, `PIPESTATUS[0]`.
- Comparar nomes de arquivo ⇒ `find`/`git ls-files`, nunca `ls` (alias com ícone, cor, `-F`).
- **Contra-braço obrigatório** quando o veredito é uniforme: rode o mesmo teste contra um caso que
  você *sabe* ter o resultado oposto. Se ele também sair igual, o teste está quebrado.

Relacionado: [[medir-com-a-regra-nao-com-grep]] — lá o erro é medir o **proxy** errado; aqui o
instrumento mente sobre o que leu. E [[verificacao-visual-obrigatoria]], mesma família: gate verde
não é evidência de que se mediu o objetivo.

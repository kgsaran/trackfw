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

**Quarta ocorrência (2026-09-17), e a mais traiçoeira:** um laço para apagar branches integradas
devolveu `apagadas: 0` e **nenhuma linha de diagnóstico**. Causa: `git rev-parse @{upstream}` sai com
**rc=128 justamente quando o remoto sumiu** — ou seja, o comando falha exatamente no caso que eu
queria detectar, e o `|| continue` engoliu todas as 22 branches. O certo é
`git for-each-ref --format='%(refname:short)|%(upstream:track)'`, onde remoto apagado vem como
`[gone]` em vez de erro. 🔴 **Zero também é um resultado uniforme** — a regra da unanimidade vale
para "nada aconteceu", não só para "tudo deu igual".

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

## `ls` com ícones de Nerd Font — reincidi em 2026-09-23

O `ls` desta shell emite **ícone de Nerd Font** (área de uso privado, ex.: `\uf48a`) **antes** do
nome. Capturar nome de arquivo com `ls | grep | head -1` embute o ícone **e** o espaço seguinte:

```
roadmap: "docs/roadmaps/backlog/\uf48a ROADMAP-....md"     ← caminho quebrado, gravado numa REQ
```

🔴 **E o `grep` mostra o ícone como se fosse espaço**, então a inspeção visual não denuncia. Só
`[hex(ord(c)) for c in ...]` revelou. Meu `str.replace('backlog/ ROADMAP', ...)` falhou com
`AssertionError` — e foi o assert que me salvou de achar que tinha corrigido.

**Use `find ... -name` ou glob do shell para capturar nome de arquivo. Nunca `ls`.**
Para limpar um já contaminado: `re.sub(r'[\ue000-\uf8ff]\s*','',s)`.

Esta é a **segunda** vez: o aviso sobre `ls` com alias já estava nesta nota.


## Ocorrência 2026-09-24 — a quinta vez, e eu estava auditando exatamente isto

Auditando um ML **sobre** `grep` que sai 1, medi o braço de controle assim:

```bash
bash -c 'set -eu; set +o pipefail; v=$(grep -o ausente <<<"x"); echo "NAO DEVERIA CHEGAR"' 2>&1 | head -2
echo "rc=$?"     # -> 0
```

Li **rc=0** e quase registrei que o script sobrevivia. **O `0` é do `head`.** Refeito sem cano:
**rc=1**, o script morre.

🔴 **O padrão a vigiar não é o comando — é o momento.** As cinco ocorrências desta campanha
aconteceram quando eu estava com pressa de confirmar algo que já acreditava. O `| head`/`| tail`
entra para "encurtar a saída" e leva o rc junto.

**Regra operacional:** se a linha seguinte à do comando usa `$?`, o comando **não pode** ter cano.
Redirecione para arquivo e leia depois.

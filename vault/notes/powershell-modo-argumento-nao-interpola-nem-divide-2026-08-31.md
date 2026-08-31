---
title: PowerShell — em modo argumento a vírgula não constrói array E a variável não é interpolada
tags: [powershell, windows, ci, gotcha, diagnostico]
date: 2026-08-31
related: [[lstat-nao-ve-junction-e-guarda-de-folha-nao-olha-ancestral-2026-08-31]]
---

## Sintoma

Num step `shell: pwsh`, isto falha:

```powershell
$blob = (git hash-object -w link_target.tmp).Trim()
git update-index --add --cacheinfo 120000,$blob,mylink
# error: option 'cacheinfo' expects <mode>,<sha1>,<path>
```

A mensagem do git sugere *"o argumento chegou partido"*, e leva direto ao diagnóstico errado.

## Causa raiz — e por que o diagnóstico óbvio está errado

Medido com `pwsh` 7 e um executável nativo:

```
modo expressão   $arr = 120000,$blob,"mylink"   →  Object[], 3 elementos      ← vírgula CONSTRÓI array
modo argumento   & exe 120000,$blob,mylink      →  1 arg: '120000,$blob,mylink'
                                                     ↑ nem array, e `$blob` LITERAL, não interpolado
forma citada     & exe "120000,$blob,mylink"    →  1 arg: '120000,aaaa...,mylink'   ← interpolado
```

Duas coisas ao mesmo tempo, e a segunda é a que morde:

1. A vírgula **constrói array em modo expressão**, mas **não** em modo argumento (invocação de
   executável nativo). O token chega como **uma** string.
2. Dentro de um token **não-citado** que começa com bareword, PowerShell **não expande a variável**.
   O git recebeu o texto literal `$blob` como sha1.

O erro `expects <mode>,<sha1>,<path>` é **compatível com as duas hipóteses** — é por isso que ele
não discrimina, e por isso vale medir em vez de deduzir.

## Por que importa: o remédio muda com o diagnóstico

- Se o problema fosse *"virou array de 3"*, o remédio seria **juntar** os argumentos (escapar a
  vírgula, usar `--%`, montar um array e passar com `@`).
- O problema real é *"não interpolou"*, e o remédio é **citar a string**:

```powershell
$cacheinfo = "120000,$blob,mylink"
git update-index --add --cacheinfo $cacheinfo
```

Quem aplicasse a correção pela leitura errada escaparia vírgulas e **continuaria sem interpolação** —
o comando passaria a montar um argumento sintaticamente válido com um sha1 literal `$blob`, falhando
por outra mensagem ou, pior, silenciosamente.

## Regra prática

Em `shell: pwsh`, **toda** montagem de argumento que contenha variável e pontuação (vírgula,
dois-pontos, igual, chaves) vai para uma variável intermediária **entre aspas duplas**, e essa
variável é que é passada:

```powershell
$arg = "chave=$valor,outra=$outro"
exe --opt $arg
```

Não é preferência de estilo — é a diferença entre interpolar e não interpolar.

## E verifique o efeito, não o exit code

Um `git update-index` pode devolver `exit 0` sem ter feito o que se queria. A verificação certa lê o
**efeito**:

```powershell
git ls-files --stage mylink    # imprime mode/blob/path reais do índice
```

## Como foi descoberto

Step 19 (`Pergunta 7`) da sonda de Windows falhou no run
[`33338382066`](https://github.com/kgsaran/trackfw/actions/runs/33338382066). O arquiteto
diagnosticou "vírgula constrói array" e **escreveu isso na REQ e no roadmap**; `ares-tf` reproduziu
localmente, contestou, e a medição com `pwsh` confirmou que a contestação estava certa. A versão
errada ficou registrada como errada nos dois artefatos, em vez de ser reescrita.

Lição além do PowerShell: **uma mensagem de erro compatível com a sua hipótese não é evidência a
favor dela.** Só discrimina o teste que separa as hipóteses.

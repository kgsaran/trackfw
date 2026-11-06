# Guard de git: dividir o comando antes de extrair a substituição esconde o `git` de dentro

**Data:** 2026-11-05 · **REQ:** REQ-2026-11-05 (guards do primeiro usuário externo) · **ML:** ML-2G · **Issue:** #663

## Sintoma

`echo $(git push | cat)`, `OUT=$(git push && echo x)`, `cat <(git push | cat)` e
`bash -c 'echo $(git push | cat)'` davam `rc=0`. As formas sem operador dentro do corpo
(`echo $(git push)`) já davam `rc=2`. Foram quatro rodadas de correção pontual (ML-2C, 2E, 2F)
antes de enxergar a causa.

## Causa raiz

`matchSubcommandPOSIX` dividia a string em `|`, `&&`, `||`, `;` (`quoteAwareSplitPOSIX`) **antes**
de procurar corpos de `$( )`, crase, `<( )` e `>( )`. O corte acontecia dentro do corpo:
`echo $(git push | cat)` virava `echo $(git push` + `cat)`, e nenhum segmento tinha um corpo
balanceado para extrair.

Regra geral para quem mexer em analisador de shell: **substituição é estrutura aninhada; separador é
estrutura plana. Extraia o aninhado da string crua antes de qualquer divisão plana.** Cada remendo
por segmento fecha um operador e deixa o próximo aberto.

## Correção (ML-2G)

`extractRawCmdSubstBodies` varre a string inteira (já sem heredoc) antes do split, com controle de
aspas simples, aspas duplas, barra invertida fora de aspas e comentário `#` no início de palavra.
Cada corpo é reanalisado como comando completo com `depth+1`; acima de `maxWrapperDepth` o resultado
é `wrapper-not-analyzed` (bloqueia). A extração por segmento foi removida.

## Comportamentos que parecem furo e não são (medidos em bash 3.2, bash 5, zsh e sh)

- `` echo `echo "hi`; git push` `` dá `rc=0` no guard. Em todos os shells é **erro de sintaxe**
  (crase sem par) e nada executa. O executor do ML-2G registrou isso como furo pendente; não é.
  As variantes válidas (`` echo `echo ")"; git push` ``, `` echo `echo '"'; git push` ``,
  `` echo "`git push | cat`" ``) dão `rc=2`.
- `echo \$(git push)` passou de `rc=2` para `rc=0`. Fora de aspas, `\$` é `$` literal; o bash
  recusa com erro de sintaxe no `(` e o zsh trata como texto. Mudança correta.

## Como medir sem executar nada perigoso

Só via `trackfw guard git-branch --command '<cmd>'`, num diretório **com `trackfw.yaml`**. Sem esse
arquivo o guard fica inativo e tudo dá `rc=0`; caí nisso na própria auditoria. Para provar a semântica
do shell, use um payload inofensivo (`echo PAYLOAD_RAN`), nunca o `git` de escrita.

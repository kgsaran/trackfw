---
title: extending a case-block in gitBranchGuardScript breaks existing corrupt_literal P4 scenarios that span its closing `;;`
date: 2026-08-19
tags: [git-branch-guard, check-gates-falsify, corrupt_literal, scaffold.go]
---

## Contexto

ML-3A (ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md, Wave 3) adicionou
detecção de `git checkout -- <path>`/`git checkout .` dentro do bloco `checkout)` já existente do
`match_subcommand` (`internal/generators/scaffold.go`, const `gitBranchGuardScript`) — inserindo
código novo **entre** o for-loop de `-b`/`-B`/`--orphan` já existente e o `;;` que fecha o case.

## Sintoma

`scripts/check-gates-falsify.sh` Cenário 62b (que já existia, de um ML anterior — flag do
`checkout -b` fora da primeira posição de token) começou a falhar com:

```
[s62b-go-checkout-token-scan-reverted-to-pre-ml4b] expected exactly 1 occurrence of pattern, got 0
```

`corrupt_literal` exige `count == 1` do literal-alvo no arquivo fonte (falha alto e explícito se não
achar, por desenho — nunca silencia). O literal-alvo do Cenário 62b incluía o `;;` de fechamento do
case **logo após** o for-loop de `-b`, porque na época em que o cenário foi escrito esse `;;` vinha
imediatamente depois do for-loop. Depois do ML-3A, o `;;` passou a vir depois de ~15 linhas de código
novo (a detecção de `--`/`.`), então o literal antigo (for-loop + `;;` colados) deixou de existir no
arquivo — não é um bug no código novo, é um cenário de teste que assumia uma adjacência que deixou de
ser verdade.

## Causa raiz

`corrupt_literal` faz substituição de string exata (`source.replace(old, new, 1)` com
`count(old) == 1` obrigatório) — não é ciente de estrutura de case/AST. Qualquer cenário P4 cujo
`old` literal atravesse a fronteira final de um bloco `case ...) ... ;;` é frágil a qualquer inserção
de código **dentro** desse mesmo bloco, mesmo que a inserção não mude o comportamento que o cenário
testa.

## Fix aplicado

Restringir o literal-alvo do Cenário 62b só ao for-loop de `-b`/`-B`/`--orphan` (sem o `;;` de
fechamento), preservando a intenção original do cenário (reverter só o token-scan de `-b`, não
tocar em nada mais). O `;;` real (agora bem depois) nunca precisou fazer parte do alvo — foi incluído
originalmente só porque estava colado, não porque era necessário à prova.

## Regra prática para o próximo ML que editar `match_subcommand`

Antes de inserir código novo **dentro** de um bloco `case` já existente (entre o corpo e o `;;` de
fechamento), rode `check-gates-falsify.sh` cedo — qualquer cenário cujo alvo de `corrupt_literal`
inclua o `;;` desse bloco especificamente vai quebrar com "got 0", não silenciosamente. Prefira, ao
escrever um `corrupt_literal` novo, alvos que terminem no `;;`/fechamento **só quando o teste
realmente depende do fechamento** (raro) — normalmente o alvo deve parar no fim do corpo lógico que
importa (ex.: o for-loop, não o `;;` que o segue).

## Achado lateral (não deste ML, mas custou tempo)

O alias `grep` deste ambiente (`ARGV0=ugrep ... --ignore-files --hidden`) retornou **vazio
silenciosamente** para `npm/src/validator/index.js` (arquivo de ~3000 linhas) ao procurar
`GIT_BRANCH_GUARD_SCRIPT_REFERENCE`, sem erro — só apareceu usando `command grep`. Não investigado a
fundo (possivelmente relacionado a `--ignore-files` interagindo com algum `.ignore`/`.gitignore`
efetivo, ou a um limite de tamanho do ugrep), mas o sintoma prático é: se um `grep` "não encontrar
nada" num arquivo que você tem certeza que contém o padrão, tente `command grep` antes de concluir
que o padrão não existe.

## Ver também

`vault/notes/index.md`

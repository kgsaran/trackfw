# Forge anchoring: the branch NAME must be unconditional, only the SHA cross-checks

**Data:** 2026-08-19
**Contexto:** ML-4B, ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md,
ADR Emenda 1 (`release tag`'s commit-target resolution).

## O erro (pego em revisão, antes do handoff)

A primeira implementação da Emenda 1 fez duas comparações forge-vs-local: uma para o NOME do
branch padrão (`repoInfo.DefaultBranch != base` → recusa) e uma para o SHA
(`localSHA != commitObj.SHA` → recusa). Isso parecia simétrico com o princípio "o forge decide, o
local só verifica cruzado, nunca refusa por conta própria uma divergência silenciosamente" — mas
**a comparação de nome estava invertendo o próprio princípio**.

`base` (o nome) vem de `git symbolic-ref refs/remotes/origin/HEAD`, que em um clone raso/novo
**pode não existir**. Nesse caso `defaultBaseBranch` cai no fallback `"main"`. Se o branch padrão
REAL do repositório for `"master"`, a antiga comparação `repoInfo.DefaultBranch != base` disparava
recusa (`"master" != "main"`) contra um clone **legítimo, sem nenhum atacante envolvido** — puro
falso positivo introduzido pela própria correção de segurança.

## Por que isso não era óbvio

A confusão nasce de tratar "symref ausente" (não há opinião local) e "symref repontado" (há uma
opinião local, e ela é forjada) como o mesmo sinal — `defaultBaseBranch` colapsa os dois casos no
mesmo valor de retorno (`"main"`), então o código que consome esse valor não consegue distinguir
"não sei" de "sei e é isto". Uma comparação de igualdade contra esse valor trata ambos como
divergência potencial, quando só o segundo caso é.

## A correção

O nome do branch do forge (`repoInfo.DefaultBranch`) passou a ser autoritativo
**incondicionalmente** — nenhuma comparação de nome existe mais. Só o SHA é verificado, e essa
verificação usa uma resolução local FRESCA, chaveada ao NOME DO FORGE
(`git rev-parse origin/<repoInfo.DefaultBranch>`), nunca à `base` local. Consequência prática: um
`origin/HEAD` symref repontado (o vetor de ataque original da Emenda 1) é **neutralizado**, não
recusado — a publicação segue contra o branch/sha reais do forge, ignorando o repoint. Isso é mais
forte que uma recusa (o atacante não ganha nem um DoS) e elimina o falso positivo do clone raso.

## Regra geral para revisar `forge decide, local verifica`

Antes de aceitar um refusal baseado em "local X != forge X", perguntar: **o local pode
legitimamente estar ausente/no-fallback, sem que isso signifique um ataque?** Se sim, essa
dimensão não deve gerar recusa por divergência — só a dimensão que tem um valor local
genuinamente resolvido (não um fallback) deve ser cross-checked, e mesmo assim de forma
best-effort/non-fatal quando a resolução falha.

## Ver também

- `vault/notes/git-fetch-self-heals-forged-origin-head-and-tracking-refs-2026-08-19.md` — mecanismo
  irmão descoberto na mesma ML (fetch interno autocura refs forjadas sob refspec padrão).
- `internal/commands/release.go` — `runReleaseTag`, bloco "The commit-target comes from the forge".

---
name: followremotehead-never-reproduces-pre248
description: followRemoteHEAD=never reproduz comportamento pré-git-2.48 (sem symref após fetch) em qualquer versão do git
metadata:
  type: project
---

`git config remote.origin.followRemoteHEAD never` impede que `git fetch` crie
`refs/remotes/origin/HEAD`, mesmo em git ≥ 2.48 (que normalmente cria automaticamente).

**Why:** git 2.48 introduziu criação automática do symref no fetch. A config `followRemoteHEAD`
controla esse comportamento. Valor `never` desabilita em todas as versões — permite simular
o ambiente pré-2.48 sem precisar de uma versão antiga do git.

Confirmado em git 2.54: após fetch com `remote.origin.followRemoteHEAD=never`, `git symbolic-ref
refs/remotes/origin/HEAD` falha com exit 128; `git ls-remote --symref origin HEAD` retorna a
resposta correta (`ref: refs/heads/master`).

**How to apply:** fixtures de teste que precisam replicar o bug do issue #633 (branch padrão
errada em git < 2.48) devem:
1. `git config remote.origin.followRemoteHEAD never` no fixture antes de qualquer fetch
2. `git remote set-head origin --delete` para remover o symref se já existir
3. `git fetch origin` confirma que o symref NÃO é recriado

Isso garante que o teste reprova com o código atual e passa após o fix, em qualquer versão de git.

---
name: guard-blocks-git-branch-in-scratchpad
description: O guard do trackfw bloqueia `git branch`, `git checkout -b` e compostos com esses comandos em qualquer diretório, incluindo scratchpad
metadata:
  type: feedback
---

O guard intercepta `git branch <name>`, `git checkout -b <name>` e comandos compostos que os
contenham — mesmo em diretórios de fixture descartáveis no scratchpad. O guard não verifica se
o diretório está sob governança.

**Why:** o guard de branch bloqueia criação de branches sem passar por `trackfw branch new`.

**How to apply:** para criar branches em fixtures de teste, usar exclusivamente:
- `zz_*_test.go` em Go com `t.TempDir()` — isolado do guard completamente
- `git init --initial-branch <name>` para configurar o nome da branch no init

`git switch --create` não foi testado como evasão — pode estar igualmente bloqueado.
O único caminho CONFIRMADO e sancionado para fixtures de segurança é o Go test em `t.TempDir()`.

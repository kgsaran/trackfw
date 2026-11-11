---
name: ml-scope-le-roadmap-commitado
description: o check ml_scope do barrier lê o roadmap do HEAD, não o do disco; e só aceita caminhos entre crases em **Files affected:**
metadata:
  type: project
---

O `ml_scope` do `trackfw barrier` usa o roadmap **commitado** (HEAD), não o arquivo em edição. Corrigir o
escopo no disco e rodar de novo dá o mesmo bloqueio até commitar.

Ele só reconhece caminhos entre crases na lista de `**Files affected:**`. Uma linha como "os `_test.go`
correspondentes" não declara nada, e cada teste tocado aparece como "out of scope".

**Why:** medido em 2026-11-10, na Wave 3 da REQ de idioma: 11 testes ficaram fora de escopo; acrescentei os
caminhos, rodei de novo e continuou bloqueado até o commit.

**How to apply:** ao escrever um ML, liste cada `_test.go` pelo caminho. Antes de rodar o barrier depois de
editar o roadmap, commite o roadmap.

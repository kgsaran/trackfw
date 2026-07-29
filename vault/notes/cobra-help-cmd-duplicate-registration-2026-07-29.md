---
title: "cobra-help-cmd-duplicate-registration"
tags: [go, cobra, cli, help, parity, bug]
date: 2026-07-29
related: []
---

# cobra-help-cmd-duplicate-registration

## Problem

`trackfw --help` (Go) listava **duas** entradas `help` em "Available Commands":
uma com a descrição customizada ("Exibe documentação das chaves de configuração...")
e outra com a descrição default do cobra ("Help about any command"). Node.js
(commander) e Python (argparse) não tinham esse problema — só o runtime Go
duplicava.

O script `scripts/check-cli-parity.sh` já tinha uma defesa silenciosa contra isso
(`awk '!seen[$0]++'` ao parsear `Available Commands:`, com o comentário "cobra may
list help twice") — ou seja, o sintoma era conhecido e contornado no gate, mas a
causa raiz nunca foi corrigida na CLI em si.

## Root cause

`cobra.Command.InitDefaultHelpCmd()` é chamado automaticamente por `Execute()` e
adiciona um comando `help` interno **sempre que `c.helpCommand == nil`** — esse
campo é interno (`helpCommand *cobra.Command`) e é **independente** de qualquer
comando chamado "help" que você tenha adicionado via `root.AddCommand(meuHelpCmd)`.
Ou seja: registrar seu próprio `cobra.Command{Use: "help"}` via `AddCommand` NÃO
impede o cobra de registrar o dele — os dois convivem como duas entradas
distintas em `root.Commands()`, mesmo tendo o mesmo `Name()`.

A única forma de evitar isso é chamar explicitamente:

```go
root.SetHelpCommand(meuHelpCmd)
```

Isso atribui `meuHelpCmd` ao campo interno `c.helpCommand`, e `InitDefaultHelpCmd`
passa a fazer `c.RemoveCommand(c.helpCommand); c.AddCommand(c.helpCommand)` — ou
seja, reusa a mesma instância em vez de criar uma segunda.

Node.js (commander) e Python (argparse) não sofrem disso: commander verifica se
já existe um comando chamado "help" antes de registrar o seu default; argparse
nunca registra um subcomando "help" automático (só a flag `-h`/`--help`).

## Solution

Em `internal/commands/root.go`:

```go
helpCmd := newHelpCmd()
root.AddCommand(helpCmd, /* ...demais comandos... */)
root.SetHelpCommand(helpCmd) // evita a segunda entrada "help" do cobra
```

## Lição de processo

Se um gate de paridade (`check-cli-parity.sh`, `check-*-parity.sh`) contém uma
linha de-duplicando ou "normalizando" a saída de um runtime com um comentário
explicando *por que* isso é necessário (ex: "cobra pode listar help duas vezes"),
trate esse comentário como um ticket de dívida técnica implícito — é o gate
absorvendo um defeito real do runtime em vez de expor. Vale a pena corrigir na
raiz (aqui: `SetHelpCommand`) mesmo que o dedup no shell continue funcionando
como defesa em profundidade.

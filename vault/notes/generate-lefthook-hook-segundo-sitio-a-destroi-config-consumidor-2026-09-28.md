# `generateLefthookHook` é segundo sítio (a) — destroi configuração do consumidor

> 2026-09-28 · Hades · ML-0A Wave 0 · REQ-2026-09-28-trackfw-init-reexecutado

## Causa

`generateLefthookHook` (scaffold.go linha 2806) faz `os.WriteFile("lefthook.yml", ...)` incondicional
— não lê o arquivo existente antes de sobrescrever. Na reexecução de `init`, destrói qualquer hook
que o consumidor tenha adicionado (lint, pre-push, stage customizado).

## Ordem de chamada que confunde

O orquestrador de `init` chama (scaffold.go):
- Linha 165: `generateGitHooks(cfg)` → `generateLefthookHook()` → sobrescreve `lefthook.yml` inteiro
- Linha 169: `generateCommitMsgHook(cfg)` → lê `lefthook.yml`, acrescenta `commit-msg:` se ausente

Na primeira execução: correto. Na reexecução: passo 1 destroi o conteúdo do passo 2 anterior
E o conteúdo do consumidor. Passo 2 re-adiciona `commit-msg:`, mas o resto sumiu.

## Por que é (a), não (c)

A régua é a natureza do conteúdo destruído. `lefthook.yml` é arquivo de configuração de hook-manager
do projeto — o consumidor adiciona seus próprios estágios nele. Essa autoria é destruída.
Diferente da linha 2498 (`generateCommitMsgHook`, ramo lefthook) que lê-verifica-acrescenta e é (c).

## Contraste com o sítio principal

Linha 864 (`writeTrackfwConfig`) destrói `trackfw.yaml`, corrompendo o resultado de `validate`.
Linha 2806 destrói `lefthook.yml`, não afeta `validate`. Severidade menor, mas é (a) pela mesma régua.

## O que precisa mudar

`generateLefthookHook` deve ler o `lefthook.yml` existente, verificar se `pre-commit:` já declara
`trackfw-validate`, e acrescentar apenas se ausente — mesmo padrão de `generateCommitMsgHook`.

## Implicação para o roadmap

A AC "Todo sítio (a) corrigido por ponto único" requer ML adicional para linha 2806.
O roadmap foi entregue com (a) = 1; após a Wave 0 o correto é (a) = 2.

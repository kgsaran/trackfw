---
name: project_generate_lefthook_hook_segundo_sitio_a
description: generateLefthookHook (scaffold.go:2806) é segundo sítio (a) — overwrite incondicional de lefthook.yml na reexecução de init
metadata:
  type: project
---

`generateLefthookHook` (scaffold.go linha 2806) faz `os.WriteFile("lefthook.yml", ...)` incondicional.
Na reexecução de `init`, destroi hooks customizados do consumidor (lint, pre-push, etc.).

**Por quê (a) e não (c):** a régua da ADR é a natureza do conteúdo destruído. `lefthook.yml`
pode conter autoria do consumidor; o overwrite incondicional destrói isso. Diferente da linha 2498
(append com leitura prévia = (c) completo).

**Ordem de chamada confirmada (scaffold.go):**
- L165: `generateGitHooks(cfg)` → `generateLefthookHook()` → overwrite
- L169: `generateCommitMsgHook(cfg)` → lê, append se ausente

O produto desfaz no passo 1 o que reconstrói no passo 2 — auto-contradição medida.

**Why:** REQ-2026-09-28 foi escrita com (a) = 1. A Wave 0 revelou (a) = 2.
AC "ponto único" requer ML adicional no roadmap antes de Wave 1.

**How to apply:** ao auditar ML-1A ou ML-2A desta REQ, verificar se `generateLefthookHook`
foi corrigido. Se não foi, a AC de "Todo sítio (a) corrigido" não está satisfeita.

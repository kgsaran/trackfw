---
name: codex-home-env-isolation
description: CODEX_HOME deve ser limpo em TestMain e scripts da mesma forma que CLAUDE_CONFIG_DIR, senão testes sobrescrevem hooks.json real do dev
metadata:
  type: feedback
---

Ao adicionar suporte a CODEX_HOME (ML-4A4), não esquecer de limpar a variável em:
1. TestMain dos pacotes que chamam UpdateHarness com alvos codex (validators, generators, commands)
2. Scripts que rodam `update harness` com `CLAUDE_CONFIG_DIR=` — adicionar `CODEX_HOME=` ao lado

**Why:** um desenvolvedor com CODEX_HOME definido teria seu hooks.json real sobrescrito pelos testes.

**How to apply:** sempre que `CLAUDE_CONFIG_DIR=` aparecer num script/TestMain que roda harness, adicionar `CODEX_HOME=` ao lado. Usar python3 (não sed global) para múltiplas ocorrências em scripts longos como check-gates-falsify.sh.

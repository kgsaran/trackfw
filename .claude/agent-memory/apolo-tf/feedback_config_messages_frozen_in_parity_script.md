---
name: config-messages-frozen-in-parity-script
description: config.go mensagens traduzidas têm cópias congeladas em check-agent-models-parity.sh que falham silenciosamente no gate de parity
metadata:
  type: feedback
---

Ao traduzir `MalformedGlobalConfigMessage`, `GlobalAgentModelsNoneMessage` e `GlobalAgentModelsProjectOnlyMessage` em `internal/config/config.go`, o `scripts/check-agent-models-parity.sh` tem cópias congeladas dessas strings em variáveis shell:

- `GLOBAL_WRONG_PLACE_MSG` (~linha 677) ← cópia de `GlobalAgentModelsProjectOnlyMessage`
- `GLOBAL_NOT_CONFIGURED_MSG` (~linha 720) ← cópia de `GlobalAgentModelsNoneMessage`
- `GLOBAL_MALFORMED_MSG` (~linha 796) ← cópia de `MalformedGlobalConfigMessage`

**Por que importa:** o `check-gates-falsify.sh` chama esse script na sua fase `no-repo-mutation` (emite WARN, não FAIL) mas também em cenários 86 e 169 — nesses, a falha do script é fatal para o falsify (FAIL + exit 1). Traduzir as constantes sem atualizar as variáveis shell faz os cenários 86/169 falharem.

**Why:** Medido em ML-3B2 (2026-11-10): traduzi as constantes em config.go, rodei o falsify, e os cenários `setup-s86-baseline` e `setup-s169-baseline` falharam com "check-agent-models-parity.sh já reprova com binário real".

**How to apply:** Sempre que traduzir ou alterar essas 3 constantes em `config.go`, atualizar imediatamente as 3 variáveis correspondentes em `scripts/check-agent-models-parity.sh`.

Links relacionados: [[config-load-singleton-reset]]

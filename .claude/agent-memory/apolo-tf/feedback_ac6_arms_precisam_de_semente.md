---
name: feedback_ac6_arms_precisam_de_semente
description: AC6 arms que verificam remoção devem semear dirs com formas legadas — dir vazio nunca exercita removeGuardFromFlatArrayInHooks
metadata:
  type: feedback
---

Arms de teste D14 que verificam REMOÇÃO de guards devem SEMPRE semear o diretório com formas legadas antes de chamar InjectCursorHooks.

**Why:** Um dir vazio nunca aciona o caminho de remoção. Origin/main passava ArmA porque o arm verificava apenas que a forma canônica foi adicionada (não que a legada foi removida). O advisor identificou que "removeGuardFromFlatArrayInHooks is never exercised on seeded data."

**How to apply:**
- Arm "no global": semear beforeShellExecution, afterShellExecution e pares preToolUse/postToolUse com formas legadas; verificar que AMBAS a canonical está presente E a legacy foi removida
- Arm "global installed": semear com legacy+user hook; verificar que legacy foi removida, user hook sobreviveu, canonical foi adicionada nos pares Read/Write
- ArmA especificamente: além de verificar legacy removida de shell events, verificar também que legacy foi removida dos pares matcher (não só dos shell events)

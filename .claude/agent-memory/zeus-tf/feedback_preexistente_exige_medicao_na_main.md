---
name: preexistente-exige-medicao-na-main
description: "falha pré-existente" de subagente só vale com o mesmo gate rodado em origin/main; no #REQ-2026-09-05 foi falso 4 vezes
metadata:
  type: feedback
---

Rótulo "pré-existente" de subagente não é aceito sem o mesmo gate rodado em `origin/main` (worktree temporário) com a saída colada.

**Why:** em 2026-10-06, na REQ-2026-09-05 (guard em Go), quatro falhas foram chamadas de pré-existentes e todas vinham da branch: `pathguard` (escrita do ML-1B), `parity-falsify` (cenários que corrompiam o template `.sh` virado invólucro), e dois FAILs de fixture que procuravam o `.sh` nas configs. Uma delas escondia uma asserção que tinha ficado vacua.

**How to apply:** no handoff de qualquer ML que termine em gate, escrever a proibição explícita e o comando do worktree em `origin/main`. Na auditoria, conferir o histórico (`git log -S`) do teste que falha antes de aceitar. Ver [[medir-com-a-regra-nao-com-grep]] e [[proibir-make-quality-cega-o-executor]].

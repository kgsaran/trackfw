---
name: mkdir-before-lnfs-creates-directory-not-symlink
description: mkdir -p antes de ln -sf cria diretório; ln-sf coloca o link DENTRO em vez de substituir — CODEX_HOME torna-se dir real, não symlink
metadata:
  type: feedback
---

`mkdir -p $H/x; ln -sf $REAL $H/x` → `$H/x` já existe como diretório → `ln -sf` cria symlink DENTRO de `$H/x` apontando para `$REAL`. O diretório `$H/x` em si não é symlink.

Medido em 2026-11-17 ML-FA: test de CODEX_HOME como symlink (vetor 1C-b) retornou rc=0 (binary operou normalmente) por causa deste erro de setup. Run correto sem `mkdir -p`: rc=1 (RESISTENTE, como esperado).

**Why:** é um erro sutil de shell que inverte o comportamento esperado sem mensagem de erro.
**How to apply:** ao criar symlink para test: NÃO usar `mkdir -p` no diretório alvo antes de `ln -sf`. Verificar com `ls -la $H/x` que o próprio `x` é o symlink, não um dir.

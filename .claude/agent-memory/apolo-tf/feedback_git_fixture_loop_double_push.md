---
name: git-fixture-loop-double-push
description: Dois clones num loop que tentam commitar+push os mesmos arquivos ao mesmo origin falham no segundo ("nothing to commit")
metadata:
  type: feedback
---

Ao criar dois clones independentes de um mesmo origin em `check-barrier.sh`, nunca use um `for` loop que tenta `git add . && git commit && git push` em cada clone — o segundo clone já baixa os arquivos que o primeiro commitou, então `git commit` sai com rc=1 ("nothing to commit, working tree clean"), quebrando o `set -euo pipefail`.

**Why:** o origin já tem o conteúdo após o push do primeiro clone; o segundo clone recebe tudo via `git clone`.

**How to apply:** criar um clone "semente" separado que faz o commit+push inicial, depois clonar o origin para os arms de teste individualmente (sem tentar re-commitar).

```bash
# Correto:
env "${ge[@]}" git clone -q "$S_ORIGIN" "$S_SEED"
# ... escreve arquivos em $S_SEED, commit, push ...
env "${ge[@]}" git clone -q "$S_ORIGIN" "$S_ARM_NEG"   # já tem os arquivos
env "${ge[@]}" git clone -q "$S_ORIGIN" "$S_ARM_POS"   # já tem os arquivos
```

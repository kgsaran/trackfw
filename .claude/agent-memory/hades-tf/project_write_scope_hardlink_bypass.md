---
name: write-scope-hardlink-bypass
description: EvalSymlinks não resolve hardlink — barreira de caminho por comparação de string precisa de os.SameFile; caso do guard write-scope corrigido no #606
metadata:
  type: project
---

Uma barreira que compara caminhos como string, depois de `filepath.EvalSymlinks`, deixa passar hardlink. As duas entradas de diretório apontam para o mesmo inode, mas cada uma resolve para o próprio caminho.

**Why:** medido em 2026-10-23 (ML-4B da REQ-2026-10-23). `ln docs/agents-working-context.md docs/alias.md` seguido de `Write(docs/alias.md)` passava pelo `trackfw guard write-scope` com exit 0. Corrigido no ML-4C (#606): o `pathMatchesFrozen` agora chama `os.SameFile` quando o alvo e o arquivo congelado existem.

**How to apply:** em toda barreira de caminho, use `pathEqual(a, b) || os.SameFile(statA, statB)` quando os dois existem. Arquivo que ainda não existe não pode ser hardlink. Resíduo: um hardlink para um arquivo dentro de um diretório congelado só pode ser criado por Bash, e escrita por Bash é resíduo declarado.

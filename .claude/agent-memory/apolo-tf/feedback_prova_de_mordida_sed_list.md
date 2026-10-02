---
name: prova-de-mordida-sed-list
description: Quais casos da tabela C01-C22 realmente falham abertos com o sed antigo (3b2eff09) no modo sem jq — medidos, não inferidos
metadata:
  type: feedback
---

O handoff listou C02-C05, C09-C12, C22 como casos que falham abertos com o script antigo (3b2eff09) no modo sem jq. A medição real divergiu:

- **Falham abertos (rc=0) com o sed antigo**: C02, C03, C04, C05, C12, C22 ✓
- **NÃO falham abertos**: C09 (sed greedy `.*` acha o último "command" = "git push"), C10 (sed processa linha a linha, acha command na linha 3), C11 (sed fallback 1 com `[^}]*` backtracking encontra tool_input.command corretamente)

**Why:** o sed com `.*` greedy em BRE usa leftmost-longest: o `.*"command"` expande até o ÚLTIMO "command" na linha, capturando o valor correto para C09/C11. Para C10 (blank line), sed processa linha a linha sem o problema do `RS=""`.

**How to apply:** ao escrever prova de mordida para o guard: medir o comportamento real do old script antes de escrever os casos. A lista do handoff pode conter casos que o sed antigo já tratava corretamente. Usar `git show <commit>:scripts/...` + `env -i PATH=<fakebin>` + `printf '%s' '<payload>' | bash <old-script>` para medir.

# 2026-10-24 — zeus-tf — FIM: encerra a REQ-2026-10-23 das regras de escrita dos agentes

- #606 mergeado (7bddcd34); CI 20/20 no 72ba29f5.
- Entregue: D1 (assets e bloco de regras), D2 (`ml_scope` no barrier), D3 (`guard write-scope` com congelados), D4 (permissão por despacho com expiração; toda permissão encontrada precisa liberar).
- Pendente, nesta ordem: release → KG instala e roda `trackfw agents update` → PR que cabeia o `guard write-scope` no `.claude/settings.json` deste repo.
- Resíduos declarados: escrita por Bash (só o `ml_scope` pega), CLIs sem identidade de subagente, MCP sem campo de caminho padronizado.

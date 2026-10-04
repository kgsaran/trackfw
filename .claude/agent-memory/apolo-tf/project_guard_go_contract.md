---
name: project_guard_go_contract
description: Contratos não-óbvios do guard Go (ML-1A): map vs struct, idleTimeout parametrizado, os.Exit direto no deny
metadata:
  type: project
---

Guard Go (internal/guard/, internal/commands/guard.go) — decisões medidas no ML-1A:

- **map[string]json.RawMessage, nunca struct**: encoding/json faz case-fold em struct (Command==command), o que seria fail-open. map preserva case-sensitivity exata (gate ii da Wave 0). Usar em TODOS os níveis (root e nested).
- **RunGitBranch recebe `idleTimeout time.Duration`**: a constante de 2s bloquearia os testes. O cobra RunE passa `guard.StdInIdleTimeout`; testes passam `50*time.Millisecond`. StdInIdleTimeout é exportado para o package commands.
- **deny usa `os.Exit(code)` diretamente no RunE, não `return error`**: retornar error faria cobra imprimir "Error: ..." no stderr, que deve ficar limpo para o protocolo JSON.
- **guardError é o sentinel D7**: cobra errors de args/flags inválidos sob guard retornam *guardError; Execute() detecta via isCommandUnderGuard e sai com 2.
- **isCommandUnderGuard usa cmd.CommandPath()**: verifica `strings.HasPrefix(path, "trackfw guard")`.

**Why:** contratos da ADR-2026-10-02 (D1-D3) e ADR-2026-10-04 (D7-D9). Desvio → fail-open silencioso.

**How to apply:** ao escrever testes do guard ou estender subcomandos do guard, sempre verificar estes quatro pontos.

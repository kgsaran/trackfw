# Zeus — início: memória de agente em subpasta (#646)

- **Branch:** `fix/memoria-de-agente-em-subpasta`.
- **REQ:** `REQ-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio`; roadmap em `wip/`.
- **Medido:** o Claude Code cria `<cwd do despacho>/.claude/agent-memory/<agente>/` vazio no ato do despacho e injeta esse caminho no prompt; a sessão principal segue o mesmo cwd. Reproduzido com `artemis-tf` despachado de `scripts/`; diretórios do experimento removidos.
- **Próximo:** Wave 0 (`hades-tf`).
- **Paralelo:** issue #649 aberto (timeout do PowerShell 5.1 no `windows-full-suites`), fora desta frente.

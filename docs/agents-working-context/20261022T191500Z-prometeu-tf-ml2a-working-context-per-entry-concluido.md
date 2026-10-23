## ML-2A concluído — Working Context per Entry (ADR D2, AC8)

**Agente:** prometeu-tf  
**Worktree:** `/private/tmp/trackfw-toml-escape`  
**Branch:** `fix/trackfw-log-conflita-em-toda-branch-paralela`

### O que foi feito

- Todos os 12 assets de agente em `internal/integrations/assets/agents/*.md`: secção `## Working context` reescrita para instruir escrita em `docs/agents-working-context/<YYYYMMDDTHHMMSSZ>-<slug>-<topic>.md` e leitura das 10 entradas mais recentes; proíbe escrita no `.md` legado.
- `internal/generators/agentfiles.go` linhas 92-93 atualizadas para refletir o novo protocolo no bloco gerado (CLAUDE.md / AGENTS.md / GEMINI.md / etc.).
- `internal/generators/context.go`: adicionadas funções `recentWorkingContextEntries`, `extractFirstHeading` e tipo `workingContextLog`; saída de `trackfw context` ganha secção `## Recent working context (last N entries)`.
- 4 goldens atualizados.
- `CLAUDE.md`, `AGENTS.md`, `GEMINI.md` atualizados nas linhas de protocolo antes/depois.
- `docs/agents-working-context.md` recebeu cabeçalho de redirecionamento (arquivo congelado).
- Testes novos: `internal/integrations/working_context_dir_test.go` (12 agentes × todas superfícies + braço negativo) e `internal/generators/working_context_log_test.go` (10 mais recentes, dir ausente, symlink ignorado, bloco gerado).

### Validação (protocolo de microlote)

| Gate | Resultado |
|------|-----------|
| `go build ./...` | 0 |
| `go vet ./...` | 0 |
| `go test ./internal/integrations/ ./internal/generators/ ./internal/commands/ ./internal/thirdparty/` | ok (4 pacotes) |
| `trackfw validate` | 0 (lenient) |
| `make parity-rest` | falha em `check-wheel-filename.sh` (packaging ausente) — pré-existente em `main`, fora do escopo desta ML |
| Bite 1 (worktree real) | PASS |
| Bite 2 (backend.md revertido) | FAIL (esperado) |
| Bite 3 (backend.md restaurado) | PASS |

### Roadmap

ML-2A marcado ✅ Concluído em `ROADMAP-2026-09-22-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md`.

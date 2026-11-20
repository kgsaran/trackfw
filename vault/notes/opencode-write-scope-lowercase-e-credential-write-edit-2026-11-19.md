# OpenCode: write-scope bypass por lowercase e lacuna credential guard em Write/Edit (2026-11-19)

**Contexto:** ML-5-0 (Wave 5, REQ-2026-11-17). Medições com branch binary v11.7.0 (commit `1f8e82c0`).

## Achado 1 — write-scope: lowercase tool names bypassam Step 4

`writeScopeWriteTools` (`writescope.go:45–48`) contém apenas PascalCase: `"Write"`, `"Edit"`, `"NotebookEdit"`. Step 4 faz comparação case-sensitive exata. OpenCode envia tool names lowercase (`"write"`, `"edit"`, `"shell"`).

**Medido:**
```
tool_name="write" + agent_id + .agents/hooks.json → rc=0 (allow, Step 4)
tool_name="Write" + agent_id + .agents/hooks.json → rc=2 (blocked, Step 10 frozen)
```

O plugin gerado deve mapear `write → Write`, `edit → Edit` antes de chamar o write-scope guard.

## Achado 2 — write-scope: campo de caminho `path`/`filePath` ≠ `file_path`

`writescope.go` usa `tool_input.file_path` como único campo de caminho.
- V1 OpenCode envia `filePath` (fonte: `packages/opencode/src/tool/write.ts@v1.19.5`)
- V2 OpenCode envia `path` (fonte: `packages/core/src/tool/plugin/write.ts@v2.3.0`)

**Medido:**
```
tool_name="Write" + agent_id + tool_input.filePath → rc=2 "tool_input.file_path absent" (bloqueia TODA write)
tool_name="Write" + agent_id + tool_input.path    → rc=2 "tool_input.file_path absent" (idem)
```

Comportamento é fail-closed mas bloqueia TODAS as writes (falso positivo), não apenas frozen paths.
O plugin deve mapear: `V1 filePath → file_path`, `V2 path → file_path`.

## Achado 3 — credential guard: três matchers requeridos (Bash|PowerShell, Read, Write|Edit)

Claude Code wires credential guard a três matchers em `agentfiles.go:414–417`:
- `Bash|PowerShell` (claudeShellMatcher)
- `Read`
- `Write|Edit`

Um plugin OpenCode que chame o credential guard apenas para tool `shell`/`bash` perderá credenciais em:
- `content` de Write calls
- `newString` de Edit calls
- Read calls: Layer 2 é no-op para Read (`credSecondLayer` retorna vazio quando `shellCmd == ""`);
  Layer 1 varre o caminho como string no payload (proteção marginal)

**Medido:** `trackfw validate --json` em projeto minimal com `.claude/settings.json` vazio emite `credential_guard_presence_warning` para todos os três matchers separadamente.

## Consequência

**Cinco pré-requisitos independentes** para write-scope funcionar com OpenCode (R7):
1. Adicionar `.opencode/plugins/` etc. a `writeScopeSubagentFrozenRelPaths`
2. Plugin mapeia tool names para PascalCase (`write→Write`, `edit→Edit`)
3. Plugin encaminha `agent_id` somente em chamadas de subagente (a discriminar no ML-5B)
4. O valor do caminho deve ser **absoluto** — write-scope rejeita relativos com rc=2
5. Campo de caminho deve ser `file_path` — não `filePath`/`path` (rc=2 "file_path absent")

**Notas de arm anteriores:** o binário 11.5.0 (antes do ML-FA2) não continha
`.agents/hooks.json` em `writeScopeSubagentFrozenRelPaths`, portanto probes com esse path
e 11.5.0 retornavam rc=0. As probes acima foram executadas com build da árvore em `1f8e82c0`
(v11.7.0) que inclui a entrada ML-FA2.

Mais: o plugin deve chamar credential guard para `shell`/`bash`, `read`, `write`, `edit`, e `patch` (V2).

Nenhuma dessas mudanças afeta código fora do plugin gerado (exceto a adição de caminhos à lista de frozen paths, que requer mudança no produto).

## Referências
- `internal/guard/writescope.go:45–48` — `writeScopeWriteTools`
- `internal/generators/agentfiles.go:414–417` — credential guard matchers
- `docs/seguranca/2026-11-19-wave5-opencode-threat-model.md` §1.12, T3d–T3g, T13

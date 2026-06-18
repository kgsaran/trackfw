---
name: agent-rules-inject-2026-06-18
title: "feat: inject trackfw rules em arquivos de agentes (init + discover)"
status: done
req: ~
created: 2026-06-18
author: zeus
---

# Roadmap: Injeção de Regras trackfw em Arquivos de Agentes

> Criado em: 2026-06-18 | Status: 🔄 WIP

## Diagnóstico / Contexto

**Problema 1 — `claudemd.go` sempre sobrescreve:**
`os.WriteFile("CLAUDE.md", ...)` sem checar existência → customizações do usuário são
perdidas ao rodar `trackfw init` novamente.

**Problema 2 — `discover --init` ignora arquivos de agentes:**
Só gera `trackfw.yaml` + gates. Não toca em CLAUDE.md, AGENTS.md, GEMINI.md,
`.github/copilot-instructions.md`, `.windsurfrules`, `.amazonq`, `.cursor/rules/`.

**Problema 3 — `init` só injeta regras no CLAUDE.md:**
Os equivalentes das outras ferramentas ficam sem as regras de governança do trackfw.

### Solução: marcadores de idempotência + função central de inject

Delimitadores HTML (funciona em todos os formatos Markdown):
```
<!-- trackfw:rules:start -->
...conteúdo das regras...
<!-- trackfw:rules:end -->
```

Lógica para qualquer arquivo de agente:
- **Arquivo não existe** → criar com conteúdo mínimo + bloco de regras
- **Arquivo existe, sem marcador** → append do bloco no fim
- **Arquivo existe, com marcador** → substituir conteúdo entre marcadores (update idempotente)

### Arquivos gerenciados

| Arquivo | Ferramenta | Comportamento |
|---------|-----------|---------------|
| `CLAUDE.md` | Claude Code | inject-or-create (full content se novo) |
| `AGENTS.md` | OpenAI Codex | inject-or-create (header mínimo se novo) |
| `GEMINI.md` | Gemini CLI | inject-or-create (header mínimo se novo) |
| `.github/copilot-instructions.md` | GitHub Copilot | inject-or-create |
| `.windsurfrules` | Windsurf | inject-or-create |
| `.amazonq/developer/guidelines.md` | Amazon Q | inject-or-create |
| `.cursor/rules/trackfw.mdc` | Cursor | sempre criar/atualizar (arquivo trackfw-owned) |

### Comportamento por comando

**`trackfw init`** (usuário escolheu as ferramentas):
- `generateClaudeMD` → inject-or-create (não mais overwrite)
- Para cada ferramenta selecionada pelo usuário → `InjectRulesForTool(tool, cwd)`

**`trackfw discover --init`** (projeto existente, ferramentas desconhecidas):
- Após instalar gates → `InjectRulesDetected(cwd)` — varre e injeta apenas nos arquivos JÁ EXISTENTES
- Exceção: `.cursor/rules/trackfw.mdc` é sempre criado/atualizado se `.cursor/` existe

### Conteúdo do bloco injetado (tool-agnostic)

```markdown
<!-- trackfw:rules:start -->
## trackfw — Governance Rules

This project uses **trackfw** for AI-native delivery governance.
Chain: `ADR → REQ → ROADMAP` · States: `backlog / wip / blocked / done / abandoned`

### Agent Protocol
1. **Before starting:** run `trackfw context` · read `docs/agents-working-context.md`
2. **After finishing:** update `docs/agents-working-context.md` with what changed
3. **Before PR:** `trackfw validate` must pass

### Key Commands
- `trackfw context` — current governance state (always run first)
- `trackfw status` — all artifacts and states
- `trackfw validate` — governance consistency check
- `trackfw roadmap move <name> <state>` — transition roadmap state
- `trackfw serve` — live Kanban board at http://localhost:4080

### Attention Signal (when you need user input during a task)
Write `docs/roadmaps/.trackfw-attention.json`:
{"roadmap":"file.md","ml":"ML-1A","message":"what you need","level":"action_required","timestamp":"ISO8601Z"}
Delete the file when resolved. Visible as a live banner in `trackfw serve`.
<!-- trackfw:rules:end -->
```

### Paridade: 3 CLIs (Go + Node.js + Python)

Esta feature afeta lógica core do CLI → paridade obrigatória.

---

## Wave 1 — Core Go: função central + fix claudemd (2 MLs em paralelo)

> Dependências: nenhuma

### ML-1A — `internal/generators/agentfiles.go` (novo)

**Status:** ✅ Concluído

**Arquivo:** `internal/generators/agentfiles.go` (novo)

**Ações:**
1. Constantes de marcadores:
   ```go
   const rulesStart = "<!-- trackfw:rules:start -->"
   const rulesEnd   = "<!-- trackfw:rules:end -->"
   ```

2. Função `trackfwRulesBlock() string` — retorna o bloco de regras (conteúdo do template acima).

3. Função `injectOrUpdateRules(filePath, headerIfNew string) error`:
   - Se arquivo não existe: cria com `headerIfNew + "\n\n" + trackfwRulesBlock()`
   - Se existe e tem `rulesStart`: substitui tudo entre marcadores pelo novo `trackfwRulesBlock()`
   - Se existe e NÃO tem marcador: appenda `"\n\n" + trackfwRulesBlock()` no fim
   - Cria diretórios pai com `os.MkdirAll` se necessário

4. Mapa de arquivos por ferramenta:
   ```go
   var agentFiles = map[string]string{
       "claude":  "CLAUDE.md",
       "codex":   "AGENTS.md",
       "gemini":  "GEMINI.md",
       "copilot": ".github/copilot-instructions.md",
       "windsurf":".windsurfrules",
       "amazonq": ".amazonq/developer/guidelines.md",
       "cursor":  ".cursor/rules/trackfw.mdc",
   }
   ```

5. Header mínimo por ferramenta (usado quando arquivo não existe):
   - `claude`/`codex`/`gemini`: `"# Project Instructions\n"`
   - `copilot`: `"# GitHub Copilot Instructions\n"`
   - `windsurf`: `"# Windsurf Rules\n"`
   - `amazonq`: `"# Amazon Q Developer Guidelines\n"`
   - `cursor`: `"---\ndescription: trackfw governance rules\nglob: '**/*'\nalwaysApply: true\n---\n"`

6. `InjectRulesForTool(tool, cwd string) error`:
   - Lookup em `agentFiles[tool]`; skip se tool não reconhecida
   - `injectOrUpdateRules(filepath.Join(cwd, path), header)`

7. `InjectRulesDetected(cwd string) error`:
   - Para cada `tool, path` em `agentFiles`: se `os.Stat(filepath.Join(cwd, path))` OK → `injectOrUpdateRules`
   - Cursor: se `.cursor/` existe → sempre inject (mesmo que `trackfw.mdc` não exista)
   - Retorna erros como warnings (non-fatal), continua para os demais

**Critérios de aceite:**
- [ ] `make build` sem erros
- [ ] Arquivo novo: criado com header + bloco
- [ ] Arquivo existente sem marcador: bloco appendado
- [ ] Arquivo existente com marcador: conteúdo entre marcadores atualizado, resto preservado
- [ ] Segunda execução é idempotente (não duplica)

---

### ML-1B — Corrigir `claudemd.go` — inject-or-create

**Status:** ✅ Concluído

**Arquivo:** `internal/generators/claudemd.go`

**Ações:**
1. Mover o conteúdo gerado atualmente (a string completa do CLAUDE.md) para ser o `fullContent`
2. Substituir o `os.WriteFile` no final por chamada a `injectOrUpdateRules("CLAUDE.md", fullContent)`:
   - Se CLAUDE.md não existe → comportamento atual (cria com tudo)
   - Se existe → injeta/atualiza apenas o bloco `<!-- trackfw:rules:start/end -->`

   **Atenção:** a função `injectOrUpdateRules` foi projetada para tratar o conteúdo completo
   como `headerIfNew`. Aqui o `headerIfNew` É o conteúdo completo do CLAUDE.md gerado.
   O bloco de regras é appendado ao final quando criando do zero, ou atualizado no lugar
   quando o arquivo já tem o marcador.

   **Portanto**: ao gerar o CLAUDE.md novo (arquivo não existia), gerar sem o bloco de
   regras embutido e deixar `injectOrUpdateRules` adicioná-lo. Isso mantém a seção sempre
   no formato padrão independente do gerador.

3. Remover a linha de regras de atenção de agente que foi adicionada hardcoded no CLAUDE.md
   gerado (se existir) — a seção agora vem do bloco padronizado.

**Critérios de aceite:**
- [ ] `make build` sem erros
- [ ] CLAUDE.md não existia: criado com conteúdo completo + bloco de regras no final
- [ ] CLAUDE.md existia (customizado): bloco appendado no final sem tocar no resto
- [ ] CLAUDE.md existia (criado por trackfw antes): bloco atualizado entre marcadores

---

## Wave 2 — Wiring nos comandos Go (2 MLs em paralelo)

> Dependências: Wave 1 completa

### ML-2A — Wiring em `discover.go`

**Status:** ✅ Concluído

**Arquivo:** `internal/commands/discover.go`

**Ações:**
Após a linha `fmt.Fprintln(out, "✓ governance gates installed")` (linha ~128),
adicionar:
```go
// Injetar regras trackfw em arquivos de agentes detectados
if err := generators.InjectRulesDetected(cwd); err != nil {
    fmt.Fprintf(out, "⚠ agent rules inject partial: %v\n", err)
} else {
    fmt.Fprintln(out, "✓ trackfw rules injected into agent config files")
}
```

**Critérios de aceite:**
- [ ] `trackfw discover --init` num projeto com CLAUDE.md existente: injeta regras
- [ ] `trackfw discover --init` num projeto sem CLAUDE.md: não cria (só injeta em existentes)
- [ ] Saída mostra `✓ trackfw rules injected into agent config files`

---

### ML-2B — Wiring em `scaffold.go` (init)

**Status:** ✅ Concluído

**Arquivo:** `internal/generators/scaffold.go`

**Ações:**
Após a chamada `generateClaudeMD(cfg)` em `Scaffold()`, adicionar chamada para
injetar nos demais arquivos selecionados. O `init.go` já recebe os `aiTools` selecionados
mas eles não chegam ao `Scaffold`. Há duas opções:

**Opção escolhida (mais simples):** Adicionar campo `AITools []string` em `Config` e
passá-lo de `init.go` para `Scaffold()`. Dentro de `Scaffold()`, após `generateClaudeMD`:

```go
for _, tool := range cfg.AITools {
    if tool == "claude" {
        continue // já tratado por generateClaudeMD
    }
    if err := InjectRulesForTool(tool, "."); err != nil {
        fmt.Printf("  ⚠ %s rules inject: %v\n", tool, err)
    } else {
        fmt.Printf("  ✓ rules injected into %s config\n", tool)
    }
}
```

Em `init.go`, antes de chamar `generators.Scaffold(cfg)`, popular `cfg.AITools = aiTools`.

**Critérios de aceite:**
- [ ] `trackfw init` com gemini selecionado: GEMINI.md recebe regras
- [ ] `trackfw init` com copilot selecionado: `.github/copilot-instructions.md` recebe regras
- [ ] Claude sempre injeta (via `generateClaudeMD`)

---

## Wave 3 — Paridade Node.js + Python (2 MLs em paralelo)

> Dependências: Wave 1 + 2 completas (referência de implementação Go pronta)

### ML-3A — Node.js: `npm/src/generators/init.js`

**Status:** ✅ Concluído

**Arquivo:** `npm/src/generators/init.js`

**Ações:**
1. Adicionar constantes `RULES_START`, `RULES_END`, função `trackfwRulesBlock()`
2. Implementar `injectOrUpdateRules(filePath, headerIfNew)` em Node.js puro (fs.readFileSync/writeFileSync)
3. Implementar `injectRulesForTool(tool, cwd)` e `injectRulesDetected(cwd)`
4. Modificar `generateClaudeMD(cfg, cwd)` para usar `injectOrUpdateRules`
5. Em `scaffold(cfg)`, após generateClaudeMD: chamar `injectRulesForTool` para ferramentas em `cfg.aiTools`

**Critérios de aceite:**
- [ ] `node --check npm/src/generators/init.js` sem erros
- [ ] Comportamento idêntico ao Go

---

### ML-3B — Python: `pypi/trackfw/generators/init_gen.py`

**Status:** ✅ Concluído

**Arquivo:** `pypi/trackfw/generators/init_gen.py`

**Ações:**
1. Adicionar constantes e `_trackfw_rules_block()` em Python
2. Implementar `inject_or_update_rules(file_path, header_if_new)` com pathlib/os
3. Implementar `inject_rules_for_tool(tool, cwd)` e `inject_rules_detected(cwd)`
4. Integrar na função `scaffold(cwd, opts)` após geração de CLAUDE.md

**Critérios de aceite:**
- [ ] `python3 -m py_compile pypi/trackfw/generators/init_gen.py` sem erros
- [ ] Comportamento idêntico ao Go

---
name: trackfw-update-command-2026-06-18
title: "trackfw update — comando de atualização de artefatos gerenciados"
status: wip
date: 2026-06-18
req: REQ-2026-06-18-trackfw-update-command
branch: feat/kanban-roadmap-progress
---

# Roadmap: trackfw update

> Criado em: 2026-06-18 | Status: 🔄 WIP

## Diagnóstico / Contexto

Após `trackfw init` ou `trackfw discover --init`, o projeto tem: regras nos arquivos de agente (via markers), gates (hooks + CI), slash commands Claude (`.claude/commands/trackfw/`) e skill global (`~/.claude/skills/trackfw/`).

Com a atualização do binário, esses artefatos ficam na versão antiga. O `trackfw update` re-aplica os templates atuais embutidos no binário, respeitando propriedade de cada arquivo:

- **Categoria 1 — marker-delimited** (user-owned, trackfw injeta bloco): usa `InjectRulesDetected` já implementado.
- **Categoria 2 — trackfw-owned** (overwrite seguro): validate script, CI workflow, Claude commands, skill.
- **Categoria 3 — shared user files** (cirúrgico): `.husky/pre-commit`, `lefthook.yml`.

**Gotcha de instaladores:** todos os instaladores atuais são skip-if-exists → precisam de variantes force para o `update`.

---

## Wave 1 — Go: generators de suporte (base para o comando) [2 MLs paralelos]

> Independente

### ML-1A — ReadUpdateConfig + Update() em internal/generators/update.go
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/generators/update.go` (NOVO)
**Ações:**
1. Criar `internal/generators/update.go` com:
   - `ReadUpdateConfig(cwd string) Config` — lê `trackfw.yaml` linha a linha (sem deps externas) e popula `Config{Hooks, CI, Backend, Frontend, PkgManager}`
   - `Update(cwd string) error` — orquestrador central: chama InjectRulesDetected, regenera gates e commands com force, cirurgicamente atualiza hooks; imprime sumário de ✓/⚠ para cada passo
   - `updateGitHooksurgical(cfg Config, cwd string)` — para husky: append `trackfw validate` se não presente em `.husky/pre-commit`; para lefthook: append block se `trackfw-validate:` não presente em `lefthook.yml`
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `ReadUpdateConfig` parseia corretamente hooks/ci/backend/frontend/pkg_manager de um `trackfw.yaml` de exemplo
- [ ] `Update` compila e chama todas as sub-funções sem panic
**Dependência:** ML-1B (precisa das funções force)

### ML-1B — Force variants: ForceGenerateClaudeCommands + ForceInstallSkills
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/generators/scaffold.go`
**Ações:**
1. Extrair lógica interna de `generateClaudeCommands()` para `generateClaudeCommandsInner(force bool) error`:
   - Se `force=false`: comportamento atual (skip se arquivo existe)
   - Se `force=true`: sobrescreve sempre
   - `generateClaudeCommands()` passa `force=false` (sem quebrar callers)
2. Adicionar func pública `ForceGenerateClaudeCommands() error` que chama `generateClaudeCommandsInner(true)`
3. Extrair lógica interna de `InstallSkills()` para `installSkillsInner(force bool) error`:
   - Se `force=false`: comportamento atual (skip se SKILL.md existe)
   - Se `force=true`: sobrescreve sempre
   - `InstallSkills()` passa `force=false`
4. Adicionar func pública `ForceInstallSkills() error` que chama `installSkillsInner(true)`
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/generators/...` verde
- [ ] `ForceGenerateClaudeCommands()` sobrescreve arquivo existente com conteúdo atualizado
- [ ] `ForceInstallSkills()` sobrescreve SKILL.md existente

---

## Wave 2 — Go: comando CLI update [depende de Wave 1]

> Dependência: Wave 1 completa

### ML-2A — internal/commands/update.go + registro no root
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `internal/commands/update.go` (NOVO)
- `internal/commands/root.go` (ou arquivo onde commands são registrados)
**Ações:**
1. Criar `internal/commands/update.go`:
   ```go
   package commands

   import (
       "os"
       "github.com/kgsaran/trackfw/internal/generators"
       "github.com/spf13/cobra"
   )

   func newUpdateCmd() *cobra.Command {
       return &cobra.Command{
           Use:   "update",
           Short: "Update trackfw-managed artifacts to the current version",
           RunE: func(cmd *cobra.Command, args []string) error {
               cwd, _ := os.Getwd()
               return generators.Update(cwd)
           },
       }
   }
   ```
2. Localizar onde `newInitCmd()` é registrado no root e adicionar `newUpdateCmd()` ao mesmo lugar
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `bin/trackfw update --help` mostra descrição correta
- [ ] `bin/trackfw update` (em diretório com `trackfw.yaml`) imprime sumário de atualização

---

## Wave 3 — Node.js: update command [paralelo com Wave 3-Py]

> Independente de Wave 1 (implementação própria)

### ML-3A — npm/src/commands/update.js + registro
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `npm/src/commands/update.js` (NOVO)
- `npm/src/index.js` ou `npm/src/cli.js` (registro)
**Ações:**
1. Criar `npm/src/commands/update.js` como módulo Commander:
   - Ler `trackfw.yaml` para extrair hooks/ci/backend/frontend
   - Chamar `generators.injectRulesDetected(cwd)` para regras
   - Chamar `writeValidateScript(rootDir)` (já sobrescreve, sem guarda)
   - Chamar `writeCIWorkflow(rootDir)` com force (remover guarda skip-if-exists)
   - Chamar `generateClaudeCommandsForce(rootDir)` — nova função em `npm/src/generators/init.js`
   - Chamar `installSkillsForce(rootDir)` — nova função em `npm/src/generators/init.js`
   - Atualizar hooks cirurgicamente: ensure `trackfw validate` está presente sem sobrescrever
   - Imprimir sumário ✓/⚠
2. Em `npm/src/generators/init.js`: adicionar `generateClaudeCommandsForce(rootDir)` e `installSkillsForce(rootDir)` que fazem overwrite sem guards
3. Em `npm/src/generators/init.js`: adicionar `writeCIWorkflowForce(rootDir)` que não tem `isFile(dest) return`
4. Registrar o comando no CLI Node.js
5. Adicionar às exports de discover.js: `installHook` e `writeCIWorkflow`

**Referência discover.js:** `writeCIWorkflow` tem `if (isFile(dest)) return; // idempotente` → a versão force remove esse guard
**Referência init.js:** `generateClaudeCommands` tem `if (_, err := os.Stat(path); err == nil) continue` — Node deve espelhar
**Critérios de aceite:**
- [ ] `node npm/src/cli.js update --help` mostra descrição
- [ ] `node npm/src/cli.js update` em projeto com `trackfw.yaml` atualiza artefatos e imprime sumário
- [ ] Chamadas a `injectRulesDetected` e `ForceGenerateClaudeCommands` não jogam exceção em pasta limpa

---

## Wave 3-Py — Python: update command (escopo reduzido) [paralelo com Wave 3]

> Independente de Wave 1

### ML-3B — pypi/trackfw/commands/update.py + registro
**Status:** ⬜ Pendente
**Arquivos afetados:**
- `pypi/trackfw/commands/update.py` (NOVO)
- `pypi/trackfw/cli.py` (registro)
**Ações:**
1. Criar `pypi/trackfw/commands/update.py`:
   - Verificar que `trackfw.yaml` existe (erro se não)
   - Chamar `inject_rules_detected(cwd)` para atualizar regras de agente
   - Imprimir aviso: "Para atualizar gates (hooks/CI) e Claude commands, use o CLI Go ou Node.js"
   - Sumário simples ✓/⚠
2. Registrar subcomando `update` em `pypi/trackfw/cli.py`
**Critérios de aceite:**
- [ ] `python -m trackfw update --help` mostra descrição
- [ ] `python -m trackfw update` em projeto com `trackfw.yaml` atualiza regras de agente e imprime aviso de escopo
- [ ] Sem `trackfw.yaml` → erro claro: "trackfw.yaml não encontrado — execute trackfw init primeiro"

---

## Protocolo de conclusão de cada ML:
```
1. Build       → go build ./... (para MLs Go)
2. Testes      → go test ./...
3. Gate/Lint   → go vet ./...
4. Commit      → git commit -m "feat(update): <descrição>"
5. Push        → git push origin feat/kanban-roadmap-progress
6. Atualizar roadmap → marcar ML como ✅ Concluído
```

---
status: wip
date: 2026-06-20
req: "REQ-2026-06-20-gate-pre-trabalho-branch-wip-roadmap-e-fallback-husky-node.md"
squad: ""
---

# Roadmap: gate-pre-trabalho-branch-wip-roadmap-e-fallback-husky-node

> Criado em: 2026-06-20 | Status: ✅ Done

REQ: REQ-2026-06-20-gate-pre-trabalho-branch-wip-roadmap-e-fallback-husky-node.md

## Diagnóstico / Contexto

O `trackfw validate` valida consistência entre artefatos existentes, mas é cego à ausência total de
artefatos. Quando um agente cria uma branch `feat/*` sem criar REQ + Roadmap, `wip/` fica vazio e
todos os checks passam — nenhuma violation é gerada.

Dois problemas a resolver:
1. **Gap de gate:** nenhuma regra detecta "branch ativa sem roadmap em wip"
2. **Fallback de hooks no Windows:** lefthook não instala em ambientes corporativos Windows com
   restrições de rede, mas Node.js está disponível → husky deveria ser usado automaticamente

## Acceptance Criteria

- [ ] `trackfw validate` falha com `branch_has_wip_roadmap` em branch feat/fix/refactor sem wip/ roadmap
- [ ] Regra configurável via `trackfw.yaml` (off/warning/error)
- [ ] Paridade Go + Node.js + Python
- [ ] `trackfw init`/`discover --init` detectam Node.js e usam husky quando lefthook indisponível
- [ ] `trackfwRulesBlock()` inclui instrução do protocolo REQ→Roadmap→branch
- [ ] `trackfw update` propaga para todos os agentes
- [ ] Todos os testes verdes

---

## Wave 1 — Validator + agentfiles (paralelo: MLs independentes)
> Dependências: nenhuma

### ML-1A — Regra `branch_has_wip_roadmap` no validator Go
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/validator/validator.go` — nova função `validateBranchHasWIPRoadmap()` + chamada em `ValidateUnfiltered()`

**Ações:**
1. Adicionar função `validateBranchHasWIPRoadmap()`:
   - Executar `git symbolic-ref --short HEAD` para obter a branch atual
   - Se branch não começa com `feat/`, `fix/` ou `refactor/` → retornar nil (skip)
   - Chamar `resolveWIPDirs(cfg)` e contar total de arquivos `.md` em todos os dirs
   - Se total == 0 → retornar `[]string{fmt.Sprintf("branch %q is a feat/fix/refactor branch but no roadmap is in wip/ — create REQ and ROADMAP first with: trackfw req new / trackfw roadmap new / trackfw roadmap move <name> wip", branch)}`
2. Adicionar chamada em `ValidateUnfiltered()` com `applyRule("branch_has_wip_roadmap", ...)` logo após `validateFolderStatusCoherence()`
3. Adicionar chamada equivalente em `validateUnfilteredTagged()` com `applyRuleTagged`

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./...` verde
- [ ] `trackfw validate` em branch feat/* sem wip/ roadmap retorna exit code 1 com mensagem clara

**Comandos de validação:**
```bash
go build ./...
go test ./internal/validator/...
```

---

### ML-1B — Regra `branch_has_wip_roadmap` no CLI Node.js
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/validator.js` (ou equivalente) — nova função `validateBranchHasWIPRoadmap()`

**Ações:**
1. Localizar a função equivalente a `ValidateUnfiltered` no CLI Node.js
2. Adicionar `validateBranchHasWIPRoadmap()` com a mesma lógica:
   - `execSync('git symbolic-ref --short HEAD')` para obter branch
   - Verificar prefixo feat/fix/refactor
   - Contar arquivos em wip dirs
   - Se 0 → violation com mensagem idêntica ao Go
3. Chamar com `applyRule('branch_has_wip_roadmap', ...)` no fluxo principal
4. Adicionar ao `trackfw.yaml` gerado por `init` a regra `branch_has_wip_roadmap: error`

**Critérios de aceite:**
- [ ] `npm test` verde (ou equivalente do workspace)
- [ ] Comportamento idêntico ao Go CLI

**Comandos de validação:**
```bash
cd npm && npm test
```

---

### ML-1C — Regra `branch_has_wip_roadmap` no CLI Python
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/trackfw/validator.py` (ou equivalente) — nova função `validate_branch_has_wip_roadmap()`

**Ações:**
1. Localizar o módulo de validação no CLI Python
2. Adicionar `validate_branch_has_wip_roadmap()`:
   - `subprocess.run(['git', 'symbolic-ref', '--short', 'HEAD'], ...)` para obter branch
   - Verificar prefixo feat/fix/refactor
   - Contar arquivos em wip dirs
   - Se 0 → violation com mensagem idêntica
3. Chamar no fluxo principal de `validate_unfiltered()`

**Critérios de aceite:**
- [ ] `pytest` verde (ou equivalente)
- [ ] Comportamento idêntico ao Go CLI

**Comandos de validação:**
```bash
cd pypi && python -m pytest
```

---

### ML-1D — Atualizar `trackfwRulesBlock()` com protocolo REQ→Roadmap
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/agentfiles.go` — função `trackfwRulesBlock()`

**Ações:**
1. Localizar a seção `### Agent Protocol` dentro de `trackfwRulesBlock()`
2. Adicionar item `0. **Before any implementation:**` com a sequência obrigatória:
   ```
   0. **Before any implementation (mandatory):**
      trackfw req new "title" → trackfw roadmap new "title" → trackfw roadmap move <name> wip → git checkout -b feat/<branch>
      ❌ Never create a branch before REQ + ROADMAP are in wip
      ❌ Never delegate REQ/ROADMAP creation to a future task — they are prerequisites, not deliverables
   ```
3. Garantir que a instrução mencione `branch_has_wip_roadmap` (disponível a partir de v2.7.0)

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `trackfw update` num projeto de teste injeta o novo bloco em CLAUDE.md e demais agentes

**Comandos de validação:**
```bash
go build ./...
go test ./internal/generators/...
```

---

## Wave 2 — Fallback Node.js → Husky (depende de nada da Wave 1)
> Dependências: independente da Wave 1 (pode rodar em paralelo)

### ML-2A — Detecção de Node.js como fallback para husky (Go)
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/discover/discover.go` — função `installHook()` e nova `installHuskyNPX()`

**Ações:**
1. Em `installHook()`, no case `default:`, após checar `package.json`, adicionar:
   ```go
   // Node.js disponível mas sem package.json → husky via npx
   if _, err := exec.LookPath("node"); err == nil {
       fmt.Fprintf(w, "ℹ node detected — using husky (no package.json required)\n")
       return installHuskyNPX(rootDir, w)
   }
   ```
2. Criar `installHuskyNPX(rootDir string, w io.Writer) error`:
   - NÃO rodar `npm install husky` (sem package.json para salvar devDep)
   - Rodar `npx husky init` (cria `.husky/` e instala o hook handler)
   - Criar/append `.husky/pre-commit` com `scripts/trackfw-validate.sh`
   - Tratar erros de npx como warning não-bloqueante (idêntico ao padrão existente)
3. Atualizar `Scan()` para detectar Node.js: se `HookFramework == "none"` e `node` no PATH → definir `HookFramework = "husky-npx"` (ou manter "none" e deixar o install decidir — avaliar qual é mais limpo)

**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/discover/...` verde (adicionar teste `TestInstallHuskyNPX_SemPackageJSON`)
- [ ] Em máquina sem lefthook mas com node: `trackfw discover --init` cria `.husky/pre-commit`

**Comandos de validação:**
```bash
go build ./...
go test ./internal/discover/...
```

---

### ML-2B — Fallback Node.js no CLI Node.js
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/discover.js` (ou equivalente) — função `installHook()`

**Ações:**
1. Replicar a lógica de ML-2A no CLI Node.js:
   - Usar `child_process.execSync('node --version')` ou `which('node')` para detectar Node.js
   - Se disponível e sem package.json → chamar `installHuskyNPX()`
2. `installHuskyNPX()` Node.js: usa `execSync('npx husky init')` e cria `.husky/pre-commit`

**Critérios de aceite:**
- [ ] `npm test` verde
- [ ] Comportamento idêntico ao Go

**Comandos de validação:**
```bash
cd npm && npm test
```

---

### ML-2C — Fallback Node.js no CLI Python
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/trackfw/discover.py` (ou equivalente) — função `install_hook()`

**Ações:**
1. Replicar a lógica de ML-2A no CLI Python:
   - `shutil.which('node')` para detectar Node.js
   - Se disponível e sem package.json → chamar `install_husky_npx()`
2. `install_husky_npx()` Python: usa `subprocess.run(['npx', 'husky', 'init'])` e cria `.husky/pre-commit`

**Critérios de aceite:**
- [ ] `pytest` verde
- [ ] Comportamento idêntico ao Go

**Comandos de validação:**
```bash
cd pypi && python -m pytest
```

---

## Wave 3 — Testes de integração + VISION.md
> Dependências: Wave 1 e Wave 2 completas

### ML-3A — Testes de integração da regra `branch_has_wip_roadmap`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/validator/validator_test.go` (ou arquivo de teste existente)

**Ações:**
1. Adicionar `TestValidateBranchHasWIPRoadmap_Violation`: branch feat/* sem wip/ → deve retornar violation
2. Adicionar `TestValidateBranchHasWIPRoadmap_Pass`: branch feat/* com 1 roadmap em wip/ → sem violation
3. Adicionar `TestValidateBranchHasWIPRoadmap_MainBranch`: branch `main` → skip (sem violation)
4. Adicionar `TestValidateBranchHasWIPRoadmap_ConfigurableOff`: regra `branch_has_wip_roadmap: off` → silencioso

**Critérios de aceite:**
- [ ] `go test ./internal/validator/...` verde com 4 novos testes

**Comandos de validação:**
```bash
go test ./internal/validator/... -v -run TestValidateBranchHasWIPRoadmap
```

---

### ML-3B — Atualizar VISION.md com v2.7.0 e nova regra
**Status:** ✅ Concluído
**Arquivos afetados:**
- `docs/visao-projeto/VISION.md`

**Ações:**
1. Atualizar versão no header para v2.7.0
2. Adicionar `branch_has_wip_roadmap` à tabela de rules de `trackfw validate`
3. Adicionar nota sobre fallback Node.js → husky na seção de `trackfw init`
4. Adicionar v2.7.0 à tabela "Current State"

**Critérios de aceite:**
- [ ] VISION.md reflete as duas novas capacidades

---

## Protocolo de conclusão de cada ML

```
1. Build       → go build ./... (Go) | npm test (Node) | pytest (Python)
2. Testes      → go test ./... | npm test | pytest
3. Commit      → git commit -m "feat(validator): <descrição>"
4. Push        → git push origin feat/gate-pre-trabalho-e-husky-node
5. Atualizar roadmap → marcar ML como ✅ Concluído
```

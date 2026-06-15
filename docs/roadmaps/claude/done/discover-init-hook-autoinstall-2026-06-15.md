---
name: discover-init-hook-autoinstall-2026-06-15
title: "feat: discover --init instala hook framework automaticamente"
status: done
req: docs/requisições/claude/REQ-2026-06-15-discover-init-hook-autoinstall.md
created: 2026-06-15
author: zeus
---

# Roadmap: discover --init — auto-instalação de hook framework

> Criado em: 2026-06-15 | Status: 🔄 WIP

## Diagnóstico / Contexto

`trackfw discover --init` pula a instalação de hook quando nenhum framework é detectado,
tornando `governance_mode: strict` inoperante. A solução é detectar o ecossistema do projeto
(`package.json` presente → husky; caso contrário → lefthook) e instalar automaticamente.

**Branch:** `feat/discover-init-hook-autoinstall`

---

## Wave 1 — Go CLI (base da lógica) — 1 ML

### ML-1A — Implementar auto-instalação de hook no pacote discover (Go)

**Status:** ⬜ Pendente  
**Arquivos afetados:**
- `internal/discover/discover.go` — funções `installHook`, `InstallGates`, novos helpers
- `internal/discover/discover_test.go` — testes das novas funções

**Ações:**

1. Em `installHook(framework, rootDir string)`, no `case default`:
   - Verificar se `filepath.Join(rootDir, "package.json")` existe
   - Se sim → chamar `installHusky(rootDir)`
   - Se não → chamar `installLefthook(rootDir)`

2. Implementar `installLefthook(rootDir string) error`:
   - Criar `lefthook.yml` na raiz com conteúdo:
     ```yaml
     pre-commit:
       commands:
         trackfw-validate:
           run: scripts/trackfw-validate.sh
     ```
   - Executar `lefthook install` via `exec.Command` (se lefthook disponível no PATH)
   - Se lefthook não estiver no PATH, imprimir instrução de instalação e retornar nil (não erro)

3. Implementar `installHusky(rootDir string) error`:
   - Executar `npm install --save-dev husky` via `exec.Command`
   - Executar `npx husky init` via `exec.Command`
   - Criar/append `.husky/pre-commit` com linha `scripts/trackfw-validate.sh`

4. Corrigir `fmt.Println` em `installHook` → usar `io.Writer` passado como parâmetro
   (assinatura: `installHook(framework, rootDir string, w io.Writer) error`)
   Ajustar chamada em `InstallGates` para aceitar e repassar o writer.

5. Adicionar testes em `discover_test.go`:
   - Projeto sem `package.json` → `lefthook.yml` criado
   - Projeto com `package.json` → `.husky/pre-commit` criado
   - Projeto com framework já configurado → sem alteração (idempotente)

**Critérios de aceite:**
- [ ] `make build` sem erros
- [ ] `make test` verde (novos testes passando)
- [ ] `make lint` sem warnings

---

## Wave 2 — Node.js CLI (paridade) — 1 ML

### ML-2A — Paridade no CLI Node.js

**Status:** ⬜ Pendente  
**Dependência:** ML-1A concluído  
**Arquivos afetados:**
- `npm/src/commands/discover.js` — lógica de `installHook`

**Ações:**

1. Localizar a função equivalente a `installHook` em `npm/src/commands/discover.js`
2. No bloco `default` (nenhum framework detectado):
   - Verificar existência de `package.json` no `cwd`
   - Se sim → instalar husky (`execSync('npm install --save-dev husky')`, `execSync('npx husky init')`, append em `.husky/pre-commit`)
   - Se não → criar `lefthook.yml` e tentar `execSync('lefthook install')`
3. Erros de execução capturados como warn (não abortar o `--init`)

**Critérios de aceite:**
- [ ] `node npm/src/index.js discover --init` em projeto sem framework cria `lefthook.yml`
- [ ] Sem regressões nos testes Node existentes

---

## Wave 3 — Python CLI (paridade) — 1 ML

### ML-3A — Paridade no CLI Python

**Status:** ⬜ Pendente  
**Dependência:** ML-1A concluído  
**Arquivos afetados:**
- `pypi/trackfw/commands/discover.py` — lógica de `install_hook`

**Ações:**

1. Localizar a função equivalente a `install_hook` em `pypi/trackfw/commands/discover.py`
2. No bloco `else`/`default` (nenhum framework detectado):
   - Verificar existência de `package.json` no `cwd`
   - Se sim → instalar husky via `subprocess.run`
   - Se não → criar `lefthook.yml` via `open()` e tentar `subprocess.run(['lefthook', 'install'])`
3. `CalledProcessError` capturado como warn

**Critérios de aceite:**
- [ ] `python -m trackfw discover --init` em projeto sem framework cria `lefthook.yml`
- [ ] Sem regressões nos testes Python existentes

---

## Legenda de status
- ⬜ Pendente
- 🔄 Em andamento
- ✅ Concluído
- ❌ Bloqueado

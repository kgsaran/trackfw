---
name: npm-nodejs-rewrite
description: Reescrever o pacote npm de fat-package (Go binary) para Node.js puro
metadata:
  type: project
status: backlog
---

# Roadmap: Reescrita do pacote npm em Node.js puro

> Criado em: 2026-06-12 | Status: ✅ Done

## Diagnóstico / Contexto

O pacote npm atual embute 5 binários Go compilados (~25MB). Em ambientes Windows corporativos o executável `.exe` é bloqueado/quarentenado pelo antivírus antes mesmo de ser executado, tornando o `npm install -g trackfw` inoperante.

Decisão arquitetural: manter o binário Go para distribuição nativa (brew, install.sh, GitHub Releases) e reescrever o pacote **npm como Node.js puro** — sem binário, sem postinstall, sem code signing.

Resultado: distribuição em dois canais independentes com funcionalidade idêntica.

## Dependências npm a adicionar

- `commander` — framework CLI (equivalente ao cobra)
- `@inquirer/prompts` — wizard interativo (equivalente ao huh)

## Estrutura alvo do pacote npm

```
npm/
├── bin/
│   └── trackfw          ← entry point (#!/usr/bin/env node)
├── src/
│   ├── commands/        ← um arquivo por comando
│   ├── generators/      ← lógica de geração de arquivos
│   └── validator/       ← regras de validação e status
└── package.json
```

---

## Wave 1 — Scaffold + infraestrutura (paralelo)

> Independente

### ML-1A — package.json e entry point

**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/package.json`, `npm/bin/trackfw`
**Ações:**
- Substituir `package.json`: remover `files: ["bin/"]` fat-package, adicionar `files: ["bin/", "src/"]`, adicionar deps `commander` e `@inquirer/prompts`, remover restrições `os`/`cpu`, manter `bin: { "trackfw": "./bin/trackfw" }`
- Reescrever `npm/bin/trackfw`: entry point Node.js que importa `src/commands/index.js` e executa o programa
- Remover `npm/bin/.gitkeep`
**Critérios de aceite:**
- [ ] `node npm/bin/trackfw --help` imprime usage sem erro
- [ ] `npm install` dentro de `npm/` instala `commander` e `@inquirer/prompts`

### ML-1B — Estrutura de diretórios src/

**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/src/commands/index.js`, `npm/src/generators/`, `npm/src/validator/`
**Ações:**
- Criar `npm/src/commands/index.js` com scaffolding de todos os subcomandos (stubs que imprimem "TODO")
- Criar `npm/src/generators/` com módulos vazios: `adr.js`, `req.js`, `roadmap.js`, `init.js`
- Criar `npm/src/validator/index.js` vazio
**Critérios de aceite:**
- [ ] `node npm/bin/trackfw adr --help` mostra subcomandos `new` e `list`
- [ ] `node npm/bin/trackfw version` imprime a versão do package.json

---

## Wave 2 — Comandos simples (paralelo, após Wave 1)

> Dependência: Wave 1 completa

### ML-2A — `adr new` e `adr list`

**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/src/generators/adr.js`, `npm/src/commands/adr.js`
**Ações:**
- `generators/adr.js`: porta `NewADR(content)` — cria `docs/adr/YYYY-MM-DD-<slug>.md` com template idêntico ao Go
- `generators/adr.js`: porta `ListADRs(dir)` — lista arquivos `.md` em `docs/adr/`, imprime basename + primeira linha de título
- `commands/adr.js`: registra `adr new <title>` e `adr list` via commander
**Critérios de aceite:**
- [ ] `trackfw adr new "Escolher banco de dados"` cria arquivo com estrutura correta
- [ ] `trackfw adr list` lista ADRs existentes

### ML-2B — `req list` e `roadmap list/show/move`

**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/src/generators/req.js`, `npm/src/generators/roadmap.js`, `npm/src/commands/req.js`, `npm/src/commands/roadmap.js`
**Ações:**
- `generators/req.js`: porta `ListREQs(dir)`
- `generators/roadmap.js`: porta `ListRoadmaps()`, `ShowRoadmap(name)`, `MoveRoadmap(name, state)` com `appendTransitionLog()`
- `commands/req.js`: registra `req list`
- `commands/roadmap.js`: registra `roadmap list`, `roadmap show <name>`, `roadmap move <name> <state>`
**Critérios de aceite:**
- [ ] `trackfw roadmap list` lista roadmaps por estado
- [ ] `trackfw roadmap move <name> wip` move o arquivo para `wip/`
- [ ] `trackfw roadmap show <name>` imprime conteúdo com cabeçalho de estado

### ML-2C — `log` e `version`

**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/src/commands/log.js`
**Ações:**
- Porta `runLog(tail)`: lê `docs/roadmaps/.trackfw-log`, imprime últimas N linhas
- Registra `log [--tail N]`
- `version` já implementado no ML-1B; verificar que imprime corretamente
**Critérios de aceite:**
- [ ] `trackfw log --tail 5` imprime as 5 últimas transições

---

## Wave 3 — Comandos complexos (sequencial, após Wave 2)

> Dependência: Wave 2 completa

### ML-3A — `validate` e `status`

**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/src/validator/index.js`, `npm/src/commands/validate.js`, `npm/src/commands/status.js`
**Ações:**
- Porta as 7 regras de validação de `internal/validator/validator.go`:
  1. ADR sem Status definido
  2. REQ apontando para ADR inexistente
  3. REQ sem critérios de aceite
  4. Roadmap em `wip/` sem REQ vinculada
  5. REQ sem roadmap vinculada
  6. ADR duplicado (mesmo slug)
  7. WIP stale (≥7 dias sem modificação)
- Porta `GetStatus()` — resumo por categoria
- Registra `validate` e `status`
**Critérios de aceite:**
- [ ] `trackfw validate` retorna violations e warnings no mesmo formato do binário Go
- [ ] `trackfw status` mostra resumo de ADRs, REQs, roadmaps e WIP stale

### ML-3B — `req new` (wizard interativo)

**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/src/generators/req.js`, `npm/src/commands/req.js`
**Ações:**
- Porta `DetectDomains(intention)` — detecção de domínios por palavras-chave
- Porta wizard de dois formulários usando `@inquirer/prompts`:
  - Form 1: título + motivação
  - Form 2: critérios de aceite + selects dinâmicos por domínio detectado
- Porta `NewADRDraft(slug)` para geração automática de ADR drafts
- Porta `NewREQ(content)` — cria `docs/req/YYYY-MM-DD-<slug>.md`
**Critérios de aceite:**
- [ ] `trackfw req new "Autenticação OAuth"` abre wizard interativo
- [ ] Domínios detectados geram selects com opções de ADR
- [ ] ADR drafts criados automaticamente ao final

### ML-3C — `init` (wizard de inicialização)

**Status:** ⬜ Pendente
**Arquivos afetados:** `npm/src/generators/init.js`, `npm/src/commands/init.js`
**Ações:**
- Porta wizard condicional por tipo de projeto (API, Frontend, Full-stack, etc.)
- Cria estrutura de pastas `docs/` e `CLAUDE.md` com template correto
**Critérios de aceite:**
- [ ] `trackfw init` em diretório vazio cria estrutura completa
- [ ] `CLAUDE.md` gerado contém o nome do projeto inferido do diretório

---

## Wave 4 — Finalização (após Wave 3)

> Dependência: Wave 3 completa

### ML-4A — `plugins list/add/remove` e plugin dispatch

**Status:** ✅ Concluído
**Arquivos afetados:** `npm/src/commands/plugins.js`
**Ações:**
- Porta `Dir()`, `List()`, `Install()`, `Remove()` de `internal/plugins/plugins.go`
- Porta dispatch automático para comandos desconhecidos (equivalente ao `rootCmd.RunE`)
**Critérios de aceite:**
- [ ] `trackfw plugins list` lista plugins em `~/.trackfw/plugins/`

### ML-4B — Atualização do CI (release.yml)

**Status:** ✅ Concluído
**Arquivos afetados:** `.github/workflows/release.yml`
**Ações:**
- Remover step "Embute binários no pacote" do job `publish-npm`
- O job passa a fazer apenas: `cp README.md`, `npm version`, `npm publish`
- Remover `npm/bin/.gitkeep` (não há mais binários em `bin/`)
**Critérios de aceite:**
- [ ] Job `publish-npm` não faz mais download de binários da release
- [ ] `npm publish` publica pacote leve (~100KB)

### ML-4C — Limpeza de arquivos legados

**Status:** ✅ Concluído
**Arquivos afetados:** `npm/bin/.gitkeep`, `.goreleaser.yaml` (sem mudanças — permanece igual)
**Ações:**
- Remover `npm/bin/.gitkeep`
- Confirmar que `.goreleaser.yaml` não precisa de mudanças (continua gerando binários para brew/install.sh)
**Critérios de aceite:**
- [ ] Repositório sem arquivos legados do fat-package
- [ ] GoReleaser continua funcionando para brew e install.sh

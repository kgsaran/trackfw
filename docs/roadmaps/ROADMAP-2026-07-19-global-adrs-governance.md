---
status: wip
date: 2026-07-19
req: "docs/req/REQ-2026-07-19-global-adrs-governance.md"
adr: "docs/adr/ADR-2026-07-19-global-adrs-governance.md"
branch: "feat/global-adrs-governance"
---

# Roadmap: Suporte a ADRs Globais Compartilhados e Diretivas de IA

> Criado em: 2026-07-19 | Status: 🔄 WIP
> REQ: `docs/req/REQ-2026-07-19-global-adrs-governance.md`
> ADR: `docs/adr/ADR-2026-07-19-global-adrs-governance.md`

## Diagnóstico / Contexto

Organizações necessitam compartilhar decisões de arquitetura (ADRs) entre múltiplos repositórios sem duplicar arquivos.
O `trackfw` deve aceitar caminhos com expansão de til (`~`) na diretiva `adr_dirs` do `trackfw.yaml`.
Além disso, em ambientes de CI/CD limpos (runners), diretórios de Home do desenvolvedor local podem não existir. O validador não deve falhar a build por causa de caminhos externos ausentes (emitindo `Warning` em vez de `Error`, salvo se `strict_ci_paths: true`).
ADRs globais/externos também não devem ser marcados como `adr_orphan` quando escaneados.

---

## Wave 1 — Expansão de Til (`~`) no Carregamento de Caminhos (3 MLs paralelos)
> Dependências: Nenhuma

### ML-1A — Go: Expansão de `~` em `adr_dirs`
**Status:** ✅ Concluído
**Agente:** Apolo (Backend Specialist)
**Arquivos afetados:**
- `internal/config/config.go`
- `internal/validator/validator.go`
- `internal/config/config_paths_test.go`
- `internal/validator/validator_test.go`

**Ações:**
1. Criar utilitário de resolução de caminho em `internal/config` (ou estender `expandPath`) utilizando `os.UserHomeDir()` para substituir o prefixo `~` ou `~/`.
2. Aplicar essa resolução ao carregar e iterar sobre a lista `adr_dirs`.
3. Adicionar testes unitários validando a expansão de `~/...` para o diretório Home do usuário.

---

### ML-1B — Node.js: Expansão de `~` em `adr_dirs`
**Status:** ✅ Concluído
**Agente:** Afrodite (Frontend/Node Specialist)
**Arquivos afetados:**
- `npm/src/config/index.js` (ou `npm/src/config/config.js`)
- `npm/src/validator/index.js`
- `npm/tests/config.test.js`
- `npm/tests/validator.test.js`

**Ações:**
1. Criar helper de expansão usando `os.homedir()` e `path.join()`.
2. Aplicar a expansão na leitura e validação de `adr_dirs`.
3. Adicionar testes para validação do caminho com `~`.

---

### ML-1C — Python: Expansão de `~` em `adr_dirs`
**Status:** ✅ Concluído
**Agente:** Apolo (Backend Specialist)
**Arquivos afetados:**
- `pypi/trackfw/config.py`
- `pypi/trackfw/validator.py`
- `pypi/tests/test_config.py`
- `pypi/tests/test_validator.py`

**Ações:**
1. Usar `os.path.expanduser` na leitura e iteração de `adr_dirs`.
2. Adicionar testes cobrindo a expansão de caminhos `~/...`.

---

## Wave 2 — Resiliência CI/CD & Regra adr_orphan (3 MLs paralelos)
> Dependências: Wave 1 concluída

### ML-2A — Go: Bypass de CI/CD para Dirs Inexistentes + Isenção adr_orphan
**Status:** ⬜ Pendente
**Agente:** Apolo (Backend Specialist)
**Arquivos afetados:**
- `internal/config/config.go` (adicionar campo `StrictCIPaths bool` em Config)
- `internal/validator/validator.go`
- `internal/validator/validator_test.go`

**Ações:**
1. No validador Go, quando um diretório em `adr_dirs` não for encontrado:
   - Se `strict_ci_paths: true` em `trackfw.yaml` → retornar `Error`.
   - Se `strict_ci_paths: false` ou omisso → registrar `Warning` indicando que o diretório externo não foi encontrado no runner.
2. Na verificação de `adr_orphan`, filtrar arquivos cujo caminho absoluto não esteja contido no `cwd` (raiz do projeto local).
3. Adicionar testes unitários para o comportamento de `Warning` e `adr_orphan` em caminhos externos.

---

### ML-2B — Node.js: Bypass de CI/CD para Dirs Inexistentes + Isenção adr_orphan
**Status:** ⬜ Pendente
**Agente:** Afrodite (Frontend/Node Specialist)
**Arquivos afetados:**
- `npm/src/config/index.js`
- `npm/src/validator/index.js`
- `npm/tests/validator.test.js`

**Ações:**
1. Tratar diretórios externos não encontrados como `Warning` no validador Node (a menos que `strict_ci_paths: true`).
2. Isentar ADRs externos da verificação `adr_orphan`.
3. Adicionar testes unitários cobrindo esses dois cenários.

---

### ML-2C — Python: Bypass de CI/CD para Dirs Inexistentes + Isenção adr_orphan
**Status:** ⬜ Pendente
**Agente:** Apolo (Backend Specialist)
**Arquivos afetados:**
- `pypi/trackfw/config.py`
- `pypi/trackfw/validator.py`
- `pypi/tests/test_validator.py`

**Ações:**
1. Implementar tratamento de `Warning` para diretórios `adr_dirs` externos não encontrados (respeitando `strict_ci_paths`).
2. Isentar caminhos fora de `cwd` da regra `adr_orphan`.
3. Adicionar testes unitários.

---

## Wave 3 — Diretivas Obrigatórias nos Geradores de Regras (2 MLs paralelos)
> Dependências: Wave 2 concluída

### ML-3A — Go & Node.js: Injeção de Diretiva de ADRs Globais
**Status:** ⬜ Pendente
**Agente:** Apolo (Backend Specialist)
**Arquivos afetados:**
- `internal/generators/claudemd.go`
- `internal/generators/scaffold.go`
- `npm/src/generators/init.js`
- `internal/generators/claudemd_test.go`
- `npm/tests/generators.test.js`

**Ações:**
1. Adicionar ao bloco de regras dos assistentes de IA (`CLAUDE.md` / `AGENTS.md`) a instrução explícita:
   `"Obrigatório: Inspecione e respeite todos os ADRs globais nos diretórios listados em adr_dirs (inclusive caminhos ~/...) antes de propor alterações de arquitetura."`
2. Validar que `trackfw init` e `trackfw update` geram/injetam a nova regra corretamente.

---

### ML-3B — Python: Injeção de Diretiva de ADRs Globais
**Status:** ⬜ Pendente
**Agente:** Apolo (Backend Specialist)
**Arquivos afetados:**
- `pypi/trackfw/generators/init_gen.py`
- `pypi/tests/test_generators.py`

**Ações:**
1. Atualizar o gerador Python para incluir a diretiva de ADRs globais no bloco de regras.
2. Adicionar teste automatizado.

---

## Wave 4 — Validação E2E e Fechamento de Versão (Sequencial)
> Dependências: Waves 1, 2 e 3 concluídas

### ML-4A — Testes de Integração e Paridade das 3 Distribuições
**Status:** ⬜ Pendente
**Agente:** Artemis (QA Specialist)
**Ações:**
1. Executar suítes completas de teste (`go test ./...`, `npm test`, `pytest`).
2. Garantir 100% de paridade nos relatórios de validação dos 3 CLIs com diretórios globais simulados em `~/...`.

---

## Protocolo de Conclusão por Agente
1. Build e testes unitários verdes no módulo modificado.
2. Atualizar este roadmap marcando o ML como `✅ Concluído`.
3. Notificar Zeus para auditoria e avanço para a próxima wave.

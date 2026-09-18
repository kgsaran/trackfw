---
status: done
date: 2026-08-08
req: ""
squad: ""
---

# Roadmap: adr new com escopo global (--scope global) escrevendo em ~/.trackfw/adr

> Created: 2026-08-08 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-08-08-adr-new-com-escopo-global-scope-global-escrevendo-em-trackfw-adr.md

Feature aditiva: novo flag `--scope global|project` (default `project`,
preserva 100% do comportamento atual) em `trackfw adr new`/`trackfw adr
list`, para criar/listar ADRs cross-project em `~/.trackfw/adr/` — mesmo
diretório-base já usado por `~/.trackfw/scripts/` (credential-guard) e
`~/.trackfw/identity.json`. Cada stack tem seu próprio ML porque os
arquivos afetados são disjuntos (nenhum ML depende do resultado de outro)
— paralelizável.

**Fora de escopo (ver REQ):** `validate`/`status`/`context` não passam a
varrer `~/.trackfw/adr` implicitamente; `NewADRDraft`/o fluxo `req`→ADR
draft não ganha escopo global; o `--dir`/`--status` que já existem só em
Python (`pypi/trackfw/commands/adr.py`) não são tocados nem estendidos aos
outros stacks.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] `--scope global` escreve em `~/.trackfw/adr/ADR-<data>-<slug>.md` nos
      3 stacks, sem exigir `trackfw.yaml`/raiz de projeto
- [ ] `--scope project` (default) idêntico ao comportamento atual, byte a
      byte, nos 3 stacks
- [ ] `adr list --scope global` lista `~/.trackfw/adr/*.md`
- [ ] Testes com `$HOME` de fixture (nunca o `$HOME` real) verdes nos 3 stacks
- [ ] `docs/cli-parity.md` documenta a nova flag
- [ ] `trackfw validate` sem violações

## Wave 1 — Implementação por stack (3 MLs paralelos, arquivos disjuntos)
> Dependencies: none

### ML-1A — Go: --scope global em `adr new`/`adr list`
**Status:** ✅ Concluído
**Files affected:**
- `internal/generators/scaffold.go` — nova função `GlobalADRDir(home string) string`,
  retorna `filepath.Join(home, ".trackfw", "adr")`, mesmo padrão de
  `GlobalClaudeSkillPath(home)` já existente no mesmo arquivo
- `internal/generators/adr.go` — `NewADR(content ADRContent)` passa a
  `NewADR(content ADRContent, adrDir string) error`: o chamador resolve o
  diretório (via `config.Load().ADRDirs[0]` para `--scope project` ou via
  `GlobalADRDir(home)` para `--scope global`) e passa explicitamente; a
  função não chama mais `config.Load()` internamente. Único chamador
  existente é `internal/commands/adr.go` — atualizar junto.
- `internal/commands/adr.go` — adicionar flag `--scope` (`cmd.Flags().String("scope", "project", ...)`)
  em `newADRNewCmd()` e `newADRListCmd()`; validar valor (`project`/`global`,
  erro claro em qualquer outro valor); resolver `home, err := os.UserHomeDir()`
  só quando `scope == "global"`.
- `internal/generators/adr_test.go` (ou arquivo de teste equivalente) —
  testes novos para `--scope global` com `$HOME` de fixture (`t.TempDir()`)
**Actions:**
- Não alterar `NewADRDraft` (usado por `internal/commands/req.go`) — fica
  exclusivamente project-scoped, sem mudança.
- `ListADRs(dir string)` já aceita `dir` explícito — não precisa mudar de
  assinatura; só o comando (`newADRListCmd`) precisa resolver `dir` conforme
  `--scope` antes de chamar.
- Escrita em `--scope global` não deve exigir `trackfw.yaml` no cwd (mesmo
  padrão de `UpdateHarness` — nunca requer projeto).
**Acceptance criteria:**
- [ ] `go build ./... && go vet ./...` sem erros
- [ ] `go test ./internal/generators/... ./internal/commands/...` verde
- [ ] `trackfw adr new "teste" --scope global` (com `$HOME` isolado) cria o
      arquivo em `$HOME/.trackfw/adr/`, sem exigir `trackfw.yaml`
**Comandos de validação:** `go build ./... && go vet ./... && go test ./internal/generators/... ./internal/commands/...`

### ML-1B — Node.js: --scope global em `adr new`/`adr list`
**Status:** ✅ Concluído
**Files affected:**
- `npm/src/generators/adr.js` — `newADR(content)`/`listADRs(dir)`: `newADR`
  passa a aceitar um segundo parâmetro `adrDir` (o chamador resolve via
  `require('../config').load().adrDirs[0]` para escopo `project` ou via
  `path.join(os.homedir(), '.trackfw', 'adr')` para `global`); `listADRs`
  já recebe `dir` explícito, sem mudança de assinatura
- `npm/src/commands/adr.js` — adicionar `.option('--scope <scope>', ...)`
  em `new`/`list` (default `'project'`), validar valor, resolver `adrDir`
  antes de chamar o generator
- `npm/tests/adr.test.js` (não existe hoje — criar do zero) — casos de
  `--scope global`/`--scope project` com `HOME` de fixture (`fs.mkdtempSync`)
**Actions:**
- Mesma regra do Go: não exigir `trackfw.yaml`/cwd de projeto para
  `--scope global`.
- Confirmar com `grep -rn "adrDirs\[0\]" npm/src/generators/adr.js` que
  todos os pontos de resolução de diretório passam a receber o parâmetro
  explícito, não recalculam a config internamente.
**Acceptance criteria:**
- [ ] `node --test npm/tests/*.test.js` verde
- [ ] `trackfw adr new "teste" --scope global` (Node, `HOME` isolado) cria o
      arquivo em `$HOME/.trackfw/adr/`
**Comandos de validação:** `node --test npm/tests/*.test.js`

### ML-1C — Python: --scope global em `adr new`/`adr list`
**Status:** ✅ Concluído
**Files affected:**
- `pypi/trackfw/generators/adr.py` — nova função `global_adr_dir(home)` →
  `os.path.join(home, ".trackfw", "adr")`; `generate_adr` já aceita
  `adr_dirs` explícito (sem mudança de assinatura) — o comando é quem decide
  a lista.
- `pypi/trackfw/commands/adr.py` — adicionar `--scope` (`choices=["project","global"]`,
  `default="project"`) em `new` e no (novo) subcomando `list` — hoje só existe
  `new` neste arquivo (verificar se `list` já existe em algum outro módulo
  Python antes de criar duplicado; se não existir, criar o subcomando `list`
  aqui mesmo, espelhando Go/Node, já com `--scope`). Quando `--scope global`,
  `adr_dirs = [global_adr_dir(home)]` — precedência sobre `--dir` se ambos
  forem passados (erro claro, não silenciar um dos dois).
- `pypi/tests/test_generators_adr.py` (já existe — estender com os novos
  casos, não criar arquivo duplicado) — casos de `--scope global` com
  `tmp_path`/`HOME` de fixture via `monkeypatch`.
**Actions:**
- Não remover nem alterar o comportamento existente de `--dir`/`--status`
  (fora de escopo desta REQ — ver Context).
- Confirmar que `trackfw adr list` (Python) hoje existe ou não antes de
  decidir se ML cria o subcomando do zero ou só adiciona `--scope` a um já
  existente — reportar o achado no relatório final do ML.
**Acceptance criteria:**
- [ ] `python3 -m pytest pypi/tests/` verde
- [ ] `trackfw adr new "teste" --scope global` (Python, `HOME` isolado) cria
      o arquivo em `$HOME/.trackfw/adr/`
**Comandos de validação:** `python3 -m pytest pypi/tests/`

## Wave 2 — Documentação e auditoria final (1 ML, orquestrador)
> Dependencies: Wave 1 completa

### ML-2A — Atualizar cli-parity.md e confirmar paridade cross-stack
**Status:** ✅ Concluído
**Files affected:**
- `docs/cli-parity.md` — seção `adr new <title>`: documentado `--scope
  project|global`, caminho de destino em cada escopo, escopo deliberadamente
  limitado (validate/status/context não varrem `~/.trackfw/adr`
  implicitamente), e a exceção Python-only pré-existente (`--dir`/`--status`,
  mutuamente exclusivos com `--scope global`) incluindo a nota de que
  `adr list` não existia em Python antes desta feature.
**Actions:**
- Build+test completo dos 3 stacks — verde.
- Confirmado manualmente com `HOME` isolado por stack: os 3 CLIs criam o ADR
  global no mesmo caminho relativo (`.trackfw/adr/ADR-<data>-<slug>.md`) com
  conteúdo **byte-idêntico** (`diff` vazio Go-vs-Node e Go-vs-Python) e
  `adr list --scope global` produz a mesma linha nos 3 stacks.
**Acceptance criteria:**
- [x] `go build ./... && go vet ./... && go test ./...` verde
- [x] `node --test npm/tests/*.test.js` verde (437 pass)
- [x] `python3 -m pytest pypi/tests/` verde (980 passed)
- [x] `trackfw validate` sem violações (após mover roadmap/REQ para done)
**Comandos de validação:** `go build ./... && go vet ./... && go test ./... && node --test npm/tests/*.test.js && python3 -m pytest pypi/tests/ && trackfw validate`

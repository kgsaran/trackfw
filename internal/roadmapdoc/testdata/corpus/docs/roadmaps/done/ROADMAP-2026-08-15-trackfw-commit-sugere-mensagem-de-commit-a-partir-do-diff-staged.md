---
status: done
date: 2026-08-15
req: "docs/req/REQ-2026-08-15-trackfw-commit-sugere-mensagem-de-commit-a-partir-do-diff-staged.md"
squad: ""
---

# Roadmap: trackfw commit sugere mensagem de commit a partir do diff staged

> Created: 2026-08-15 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-08-15-trackfw-commit-sugere-mensagem-de-commit-a-partir-do-diff-staged.md -->
REQ: docs/req/REQ-2026-08-15-trackfw-commit-sugere-mensagem-de-commit-a-partir-do-diff-staged.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] `trackfw commit --suggest` (sem `-m`) imprime um esqueleto de mensagem e sai sem
      commitar.
- [x] Esqueleto inclui tipo Conventional Commits sugerido por heurística simples + lista
      de arquivos staged por status + placeholder de corpo.
- [x] `trackfw commit -m "..."` (uso normal) continua funcionando exatamente como hoje.
- [x] Comportamento idêntico nos 3 CLIs.
- [x] `make quality` passa sem novas divergências de paridade.

## Diagnóstico / Contexto
`trackfw commit` hoje (`internal/commands/commit.go`) exige `-m` obrigatório
(`newCommitCmd`, linha ~79-81) e só repassa a mensagem para `git commit -m <message>`
depois da checagem de governança — não analisa o diff staged, não sugere nada.
Confirmado com o usuário: escopo deliberadamente **sem chamada a LLM** — sugestão
heurística/estrutural, não geração de prosa natural.

Ponto de inserção: `newCommitCmd`'s `RunE`, ANTES da checagem `if strings.TrimSpace(message) == ""`
atual (linha ~79) — se a flag `--suggest` estiver presente, tomar um caminho totalmente
separado que nunca chega em `runCommit`/`git commit` real.

## Wave 1 — Go (implementação de referência, 1 ML)
> Dependências: nenhuma

### ML-1A — `internal/commands/commit.go`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/commands/commit.go` (`newCommitCmd`, nova função `buildSuggestedMessage`)
- `internal/commands/commit_test.go`
**Ações:**
1. Adicionar flag `--suggest` (bool) em `newCommitCmd`. No `RunE`, se `suggest == true`:
   ignorar completamente a exigência de `-m` (não retornar erro de mensagem vazia),
   chamar uma nova função `buildSuggestedMessage(deps) (string, error)`, imprimir o
   resultado em `cmd.OutOrStdout()` e retornar `nil` sem nunca chamar `runCommit`/
   `execGitCommit` — nenhum commit real deve acontecer nesse modo.
2. `buildSuggestedMessage`:
   a. Ler arquivos staged via `git diff --cached --name-status` (novo campo injetável em
      `commitDeps`, ex. `stagedNameStatus func() (string, error)` — seguir o padrão de
      dependências injetáveis já usado no arquivo, não chamar `exec.Command` direto no
      meio da lógica de negócio).
   b. Se não houver nada staged: retornar mensagem de erro clara ("nothing staged — `git
      add` files first"), sem heurística.
   c. Heurística de tipo Conventional Commits, nesta ordem de prioridade (primeira que
      bater vence):
      - Todos os arquivos staged casam com `*_test.go`, `*.test.js`, `test_*.py`,
        `*_test.py` → `test`
      - Todos os arquivos staged estão sob `docs/` ou `vault/`, ou têm extensão `.md` →
        `docs`
      - Existe pelo menos 1 arquivo com status `A` (novo) em `internal/commands/`,
        `npm/src/commands/`, `pypi/trackfw/commands/`, ou diretórios de comando
        equivalentes → `feat`
      - Caso contrário → `fix`
      (Documentar essa ordem exata num comentário — é uma heurística simples e
      declarada como tal, não pretende ser perfeita.)
   d. Montar a saída:
      ```
      # Sugestão heurística — NÃO é uma mensagem pronta, revise antes de usar.
      # Tipo sugerido: <tipo>

      <tipo>(<escopo?>): <descrição>

      ## Arquivos staged
      A  <arquivo1>
      M  <arquivo2>
      D  <arquivo3>
      ...
      ```
      onde `<escopo>`/`<descrição>` ficam como placeholder literal para o usuário
      preencher (ex.: `<tipo>(<escopo>): <descrição>` sem tentar adivinhar texto livre).
3. Documentar explicitamente no `Long` do comando e no início da saída do `--suggest`
   que é heurística de apoio, não mensagem pronta (já coberto no template acima — não
   deixar a ressalva só no código, ela precisa aparecer na saída real).
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/commands/... -run TestCommit` verde, incluindo casos novos:
      `--suggest` sem nada staged → erro claro, sem commit; `--suggest` com só arquivos
      de teste staged → tipo `test`; só docs/.md → tipo `docs`; novo arquivo em
      `internal/commands/` → tipo `feat`; caso genérico → tipo `fix`; `--suggest` nunca
      invoca `execGitCommit` (usar um fake que falha o teste se for chamado); uso normal
      `-m "..."` sem `--suggest` continua idêntico ao comportamento atual (não
      regressão)
- [ ] `go vet ./...` sem warnings
**Comandos de validação:** `go build ./... && go test ./internal/commands/... && go vet ./...`

## Wave 2 — Node.js e Python (2 MLs em paralelo — arquivos distintos por stack)
> Dependências: Wave 1 completa

### ML-2A — Node.js
**Status:** ✅ Concluído
**Arquivos afetados:** `npm/src/commit/runner.js`, `npm/src/commands/commit.js`, teste
equivalente
**Ações:** replicar 1:1 a lógica do ML-1A em JS puro, lendo o Go real (já implementado
nesta branch) como fonte de verdade — mesma heurística de tipo, mesmo template de saída
(texto byte-idêntico onde não depender dos dados reais staged).
**Critérios de aceite:**
- [ ] testes do workspace Node verdes, mesmos casos do ML-1A
- [ ] template de saída idêntico ao Go
**Comandos de validação:** `npm test --workspace=npm` (ajustar nome real do workspace)

### ML-2B — Python
**Status:** ✅ Concluído
**Arquivos afetados:** `pypi/trackfw/commands/commit.py`, teste equivalente
**Ações:** replicar 1:1 a lógica do ML-1A em Python puro, lendo o Go real como fonte de
verdade.
**Critérios de aceite:**
- [ ] `pytest pypi/tests -k commit` verde, mesmos casos do ML-1A
- [ ] template de saída idêntico ao Go
**Comandos de validação:** `python -m pytest pypi/tests -k commit`

## Wave 3 — Validação cruzada (1 ML)
> Dependências: Wave 2 completa

### ML-3A — Paridade e teste manual
**Status:** ✅ Concluído

**Execução real:** testado manualmente `trackfw commit --suggest` com arquivo `.md`
staged (tipo `docs` detectado corretamente) e arquivo `*_test.go` staged (tipo `test`
detectado corretamente), confirmando em ambos os casos que `git status` fica inalterado
antes/depois — nenhum commit real é gerado. `make quality` completo, zero falha.
**Arquivos afetados:** nenhum novo
**Ações:**
1. Rodar `make quality` na raiz.
2. Teste manual: stage um conjunto real de arquivos de tipos variados (ex.: um arquivo
   de teste + um arquivo de doc) neste repo, rodar `trackfw commit --suggest` nos 3
   binários e conferir a saída; confirmar que nenhum commit real aconteceu
   (`git status` inalterado antes/depois); descomitar/limpar antes de finalizar.
**Critérios de aceite:**
- [ ] `make quality` verde
- [ ] teste manual confirmado, nenhum commit real gerado pelo `--suggest`
**Comandos de validação:** `make quality`

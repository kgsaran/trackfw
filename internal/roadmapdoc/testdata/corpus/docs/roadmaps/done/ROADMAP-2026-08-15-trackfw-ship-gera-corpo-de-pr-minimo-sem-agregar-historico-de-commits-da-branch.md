---
status: done
date: 2026-08-15
req: "docs/req/REQ-2026-08-15-trackfw-ship-gera-corpo-de-pr-minimo-sem-agregar-historico-de-commits-da-branch.md"
squad: ""
---

# Roadmap: trackfw ship gera corpo de PR minimo sem agregar historico de commits da branch

> Created: 2026-08-15 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-08-15-trackfw-ship-gera-corpo-de-pr-minimo-sem-agregar-historico-de-commits-da-branch.md -->
REQ: docs/req/REQ-2026-08-15-trackfw-ship-gera-corpo-de-pr-minimo-sem-agregar-historico-de-commits-da-branch.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] `buildPRBody`/título de PR (Go/Node/Python) agregam `git log <base>..HEAD --no-merges`
      em vez do corpo mínimo atual.
- [x] `trackfw ship` ganha exceção doc-only para os 2 gates que hoje bloqueiam push de
      commits puramente de documentação (`isShipBranch` + `CheckShipGovernance`).
- [x] `make quality` passa sem novas divergências de paridade.

## Diagnóstico / Contexto
Achado real ao abrir o PR #169 desta sessão (`internal/commands/ship.go`):
`title := firstLine(opts.message)` e `body := buildPRBody(branch)` — onde
`buildPRBody` é literalmente `fmt.Sprintf("Branch: %s\n\nCreated by trackfw ship.", branch)`.
Ignora os outros commits já feitos na branch. Corrigido manualmente via `gh pr edit`
após o fato — este roadmap resolve isso na origem.

Achado adicional, reproduzido ao vivo tentando commitar
`REQ-2026-08-15-trackfw-commit-sugere-mensagem-...` para `backlog/`: com o
`git-branch-guard` (roadmap irmão, já mergeado) bloqueando `git push` bruto, `trackfw
ship` é o único caminho de push — mas tem 2 gates sem exceção para doc-only, confirmados
por teste direto do binário:
1. `isShipBranch` (linha ~161) exige `feat|fix|refactor/<slug>` sem exceção.
2. `CheckShipGovernance`/`validateBranchHasWIPRoadmap` (mesmo numa branch `feat/`
   corretamente nomeada) exige roadmap em `wip/`, sem exceção.

Ponto de inserção no fluxo (Go, `runShip`, `internal/commands/ship.go`): hoje a leitura
dos arquivos staged (`git diff --cached --name-only`, variável `cachedFiles`) só acontece
no **Step 4** (linha ~216), DEPOIS do Step 1 (branch, linha ~147) e Step 2 (governança,
linha ~171). Para condicionar os 2 gates ao conteúdo staged, a leitura de `cachedFiles`
precisa subir para ANTES do Step 1.

## Wave 1 — Go (implementação de referência, 1 ML)
> Dependências: nenhuma

### ML-1A — `internal/commands/ship.go` + `internal/validator/validator.go`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/commands/ship.go` (`runShip`, `buildPRBody`, `firstLine`, `isShipBranch`)
- `internal/validator/validator.go` (`CheckShipGovernance`, `validateBranchHasWIPRoadmap`)
- `internal/commands/ship_test.go`
**Ações:**
1. Em `runShip`, mover a leitura de `git diff --cached --name-only` para o INÍCIO da
   função (antes do atual Step 1), guardando em uma variável `stagedFiles []string`
   (split por linha, trim). Calcular `docOnly := allDocOnly(stagedFiles)`, onde
   `allDocOnly` é uma função nova: retorna `true` somente se `len(stagedFiles) > 0` e
   TODOS os arquivos estiverem sob `docs/` ou `vault/` (prefixo de path) OU tiverem
   extensão `.md` — um único arquivo fora desse critério já faz retornar `false`.
   Reaproveitar essa mesma leitura para o Step 4 existente (não duplicar a chamada de
   `git diff --cached --name-only`).
2. No Step 1 (branch pattern): se `docOnly == true`, pular a checagem `isShipBranch` —
   qualquer nome de branch é aceito (ainda bloqueando `main`/`master`, essa checagem
   permanece incondicional). Se `docOnly == false`, comportamento idêntico ao atual.
3. No Step 2 (governança): se `docOnly == true`, pular a chamada a
   `deps.checkGovernance()` inteiramente (não chamar `CheckShipGovernance`). Se
   `docOnly == false`, comportamento idêntico ao atual. Imprimir uma linha indicando o
   motivo do skip quando aplicável (ex.: `Governance: skipped (doc-only change)`), para
   não parecer que a checagem rodou e passou silenciosamente.
4. `buildPRBody(branch string, commits []string) string` — nova assinatura, recebendo as
   mensagens de commit completas (não só a primeira linha) de
   `git log <base>..HEAD --no-merges --format=%B` (separador entre commits: `%x00` ou
   uma linha `---` — decidir e documentar; usar algo que sobreviva ao parse posterior
   sem ambiguidade com conteúdo de commit message real). Corpo gerado:
   ```
   ## Commits

   - <primeira linha do commit 1>
   - <primeira linha do commit 2>
   ...

   ## Detalhes

   <corpo completo de cada commit, em blocos, se houver mais que a primeira linha>

   ---
   Branch: <branch>
   ```
   Zero commits não-merge além do atual (`opts.message`): manter o comportamento atual
   (corpo mínimo), não regressão.
5. `title`: se `len(commits) == 1` (só o commit que o próprio `ship` acabou de fazer),
   manter `firstLine(opts.message)` (comportamento atual, não regressão). Se
   `len(commits) > 1`, usar `firstLine(opts.message)` mesmo assim — a mensagem `-m`
   passada na chamada atual de `ship` já é, por convenção desta sessão, a mensagem-resumo
   do PR inteiro (documentar essa decisão de design no comentário da função, é a
   interpretação mais simples e sem ambiguidade adicional — não inventar heurística de
   "título do PR" a partir de N commits distintos).
6. Resolução de branch base para `git log <base>..HEAD`: função nova
   `defaultBaseBranch(execGit) string` — tenta `git symbolic-ref refs/remotes/origin/HEAD`
   (formato `refs/remotes/origin/main`, extrair a última parte), fallback `"main"` se
   falhar. Mesmo padrão já usado em `internal/commands/commit.go` (branch protegida) e
   `internal/validator/validator.go`.
7. `--dry-run`: garantir que o corpo/título calculados aparecem no output do dry-run
   (hoje o dry-run já intercepta comandos de escrita — confirmar que a leitura de `git
   log`/`git diff --cached` continua rodando mesmo em dry-run, já que são comandos de
   leitura, não escrita — `isGitWriteCmd` não deve interceptá-los).
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/commands/... ./internal/validator/...` verde, incluindo casos
      novos: branch `docs/<slug>` com só arquivos doc-only staged → `ship` permite;
      branch `feat/<slug>` sem roadmap em `wip/` com só doc-only staged → `ship` permite;
      qualquer uma das duas com 1 arquivo de código staged junto → `ship` bloqueia como
      hoje; `buildPRBody` com 1 commit → corpo mínimo atual (não regressão); com N
      commits → corpo agregado.
- [ ] `go vet ./...` sem warnings
**Comandos de validação:** `go build ./... && go test ./internal/commands/... ./internal/validator/... && go vet ./...`

## Wave 2 — Node.js e Python (2 MLs em paralelo — arquivos distintos por stack)
> Dependências: Wave 1 completa (Go é a referência comportamental — mesmo padrão de
> paridade já usado nos MLs do roadmap `git-branch-guard`)

### ML-2A — Node.js
**Status:** ✅ Concluído
**Arquivos afetados:** módulo Node equivalente a `internal/commands/ship.go` (localizar
via `npm/src/commands/ship.js` ou `npm/src/ship/runner.js` — confirmar nome exato,
seguindo o padrão já estabelecido por `npm/src/commit/runner.js` no roadmap anterior),
teste equivalente
**Ações:** replicar 1:1 a lógica dos passos 1-7 do ML-1A em JS puro, lendo o Go real
(já implementado nesta branch) como fonte de verdade em vez de re-derivar só da prosa
deste roadmap — mesma orientação que funcionou bem nos MLs anteriores deste conjunto de
roadmaps.
**Critérios de aceite:**
- [ ] testes do workspace Node verdes, mesmos casos do ML-1A
- [ ] mensagens/formatação do corpo de PR idênticas (byte-a-byte onde aplicável) às do Go
**Comandos de validação:** `npm test --workspace=npm` (ajustar nome real do workspace)

### ML-2B — Python
**Status:** ✅ Concluído
**Arquivos afetados:** módulo Python equivalente a `internal/commands/ship.go`
(`pypi/trackfw/commands/ship.py` — confirmar), teste equivalente
**Ações:** replicar 1:1 a lógica dos passos 1-7 do ML-1A em Python puro, lendo o Go real
como fonte de verdade (mesma orientação do ML-2A).
**Critérios de aceite:**
- [ ] `pytest pypi/tests -k ship` verde, mesmos casos do ML-1A
- [ ] mensagens/formatação idênticas ao Go, mesma ressalva do ML-2A
**Comandos de validação:** `python -m pytest pypi/tests -k ship`

## Wave 3 — Validação cruzada (1 ML)
> Dependências: Wave 2 completa

### ML-3A — Paridade e teste manual end-to-end
**Status:** ✅ Concluído

**Execução real:** os 3 cenários da exceção doc-only testados manualmente num clone
descartável em `/tmp` (não a branch real, para não sujar o histórico): (1) branch
`docs/<slug>` com só `.md` staged → `Governance: skipped (doc-only change)`, permite; (2)
mesma branch + 1 arquivo `.go` staged junto → volta a bloquear (`does not match the
required pattern feat|fix|refactor/<slug>`); (3) branch `feat/<slug>` sem roadmap em
`wip/`, só `.md` staged → permite. Corpo de PR testado direto nesta branch real
(`--dry-run`, arquivo de teste descartado antes do commit real): 11 commits agregados em
`## Commits`/`## Detalhes`, título correto — confirma que o problema original do PR #169
está resolvido pelo próprio mecanismo.
**Arquivos afetados:** nenhum novo
**Ações:**
1. Rodar `make quality` na raiz.
2. Teste manual: criar uma branch de teste descartável fora do padrão feat/fix/refactor,
   staged só com um arquivo `.md` de teste, rodar `trackfw ship --dry-run -m "test"` e
   confirmar que passa (não bloqueia); adicionar um arquivo `.go`/`.js`/`.py` staged
   junto e confirmar que volta a bloquear.
3. Teste manual: numa branch com histórico real de múltiplos commits (pode reusar o
   padrão de teste do roadmap `git-branch-guard`), confirmar que o corpo do PR gerado
   por `--dry-run` lista os commits, não só "Created by trackfw ship."
4. Descartar qualquer branch/roadmap de teste criado no passo 2-3.
**Critérios de aceite:**
- [ ] `make quality` verde
- [ ] os 2 testes manuais acima confirmados
**Comandos de validação:** `make quality`

---
status: done
date: 2026-07-24
req: "docs/req/REQ-2026-07-24-corrige-resolve-de-integrations-em-windows-destinos-validos-rejeitados.md"
squad: "Apolo"
---

# Roadmap: Fix Windows path resolve em integrations (Node+Go) e guard de regressao

> Created: 2026-07-24 | Status: ✅ Done

## Diagnóstico / Contexto

**Sintoma reportado (usuário, Windows, npm CLI v2.15.0):**
`trackfw agents list` e `trackfw agents install` abortam com
`Error: Unsafe destination: .amazonq/cli-agents/trackfw-architect.json`
(e `.claude/agents/trackfw-architect.md`).

**Causa raiz confirmada (análise estática):**
O catálogo de integrações declara destinos com **barra POSIX** (`/`), e a função
`resolve()` valida "destino já normalizado" comparando o input contra a normalização
**dependente de plataforma**:

- **Node.js** — `npm/src/integrations/manager.js:31`:
  `path.normalize(destination) !== destination`
  No Windows, `path.normalize('.claude/agents/x.md')` → `.claude\agents\x.md` (backslash),
  que **difere** do input com `/`. A condição dispara e lança `Unsafe destination`.
  Em Linux/macOS `path.normalize === path.posix.normalize`, então o bug é **invisível** no dev/CI atual.

- **Go** — `internal/integrations/manager.go:398`:
  `filepath.Clean(destination) != destination` — **mesmo defeito** no Windows
  (`filepath.Clean` usa separador nativo). Não foi o que o usuário disparou (ele usa npm),
  mas viola paridade e falharia igual no Windows.
  Argumento de consistência: `internal/integrations/catalog.go:282,298` **já usa `path.Clean`** (POSIX);
  o `manager.go` regrediu dessa convenção — alinhar é restaurar consistência, não decisão de design.

- **Python** — `pypi/trackfw/integrations/manager.py:47`:
  `".." in Path(raw).parts` — verificação **semântica**, não string-equality-após-normalize.
  Já é **cross-platform correto**. É a implementação de referência. **Sem mudança de lógica.**

**Escopo (fechado, §11 — bug concreto, sem expansão):** apenas `resolve()` de integrations
nos 3 CLIs + teste de paridade + guard de regressão de CI. Não tocar outra lógica de path.

**Verdade sobre o guard:** o CI atual (`.github/workflows/quality.yml`) é **100% `ubuntu-latest`**.
Um teste `resolve('.claude/agents/x.md')` passa em Linux **antes e depois** do fix — não é guard real.
O único guard honesto é um job **`windows-latest`** rodando os testes de integrations (ML-1D).

## Wave 1 — Fix + testes (paralelizável por árvore de arquivos; entregue por Apolo como unidade coerente de paridade)
> Dependências: nenhuma. Node/Go/Python/CI tocam árvores distintas, mas formam uma única correção de paridade — entregar atômico.

### ML-1A — Fix Node.js resolve() (POSIX normalize)
**Status:** ✅ Concluído
**Arquivos afetados:** `npm/src/integrations/manager.js`
**Ações:**
- Linha 31: trocar `path.normalize(destination)` por `path.posix.normalize(destination)`.
- Na mesma linha, trocar `` destination.startsWith(`..${path.sep}`) `` por `destination.startsWith('../')`
  (o destino é garantidamente forward-slash — backslash já é rejeitado na linha 23).
- **Não alterar** as linhas 34-35, `assertNoSymlinks`, `rootFor`, `cleanEmpty`: operam sobre
  paths já resolvidos por `path.resolve`/`path.relative` (separador nativo) — `path.sep` está correto lá.
**Critérios de aceite:**
- [ ] `resolve('project', '.claude/agents/x.md')` retorna caminho resolvido (não lança) — simulável validando via `path.posix`.
- [ ] Traversal ainda rejeitado: `'../x'`, `'a/../../x'`, `'.'`, `'./x'` lançam `Unsafe destination`.
- [ ] `node --test tests/*.test.js` verde.

### ML-1B — Fix Go resolve() (path.Clean POSIX)
**Status:** ✅ Concluído
**Arquivos afetados:** `internal/integrations/manager.go`
**Ações:**
- Linha 398: trocar `filepath.Clean(destination) != destination` por `path.Clean(destination) != destination`
  e `strings.HasPrefix(destination, ".."+string(filepath.Separator))` por `strings.HasPrefix(destination, "../")`.
- Garantir import do pacote `path` (POSIX) além de `path/filepath`. `catalog.go` já importa `path` — seguir o mesmo padrão.
- **Não alterar** `beneath`, `rejectSymlinks`, `filepath.Join(root, destination)` (linha 401): corretos com separador nativo.
**Critérios de aceite:**
- [ ] `go build ./...` sem erros.
- [ ] `go test ./internal/integrations/...` verde.
- [ ] Novo teste table-driven em `manager_test.go` cobrindo aceitação de destino aninhado forward-slash e rejeição de traversal.

### ML-1C — Testes de paridade (Node + Go + Python-referência)
**Status:** ✅ Concluído
**Arquivos afetados:** `npm/tests/` (novo `integrations_resolve.test.js` ou anexar a `agents-skills.test.js`), `internal/integrations/manager_test.go`, `pypi/tests/` (novo `test_integrations_resolve.py`)
**Ações:**
- Casos de aceitação: `.claude/agents/trackfw-architect.md`, `.amazonq/cli-agents/trackfw-architect.json` → resolvem sem erro.
- Casos de rejeição (idênticos nos 3): `..`, `../x`, `a/../../x`, `.`, `./x`, string vazia, path com `\x00`.
- Python: teste **trava a referência** (comportamento já correto) para impedir regressão futura.
**Critérios de aceite:**
- [ ] 3 suites verdes localmente.
- [ ] Conjunto de casos de aceite/rejeição idêntico entre os 3 CLIs (paridade).

### ML-1D — Guard de regressão real: job Windows no CI
**Status:** ✅ Concluído
**Arquivos afetados:** `.github/workflows/quality.yml`
**Ações:**
- Adicionar job(s) `runs-on: windows-latest` executando os testes de integrations dos 3 CLIs
  (mínimo: Node `node --test`, Go `go test ./internal/integrations/...`, Python `pytest tests/test_integrations_resolve.py`).
- Objetivo explícito: sem este job, os testes do ML-1C passam em Linux e mascaram a regressão do Windows.
**Critérios de aceite:**
- [ ] Workflow válido (yaml lint / actionlint se disponível).
- [ ] Job Windows verde no PR (evidência de que o fix funciona sob semântica win32).

## Wave 2 — Release (barrier: Wave 1 completa e auditada)
> Dependências: Wave 1 mergeada. O usuário está bloqueado no **pacote npm publicado** — o fix só é entregue com nova versão.

### ML-2A — Bump de versão + release (patch)
**Status:** ✅ Concluído — bump 2.15.0 → 2.15.1 nos 5 arquivos de versão; `make quality` verde; tag `v2.15.1` na branch `chore/release-v2.15.1`.
**Arquivos afetados:** `npm/package.json`, `pypi/pyproject.toml`, `internal/version/version.go` (e demais arquivos de versão do protocolo de release)
**Ações:** seguir o **Protocolo de Release** do CLAUDE.md do projeto — bump patch `2.15.0 → 2.15.1`,
changelog agrupado (Fixes: Windows path resolve em integrations), tag anotada após merge.
**Critérios de aceite:**
- [ ] Versão consistente nos 3 CLIs.
- [ ] `make quality` verde.
- [ ] Publicação npm/pypi executada (decisão de disparo é do usuário).

## Validação global
```bash
make quality                       # Go + Node + Python + contratos de paridade
node --test npm/tests/*.test.js
go test ./internal/integrations/...
cd pypi && python -m pytest tests/test_integrations_resolve.py
trackfw validate
```

---
status: wip
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md"
squad: "apolo-tf"
---

# Roadmap: comando trackfw branch new para bloquear criação de branch sem REQ+roadmap em wip

> Created: 2026-08-04 | Status: wip

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md`

Hoje `branch_has_wip_roadmap` só pega o problema quando alguém roda `trackfw validate`/`ship` — depois
que a branch já existe. `trackfw branch new` move esse gate para antes da criação da branch,
reutilizando a mesma lógica de matching de slug já implementada e testada em
`internal/validator/validator.go` (`normalizeBranchSlug` + varredura de `wip/`/`done/`), sem duplicá-la.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] `trackfw branch new <type>/<slug>` implementado nos 3 CLIs com contrato idêntico ao REQ
- [ ] Lógica de matching compartilhada com o validador (extraída, não duplicada) — Go é a referência
      comportamental (`docs/cli-parity.md`: "Go is the behavioral reference")
- [ ] `docs/cli-parity.md` documenta o novo comando
- [ ] Testes replicados nos 3 runtimes cobrindo match em wip/, match em done/, sem match (bloqueia),
      `--dry-run`, tipo inválido, branch já existente
- [ ] `make quality` (Go/Node/Python/paridade) verde

## Wave 1 — Go: extrair matching + implementar comando (referência comportamental)
> Dependencies: none

### ML-1A — Extrair matching de slug do validador para função reutilizável
**Status:** ✅ Concluído
**Files affected:**
- `internal/validator/validator.go` (extrair `branchSlugMatchesRoadmap(slug string, wipDirs, doneDirs []string) (matched bool, candidates []string)` a partir do corpo de `validateBranchHasWIPRoadmap`, linhas ~1926-1944)
**Actions:**
1. Extrair a extração de `wipDirs`/`doneDirs` + o laço de matching (`normalizeBranchSlug` + `strings.Contains`) para uma função exportada ou de pacote reutilizável pelo novo comando `branch`.
2. `validateBranchHasWIPRoadmap` passa a chamar essa função — comportamento observável idêntico (nenhuma mensagem muda).
**Acceptance criteria:**
- [x] `go build ./...` sem erros
- [x] `go test ./internal/validator/...` verde, sem alterar nenhuma asserção existente (refactor puro)

### ML-1B — Implementar `trackfw branch new` em Go
**Status:** ✅ Concluído
**Files affected:**
- `internal/commands/branch.go` (novo)
- `internal/commands/root.go` (registrar subcomando)
**Actions:**
1. Novo comando cobra `branch new <type>/<slug>` — valida `type` ∈ `{feat, fix, refactor}` (mesmo
   vocabulário de `trackfw ship` passo 1); slug vazio é erro de uso.
2. Usa a função extraída no ML-1A para checar match contra `wip/`+`done/`.
3. Sem match: imprime a mesma mensagem de orientação de `validateBranchHasWIPRoadmap` (reutilizar a
   string, não duplicar), exit não-zero, **não chama `git checkout -b`**.
4. Com match: executa `git checkout -b <type>/<slug>` via `exec.Command`, propaga stdout/stderr/exit
   code do Git literalmente (não reformatar a saída do Git).
5. `--dry-run`: roda a checagem de match e imprime o resultado ("would create" / "would block: <motivo>"), nunca chama `git checkout`.
**Acceptance criteria:**
- [x] `go build ./...` sem erros
- [x] Testes cobrindo: match em wip/, match em done/, sem match, `--dry-run` (ambos os casos), tipo
      inválido, branch já existente (delega ao erro do Git)
- [x] `trackfw help branch` funcional

> Auditoria manual (trackfw_architect): testei o binário real ponta a ponta (não só os testes
> unitários) — `branch new --dry-run` sem match bloqueia sem tocar no git; sem `--dry-run` bloqueia
> igual; tipo inválido rejeitado; com match real (slug desta própria REQ) reporta "would create"
> corretamente. `go test ./internal/...` completo (não só os pacotes tocados) roda verde.

## Wave 2 — Node.js + Python (paralelo entre si, dependem da Wave 1 como referência de contrato)
> Dependencies: Wave 1 completa (comportamento Go é a fonte da verdade)

### ML-2A — Implementar `trackfw branch new` em Node.js
**Status:** pending
**Files affected:**
- `npm/src/commands/branch.js` (novo)
- `npm/src/cli.js` ou equivalente (registrar comando)
- Função de matching equivalente ao ML-1A, extraída do validador Node existente
**Actions:** Espelhar exatamente o contrato validado em Go (ML-1B): mesmos flags, mesmas mensagens, mesmo exit code, mesma decisão de dry-run.
**Acceptance criteria:**
- [ ] `npm test` verde com os mesmos cenários do ML-1B
- [ ] Mensagens de erro/orientação byte-idênticas às do Go

### ML-2B — Implementar `trackfw branch new` em Python
**Status:** pending
**Files affected:**
- `pypi/trackfw/commands/branch.py` (novo)
- `pypi/trackfw/cli.py` ou equivalente (registrar comando)
- Função de matching equivalente, extraída do validador Python existente
**Actions:** Espelhar exatamente o contrato validado em Go (ML-1B).
**Acceptance criteria:**
- [ ] `python3 -m pytest` verde com os mesmos cenários do ML-1B
- [ ] Mensagens de erro/orientação byte-idênticas às do Go

## Wave 3 — Documentação e gate de paridade
> Dependencies: Wave 2 completa

### ML-3A — Documentar e cobrir com gate de paridade
**Status:** pending
**Files affected:**
- `docs/cli-parity.md`
- `scripts/check-cli-parity.sh` (ou script de paridade equivalente)
**Actions:**
1. Adicionar `branch` à tabela de comandos em `docs/cli-parity.md` e uma seção descrevendo o contrato (espelhando o estilo das seções `trackfw ship`/`trackfw barrier`).
2. Adicionar cenário do novo comando ao gate de paridade existente.
**Acceptance criteria:**
- [ ] `make quality` verde
- [ ] `trackfw validate` sem violações

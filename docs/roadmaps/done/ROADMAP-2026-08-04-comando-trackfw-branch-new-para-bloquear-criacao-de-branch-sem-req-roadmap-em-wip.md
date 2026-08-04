---
status: done
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md"
squad: "apolo-tf"
---

# Roadmap: comando trackfw branch new para bloquear criação de branch sem REQ+roadmap em wip

> Created: 2026-08-04 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip.md`

Hoje `branch_has_wip_roadmap` só pega o problema quando alguém roda `trackfw validate`/`ship` — depois
que a branch já existe. `trackfw branch new` move esse gate para antes da criação da branch,
reutilizando a mesma lógica de matching de slug já implementada e testada em
`internal/validator/validator.go` (`normalizeBranchSlug` + varredura de `wip/`/`done/`), sem duplicá-la.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] `trackfw branch new <type>/<slug>` implementado nos 3 CLIs com contrato idêntico ao REQ
- [x] Lógica de matching compartilhada com o validador (extraída, não duplicada) — Go é a referência
      comportamental (`docs/cli-parity.md`: "Go is the behavioral reference")
- [x] `docs/cli-parity.md` documenta o novo comando
- [x] Testes replicados nos 3 runtimes cobrindo match em wip/, match em done/, sem match (bloqueia),
      `--dry-run`, tipo inválido, branch já existente
- [x] `make quality` (Go/Node/Python/paridade) verde

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
**Status:** ✅ Concluído
**Files affected:**
- `npm/src/commands/branch.js` (novo)
- `npm/src/cli.js` ou equivalente (registrar comando)
- Função de matching equivalente ao ML-1A, extraída do validador Node existente
**Actions:** Espelhar exatamente o contrato validado em Go (ML-1B): mesmos flags, mesmas mensagens, mesmo exit code, mesma decisão de dry-run.
**Acceptance criteria:**
- [x] `npm test` verde com os mesmos cenários do ML-1B (374/374)
- [x] Mensagens de erro/orientação byte-idênticas às do Go

> Auditoria manual (trackfw_architect) encontrou um bug real de exit code no cenário "branch já
> existe" — o único caminho que exercita o `defaultGitCheckout` de produção de ponta a ponta (os
> testes unitários injetam fake e nunca o exercitam). Go vazava `"exit status 128"` como linha
> extra (nunca produzida pelo git) por causa de `Execute()` sempre imprimir o erro retornado,
> independente de `SilenceErrors`; Node "propagava" um exit code fixo (`1`) em vez do código real
> do git. Python já estava correto (`return result.returncode`). Corrigido nos dois: Go agora sai
> com `os.Exit(exitErr.ExitCode())` diretamente sem devolver erro pro cobra imprimir; Node mudou
> `defaultGitCheckout` para retornar o exit code numérico real em vez de `Error|null`. Confirmado
> empiricamente pós-fix: os três binários reais produzem `exit=128` idêntico para esse cenário.
> Detalhe completo em `vault/notes/branch-new-exit-code-leak-vs-propagation-2026-08-04.md`.

### ML-2B — Implementar `trackfw branch new` em Python
**Status:** ✅ Concluído
**Files affected:**
- `pypi/trackfw/commands/branch.py` (novo)
- `pypi/trackfw/cli.py` ou equivalente (registrar comando)
- Função de matching equivalente, extraída do validador Python existente
**Actions:** Espelhar exatamente o contrato validado em Go (ML-1B).
**Acceptance criteria:**
- [x] `python3 -m pytest` verde com os mesmos cenários do ML-1B (890 passed, 8 subtests)
- [x] Mensagens de erro/orientação byte-idênticas às do Go — já corretas desde a primeira versão,
      inclusive o exit code real do git (achado acima)

## Wave 3 — Documentação e gate de paridade
> Dependencies: Wave 2 completa

### ML-3A — Documentar e cobrir com gate de paridade
**Status:** ✅ Concluído
**Files affected:**
- `docs/cli-parity.md`
- `scripts/check-branch-new-parity.sh` (novo — dedicado, seguindo o padrão de `check-roadmap-move-parity.sh`)
- `Makefile` (wired ao alvo `parity`, antes de `check-gates-falsify.sh`)
- `scripts/check-gates-falsify.sh` (cenário 42, prova de falsificabilidade P4)
**Actions:**
1. Adicionar `branch` à tabela de comandos em `docs/cli-parity.md` e uma seção completa descrevendo
   o contrato, incluindo o achado de exit code da Wave 2 (documentado com histórico).
2. Gate dedicado com 3 cenários (sem match / match + dry-run / match + git real com branch já
   existente — este último é o único que exercita o wrapper de produção `defaultGitCheckout`, não
   só o fake injetado pelos testes unitários), cada um comparando stdout+stderr+exit-code
   byte-a-byte entre os 3 runtimes, com guarda de vacuidade.
3. Cenário de falsificação (P4): corrompe a mensagem de stderr do Node.js, confirma que o gate
   detecta e falha com o rótulo exato esperado.
**Acceptance criteria:**
- [x] `make quality` verde — build/vet Go, `check-branch-new-parity.sh` (3/3), `check-gates-falsify.sh`
      completo (100/100 cenários, incluindo o cenário 42 novo) — tudo reexecutado e confirmado
      manualmente pelo orquestrador, não só relatado pelo agente
- [x] `trackfw validate` sem violações

> Achado fora de escopo (não corrigido aqui, reportado): `scripts/check-identity-parity.sh` falha
> pré-existente por HTML-escaping assimétrico do `encoding/json` do Go (`<slug>` → `<slug>`)
> em 3 targets (kiro/cli, amazonq/cli, antigravity/legacy-cli), todos usando a representação
> `agent-json`/`cli-agent-json` (`internal/integrations/render.go:57`, um único `json.MarshalIndent`
> sem `SetEscapeHTML(false)`). Reproduzido e causa raiz confirmada de forma independente pelo
> orquestrador. **Não está no CI** — `.github/workflows/quality.yml` só roda `check-cli-parity.sh` e
> `check-validate-parity.sh` no job `parity`, não `make parity` completo — então isso nunca bloqueou
> PR nenhuma, só `make quality` local. Tratado como REQ própria, fora deste roadmap.
> Nota: `vault/notes/check-identity-parity-json-html-escaping-pre-existing-2026-08-04.md`.

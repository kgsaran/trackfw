---
status: wip
date: 2026-10-23
req: "docs/req/REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: update não remove gerações antigas do credential guard em pares evento×matcher

> Created: 2026-10-23 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md
ADR: ADR-2026-10-04, adendo de 2026-10-23 (D12–D14). Issue: #599.

Causa (lida no código pelo arquiteto): `InjectCursorHooks` migra as formas legadas só em `beforeShellExecution` e
`afterShellExecution`. Os pares `preToolUse`/`postToolUse` × `Read`/`Write` recebem apenas `mergeCursorGuardMatcherEntry`,
que acrescenta a forma atual e deixa as legadas. A lista de gerações está copiada à mão em cada sítio, e as cópias
divergem entre os CLIs.

## Acceptance Criteria
- [ ] AC1–AC8 da REQ, cada um com a evidência registrada no ML que o fecha.
- [ ] `make quality` e o CI ficam verdes.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model e matriz da mesma causa
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model e matriz CLI × escopo × evento × matcher × geração
**Status:** ✅ Concluído
**Agente:** hades-tf
**Files affected:** `docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md` (novo; o único arquivo que o ML escreve)
**Actions:**
1. **Matriz medida.** Para cada sítio que emite um guard (credential e git-branch), em todo `Inject*Hooks` de
   `internal/generators/agentfiles.go` e em todo alvo `harness*Target*` de `internal/generators/update.go`: quais pares
   (evento, matcher) ele emite e quais gerações ele migra em cada par. Compare com o que o validador reconhece como legado
   (`internal/validator/validator_credential_guard.go`, `validator_git_branch_guard.go`, `validator_guard_wiring.go`,
   `validator_guard_binary_probe.go`). Marque cada célula como: migra e deduplica / migra e duplica / não migra.
   A medição é por teste Go em `t.TempDir()` (fixture com a geração → rodar o sítio real → contar as entradas), nunca no
   checkout. Um teste de medição descartável pode existir só durante a medição e não é entregue.
2. **Completude da enumeração.** Mostre que a lista de sítios está fechada, com o grep pelos literais das gerações e por
   `migrateHookCommand|migrateCursorSimpleCommand|merge.*Hook` em toda a árvore, fora de `testdata/`.
3. **Threat model.** O que a remoção abre: uma regra de remoção que case hook do usuário (comando que contém
   `trackfw guard credential`, mas não é igual a uma geração); a D14 removendo a entrada de projeto quando a detecção do
   global erra; uma geração nova acrescentada sem que algum sítio a migre.
4. **Alvos de falsificação nos dois sentidos**, por célula: o teste que reprova se a legada ficar, e o que reprova se um
   hook do usuário sumir.
5. **Resíduo declarado.**
**Acceptance criteria:**
- [x] A matriz com cada célula medida (comando e resultado), não inferida
- [x] As quatro seções acima respondidas com evidência
- [x] Nenhuma linha de produto escrita
**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md
grep -q 'Matriz' docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md
git diff --quiet origin/main -- internal/
```

### ML-0B — Corretivo da auditoria: medir os CLIs que faltaram e o guard de git-branch
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Por que existe:** a auditoria da ML-0A achou três problemas. A matriz §1.3 só mediu o Cursor: as afirmações sobre
Claude, Codex, Gemini, Windsurf e Amazon Q ("migra e duplica", "não migra") não têm cenário medido. O R5 tirou o guard de
git-branch do escopo, mas o AC2 da REQ o inclui, e a Regra Dura manda medir. E a D14 foi revista por par, depois do T4/R2.
**Files affected:** `docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md` (acréscimo de seção)
**Acceptance criteria:**
- [ ] Matriz medida, com cenário e contagem, para Claude, Codex, Gemini, Windsurf e Amazon Q, nos estados só-legadas,
  N-legadas e legada+atual, em cada par (evento, matcher) que o sítio emite
- [ ] Mesma matriz para o guard de git-branch, em todo sítio de projeto e global que o emite
- [ ] Para cada CLI, quais pares o guard global cobre, que é o insumo da D14 revista
- [ ] R5 reescrito, sem tirar o git-branch do escopo
- [ ] Nenhuma linha de produto (`git diff --quiet origin/main -- internal/` rc=0)
**Gates da wave:**
```bash
grep -q 'ML-0B' docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md
git diff --quiet origin/main -- internal/
```

## Wave 1 — Registro único, migração com deduplicação e todos os sítios da matriz
> Dependencies: Wave 0 auditada pelo arquiteto.

### ML-1A — Registro de gerações, Cursor e os demais sítios da matriz
**Status:** ⬜ Pendente
**Agente:** apolo-tf
**Files affected:** `internal/generators/agentfiles.go`, `internal/generators/update.go`, os arquivos de validador que a
Wave 0 apontar, e os testes correspondentes. A lista exata sai da matriz da Wave 0 e é escrita aqui antes do despacho.
**Actions:** implementar D12 (registro único), D13 (migrar → deduplicar → mesclar, uma entrada por par) e D14 (remover com
o global instalado) em toda célula marcada pela Wave 0.
**Acceptance criteria:**
- [ ] AC1, AC2, AC3, AC4, AC5, AC6 e AC7 da REQ, cada um com o nome do teste que o fecha e a frase do que ele afirma
- [ ] Cada teste novo reprova no código de `origin/main` (contra-braço medido)
- [ ] `go build ./...`, `go test ./internal/generators/ ./internal/validator/ ./internal/pathguard/` verdes
**Gates da wave:**
```bash
go build ./...
go test ./internal/generators/ ./internal/validator/ ./internal/pathguard/
```

## Wave 2 — Revisões independentes
> Dependencies: Wave 1 auditada.

### ML-2A — Revisão de qualidade
**Status:** ⬜ Pendente
**Agente:** hefesto-tf
**Acceptance criteria:**
- [ ] Parecer sem bloqueio, ou bloqueios corrigidos por ML corretivo

### ML-2B — Red-team da remoção
**Status:** ⬜ Pendente
**Agente:** hades-tf
**Acceptance criteria:**
- [ ] Reimplementação independente da regra de remoção a partir da leitura; nenhum hook do usuário removido

## Wave 3 — Fechamento
> Dependencies: Wave 2 aprovada.

### ML-3A — make quality e CI
**Status:** ⬜ Pendente
**Agente:** zeus-tf
**Acceptance criteria:**
- [ ] `make quality` rc=0 com a máquina ociosa e o HOME isolado
- [ ] CI do PR verde; PR com `Closes #599`

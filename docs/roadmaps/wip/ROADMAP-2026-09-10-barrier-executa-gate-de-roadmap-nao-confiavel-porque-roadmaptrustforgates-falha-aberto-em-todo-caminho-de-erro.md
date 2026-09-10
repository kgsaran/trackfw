---
status: wip
date: 2026-09-10
req: "docs/req/REQ-2026-08-30-barrier-executa-gate-de-roadmap-nao-confiavel-porque-roadmaptrustforgates-falha-aberto-em-todo-caminho-de-erro.md"
squad: ""
---

# Roadmap: `barrier` executa gate de roadmap não confiável porque `roadmapTrustForGates` falha aberto em todo caminho de erro

> Created: 2026-09-10 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-08-30-barrier-executa-gate-de-roadmap-nao-confiavel-porque-roadmaptrustforgates-falha-aberto-em-todo-caminho-de-erro.md -->
REQ: docs/req/REQ-2026-08-30-barrier-executa-gate-de-roadmap-nao-confiavel-porque-roadmaptrustforgates-falha-aberto-em-todo-caminho-de-erro.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] AC1–AC7: postura fail-closed implementada nos 3 CLIs — Go, Node.js, Python
- [x] AC8: build green, todos os testes passam, falsificação nas duas direções confirmada

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none

### ML-1A — **AC1** — Postura invertida: **fecha por padrão**, abre só quando conseguir **provar** que o
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — Postura invertida: **fecha por padrão**, abre só quando conseguir **provar** que o
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — Cada condição hoje fail-open passa a not_evaluated com **razão nomeada**: sem
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — Cada condição hoje fail-open passa a not_evaluated com **razão nomeada**: sem
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — A distinção não pode depender de casamento de substring de mensagem do git. Use
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — A distinção não pode depender de casamento de substring de mensagem do git. Use
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — --trust-local-gates continua sendo a saída explícita e auditável, e é a única.
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — --trust-local-gates continua sendo a saída explícita e auditável, e é a única.
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — Falsificação nas duas direções: roadmap idêntico a origin/main → gates **executam**;
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — Falsificação nas duas direções: roadmap idêntico a origin/main → gates **executam**;
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — Paridade nos 3 CLIs; gate falsificável cobrindo as condições de AC2.
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — Paridade nos 3 CLIs; gate falsificável cobrindo as condições de AC2.
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7** — Não quebrar o fluxo legítimo do arquiteto neste repositório, que usa
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7** — Não quebrar o fluxo legítimo do arquiteto neste repositório, que usa
- [ ] build passes
- [ ] tests green

### ML-1H — **AC8** — make quality exit 0 **e CI verde**.
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC8** — make quality exit 0 **e CI verde**.
- [ ] build passes
- [ ] tests green

## Wave 2 — Corretivos do parecer hades-tf (2026-09-10)
> Dependencies: Wave 1 completa

### ML-2A — F1: prova cobre o mesmo buffer que executa
**Status:** ✅ Concluído
**Files affected:** `internal/commands/barrier.go`, `npm/src/commands/barrier.js`, `pypi/trackfw/commands/barrier.py`, `internal/commands/barrier_test.go`, `npm/tests/barrier.test.js`, `pypi/tests/test_barrier.py`
**Actions:**
1. Go: `roadmapTrustForGates(path string, localContent []byte)` — remove Step 7 re-read; usar o buffer passado; call site em `runBarrier` passa `data`
2. Node: `roadmapTrustForGates(path, localContentBuf)` — Step 7 usa buffer passado; Step 6 spawnSync sem encoding → Buffer; comparação com `.equals()`; `runBarrier` lê Buffer e passa à função
3. Python: `_roadmap_trust_for_gates(path, local_content: bytes)` — Step 7 usa bytes passados; `_build_result_document` lê como bytes, decodifica para parsing
4. Remove o caminho de erro "cannot read local roadmap file" (agora inalcançável nos 3 CLIs)
5. Testes: atualizar call sites das funções de confiança; adicionar teste TOCTOU (TestRoadmapTrustForGates_VerifiesPassedBuffer)
**Acceptance criteria:**
- [x] F1: gates parseados apenas de buffer verificado nos 3 CLIs
- [x] F1: teste TOCTOU que falha se alguém voltar a parsear de buffer não verificado
- [x] build passes
- [x] tests green

### ML-2B — F2: defaults fail-open removidos (Node e Python)
**Status:** ✅ Concluído
**Files affected:** `npm/src/commands/barrier.js`, `pypi/trackfw/commands/barrier.py`, `npm/tests/barrier.test.js`
**Actions:**
1. Node: `evalGates(commands, cwd, trustResult = { trusted: false })` — default seguro
2. Python `_check_gates`: remover `if trust_result is None: trust_result = {"trusted": True}`; trocar `get("trusted", True)` por `get("trusted", False)`
3. Node tests: adicionar `{ trusted: true }` explícito nas 4 chamadas de `evalGates` com 2 argumentos
**Acceptance criteria:**
- [x] F2: default de segurança `false` nos 3 CLIs
- [x] testes de call site passam
- [x] build passes

### ML-2C — F3: guarda comportamental por razão nomeada (3 CLIs)
**Status:** ✅ Concluído
**Files affected:** `internal/commands/barrier_test.go`, `npm/tests/barrier.test.js`, `pypi/tests/test_barrier.py`
**Actions:**
1. Go: 4 testes comportamentais com sentinel por razão de `not_evaluated` (not-git-repo, no-remote, not-committed, content-differs) — cada um confirma ausência do sentinel
2. Node.js: 4 testes equivalentes
3. Python: 4 testes equivalentes
**Acceptance criteria:**
- [x] F3: guarda comportamental por razão nomeada nos 3 CLIs, ausência de sentinela confirmada
- [x] testes passam

### ML-2D — F4: comparação binária no Node
**Status:** ✅ Concluído (coberto pelo ML-2A, Step 6)
**Files affected:** `npm/src/commands/barrier.js`

### ML-2E — F5: mensagens separadas por causa (git not found in PATH)
**Status:** ✅ Concluído
**Files affected:** `internal/commands/barrier.go`, `npm/src/commands/barrier.js`, `pypi/trackfw/commands/barrier.py`, `docs/cli-parity.md`
**Actions:**
1. Go Step 1: distinguir `*exec.ExitError` (git nonzero) de spawn failure → "git not found in PATH"
2. Node Step 1: `revParse.error` → nova mensagem; `revParse.status !== 0` → mensagem existente
3. Python Step 1: `FileNotFoundError` → nova mensagem "git not found in PATH"
4. `docs/cli-parity.md`: substituir "Local roadmap file cannot be read" (removido F1) por "git binary not found in PATH"; atualizar pinned strings correspondentes
5. cat-file Step 5 triple: declarar não-separável em comentário de código (3 CLIs)
**Acceptance criteria:**
- [x] F5: mensagens distintas para git-ausente vs não-em-repo nos 3 CLIs
- [x] cli-parity.md atualizado (8 strings: 1 removida + 1 adicionada = 8)
- [x] build passes

### ML-2F — F6: comentário obsoleto em check-barrier.sh
**Status:** ✅ Concluído
**Files affected:** `scripts/check-barrier.sh`
**Actions:**
1. Linha ~941: "barrier fails-open (trusted)" → "barrier fails-closed (not_evaluated)"
**Acceptance criteria:**
- [x] F6: comentário corrigido

### ML-W3A — Resíduos do parecer de segurança: as duas metades do F1
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` · **não bloqueia o merge** (parecer: **APROVA**)

O `hades-tf` verificou o fechamento dos próprios achados e aprovou. Sobraram **dois resíduos do F1**,
que ele separou em metades — e a separação é o valor do achado:

**Metade do CALLEE — cobertura de paridade.** `TestRoadmapTrustForGates_VerifiesPassedBuffer` existe
**só no Go**. Node e Python não têm equivalente. É o **F3 repetido dentro da correção do F1**: a
guarda que prova a correção não tem paridade, mesmo com o código tendo.

**Portar para Node e Python.**

**Metade do CALLER — 🔴 não verificável por teste comportamental.** O invariante *"o buffer passado ao
verificador é o buffer consumido pelo parser de gates"* **não é falsificável por fixture**: se o
caller fizer duas leituras sem escritor concorrente, as duas devolvem bytes idênticos e **nenhum teste
determinístico distingue**.

Fechável **só estruturalmente** — guarda estática ou de shell contra `readFile`/`open` duplicado no
caller. **Ausente nos 3 CLIs hoje.**

🔴 **Este é o caso raro em que a guarda estrutural NÃO é o remédio fraco.** O projeto vinha
preferindo guarda comportamental à estrutural (recomendação do próprio `hades-tf` no F3), e está
certo — mas aqui o comportamento **não discrimina por construção**. Quando o observável não separa os
estados, a estrutura é o único lugar onde a separação existe.

**Também residual, declarado e julgado pelo parecer:**

- **F2 PARCIAL** — o análogo em Node (`evalGates` sem `failureMsg` ⇒ `failures: [undefined]`) e o
  `KeyError` do Python são **código morto**: nenhum call site omite o argumento. Fail-closed em
  produção; dívida de docstring.
- **Step 5 `cat-file`** — três causas, um `returncode`, **não separável** sem reescrever o protocolo
  com o subprocesso. Aceito como limitação declarada, com comentário no código dos 3 CLIs.

#### Correção de número, registrada

Eu escrevi **"Go 8 guardas comportamentais"** no handoff de verificação, repetindo o relatório de
implementação **sem contar**. O `hades-tf` contou: `grep -c "^func TestRoadmapTrustForGates"` ⇒ **7**.
Os 7 passam. **Número repetido não é número medido** — é a mesma classe dos 4 `grep` errados desta
campanha.

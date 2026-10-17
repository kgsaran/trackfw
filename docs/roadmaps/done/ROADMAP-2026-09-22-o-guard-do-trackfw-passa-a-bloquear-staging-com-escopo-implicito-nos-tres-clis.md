---
status: done
date: 2026-09-22
req: "docs/req/REQ-2026-09-05-o-guard-do-trackfw-passa-a-bloquear-staging-com-escopo-implicito-nos-tres-clis.md"
squad: ""
---

# Roadmap: o guard do trackfw passa a bloquear staging com escopo implicito nos tres CLIs

> Created: 2026-09-22 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-09-05-o-guard-do-trackfw-passa-a-bloquear-staging-com-escopo-implicito-nos-tres-clis.md -->
REQ: docs/req/REQ-2026-09-05-o-guard-do-trackfw-passa-a-bloquear-staging-com-escopo-implicito-nos-tres-clis.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído — `docs/seguranca/2026-10-17-wave0-guard-staging-implicito.md`
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

### ML-1A — **AC1** — O guard bloqueia git add de escopo implícito: -A, --all, ., -u,
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — O guard bloqueia git add de escopo implícito: -A, --all, ., -u,
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — Staging por caminho explícito **continua passando**, inclusive múltiplos caminhos e
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — Staging por caminho explícito **continua passando**, inclusive múltiplos caminhos e
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **Controle de não-afrouxamento**, e é o critério principal: enumerar o que passou a
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **Controle de não-afrouxamento**, e é o critério principal: enumerar o que passou a
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — 🔴 **Guarda de vacuidade:** com a regra desligada, os cenários de bloqueio **falham**.
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — 🔴 **Guarda de vacuidade:** com a regra desligada, os cenários de bloqueio **falham**.
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — Mensagem de commit contendo o texto git add -A **não** é tratada como comando —
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — Mensagem de commit contendo o texto git add -A **não** é tratada como comando —
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — 🔴 **A mensagem ensina.** Mostra git status --short para enumerar e a forma válida.
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — 🔴 **A mensagem ensina.** Mostra git status --short para enumerar e a forma válida.
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7** — Os limites de tokenização ficam **declarados** (git${IFS}add, {git,add},
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7** — Os limites de tokenização ficam **declarados** (git${IFS}add, {git,add},
- [ ] build passes
- [ ] tests green

### ML-1H — **AC8** — 🔴 **O gate de paridade cobre a regra nova.** Os guards existentes têm cobertura
**Status:** ✅ Concluído (2026-10-17)
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC8** — 🔴 **O gate de paridade cobre a regra nova.** Os guards existentes têm cobertura
- [ ] build passes
- [ ] tests green

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

🔴 **Vivo** — prioridade 1: `git add -A` → exit 0 no `trackfw guard git-branch` (`git commit` → exit 2); a regra do ADR-2026-09-05 nunca entrou em `internal/guard/gitbranch.go`. AC de paridade nos 3 CLIs sumiu com a v8.

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.

---

## Decisões — 2026-10-17 (Wave 0 + usuário)

Wave 0: `docs/seguranca/2026-10-17-wave0-guard-staging-implicito.md` (hades-tf, aprova com ressalvas).

- **D-U1** `git add` sem operando → **bloqueia** (ADR D1).
- **D-U2 (usuário)** diretório como alvo (`git add docs/`, `git add -A docs/`) → **passa**: medido que,
  com pathspec, `-A` não alarga o escopo. Bloqueia só sem pathspec ou com pathspec que é a raiz
  inteira (`.`, `./`, `..`, `../`, `:/`, `:(top)`), inclusive depois de `--`.
- **D-U3** `git stage` (alias de `add`) → mesma regra.
- **D-U4** `:/` e `:(top)` → **bloqueiam**.
- **D-U5 (usuário)** **ligada por padrão**, como os demais guards; entra no CHANGELOG como mudança de
  comportamento.
- Interativos e simulação (`-p`, `-i`, `--patch`, `--interactive`, `-n`, `--dry-run`) → passam.
- `git commit -a`/`--all` já é bloqueado (o guard bloqueia `git commit` cru incondicionalmente).
- **D4/AC8 pós-v8:** uma implementação; a paridade vira contrato em `docs/cli-parity.md` + testes Go
  + cenários em `scripts/check-gates-falsify.sh`.

### ML-1I — Sítios de mesma causa achados na Wave 0
**Status:** ✅ Concluído (2026-10-17)
- **B:** `internal/generators/scaffold.go:~732` — o template de skill instrui `git add -A`; passa a
  instruir caminho explícito.
- **C:** `internal/generators/agentfiles.go:~1949` — denylist nativa do Amazon Q
  (`^git (commit|push|checkout -b)`) não cobre o staging implícito; estender.
- **R-6 (medir):** a razão do guard tem de ser linha única e segura dentro de JSON; o hades observou
  que `reasonResetHard` já sai inválida. Se for o mesmo mecanismo de emissão, corrige junto.

## Entrega — 2026-10-17

Regra em `internal/guard/gitbranch.go` (`add`/`stage`), ligada por padrão. Três rodadas de red-team
(`docs/seguranca/2026-10-17-red-team-guard-staging-implicito.md`) bloquearam B-1…B-6; a causa
recorrente (parênteses tratados por recorte de string) foi corrigida no tokenizador: `(`/`)` fora de
aspas separam, redirecionamentos e `#` saem dos tokens. Conferência pós-rodada 3: **libera**.

**Reconferido pelo arquiteto no binário:** 35 vetores de `add` corretos; 30 comandos não-add com
decisão idêntica ao binário da `origin/main`; invariante `(C)` = `( C )` = `C` em 11 comandos de todos
os guards. Teste `TestMatchSubcommand_RegressionAgainstMain` (38 comandos) fixa isso. Falsify: 272
cenários, exit 0.

**Resíduos declarados (`docs/cli-parity.md`):** `$(git add -A)` e crase; `git${IFS}add`, `{git,add}`,
alias do usuário; `git add '*'` (o shell expande antes); `:(icase)*`; `--pathspec-from-file`. A
conferência listou `(git add -A) > redir` como resíduo, mas o arquiteto mediu **exit 2** (bloqueia) —
não é resíduo.

**Mergeado** no #576 (`a32d16a7`), 2026-10-17.

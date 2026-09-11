---
status: wip
date: 2026-09-11
req: "docs/req/REQ-2026-09-11-install-sh-extrai-o-tarball-sem-conferir-o-checksums-txt-que-o-goreleaser-publica.md"
squad: ""
---

# Roadmap: install.sh extrai o tarball sem conferir o checksums.txt que o goreleaser publica

> Created: 2026-09-11 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-11-install-sh-extrai-o-tarball-sem-conferir-o-checksums-txt-que-o-goreleaser-publica.md -->
REQ: docs/req/REQ-2026-09-11-install-sh-extrai-o-tarball-sem-conferir-o-checksums-txt-que-o-goreleaser-publica.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pendente
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

### ML-1A — **AC1** — o instalador baixa o checksums.txt **da mesma tag**, valida o nome esperado e confere
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — o instalador baixa o checksums.txt **da mesma tag**, valida o nome esperado e confere
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — 🔴 **falha fechado** quando o checksum está **ausente**, **duplicado** ou **divergente**.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — 🔴 **falha fechado** quando o checksum está **ausente**, **duplicado** ou **divergente**.
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **Falsificação:** stub de download que troca **um byte** do tarball ⇒ **nenhum
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **Falsificação:** stub de download que troca **um byte** do tarball ⇒ **nenhum
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — 🔴 **Contra-braço:** tarball íntegro instala normalmente. Verificador que só recusa é
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — 🔴 **Contra-braço:** tarball íntegro instala normalmente. Verificador que só recusa é
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — funciona onde não há sha256sum (macOS usa shasum -a 256). 🔴 **Ausência da
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — funciona onde não há sha256sum (macOS usa shasum -a 256). 🔴 **Ausência da
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — gate que reprova se o install.sh voltar a extrair sem conferir.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — gate que reprova se o install.sh voltar a extrair sem conferir.
- [ ] build passes
- [ ] tests green

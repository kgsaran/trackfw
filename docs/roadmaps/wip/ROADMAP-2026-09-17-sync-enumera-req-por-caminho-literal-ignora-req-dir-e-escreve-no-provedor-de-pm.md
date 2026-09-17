---
status: wip
date: 2026-09-17
req: "docs/req/REQ-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md"
squad: ""
---

# Roadmap: sync enumera REQ por caminho literal ignora req_dir e escreve no provedor de PM

> Created: 2026-09-17 | Status: wip


## Wave 0 — Threat model
> Dependências: nenhuma. 🔴 **Auditada antes de qualquer wave de implementação.**

### ML-0A — o que um enumerador errado permite quando o consumidor escreve fora
**Status:** ⬜ Pendente · **Papel:** `hades-tf`

- O `sync` leva **conteúdo de REQ** para um provedor externo. Com o caminho errado, que conteúdo
  pode vazar para o projeto de PM errado — e o id injetado de volta marca arquivo alheio como
  sincronizado?
- Credenciais: `SyncToLinear`/`SyncToJira` usam token. Um `req_dir` mal resolvido muda o **alvo** da
  escrita autenticada?
- 🔴 A correção passa a ler `cfg.REQDir`, que vem do `trackfw.yaml` — arquivo do repositório. Um
  `req_dir` hostil (`../..`, caminho absoluto, symlink) pode fazer o `sync` **ler fora da árvore** e
  publicar isso? O resolvedor contém o caminho?
- O AC6 manda falar quando acha 0 REQ. Falar **o quê**? A mensagem pode revelar caminho de sistema?

**Aceite:** parecer com os vetores enumerados e, por vetor, se a correção fecha, mitiga ou não toca.
🔴 Vetor não fechado tem de estar nomeado.

---

## Wave 1 — Correção
> Dependências: **Wave 0 auditada.**

### ML-1A — resolver pelo ponto único, com falsificação em `req_dir` não-padrão e `by_agent`
**Status:** ⬜ Pendente · **Papel:** `apolo-tf`
Cobre AC1 a AC6.

---

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-09-17-sync-enumera-req-por-caminho-literal-ignora-req-dir-e-escreve-no-provedor-de-pm.md

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

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — sync enumera REQ por caminho literal ignora req_dir e escreve no provedor de PM
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes

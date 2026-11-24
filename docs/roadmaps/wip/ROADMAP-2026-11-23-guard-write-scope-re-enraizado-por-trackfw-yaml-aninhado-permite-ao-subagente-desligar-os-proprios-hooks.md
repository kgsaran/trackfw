---
status: wip
date: 2026-11-23
req: "docs/req/REQ-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: Guard write-scope re-enraizado por trackfw.yaml aninhado permite ao subagente desligar os próprios hooks

> Created: 2026-11-23 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md -->
REQ: docs/req/REQ-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md

Achado F-5 do red team da REQ-2026-11-22 (`docs/seguranca/2026-11-22-red-team-onboarding.md`, F-5). O guard
write-scope (`internal/guard/writescope.go`) resolve a raiz pelo primeiro ancestral do **alvo** que tem `trackfw.yaml`
(`findProjectRoot`, ~:504; Step 7, ~:165). O contrato vem do ADR-2026-10-23 (adendo, "D3, contrato do ML-3A"): a raiz é a
do alvo, para que um agente num worktree seja julgado pelas regras do checkout principal. O efeito não declarado: um
subagente cria `<sub>/trackfw.yaml` (fora dos congelados) e passa a escrever em `<sub>/...` sob uma raiz sem regras —
por exemplo `.claude/settings.json`, onde ficam os hooks. O mesmo ADR já declara como residual que escrita por Bash
escapa da D3 e da D4; a Wave 0 mede o que esta REQ fecha diante disso.

Varredura de issues abertos em 2026-11-24: nenhum.

## Acceptance Criteria
- [ ] AC1: criar um `trackfw.yaml` aninhado não muda a raiz nem as regras que o guard aplica a um subagente (braço e
  contra-braço no binário)
- [ ] AC2: decisão sobre `trackfw.yaml` novo escrito por subagente fora da raiz registrada em ADR
- [ ] AC3: mesmo mecanismo varrido nos outros guards e nos comandos que resolvem a raiz pelo `trackfw.yaml` mais próximo
- [ ] AC4: `make quality` rc=0 e CI verde

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model, medição e varredura (hades-tf)
**Status:** ⬜ Pending
**Files affected:** `docs/seguranca/2026-11-24-wave0-write-scope-raiz-aninhada-threat-model.md` (novo; único arquivo)
**Actions:**
1. Reproduce F-5 on the binary with an isolated HOME: the two writes (`.claude/trackfw.yaml`, then
   `.claude/settings.json`) as a subagent payload (`agent_id` present), arm and counter-arm (main session; existing
   nested project).
2. Enumeration completeness: every place that resolves a project root by the nearest `trackfw.yaml` — in `internal/guard/`
   (write-scope Step 7 and Step 11a, lease roots, credential, git-branch, opencode/antigravity plugins) and in commands
   (`config`, `validate`, `serve`, `barrier`, `ship`, `push`, `commit`). For each: can a nested `trackfw.yaml` change who
   is allowed to do what?
3. Candidate designs with the measurement for each: (a) every `trackfw.yaml` is frozen for subagents regardless of
   location; (b) root anchored on the outermost `trackfw.yaml` or on the session `cwd` root; (c) a nested root only
   counts if it predates the session or is committed; plus the legitimate case to preserve (agent in a worktree writing
   to the main checkout, ADR-2026-10-23) and monorepos with real nested projects.
4. Threat model with the Bash residual: what this REQ closes given that a subagent can already write through Bash
   (ADR-2026-10-23 residual); recommend the D for the ADR.
5. Falsification targets in both directions and declared residual.
**Acceptance criteria:**
- [ ] The sections above answered with evidence (commands and output), not a one-line assertion
- [ ] No implementation line written for this ML

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-24-wave0-write-scope-raiz-aninhada-threat-model.md
```

## Wave 1 — Implementation
> Dependencies: Wave 0 audited and ADR written. Planned after the Wave 0.

## Wave 2 — Review and red team
> Dependencies: Wave 1 audited.

## Wave 3 — Fechamento
> Dependencies: Wave 2 audited.

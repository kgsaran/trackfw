---
status: wip
date: 2026-09-17
req: "docs/req/REQ-2026-09-17-gate-escreve-na-arvore-que-audita-e-a-guarda-existente-nao-cobre-o-culpado.md"
squad: ""
---

# Roadmap: gate escreve na arvore que audita e a guarda existente nao cobre o culpado

> Created: 2026-09-17 | Status: wip


## Wave 0 — Threat model
> Dependências: nenhuma. 🔴 **Auditada antes de qualquer wave de implementação.**

### ML-0A — superfície de ataque de um gate que escreve na árvore auditada
**Status:** ⬜ Pendente
**Papel:** `hades-tf`

A pergunta não é "o gate sujou o arquivo": é **o que um gate com escrita na árvore auditada permite**.
Pontos a cobrir:

- Um PR hostil pode **induzir** um gate a escrever conteúdo escolhido na árvore? O `init` grava a
  partir de `trackfw.yaml`, `.claude/`, env — algum desses é influenciável pelo PR sob teste?
- 🔴 O dano concreto já observado foi **rebaixar a política de governança** (`governance_mode`
  removido ⇒ warning vira erro; o inverso também é possível ⇒ erro vira warning). Um PR que
  silenciasse violações via efeito colateral de gate seria detectado?
- A guarda proposta (comparar `git status --porcelain` antes/depois) é **suficiente**? Enumere o que
  ela não vê: escrita fora do repositório (`$HOME`, `/tmp`, `~/.trackfw/`), escrita seguida de
  reversão dentro do mesmo gate, e mudança de **modo** de arquivo.
- O `GEMINI.md` criado por gate e commitado por acidente: que classe de arquivo um gate pode
  introduzir na árvore sem ninguém notar?

**Critérios de aceite:** parecer escrito com os vetores enumerados e, para cada um, se a correção
proposta na REQ o fecha, o mitiga ou não o toca. 🔴 Vetor que a correção **não** fecha tem de estar
nomeado — um parecer que só confirma o plano não mediu nada.

---

## Wave 1 — Correção
> Dependências: **Wave 0 auditada.**

### ML-1A — isolar o `cwd` e enumerar a cobertura da guarda
**Status:** ⬜ Pendente
**Papel:** `ares-tf`
Cobre AC1, AC2, AC3, AC4 e AC5.

---

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-09-17-gate-escreve-na-arvore-que-audita-e-a-guarda-existente-nao-cobre-o-culpado.md

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

### ML-1A — gate escreve na arvore que audita e a guarda existente nao cobre o culpado
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes

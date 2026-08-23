---
status: wip
date: 2026-08-23
req: "docs/req/REQ-2026-08-23-titulo-de-roadmap-new-forja-secao-com-gate-que-o-barrier-executa.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: barrier nao executa gate de roadmap nao confiavel e roadmap new sanitiza o titulo

> Created: 2026-08-23 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-23-titulo-de-roadmap-new-forja-secao-com-gate-que-o-barrier-executa.md`
ADR: `docs/adr/ADR-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md`

`trackfw barrier` executa o bloco de gates via `sh -c`. Um roadmap que chega por **PR de terceiro**
faz o mantenedor executar shell que ele nunca aceitou. O título de `roadmap new`, interpolado sem
sanitizar newline, é o vetor plantável — o menor dos dois.

**Reproduzido:** `test -f /tmp/PWNED_TEST` → EXISTE, com `result: blocked` na mesma execução.


## Acceptance Criteria

- [ ] AC1–AC10 da REQ, integralmente
- [ ] 🔴 **AC5 é o critério que decide a entrega:** o fluxo normal de implementação (roadmap
      modificado e não commitado) não pode virar fricção que faça desligar o controle
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça do discriminante de confiança
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md`

**A decisão central do AC4 está em aberto e é sua:** qual é o discriminante de confiança, e como o
consentimento é dado no fluxo normal. Recomende **uma** forma, com o motivo e o que ela custa.

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
test -f docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md
grep -q "Completude de enumera" docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md
grep -q "Residual declarado" docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md
grep -q "discriminante" docs/seguranca/2026-08-23-modelo-de-ameaca-do-gate-nao-confiavel.md
```

## Wave 1 — Sanitização do título

> Dependências: ML-0A auditado. **Parte barata e independente da decisão do discriminante.**

### ML-1A — `roadmap new` sanitiza o título nos 3 CLIs
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

`internal/generators/roadmap.go:150` e o caminho `--from-req`, mais os equivalentes Node e Python.
Newline e retorno de carro no título são entrada malformada.

**Critérios de aceite:** AC1, AC2 · fixture com o título forjado do exemplo · `make quality` exit 0

---

## Wave 2 — Discriminante de confiança no `barrier`

> Dependências: ML-1A auditado. **A forma vem da decisão do ML-0A.**

### ML-2A — `barrier` recusa gate de roadmap não confiável
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Critérios de aceite:** AC3, AC4, AC5, AC6 · **prova de que o fluxo normal segue usável**

---

## Wave 3 — Gate

### ML-3A — Paridade e falsificação nas duas direções
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-2A

**Critérios de aceite:** AC7, AC8, AC9, AC10

---

## Wave 4 — Barreira

### ML-4A — Reverificação
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

Quem escreveu a Wave 0 verifica se a implementação honra o que ela decidiu. **Veredito explícito.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.

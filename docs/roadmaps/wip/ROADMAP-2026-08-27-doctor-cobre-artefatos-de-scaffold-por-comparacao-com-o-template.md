---
status: wip
date: 2026-08-27
req: "docs/req/REQ-2026-08-27-doctor-nao-cobre-os-artefatos-do-scaffold-e-um-slash-command-defasado-nao-e-acusado.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: doctor cobre artefatos de scaffold por comparacao com o template

> Created: 2026-08-27 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-27-doctor-nao-cobre-os-artefatos-do-scaffold-e-um-slash-command-defasado-nao-e-acusado.md`
ADR: `docs/adr/ADR-2026-08-27-doctor-cobre-artefatos-de-scaffold-por-comparacao-com-o-template-com-propriedade-dada-pelo-caminho.md`

**Medido:** manifesto tem 290 artefatos e **zero** de scaffold. Um slash command defasado — ensinando
a estrutura antiga de roadmap, sem Wave 0 — **não é acusado por nada**. Só o `update` revela, e ele
corrige no mesmo passo: o usuário nunca sabe que esteve defasado.


## Acceptance Criteria

- [ ] AC1–AC10 da REQ, integralmente
- [ ] 🔴 **AC4 decide a entrega:** projeto íntegro reporta `no mismatches`. Ruído em projeto correto
      treina o usuário a ignorar o `doctor` — e aí perdemos também o que ele já detecta.
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Completude do inventário e o risco de falso-positivo
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md`

**Duas perguntas decidem a entrega:**

1. **O inventário está completo?** Enumere **por busca** tudo o que o `Scaffold` escreve, nos 3 CLIs —
   e diga o que já tem cobertura (os 2 guards, via `validate`) e o que não tem. A lista da REQ é
   hipótese.
2. 🔴 **Onde isto gera falso-positivo?** Customização deliberada, projeto que nunca rodou `init`
   completo, artefato de outro produto no mesmo caminho, binário antigo em projeto novo. **AC4 é o
   critério que reprova** — ruído em projeto íntegro mata o `doctor` inteiro.

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
test -f docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md
grep -q "Completude de enumera" docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md
grep -q "Residual declarado" docs/seguranca/2026-08-27-modelo-de-ameaca-da-cobertura-de-scaffold.md
```

## Wave 1 — Cobertura no doctor

> Dependências: ML-0A auditado. **ML único:** os 3 stacks saem byte-idênticos.

### ML-1A — `doctor` compara artefatos de scaffold com o template
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

`internal/integrations/doctor.go` + equivalentes. Propriedade por caminho, comparação com template,
**sem** tocar o manifesto.

**Critérios de aceite:** AC1–AC7 da REQ · **projeto íntegro reporta `no mismatches`** ·
`make quality` exit 0 medido

---

## Wave 2 — Gate

### ML-2A — Paridade e falsificação nas duas direções
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Critérios de aceite:** AC8, AC9, AC10 da REQ

---

## Wave 3 — Barreira

### ML-3A — Reverificação
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.

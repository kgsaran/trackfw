---
status: wip
date: 2026-08-26
req: "docs/req/REQ-2026-08-26-checkout-de-pr-executa-hook-versionado-sem-que-nada-avise-o-mantenedor.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: comando que audita a superficie executavel de um checkout de PR

> Created: 2026-08-26 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-26-checkout-de-pr-executa-hook-versionado-sem-que-nada-avise-o-mantenedor.md`
ADR: `docs/adr/ADR-2026-08-26-superficie-executavel-de-um-checkout-de-pr-e-auditada-por-comando-dedicado-nao-por-regra-de-validate.md`

Um checkout de PR hostil executa hook na máquina do mantenedor **sem exigir comando nenhum do
trackfw** — mais amplo que o vetor do `#208`, que exigia rodar `barrier`. **Medido:** `validate` fica
em silêncio diante de hook novo apontando para script novo.


## Acceptance Criteria

- [ ] AC1–AC11 da REQ, integralmente
- [ ] 🔴 **AC5 e AC6 decidem a entrega:** o comando **nomeia o que executa**, não julga se é hostil, e
      **não acusa** wiring legítimo
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça da superfície executável
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-26-modelo-de-ameaca-da-superficie-executavel-de-checkout.md`

**A pergunta que decide o escopo é sua: o que, de fato, executa a partir de um checkout?** O ADR lista
hooks de agente, scripts referenciados, `Makefile` e CI. **Essa lista é a hipótese, não a resposta** —
e a lista dada já errou três vezes na REQ do pin de modelo. Enumere por busca.

Considere, sem se limitar: `.claude/settings.json` e equivalentes dos **6 runtimes** · hooks de git
versionados (`.githooks/`, `core.hooksPath`) · `direnv`/`.envrc` · `package.json` scripts
(`preinstall`, `postinstall`) · `pyproject.toml`/`setup.py` · devcontainer · `.vscode/tasks.json` ·
arquivos de skill e de agente que o CLI de agente lê e interpreta.

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
test -f docs/seguranca/2026-08-26-modelo-de-ameaca-da-superficie-executavel-de-checkout.md
grep -q "Completude de enumera" docs/seguranca/2026-08-26-modelo-de-ameaca-da-superficie-executavel-de-checkout.md
grep -q "Residual declarado" docs/seguranca/2026-08-26-modelo-de-ameaca-da-superficie-executavel-de-checkout.md
```

## Wave 1 — O comando

> Dependências: ML-0A auditado. **ML único:** os 3 stacks saem byte-idênticos.

### ML-1A — Comando de auditoria nos 3 CLIs
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

Escopo conforme a enumeração fechada pelo ML-0A. Reusa `validate`/`doctor` para integridade de
artefato gerenciado. **Informa, não acusa; nomeia, não julga.**

**Critérios de aceite:** AC1–AC8 da REQ · `make quality` exit 0 medido

---

## Wave 2 — Gate

### ML-2A — Paridade e falsificação nas duas direções
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Critérios de aceite:** AC9, AC10, AC11 da REQ

---

## Wave 3 — Barreira

### ML-3A — Reverificação
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

Quem enumerou verifica se a implementação cobre o que ele enumerou. **Veredito explícito.**

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.

---
status: wip
date: 2026-08-28
req: "docs/req/REQ-2026-08-28-modo-de-execucao-perdido-no-validate-script-e-o-doctor-nao-compara-o-bit.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: doctor compara o bit de execucao dos artefatos de scaffold

> Created: 2026-08-28 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-28-modo-de-execucao-perdido-no-validate-script-e-o-doctor-nao-compara-o-bit.md`

O `#211` derrubou o bit de execução de `scripts/trackfw-validate.sh` (100755 → 100644) e o
`.husky/pre-commit` deste repositório parou de rodar o gate — `exit 126`. **E o `scaffold doctor`,
entregue no mesmo PR, reportou `no mismatches`:** ele compara **conteúdo**, não **modo**.

**Bloqueia a release 7.3.0.**


## Acceptance Criteria

- [ ] AC1–AC8 da REQ, integralmente
- [ ] 🔴 **AC4 decide:** artefato que o gerador escreve **sem** bit de execução não pode ser acusado
      por não tê-lo
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Quais artefatos são executáveis, e onde o modo não existe
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-28-modelo-de-ameaca-do-bit-de-execucao.md`

**Duas perguntas:**

1. **Quais dos 17 artefatos de scaffold o gerador escreve como executáveis, nos 3 CLIs?** Enumere
   **por busca** no código de escrita (`0755` × `0644`), não pela aparência do repositório. Divergência
   entre runtimes aqui é achado.
2. 🔴 **Onde o bit não é representável ou é enganoso?** Windows, filesystem montado com `noexec`,
   `core.fileMode=false` no git, checkout via zip/tarball. **Acusar nesses casos é o falso-positivo
   que reprova (AC4/AC5).**

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
test -f docs/seguranca/2026-08-28-modelo-de-ameaca-do-bit-de-execucao.md
grep -q "Completude de enumera" docs/seguranca/2026-08-28-modelo-de-ameaca-do-bit-de-execucao.md
grep -q "Residual declarado" docs/seguranca/2026-08-28-modelo-de-ameaca-do-bit-de-execucao.md
```

## Wave 1 — Correção e cobertura

> Dependências: ML-0A auditado.

### ML-1A — Restaurar o modo e comparar o bit nos 3 CLIs
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

**Critérios de aceite:** AC1–AC6 da REQ · `make quality` exit 0 medido

---

## Wave 2 — Gate

### ML-2A — Falsificação nas duas direções
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

**Critérios de aceite:** AC7, AC8 da REQ

---

## Notas
- **Fora de escopo:** tudo listado no *Negative scope* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.

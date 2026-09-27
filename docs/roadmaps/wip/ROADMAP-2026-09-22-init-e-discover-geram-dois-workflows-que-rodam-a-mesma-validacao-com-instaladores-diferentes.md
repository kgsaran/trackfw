---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-02-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md"
squad: ""
---

# Roadmap: `init` e `discover` geram dois workflows que rodam a mesma validação, com instaladores diferentes

> Created: 2026-09-22 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-02-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md -->
REQ: docs/req/REQ-2026-09-02-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md

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

### ML-1A — **AC1** — 🔴 **Determinar por que existem dois**, com evidência (histórico, ADR, comportamento
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — 🔴 **Determinar por que existem dois**, com evidência (histórico, ADR, comportamento
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — Se não houver: um único workflow gerado, com o instalador escolhido e **justificado**.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — Se não houver: um único workflow gerado, com o instalador escolhido e **justificado**.
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **Controle:** o caminho de adoção que hoje depende do workflow removido **continua
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **Controle:** o caminho de adoção que hoje depende do workflow removido **continua
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — Paridade nos 3 CLIs — a duplicação existe nos três.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — Paridade nos 3 CLIs — a duplicação existe nos três.
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — Migração para quem **já tem os dois** instalados: o update remove o obsoleto, ou o
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — Migração para quem **já tem os dois** instalados: o update remove o obsoleto, ou o
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — make quality e **CI** verdes.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — make quality e **CI** verdes.
- [ ] build passes
- [ ] tests green

---

## Correção de 2026-09-27 — a cópia do PRÓPRIO repositório ficou para trás

### Como apareceu

Ao cruzar os 4 PRs mergeados em 2026-09-27 com as issues que eles aparentavam fechar (nenhum trouxe
palavra-chave), a auditoria mediu que o **#456 corrigiu o template embutido e não a cópia versionada
deste repositório**:

```
internal/generators/scaffold_doctor.go   push: branches: [main]     ← corrigido pelo #456
.github/workflows/trackfw-validate.yml   on: [push, pull_request]   ← ficou para trás
```

🔴 **É a mesma forma do defeito que a `REQ-2026-09-23` fechou horas antes** — literal corrigido, cópia
versionada não. Outro par, o mesmo mecanismo.

### Por que NÃO estendi a guarda de paridade aos workflows

A guarda criada na `REQ-2026-09-23` (`TestScripts_LiteralMatchesVersionedCopy`) cobre **5 pares de
scripts** e **nenhum workflow** — medido: `grep -cE 'workflow|\.yml'` no teste dá **0**.

A tentação era estender. Medi a população antes:

```
trackfw-gate.yml       já com o gatilho novo
trackfw-validate.yml   🔴 gatilho antigo   ← ÚNICO divergente
```

**Um par divergente, não uma família.** A guarda de scripts nasceu de um defeito que **já tinha
acontecido duas vezes**; construir a mesma maquinaria para um caso único é maquinaria que não se
paga. Fica **declarado como dívida**: quando aparecer o segundo par de workflow divergente, a guarda
se justifica.

⚠️ **Registro uma recomendação minha que corrigi no meio do caminho.** Eu havia dito ao KG que *"o par
literal↔workflow é mesma causa, e a Regra Dura manda"* — sugerindo reabrir a `REQ-2026-09-23` pela
terceira vez. A Regra Dura manda para **mesma causa com múltiplos sítios**; com um sítio só, o que
ela exige é **corrigir**, não construir guarda. Eu estava aplicando a regra pelo formato, não pelo
conteúdo.

### Verificação

- `python3 scripts/check-required-status-checks.py --scope dw` → `[OK] declared=9, workflow_checks=45, D\W=∅`
  🔴 **Este era o risco real da mudança:** `governance-go-install` é contrato de
  `required_status_checks`. Com o gatilho novo ele continua sendo produzido em `pull_request`, que é
  onde os required checks são avaliados.
- YAML parseado com `yaml.safe_load` — não por inspeção visual
- `make quality` `exit=0`, **1367** `^OK `, **0** `: FALHA`
- `trackfw validate` 170 warnings, 0 violations

### O que continua aberto nesta REQ

A **sugestão 1 da issue #451** — *detectar workflow de governança já existente e não instalar um
segundo, ou avisar que vai substituir* — **não** foi endereçada, nem aqui nem pelo #456. É o que
sobra, e é a parte que protege o consumidor que já está onboardado.

⚠️ E a redução é de **3 para 2** execuções por push em PR, não para 1. A issue pede economia de cota;
2 ainda é duplicata. A coexistência dos dois arquivos está decidida na `ADR-2026-08-28`, então
reduzir para 1 é mudança de decisão, não de implementação.

---
status: wip
date: 2026-11-07
req: "docs/req/REQ-2026-11-07-trap-exit-de-self-test-cita-variavel-local-e-mascara-a-falha-real.md"
squad: ""
---

# Roadmap: trap EXIT de self-test cita variavel local e mascara a falha real

> Created: 2026-11-07 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-07-trap-exit-de-self-test-cita-variavel-local-e-mascara-a-falha-real.md -->
Defeito relatado na issue #671 e reproduzido pelo arquiteto (tabela na REQ). Dois sítios, um gate novo. Varredura de issues e REQs abertas: nenhuma com o mesmo mecanismo. Este roadmap fecha a issue #671.

REQ: docs/req/REQ-2026-11-07-trap-exit-de-self-test-cita-variavel-local-e-mascara-a-falha-real.md

## Acceptance Criteria
- [ ] AC1–AC4 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ✅ Concluído
**Files affected:** `docs/seguranca/2026-11-07-wave0-trap-exit-variavel-local.md`
**Actions:**
1. Completude da enumeração: os dois sítios da REQ são todos? Procurar o mesmo mecanismo em todos os `.sh` versionados e nos `run:` dos workflows, incluindo trap em função chamada por função, `trap ... ERR`/`RETURN`/`INT`/`TERM`, e `local` declarado depois do trap.
2. A forma da correção: `printf %q` capturado na instalação versus variável global — qual quebra com caminho contendo espaço, aspas ou `$`; o que acontece com a restauração de `GITHUB_OUTPUT` (exportar na saída do processo tem efeito?).
3. O gate novo: falso negativo (formas que escapam: `${VAR}`, `"$VAR"` dentro de aspas simples, trap em linha com `;`, `local -r`, `declare`/`typeset` locais, função com `function f {`) e falso positivo (trap entre aspas simples citando global com o mesmo nome de um local de outra função).
4. Residual declarado.
**Acceptance criteria:**
- [x] as quatro seções respondidas com medição, não com afirmação de uma linha
- [x] itens obrigatórios para o ML-1A listados no fim do documento
- [x] nenhuma linha de implementação escrita

**Veredito:** APROVA COM RESSALVAS. **Auditoria:** o agente caiu por erro de rede depois de escrever o documento completo (412 linhas, veredito e 7 itens obrigatórios); índice limpo. O arquiteto confirmou por leitura o terceiro sítio (`check-orphan-gates.sh`, que a varredura inicial perdeu por `head -40`) e o R6 (`check-validate-rule-pins.sh`, trap EXIT sobreposto no nível do script). Os dois entraram na REQ e no ML-1A. O nome do gate fica `check-no-trap-local-var.sh` (o documento usa outro).

**Gates da wave:**
```bash
test -s docs/seguranca/2026-11-07-wave0-trap-exit-variavel-local.md
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada.

### ML-1A — Corrigir os dois traps e criar o gate (ares-tf) — AC1, AC2, AC3
**Status:** ⬜ Pendente
**Files affected:** `scripts/check-shim-diff-applies.sh`, `scripts/check-req-roadmap-rule-is-status-aware.sh`, `scripts/check-orphan-gates.sh`, `scripts/check-validate-rule-pins.sh`, `scripts/check-no-trap-local-var.sh` (novo), `Makefile`
**Acceptance criteria:**
- [ ] os três traps capturam o valor na instalação com `printf -v q '%q'`, e a restauração inócua de `GITHUB_OUTPUT` sai do trap do shim (itens 2–4 do threat model)
- [ ] `check-validate-rule-pins.sh`: um trap EXIT só, limpando `$TMP_DIR` e `$FENCE_TMP` (AC2b)
- [ ] medição com `/bin/bash` (3.2) e com bash 5 explícito, numa cópia de cada script, nos caminhos `set -e`, `set -u`, retorno normal seguido de exit e `exit 1`: sem `unbound variable` do trap, sem diretório temporário sobrando
- [ ] `check-shim-diff-applies.sh --self-test`: 27 passam; `check-req-roadmap-rule-is-status-aware.sh --self-test`: 4 braços passam
- [ ] `scripts/check-no-trap-local-var.sh` com `--self-test` (braços positivo e negativo, guarda de vacuidade), varrendo `git ls-files '*.sh'`, ligado em `parity-rest`; cobre os casos do item 5 do threat model e declara os residuais R2–R4
- [ ] contra-braço: com cada sítio revertido numa cópia, o gate reprova

**Gates da wave:**
```bash
bash scripts/check-no-trap-local-var.sh --self-test
bash scripts/check-no-trap-local-var.sh
bash scripts/check-shim-diff-applies.sh --self-test
bash scripts/check-req-roadmap-rule-is-status-aware.sh --self-test
bash scripts/check-orphan-gates.sh --self-test
```

## Wave 2 — Red team
> Dependencies: Wave 1 completa.

### ML-2A — Red team (hades-tf)
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-11-07-redteam-trap-exit-variavel-local.md`
**Acceptance criteria:**
- [ ] cada item obrigatório da Wave 0 atacado com medição
- [ ] veredito escrito

**Gates da wave:**
```bash
test -s docs/seguranca/2026-11-07-redteam-trap-exit-variavel-local.md
```

---
status: wip
date: 2026-11-11
req: "docs/req/REQ-2026-11-11-mktemp-sem-limpeza-garantida-no-mesmo-escopo-vaza-temporarios-em-self-tests.md"
squad: "hades-tf, ares-tf, zeus-tf"
---

# Roadmap: mktemp sem limpeza garantida no mesmo escopo vaza temporarios em self-tests

> Created: 2026-11-11 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-11-mktemp-sem-limpeza-garantida-no-mesmo-escopo-vaza-temporarios-em-self-tests.md -->
REQ: docs/req/REQ-2026-11-11-mktemp-sem-limpeza-garantida-no-mesmo-escopo-vaza-temporarios-em-self-tests.md
ADR: docs/adr/ADR-2026-11-11-mktemp-em-script-tem-limpeza-garantida-por-variavel-no-mesmo-escopo.md
Issue: #676 (relato e varredura por variável de lourivalgarciajunior). Este roadmap fecha o #676.

`mktemp` sem limpeza garantida no mesmo escopo vaza temporários quando o script sai antes do `rm` do caminho
normal. O relator mediu 11 sítios e 6 falsos positivos de um discriminante por variável (tabela na REQ). A ADR
fixa o discriminante (D2), a forma de correção (D3) e a correção do registro da REQ-2026-11-07 (D4).

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] AC1 — inventário medido pelo discriminante da ADR D2, conferido contra o relato (ML-0A)
- [ ] AC2 — sítios não cobertos corrigidos, medido por efeito com falha forçada depois de 490–491 (ML-1A)
- [ ] AC3 — gate novo no `parity-rest`, com self-test por mecanismo, vacuidade e contra-braço (ML-1B)
- [ ] AC4 — registro do AC1 da REQ-2026-11-07 corrigido (ML-1C)
- [ ] AC5 — `make quality` verde (ML-2A)

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model e inventário (AC1)
> Dependencies: none. Blocks all implementation.

### ML-0A — Inventário por variável e threat model
**Status:** 🔄 In progress
**Squad:** hades-tf
**Files affected:**
- `docs/seguranca/2026-11-11-wave0-mktemp-limpeza.md`

**Actions:**
1. Inventário (AC1): todo `mktemp` em `git ls-files '*.sh'`, classificado pelo discriminante da ADR D2 com
   instrumento próprio e ciente de escopo, sem contagem por `grep`. Confronte com os 11 sítios e os 6 falsos
   positivos do #676 e explique cada divergência. Liste também `mktemp` não atribuído a variável.
2. Para cada sítio não coberto: a forma de correção da ADR D3 que se aplica. Diga se a função já tem trap EXIT,
   que seria substituído, e o que esse trap limpa hoje.
3. Threat model do gate: como ele fica verde sem cobrir (falso negativo) e como reprova código correto (falso
   positivo). Inclua trap substituído depois, `trap - EXIT`, subshell e `exec`.
4. Residual declarado.
**Acceptance criteria:**
- [ ] Inventário com contagem conferida por um segundo caminho e lista final de sítios não cobertos
- [ ] Nenhuma linha de implementação escrita neste ML

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-11-wave0-mktemp-limpeza.md
```

## Wave 1 — Correção e gate (AC2, AC3, AC4)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B em paralelo (arquivos disjuntos); ML-1C é do Zeus.

### ML-1A — Limpeza garantida nos sítios não cobertos (AC2)
**Status:** ⬜ Pending
**Squad:** ares-tf
**Files affected:**
- `scripts/check-shim-diff-applies.sh`
- `scripts/check-parity-call-site-pins.sh`
- `scripts/check-release-tag-parity.sh`
- `scripts/check-usage-silencing.sh`
- `scripts/check-gates-falsify.sh`
- `scripts/check-no-literal-nul-in-source.sh`

A lista final vem do ML-0A. Sítio novo achado lá entra aqui.
**Actions:** aplicar a ADR D3 em cada sítio. Medir por efeito: falha forçada depois das linhas 490–491 do
`check-shim-diff-applies.sh`, sem diretório novo em `$TMPDIR`, e caminho de sucesso inalterado em cada script.
**Acceptance criteria:**
- [ ] Todos os sítios não cobertos do ML-0A corrigidos
- [ ] Falha forçada na posição do relato: rc diferente de zero, nenhum temporário novo
- [ ] Caminho de sucesso de cada script tocado com o mesmo resultado

### ML-1B — Gate `check-mktemp-cleanup.sh` (AC3)
**Status:** ⬜ Pending
**Squad:** ares-tf
**Files affected:**
- `scripts/check-mktemp-cleanup.sh`
- `Makefile`

**Actions:** gate da ADR D2 sobre `git ls-files '*.sh'`, ligado no `parity-rest`. `--self-test` com braços que
passam (citação direta, `${V:-}`, apelido `printf -v`/atribuição, função do trap, contenção) e braços que
reprovam (sem trap; trap do escopo que não cita V; só `rm` no caminho normal; trap substituído depois). Guarda
de vacuidade com piso medido. Mensagem de falha com arquivo, linha e variável.
**Acceptance criteria:**
- [ ] `--self-test` verde, com cada braço nomeado
- [ ] Contra o repositório, verde depois do ML-1A e vermelho com qualquer sítio do ML-0A revertido
- [ ] Nenhum falso positivo nos 6 casos legítimos do #676

### ML-1C — Registro da REQ-2026-11-07 (AC4)
**Status:** ⬜ Pending
**Squad:** zeus-tf
**Files affected:**
- `docs/req/REQ-2026-11-07-trap-exit-de-self-test-cita-variavel-local-e-mascara-a-falha-real.md`

**Acceptance criteria:**
- [ ] Nota no AC1 apontando para o #676, esta REQ e a ADR D4

**Wave gates:**
```bash
bash scripts/check-mktemp-cleanup.sh --self-test
bash scripts/check-mktemp-cleanup.sh
bash scripts/check-shim-diff-applies.sh --self-test
bash scripts/check-no-trap-local-var.sh
```

## Wave 2 — Red team e qualidade (AC5)
> Dependencies: Wave 1 completa.

### ML-2A — Red team do gate e das correções
**Status:** ⬜ Pending
**Squad:** hades-tf
**Files affected:**
- `docs/seguranca/2026-11-11-redteam-mktemp-limpeza.md`

**Acceptance criteria:**
- [ ] Parecer com veredito; falsos negativos e positivos do gate atacados com sítios sintéticos

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-11-redteam-mktemp-limpeza.md
make quality
```

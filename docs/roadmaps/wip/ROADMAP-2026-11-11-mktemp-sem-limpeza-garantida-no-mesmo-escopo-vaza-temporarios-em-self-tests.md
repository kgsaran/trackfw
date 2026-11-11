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
- [x] AC1 — inventário medido pelo discriminante da ADR D2, conferido contra o relato (ML-0A)
- [ ] AC2 — sítios não cobertos corrigidos, medido por efeito com falha forçada depois de 490–491 (ML-1A)
- [ ] AC3 — gate novo no `parity-rest`, com self-test por mecanismo, vacuidade e contra-braço (ML-1B)
- [x] AC4 — registro do AC1 da REQ-2026-11-07 corrigido (ML-1C)
- [ ] AC5 — `make quality` verde (ML-2A)

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model e inventário (AC1)
> Dependencies: none. Blocks all implementation.

### ML-0A — Inventário por variável e threat model
**Status:** ✅ Done
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
- [x] Inventário com contagem conferida por um segundo caminho e lista final de sítios não cobertos
- [x] Nenhuma linha de implementação escrita neste ML

**Auditoria (Zeus, 2026-11-11):** aprovado com uma correção. Conferido: 68 `mktemp` atribuídos e 11 não cobertos,
mesma lista do relator; os 6 falsos positivos do relato reclassificados (RFP-1..6); alcançabilidade medida em
`nul-source:89` e `shim-diff:490,491`; `git status` só com o relatório. **Correção:** a forma D3 proposta para
`check-shim-diff-applies.sh:158` (somar `$q_tf` ao trap da linha 179) está errada. `run_classify` e
`run_self_test` são ramos exclusivos do `case` final (linhas 686–696), e o self-test chama `--classify` em
processo novo (`bash "${BASH_SOURCE[0]}" --classify`). O trap da 179 nunca está ativo no processo que cria
`tmpfile`, e se estivesse, `$q_tf` seria expandido vazio na instalação. Vale a forma canônica isolada, logo
depois da 158. O próprio relatório diz que `run_classify` roda como subprocesso (§1 e T8), o que contradiz a sua
premissa de T2 para esse sítio.

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-11-wave0-mktemp-limpeza.md
```

## Wave 1 — Correção e gate (AC2, AC3, AC4)
> Dependencies: Wave 0 auditada. ML-1A e ML-1B em paralelo (arquivos disjuntos); ML-1C é do Zeus.

### ML-1A — Limpeza garantida nos sítios não cobertos (AC2)
**Status:** ✅ Done
**Squad:** ares-tf
**Files affected:**
- `scripts/check-shim-diff-applies.sh`
- `scripts/check-parity-call-site-pins.sh`
- `scripts/check-release-tag-parity.sh`
- `scripts/check-usage-silencing.sh`
- `scripts/check-gates-falsify.sh`
- `scripts/check-no-literal-nul-in-source.sh`

Lista final do ML-0A (11 sítios, formas D3 auditadas):
- `check-shim-diff-applies.sh:158` `tmpfile`: forma canônica isolada logo após a 158
  (`local q_tf; printf -v q_tf '%q' "$tmpfile"; trap "rm -f -- $q_tf" EXIT`). Não tocar a linha 179.
- `check-shim-diff-applies.sh:490,491`: contenção sob `$tmp` (`mktemp -d "$tmp/plumb-repo.XXXXXX"` etc.).
- `check-no-literal-nul-in-source.sh:89,133,148,165`: trap da linha 73 combinado,
  `'rm -rf "$WORK" "${EMPTY_EXC:-}" "${EXC_MISSING:-}" "${EXC_ZERO:-}" "${EXC_WRONG:-}"'`; os arquivos ficam fora
  de `$WORK` (o `git -C "$WORK" add .` os pegaria).
- `check-parity-call-site-pins.sh:222` `local_dryrun`: trap da linha 121 combinado,
  `'rm -f "$RECIPE_LINES_FILE" "${local_dryrun:-}"'`.
- `check-release-tag-parity.sh:330` `_src_dir`: contenção sob `$WORK`.
- `check-gates-falsify.sh:1476` `bare_origin`: contenção sob `$WORK`.
- `check-usage-silencing.sh:30` `d` em `novo_projeto` (chamada como `d=$(novo_projeto)`): `WORK` global com trap
  no topo e `mktemp -d "$WORK/proj.XXXXXX"`; unificar o trap da linha 95 com o global (residual R1 do ML-0A).
**Actions:** aplicar a ADR D3 em cada sítio. Medir por efeito: falha forçada depois das linhas 490–491 do
`check-shim-diff-applies.sh`, sem diretório novo em `$TMPDIR`, e caminho de sucesso inalterado em cada script.
**Acceptance criteria:**
- [x] Todos os sítios não cobertos do ML-0A corrigidos
- [x] Falha forçada na posição do relato: rc diferente de zero, nenhum temporário novo
- [x] Caminho de sucesso de cada script tocado com o mesmo resultado

**Auditoria (Zeus, 2026-11-11):** diff lido nos 6 arquivos, com as 11 formas exatamente as prescritas. Falha
forçada (`exit 73`) medida antes e depois em `shim-diff:490–491`, `nul-source:89` e `usage-silencing` (dentro de
`novo_projeto`): rc=73 nos dois lados, vazamento em HEAD e nenhum depois. Os self-tests de shim-diff (27/27),
nul-source (5/5) e usage-silencing (2/2) não mudaram; parity-pins tem 11 verificações antes e depois;
`check-no-trap-local-var.sh` rc=0. Sem medição local:
- `release-tag:330`: a função retorna cedo fora do Windows; fica para o CI de Windows;
- `gates-falsify:1476`: o censo completo fica para o `make quality` do ML-2A.

Desvio do executor: rodou `make test` (`go test ./...`, rc=0), que o handoff proibia; ele mesmo declarou. Achado
lateral, conferido pelo arquiteto: `mktemp` sem template ignora `TMPDIR` no macOS (nota no vault).

### ML-1B — Gate `check-mktemp-cleanup.sh` (AC3)
**Status:** 🔄 In progress
**Squad:** ares-tf
**Files affected:**
- `scripts/check-mktemp-cleanup.sh`
- `Makefile`

**Actions:** gate da ADR D2 sobre `git ls-files '*.sh'`, ligado no `parity-rest`. `--self-test` com braços que
passam (citação direta, `${V:-}`, apelido `printf -v`/atribuição, função do trap, contenção) e braços que
reprovam (sem trap; trap do escopo que não cita V; só `rm` no caminho normal; trap substituído depois). Guarda
de vacuidade com piso medido. Mensagem de falha com arquivo, linha e variável. Cobrir também os vetores
FN-1..12 e FP-1..4 da §3 do relatório do ML-0A, em especial FN-11, FN-12 e o critério sintático do FP-4 (linha
sem `$(` não é candidata), sem exclusão por caminho (R6).
**Acceptance criteria:**
- [ ] `--self-test` verde, com cada braço nomeado
- [ ] Contra o repositório, verde depois do ML-1A e vermelho com qualquer sítio do ML-0A revertido
- [ ] Nenhum falso positivo nos 6 casos legítimos do #676

### ML-1C — Registro da REQ-2026-11-07 (AC4)
**Status:** ✅ Done
**Squad:** zeus-tf
**Files affected:**
- `docs/req/REQ-2026-11-07-trap-exit-de-self-test-cita-variavel-local-e-mascara-a-falha-real.md`

**Acceptance criteria:**
- [x] Nota no AC1 apontando para o #676, esta REQ e a ADR D4

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

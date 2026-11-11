---
status: Open
date: 2026-11-11
author: "Zeus (trackfw_architect)"
adr: "docs/adr/ADR-2026-11-11-mktemp-em-script-tem-limpeza-garantida-por-variavel-no-mesmo-escopo.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-11-mktemp-sem-limpeza-garantida-no-mesmo-escopo-vaza-temporarios-em-self-tests.md"
---

# REQ: mktemp sem limpeza garantida no mesmo escopo vaza temporarios em self-tests

> Date: 2026-11-11 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Relato #676 (Lourival, medido por efeito no Windows, blob `108c66dd` de `scripts/check-shim-diff-applies.sh`): o
self-test deixa dois diretórios temporários por execução. `plumb_repo` e `script_copy_dir` (linhas 490–491) só
são apagados no `rm -rf` da linha 670, no fim do caminho normal; qualquer saída no meio vaza os dois. A linha
158 (`tmpfile=$(mktemp)`) tem a mesma causa.

É a classe "`mktemp` sem limpeza garantida no mesmo escopo", e não um arquivo só. A varredura por variável do
relator, sobre `scripts/` de `main`, achou **11 sítios** da classe:

| Arquivo | Linha | Variável |
|---|---|---|
| `scripts/check-shim-diff-applies.sh` | 158 | `tmpfile` |
| `scripts/check-shim-diff-applies.sh` | 490 | `plumb_repo` |
| `scripts/check-shim-diff-applies.sh` | 491 | `script_copy_dir` |
| `scripts/check-parity-call-site-pins.sh` | 222 | `local_dryrun` |
| `scripts/check-release-tag-parity.sh` | 330 | `_src_dir` |
| `scripts/check-usage-silencing.sh` | 30 | `d` |
| `scripts/check-gates-falsify.sh` | 1476 | `bare_origin` |
| `scripts/check-no-literal-nul-in-source.sh` | 89, 133, 148, 165 | `EMPTY_EXC`, `EXC_MISSING`, `EXC_ZERO`, `EXC_WRONG` |

A Wave 0 confere a lista com instrumento próprio: os números são do relator, e a ADR manda medir de novo.

O AC1 da REQ-2026-11-07 está marcado `[x]` com a cláusula "nenhum diretório temporário sobrando", que este relato
falsifica: a falha foi forçada antes das linhas 490–491.

## Acceptance Criteria
- [ ] **AC1 — Inventário medido.** Uma tabela de todo `mktemp` em `git ls-files '*.sh'`, classificado pelo
  discriminante da ADR D2 (coberto por citação direta, apelido, função do trap ou contenção; ou não coberto),
  conferida contra os 11 sítios e os 6 falsos positivos do relato #676. Toda divergência é explicada.
- [ ] **AC2 — Os sítios não cobertos passam a ter limpeza garantida** na forma da ADR D3. Medido por efeito em
  `check-shim-diff-applies.sh --self-test`, com falha forçada **depois** das linhas 490–491 (na posição do
  relato): rc diferente de zero e nenhum diretório temporário novo. O caminho de sucesso de cada script tocado
  dá o mesmo resultado de antes.
- [ ] **AC3 — Gate novo**, ligado no `parity-rest`, reprova `mktemp` atribuído a variável sem cobertura (ADR D2).
  Ele tem `--self-test` com braços para cada mecanismo legítimo (citação direta, `${V:-}`, apelido `printf -v`,
  função do trap e contenção), que devem passar, e para cada forma da classe (sem trap; trap no escopo que não
  cita V; `rm` só no caminho normal; trap substituído depois), que devem reprovar. Tem também guarda de
  vacuidade. Contra-braço: reverter qualquer um dos 11 sítios faz o gate reprovar.
- [ ] **AC4 — Registro da REQ-2026-11-07 corrigido** com uma nota no AC1 que aponta para o #676 e para esta REQ
  (ADR D4).
- [ ] **AC5 — `make quality` verde.**

## Negative scope

- Não muda o comportamento de nenhum gate além da limpeza dos seus temporários.
- Não trata `mktemp` fora de `.sh` versionado (por exemplo, `run:` de workflow do GitHub).
- Não trata a parte 5 do self-test do `check-shim-diff-applies.sh` no Windows (nome com caractere de controle),
  que já estava fora de escopo na REQ-2026-11-07.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-11-11-mktemp-em-script-tem-limpeza-garantida-por-variavel-no-mesmo-escopo.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-11-mktemp-sem-limpeza-garantida-no-mesmo-escopo-vaza-temporarios-em-self-tests.md

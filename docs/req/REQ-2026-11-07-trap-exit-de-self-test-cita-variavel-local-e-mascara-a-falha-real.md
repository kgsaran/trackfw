---
status: Open
date: 2026-11-07
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-07-trap-exit-de-self-test-cita-variavel-local-e-mascara-a-falha-real.md"
---

# REQ: trap EXIT de self-test cita variavel local e mascara a falha real

> Date: 2026-11-07 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Relatado por um consumidor externo na issue #671 (2026-11-06). Em `scripts/check-shim-diff-applies.sh`, `run_self_test` declara `local tmp` e instala `trap 'rm -rf "$tmp"' EXIT`. O trap é avaliado na saída do processo. Quando a função aborta por `set -e` ou `set -u`, o bash 5 desempilha o quadro da função antes de rodar o trap: `tmp` já não existe, e sob `set -u` o próprio trap falha. Três efeitos:

1. o diretório temporário não é removido;
2. a restauração de `GITHUB_OUTPUT` do segundo trap não roda;
3. a última linha impressa é `line 1: tmp: unbound variable`, que esconde a falha real.

Reprodução do arquiteto em 2026-11-07, função com `local tmp` e `trap 'rm -rf "$tmp"' EXIT`:

| saída da função | bash 5.3 | bash 3.2 (macOS) |
|---|---|---|
| falha por `set -e` | vaza + mensagem espúria | ok |
| falha por `set -u` | vaza + mensagem espúria | vaza + mensagem espúria |
| `exit 1` explícito | ok | ok |

O CI roda em `ubuntu-latest` (bash 5). O caminho de sucesso do self-test sai por `exit` explícito dentro da função, por isso o defeito nunca apareceu.

Varredura dos traps de `scripts/*.sh` (variável do trap declarada `local` na função que o instala): três sítios. A varredura inicial do arquiteto achou dois; o threat model da Wave 0 achou o terceiro (a saída do grep tinha sido truncada com `head -40`).

- `scripts/check-shim-diff-applies.sh`, `run_self_test` (`tmp` e `saved_github_output`);
- `scripts/check-req-roadmap-rule-is-status-aware.sh`, `self_test` (`WORK`, sob `set -uo pipefail`);
- `scripts/check-orphan-gates.sh`, `self_test` (`WORK`, sob `set -uo pipefail`).

O threat model mediu também um terceiro caminho: se a função **retorna** normalmente e o processo sai depois, o trap vaza nas duas versões do bash.

**Mesmo sintoma, outra causa (entra nesta REQ):** em `scripts/check-validate-rule-pins.sh`, o `trap 'rm -rf "$FENCE_TMP"' EXIT` da linha ~910 substitui o `trap 'rm -rf "$TMP_DIR"' EXIT` da linha ~35, no nível do script. O `$TMP_DIR` vaza em toda execução.

Varredura de issues abertas e REQs abertas: nenhuma com o mesmo mecanismo. O vault não tem nota sobre ele.

## Acceptance Criteria
- [ ] **AC1.** Nos três sítios, o trap não depende de variável `local` que deixa de existir na saída: o valor é capturado na instalação do trap, com quoting seguro (`printf %q`), ou a variável deixa de ser `local`. Medido no bash 5 com falha forçada (por `set -e` e por `set -u`) numa cópia de cada script: rc diferente de zero, nenhum `unbound variable` vindo do trap e nenhum diretório temporário sobrando.
- [ ] **AC2.** O caminho de sucesso dos três self-tests continua com o mesmo resultado (`check-shim-diff-applies.sh --self-test`: 27 passam; `check-req-roadmap-rule-is-status-aware.sh --self-test`: 4 braços passam; `check-orphan-gates.sh --self-test`: mesmo resultado de antes).
- [ ] **AC2b.** `check-validate-rule-pins.sh` remove os dois diretórios temporários ao sair: um trap só, que limpa `$TMP_DIR` e `$FENCE_TMP`. Medido por existência depois de uma execução.
- [ ] **AC3.** Um gate novo, ligado em `parity-rest`, reprova quando um `trap` com corpo entre aspas simples cita uma variável declarada `local` na função que instala o trap. Ele varre todos os `.sh` versionados (`git ls-files '*.sh'`), tem `--self-test` com braço positivo e negativo, e uma guarda de vacuidade (reprova se não encontrar nenhum `trap`). Contra-braço: com qualquer um dos três sítios revertido, o gate reprova.
- [ ] **AC4.** `make quality` verde.

## Negative scope

- Não muda o comportamento dos dois gates fora do caminho de saída (casos, mensagens, rc).
- Não trata a parte 5 do self-test do shim no Windows (nome de arquivo com quebra de linha, inconstruível pelo git do Windows). O `classify` roda em ubuntu, e o relato não pede nada sobre isso.
- Não reescreve os outros traps do repositório, que estão no nível do script ou citam variável global.
- O gate não detecta sobreposição de trap EXIT no nível do script (`check-channels-content.sh` tem três, em blocos de modo exclusivos); o único caso real, `check-validate-rule-pins.sh`, é corrigido à mão.
- Não cobre `trap` com corpo entre aspas duplas nem trap instalado via `eval`; o gate declara isso como residual.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-07-trap-exit-de-self-test-cita-variavel-local-e-mascara-a-falha-real.md

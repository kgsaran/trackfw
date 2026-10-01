---
status: Open
date: 2026-10-01
author: "zeus-tf"
adr: "docs/adr/ADR-2026-09-01-gate-de-wave-e-contrato-portavel-em-shell-posix-nao-script-do-sistema-operacional.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-01-o-barrier-executa-cada-linha-do-bloco-de-gates-como-sh-c-separado-e-o-contrato-nao-diz.md"
---

# REQ: o barrier executa cada linha do bloco de gates como sh -c separado e o contrato não diz a consequência

> Date: 2026-10-01 | Status: Open
| GitHub Issue: #491

## Motivation

A regra 5 de `docs/cli-parity.md` diz que **cada linha** não vazia e não comentada do bloco
`**Gates da wave:**` é um comando de gate. O `ParseGates` (`internal/roadmapdoc/roadmapdoc.go`) faz
exatamente isso, e o `barrier` roda cada linha num `sh -c` próprio.

🔴 **Premissa do issue corrigida:** o título do #491 diz que isso "não está no contrato". Está, na
regra 5. O que falta são três coisas, e cada uma já custou caro em 2026-09-30:

1. **A consequência não está escrita.** Linhas não compartilham estado (`export`, `cd`, variável) e
   construções multilinha (aspas abertas, `$(` sem fechar, heredoc, `\` no fim) viram fragmentos.
   Um fusível de profundidade numa linha e a chamada na seguinte → o fusível nunca disparou, e uma
   recursão correu ~120 s numa revisão de segurança.
2. **A superfície de autoria não avisa.** Quem escreve o gate o testa extraindo o bloco e rodando
   com `bash`, e ele **passa**. Aconteceu com os gates da Wave 0 do #476 e do #485: passavam em
   `bash` e reprovavam com **13 falhas** sob o `barrier` real.
3. **O produto não distingue "bloco malformado" de "gate reprovou".** `n=$(python3 -c "` sozinho é
   erro de sintaxe do `sh` (exit 2) e aparece como gate reprovado, sem nomear a causa.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Wave 0:** decisão entre (a) só documentar e avisar, (b) detectar linha que não é
  comando completo e reportar como bloco malformado, (c) executar o bloco inteiro como um script.
  Medida no acervo: quantos blocos reais **dependem** da semântica por linha (por exemplo, um gate
  que falha no meio e outro depois que ainda deve rodar e aparecer na evidência)
- [ ] **AC2** — `docs/cli-parity.md` (regra 5) escreve a consequência: sem estado compartilhado, sem
  construção multilinha, com um exemplo do erro e da forma certa (uma linha)
- [ ] **AC3** — o template gerado de roadmap e os assets de autoria (agente/skill do arquiteto) avisam
  isso no ponto em que o autor escreve o gate
- [ ] **AC4** — conforme a decisão do AC1: se for (b), o `barrier` reprova um bloco com fragmento
  incompleto com mensagem que **nomeia a linha e diz "comando incompleto"**, distinta de "gate
  reprovou"; se for (c), a evidência por comando continua existindo
- [ ] **AC5** — 🔴 **Braço contrário:** todos os blocos de gate do acervo que hoje passam linha a linha
  continuam passando, medido sobre o acervo real
- [ ] **AC6** — `make quality` e CI verdes

## Negative scope

- ❌ Não muda o interpretador (`sh` via `$PATH`, ADR-2026-09-01) nem o trust check.
- ❌ Não reabre a contenção de reentrada do #485.
- ❌ Não corrige gates do acervo que já funcionam: só os que o AC5 medir quebrados, se houver.

## Linked ADR
ADR: docs/adr/ADR-2026-09-01-gate-de-wave-e-contrato-portavel-em-shell-posix-nao-script-do-sistema-operacional.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-01-o-barrier-executa-cada-linha-do-bloco-de-gates-como-sh-c-separado-e-o-contrato-nao-diz.md

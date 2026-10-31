---
status: Accepted
date: 2026-10-31
author: "zeus-tf"
---

# ADR: o ml_scope do roadmap-alvo cobre as waves já iniciadas, não só as waves 0..N

> Date: 2026-10-31 | Status: Accepted

Emenda: `ADR-2026-10-23-regras-de-escrita-dos-agentes-sao-aplicadas-pelo-harness-e-nao-pelo-texto-do-handoff.md`
(D2, "o escopo da wave N é a união dos MLs das waves 0..N") e
`ADR-2026-10-29-barrier-ml-scope-le-a-declaracao-em-lista-e-une-os-roadmaps-tocados-pela-branch.md`
(D2, "o alvo do barrier contribui com as waves 0..N").

## Context

O check `ml_scope` (`checkMLScope`, `internal/commands/barrier.go`) compara o diff da branch inteira
(`merge-base(origin/main, HEAD)` mais a árvore de trabalho) com os `Files affected` das waves 0..N do
roadmap-alvo. Enquanto a wave N é a última commitada, o veredito está certo. Quando a wave N+1 é
commitada, os arquivos dela aparecem no diff e não no escopo da wave N, e `barrier --wave N` passa a
reprovar uma wave já auditada sem que nada nela tenha mudado (issue #641, medido no consumidor CMDB
com 11.2.1: das waves 0 a 3, só a 3 passa).

Na mesma função, os outros roadmaps tocados pela branch (membros da união) já contribuem com as waves
0..K, em que K é a wave mais alta com algum ML iniciado, concluído ou cancelado (`mlScopeComputeK`).

## Decision

1. **O roadmap-alvo contribui com os `Files affected` das waves 0..max(N, K)**, em que K é calculado
   pelo mesmo `mlScopeComputeK` usado para os membros da união, sobre o conteúdo de `HEAD`.
2. **A exigência de declaração continua só nas waves 0..N.** Um ML de wave > N sem `Files affected`
   não reprova o barrier da wave N; ele é cobrado no barrier da própria wave.
3. **Nada mais muda:** diff, leitura do roadmap em `HEAD`, conjunto de orquestração, rejeição de
   padrão sem segmento literal, corte de data e regras da união.

## Consequences

- Positivas: o veredito de uma wave auditada fica estável quando as waves seguintes entram, e volta a
  servir como registro reauditável. Arquivo fora de qualquer ML continua reprovando. Trabalho
  adiantado de wave que ainda não começou (todos os MLs ⬜) continua reprovando.
- Negativas: depois que a wave N+1 começa, a reauditoria da wave N não distingue mais se um arquivo da
  N+1 foi tocado pelos commits da N. Essa atribuição exigiria olhar commit por commit.

## Alternatives Considered

- **Todas as waves do alvo (opção 1 do issue).** Também estabiliza o veredito, mas deixa passar
  trabalho adiantado de wave ainda não iniciada. Rejeitada.
- **Restringir o diff aos commits da wave N (opção 2 do issue).** Depende de convenção de mensagem de
  commit, que o barrier não controla. Rejeitada.

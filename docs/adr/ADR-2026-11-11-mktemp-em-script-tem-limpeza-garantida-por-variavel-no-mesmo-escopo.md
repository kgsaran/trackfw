---
status: Accepted
date: 2026-11-11
author: "Zeus (trackfw_architect)"
---

# ADR: mktemp em script tem limpeza garantida por variável no mesmo escopo

> Date: 2026-11-11 | Status: Accepted

## Context

A REQ-2026-11-07 (#671, #674) corrigiu traps EXIT que citavam variável `local`. Depois do merge, o relato #676
(Lourival, medido por efeito no Windows) mostrou outra causa no mesmo arquivo. Em
`scripts/check-shim-diff-applies.sh`, `plumb_repo` e `script_copy_dir` (linhas 490–491) só são apagados por um
`rm -rf` no fim do caminho normal (linha 670). Uma falha entre 491 e 670, sob `set -euo pipefail`, deixa os dois
diretórios para trás. A linha 158 (`tmpfile=$(mktemp)`) tem o mesmo defeito.

Corrigir o trap que cita `local` não fecha o trap ausente, e vice-versa: são causas diferentes. O gate do #674
(`check-no-trap-local-var.sh`) não enxerga esta classe por construção, porque não há trap nenhum para casar.

O relator varreu `scripts/` em `main` (93 `mktemp` atribuídos a variável) e mediu três discriminantes:

1. **Contagem de `mktemp` contra contagem de `trap` por arquivo.** Mente nas duas direções: um trap depois de
   `;` na mesma linha não é contado, e a palavra `mktemp` aparece em listas de utilitários.
2. **"Existe trap no mesmo escopo de função".** 5 sem trap. É falso negativo justamente no caso do #676:
   `run_self_test` tem um trap (linha 179), mas ele limpa `$q`, não `plumb_repo`.
3. **"Algum trap do escopo cita esta variável".** 17 não cobertos: 11 da classe e 6 falsos positivos, por três
   mecanismos:
   - **apelido:** o conserto do #674 guarda o valor em `q` (`printf -v q '%q' "$WORK"; trap "rm -rf -- $q" EXIT`),
     e o nome `WORK` não aparece no trap;
   - **expansão com padrão:** `"${FENCE_TMP:-}"`;
   - **contenção:** `mktemp -d "$WORK/…"` é limpo pelo trap que remove `$WORK`.

## Decision

**D1 — A regra.** Todo `var=$(mktemp …)` em script versionado (`git ls-files '*.sh'`) tem limpeza garantida no
mesmo escopo, e "garantida" significa disparar em qualquer saída: um `trap … EXIT` (ou uma função chamada pelo
trap) que remove o caminho. Um `rm` no fim do caminho normal **não** conta como limpeza garantida.

**D2 — O discriminante é por variável, com os três mecanismos legítimos do relato.** Uma variável V criada por
`mktemp` está coberta quando um trap do escopo de V, ou de um escopo que o envolve:

- cita V em qualquer forma de expansão: `$V`, `${V}`, `${V:-}`, `${V:-x}`;
- cita um **apelido** de V: uma variável cujo valor foi derivado de V no mesmo escopo, antes do trap
  (`printf -v A '%q' "$V"`, `A=$V`, `A="$V"`);
- chama uma função que satisfaz uma das condições acima (um nível de indireção);
- ou V foi criada **dentro** de um diretório coberto (`mktemp … "$W/…"`, `mktemp -p "$W"`, `TMPDIR="$W"`), com W
  coberto.

O gate implementa exatamente isso. Formas não cobertas pelo discriminante (indireção com mais de um nível,
limpeza por caminho literal ou glob) não são "provavelmente ok": o gate reprova, e o sítio é reescrito na forma
canônica.

**D3 — Forma canônica de correção.** `V=$(mktemp …)` seguido, na linha seguinte, do trap que captura o valor (o
mesmo padrão do #674). Quando a função já tem um trap EXIT, o novo trap **substitui** o anterior, então ele
precisa limpar tudo o que o anterior limpava. Combinar num trap só, ou criar o novo caminho sob o diretório já
coberto (contenção), é a correção preferida.

**D4 — O registro do AC1 da REQ-2026-11-07 é corrigido.** O critério está marcado `[x]` com a cláusula "nenhum
diretório temporário sobrando", que o #676 falsifica. A falha foi forçada antes das linhas 490–491, então a
cláusula passou pela posição do teste, não pelo desenho. A REQ ganha uma nota de correção apontando para esta
ADR. O `[x]` não é desmarcado, porque a parte do trap com `local` continua verdadeira.

## Consequences

- A classe fecha nos 11 sítios medidos e em qualquer sítio novo, que o gate reprova.
- O gate tem custo de manutenção: um analisador de escopo e apelido em shell. Ele herda os limites declarados
  pelo relator (análise estática: "coberto" não prova que o trap é alcançável). A guarda de vacuidade e a
  medição contra o corpus real mitigam isso.
- **Residual declarado:** um trap substituído mais adiante por outro `trap … EXIT` na mesma função faz a cobertura
  estática mentir. O gate verifica que o trap que cobre V é o último `trap … EXIT` instalado antes do fim do
  escopo; se não conseguir decidir, reprova.

## Alternatives Considered

- **Corrigir só os 3 sítios do arquivo do #676.** Rejeitada: a Regra Dura de Causa Raiz manda tratar a classe, e
  já existem 8 sítios medidos fora do arquivo.
- **Discriminante por presença de trap no escopo.** Rejeitada: falso negativo medido no próprio caso do #676.
- **Exigir o nome da variável dentro do trap.** Rejeitada: reprova o conserto do #674 em 3 sítios (o apelido `q`).

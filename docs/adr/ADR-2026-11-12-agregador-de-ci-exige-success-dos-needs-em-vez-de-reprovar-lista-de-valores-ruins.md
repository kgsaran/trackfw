---
status: Accepted
date: 2026-11-12
author: "Zeus (trackfw_architect)"
---

# ADR: Agregador de CI exige success dos needs em vez de reprovar lista de valores ruins

> Date: 2026-11-12 | Status: Accepted

## Context

Discussion #664, comentário de 2026-11-06 (Lourival, fork `lourivalgarciajunior/trackfw`): num job de matriz
abandonado por falta de runner, `needs.<job>.result` chegou como **`abandoned`**. Esse valor não está entre os
quatro documentados pelo GitHub (`success`, `failure`, `cancelled`, `skipped`).

Os dois agregadores de `.github/workflows/quality.yml` avaliam esse resultado de formas opostas:

| job | guarda | com `abandoned` |
|---|---|---|
| `shim-byte-identity-gate` (linha ~968) | exige `"$SHIM_RESULT" == "success"` (evidência positiva) | reprova |
| `parity` (linha ~1010) | reprova se `contains(needs.*.result, …)` for `failure`, `cancelled` ou `skipped` (lista de valores ruins) | **o passo de guarda não dispara** |

O `parity` roda com `if: always()`. O comentário dele já diz que `always()` sozinho faz o job passar quando um
`needs` não foi verificado, e que o primeiro passo existe "para RODAR e REPROVAR, nunca para mascarar". A lista
de valores ruins quebra essa promessa diante de qualquer valor que ninguém previu. Ainda não está medido se a
segunda linha de defesa (o conjunto de artefatos por shard, baixado logo depois) segura o caso. O comentário do
ML-4E (2026-09-08), sobre o gate do shim, já registrava: "aqui a guarda precisa exigir evidência POSITIVA, não
só ausência de falha".

## Decision

**D1 — Lista de permitidos, nunca de proibidos.** Todo job agregador que roda com `if: always()` e decide o
veredito a partir de `needs.*.result` aprova somente quando cada resultado pertence a uma lista fechada de
valores aceitos. O padrão dessa lista é `success`. Qualquer outro valor, inclusive um que não esteja
documentado, reprova.

**D2 — A regra fica num script testável fora do CI.** A guarda do `parity` passa a ser
`scripts/check-needs-success.sh`, que recebe `${{ toJSON(needs) }}` por variável de ambiente e reprova se algum
`result` for diferente de `success`, ou se o JSON vier vazio, inválido ou sem nenhum job. O script tem
`--self-test`, com braços para `abandoned`, para um valor inventado, para objeto vazio e para JSON inválido.
Valor que o GitHub nunca produziu no nosso CI não se mede por efeito; a lista de permitidos se mede pelo
self-test.

**D3 — Inventário antes de corrigir.** A Wave 0 lista todo consumidor de `needs.*.result`, de `contains(needs…)`
e de variável `*_RESULT` em `.github/workflows/` e `scripts/`. Pela Regra Dura de Causa Raiz, todo sítio que
usa lista de proibidos num veredito de gate entra nesta REQ. Job de diagnóstico com `continue-on-error`, que não
decide merge, fica de fora, e o motivo é escrito.

**D4 — Sem gate estático novo.** A direção de 2026-10-28 deixa em pausa a criação de mais gates. A regressão é
coberta pelo self-test do D2, ligado no `parity-rest`. Se a Wave 0 encontrar mais de um sítio da classe, a
decisão é reaberta.

## Consequences

- Valor novo de `result` passa a reprovar por construção. O custo é falso vermelho se o GitHub criar um valor
  benigno, e esse é o lado aceito, o mesmo do `shim-byte-identity-gate`.
- O comentário de `quality.yml` que justifica a diferença entre as duas guardas (linhas ~948–955) fica
  desatualizado e é reescrito no mesmo ML.
- Os dois agregadores passam a ter a mesma semântica de evidência. O `shim-byte-identity-gate` mantém a regra de
  dispensa por diff inerte (ADR-2026-11-06), que não muda.

## Alternatives Considered

- **Acrescentar `abandoned` à lista de proibidos.** Rejeitada: conserta o valor de hoje e deixa aberta a
  classe, que é o próximo valor não documentado.
- **Expressão `if:` com `!contains` de cada valor permitido.** Rejeitada: o GitHub Actions não tem `all()`, e a
  expressão não se testa fora do CI.
- **Confiar na segunda linha de defesa (artefatos por shard).** Rejeitada: o `shim-byte-identity-gate` não tem
  essa segunda linha, e uma guarda que só vale por causa da linha seguinte não está cumprindo o próprio
  comentário.

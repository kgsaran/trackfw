---
name: censo-windows-13-rotulos-sao-11-dois-fantasmas-proof
description: Censo 36036473391 — a REQ listou 13 rótulos em FAIL mas são 11; os 2 extras são texto citado dentro de mensagens PROOF de não-vacuidade, e ambos estão OK
metadata:
  type: project
---

Medido em 2026-09-25 (ML-0A da REQ-2026-09-24 do cluster de Windows). A REQ e o roadmap enumeraram
**13 rótulos distintos em `FAIL`** no censo `36036473391`. O censo mede `FAIL=11`, e são **11**.

Os dois fantasmas — e os dois são **rótulos de controle de segurança**:

- `credential-guard-script-integrity/detected` → está **`OK`** (log l. 5134)
- `git-branch-guard-global-script-integrity/detected` → **não existe**; o rótulo real é
  `/detected-without-wiring`, e está **`OK`** (l. 4163)

Entraram na lista porque aparecem como **texto citado dentro de uma mensagem `PROOF …/non-vacuity`**
(l. 4164 e 5135), que ecoa literalmente `"FAIL [falsify/<label>]: saiu com 0, esperava != 0"` para
explicar o que aconteceria com a regra desligada.

**Why:** um filtro que casa o token `FAIL [falsify/` em vez da **forma** (`^FAIL` ancorado) colhe as
duas. O censo faz certo (`grep -ac '^FAIL'`); a enumeração que produziu a REQ, não. É o mesmo defeito
que já produziu **três limites inferiores** consecutivos nesta campanha — aqui produzindo **limite
superior**, e inflando justamente a superfície de segurança: lido como FAIL, diria que a integridade
dos guards não é provada no Windows; medido, os dois estão verdes.

**How to apply:** ao receber uma lista de rótulos de censo num handoff, **reconte antes de aceitar**.
O teste barato que fecha a leitura "seu log veio truncado": contar `^FAIL` por shard e conferir contra
a tabela que o job `apuracao` computa dentro do CI a partir dos artefatos. Se as duas apurações
independentes baterem (aqui: 1·2·1·3·4·0·0·0 = 11), a lista do handoff é que está errada.

Ver também [[forma-b-echo-ok-fora-do-if-falso-verde-check-gates-falsify]] e
[[enumerar-pela-forma-nao-pelo-token]].

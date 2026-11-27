---
name: gate-de-wave-inclui-pacotes-de-contrato
description: gate de wave com só os pacotes tocados deixa passar goldens, winskips e literais do falsify; 3 rodadas de make quality vermelhas no bootstrap (11-26)
metadata:
  type: feedback
---

Quando uma wave muda um asset, um teste com skip de SO ou um literal do guard, o gate da wave inclui:
- `./internal/integrations/` (goldens);
- `./internal/winskips/` (registro de skips);
- uma sonda dos literais de `corrupt_literal` em `scripts/check-gates-falsify.sh` nos arquivos tocados.

**Why:** 2026-11-26, REQ do bootstrap. O gate da Wave 1 rodava só commands/guard/generators/validator. O `make quality` reprovou três vezes seguidas:
1. goldens de `architect.md` desatualizados;
2. 4 skips de Windows sem registro;
3. o literal do s60 sem casar.

E ainda mostrou um defeito real de produto: `rev-parse` com rc=128 era tratado como unborn.

**How to apply:** no handoff, acrescente esses pacotes ao gate sempre que o diff tocar `internal/integrations/assets`, `runtime.GOOS` em teste ou `internal/guard/*.go`. Ao sondar literais do falsify, avalie bloco a bloco e NÃO esconda o stderr: a primeira sonda abortou em erro de sintaxe e disse "nenhum quebrado". Ver [[mudar-contrato-auditar-quem-afirma-o-antigo]], [[o-instrumento-mente]].

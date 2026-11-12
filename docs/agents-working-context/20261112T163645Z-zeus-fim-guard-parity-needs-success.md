# Zeus — fim da implementação da REQ do guard do parity (#664)

- **PR #683** (draft), branch `fix/guard-do-parity-exige-success`: 22 checks verdes. O log do `parity` mostra os
  3 needs avaliados, todos `success` (run 43258143554).
- **Entregue:** `scripts/check-needs-success.sh`, com lista de permitidos, conjunto exato de jobs, `python3 -I` e
  self-test de 33 braços no `parity-rest`, no lugar do `contains(needs.*.result, …)` do `parity`.
- **Auditoria:** ML-1A teve um corretivo (`PYTHONIOENCODING`, gate não rodado pelo executor). O red team (ML-2A)
  achou o NF-1 (`json.py` no cwd aprovava `abandoned`), fechado no ML-2C. `make quality` deu rc=0.
- **Pendente:** AC8, a resposta na Discussion #664. O modo automático negou a escrita duas vezes, e o texto foi
  entregue ao KG. Depois disso: tirar o PR do draft e, após o merge, fechar a REQ e mover o roadmap para `done/`.

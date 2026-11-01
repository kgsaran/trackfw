# Zeus — fim do #649 (PowerShell 5.1 a frio)

- PR #652 mergeado (`644f8b7c`); o #649 foi fechado pelo `Closes`.
- Entregue:
  - matriz de guard com aquecimento por binário e timeout distinguível; no CI de Windows, o `RC0-allow` passou de 2,91–15,12 s para 1,37 s e 1,46 s;
  - no produto, a sonda de política do PowerShell tem prazo de 20 s; a política ilegível gera aviso que não bloqueia; a sonda roda uma vez por validação.
- Achados que viraram ML no mesmo PR:
  - P-1 da Wave 0: mesma causa no `guardGetPSPolicy` (ML-1B);
  - auditoria: o retorno cedo pulava a sonda do Git Bash (correção do ML-1B);
  - MEDIUM-1 do red team: o aviso virava violação (ML-1C);
  - auditoria: `ValidateUnfiltered` estava sem teste.
- Nota de vault: `validator-mensagem-incerta-em-regra-error-vira-violation-e-powershell-a-frio-2026-11-01.md`.
- Pendente: release 11.2.5, a critério do KG (o comportamento do `validate` mudou).

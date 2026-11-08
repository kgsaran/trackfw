# Zeus: fim da Wave 2 da REQ de idioma (leitor bilíngue pronto para o PR 1)

- **Branch:** `feat/idioma-do-produto-ingles-primeiro`. Waves 0, 1 e 2 entregues, exceto o ML-2B (merge e tag, pós-PR).
- **Decisões do KG (2026-11-08):** dois PRs com uma tag no meio. O PR 1 leva o leitor bilíngue (Waves 0–2) e o PR 2 leva o gerador em inglês (Waves 3–4). O KG aceitou como residual a ofuscação deliberada do cabeçalho de gates.
- **Red team:** cinco rodadas. Da 1 à 4, todas reprovaram, e o escopo cresceu dentro da REQ (MLs 2C, 2D, 2F, 2G, 2I, 2J, 2L). A rodada 5 aprovou. O ciclo convergiu quando a ADR ganhou um modelo de ameaça: o parser protege contra acidente, não contra adversário, porque apagar o cabeçalho já é legal. A regra também passou a se ancorar na cerca ligada (`boundGateFence`: a primeira cerca depois do cabeçalho precisa ser ```` ```bash ````).
- **Acervo:** 21 waves de `done/` passam a dar erro de gates. São 14 de D3 (cabeçalho duplicado), 1 de E3 (cabeçalho só mascarado), 5 de quase-cabeçalho e 1 de cerca sem etiqueta. Nenhuma está em `wip/`. O oráculo `scripts/check-marker-corpus-parity.sh` ainda não compara os gates com a `main`, porque a `main` não tem o campo; isso passa a funcionar depois do merge do PR 1.
- **Pendente:**
  - PR 1, quando o KG pedir; depois o merge, `trackfw release tag` minor e o barrier da Wave 2.
  - Waves 3–4 no PR 2. O ML-3C inclui tirar a indentação do modelo de `roadmap.md` em `scaffold.go`.
  - Issue #676 (mktemp sem trap; classe em `scripts/`): REQ própria depois desta branch. O KG já foi avisado no issue.

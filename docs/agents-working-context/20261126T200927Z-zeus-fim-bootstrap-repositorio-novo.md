# Zeus — fim: bootstrap de repositório novo (REQ-2026-11-26)

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** fix/bootstrap-de-repositorio-novo-publicar-branch-padrao-e-init-git
- **Entregue:** D1–D6 da ADR-2026-11-26, agora Accepted.
  - `init` roda `git init` numa pasta vazia.
  - O primeiro commit e o primeiro push da `main` passam pelo trackfw; só são aceitos com remoto vazio e com um
    único commit.
  - O guard nega `gh repo create --push`, orphan e `symbolic-ref`.
- **Wave 2:** o Hades reprovou por D3-bypass: `insteadOf` reescreve a URL de novo no `ls-remote`. Corrigido no
  ML-2C; a nota está no vault.
- **Correções a partir do `make quality`:** ML-2D (goldens do arquiteto e testes rodando no Windows), ML-2E (D2 só
  com rc=1, porque fora de um repo o `rev-parse` dá 128) e ML-2F (literal do s60 no falsify).
- **Gates:** replay do relato no binário da branch: 0 arquivos não rastreados e a `main` publicada. `make quality`
  rc=0, com 365 OK no falsify.
- **Residuais na ADR:**
  - FP do `insteadOf` em cascata;
  - TOCTOU de ref com outro nome;
  - git fora do PreToolUse;
  - alias;
  - aspas do PowerShell.
- **Lição:** o gate de wave precisa incluir os pacotes que fixam contrato, como `integrations` (goldens),
  `winskips` e o falsify. Os três só apareceram no `make quality`.
- **Próximo:** CI; PR quando o KG pedir. Depois do merge, fechar a REQ. Em seguida, a REQ do #707.

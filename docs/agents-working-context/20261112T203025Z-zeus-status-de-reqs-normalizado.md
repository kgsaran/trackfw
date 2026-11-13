# Zeus — triagem das 19 REQs "Other" e normalização de status

- **Triagem:** as 19 REQs "Other" do `trackfw status` são 15 `Superseded` e 4 `Abandoned`, todas em estado
  terminal. Nenhuma está aberta.
- **Corrigido (doc-only, branch `chore/status-de-reqs-normalizado`):** 15 REQs com `done` minúsculo passaram a
  `Done` no frontmatter e no cabeçalho. O cabeçalho da REQ-2026-09-11 (by_agent) dizia `Status: Open` com o
  frontmatter em `Superseded` e foi alinhado.
- **Medido:** 16 arquivos, 31 linhas; nenhum `done` minúsculo restante; `trackfw validate` rc=0.
- **Pendente:** merge do PR.

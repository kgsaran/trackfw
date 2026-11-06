# Zeus — início dos guards do primeiro usuário externo (#662 e #663)

- REQ, roadmap (em `wip/`) e ADR-2026-11-05 criados. Branch `fix/guards-do-primeiro-usuario-externo`.
- Medido com a 11.4.0:
  - CRLF no `trackfw.yaml` faz o `mode: block` virar `warn` (`rc=0`);
  - `bash -c`, `sh -c`, `eval` e `powershell -c` atravessam o guard de git.
- Decisões do KG:
  - padrões de conteúdo ampliados e README corrigido, sem detecção por caminho;
  - guards antes da REQ de CI (#664, #665 e porta fixa).
- Próximo passo: Wave 0 (`hades-tf`).

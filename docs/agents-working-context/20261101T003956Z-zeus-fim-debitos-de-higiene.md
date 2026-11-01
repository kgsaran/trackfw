# zeus-tf — fim dos débitos de higiene

- **Quando:** 2026-10-31.
- **Branch:** `fix/debitos-de-higiene-package-lock-e-validate-script` (pushada; PR não aberto).
- **O que mudou:**
  - `release tag` confere o `npm/package-lock.json` (raiz e `packages[""]`), lendo a chave exata `version`; 4 arquivos, 5 conferências.
  - `trackfw-validate.sh` e hooks do husky gravados por `pathguard.GuardedWrite` com `0755`: bit de execução também em arquivo pré-existente, sem corrida com symlink nem escrita por hardlink.
  - `update harness`: mensagem de `agent_models` global uma vez por execução, só com destino do catálogo (antes, uma por destino).
  - Nota do R-5 no parecer do #633; notas de vault de `GIT_TERMINAL_PROMPT` (medido: sem terminal o git falha em 0 s; nenhuma mudança) e do fixture `followRemoteHEAD never`.
  - Barriers das waves 0–3 verdes; `make quality` verde.
- **Pendente:** PR quando o KG pedir; depois do merge, roadmap para `done` e REQ `Done`; em seguida, a frente do #646.
- **Erros meus:** copiei do relatório a contagem "5 arquivos / 7 conferências" na auditoria do ML-1A sem conferir (o red team pegou); três contra-braços de executor não compilavam ou não foram reportados e foram medidos por mim.

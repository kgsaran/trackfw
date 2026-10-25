# zeus — fim da implementação da #618 (D16), PR aberto (2026-10-25)

- **Mudou:** o validador só reconhece o guard no campo de comando de hook, por esquema de cada CLI. Os 14 chamadores migraram, e o Copilot global usa a chave `bash`. O Windsurf global passou a ser lido.
- **Fechado:**
  - o fail-open da linha do guard num campo `note`, no Windsurf e no Amazon Q;
  - os falsos positivos de `autoMode`, `permissions.allow` e `env`.
- **Medido:**
  - os testes novos reprovam numa cópia de `origin/main`;
  - `make quality` rc=0;
  - o `validate` da branch neste repositório não emite mais o aviso falso (o 11.0.1 emite 1).
- **Reclassificado com medição:** os B1/B2 do red-team (guard em `PostToolUse` e `command` no nível do grupo) dão saída idêntica à de um repositório sem guard. Viraram a #619, a política de presença absoluta em 6 CLIs, que é decisão do KG.
- **Pendente:** o CI, o merge pelo KG, o encerramento e a decisão de versão.

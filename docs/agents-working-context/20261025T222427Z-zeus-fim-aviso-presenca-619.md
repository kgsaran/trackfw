# zeus — fim da implementação da #619 (D17), PR aberto (2026-10-25)

- **Mudou:** o aviso `credential_guard_presence_warning` cobre os 6 CLIs, com uma tabela única por CLI. Windsurf e Amazon Q continuam com violation.
- **Medido:**
  - o contra-braço com binários: 0 avisos em `origin/main` e 3 na branch, com rc=0 nos dois;
  - `make quality` rc=0, com 367 OK;
  - o `validate` deste repositório não emite o aviso.
- **Erros meus registrados:** o critério de aceite do ML-2B foi posto na seção errada, o que bloqueou o barrier, e foi corrigido. O relatório de falsificação do executor ("365 OK") não tinha log.
- **Pendente:** o CI, o merge, o encerramento e a release 11.1.0 (minor).

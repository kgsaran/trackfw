# Zeus — início da REQ da linha de hook sob errexit herdado

- **REQ-2026-11-13** aberta a partir do resíduo R4 da REQ-2026-10-20: a linha de hook PS/POSIX sai 127 (libera)
  sob `bash -e`, `bash -u`, `sh -e`, `sh -u`, `dash -e` e `BASH_ENV` com `set -e`. Medido pelo arquiteto no macOS.
- **ADR-2026-10-04** ganhou o adendo D18: nenhum segmento antes do guard pode encerrar o shell.
- **Fora:** `GIT_TERMINAL_PROMPT`, já refutado no vault (2026-10-31).
- **Próximo:** Wave 0 (hades-tf), branch `fix/linha-de-hook-do-guard-sai-127-sob-errexit-herdado`.

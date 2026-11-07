# Zeus — início da REQ do trap EXIT que cita variável local (#671)

- **Release 11.5.1 publicada** (`trackfw release tag 11.5.1`; `make check-required-full` verde antes).
- **Issue #671** (consumidor externo): reproduzida no bash 5.3 e no bash 3.2; dois sítios em `scripts/`. Rótulo `bug`.
- **REQ-2026-11-07** aberta, roadmap em `wip/`, branch `fix/trap-exit-variavel-local`.
- **Plano:** Wave 0 hades-tf (threat model), Wave 1 ares-tf (dois traps + gate `check-no-trap-local-var.sh` em `parity-rest`), Wave 2 hades-tf (red team).

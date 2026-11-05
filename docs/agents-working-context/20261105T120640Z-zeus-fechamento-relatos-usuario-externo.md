# Zeus — fechamento dos relatos do primeiro usuário externo

- PR #666 mergeado (`8f8ed785`). Fecha os issues #657 a #661.
- Tag `v11.4.0` publicada nos três canais: GitHub (sem marca de pré-release, 9 arquivos), npm (`latest` 11.4.0) e PyPI (11.4.0).
- Cada issue recebeu um comentário com o que mudou e a versão (AC6).
- Roadmap movido para `done/` e REQ marcada como `Done`.
- CI do #666:
  - o ML-2D corrigiu a fixture `initFixture`, que só isolava o `HOME` (no Windows, o `~/.claude` é resolvido pelo `USERPROFILE`);
  - a porta 46202 ocupada no runner foi uma falha isolada do `check-serve-address-parity.sh`, que usa portas fixas.
- Pendente:
  - REQ da Discussion #664, opção 2 (decisão do KG em 2026-11-04): o `shim-byte-identity-gate` dispensa o braço macOS quando o diff não toca as entradas do shim, e o braço continua obrigatório no push para a `main` e na tag;
  - porta livre no `check-serve-address-parity.sh`.

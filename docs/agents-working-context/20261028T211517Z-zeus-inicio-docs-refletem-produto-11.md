# zeus — documentação reflete o produto 11.x, Windows e install.ps1 (2026-10-28)

- **Começou:** auditoria só de leitura (README, VISION, cli-parity, site, npm/pypi, install.sh) contra o código 11.1.0. Achados altos confirmados: tap do Homebrew errado (`kgsaran/tap` → 404), página do PyPI vazia (`build_wheel.py` não grava README), linha de hook do README sem `emulate -R sh` (#595), e o VISION aponta para o namespace `trackfw` no GitHub, que está livre.
- **Decisões do KG:** escopo inclui o site; correção do PyPI no mesmo PR; criar `install.ps1`. Direção do projeto: consolidação + adoção (Discussion #525).
- **Pendente:** ADR, REQ e roadmap; branch; Wave 0 (hades-tf) antes de qualquer implementação.

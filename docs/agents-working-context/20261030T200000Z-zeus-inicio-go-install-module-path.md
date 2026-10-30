# zeus — início: go install e module path sem sufixo de major (#637) (2026-10-30)

- **Começou:** KG reportou que o #637 trava o CMDB. KG escolheu o instalador da tag (ADR-2026-10-30: `go install` não é canal; module path não muda). REQ-2026-10-30, roadmap em `wip/`, branch `fix/go-install-module-path-sufixo-de-major`.
- **Medido:** `go install ...@v11.2.0` → rc=1; `go list -m github.com/kgsaran/trackfw@latest` → `v1.1.0` (o `go install @latest` do README e do site instala a v1.1.0 em silêncio). `install.sh` é asset de cada release.
- **Pendente:** Wave 0 (hades-tf), Wave 1 (apolo-tf gerador ∥ afrodite-tf docs), Wave 2 red team + `make quality`; depois, patch release para o CMDB receber via `trackfw update`.

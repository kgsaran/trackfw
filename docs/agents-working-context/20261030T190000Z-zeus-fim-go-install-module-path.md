### [2026-10-30] zeus-tf — fim: #637, templates de consumidor instalam pelo install.sh da tag

**Branch:** `fix/go-install-module-path-sufixo-de-major`
**REQ:** `docs/req/REQ-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md`
**ADR:** `docs/adr/ADR-2026-10-30-go-install-nao-e-canal-de-instalacao-do-trackfw.md`
**Roadmap:** `docs/roadmaps/wip/ROADMAP-2026-10-30-go-install-entrega-versao-errada-ou-falha-porque-o-module-path-nao-tem-sufixo-de-major.md`

**O que mudou**
- `trackfw-validate.yml` de consumidor: `go install` (rc=1 desde a v2, module path sem `/v11`) trocado por `install.sh` da tag, com `TRACKFW_VERSION` = `version.Version`; job id `governance-go-install` mantido (D3).
- `trackfw-gate.yml` e `.gitlab-ci-trackfw.yml` de consumidor: script baixado de `releases/download/v${TRACKFW_VERSION}/`, não de `releases/latest`.
- README, VISION e site sem `go install @latest` (instalava a v1.1.0 em silêncio).
- Testes: `consumer_install_test.go` (7 testes), `discover_test.go`, `scaffold_doctor_test.go`, goldens; paridade de pin com 27 cenários (eram 21).
- Medido pelo arquiteto: o passo `run:` gerado instala `trackfw 11.2.0` de verdade.

**Waves:** 0 (hades) → 1 (apolo, afrodite) → 2 red team (hades, 4 lacunas de teste) → 3 corretivos (apolo). Barrier das 4 waves verde; `make quality` exit=0.

**Pendente**
- PR com `Closes #637`, só quando o KG pedir.
- Pós-merge: roadmap para `done/`, REQ `Done` (PR de governança) e patch release 11.2.1, para consumidores receberem a correção com `trackfw update`.
- Residual aceito: release assets mutáveis e sem assinatura (`.immutable=false`).

# Zeus — início da REQ do guard do parity que deixa passar resultado de needs não documentado (#664)

- **Origem:** Discussion #664, comentário de 2026-11-06 (Lourival): `needs.*.result` chegou como `abandoned`; o
  primeiro passo do `parity` (`quality.yml:1010`) só reprova `failure`/`cancelled`/`skipped`.
- **Discussions #525 e #594** fechadas como resolvidas (a pedido do KG).
- **ADR-2026-11-12** (lista de permitidos num script testável), **REQ-2026-11-12**, roadmap em `wip/`, branch
  `fix/guard-do-parity-exige-success`.
- **Plano:** Wave 0 hades-tf (inventário e threat model), Wave 1 ares-tf (`check-needs-success.sh` + troca no
  `parity` + `parity-rest`), Wave 2 hades-tf (red team) e ares-tf (`make quality`), Wave 3 Zeus (CI e resposta na #664).

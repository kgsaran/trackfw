---
name: project-serve-binds-all-interfaces
description: FIXED 2026-08-16 — trackfw serve now defaults to loopback in all 3 CLIs, --host is explicit opt-in with warning; verified by ML-2A barrier review
metadata:
  type: project
---

**Status: fixed and independently verified 2026-08-16 (ML-2A barrier review).** All 3 CLIs now
default to `127.0.0.1`; `--host` is opt-in with a byte-identical exposure warning; verified by
`lsof` + `curl` for LAN IPv4 and both IPv6 addresses of the test machine (default bind
unreachable), and by a positive control (`--host 0.0.0.0` reachable as expected). Gate
`scripts/check-serve-address-parity.sh` passes 10/10 independently. No env var or config-file path
(`TRACKFW_HOST`, `trackfw.yaml` `host` key) can set a non-loopback host — the CLI flag is the only
vector, and it doesn't appear anywhere in the repo with a non-loopback value. Full writeup:
appendix to `docs/seguranca/2026-08-16-vazamento-de-stack-no-cli-node.md` ("Apêndice — Barreira
ML-2A"). Residual, non-blocking: stderr warning doesn't protect non-interactive use
(Makefile/Dockerfile/CI) if someone ever commits a non-loopback `--host`; dead code
`internal/server/server.go` (unlinked, confirmed via `go tool nm`) still had the original wildcard
bind pattern — **removed by the architect in ML-3A of this same roadmap**, so do not go looking for
it; `go build ./...` and `make quality` stayed green after the deletion.

Original finding (below), kept for history — the bug this describes is fixed now:

`trackfw serve` (dashboard for ADRs/REQs/roadmaps, `/api/board`, `/api/chain`, `/api/metrics`,
`/api/file`, `/api/attention`) used to bind differently across the 3 CLIs:

- Node (`npm/src/commands/serve.js`): `server.listen(port, '127.0.0.1', ...)` — correct, loopback-only.
- Go (`internal/serve/serve.go`): `addr := fmt.Sprintf(":%d", port)` → binds all interfaces.
- Python (`pypi/trackfw/commands/serve.py`): `HTTPServer(("", port), ...)` → binds all interfaces
  (empty string host = `INADDR_ANY` in `http.server`/`socketserver`).

Confirmed by actually starting the server and curling the LAN IP (`ipconfig getifaddr en0`) — got
HTTP 200 from a non-loopback address on both Go and Python builds.

**Why:** found 2026-08-16 during a security barrier review (ML-2A) of an unrelated fix (handler
global de erro), while sweeping `trackfw serve` as instructed. Not introduced by that fix — a
pre-existing gap, likely from Go/Python `serve` implementations not mirroring Node's explicit
`127.0.0.1` bind. Compounds with a second bug: `pypi/trackfw/commands/serve.py:104` echoes
`str(OSError)` (includes absolute install path) straight into a 500 response body — remotely
observable by anyone on the same network once combined with the bind issue.

**How to apply:** if asked to review or fix `trackfw serve` in any CLI, check the bind address
first — this is the single highest-severity open finding in this codebase as of 2026-08-16 (higher
than the info-leak class that motivated the REQ-2026-08-16-erro-nao-tratado work). Full writeup:
`docs/seguranca/2026-08-16-vazamento-de-stack-no-cli-node.md`. No REQ/roadmap had been opened for
this specific fix as of this writing — verify before assuming it's tracked.

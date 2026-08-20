# `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1` inherited from the `make parity` CI step made `check-ship-force-parity.sh` collapse 3 of 5 scenarios onto the wrong refusal — not a Linux-vs-macOS bug — 2026-08-19

## Contexto

PR #194, job `parity`, run `32314033472`: `check-ship-force-parity.sh` green on macOS locally,
red on Linux CI, with 3 scenarios (`forge-zero-pr`, `forge-unverifiable`, `forge-pr-open-pushes`)
all failing with the SAME stderr — the "no forge CLI is available for this repository" refusal —
even though those scenarios stub `gh` in `PATH` and expect it to be detected. Only `no-forge-cli`
(which expects exactly that refusal) and `remote-advanced-lease-mismatch` (whose refusal fires
before forge detection matters) passed — by coincidence, not because the gate was actually
proving anything about those two paths on Linux.

## Causa raiz — NOT a platform difference

`internal/forge/adapter.go`'s `defaultAvailFn` (mirrored in `npm/src/forge/adapter.js` and
`pypi/trackfw/forge/adapter.py`) treats `gh`/`glab`/`az` as unavailable whenever
`TRACKFW_DISABLE_EXTERNAL_COMMANDS=1` is set, **regardless of PATH** — this is intentional
product behavior (used by `check-ship-parity.sh` to force the no-forge-CLI path deterministically,
and by `make test`/`go test` to keep unit tests hermetic).

`.github/workflows/quality.yml`'s `parity` job sets this env var at the **step level** for the
entire `- run: make parity` invocation — it is inherited by every one of the 15+ scripts
`make parity` runs in sequence, not just the ones that intend it. `check-ship-force-parity.sh`
builds its own `PATH` from scratch specifically so a stubbed `gh` is genuinely detected — but it
never unset the inherited env var, so `defaultAvailFn` short-circuited to "unavailable" before
`exec.LookPath` (or the Node/Python equivalent) even ran, for every scenario, on every runtime.

Locally the gate was invoked directly (`GO_BIN=... bash scripts/check-ship-force-parity.sh`,
no `make parity` wrapper), so the env var was never set and the bug never manifested. In CI it
runs via `make parity` under the step's env, so it manifested — on Linux only because that's
where CI happens to run, not because of anything Linux-specific. Confirmed by reproducing the
exact CI failure text on **macOS** by setting the env var by hand, and separately reproducing it
in an Ubuntu container running the **unfixed** script under the same env var (byte-identical
`FAIL` lines to the CI log, including the pyyaml-missing noise from a minimal container being a
red herring unrelated to the bug).

## Fix

`unset TRACKFW_DISABLE_EXTERNAL_COMMANDS || true` in the gate itself, right after `BASE_PATH` is
built, before any scenario runs. This is the exact fix already present in the sibling gate
`scripts/check-release-tag-parity.sh` (line ~105, same comment) — that gate never hit this bug in
CI because it already carried the fix; it's the discriminator that proves "sibling gate passing in
CI" is not proof of correctness, since it could just as easily have been passing by the same
vacuous-refusal coincidence.

`check-gates-falsify.sh` scenario 73 invokes `check-ship-force-parity.sh` as a subprocess for its
baseline arm (`GO_BIN=... bash scripts/check-ship-force-parity.sh`) — it inherits whatever env
`make parity` was called with too, so under CI's env var it would ALSO have failed (its own
"baseline must be green" guard), which is why the CI failure never even reached
`check-gates-falsify.sh`'s output (Make stops at the first non-zero recipe line). No separate fix
was needed there: once the invoked script unsets the var internally, the inherited leak stops
mattering to any caller.

## Por que importa para outros gates futuros

- **Local-green/CI-red is not proof of a platform bug.** Before assuming Linux vs macOS
  divergence, diff how the gate is invoked locally vs in CI — direct script execution vs through
  `make parity`/`make quality` inherits a materially different environment (env vars set at the
  CI step level). Reproduce by setting the suspect env var locally FIRST; only reach for a
  container if that doesn't reproduce it.
- **Same failure class as
  [[check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08]]**: a gate that stubs/mocks
  an external dependency (CLI, `$HOME`) must actively neutralize ambient environment that could
  make the real dependency detection resolve to the wrong answer for reasons unrelated to the code
  under test. `$HOME` leaks into hook-presence checks; `TRACKFW_DISABLE_EXTERNAL_COMMANDS` leaks
  into forge-CLI-presence checks. Same lesson, different variable.
- **Any new gate that stubs `gh`/`glab`/`az` in `PATH` and asserts a "CLI detected" path must
  `unset TRACKFW_DISABLE_EXTERNAL_COMMANDS` near its `PATH` setup**, mirroring
  `check-release-tag-parity.sh`/`check-ship-force-parity.sh`. Only `check-ship-parity.sh` should
  `export` it deliberately (it wants the no-forge-CLI path forced).
- **A scenario that only asserts a refusal can pass for the wrong reason.** `no-forge-cli` and
  `remote-advanced-lease-mismatch` both "passed" in the broken CI run precisely because they
  expect *a* refusal, not a *specific* one tied to real PATH state — the vacuity was invisible
  without comparing against the scenarios that expect a *different* outcome.

## Referências

- `scripts/check-ship-force-parity.sh` — fix (`unset TRACKFW_DISABLE_EXTERNAL_COMMANDS || true`,
  next to `BASE_PATH`)
- `scripts/check-release-tag-parity.sh:102-105` — sibling gate, same fix already present
- `internal/forge/adapter.go:18-29` (`defaultAvailFn`) / `npm/src/forge/adapter.js` /
  `pypi/trackfw/forge/adapter.py` — the env-var-gated availability check
- `.github/workflows/quality.yml` — `parity` job, `env: TRACKFW_DISABLE_EXTERNAL_COMMANDS: "1"` on
  the `make parity` step
- `Makefile:18-45` (`parity` target) — sequential recipe, stops at first failing line
- `scripts/check-gates-falsify.sh:6696-6737` (scenario 73) — subprocess caller unaffected by this
  fix once the callee unsets the var internally
- [[check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08]] — same failure class,
  different leaked variable (`$HOME` vs `TRACKFW_DISABLE_EXTERNAL_COMMANDS`)
- `docs/roadmaps/wip/ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md`
  (ML-6A)

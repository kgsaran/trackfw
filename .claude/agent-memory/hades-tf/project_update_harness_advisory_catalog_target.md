---
name: update-harness-advisory-catalog-target
description: update harness emits advisory once per catalog target (no WARN token); fix site is harnessCatalogTarget in update.go; UpdateOptions has no stderr writer; counter-arm requires os.Pipe or injected writer
metadata:
  type: project
---

`trackfw update harness` advisory is emitted by `harnessCatalogTarget` in `internal/generators/update.go:2669-2671` — once per catalog target (`-agents`/`-skills` suffix). With 20 catalog targets, 20 lines appear.

**Why:** `config.ResolveAgentModels("global", home, harnessCwd)` is called inside `harnessCatalogTarget`, which is invoked once per catalog target in the `UpdateHarness` loop.

**How to apply:** When investigating `update harness` advisory flooding, go to `update.go:harnessCatalogTarget`, not `agents_models.go`. For AC3 fixes, add `ErrWriter io.Writer` to `UpdateOptions` or accept that counter-arm tests need `os.Pipe()` to capture real stderr.

The advisory text is `GlobalAgentModelsNoneMessage` / `GlobalAgentModelsProjectOnlyMessage` / `MalformedGlobalConfigMessage` — never contains "WARN:". `WARN: agent_models.<tier>` only comes from `trackfw agents models` (`agents_models.go:122`).

Fix: extract `ResolveAgentModels` out of `harnessCatalogTarget` to `UpdateHarness` caller. Use `splitHarnessCatalogTargetID(id)` (not reimplemented suffix check) to determine if catalog targets are selected, then compute once. "Move unconditionally to top of UpdateHarness" is WRONG — emits advisory even for `--targets credential-guard-script` (0 catalog targets, 0 advisory lines measured).

Report text goes to stdout (`cmd.OutOrStdout()`), only advisories go to stderr.

---
name: config-load-singleton-reset
description: config.Load() is a singleton — tests that set ci:/hooks: in fixture must call config.Reset() before RunScaffoldDoctor, or cfg.CI comes from a prior test's cached read
metadata:
  type: feedback
---

`config.Load()` (internal/config/config.go) uses `sync.Once` — it reads `trackfw.yaml` ONCE per process and caches the result. Any test that sets `ci: github-actions` in a fixture and then calls `RunScaffoldDoctor` (which calls `loadUpdateConfig()` → `config.Load()`) will get the CACHED singleton from a prior test's `trackfw.yaml`, not the fixture's.

**Why:** this burned me in ML-1A: two tests (`BothMissing` and `GateYmlStale`) passed in isolation but failed in the full suite because a prior test had cached `CI == ""`. No error — just wrong cfg.CI.

**How to apply:** any test that:
1. writes a trackfw.yaml with config keys (ci:, hooks:, backend:, etc.)
2. then calls RunScaffoldDoctor(), loadUpdateConfig(), or config.Load()

must call `config.Reset()` before the call AND register `t.Cleanup(config.Reset)` to avoid polluting subsequent tests. Import: `github.com/kgsaran/trackfw/internal/config`.

```go
config.Reset()
t.Cleanup(config.Reset)
findings, err := RunScaffoldDoctor(dir)
```

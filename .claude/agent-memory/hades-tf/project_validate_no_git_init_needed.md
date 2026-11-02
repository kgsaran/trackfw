---
name: project-validate-no-git-init-needed
description: trackfw validate works in a plain directory (no git repo needed); also does not use HOME
metadata:
  type: project
---

Measured 2026-11-02 (feat/primeira-experiencia, $SCRATCH/qs-final2 — plain dir, no .git):

- `HOME=fake trackfw validate` → rc=0, 1 warning (write_scope_not_wired), 0 violations.
- validate does not require `git init`.
- validate does not read from HOME (diff=0 between real HOME and fake HOME).
- `trackfw init` DOES write to HOME (30 files: 1 manifest + 12 agents + 17 skills).

**How to apply:** AC4 Quick Start test needs `t.Setenv("HOME", t.TempDir())` only for the `init` step, not for `validate`. The project dir needs `trackfw.yaml` (created by `init`) but no git repo.

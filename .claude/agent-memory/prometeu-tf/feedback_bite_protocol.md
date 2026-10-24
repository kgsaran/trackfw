---
name: bite-protocol
description: How to correctly set up bite (falsification) tests for ML corretivos in trackfw
metadata:
  type: feedback
---

Each bite requires a saved copy of the pre-fix file before any edits. The canonical bite for an enumeration corretivo (ML-4F pattern):

1. `rsync` the worktree (exclude `.git`) to a bite directory in the scratchpad.
2. Overwrite the relevant source files in the bite with `git show HEAD:<path>` — the test files stay as-is (they contain the new tests).
3. Run `go test -run TestYAMLScalarEnumerationAllRunes -v` in the bite directory.
4. The test must show the expected rune failures (confirming the bite has teeth).

For structured-rep fixes (ML-4D pattern), the three canonical bites are:

1. HEAD plan.go (no handling) → SkipBlockKeepValid FAILS (block is present in artifact)
2. v1 plan.go (return nil, error) → StaleRegistryDoesNotBreakOtherTargets FAILS (error aborting plan)
3. HEAD commands file (no check) → commands test FAILS (different error, not the representation refusal)

**Why:** Restore each file variant in turn, run only the target test, then restore the fixed version before the next bite.

**How to apply:** Always capture `GOMODCACHE` and `GOCACHE` before running `go test` with a custom `HOME` — integrationCommandFixture changes HOME for isolation, which breaks module downloads if GOMODCACHE is also moved.

**ML-4F measurement:** yaml.v3 ACCEPTS `￾` and `￿` escape sequences in double-quoted YAML scalars. Hades reported they were rejected — that was only for RAW bytes, not for the escaped form. When fixing U+FFFE/U+FFFF: the policy is to REJECT them (YAML 1.2 §5.1 c-printable exclusion), but the error message must NOT claim "no valid YAML representation" since `￾` is technically accepted by yaml.v3.

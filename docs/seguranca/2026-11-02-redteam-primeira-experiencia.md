# Red Team — Wave 3: Primeira Experiência

> Reviewer: Hades (hades-tf)
> Date: 2026-11-02
> Branch: feat/primeira-experiencia
> Binary: 11.2.5 (built from branch HEAD in scratchpad)
> REQ: REQ-2026-11-02-primeira-experiencia-guard-em-ingles-sem-referencia-privada-req-new-vincula-adr-e-quick-start-passa-de-ponta-a-ponta.md
> Wave 0: docs/seguranca/2026-11-02-wave0-primeira-experiencia.md (T1–T11, P1–P8)

---

## 1. Scope and Method

Wave 3 reimplements the Wave 0 falsification targets independently against the diff. Every
T/P item was verified by running the branch binary against constructed inputs, not by reading
code and inferring behavior. The session did not run `trackfw barrier`, `make quality`, or
`go test ./...` (full suite); it used targeted `go test ./internal/commands/ -run <name>` and
direct binary invocations.

Quick Start sequence: followed the README verbatim in an isolated `t.TempDir()` equivalent
(`$SCRATCH/qs-redteam`, `HOME=$SCRATCH/qs-home`), using the branch binary.

---

## 2. Falsification Target Results

### T1 — Newline injection via --adr

**CLOSED.**
Measured with an actual LF byte, confirmed via `od -c`:
```
arg=$(printf 'ADR-x\nfoo')
od -c → 0000000    A   D   R   -   x  \n   f   o   o
```
`validateADRFlagValue` rejects with:
```
Error: --adr: invalid character '\n' at position 5 in "ADR-x\nfoo"
  (path separators, control characters, quotes and backticks are not allowed)
```
exit=1, no file written. CR (`\r`) also confirmed rejected at position 5.

### T2 — Path traversal via slash in --adr value

**CLOSED.**
`/` rejected by the character-by-character loop. Tested:
`--adr "../../etc/passwd"` → exit=1, no file.
`--adr "foo/../../../etc/passwd"` → exit=1, no file (slash rejected before path evaluation).

### T2c — Traversal via ".."

**CLOSED.**
Explicit `if v == ".."` guard upstream of the character loop. exit=1, no file.

### T3 — Ambiguous ADR match

**CLOSED.**
`resolveADRByArg` returns "ambiguous — found in multiple locations" when `len(matches) > 1`.
Tested by adding a second entry in `adr_dirs` pointing to a directory containing an identically
named ADR file. exit=1, no file created.

### T4 — Absolute path written into REQ adr: field

**CLOSED.**
`adrPortablePath` uses `filepath.Rel` against the project root (EvalSymlinks applied to parent
only). Written REQ contains `docs/adr/ADR-...-user-authentication-strategy.md`, not
`/Users/...`. Verified by reading the generated file directly.

### T5 — Case variant (lowercase filename)

**CLOSED by spec.** `resolveADRByArg` does exact string match: `base == arg || base == arg+".md"`.
Lowercase variant returns "not found," exit=1. Documented Wave 0 P6 / AC3 behavior.

### T6 / AC3b — req new overwrites existing REQ

**CLOSED.** Three sub-cases verified:

| Input | Outcome |
|---|---|
| Same title, same `--adr` | File kept byte-identical; stderr "REQ ... already exists — kept as is (not rewritten)"; exit=0 |
| Same title, different `--adr` | exit=1, file unchanged, error message |
| Same title, existing `adr: ""`, `--adr` provided | exit=1, file unchanged |

Relink path (with `--no-roadmap` omitted): diff against pre-existing file shows exactly 2 changed
lines (`roadmap:` in frontmatter, `Roadmap:` in body). All other content including appended prose
preserved.

### T7 — Orientation removed by English translation

**CLOSED.**
All 12 constants with `trackfw`-alternative references still contain them. All 17 constants that
had "Nada antes" now contain "Nothing before this command was executed."
Tested guard outputs for: `git push`, `git checkout -b`, `git commit`, `git stash`,
`git reset --hard`, `git add .`, `git clean -f`. All emit English with no `§` or Portuguese.

### T8 — Fixture logic lines changed alongside REASON= lines

**CLOSED.**
`git diff origin/main -- internal/generators/testdata/guard-sh-reference/git-branch-guard.sh`
filtered for non-REASON lines returns 0 lines. SHA256 pin in `guard_parity_helper_test.go`
(`75b71725732b504cf58b1e9317cfe158b9d1a82ab9c281551ecf7ab6b057c78a`) matches actual file.

### T9 — Three § sites in non-test command output

**CLOSED.**
`internal/commands/branch_prune.go` Long: no `§` in emitted text.
`internal/commands/agents_models.go` Long: no `§` in emitted text.
`internal/commands/commit.go:343`: "direct commit on %q is not permitted. Use 'trackfw branch
new <type>/<slug>' first." — no `§`.
Residual: `§` in Go comments at branch_prune.go lines 47, 122, 394 — not user-facing, not in
BasicLit scope. Out of AC2 scope.

### T10 — Symlink to external ADR written as absolute path

**CLOSED.**
`adrPortablePath` resolves symlinks on the parent directory only; writes logical name
`docs/adr/ADR-...-external-via-symlink.md`, not the external target.

### T11 — HOME isolation in quickstart_e2e_test.go

**CLOSED on macOS/Linux. PARTIAL on Windows.**
`quickstartSetup` uses `t.Setenv("HOME", t.TempDir())` and `t.Setenv("XDG_CONFIG_HOME", ...)`.
`init.go:81,435` call `homedir.Dir()` (HOME-first); `config.ExpandPath` also uses `homedir.Dir()`.
On macOS/Linux this is sufficient.

Two Windows gaps (Items 6 and 8):
- `init.go:473` calls `agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)`. On Windows,
  `OSCLIHomeResolver.HomeFor(CLIClaude)` returns `effectiveUserHomeDir()` = `os.UserHomeDir()`
  (USERPROFILE), not `$HOME`. The test (`TestQuickStartEndToEnd`) does not set `USERPROFILE`.
  Measured on macOS: init wrote 29 files to `$SCRATCH/qs-home/.claude/` (12 agent `.md` files +
  17 skill `SKILL.md` files). The `.trackfw/integrations-manifest.json` is written via
  `homedir.Dir()` and stays isolated on all platforms. On Windows without USERPROFILE isolation,
  those 29 `.claude/` files land in the real `%USERPROFILE%\.claude\`.
- `req.go:368` (new to this branch) uses `os.UserHomeDir()` in the global-ADR fallback.

---

## 3. Positive Target Results

### P1 — Four additional § sites in ML-1A

**CLOSED.** commit.go, branch_prune.go Long, agents_models.go Long all cleaned.

### P2 — Three test layers (AC1/AC2)

**CLOSED for gitbranch. Partial for credential and write-scope (see Item 7).**

Layers verified:
- Layer A: `TestReasonOrientationSubstrings` covers 12 constants for the alternative-command
  substring; `TestReasonNothingBeforePhrase` covers 17 constants for the "Nothing before" phrase
  (the 3 special constants reasonTruncated/reasonNUL/reasonIndecodeable do not carry "Nothing before").
- Layer B: `TestNoPrivateSectionReferenceInternal` — `go/ast BasicLit` scan of all non-test `.go`
  under `internal/`, catches `§` in string literals including credential.go and writescope.go.
  Portuguese in comments is correctly excluded by BasicLit.
- Layer C-guard: `TestReasonNoPortugueseStopwords` — covers all 20 reason constants including
  `reasonAddImplicit` (item 13) and `reasonWorktreeAddImplicit` (item 20).
- Layer C-commands: `TestBranchPruneLong_NoPrivateSectionReference`,
  `TestAgentModelsLong_NoPrivateSectionReference`.

Portuguese stopword grep across all `internal/guard/*.go` (non-test): only comments contain
Portuguese — credential.go:32, writescope.go:523/548/605/614 are all comment lines, not string
literals. The `%v` deny paths at writescope.go:122,138,170,238 emit English error text (confirmed
by reading `extractStringField`, `extractToolInputPath`, `buildFrozenList`, and
`loadWriteScopeLease` — all return English errors; `readWriteScopeFromYAML` wraps
`config.ParseWriteScopeConfig` errors — grep of `internal/config/*.go` for Portuguese
stopwords returned no hits). Credential match names ("JWT", "AWS access key") are English
(defined inline in credential.go:94,96,209,211). Current state clean. No regression test
covers future Portuguese additions to credential.go or writescope.go user-facing strings beyond
`§` (see Item 7).

**permissionDecisionReason:** Only used in `gitbranch.go:193`. Write-scope and credential do not
emit it.

### P3 — generators/req.go updated

**CLOSED.** `FrontmatterADR string` field added to `REQContent`. Template writes `adr: "<path>"`
when set, `adr: ""` when empty. Body `ADR:` line written with full portable path.

### P4 — Portable path

**CLOSED.** `adrPortablePath` normalizes via `EvalSymlinks` on parent directory, then
`filepath.ToSlash`. Tested on macOS where `/var` is a symlink to `/private/var`.

### P5 — Validate before write

**CLOSED.** `validateADRFlagValue` called before any file creation. Rejects: `/`, `\`, `\x00`,
`\n`, `\r`, `"`, backtick, `..`. Space accepted.

### P6 — Error on ambiguous or not found

**CLOSED.** Both "not found" and "ambiguous" return exit=1, no file written.

### P7 — t.Setenv HOME isolation

**PARTIAL.** Correct on macOS/Linux. Two Windows gaps in T11 (Items 6 and 8).

### P8 — Fixture diff restricted to REASON= lines

**CLOSED.** See T8 above.

---

## 4. Quick Start as New User (Live Run)

Ran all 5 README steps with branch binary in `$SCRATCH/qs-redteam`, isolated HOME:

```
1. trackfw init --ai-tools claude --forge github          → exit=0
2. trackfw adr new "User Authentication Strategy"         → created docs/adr/ADR-2026-11-02-...md
3. trackfw req new "User Authentication" \
      --adr ADR-2026-11-02-user-authentication-strategy.md → created REQ + roadmap, exit=0
4. trackfw roadmap move ROADMAP-2026-11-02-...md wip      → moved, synced REQ, exit=0
5. trackfw validate                                        → "✓ No violations found." exit=0
```

Guard output for `git push` (wired via PreToolUse): English, no `§`, no Portuguese:
```
trackfw: raw git push blocked. Use `trackfw push`...
Nothing before this command was executed...
```

Credential guard (AWS key pattern in payload): English, exit=0 (warn mode — correct by spec):
```
trackfw-credential-guard: warning - possible AWS access key detected in tool payload.
```

Write-scope guard: All user-facing deny strings verified English-only. With no lease configured
(state of QS project after `trackfw init`), guard returns exit=0 for any absolute path.
This is the documented fail-open (`writescope.go:202`: "misses the lease and falls through to
D3-only, which is a fail-open for D4"). The README first screen does not claim write-scope
containment.

---

## 5. AC4b — Identity Default

`TestDefaultIdentitySelectionIsNeutral` PASS: `defaultIdentitySelection()` returns `"neutral"`.
`TestIdentitySelectDefaultCursorIsNeutral` PASS: `huh.Select.Hovered()` returns `"neutral"`
without TTY or user input. Both in `internal/commands/identity_wizard_default_test.go`.

---

## 6. Global ADR via ~/  — Item 8 measurement

`req new --adr <name>` with `~/.trackfw/adr` in `adr_dirs` (isolated HOME): exit=0, REQ
written with `adr: "~/.trackfw/adr/ADR-2026-11-02-global-auth.md"`. `validate --json`:
violations=0. `req_has_adr` accepts the `~/` reference. On macOS, `os.UserHomeDir()` and
`homedir.Dir()` both return `$HOME`, so the defect in `req.go:368` is latent (Item 8 is LOW;
no live defect on macOS/Linux).

---

## 7. Findings

### MEDIUM — README Quick Start fails with currently published binary (Item 1)

**Evidence:** Running step 3 with `/opt/homebrew/bin/trackfw` (v11.2.5, the current release):
```
Error: unknown flag: --adr
```
`--adr` does not exist in 11.2.5. The branch binary also reports `11.2.5` in `--version`; the
distinguishing signal is presence of `--adr` in `trackfw req new --help`.

Every user who installs from npm, pip, or Homebrew gets this failure at step 3 until a new
release ships.

**Disposition:** Conditional blocker on merging without a simultaneous release:
(a) release before or simultaneously with merging, OR
(b) add a minimum-version callout ("requires trackfw ≥ <next release>").
Owned by `trackfw_architect`.

---

### LOW — AC3b error message teaches data loss (Item 5)

**Evidence:** `internal/generators/req.go:195`:
```
"REQ %s already exists and the --adr link was not applied (existing adr: %q,
 requested: %q — use a different title or remove the existing file)"
```
"remove the existing file" is the exact anti-pattern AC3b was designed to prevent.

No test currently pins this message text (`grep -rn 'remove the existing file' internal/`
returns only `req.go:195`).

**Fix:** Change the guidance to: "edit the `adr:` frontmatter field and the `ADR:` body line to
the requested path and rerun." Add a test that asserts the new message.

**Disposition:** Product code — owned by implementing agent.

---

### LOW — Test date basename not pinned; warnings not asserted (Item 2)

**Evidence:** `internal/commands/quickstart_e2e_test.go` (`TestQuickStartEndToEnd`) discovers
ADR and roadmap names via `filepath.Glob(...)[0]`. The README uses
`ADR-$(date +%F)-user-authentication-strategy.md` literally. The test passes if any ADR glob
matches, even if the name diverges from the README command.

`validate` result is consumed as `violations, _, err` — warnings are discarded.

The home-divergence rule (`validateHomeDivergenceWindows`) returns nil on macOS/Linux by
construction (gated on `divergent`, which is always false outside Windows). However, on Windows
a temp HOME against the real USERPROFILE makes the rule fire, so `len(warnings)==0` would fail
on Windows until Item 6 (USERPROFILE isolation) is in place. The `warnings==0` fix therefore
requires Item 6 as a prerequisite on Windows.

**Fix:**
- Compute expected basename: `"ADR-" + time.Now().Format("2006-01-02") + "-user-authentication-strategy.md"`.
- Assert `len(warnings) == 0` after validate (safe on macOS/Linux immediately; requires Item 6 on Windows).

**Disposition:** Product code — owned by implementing agent.

---

### LOW — AC2 regression gap: no Portuguese stopword test for credential and write-scope (Item 7)

**Evidence:** `TestReasonNoPortugueseStopwords` covers only the 20 gitbranch `reason*` constants.
Layer B catches `§` in all `internal/` string literals. No test covers Portuguese stopwords
in `internal/guard/credential.go` or `internal/guard/writescope.go`. A future edit that adds
Portuguese to a user-facing string (without `§`) in these files passes all tests.

Point-in-time grep (`internal/guard/*.go` non-test) shows current state clean: Portuguese words
found are in comments only (credential.go:32, writescope.go:523/548/605/614). The `%v` deny
paths at writescope.go:122,138,170,238 emit English errors (all producing functions confirmed).
Wave 0 confirms clean at analysis time (Varredura D). Wave 0 does not declare this as a residual.

**Fix:** Extend the stopwords check to user-facing string literals in credential.go and
writescope.go (a sibling test or widened Layer C table).

**Disposition:** Product code — owned by implementing agent.

---

### LOW — agentconfig writes to real %USERPROFILE%\.claude\ on Windows (Item 6)

**Evidence:** `init.go:473` calls `agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)`.
`OSCLIHomeResolver.HomeFor(CLIClaude)` on Windows routes to `effectiveUserHomeDir()` =
`os.UserHomeDir()`, not `homedir.Dir()`. This is by design (ADR-2026-10-14 D1: Claude Code on
Windows reads `USERPROFILE`, and the writer/auditor must agree). On macOS, this same path
resolved to `qs-home/.claude/` (measured: 29 files written there — 12 agents + 17 skills; the
integrations-manifest.json goes via `homedir.Dir()` and stays isolated). On Windows without
USERPROFILE isolation, those 29 `.claude/` files land in the real `%USERPROFILE%\.claude\`.
`Manager.Install(plans, false)` with a fresh destination creates new files; for a developer
machine where these files exist and are modified, it errors ("artifact is modified; use force to
replace it"), making the test fail. Neither case silently overwrites customized agents. Item 6
stays LOW.

`TestQuickStartEndToEnd` does not set `USERPROFILE`. The `agentconfig` package has
`SetPlatformForTest` but it overrides the package-level resolver, which is the mechanism that
agrees with Claude Code's reading path by design. The correct fix follows P7's prescription:
isolate at the environment level so both `os.UserHomeDir()` and `homedir.Dir()` return the temp
directory on Windows.

**Fix:** Add to `quickstartSetup`:
```go
if runtime.GOOS == "windows" {
    t.Setenv("USERPROFILE", homeDir)
}
```
This makes `os.UserHomeDir()` return `homeDir` on Windows, closing the agentconfig path, the
test-side exposure of `req.go:368`, and the home-divergence warning coupling — all at once.
Note: Item 6's fix sets `USERPROFILE == HOME` in the test, so `TestQuickStartEndToEnd` can no
longer exercise the HOME≠USERPROFILE case for Item 8. Item 8's production fix (`homedir.Dir()`)
therefore needs its own separate test vector to remain covered.

**Disposition:** Product code — owned by implementing agent.

---

### LOW — adrPortablePath uses os.UserHomeDir() instead of homedir.Dir() (Item 8)

**Evidence:** `internal/commands/req.go:368` (new to this branch, confirmed via `git diff
origin/main -- internal/commands/req.go | grep UserHomeDir`):
```go
home, _ := os.UserHomeDir()
```
This is in the `adrPortablePath` fallback for global ADRs outside the project root. The
`internal/homedir/homedir.go` package doc states: "Dir() replaces os.UserHomeDir() throughout
trackfw's production code." `config.ExpandPath` (used by `ResolveADRFiles` to expand `~/`) uses
`homedir.Dir()`. This new code violates the single-point rule.

Measured live on macOS with an isolated HOME, a `~/.trackfw/adr` entry in `adr_dirs`, and a
valid global ADR: `req new --adr <name>` exits 0; `validate --json` returns violations=0
(`req_has_adr` satisfied). On macOS `os.UserHomeDir()` and `homedir.Dir()` return the same path,
so the defect is latent. On Windows with HOME ≠ USERPROFILE and both on the same volume,
`filepath.Rel(USERPROFILE, absHitCanon)` returns a `..`-prefixed path rather than an error;
exit 1 follows because line 362 guards `!strings.HasPrefix(relHome, ".."+sep)`.
(On different volumes, `filepath.Rel` itself returns an error.) Either way the command exits 1
with "ADR is outside both the project root and HOME" — inferred from code reading; no Windows
run performed.

**Fix:** Replace `os.UserHomeDir()` at `req.go:368` with `homedir.Dir()`.

**Disposition:** Product code — owned by implementing agent.
Note: the cause here (new production code bypassing `homedir.Dir()`) is distinct from Item 6
(test not isolating `USERPROFILE` per P7). Item 6 and Item 8 require separate fixes.

---

### LOW — R4 residual: two constants absent from bash fixture (Item 4)

**Evidence (pre-existing, Wave 0 R4):**
The fixture has 18 entries: 3 `_TRACKFW_*_REASON` variables (reasonTruncated, reasonNUL,
reasonIndecodeable) + 15 `REASON=` lines, covering 18 of the 20 Go constants. The 2 absent are
`reasonAddImplicit` and `reasonWorktreeAddImplicit`. SHA256 pin catches mutations to the 18
present entries only.

Layer C-guard covers both absent constants (items 13 and 20 in the 20-constant table). Layer A
also covers them via substring assertions. No new action required.

---

### LOW — Space in ADR name causes req_has_adr false positive (documented residual)

P5 accepts space. `extractRefPath` reads only the first whitespace-delimited token of `adr:`.
Documented in `docs/cli-parity.md`. Workaround: use hyphens. No new action.

---

### INFO — CRLF residual in AC3b byte-identical guarantee

`rewriteREQRoadmapRefWith` calls `NormalizeCRLF`. CRLF-encoded REQs that trigger a relink write
have line endings normalized to LF. "Byte-identical" applies to the non-relink path only.
Documented in `docs/cli-parity.md`. No new action.

---

### INFO — Credential guard exit=0 is correct by spec

`RunCredential` returns 0 for `warn` mode (default). AC1 prescribes English, not exit code.
README first screen: "flag credentials" — warn, not block. No action.

---

### INFO — Write-scope fail-open with no lease: documented behavior

Guard returns exit=0 when no lease is configured (`writescope.go:202`: "misses the lease and
falls through to D3-only, which is a fail-open for D4"). README first screen does not claim
write-scope containment. No action.

---

## 8. AC Coverage Table

| AC | Measured by | Result |
|---|---|---|
| AC1 — Guard messages in English | Live binary: git push, stash, reset --hard, add ., clean -f; Layer A/B/C targeted | PASS |
| AC2 — No private reference (`§`, PT) | Live binary; Layer B go/ast; Layer C-guard; credential/write-scope point-in-time grep | PASS — regression gap LOW Item 7 |
| AC3 — req new --adr | Live binary: T1–T6 verified; QS run; global-ADR measured | PASS on macOS — LOW Item 8 latent on Windows |
| AC3b — req new idempotent | Live binary: 3 sub-cases; relink diff | PASS — LOW Item 5 (error message) |
| AC4 — Quick Start end-to-end | Live binary: 5-step QS, validate exit=0 | PASS on macOS — MEDIUM Item 1; LOW Items 2, 6 |
| AC4b — Identity default neutral | `TestDefaultIdentitySelectionIsNeutral` + `TestIdentitySelectDefaultCursorIsNeutral`: PASS | PASS |
| AC5 — README first screen | Not measured (visual check, architect scope) | NOT MEASURED |
| AC6 — GIF | Not measured (visual check, architect scope) | NOT MEASURED |

---

## 9. Not Measured / Inference Scope

- TTY interactive wizard path for `--adr` (no TTY in scratchpad; non-TTY path verified).
- AC5 and AC6: visual checks owned by architect.
- Items 6 and 8 on Windows: behavior inferred from macOS measurement and code reading. No
  Windows run was performed. The `filepath.Rel` claim for Item 8 is inferred from reading
  `req.go:360–376` (confirmed lines); the `..`-prefix guard at line 362 produces exit 1 on
  same-volume Windows; cross-volume behavior is a secondary path.

**Timezone measurement (T2-date — advisor-requested, measured 2026-11-02):**
Both `adr new` and `req new` use `time.Now().Format("2006-01-02")` without `.UTC()`. The
README uses `$(date +%F)`. Both respect `TZ`. Tested with `TZ=Pacific/Kiritimati` (UTC+14),
where local date was 2026-11-03 vs UTC 2026-11-02:
- `adr new` produced `ADR-2026-11-03-...` (local date).
- `req new --adr ADR-$(date +%F)-...` with same `TZ` expanded to `ADR-2026-11-03-...`.
- `validate`: "No violations found." exit=0.
**Result: CLOSED.** UTC-vs-local mismatch does not occur because both the binary and the shell
command use the same local time source. The only real risk is a midnight boundary between the
two README commands, which is benign and pre-existing.

---

## 10. Verdict Summary

| Item | Severity | Owner |
|---|---|---|
| README Quick Start fails with 11.2.5 | MEDIUM | trackfw_architect — conditional blocker on merge-without-release |
| AC3b error message teaches data loss | LOW | implementing agent — internal/generators/req.go:195 |
| Test date basename not pinned; warnings not asserted | LOW | implementing agent — quickstart_e2e_test.go |
| AC2 no stopword test for credential/write-scope | LOW | implementing agent — internal/guard/*_test.go |
| agentconfig writes to real %USERPROFILE%\.claude\ on Windows | LOW | implementing agent — quickstart_e2e_test.go (USERPROFILE) |
| adrPortablePath uses os.UserHomeDir() not homedir.Dir() | LOW | implementing agent — internal/commands/req.go:368 |
| R4: two constants absent from bash fixture | LOW | pre-existing, Wave 0 R4 |
| Space in ADR name → req_has_adr FP | LOW | pre-existing, documented |
| CRLF residual in AC3b guarantee | INFO | documented |
| Credential guard exit=0 by default | INFO | correct by spec |
| Write-scope fail-open with no lease | INFO | documented behavior |

T1–T10: CLOSED. T11: CLOSED on macOS/Linux; PARTIAL on Windows (Items 6 and 8).
P1–P6, P8: CLOSED. P7: PARTIAL (same Windows gaps).

No CRITICAL or HIGH findings. The one MEDIUM is a release-sequencing issue; the branch code is
correct, the release timing is not. Five LOWs require code changes (Items 2, 5, 6, 7, 8); all
are product code owned by the implementing agent.

---

*Hades, Security Reviewer — 2026-11-02*

# CLI parity contract

Go is the behavioral reference. Node.js and Python must expose the same public
commands unless an exception is listed below.

Supported runtimes: Go 1.25+, Node.js 18+, and Python 3.10+.

| Command | Go | Node.js | Python | Contract |
|---|---:|---:|---:|---|
| `init` | yes | yes | yes | Creates governance structure and `trackfw.yaml`; `--identity-preset` selects an agent identity preset |
| `adr` | yes | yes | yes | `new`, `list` |
| `req` | yes | yes | yes | `new`, `list` |
| `roadmap` | yes | yes | yes | `new`, `move`, `list`, `show` |
| `validate` | yes | yes | yes | Text and `--json`; nonzero on violations |
| `status` | yes | yes | yes | Governance summary |
| `context` | yes | yes | yes | Markdown/JSON context |
| `log` | yes | yes | yes | Append/read transition log |
| `baseline` | yes | yes | yes | Persist accepted findings |
| `help` | yes | yes | yes | Configuration key documentation |
| `configure` | yes | yes | yes | Generate configuration |
| `discover` | yes | yes | yes | Inspect existing repository |
| `update` | yes | yes | yes | Refresh managed artifacts |
| `metrics` | yes | yes | yes | Delivery metrics |
| `sync` | yes | yes | yes | Jira/Linear synchronization |
| `plugins` | yes | yes | yes | Plugin operations supported by runtime |
| `serve` | yes | yes | yes | Local dashboard |
| `agents` | yes | yes | yes | `list`, `install`, `uninstall`, `update` across supported AI CLIs |
| `skills` | yes | yes | yes | `list`, `install`, `uninstall`, `update` across supported AI CLIs |
| `note` | yes | yes | yes | `new <title>` — creates `vault/notes/<slug>-YYYY-MM-DD.md` and links in `index.md`; idempotent (fails on duplicate) |
| `ship` | yes | yes | yes | Governed `git commit + push + open PR/MR`; hard governance gate (see below) |
| `gemini` / `cursor` / `copilot` / `windsurf` / `amazonq` | yes | no | no | Historical Go-only compatibility aliases |
| `version` / `--version` | yes | yes | yes | Prints `trackfw <version>` |

## Vault de conhecimento

`trackfw init` cria `vault/notes/` e gera `vault/notes/index.md` nos três CLIs.

O comando `note new "<título>"` cria `vault/notes/<slug>-YYYY-MM-DD.md` com frontmatter
(`title`, `tags`, `date`, `related`) e seções `## Problem`, `## Root cause`, `## Solution`.
Após criar o arquivo, acrescenta automaticamente uma linha de link no `index.md`.

Regra de validação `note_orphan` — notas em `vault/notes/` não referenciadas no `index.md`:

| Aspecto | Valor |
|---|---|
| Severidade default | `warning` (não bloqueia `trackfw validate`) |
| Para elevar a error | `rules: { note_orphan: error }` no `trackfw.yaml` |
| Projeto sem `vault/` | nenhum warning gerado |
| `index.md` | não conta como nota órfã |
| Detecção de link | aceita `[texto](arquivo.md)` e `[[nome-da-nota]]` |

## Canonical governance references

REQ frontmatter fields `adr:` and `roadmap:` use the same canonical reference
format in Go, Node.js, and Python: a complete path from the project root,
including the `.md` suffix.

Examples:

```yaml
adr: docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md
roadmap: docs/roadmaps/done/ROADMAP-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md
```

The validator checks the referenced path literally after normal path expansion
such as `~/`. It does not fall back to recursive basename matching. A basename
like `ROADMAP-001.md`, or a path that points to the wrong state directory such
as `docs/roadmaps/wip/X.md` when the file is in `done/`, is invalid even when a
file with the same basename exists elsewhere under `docs/roadmaps/`.

## JSON Schema artifacts

`trackfw init` publishes `docs/schema/adr.schema.json`,
`docs/schema/req.schema.json`, and `docs/schema/roadmap.schema.json` as
cross-runtime helper artifacts for external agents and automation. They describe
the expected frontmatter object after a caller has extracted it from Markdown.

The Go, Node.js, and Python `trackfw validate` implementations do not load or
execute those JSON Schemas automatically. `trackfw validate` remains governed by
the internal validation rules documented in this contract, including
frontmatter presence, folder/status coherence, reference integrity, and
traceability checks.

## Validator `stale_wip` and inspection errors

The Go, Node.js, and Python validators share the same `stale_wip` contract:

- A roadmap's WIP age is measured from its latest transition into `wip/` in
  `docs/roadmaps/.trackfw-log`.
- Valid WIP-entry transitions include any log line for the current roadmap whose
  destination state is `wip`, such as `backlog → wip`, `analyzing → wip`, or
  `blocked → wip`.
- In `roadmap_namespacing: by_agent`, the roadmap identity includes the agent
  prefix exactly as written in the log, for example
  `zeus/ROADMAP-YYYY-MM-DD-<slug>.md`.
- If `.trackfw-log` is absent, or if the current roadmap has no parseable entry
  into `wip`, the backward-compatible fallback is the file `mtime`.
- Git commit time is not part of the cross-runtime contract for WIP age. It
  describes file edit history, not time spent in the WIP state.
- The default stale threshold remains 7 days and the default rule severity
  remains `warning` unless `rules.stale_wip` overrides it.
- Projects may override the threshold with `stale_wip_days` in `trackfw.yaml`.
  Values less than or equal to zero are ignored and fall back to the default.

```yaml
stale_wip_days: 14
rules:
  stale_wip: warning
```

Inspection failures must not degrade silently:

| Condition | Contract |
|---|---|
| Missing optional state directory such as `wip/`, `blocked/`, or `done/` | No finding; missing state directories are treated as empty states. |
| Permission denied, `ENOTDIR`, or walk/list failure for an existing configured directory | Emit a diagnostic for the owning rule, including the path and cause. Severity follows that rule's configured severity. |
| Expected file exists but cannot be `stat`ed or read | Emit a diagnostic for the owning rule and continue inspecting the remaining files. |
| Invalid support file or invalid transition-log line | Emit a diagnostic and use the documented fallback for the affected artifact when available. |

The implemented coverage is intentionally cross-runtime: Go, Node.js, and
Python test the `.trackfw-log` source of truth, configurable boundary behavior,
`mtime` fallback, and `ENOTDIR`/walk-error diagnostics for `wip/`.

## AI integration lifecycle

The Go, Node.js, and Python runtimes expose the same public lifecycle:

```bash
trackfw agents list|install|uninstall|update
trackfw skills list|install|uninstall|update
```

The common flags are `--targets`, `--items`, `--scope`, `--surface`, `--json`,
and, for mutations that may replace or remove content, `--force`. Mutations
without `--targets` open a TTY selector; in non-interactive execution the flag
is required. Supported targets are Claude Code, Codex, Gemini CLI, Antigravity,
Cursor, GitHub Copilot, Windsurf, Amazon Q, and Kiro.

Lifecycle state is one of `not-installed`, `current`, `outdated`, `modified`, or `analyzing`
(a transient state set while the manager reads and hashes an artifact — not user-visible in
normal operation but testable; present in `scaffold.go`, `claudemd.go`, `codex.go`,
`api_board.go` and the validator).
Ownership and SHA-256 are stored per project or global scope. Update and
uninstall preserve modified files unless `--force` is explicit; uninstall never
removes an unmanaged file or a shared artifact that still has another claim.
Legacy surfaces are inspected by `list` and selected explicitly for mutations,
for example `--surface antigravity=legacy-cli`. Known legacy templates can be
adopted safely; unknown content is never adopted by `update`, even with force.

The catalog ships **12 agents** (architect, backend, code-quality, data, dba, frontend, iac,
infra, qa, security, tooling, ux) and **17 skills** (5 process: governance, implement, plan,
release, review; 12 technical: backend-skill, code-quality-skill, data-skill, dba-skill,
frontend-skill, iac-skill, infra-skill, qa-skill, security-skill, tooling-skill, ux-skill, and
vault-skill once added).

**Why technical skills carry a `-skill` suffix:** `internal/integrations/catalog.go` validates
that `id` is unique *across* agents and skills in a single namespace. Because agent ids like
`backend` already exist, a skill with the same id would collide. The `-skill` suffix
(e.g. `backend-skill`) is the chosen disambiguation strategy — do not "fix" it without
understanding this constraint; removing the suffix without renaming or removing the matching
agent would cause a catalog load error at startup.

The standalone `gemini`, `cursor`, `copilot`, `windsurf`, and `amazonq` names
exist only in the Go distribution for historical compatibility. They are not
part of the cross-runtime contract; use `agents` and `skills` in new automation.

## Install scope (`--scope`)

`agents|skills install|update|uninstall`, and `trackfw init`'s AI-tools step,
share one scope-resolution contract across the three runtimes
(ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills):

| Situation | Resolved scope | Notes |
|---|---|---|
| `--scope project` / `--scope global` passed | exactly that value | Detected by *flag-set*, never by comparing the resolved value against `"project"` — `cmd.Flags().Changed("scope")` (Go), `options.scope !== undefined` (Node), `args.scope is not None` (Python). Never prompts. |
| No `--scope`, no TTY, operation is `install` or `update` | `global` (`~/.claude/...`) | Breaking change vs. the pre-ADR default of `project`. |
| No `--scope`, no TTY, operation is `uninstall` | **error** | `"uninstall requires --scope in non-interactive mode"` (D8) — see below. |
| No `--scope`, TTY | interactive select, **`global` pre-selected** | Same wording/options in all three runtimes; fires even when `--targets` was already supplied — it is a gate independent of target/item selection. |
| `list` (any TTY state) | `global` if no `--scope` | Read-only command: never prompts (D6), but keeps the same default so it reports the destinations `install` actually wrote to. |

**D8 — `uninstall` does not inherit the `global` default in non-interactive
mode.** Defaulting a destructive operation to a location the caller never
chose would let a CI script that today cleans up `.claude/agents/` in the
repo start deleting files from the user's home directory instead. In TTY,
`uninstall` prompts exactly like `install`/`update` — the user sees the choice
before anything is destroyed.

**Destination transparency (D5):** before writing anything, in every mutating
command and in `init`'s AI-tools step, the three runtimes print the resolved
destination paths (skipped only for `--json`, which is the deterministic
channel scripts consume instead).

### Internal codex-sync paths fixed to `scope: "project"`

Two call sites bypass the scope gate entirely and hardcode `project`, in all
three runtimes:

- The Codex generator itself — `internal/generators/codex.go:InstallCodex`,
  `npm/src/generators/codex.js:installCodex`,
  `pypi/trackfw/generators/codex.py:install_codex` — writes `AGENTS.md`,
  `.codex/agents/`, and `.codex/config.toml` directly into the repository. It
  never goes through the shared plan/scope machinery in Go or Python
  (Node's `installCodex` happens to reuse `execute()` internally, but always
  with `scope: 'project'` fixed); the `.codex/` directory is inherently
  repo-scoped by Codex's own design, not a user choice.
- `trackfw update`'s "re-sync detected Codex integration" step —
  `internal/generators/update.go:updateDetectedCodexIntegrations`,
  `npm/src/commands/update.js` (the `AGENTS.md`/`.codex` branch), and
  `pypi/trackfw/commands/update.py` — re-applies whatever Codex agent/skill
  artifacts are already installed in the current project. All three runtimes
  fix this to `scope: "project"` for the same reason: it operates on files
  that already live in the repo, not a fresh install a user is choosing a
  destination for.

Neither is a parity gap: all three runtimes agree, and neither is reachable
through the public `--scope` flag.

## Non-interactive `--targets` error message (pre-existing, not part of the
install-scope contract)

`install|update|uninstall` without `--targets` and without a TTY fail with a
message that already diverged before the install-scope ADR:

- Go / Node: `"{operation} requires --targets in non-interactive mode"`
- Python: `"--targets is required for non-interactive {action}"`

Both are asserted by existing tests in all three suites
(`internal/commands/agents_skills_test.go`,
`npm/tests/agents-skills.test.js`, `pypi/tests/test_agents_skills.py`).
Left as-is by ML-2A: unifying the wording is a small, low-risk change, but it
is orthogonal to install-scope reconciliation and would touch pre-existing,
already-asserted strings in an unrelated code path. Tracked here rather than
silently fixed, so a future REQ can pick it up deliberately.

## Non-zero exit codes for integration lifecycle errors

Go and Node exit `1` (the default for cobra/Node's uncaught-throw path) on
integration errors (invalid `--scope`, missing `--targets`, `uninstall`
without `--scope`, etc.). Python's `agents`/`skills` command handler
(`pypi/trackfw/integrations/command.py`) catches
`IntegrationError | OSError | ValueError` and exits `2`
(`raise SystemExit(2) from error`) — a pre-existing Python-CLI convention,
unrelated to and unaffected by the install-scope feature.

## Agent identity

Agent identity is a cross-runtime contract, not a per-distribution feature. The
Go, Node.js, and Python CLIs read the **same** configuration file and must
produce the same artifact bytes for the same input.

### Shared configuration

```
~/.trackfw/identity.json
```

```json
{
  "schema_version": 1,
  "user_nickname": "Kleber",
  "agents": {
    "architect": { "display_name": "Zeus", "slug": "zeus" }
  }
}
```

The file is global — it is **not** mirrored per scope. Identity is a user
preference, not project state, so the same file applies to both `global` and
`project` scopes and to every repository on the machine. `schema_version` must
be `1`; any other value is an error in all three runtimes. A missing file, an
empty `agents` map, or a missing entry for a given agent `id` produces the
current default output **byte for byte** — the feature is opt-in and cannot
regress an existing installation.

Identity is materialized at install time by the render pipeline and written
into the artifact. The agent never reads the configuration at runtime.

The canonical agent `id` (`architect`, `backend`, …) and the installation path
(`trackfw-{{id}}`) are unaffected by identity. Only the `name` and
`description` frontmatter fields and the first line of the body change. The
`name` always carries the fixed `-tf` suffix (`zeus` → `zeus-tf`), which
prevents silent shadowing of a personal agent that happens to use the same
name.

### Slug contract

1. **Preset slugs are hardcoded.** Every themed preset ships an explicit
   `display_name`/`slug` pair. Slugs are never derived at runtime, so the three
   runtimes cannot diverge through differences in Unicode normalization
   (`Ártemis` → `artemis` is a table entry, not a computation).
2. **Dynamic slugification is used only in `custom` mode**, where the user
   types the ten names freely. The algorithm is identical in all three
   runtimes: NFD decomposition + diacritic removal (ASCII-fold), lowercase,
   `[ _]` → `-`, discard of every character outside `[a-z0-9-]`, collapse of
   repeated `-`, trim of leading and trailing `-`.
3. **Invalid input is rejected with an error, never silently corrected.** A
   value that normalizes to the empty string, or that exceeds 40 characters,
   fails. Two agents resolving to the same slug also fail.

### Shared test fixture

The slug vectors live in a single fixture replicated **byte-identically** in
the three packages:

| Runtime | Path |
|---|---|
| Go | `internal/identity/testdata/slug_vectors.json` |
| Node.js | `npm/tests/fixtures/slug_vectors.json` |
| Python | `pypi/tests/fixtures/slug_vectors.json` |

It covers accents, uppercase, spaces, repeated separators, emoji, the empty
string, and the over-length case. Each suite consumes the fixture directly;
adding a vector in one runtime without propagating it is a contract break.

### Parity gate

`scripts/check-identity-parity.sh` is the cross-CLI gate for this contract. It
verifies that the three `slug_vectors.json` copies are byte-identical and that
the three runtimes render the same artifact for the same `identity.json`.
Target/surface coverage is derived from the canonical integration catalog
(`internal/integrations/assets/catalog.json`): every surface whose `agents`
support level is not `unsupported` is exercised, using the default target name
for the default surface and `target=surface` for additional surfaces. This means
a new agent-capable catalog surface enters the gate without editing a manual
target list.
It runs as part of `make quality`, alongside `check-cli-parity.sh` and
`check-integration-assets.sh`.

`scripts/check-gates-falsify.sh` includes the P4 scenario
`identity-parity/catalog-target-missing`, which mutates a temporary catalog copy
to add an uncovered agent-capable surface and requires
`check-identity-parity.sh` to fail with a catalog coverage diagnostic.

### The wizard's UX is also part of the contract

Identity is not only configured by `init` — `trackfw agents install` offers
the same interactive wizard, and the two entry points must feel identical
across the three CLIs. Specifically:

- **Order of steps** — targets → agents → surface → preset or custom → names
  (custom only) → nickname → confirmation → install. Verified directly in
  `runIdentityWizard` in all three CLIs (`internal/commands/identity_wizard.go`,
  `npm/src/commands/identity-wizard.js`,
  `pypi/trackfw/commands/identity_wizard.py`): the nickname prompt runs after
  the preset/custom-names step and before validation and confirmation, in all
  three.
- **Trigger rule** — the wizard appears in `agents install` only when
  `kind == agents` **and** stdin is a TTY **and** (`identity.json` is absent
  **or** `--identity` was passed). `skills install` never triggers it, and a
  non-interactive run never blocks on a prompt.
- **Confirmation screen content** — before any write, the ten
  `specialty → name` pairs plus the nickname, for preset and custom alike;
  declining returns to preset selection without persisting anything.
- **Custom-mode labels** — each field shows `Item.Name` + `Item.Description`
  from the catalog (e.g. `Architect — Architecture, ADRs and governed
  coordination`), never the raw `id`.

Unlike the artifact bytes and the slug algorithm, this UX contract has **no
automated cross-CLI test** yet — `check-identity-parity.sh` validates the
generated artifacts, not the wizard's interactive flow. Parity here is
maintained by review: a change to the wizard's steps, labels, trigger rule,
or confirmation layout in one CLI must be ported to the other two in the same
change.

## `trackfw ship`

`trackfw ship` runs a seven-step governed delivery sequence in all three runtimes:

```
1. Validates branch name — must match feat|fix|refactor/<slug>
2. Validates governance — REQ + roadmap in wip/ or done/ must exist
   (hard gate: not affected by lenient mode or per-rule severity)
3. Detects pending squash-merges in other branches (advisory only)
4. Reviews what is staged (git status --short + git diff --cached --stat)
5. Commits with Conventional Commits format (-m is required)
6. Pushes to origin (adds -u if no upstream is configured yet)
7. Opens PR/MR via the resolved forge CLI, or prints a browser fallback URL if the CLI is absent
```

### Flags

| Flag | Type | Description |
|---|---|---|
| `-m` / `--message` | string | Commit message (Conventional Commits format required) |
| `--dry-run` | bool | Print what would be done without executing write commands; in step 7, also reports forge CLI availability and prints the fallback URL when CLI is absent |
| `--no-pr` | bool | Skip PR/MR creation after push (steps 1–6 still run) |
| `--forge` | string | Override forge detection (`github`, `gitlab`, `bitbucket`, `azure`) |

### Forge resolution and `forge:` field

The resolved forge is printed before step 7:

```
Forge:     github (source: flag)
```

Precedence (highest to lowest):

1. `--forge` flag (source: `flag`)
2. `forge:` key in `trackfw.yaml` (source: `config`)
3. Remote URL pattern (source: `remote`) — `github.com`, `gitlab.com`, `bitbucket.org`, `dev.azure.com`
4. CI file detection (source: `ci`) — `.gitlab-ci.yml` → gitlab; `.github/workflows/` → github
5. Manual (source: `none`) — no forge detected; prints `Open your Pull Request manually at: <remote-url>`

`trackfw discover` and `trackfw init --forge` both write `forge: <value>` to `trackfw.yaml`, enabling
source `config` on subsequent `ship` calls.

### Adapter table

| Forge | CLI | Noun | Fallback URL pattern |
|---|---|---|---|
| `github` | `gh` | Pull Request | `{base}/compare/{branch}?expand=1` |
| `gitlab` | `glab` | Merge Request | `{base}/-/merge_requests/new?merge_request[source_branch]={branch}` |
| `bitbucket` | _(none)_ | Pull Request | `{base}/pull-requests/new?source={branch}` |
| `azure` | `az` | Pull Request | `{base}/pullrequestcreate?sourceRef={branch}` |

`{base}` is the HTTPS URL derived from `git remote get-url origin`, normalised from any
SSH/git@ format. Self-hosted instances are supported: the base URL is extracted from the
remote URL regardless of the host.

### Graceful degradation

When the forge CLI is not available in `$PATH` (or `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1`
is set), step 7 prints the fallback browser URL and exits 0 — it does not fail the
delivery sequence. This behaviour is identical across all three runtimes.

`--dry-run` queries CLI availability at step 7 and prints the same fallback URL when the
CLI is absent, making graceful degradation verifiable without executing a real push:

```
# CLI present
[dry-run] would open Pull Request via github CLI

# CLI absent
[dry-run] Pull Request CLI (gh) not available — would open in browser:
  https://github.com/org/repo/compare/feat/my-feature?expand=1
```

This text is identical in Go, Node.js, and Python.

### Behavioural divergence from `trackfw validate`

`trackfw validate` respects `governance_mode: lenient` (configured in `trackfw.yaml`)
and per-rule severity overrides — in lenient mode, governance violations become warnings
and exit code is 0.

`trackfw ship` does **not** respect lenient mode or per-rule severity. The governance
check in step 2 (`CheckShipGovernance`) is always a hard gate: a branch without a linked
REQ and a roadmap in `wip/` or `done/` **always** aborts ship with exit code 1, regardless of
`governance_mode` or `rules:` configuration.

**Why:** `ship` is a delivery gate, not an audit tool. Lenient mode exists for teams
still ramping up governance — it does not mean "ship without governance artifacts".

**Impact on users:** a team running `governance_mode: lenient` may see `trackfw validate`
pass (exit 0) but `trackfw ship` abort. This is intentional. The error message from step
2 explicitly mentions lenient mode to prevent confusion.

### Usage silencing

Runtime errors (branch pattern, governance gate, nothing staged, missing `-m`) set
`SilenceUsage = true` inside the command handler (Go/cobra) or return a non-zero exit
code directly from the runner function (Node.js/Python), so the usage text is never
printed for runtime errors. Parse-time errors (unknown flags) still show usage, because
they are raised by cobra/commander/argparse before the command handler runs.

## `trackfw barrier`

`trackfw barrier <roadmap> --wave <n>` is the deterministic core of the wave-release barrier.
It is **stack-agnostic**: it never assumes a build tool, a test runner or a parity rule. Every
executable check comes from the roadmap itself. The agent-orchestration layer (specialist
inspections for code quality and security) lives in the `/trackfw:barrier` slash command, never
in the binary.

### Command surface

| Element | Value |
|---|---|
| Invocation | `trackfw barrier <roadmap> --wave <n>` |
| `<roadmap>` | Basename with or without `.md`, resolved against `wip/` then `done/` under `roadmap_dir` (both `flat` and `by_agent` layouts) |
| `--wave` | Integer ≥ 1. Required. |
| `--json` | Emit the result document instead of the text report |
| Exit 0 | `status: "passed"` |
| Exit 1 | `status: "blocked"` — at least one check failed |
| Exit 2 | Usage/resolution error (roadmap not found, wave not found, malformed `--wave`) |

Exit code 2 is **not** `blocked`: a barrier that could not be evaluated is distinct from a
barrier that evaluated to a failure. The three runtimes must agree on this distinction.

**Exit-2 messages must be specific.** The message on `stderr` must name the unresolved entity —
the roadmap basename that could not be found, or the wave number that does not exist in the
roadmap. A generic parser error ("invalid choice", "unknown command") does not satisfy the
contract: it is indistinguishable from the CLI not implementing `barrier` at all, which makes any
exit-2 assertion vacuously true before implementation. This is the exact false positive found
while characterizing the contract in ML-1A; see
`vault/notes/barrier-contract-xfail-false-positive-2026-07-29.md`.

### States

| State | Meaning |
|---|---|
| `pending` | Check declared but not yet evaluated (only ever appears mid-run, and in `--json` when a preceding check aborted the run) |
| `running` | Check currently executing (text output only; never present in a final JSON document) |
| `passed` | Check evaluated and green |
| `blocked` | Check evaluated and red |

The wave-level `status` is `passed` only when **every** check is `passed`; otherwise `blocked`.

### Roadmap parsing rules (string-level — no heuristics)

These are literal parsing rules. All three runtimes must implement them identically.

1. **Wave heading.** A wave starts at a line matching `^## Wave <n> ` (H2, the literal word
   `Wave`, the integer, then a space). The wave ends at the next `^## ` line or EOF.
2. **ML heading.** Inside a wave, an ML starts at a line matching `^### ML-` (H3). The ML ends
   at the next `^### ` or `^## ` line or EOF.
3. **ML completion.** An ML is complete when its body contains a line matching
   `^\*\*Status:\*\*` whose remainder contains `✅`. Any other marker (`⬜`, `🔄`, `❌`) is
   incomplete. Absence of a `**Status:**` line is incomplete.
4. **Acceptance evidence.** Inside an ML, the acceptance block starts at a line matching
   `^\*\*Critérios de aceite:\*\*` and ends at the next `^\*\*` line or at the ML boundary.
   Every line in that block matching `^- \[ \]` is unmet evidence. The ML has evidence only
   when the block exists, is non-empty, and contains zero `- [ ]` lines.
   **An ML with no acceptance block at all is `blocked`, not vacuously passed.**
5. **Wave gates.** Gates are declared per wave by a `**Gates da wave:**` line immediately
   followed by a fenced ```` ```bash ```` block. Each non-empty, non-comment line in that block
   is one gate command, executed from the repository root, in declaration order.
   A wave with no `**Gates da wave:**` block declares zero gates — that is legal and yields a
   `gates` check with `status: "passed"` and an empty `commands` array. The barrier **never**
   invents a gate.
6. **Malformed input.** A wave heading whose number is not parseable, an ML whose body cannot
   be delimited, or an unterminated fence is a usage error (exit 2) with an explicit message
   naming the offending line number — never a silent pass.

### Built-in checks

Evaluated in this fixed order; the run continues through all checks so the report is complete.

| `name` | Passes when |
|---|---|
| `mls_complete` | Wave contains ≥ 1 ML and every ML satisfies rule 3 |
| `acceptance_evidence` | Every ML in the wave satisfies rule 4 |
| `gates` | Every command from rule 5 exits 0 |
| `validate` | `trackfw validate --json` reports `violations: 0` |

`trackfw validate` is invoked in-process (Go/Node/Python each call their own validator), not by
shelling out to a `trackfw` binary that may not be on `PATH`.

### JSON document

```json
{
  "roadmap": "ROADMAP-2026-07-29-example.md",
  "wave": 2,
  "status": "blocked",
  "started_at": "2026-07-29T10:30:00Z",
  "finished_at": "2026-07-29T10:30:04Z",
  "checks": [
    {
      "name": "mls_complete",
      "status": "passed",
      "evidence": ["ML-2A: ✅", "ML-2B: ✅", "ML-2C: ✅"],
      "failures": []
    },
    {
      "name": "acceptance_evidence",
      "status": "blocked",
      "evidence": [],
      "failures": ["ML-2C: 2 unmet acceptance criteria"]
    },
    {
      "name": "gates",
      "status": "passed",
      "commands": ["make quality"],
      "evidence": ["make quality: exit 0"],
      "failures": []
    },
    {
      "name": "validate",
      "status": "passed",
      "evidence": ["0 violations, 0 warnings"],
      "failures": []
    }
  ],
  "failures": ["acceptance_evidence: ML-2C: 2 unmet acceptance criteria"]
}
```

Determinism contract:

- Key order is fixed as shown; `checks` is always in the built-in evaluation order.
- `evidence` and `failures` are always arrays, never `null`, never omitted.
- `commands` is present only on the `gates` check.
- Timestamps are RFC 3339 UTC with second precision.
- The top-level `failures` array is the concatenation of every check's `failures`, each prefixed
  with `<check-name>: `.

### `trackfw barrier` vs `/trackfw:barrier`

| | `trackfw barrier` (CLI) | `/trackfw:barrier` (slash command) |
|---|---|---|
| Nature | Deterministic, reproducible, exit-code driven | Orchestration checklist for `trackfw_architect` |
| Runs gates | Yes, only those declared in the roadmap | Delegates to the CLI |
| Invokes agents | **Never** | Yes — `code-quality` and `security` when applicable |
| Audits the diff | No | Yes, human/agent judgement |
| Git operations | **Never** | Only `trackfw_architect`, after a green barrier |

A green CLI barrier is **necessary but not sufficient** to release a wave. The specialist
inspections and diff audit are conditions the binary cannot evaluate.

## Regra `branch_has_wip_roadmap` — comportamento unificado nos 3 runtimes

A regra verifica que toda branch `feat/`, `fix/` ou `refactor/` possui um roadmap cujo nome
contém o slug da branch. Desde REQ-2026-07-26-robustez-dos-gates-de-governanca-e-paridade, a regra
procura o slug em **`wip/` e `done/`**, não apenas em `wip/`.

| Cenário | Comportamento esperado (Go / Node.js / Python) |
|---|---|
| Roadmap em `wip/` com slug da branch | Sem violação — comportamento original preservado |
| Roadmap em `done/` com slug da branch | Sem violação — permite encerrar o roadmap na própria branch (Definition of Done) |
| Nenhum roadmap em `wip/` nem em `done/` | Violação com mensagem "no roadmap is in wip/ nor done/" + orientação de remediação |
| Roadmap em `done/` com slug **diferente** da branch | Violação com mensagem "no matching roadmap in wip/ nor done/" — casamento de slug é obrigatório |

O casamento é feito por `normalizeBranchSlug(filename).contains(branchSlug)` (substring, não
igualdade), pois nomes de roadmap carregam prefixos de data (`ROADMAP-2026-07-27-<slug>.md`).

A resolução de diretórios (`wip/`, `done/`) é centralizada em `resolveStateDirs` (Go),
`resolveStateDirs` (Node.js) e `_resolve_state_dirs` (Python) — as variantes por agente
(`by_agent`) são suportadas via os mesmos wrappers `resolveWIPDirs`/`resolveDoneDirs`.

O ID da regra (`branch_has_wip_roadmap`) e o mecanismo de severidade configurável (`rules:`) são
preservados — a aceitação de `done/` não altera a config key nem o comportamento de `off`/`warning`.

## Contrato de artefatos gerados (req, adr, roadmap, note)

Os quatro comandos de geração de artefatos produzem arquivos **byte-a-byte idênticos**
nos três runtimes para a mesma entrada. Isso inclui conteúdo e nome de arquivo.

### Frontmatter e formato — contrato explícito

#### `req new <title>`

Arquivo: `docs/req/REQ-YYYY-MM-DD-<slug>.md`

```
---
status: Open
date: YYYY-MM-DD
author: ""
adr: ""
roadmap: ""
---

# REQ: <title>

> Date: YYYY-MM-DD | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 
```

#### `adr new <title>`

Arquivo: `docs/adr/ADR-YYYY-MM-DD-<slug>.md`

```
---
status: Proposed
date: YYYY-MM-DD
author: ""
---

# ADR: <title>

> Date: YYYY-MM-DD | Status: Proposed

## Context
<!-- What is the situation that motivates this decision? -->

## Decision
<!-- What was decided? -->

## Consequences
<!-- What are the positive and negative consequences of this decision? -->

## Alternatives Considered
<!-- What other options were evaluated and why were they rejected? -->
```

#### `roadmap new <title>`

Arquivo: `docs/roadmaps/backlog/ROADMAP-YYYY-MM-DD-<slug>.md`

```
---
status: backlog
date: YYYY-MM-DD
req: ""
squad: ""
---

# Roadmap: <title>

> Created: YYYY-MM-DD | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: 

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — <title>
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes
```

O mesmo frontmatter é obrigatório para roadmaps criados por interfaces de agente,
incluindo o slash-command `/trackfw:roadmap`: `status`, `date`, `req` e `squad`.
O campo `req:` deve receber o caminho relativo completo da REQ selecionada, com
prefixo `docs/req/` e sufixo `.md`; basename solto e link Markdown não são
formato canônico.

Estados válidos para `roadmap move`, `roadmap list`, `roadmap show`, validação e
resolução de paths nos três runtimes:

```
backlog → analyzing → wip → blocked → done → abandoned
```

Ao mover um roadmap para `analyzing`, os três CLIs devem manter pasta,
frontmatter (`status: analyzing`), header (`| Status: analyzing`) e
`docs/roadmaps/.trackfw-log` sincronizados. Em `roadmap_namespacing: by_agent`,
o log preserva o prefixo do agente, por exemplo
`zeus/ROADMAP-YYYY-MM-DD-<slug>.md`.

#### `note new <title>`

Arquivo: `vault/notes/<slug>-YYYY-MM-DD.md` (slug antes da data, inverso do req/adr/roadmap).
Cria ou atualiza `vault/notes/index.md` com uma linha de link no formato `- [<slug>-YYYY-MM-DD](<slug>-YYYY-MM-DD.md)`.

```
---
title: "<title>"
tags: []
date: YYYY-MM-DD
related: []
---

# <title>

## Problem

<!-- Descreva o problema ou situação que motivou esta nota. -->

## Root cause

<!-- Qual foi a causa raiz identificada? -->

## Solution

<!-- Como foi resolvido ou mitigado? O que deve ser feito? -->
```

### Slug — normalização NFKD portável nos três runtimes

Os três runtimes usam a mesma semântica: NFKD decomposition → remoção de
combining marks (diacríticos) → lowercase → substituição de sequências
`[^a-z0-9]+` por hífen → colapso de hífens múltiplos → trim.

| Exemplo de título | Slug gerado (todos os runtimes) |
|---|---|
| `"Autenticação e Sessão"` | `autenticacao-e-sessao` |
| `"ADR Config (v2)"` | `adr-config-v2` |
| `"Minha Requisição #1"` | `minha-requisicao-1` |

Títulos com qualquer combinação de acentos (á é í ó ú), cedilha (ç), til (ã õ),
crase (à) e caracteres não-alfanuméricos produzem slugs idênticos nos três
runtimes. O gate `check-artifact-parity.sh` usa título acentuado
(`"Autenticação e Sessão"`) para validar esse comportamento.

### Data — hora local nos três runtimes

Todos os CLIs usam a data local (`date +%F` / `time.Now().Format("2006-01-02")` /
`datetime.date.today().isoformat()`) — nunca UTC. Geração cruzando meia-noite num
fuso horário avançado pode produzir datas distintas entre runtimes; o gate detecta
essa condição e falha explicitamente.

### Parity gate

`scripts/check-artifact-parity.sh` é o gate transversal que verifica esse contrato.
Para cada tipo de artefato (req, adr, roadmap, slash-command roadmap, note,
vault/notes/index.md), ele invoca os três runtimes com título posicional ASCII,
confirma que exatamente um arquivo foi gerado por runtime (vacuity guard), e faz
diff byte-a-byte acumulando todos os erros antes de sair — o diagnóstico nomeia
o tipo e os runtimes divergentes. O mesmo gate executa um ciclo E2E
`backlog → analyzing` em cada runtime nos layouts `flat` e `by_agent`,
verificando pasta, frontmatter, header, `.trackfw-log` e ausência de
`folder_status` no `validate --json`. Roda como parte de `make quality` (alvo
`parity`), antes de `check-gates-falsify.sh`.

Dois cenários negativos (P4) estão em `scripts/check-gates-falsify.sh`:

- **Cenário 7** — drift de **conteúdo**: corrompe o gerador de req do Node.js para
  emitir `status: OPEN`; asserta exit != 0 com `artifact parity drift: req (go vs node)`.
- **Cenário 8** — drift de **nome de arquivo**: compila binário Go corrompido que usa
  prefixo `RREQ-` em vez de `REQ-`; asserta exit != 0 com `arquivo ausente`.
  Os dois caminhos de comparação (conteúdo e nome) têm provas independentes.
- **Cenário 9** — drift de **slash-command roadmap**: corrompe o gerador de init
  do Node.js para emitir `status: backlogged` no `/trackfw:roadmap`; asserta
  exit != 0 com `artifact parity drift: slash_roadmap (go vs node)`.

## Princípios de design de gates (P1–P4)

Todo gate de paridade e toda regra do validator devem seguir os quatro princípios documentados em
[`docs/gate-design-principles.md`](gate-design-principles.md): nenhum número mágico (P1), falha
explícita sem degradação silenciosa (P2), independência de ambiente (P3) e falsificabilidade
obrigatória (P4). O arquivo inclui os quatro defeitos reais que motivaram os princípios e o
checklist de aceite para gates novos.

A implementação de P4 é `scripts/check-gates-falsify.sh` — todo gate novo de paridade registra
ali sua prova negativa.

## Release rule

Changes to commands, options, exit codes, JSON fields, validation rules, or
generated artifact semantics require equivalent tests in all affected runtimes.

`internal/serve/static` is the canonical dashboard asset source. Copies packaged
by npm and PyPI must remain byte-identical and are checked in CI.

Plugin downloads use timeouts, size limits, and atomic replacement. Registry
entries do not currently publish checksums; adding signed SHA-256 metadata is a
required compatibility change before checksum verification can become mandatory.

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
It runs as part of `make quality`, alongside `check-cli-parity.sh` and
`check-integration-assets.sh`.

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
2. Validates governance — REQ + roadmap in wip/ must exist
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
REQ and a roadmap in `wip/` **always** aborts ship with exit code 1, regardless of
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

### Known divergence — default `roadmap_dir`

The governance gate in step 2 (`CheckShipGovernance`) searches for a REQ-linked roadmap
in `wip/`. The root of the roadmap directory defaults differently per runtime:

| Runtime | Default `roadmap_dir` |
|---|---|
| Go | `docs/roadmaps` |
| Node.js | `docs/roadmaps/claude` |
| Python | `docs/roadmaps/claude` |

This divergence is intentional and preserved. Users who set `roadmap_dir:` in
`trackfw.yaml` override the default and bring all runtimes to the same path. Integration
tests for Node.js and Python place roadmaps in `docs/roadmaps/claude/wip/`; Go tests use
`docs/roadmaps/wip/`.

## Release rule

Changes to commands, options, exit codes, JSON fields, validation rules, or
generated artifact semantics require equivalent tests in all affected runtimes.

`internal/serve/static` is the canonical dashboard asset source. Copies packaged
by npm and PyPI must remain byte-identical and are checked in CI.

Plugin downloads use timeouts, size limits, and atomic replacement. Registry
entries do not currently publish checksums; adding signed SHA-256 metadata is a
required compatibility change before checksum verification can become mandatory.

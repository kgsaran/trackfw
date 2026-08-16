# Commands Reference

Complete reference for all `trackfw` commands.

---

## `trackfw init`

Initializes the governance structure in the current project via interactive wizard.

```bash
trackfw init [--brownfield] [--ai-tools codex,...]
```

### Flags

| Flag | Description |
|------|-------------|
| `--brownfield` | Activates lenient mode for 30 days (violations become warnings) |
| `--ai-tools` | Configures all nine AI targets non-interactively in all three runtimes |

### What gets generated

- `docs/adr/`, `docs/req/`, `docs/roadmaps/{backlog,wip,blocked,done,abandoned}/`
- `trackfw.yaml` — project configuration
- `scripts/trackfw-validate.sh` — validation script for CI
- `CLAUDE.md` — context for Claude Code (if selected)
- `.claude/commands/` — 7 slash commands for Claude Code
- `AGENTS.md`, `.agents/skills/`, `.codex/agents/`, and `.codex/hooks.json` — Codex integration (if selected)
- `.husky/` or `lefthook.yml` — git hooks (if selected)
- `.github/workflows/trackfw.yml` or `.gitlab-ci.yml` (if selected)
- `pom.xml` Spring Boot 3.3 (if backend=java)

### Example

```bash
$ trackfw init
? Project name: my-project
? Project type: fullstack
? Backend language: java
? Backend framework: Spring Boot
? Git hooks: husky
? CI/CD: GitHub Actions
? AI assistants: Claude Code

✓ Governance structure initialized.
```

---

## `trackfw discover`

Scans the repository and automatically detects the existing governance structure (ADRs, REQs,
roadmaps, stack).

```bash
trackfw discover
trackfw discover --init
trackfw discover --bootstrap-log
```

### Flags

| Flag | Description |
|------|-------------|
| `--init` | Generates a `trackfw.yaml` calibrated for the project from what was detected |
| `--bootstrap-log` | Creates a retroactive `.trackfw-log` from files already in `done/` |

---

## `trackfw configure`

Interactive wizard that guides `trackfw.yaml` configuration. Generates a sparse file: only
fields that differ from the defaults are written.

```bash
trackfw configure
```

---

## `trackfw help`

This is **not** the generic `--help` produced by the CLI framework (cobra/commander/argparse) —
that remains separately available on every command (`trackfw --help`, `trackfw <command>
--help`). `trackfw help` is the project's explicit help surface: it lists the available commands
**and** documents `trackfw.yaml` configuration keys.

```bash
trackfw help              # lists commands and every trackfw.yaml key
trackfw help <command>    # help for that command (equivalent to "<command> --help")
trackfw help <key>        # documentation for a trackfw.yaml key (type, default, example, impact)
```

With no argument, the output has two sections: the list of available commands (name + short
description) and a `KEY / DEFAULT / DESCRIPTION` table with every recognized `trackfw.yaml` key
(`adr_dirs`, `roadmap_namespacing`, `wip_limit`, `rules.*`, `trace_id_field`, etc.).

With an argument that matches a configuration key's name, it shows type, default value,
description, a YAML usage example, and the practical impact of changing it. With an argument that
matches a command's name, it shows that command's help. An unknown topic — neither command nor
key — exits with a non-zero code and, when a close-enough name exists, suggests the fix.

---

## `trackfw branch new`

Creates a `feat/`, `fix/`, `refactor/`, `chore/`, or `docs/` branch by moving the
`branch_has_wip_roadmap` governance gate (already enforced by `trackfw validate` and
`trackfw ship`) to **before** branch creation, instead of after.

```bash
trackfw branch new feat/oauth-login
trackfw branch new fix/fix-401 --dry-run
```

### Behavior

| Type | Behavior |
|---|---|
| `feat`, `fix`, `refactor` | Requires a roadmap with a matching slug already in `wip/` or `done/` — without a match, blocks and `git checkout -b` is never executed |
| `chore`, `docs` | Exempt from the roadmap gate — the branch is created normally |

### Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Reports whether the branch would be created or blocked, without executing `git` |

When allowed, runs `git checkout -b <type>/<slug>`, propagating Git's own output and exit status
literally.

### If it blocks

```bash
trackfw req new "title"
trackfw roadmap new "title"
trackfw roadmap move <name> wip
```

---

## `trackfw agents` and `trackfw skills`

Manage specialist agents and governance skills with the same contract in the
Go/Homebrew, npm, and PyPI CLIs.

```bash
trackfw agents list|install|uninstall|update [flags]
trackfw skills list|install|uninstall|update [flags]
```

Supported targets: `claude`, `codex`, `gemini`, `antigravity`, `cursor`,
`copilot`, `windsurf`, `amazonq`, `opencode`, and `kiro`.

### Flags

| Flag | Description |
|---|---|
| `--targets <csv>` | Target CLIs; required for mutations without a TTY |
| `--items <csv>` | Catalog IDs; defaults to all items |
| `--scope project\|global` | Installs in the project or user directory |
| `--surface target=surface` | Selects a specific surface; may be repeated |
| `--json` | Emits catalog and deployments in deterministic format |
| `--force` | Allows replacing/removing modified managed content |

In a TTY, `install`, `update`, and `uninstall` without `--targets` open an
interactive selector. In CI or another non-interactive environment, omitting
`--targets` is an error.

### Examples

```bash
# Lists catalog, native/fallback representation, and state; includes legacy surfaces
trackfw agents list --json

# Installs selected agents and skills in the project
trackfw agents install --targets claude,codex --items architect,backend --scope project
trackfw skills install --targets gemini,kiro --items governance,implement --scope project

# Installs globally and selects the Kiro CLI surface
trackfw agents install --targets kiro --scope global --surface kiro=cli

# Explicitly inspects the old Antigravity surface
trackfw agents list --targets antigravity --surface antigravity=legacy-cli

# Updates or removes only selected deployments
trackfw skills update --targets codex,gemini
trackfw agents uninstall --targets claude --items backend
```

States are `not-installed`, `current`, `outdated`, and `modified`. The
scope-specific `.trackfw/integrations-manifest.json` manifest records ownership,
version, SHA-256, and shared claims. `update` and `uninstall` preserve `modified`
files until `--force` is explicit. Uninstall never removes an unmanaged file or
an artifact that is still shared. A historical installation with a known hash
is adopted without overwrite and reported as `outdated`; `update` migrates it.
Unknown unmanaged content is never adopted by update, even with `--force`.

The identity parity gate is derived from the canonical integration catalog:
every agent-capable surface automatically enters the Go/Node/Python matrix.
Non-default surfaces are exercised as `target=surface`, the same format used by
`--surface`.

The standalone `gemini`, `cursor`, `copilot`, `windsurf`, and `amazonq` commands
have been removed. Use `agents` and `skills` with `--targets` (the target names
above remain valid catalog values there) for all installation and update flows.

`trackfw agents third-party` and `trackfw skills third-party` fetch and install third-party
content under a two-phase quarantine gate — `fetch` downloads into quarantine without installing,
`install` consumes an already-quarantined artifact with checksum-linked approval.

---

## `trackfw update`

Re-applies current trackfw templates to a project already initialized with `trackfw init` or
`trackfw discover --init`. A project-scope operation — it never touches the user's home
directory. Updates the trackfw rules block in agent config files (`CLAUDE.md`, `GEMINI.md`,
etc.), `scripts/trackfw-validate.sh`, the CI workflow, existing Codex agent/skill deployments,
historical Claude slash commands, and Git hooks.

```bash
trackfw update
trackfw update --dry-run --json
trackfw update --targets claude,codex --install-missing
```

### Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Computes and reports target states without writing anything |
| `--install-missing` | Also installs targets currently reported as missing |
| `--targets <csv>` | Comma-separated subset of target IDs (unknown ID is a usage error) |
| `--json` | Emits the result as a JSON document instead of the text report |

### `trackfw update harness`

Re-applies current templates to every already-installed global-scope artifact in the user's
home directory (historical Claude compatibility skill, global Codex agent/skill deployments).
Never requires `trackfw.yaml` and never touches the current repository — that is
`trackfw update`'s job. Accepts the same `--dry-run`, `--json`, `--targets`, and
`--install-missing` flags.

```bash
trackfw update harness
trackfw update harness --install-missing
```

---

## `trackfw adr new`

Creates a new Architecture Decision Record via interactive wizard.

```bash
trackfw adr new
```

### Expected output

```
created docs/adr/ADR-2026-06-13-decision-title.md
```

---

## `trackfw adr list`

Lists all project ADRs with status.

```bash
trackfw adr list
```

### Expected output

```
ADR-2026-06-13-use-postgresql.md         Proposed
ADR-2026-06-10-monolith-architecture.md  Accepted
ADR-2026-06-01-oauth-provider.md         Draft
```

---

## `trackfw req new`

Creates a new requirement via interactive wizard with contextual probes.

```bash
trackfw req new
```

The wizard detects domains (authentication, UI, persistence, API, deploy, events) based on title and motivation and presents domain-specific questions. ADR Drafts are automatically created when an answer indicates a pending architectural decision.

### Expected output

```
created docs/req/REQ-2026-06-13-oauth-login.md
created docs/adr/ADR-2026-06-13-oauth-provider.md (Draft)
```

---

## `trackfw req list`

Lists all requirements with status.

```bash
trackfw req list
```

### Expected output

```
REQ-2026-06-13-oauth-login.md        Open
REQ-2026-06-10-export-report.md      Closed
```

---

## `trackfw req move`

Updates a REQ's status in place (frontmatter and header).

```bash
trackfw req move <partial-name> <status>
```

### Example

```bash
trackfw req move oauth-login Closed
```

---

## `trackfw roadmap new`

Creates a new implementation roadmap.

```bash
trackfw roadmap new [--title "Title"] [--req docs/req/REQ-*.md] [--from-req docs/req/REQ-*.md]
```

### Flags

| Flag | Description |
|------|-------------|
| `--title "Title"` | Sets title without wizard |
| `--req <path>` | Links REQ to roadmap |
| `--from-req <path>` | Creates roadmap already linked to REQ (shorthand) |

### Examples

```bash
# Interactive wizard
trackfw roadmap new

# With title and REQ defined
trackfw roadmap new --title "Implement OAuth" --req docs/req/REQ-2026-06-13-oauth-login.md

# Shorthand
trackfw roadmap new --from-req docs/req/REQ-2026-06-13-oauth-login.md
```

---

## `trackfw roadmap list`

Lists all roadmaps grouped by state.

```bash
trackfw roadmap list
```

### Expected output

```
[backlog]  ROADMAP-2026-06-13-implement-oauth.md
[wip]      ROADMAP-2026-06-10-refactor-db.md
[done]     ROADMAP-2026-06-01-setup-ci.md
```

---

## `trackfw roadmap move`

Moves a roadmap between kanban states.

```bash
trackfw roadmap move <partial-name> <state>
```

Valid states: `backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`

### Example

```bash
trackfw roadmap move oauth wip
# ✓ moved ROADMAP-2026-06-13-implement-oauth.md → docs/roadmaps/wip
```

The transition is automatically logged to `docs/roadmaps/.trackfw-log`. The
`analyzing` state represents the reading, validation, and planning phase before
execution begins in `wip`. The CLI keeps the roadmap folder, `status:`
frontmatter, and `| Status:` header synchronized.

---

## `trackfw roadmap show`

Displays the full content of a roadmap with partial name search.

```bash
trackfw roadmap show <partial-name>
```

### Example

```bash
trackfw roadmap show oauth
```

```
─────────────────────────────────────────
ROADMAP-2026-06-13-implement-oauth.md — [WIP]
─────────────────────────────────────────

---
status: wip
date: 2026-06-13
req: docs/req/REQ-2026-06-13-oauth-login.md
squad: ""
---

# Roadmap: Implement OAuth
...

Location: docs/roadmaps/wip/ROADMAP-2026-06-13-implement-oauth.md
```

---

## `trackfw baseline`

Runs all validations and saves the result as a baseline in `.trackfw-baseline.json`. The
subsequent `trackfw validate` reports only violations that are **new** relative to that
baseline — useful for adopting governance in a brownfield project without having to clear all
existing debt at once.

```bash
trackfw baseline
```

Commit `.trackfw-baseline.json` to the repository to document the accepted debt.

---

## `trackfw validate`

Validates consistency across ADRs, REQs, and Roadmaps.

```bash
trackfw validate
```

### Rules validated

1. WIP roadmaps must have REQ field filled
2. WIP roadmaps must have acceptance criteria
3. Only one roadmap can be in WIP at a time (configurable per squad)
4. Blocked roadmaps must have REQ field filled
5. REQs must have a linked Roadmap
6. ADRs must be referenced in at least one REQ
7. Open REQs cannot be blocked by Draft ADRs
8. WIP roadmaps older than `stale_wip_days` are marked as stale
9. ADRs and REQs must have valid YAML frontmatter

### `stale_wip`

`stale_wip` uses the roadmap's latest transition into `wip/` in
`docs/roadmaps/.trackfw-log`, including `backlog → wip`,
`analyzing → wip`, and `blocked → wip`. In
`roadmap_namespacing: by_agent`, the roadmap identity includes the agent prefix
recorded in the log, for example `zeus/ROADMAP-2026-07-27-example.md`.

If the log is absent or has no parseable entry for the current roadmap, the
backward-compatible fallback is the file `mtime`. Git commit time is not part
of the contract.

```yaml
stale_wip_days: 14 # default: 7
rules:
  stale_wip: warning
```

### Validator I/O Policy

Missing optional state directories such as `wip/`, `blocked/`, or `done/` are
treated as empty. Real inspection failures, such as permission denied,
`ENOTDIR`, walk/list failures, unreadable expected files, or invalid
`.trackfw-log` lines, emit a diagnostic with rule, path, and cause; severity
follows the rule configuration (`off`, `warning`, or `error`).

### Expected output — no violations

```
✓ No violations found.
```

### Expected output — with issues

```
✗ 2 violation(s) found:

  [violation] ROADMAP-2026-06-13-implement-oauth.md missing REQ field
  [violation] REQ-2026-06-13-oauth-login.md is blocked by Draft ADR: ADR-2026-06-13-oauth-provider.md

⚠  1 warning(s):

  [warning] ROADMAP-2026-06-10-refactor-db.md in WIP for 9 days (stale)
```

### Lenient mode (brownfield)

```
[LENIENT MODE until 2026-07-13]

⚠  1 violation treated as warning:
  [warning] ROADMAP-2026-06-13-implement-oauth.md missing REQ field
```

---

## `trackfw status`

Displays an overview of the current project state.

```bash
trackfw status
```

### Expected output

```
trackfw — project status

📋 Backlog       2 roadmaps
🔄 WIP           1 roadmap
⚠  Stale WIP     0 roadmaps
❌ Blocked       0 roadmaps
✅ Done          3 roadmaps

📄 ADRs          4   (Proposed: 2, Accepted: 1, Draft: 1)
📝 REQs          3   (Open: 2, Closed: 1)

⏳ REQs blocked by Draft ADRs:
  REQ-2026-06-13-oauth-login.md → ADR-2026-06-13-oauth-provider.md
```

---

## `trackfw barrier`

Deterministic wave-release barrier. It is **stack-agnostic**: it never assumes a build tool, a
test runner, or a parity rule — every executable check comes from the roadmap itself (the wave's
declared gates) or from `trackfw validate` run in-process.

```bash
trackfw barrier <roadmap> --wave <n> [--json]
```

`<roadmap>` is the basename with or without `.md`, resolved against `wip/` then `done/` under
`roadmap_dir` (including the `by_agent` layout). `--wave` is required.

### Built-in checks

| Check | Passes when |
|---|---|
| `mls_complete` | The wave has at least one ML and every ML is `**Status:** ✅` |
| `acceptance_evidence` | Every ML has a non-empty `**Critérios de aceite:**` block with no `- [ ]` line |
| `gates` | Every command declared under `**Gates da wave:**` exits 0 — a wave with no such block declares zero gates, and the barrier never invents one |
| `validate` | `trackfw validate --json` reports `violations: 0` |

### Exit codes

| Exit | Meaning |
|---|---|
| `0` | `status: "passed"` — every check is green, the wave may release |
| `1` | `status: "blocked"` — at least one check failed; the report (text or `--json`) says which |
| `2` | Usage/resolution error — the roadmap or the wave could not be resolved. This is **not** `blocked`: a barrier that could not run is distinct from one that ran and failed |

### Correction flow

```bash
$ trackfw barrier ROADMAP-example --wave 2
✗ mls_complete: ML-2C: not complete (status: 🔄)
✗ acceptance_evidence: ML-2C: 2 unmet acceptance criteria
wave 2: blocked
```

Fix the roadmap (mark the ML `✅`, close the remaining criteria) and rerun the **same command** —
the barrier is not a permanent denial; a corrected wave passes on the next invocation:

```bash
$ trackfw barrier ROADMAP-example --wave 2
✓ mls_complete
✓ acceptance_evidence
✓ gates
✓ validate
wave 2: passed
```

### JSON output

```bash
trackfw barrier ROADMAP-example --wave 2 --json
```

```json
{
  "roadmap": "ROADMAP-example.md",
  "wave": 2,
  "status": "blocked",
  "started_at": "2026-07-29T10:30:00Z",
  "finished_at": "2026-07-29T10:30:04Z",
  "checks": [
    { "name": "mls_complete", "status": "passed", "evidence": ["ML-2A: ✅"], "failures": [] },
    { "name": "acceptance_evidence", "status": "blocked", "evidence": [], "failures": ["ML-2C: 2 unmet acceptance criteria"] },
    { "name": "gates", "status": "passed", "commands": ["make quality"], "evidence": ["make quality: exit 0"], "failures": [] },
    { "name": "validate", "status": "passed", "evidence": ["0 violations, 0 warnings"], "failures": [] }
  ],
  "failures": ["acceptance_evidence: ML-2C: 2 unmet acceptance criteria"]
}
```

### `trackfw barrier` vs. `/trackfw:barrier`

`trackfw barrier` is the deterministic, reproducible core — it never invokes agents and never
performs Git operations. The `/trackfw:barrier` slash command orchestrates around it: it dispatches
`code-quality`/`security` reviews when applicable, audits the diff against scope, and — only for
`trackfw_architect`, the sole Git authority in the workflow — commits and pushes once every check
and review is green. A green CLI barrier is necessary but not sufficient to release a wave. Full
contract: `docs/cli-parity.md` → `## trackfw barrier`.

---

## `trackfw context`

Emits governance context for LLM and AI agent consumption.

```bash
trackfw context [--format=md|json]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--format` | Output format: `md` or `json` | `md` |

### Example — JSON format

```bash
trackfw context --format=json
```

```json
{
  "project": "my-project",
  "governance_score": 80,
  "adrs": [
    { "file": "ADR-2026-06-13-use-postgresql.md", "status": "Accepted" }
  ],
  "reqs": [
    { "file": "REQ-2026-06-13-oauth-login.md", "status": "Open" }
  ],
  "roadmaps": {
    "wip": ["ROADMAP-2026-06-13-implement-oauth.md"]
  },
  "violations": [],
  "warnings": []
}
```

---

## `trackfw serve`

Starts a local HTTP server with web visualization of the ADR → REQ → ROADMAP chain.

```bash
trackfw serve [--port 8080]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--port` | Server port | `8080` |

Access `http://localhost:8080` to see:
- **Traceability** — ADR → REQ → ROADMAP traceability map
- **Timeline** — state transition timeline
- **Kanban** — visual board of roadmaps by state

---

## `trackfw metrics`

Calculates flow metrics from the transition history (`.trackfw-log`).

```bash
trackfw metrics [--since YYYY-MM-DD] [--export report.csv]
```

### Flags

| Flag | Description |
|------|-------------|
| `--since` | Start date of the period (e.g., `2026-01-01`) |
| `--export` | Export metrics to CSV |

### Metrics calculated

- **Cycle time** — average backlog → done time
- **Throughput** — roadmaps completed per week
- **WIP age** — average time of roadmaps in WIP

### Expected output

```
Metrics (2026-01-01 → 2026-06-13)

Cycle time:    4.2 days (avg)
Throughput:    2.1 roadmaps/week
WIP age:       3 days (avg)
```

---

## `trackfw sync`

Syncs open REQs with external issue tracking tools.

```bash
trackfw sync --to=linear
trackfw sync --to=jira
```

### Flags

| Flag | Description | Values |
|------|-------------|--------|
| `--to` | Sync destination | `linear`, `jira` |

### Configuration — Linear

In `trackfw.yaml` or via environment variables:

```yaml
linear_api_key: "lin_api_..."
linear_team_id: "TEAM_ID"
```

Or:
```bash
export LINEAR_API_KEY="lin_api_..."
export LINEAR_TEAM_ID="TEAM_ID"
```

### Configuration — Jira

```yaml
jira_base_url: "https://company.atlassian.net"
jira_email: "user@company.com"
jira_token: "ATATT..."
jira_project: "PROJ"
```

Or:
```bash
export JIRA_BASE_URL="https://company.atlassian.net"
export JIRA_EMAIL="user@company.com"
export JIRA_TOKEN="ATATT..."
export JIRA_PROJECT="PROJ"
```

### Expected output

```
REQ-2026-06-13-oauth-login.md → LIN-42 (created)
REQ-2026-06-10-export-report.md → already synced (skipped)
```

---

## `trackfw log`

Displays the history of roadmap state transitions.

```bash
trackfw log [--tail N]
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--tail` | Number of entries to display | `20` |

### Expected output

```
Date                 Roadmap                                            From       To
2026-06-13 14:32     ROADMAP-2026-06-13-implement-oauth.md             backlog  → wip
2026-06-12 09:15     ROADMAP-2026-06-10-refactor-db.md                 wip      → done
```

---

## `trackfw note new`

Creates a new knowledge note in `vault/notes/`.

```bash
trackfw note new "<title>"
```

### Expected output

```
created vault/notes/2026-06-13-note-title.md
```

---

## `trackfw changelog`

Reads entries from `CHANGELOG.md` without leaving the terminal.

```bash
trackfw changelog
trackfw changelog --all
trackfw changelog --version 7.0.0
```

### Flags

| Flag | Description |
|------|-------------|
| `--all` | Shows the entire `CHANGELOG.md` |
| `--version <x.y.z>` | Shows only the section for the given version |

Without flags, shows the `[Unreleased]` section (or the most recent one, if there is no
`[Unreleased]`).

---

## `trackfw commit`

The missing intermediate step between raw `git commit` and `trackfw ship`: it commits staged
changes directly, but blocks the commit **before** it happens when governance is missing, instead
of letting it land and only catching it later.

```bash
trackfw commit -m "fix(api): correct 401 on refresh"
trackfw commit --suggest
```

### Behavior by branch

| Branch | Behavior |
|---|---|
| `main`/`master` | Always blocked — committing directly on the default branch is never permitted |
| `feat/`, `fix/`, `refactor/` | Requires a roadmap with a matching slug already in `wip/` or `done/` (same logic as `trackfw branch new` and `trackfw validate`); without a match, blocks with the same governance orientation message |
| Other (e.g. `docs/`, `chore/`) | Allowed without requiring a roadmap — a warning is logged, but the commit proceeds |

### Flags

| Flag | Description |
|------|-------------|
| `-m` / `--message` | Commit message (required, except with `--suggest`) |
| `--suggest` | Prints a heuristic Conventional Commits message skeleton from `git diff --cached --name-status` and exits without committing (ignores `-m`; no LLM call) |

When allowed, runs `git commit -m <message>`, propagating Git's own output and exit status
literally.

### If it blocks

```bash
trackfw req new "title"
trackfw roadmap new "title"
trackfw roadmap move <name> wip
```

---

## `trackfw ship`

Runs a seven-step governed delivery sequence: validates branch, checks governance (REQ + roadmap in wip/), detects pending squash-merges, reviews staged changes, commits, pushes, and opens a PR/MR via the resolved forge CLI (or prints a fallback browser URL when the CLI is absent).

```bash
trackfw ship -m "feat(auth): add login flow"
trackfw ship -m "fix(api): correct 401 on refresh" --dry-run
trackfw ship -m "refactor(core): simplify handler" --no-pr
trackfw ship -m "feat(ui): new dashboard" --forge gitlab
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `-m` / `--message` | string | Commit message (Conventional Commits format required) |
| `--dry-run` | bool | Print what would be done without executing write commands; in step 7, also reports forge CLI availability and prints the fallback URL when absent |
| `--no-pr` | bool | Skip PR/MR creation after push (steps 1–6 still run) |
| `--forge` | string | Override forge detection (`github`, `gitlab`, `bitbucket`, `azure`) |

### Forge detection

The forge is resolved by precedence:

1. `--forge` flag
2. `forge:` field in `trackfw.yaml`
3. Remote URL (e.g. `github.com`, `gitlab.com`, `bitbucket.org`, `dev.azure.com`)
4. CI files (`.gitlab-ci.yml` → gitlab; `.github/workflows/` → github)
5. Manual — no forge detected

The resolved forge and its source are printed before step 7:

```
Forge:     github (source: config)
```

### Example

```bash
$ trackfw ship -m "feat(auth): add OAuth login"
✓ Branch: feat/auth-oauth
✓ Governance: REQ-2026-07-20-oauth.md + roadmap in wip/
✓ No pending squash-merges detected
  staged: auth/handler.go (+120 -5)
✓ Committed: feat(auth): add OAuth login
✓ Pushed to origin/feat/auth-oauth
Forge:     github (source: remote)
✓ Pull Request opened: https://github.com/org/repo/pull/42
```

---

## `trackfw completion`

Generates the trackfw autocompletion script for the given shell.

```bash
trackfw completion bash|fish|powershell|zsh
```

See `trackfw completion <shell> --help` for shell-specific installation instructions.

---

## `trackfw version`

Displays the installed version of trackfw.

```bash
trackfw version
# trackfw v7.0.0
```

# trackfw

> AI governance CLI for software delivery teams — ADR → REQ → ROADMAP → backlog / wip / blocked / done / abandoned

[![Release](https://img.shields.io/github/v/release/kgsaran/trackfw)](https://github.com/kgsaran/trackfw/releases/latest)
[![Go](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go)](go.mod)
[![npm](https://img.shields.io/npm/v/trackfw?logo=npm&color=CB3837)](https://www.npmjs.com/package/trackfw)
[![PyPI](https://img.shields.io/pypi/v/trackfw?logo=python&color=3776AB)](https://pypi.org/project/trackfw/)
[![License](https://img.shields.io/github/license/kgsaran/trackfw)](LICENSE)

**trackfw** is an open-source governance CLI for AI-native software delivery. It enforces a traceable chain from architectural decision to shipped code — without SaaS, accounts, or databases. Markdown files are state.

It is designed for teams looking for an ADR / REQ / ROADMAP governance framework with native support for AI coding assistants such as Codex, Claude Code, Gemini CLI, Antigravity, Cursor, GitHub Copilot, Windsurf, Amazon Q, and Kiro.

```
ADR → REQ → ROADMAP → backlog / wip / blocked / done / abandoned
```

Every piece of work traces back to a decision. Every decision links to a requirement. Every requirement lands in a roadmap. No orphan work, no undocumented choices.

> 🚧 **Platform support: Linux and macOS are supported. Windows support is partial.**
> The CLIs install on Windows and core governance commands run — but the generated
> guard hooks are POSIX shell scripts, and **on several agent CLIs they do not execute
> on Windows.** They are written to disk and reported as installed while never running.
> Native Windows hooks are in progress.
> **Read [Windows support (partial)](#windows-support-partial) before adopting on Windows.**

---

## The problem

Most teams accumulate technical debt not because they lack tools, but because they lack **governance traceability**. Decisions are made in Slack. Requirements live in someone's head. Roadmaps drift from what was actually shipped.

- **ADR tools** manage decision records, but don't connect them to delivery.
- **Kanban tools** track tasks, but don't enforce that tasks are backed by a decision.
- **CI tools** validate code, but don't validate governance.
- **AI coding assistants** generate code at unprecedented speed, but without traceability: who decided what? Why? Which requirement authorized this roadmap?

trackfw adds the governance layer that makes AI-assisted delivery auditable.

trackfw closes the loop — connective tissue between *why*, *what*, and *when*.

---

## Demo

![trackfw demo](docs/demo.gif)

```bash
$ trackfw req new "Login screen"

  ? Describe what you want to build  Login screen for the application
  ? Motivation                       Users need to authenticate to access the system

  Detected domains: authentication, ui

  ? How will users authenticate?
  > Local login (email + password)
    SSO (Google, Azure AD, Okta...)
    Not decided yet  ← generates ADR draft

  ? Is there an existing UI framework or design system?
    Yes, already chosen
  > No, need to choose a UI framework  ← generates ADR draft

ADR drafts created:
  → ADR-2026-06-12-authentication-strategy.md
  → ADR-2026-06-12-ui-framework.md

Resolve these ADRs (set Status: Accepted) before creating a roadmap.
created docs/req/REQ-2026-06-12-login-screen.md
```

---

## Installation

### macOS / Linux — curl

```bash
curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
```

### Homebrew

```bash
brew install kgsaran/tap/trackfw
```

### Go

```bash
go install github.com/kgsaran/trackfw/cmd/trackfw@latest
```

### npm (pure Node.js — no binary)

```bash
npm install -g trackfw
```

The npm package is pure Node.js — no compiled binary or postinstall download.
It installs wherever Node.js ≥ 18 is installed. Shared behavior, including the AI
integration lifecycle, follows the [CLI parity contract](docs/cli-parity.md).

> **The CLI installs on Node ≥ 18 alone — the generated guard hooks do not run on
> Node alone.** They are POSIX shell scripts and need a POSIX shell to execute. On
> Windows, see [Windows support](#windows-support-partial) before relying on them.

### Windows support (partial)

🚧 **Windows support is partial and in progress. Read this before adopting `trackfw`
on Windows.**

We publish a Windows binary and the npm/pip packages install on Windows. That is not
the same as the tool working end to end, and we would rather tell you where the edges
are than let you find them after adoption.

**What we know works**

- The three CLIs install (`npm install -g trackfw`, `pip install trackfw`, and the
  published `trackfw_<version>_windows_amd64.tar.gz`).
- Core governance commands — `req new`, `roadmap new`, `roadmap move`, `status`,
  `validate` — run, and artifacts are written with LF endings.

**What we know does not work yet**

- `scripts/install.sh` **refuses Windows**, even though we publish a Windows binary.
  Install manually from the release archive for now.
- Our Windows CI still reports **known test failures**. They are mapped by root cause,
  not unknown — but they are not zero.
- **Windows ARM64 is not built.** Only `windows_amd64` is published.

**Guard hooks on Windows — measured, per agent CLI**

The guard hooks are `.sh` scripts, executed by *your* AI agent CLI. Which shell that CLI
uses on Windows decides whether they run at all. We measured it:

| Agent CLI | Shell on Windows | Do the hooks run? | Basis |
|---|---|---|---|
| Gemini CLI | PowerShell, always | ❌ **No** | measured in vendor source |
| Codex CLI | PowerShell on the normal path | ❌ **No** | measured in vendor source |
| GitHub Copilot CLI | — | ❌ **No** — we populate the wrong config field | vendor docs + our code |
| Claude Code | Git Bash if installed, else PowerShell | ⚠️ **Only with Git Bash** | vendor documentation |
| Cursor · Kiro | unknown | ❓ **Unknown** | closed, undocumented |

🔴 **This is the failure mode we care most about, because it is silent.** A guard that
never executes still reports health over something it never inspected. On the CLIs marked
❌, `trackfw validate` will tell you the hook is installed — and it will never fire.

**Until native Windows hooks ship, do not rely on `credential_guard` or
`git_branch_guard` as an enforced control on Windows.** Treat them as documentation of
intent, not as enforcement.

We are **not** going to answer this by requiring Git Bash: it would fix one CLI out of
six and push the cost onto you. Windows hooks should run on Windows. Native hook
generation is the direction — see
[`docs/portabilidade/2026-09-05-contrato-de-execucao-de-hook-por-cli-de-agente-no-windows.md`](docs/portabilidade/2026-09-05-contrato-de-execucao-de-hook-por-cli-de-agente-no-windows.md)
for the full measurement, per CLI, with the level of certainty of each row.

**If you are on Windows**, we want your report. The Windows defects fixed so far came
from a user running the tool on real Windows 11 and measuring before reporting — open
an issue with what you measured.

### pip

```bash
pip install trackfw
```

The pip package is pure Python 3.10+ — no compiled binary or postinstall
download. Shared commands, validation rules, and by_agent behavior follow the
[CLI parity contract](docs/cli-parity.md).

---

## Quick start

```bash
# 1. Set up governance in your project (interactive wizard)
trackfw init

# 2. Document an architectural decision
trackfw adr new "Use PostgreSQL as primary database"

# 3. Create a requirement — wizard detects domains and proposes ADR drafts
trackfw req new "User authentication"

# 4. Once ADRs are accepted, plan the work
trackfw roadmap new "Auth service"

# 5. Check governance health
trackfw validate

# 6. See what is in flight
trackfw status

# 7. Governed commit + push + open PR/MR (feat/fix/refactor branches only)
git add -p                        # stage your changes explicitly
trackfw ship -m "feat(auth): add login flow"
```

---

## Commands

| Command | Description |
|---|---|
| `trackfw init` | Interactive wizard — scaffolds governance + AI integrations |
| `trackfw adr new "title"` | Create a new Architecture Decision Record |
| `trackfw adr list` | List all ADRs with status |
| `trackfw req new "title"` | Create a REQ with guided ADR discovery |
| `trackfw req list` | List all REQs with status, discovered across flat, per-state, and by_agent layouts |
| `trackfw req move <name> <status>` | Update a REQ's status; physically relocates the file when it already lives in a recognized state subfolder (see [Multi-agent namespacing](#multi-agent-namespacing)) |
| `trackfw roadmap new "title"` | Create a roadmap in `backlog/` |
| `trackfw roadmap show <name>` | Print a roadmap with its current state |
| `trackfw roadmap move <name> <state>` | Move roadmap between states |
| `trackfw roadmap list` | List all roadmaps grouped by state |
| `trackfw validate` | Check governance consistency (use as CI gate) |
| `trackfw barrier <roadmap> --wave <n>` | Deterministic wave-release gate — stack-agnostic, checks MLs, acceptance evidence, project-declared gates, and governance |
| `trackfw ship -m "msg"` | Governed `git commit + push + open PR/MR` — enforces branch pattern and governance gate; resolves forge (GitHub/GitLab/Bitbucket/Azure) automatically or via `--forge`; falls back to a browser URL when the forge CLI is absent |
| `trackfw context` | Print a structured summary of the project's governance state (REQs, Roadmaps, ADRs with counts and statuses) |
| `trackfw serve` | Start a local governance dashboard (no cloud, no accounts) |
| `trackfw status` | Show wip, blocked, REQs waiting on ADRs |
| `trackfw log [--tail N]` | Show roadmap state transition history |
| `trackfw agents list` | List available agents and deployment state across AI CLIs |
| `trackfw agents install` | Install selected specialist agents |
| `trackfw agents update` | Safely update managed agents |
| `trackfw agents uninstall` | Remove selected owned agent deployments |
| `trackfw skills list` | List available governance skills and deployment state |
| `trackfw skills install/update/uninstall` | Manage selected governance skills |
| `trackfw version` | Print version |

The same lifecycle contract is available from the Go/Homebrew, npm, and PyPI
distributions.

---

## Governance chain

| Layer | Artifact | Purpose |
|---|---|---|
| Decide | `ADR` | Document the *why* behind a technical decision |
| Specify | `REQ` | Define *what* needs to be delivered, linked to an ADR |
| Plan | `ROADMAP` | Break the requirement into microbatches with acceptance criteria |
| Execute | `backlog → wip → done` | Folder position is the source of truth |

### Roadmap states

```
docs/roadmaps/
├── backlog/     queued, not started
├── wip/         actively being worked on (one at a time)
├── blocked/     waiting on a dependency or decision
├── done/        completed and validated
└── abandoned/   discontinued — reason required in file
```

Moving a file between folders **is** the state transition. No database, no API.

---

## AI-native governance

trackfw v2.6.0 introduces features designed for teams where AI agents are first-class contributors.

### Multi-agent namespacing

```yaml
# trackfw.yaml
roadmap_namespacing: by_agent
agents: [claude, gemini, copilot]
```

Artifacts are organized by agent: `docs/roadmaps/claude/wip/`, `docs/req/gemini/done/`. `trackfw validate` and `trackfw context` are fully by_agent-aware — no false positives.

REQs reuse this same `roadmap_namespacing` setting — there is no separate `req_namespacing` key. When
`by_agent` is configured, REQs may live under `req_dir/<agent>/<state>/` just like roadmaps do.

`trackfw req list` and `trackfw req move` discover REQs across three layouts at once (each is a fixed,
non-recursive glob — a REQ nested deeper than these patterns is not found), with no extra flag required:

- **Flat (legacy):** `req_dir/*.md` — REQs loose directly in the REQ directory.
- **Per-state:** `req_dir/<state>/*.md` — organized by state, without an agent segment.
- **By agent:** `req_dir/<agent>/<state>/*.md` — used when `roadmap_namespacing: by_agent` is set.

`trackfw req move <name> <status>` behaves conditionally depending on where the REQ currently lives:

- If the REQ already sits inside a recognized state subfolder (per-state or by_agent layout above), the
  move **physically relocates the file** to the target state's folder, mirroring `trackfw roadmap move` —
  the folder is the source of truth for state. In this mode `<status>` must be one of the six governance
  states (`backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`) — any other value is rejected
  with `invalid state`.
- If the REQ is loose in `req_dir/` (flat legacy layout), the move rewrites the `status:` frontmatter
  field **in place** and does not move or create any folder. `<status>` is written verbatim in this
  mode — it accepts the free-form values existing REQs already use (`Open`, `Done`, ...), not just the
  six governance state names. Existing flat REQs are never migrated automatically — you are not forced
  to reorganize a project's existing REQs to adopt this behavior.

### Bidirectional traceability (`trace_id_field`)

```yaml
# trackfw.yaml
trace_id_field: req_id
```

Automatically verifies the REQ↔ROADMAP link in both directions. Reports 5 check types:
- `traceid_orphan_req` — REQ with no matching ROADMAP
- `traceid_orphan_roadmap` — ROADMAP with no matching REQ
- `traceid_state_mismatch` — REQ and ROADMAP in different states
- `traceid_duplicate_req` / `traceid_duplicate_roadmap` — duplicate trace IDs

### Configurable rules

Every governance rule has configurable severity:

```yaml
# trackfw.yaml
rules:
  req_has_adr:      "error"    # default
  req_has_roadmap:  "warning"  # relax for tactical REQs
  blocked_has_req:  "error"
  wip_limit:        "warning"
  stale_wip:        "warning"
  adr_orphan:       "off"      # silence during migration
```

15+ rules available. Adopt progressively — start with `warning`, tighten to `error` as your team builds the habit.

### Governance gate

```yaml
# trackfw.yaml
governance_mode: strict   # CI fails on any violation
# governance_mode: lenient # CI passes with warnings only
```

### `update` and `sync` configuration fields

`trackfw update` and `trackfw sync` read the following keys, flat at the root of `trackfw.yaml`, through
the same loader used by every other command. All default to an empty string when absent.

```yaml
# trackfw.yaml — consumed by `trackfw update`
hooks: husky          # husky | lefthook | native | "" (no hooks regenerated)
ci: github             # github | gitlab | "" (no CI workflow regenerated)
backend: node          # backend stack — informs CLAUDE.md/agent stack sections and hook commands
frontend: react         # frontend stack — same as backend
pkg_manager: npm        # npm | yarn | pnpm | ... — composes build/test commands in generated hooks

# trackfw.yaml — consumed by `trackfw sync` (checked before the matching env var, same order in all 3 CLIs)
linear_api_key: ""
linear_team_id: ""
jira_base_url: ""
jira_email: ""
jira_token: ""
jira_project: ""
```

Full contract, including per-field consumers and the intentional exception for generated Git hooks
(which read `roadmap_dir` with their own `grep`/`sed`, since they run without the `trackfw` binary
present): `docs/cli-parity.md` → `## trackfw update vs trackfw update harness`.

---

## REQ-driven ADR discovery

When you run `trackfw req new`, the wizard analyzes your intent and asks targeted questions for each detected domain — authentication, UI, persistence, API, deploy, events. Unanswered architectural decisions become ADR drafts automatically.

```
trackfw req new "checkout flow with payment integration"
```

Detected domains: **persistence**, **api**, **events**

Questions asked:
- Which database engine will be used? → *Not decided yet* → `ADR: database-engine (Draft)`
- Which API protocol will be used? → *REST (already decided)* → no ADR
- Which event broker will be used? → *Not decided yet* → `ADR: event-broker (Draft)`

The REQ is linked to its blocking ADRs. `trackfw validate` enforces that no roadmap is created until every linked ADR reaches `Accepted` status.

This is the difference between experienced architects (who know which decisions to make) and everyone else — trackfw brings the architectural checklist to the requirement.

---

## `trackfw validate` — governance gate

```bash
$ trackfw validate

✗ REQ-2026-06-12-login-screen.md is blocked by Draft ADR: ADR-authentication-strategy.md
✗ roadmap/wip/auth-service.md has no linked REQ
⚠  2 roadmaps in wip/ (recommended: 1)

2 violation(s) found
```

Designed to run as a **pre-commit hook** and a **CI quality gate**. `trackfw init` wires both automatically for your stack.

### JSON output for CI integration

```bash
trackfw validate --json
```

```json
{
  "summary": { "violations": 2, "warnings": 1, "mode": "strict", "exit_code": 1 },
  "violations": [
    { "rule": "wip_has_req", "file": "roadmaps/wip/auth-service.md" }
  ],
  "warnings": [
    { "rule": "stale_wip", "file": "roadmaps/wip/auth-service.md" }
  ]
}
```

Use `--json` for programmatic CI parsing, Slack notifications, or custom reporting.

---

## `trackfw status` — current state at a glance

```bash
$ trackfw status

── trackfw status ──────────────────────

🔄 WIP (1)
   roadmap-auth-service.md

❌ Blocked (0)

⏳ REQs blocked by Draft ADRs (1)
   REQ-2026-06-12-login-screen.md
     → ADR-2026-06-12-authentication-strategy.md (Draft)

✅ Done (last 5)
   roadmap-user-profile.md
   roadmap-db-setup.md
```

---

## `trackfw barrier` — deterministic wave-release gate

```bash
trackfw barrier <roadmap> --wave <n> [--json]
```

`trackfw barrier` is the stack-agnostic core of the wave-release gate: it never assumes a build
tool, a test runner, or a parity rule. Every check either comes from the roadmap itself (the wave's
declared gates) or from `trackfw validate` run in-process. Point it at a roadmap basename (with or
without `.md`, resolved against `wip/` then `done/`) and a wave number, and it tells you whether
that wave is ready to release.

A wave passes only when **all four** built-in checks are green:

| Check | Passes when |
|---|---|
| `mls_complete` | The wave has at least one ML and every ML is marked `**Status:** ✅` |
| `acceptance_evidence` | Every ML has a non-empty `**Critérios de aceite:**` block with no unchecked `- [ ]` line |
| `gates` | Every command declared under the wave's `**Gates da wave:**` fenced block exits 0 — a wave with no such block declares zero gates, and the barrier never invents one |
| `validate` | `trackfw validate --json` reports `violations: 0` |

### Exit codes

| Exit | Meaning |
|---|---|
| `0` | `status: "passed"` — every check is green, the wave may release |
| `1` | `status: "blocked"` — at least one check failed; the JSON/text report says which |
| `2` | Usage/resolution error — the roadmap or the wave number could not be resolved. This is **not** `blocked`: a barrier that could not run is distinct from one that ran and failed |

### Correcting a blocked wave

```bash
$ trackfw barrier ROADMAP-example --wave 2
✗ mls_complete: ML-2C: not complete (status: 🔄)
✗ acceptance_evidence: ML-2C: 2 unmet acceptance criteria
wave 2: blocked
```

Fix the roadmap (mark the ML `✅`, check off the remaining criteria) and rerun the exact same
command — the barrier is not a one-shot denial; a corrected wave passes on the next invocation:

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

`trackfw barrier` is the deterministic, reproducible CLI — it never invokes agents and never
performs Git operations. The `/trackfw:barrier` slash command wraps it with the parts a binary
cannot evaluate: dispatching `code-quality`/`security` reviews, auditing the diff against scope,
and — only for `trackfw_architect`, the sole Git authority in this workflow — committing and
pushing once every check and review is green. A green CLI barrier is necessary but not sufficient
to release a wave. Full contract: `docs/cli-parity.md` → `## trackfw barrier`.

---

## AI assistant integration

`trackfw init` can install initial AI integrations. The `agents` and `skills`
command families provide the complete lifecycle in every distribution.

| Target | Native/fallback representation |
|---|---|
| Claude Code | Subagent Markdown and Agent Skills |
| Codex | Custom-agent TOML and Agent Skills |
| Gemini CLI | Agent Markdown and skills |
| Antigravity | Agent/skill directories; explicit `legacy-cli` surface available |
| Cursor | Agent Markdown and skills |
| GitHub Copilot | Custom agents and Agent Skills |
| Windsurf | Specialist-skill fallback for agents and native skills |
| Amazon Q | CLI agent JSON and workflow-rule fallback |
| Kiro | Native IDE/CLI agents and Agent Skills |

```bash
# Inspect every deployment, including legacy surfaces
trackfw agents list --json

# Install selected items in the repository
trackfw agents install --targets codex,claude --items architect,backend --scope project
trackfw skills install --targets codex,antigravity --items governance,implement --scope project

# Select an alternate surface explicitly
trackfw agents install --targets kiro --surface kiro=cli
trackfw agents list --targets antigravity --surface antigravity=legacy-cli
```

Without `--targets`, mutations open a numbered/checkbox selector in a TTY and
fail with an actionable error in CI. The lifecycle reports `not-installed`,
`current`, `outdated`, or `modified`. A manifest under `.trackfw/` records
scope-specific ownership, version, SHA-256, and shared claims. Modified files
are never replaced or removed unless `--force` is explicit, and unmanaged files
are never removed. Known historical templates are adopted without overwriting;
unknown unmanaged content cannot be adopted by `update`, even with `--force`.

Without `--scope`, `install`/`update` default to `global` (`~/.claude/...`)
when stdin is not a TTY, and otherwise prompt interactively with `global`
pre-selected — the resolved destination paths are printed before anything is
written. `uninstall` is the one exception: without `--scope` and without a
TTY it fails instead of guessing, since silently defaulting a destructive
operation could delete artifacts from the user's home directory. `list` never
prompts and always assumes `global` unless `--scope` is given, so it reports
the same destinations `install` actually wrote to.

The **12 roles** installed for each tool: **architect · backend · frontend · qa · infra · security · code-quality · dba · ux · data · iac · tooling**

The **17 skills** cover governance process (governance, implement, plan, release, review) and technical specialties (backend-skill, code-quality-skill, data-skill, dba-skill, frontend-skill, iac-skill, infra-skill, qa-skill, security-skill, tooling-skill, ux-skill, vault-skill).

---

## Agent identity — give your agents a name

By default the agents are functional and impersonal: `trackfw-architect`,
`trackfw-backend`, … Agent identity lets you name all twelve, pick how they
address you, and call them by name.

```bash
# Non-interactive: pick a themed preset
trackfw init --identity-preset greek

# Or answer the wizard, which also offers "name them one by one"
trackfw init
```

`--identity-preset` accepts ten themed presets plus two opt-outs:

`greek` · `norse` · `potter` · `thrones` · `chaves` · `pioneers` · `starwars` · `tolkien` · `turma` · `egyptian` · `neutral` · `none`

`neutral` and `none` write nothing and keep the current behavior.

Sample mapping (three of the ten presets):

| Agent | `greek` | `pioneers` | `tolkien` |
|---|---|---|---|
| architect | Zeus | Turing | Gandalf |
| backend | Apolo | Ritchie | Aragorn |
| frontend | Afrodite | Berners-Lee | Arwen |
| qa | Ártemis | Hamilton | Legolas |
| infra | Ares | Torvalds | Gimli |
| security | Hades | Diffie | Boromir |
| dba | Poseidon | Codd | Elrond |
| ux | Atena | Norman | Galadriel |
| code-quality | Hefesto | Knuth | Faramir |
| data | Métis | Hopper | Bilbo |

The remaining presets are `norse` (Odin, Thor, Freya…), `potter` (Dumbledore,
Snape, Luna…), `thrones` (Tyrion, Jon, Arya…), `chaves` (Girafales, Madruga,
Chiquinha…), `starwars` (Yoda, Han, Leia…), `turma` (Franjinha, Cebolinha,
Magali…), and `egyptian` (Thoth, Rá, Ísis…).

### Custom mode and your nickname

The interactive wizard also offers **name them one by one**: you type all ten
display names yourself. Each entry is validated as you go — an invalid name is
rejected with an inline error, never silently corrected, and two names that
resolve to the same identifier are rejected too.

The wizard then asks for an optional **nickname for you**, which is how the
agents will address you.

### `agents install` also runs the wizard

`trackfw init` is not the only entry point: `trackfw agents install`, the
natural path in a project that is already governed, offers the same wizard.
The rule is identical across the three CLIs — it appears **only** when
**all** of the following hold:

- the command is `agents` (never `skills`: skills have no identity);
- stdin is a TTY (a non-interactive run never blocks on a prompt);
- and either no `~/.trackfw/identity.json` exists yet, or `--identity` was
  passed to force reconfiguration.

With an identity already configured and no `--identity`, the command asks
nothing — it prints `identity: N custom agent(s)` and installs directly.

```bash
# First run on this machine: offers the wizard, then installs
trackfw agents install --targets claude

# Identity already configured: no prompt, installs directly
trackfw agents install --targets claude

# Force reconfiguration
trackfw agents install --targets claude --identity

# Non-interactive, same semantics as init --identity-preset
trackfw agents install --targets claude --identity-preset chaves
```

Two new flags exist only on `agents install` (`skills install` never
registers them): `--identity` (bool, forces reconfiguration even if a file
already exists) and `--identity-preset <preset>` (same ten themed presets
plus `neutral` and `none`; an invalid value errors out listing the valid
ones).

In **name them one by one** mode, each field is now labeled by the agent's
specialty, taken from the catalog, never by its technical id:

```
Architect — Architecture, ADRs and governed coordination
> _
```

Before anything is written to disk — for a themed preset **or** for custom
names — a confirmation screen lists all ten `specialty → name` pairs plus
your nickname:

```
── Confirmation ──────────────────────────────
  Architecture, ADRs and governed coordination   →  Girafales
  Backend APIs, domain logic and integrations    →  Madruga
  ...
  What we'll call you:                              chefe

? Confirm?
```

Answering no returns to preset selection; nothing is written until you
confirm.

Everything is stored in a single global file, shared by the Go, npm, and PyPI
distributions:

```json
// ~/.trackfw/identity.json
{
  "schema_version": 1,
  "user_nickname": "Kleber",
  "agents": {
    "architect": { "display_name": "Zeus", "slug": "zeus" }
  }
}
```

The generated artifact — still installed at the unchanged path
`~/.claude/agents/trackfw-architect.md`:

```markdown
---
name: zeus-tf
description: Zeus — Principal software architect for system design, ADRs and governed multi-agent coordination.
model: opus
---

Você é Zeus. Trate o usuário como Kleber.

# Architect
...
```

### Why the `-tf` suffix

The `name` always ends in `-tf`. Two agents sharing the same `name` in the same
directory make Claude Code load *"only one of them, chosen by filesystem read
order rather than a documented precedence"* — a silent, non-deterministic
shadowing that the user cannot detect. If you already keep a personal `zeus.md`
agent, `zeus-tf` guarantees both survive.

The suffix belongs to the technical identifier only. It never appears in how
the agent presents itself: the `description` and the body both say **Zeus**.

### How to invoke it

| You type | What happens |
|---|---|
| `@agent-zeus-tf` | Works — explicit mention resolves against `name` |
| "chame o Zeus" / "ask Zeus to…" | Works — natural-language routing reads `description` |
| "quem é você?" | Answers "Sou Zeus" — the body is loaded after selection |

### Cost and non-regression

The agent **never reads the configuration at runtime**. Identity is
materialized into the artifact at install time, so the per-interaction cost is
essentially zero: the `description` is substituted rather than extended, and
the body grows by tens of tokens that are loaded only after the agent has
already been selected. No tool call, no file read, no permanent instruction.

Without `~/.trackfw/identity.json`, the generated artifacts are **byte for byte
identical** to the current ones in all three CLIs. The feature is opt-in and
regresses nothing.

---

## `trackfw init` — stack-aware scaffolding

```
? Project type?          Full-stack / Frontend / Backend / Governance only
? Frontend stack?        React / Vue / Angular
? Backend stack?         Go / Java / Node / Python
? Package manager?       npm / pnpm / yarn / bun
? Git hooks?             husky / lefthook / none
? CI system?             GitHub Actions / GitLab CI / none
? Which AI assistants?   Claude / Codex / Gemini / Antigravity / Cursor / Copilot / Windsurf / Amazon Q / Kiro
? Agent identity?        Greek / Norse / Potter / Thrones / Chaves / Pioneers / Star Wars / Tolkien / Turma / Egyptian / Name them one by one / Neutral
```

The governance structure (`docs/adr/`, `docs/req/`, `docs/roadmaps/`) is always identical — stack-agnostic. The generated hooks, workflows, and AI integrations adapt to your answers.

The Codex integration is repository-scoped: `AGENTS.md` carries persistent instructions, `.agents/skills/` provides governance workflows, `.codex/agents/` provides specialist subagents, and `.codex/hooks.json` signals permission requests to the local dashboard.

---

## Design principles

1. **Files are state** — folder position is the source of truth. No database, no lock-in.
2. **Traceability is mandatory** — `validate` is a gate, not a suggestion.
3. **Framework-agnostic, integration-aware** — governance never changes; generated artifacts adapt to your stack.
4. **One active roadmap at a time** — parallel work without traceability is the root of most delivery chaos.
5. **Human-readable, machine-parseable** — every artifact is a Markdown file with a predictable structure.
6. **Guided, not prescriptive** — the wizard surfaces decisions you might not know to ask; it never blocks work unnecessarily.
7. **Configurable by design** — every governance rule has a severity (`off`/`warning`/`error`). Start permissive, tighten as your team builds the habit.
8. **AI-agent aware** — `roadmap_namespacing: by_agent` and `trace_id_field` make AI-generated work traceable and auditable by default.

---

## Credential guard: detection, not prevention

trackfw can generate a **credential guard** hook that blocks agents from materializing real
credentials. `trackfw validate` also reports when that guard looks compromised:

- the hook is registered but the script is **missing** or not executable;
- the script's **contents diverge** from the template this version of trackfw generates;
- `credential_guard.mode` was **downgraded** from `block` on disk relative to your last commit.

**Read this before relying on it.**

These checks are **detection, not prevention.** trackfw *tells you* something diverged — it does
**not** stop it from happening. This is not an implementation gap: it was measured that no technical
prevention is possible, within trackfw's scope, against an agent that has unrestricted write access
to your workspace. An agent that can edit your files can delete the guard script, overwrite it with
a no-op, or relax the mode — and on most agent CLIs, a hook that fails to run simply lets the tool
call proceed.

What these checks are **not** able to see:

- **changes the adversary also commits** — once it is in your `HEAD`, there is nothing to diverge
  from. What remains is the audit trail: the change shows up in your diff and in review.
- **anything with no commit to compare against** — a repository with no commits, or a file that was
  never versioned. The check stays silent rather than guessing.
- **the difference between tampering and you being out of date.** The script carries no version
  marker, so a divergence may simply mean you have not run `trackfw update` since upgrading. That is
  why the message is worded neutrally and the rule defaults to a **warning**, which does **not**
  change `validate`'s exit code. If you want CI to fail on it:

```yaml
# trackfw.yaml
rules:
  credential_guard_script_integrity: error
```

- **the check can be switched off by the same edit it is meant to catch.** Rule severity is read
  from `rules:` in the `trackfw.yaml` **on disk** — so an edit that both downgrades
  `credential_guard.mode` *and* sets `credential_guard_mode_downgrade: off` silences the report,
  with nothing committed. Anchoring `rules:` to your last commit is a known open item.

### If `trackfw update` suddenly reports something it used to ignore

Since the version that introduced anchoring, the credential-guard checks are resolved against **your
last commit**, not only against the file on disk — and they can **no longer be suppressed** via
`.trackfw-baseline.json`. This is deliberate: a check that can be switched off by the same
uncommitted edit it is meant to catch is not a check.

If one of them starts reporting after an upgrade, you have two legitimate ways out, and both leave a
trail:

```yaml
# trackfw.yaml — commit this change
rules:
  credential_guard_hook_resolvable: off
```

...or fix the underlying cause (usually `trackfw update`, to regenerate the guard script and wiring).

One caveat worth knowing: `governance_mode: lenient` still turns **every** finding into a warning,
including these. Closing that is tracked separately.

The strongest protection remains the ordinary one: the guard script and `trackfw.yaml` are
**versioned files**. Review their diffs like you review any other code. Every limitation above has
the same escape hatch: the change is *visible* in `git diff`.

## What trackfw is not

- Not a project management SaaS — no accounts, no cloud sync, no data leaving your repository. A local dashboard is available via `trackfw serve`.
- Not a replacement for Git history — it complements, not duplicates
- Not a task tracker — use GitHub Issues, Linear, or Jira for tasks; trackfw governs the *why*
- Not opinionated about how you write code — only about how you document decisions

---

## Compared to alternatives

| Tool | What it does | What's missing |
|---|---|---|
| **adr-tools** | Creates ADR files | No link to requirements or roadmaps |
| **madr** | ADR template format | No enforcement, no delivery tracking |
| **Linear / Jira** | Task tracking | No traceability to architectural decisions |
| **Kosli** | SDLC compliance for regulated industries | SaaS, accounts, cost — not for every team |
| **trackfw** | Enforces the full chain: decision → requirement → roadmap → delivery | — |

trackfw is the only open-source CLI that links ADRs to requirements, requirements to roadmaps, and enforces the chain as a pre-commit and CI gate — with native support for AI coding assistants.

---

## Contributing

```bash
git clone https://github.com/kgsaran/trackfw
cd trackfw
make build   # compiles to bin/trackfw
make test    # go test ./...
make lint    # go vet ./...
```

Generators are the stack-specific components — you can add support for a new stack without touching core logic. See `internal/generators/` for examples.

Issues and pull requests welcome at [github.com/kgsaran/trackfw](https://github.com/kgsaran/trackfw).

---

## License

MIT — see [LICENSE](LICENSE)

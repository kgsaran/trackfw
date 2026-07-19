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

## `trackfw agents` and `trackfw skills`

Manage specialist agents and governance skills with the same contract in the
Go/Homebrew, npm, and PyPI CLIs.

```bash
trackfw agents list|install|uninstall|update [flags]
trackfw skills list|install|uninstall|update [flags]
```

Supported targets: `claude`, `codex`, `gemini`, `antigravity`, `cursor`,
`copilot`, `windsurf`, `amazonq`, and `kiro`.

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

The standalone `gemini`, `cursor`, `copilot`, `windsurf`, and `amazonq` commands
remain only as compatibility aliases and delegate to this lifecycle.

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

Valid states: `backlog`, `wip`, `blocked`, `done`, `abandoned`

### Example

```bash
trackfw roadmap move oauth wip
# ✓ moved ROADMAP-2026-06-13-implement-oauth.md → docs/roadmaps/wip
```

The transition is automatically logged to `docs/roadmaps/.trackfw-log`.

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
8. WIP roadmaps older than 7 days are marked as stale
9. ADRs and REQs must have valid YAML frontmatter

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

## `trackfw plugins`

Manages trackfw plugins.

```bash
trackfw plugins list
trackfw plugins add <repo>
trackfw plugins remove <name>
trackfw plugins search <keyword>
```

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `list` | Lists installed plugins in `~/.trackfw/plugins/` |
| `add <repo>` | Installs plugin from GitHub Releases (format `user/name[@tag]`) |
| `remove <name>` | Removes installed plugin |
| `search <keyword>` | Searches for plugins in the official registry |

---

## `trackfw version`

Displays the installed version of trackfw.

```bash
trackfw version
# trackfw v2.1.0
```

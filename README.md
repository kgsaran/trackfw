# trackfw

> Governed software delivery — ADR → REQ → ROADMAP → backlog / wip / done

[![Release](https://img.shields.io/github/v/release/kgsaran/trackfw)](https://github.com/kgsaran/trackfw/releases/latest)
[![Go](https://img.shields.io/badge/go-1.21+-00ADD8?logo=go)](go.mod)
[![License](https://img.shields.io/github/license/kgsaran/trackfw)](LICENSE)

**trackfw** is an open-source CLI that enforces a traceable chain from architectural decision to shipped code — without SaaS, accounts, or databases. Markdown files are state.

```
ADR → REQ → ROADMAP → backlog / wip / blocked / done / abandoned
```

Every piece of work traces back to a decision. Every decision links to a requirement. Every requirement lands in a roadmap. No orphan work, no undocumented choices.

---

## The problem

Most teams accumulate technical debt not because they lack tools, but because they lack **governance traceability**. Decisions are made in Slack. Requirements live in someone's head. Roadmaps drift from what was actually shipped.

- **ADR tools** manage decision records, but don't connect them to delivery.
- **Kanban tools** track tasks, but don't enforce that tasks are backed by a decision.
- **CI tools** validate code, but don't validate governance.

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

The npm package is pure Node.js — no compiled binary, no postinstall download. Works everywhere Node.js ≥ 18 is installed, including corporate Windows environments where unsigned `.exe` files are blocked by antivirus.

### pip

```bash
pip install trackfw
```

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
```

---

## Commands

| Command | Description |
|---|---|
| `trackfw init` | Interactive wizard — scaffolds governance + AI integrations |
| `trackfw adr new "title"` | Create a new Architecture Decision Record |
| `trackfw adr list` | List all ADRs with status |
| `trackfw req new "title"` | Create a REQ with guided ADR discovery |
| `trackfw req list` | List all REQs with status |
| `trackfw roadmap new "title"` | Create a roadmap in `backlog/` |
| `trackfw roadmap show <name>` | Print a roadmap with its current state |
| `trackfw roadmap move <name> <state>` | Move roadmap between states |
| `trackfw roadmap list` | List all roadmaps grouped by state |
| `trackfw validate` | Check governance consistency (use as CI gate) |
| `trackfw status` | Show wip, blocked, REQs waiting on ADRs |
| `trackfw log [--tail N]` | Show roadmap state transition history |
| `trackfw plugins list` | List installed plugins |
| `trackfw plugins add <user/repo>` | Install a plugin from GitHub Releases |
| `trackfw plugins remove <name>` | Remove an installed plugin |
| `trackfw agents` | Install Claude Code subagents *(Go binary only)* |
| `trackfw gemini` | Install Gemini CLI skills and commands *(Go binary only)* |
| `trackfw cursor` | Install Cursor rules *(Go binary only)* |
| `trackfw copilot` | Install GitHub Copilot instructions *(Go binary only)* |
| `trackfw windsurf` | Install Windsurf rules and workflows *(Go binary only)* |
| `trackfw amazonq` | Install Amazon Q Developer rules *(Go binary only)* |
| `trackfw version` | Print version |

> **Go binary only** commands (`agents`, `gemini`, `cursor`, `copilot`, `windsurf`, `amazonq`) are available when installed via brew, `install.sh`, or `go install`. When using the npm package, AI integrations are installed through `trackfw init`.

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

## AI assistant integration

`trackfw init` asks which AI tools your team uses and installs native governance context for each. When using the Go binary (brew, `install.sh`, `go install`), each integration can also be run as a standalone command.

| Command | Installs | Format |
|---|---|---|
| `trackfw agents` | 10 subagents in `~/.claude/agents/` | Claude Code `.md` with frontmatter |
| `trackfw gemini` | GEMINI.md + 10 skills + 3 commands | `~/.gemini/` + project root |
| `trackfw cursor` | 10 rules in `.cursor/rules/` | `.mdc` with YAML frontmatter |
| `trackfw copilot` | `copilot-instructions.md` + 10 instructions + 10 prompts | `.github/` |
| `trackfw windsurf` | 10 rules + workflows in `.windsurf/` + global rules | Appends to `~/.codeium/windsurf/memories/` |
| `trackfw amazonq` | 10 rules in `.amazonq/rules/` | Plain Markdown |

Each installer is idempotent — running it twice never overwrites your customizations.

The 10 roles installed for each tool: **architect · backend · frontend · qa · infra · security · code-quality · dba · ux · data**

---

## `trackfw init` — stack-aware scaffolding

```
? Project type?          Full-stack / Frontend / Backend / Governance only
? Frontend stack?        React / Vue / Angular
? Backend stack?         Go / Java / Node / Python
? Package manager?       npm / pnpm / yarn / bun
? Git hooks?             husky / lefthook / none
? CI system?             GitHub Actions / GitLab CI / none
? Which AI assistants?   Claude Code / Gemini CLI / Cursor / Copilot / Windsurf / Amazon Q
```

The governance structure (`docs/adr/`, `docs/req/`, `docs/roadmaps/`) is always identical — stack-agnostic. The generated hooks, workflows, and AI integrations adapt to your answers.

---

## Design principles

1. **Files are state** — folder position is the source of truth. No database, no lock-in.
2. **Traceability is mandatory** — `validate` is a gate, not a suggestion.
3. **Framework-agnostic, integration-aware** — governance never changes; generated artifacts adapt to your stack.
4. **One active roadmap at a time** — parallel work without traceability is the root of most delivery chaos.
5. **Human-readable, machine-parseable** — every artifact is a Markdown file with a predictable structure.
6. **Guided, not prescriptive** — the wizard surfaces decisions you might not know to ask; it never blocks work unnecessarily.

---

## What trackfw is not

- Not a project management SaaS — no UI, no accounts, no cloud sync
- Not a replacement for Git history — it complements, not duplicates
- Not a task tracker — use GitHub Issues, Linear, or Jira for tasks; trackfw governs the *why*
- Not opinionated about how you write code — only about how you document decisions

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

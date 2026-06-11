# trackfw

> Governed software delivery framework — ADR → REQ → ROADMAP → kanban

[![Release](https://img.shields.io/github/v/release/kgsaran/trackfw)](https://github.com/kgsaran/trackfw/releases/latest)
[![License](https://img.shields.io/github/license/kgsaran/trackfw)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.21+-00ADD8?logo=go)](go.mod)

trackfw is an open-source, stack-agnostic CLI that enforces a traceable chain from architectural decision to shipped code. No SaaS. No accounts. No database. Files are state.

```
ADR → REQ → ROADMAP → backlog / wip / blocked / done / abandoned
```

---

## Why trackfw?

Most teams accumulate technical debt not because they lack tools, but because they lack **governance traceability**. Decisions are made in Slack. Requirements live in someone's head. Roadmaps drift from what was actually shipped.

trackfw closes the loop — it is the connective tissue between *why*, *what*, and *when*.

---

## Installation

### curl (Linux / macOS)

```bash
curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
```

### npm

```bash
npm install -g trackfw
```

### pip

```bash
pip install trackfw
```

### Homebrew

```bash
brew install kgsaran/tap/trackfw
```

### go install

```bash
go install github.com/kgsaran/trackfw/cmd/trackfw@latest
```

---

## Quick start

```bash
# 1. Scaffold governance structure in your project
trackfw init

# 2. Document a decision
trackfw adr new "Use PostgreSQL as primary database"

# 3. Create a requirement linked to the ADR
trackfw req new "User authentication"

# 4. Plan the work
trackfw roadmap new "Auth service"

# 5. Check governance health
trackfw validate

# 6. See what's in flight
trackfw status
```

---

## Commands

| Command | Description |
|---|---|
| `trackfw init` | Interactive wizard — scaffolds governance structure |
| `trackfw adr new "title"` | Create a new ADR from template |
| `trackfw req new "title"` | Create a new REQ, linked to an ADR |
| `trackfw roadmap new "title"` | Create a roadmap in `backlog/` |
| `trackfw roadmap move <name> <state>` | Move a roadmap between states |
| `trackfw validate` | Check governance consistency |
| `trackfw status` | Show wip, blocked, recent done |
| `trackfw version` | Print version |

---

## The governance chain

| Layer | Artifact | Purpose |
|---|---|---|
| Govern | `ADR` | Document the *why* behind a technical decision |
| Specify | `REQ` | Define *what* needs to be delivered, linked to an ADR |
| Plan | `ROADMAP` | Break the requirement into microbatches with acceptance criteria |
| Execute | `backlog → wip → done` | Track delivery state — folder is the source of truth |

### Roadmap states

```
docs/roadmaps/
├── backlog/    # queued, not started
├── wip/        # actively being worked on (one at a time)
├── blocked/    # waiting on a dependency or decision
├── done/       # completed and validated
└── abandoned/  # discontinued (reason required in frontmatter)
```

Moving a file between folders IS the state transition. No database, no API.

---

## `trackfw validate` — governance gate

```bash
$ trackfw validate

✓  All wip/ roadmaps have a linked REQ
✓  All REQs have a linked ADR
✓  All wip/ roadmaps have acceptance criteria
✓  No orphan ADRs
⚠  2 roadmaps in wip/ (recommended: 1)
```

Designed to run as a **pre-commit hook** and a **CI quality gate**. `trackfw init` generates both automatically based on your stack.

---

## `trackfw init` — stack-aware scaffolding

The interactive wizard asks about your project's stack and generates the appropriate integration artifacts:

```
? Frontend stack?     → React / Vue / Angular / None
? Backend stack?      → Go / Java / Node / Python / None
? Package manager?    → npm / pnpm / yarn / bun / N/A
? Git hooks?          → husky / lefthook / none
? CI system?          → GitHub Actions / GitLab CI / None
```

The governance structure (`docs/adr/`, `docs/req/`, `docs/roadmaps/`) is always identical — stack-agnostic. Only the integration layer adapts to your stack.

---

## AI assistant slash commands

trackfw ships with native slash commands for AI assistants. After installing, the following are available:

| Command | Description |
|---|---|
| `/trackfw:adr <title>` | Create a new ADR |
| `/trackfw:req <title>` | Create a new REQ |
| `/trackfw:roadmap <title>` | Create a new roadmap |
| `/trackfw:validate` | Run governance validation |
| `/trackfw:status` | Show project status |

Works with **Claude Code** (`.claude/commands/trackfw/`) and **Gemini CLI** (`.gemini/commands/trackfw/`).

---

## Design principles

1. **Files are state** — moving a file between folders IS the state transition.
2. **Traceability is mandatory, not optional** — `validate` is a gate, not a suggestion.
3. **The framework is agnostic; the integration is aware** — governance structure never changes; generated hooks adapt to the stack.
4. **One active roadmap at a time** — parallel work without traceability is the root of most delivery chaos.
5. **Templates over convention** — every ADR, REQ, and ROADMAP is a markdown file with a predictable structure.

---

## Roadmap

### v0.1 — Foundation ✅
- CLI scaffold (init, adr, req, roadmap, status, validate)
- Interactive stack wizard
- Stack-specific generators: Go, Java, Node, Python, React
- Git hook generators: husky, lefthook
- CI generators: GitHub Actions, GitLab CI
- Governance gate (`trackfw validate`)
- Install script

### v0.2 — Distribution
- GoReleaser pipeline (linux/darwin/windows amd64+arm64)
- npm wrapper package
- PyPI wrapper package
- Homebrew tap

### v0.3 — Plugin System
- Generator plugin interface
- `trackfw plugins list/add/remove`
- Community generator registry

### v0.4 — Intelligence
- `trackfw log` — history of state transitions
- `trackfw roadmap show <name>` — render roadmap progress in terminal
- Validate: detect stale `wip/` entries (in wip for > N days)
- `trackfw adr list` / `trackfw req list` with status summary

---

## Contributing

trackfw is designed for community contribution from day one. The generator plugin model means you can add support for a new stack without touching core logic.

```bash
git clone https://github.com/kgsaran/trackfw
cd trackfw
make build
make test
```

---

## License

MIT — see [LICENSE](LICENSE)

# Getting Started

This guide covers installing `trackfw` and the first steps to set up delivery governance in your project.

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap kgsaran/trackfw
brew install trackfw
```

### npm (Node.js >= 18)

```bash
npm install -g trackfw
```

### Go (requires Go >= 1.21)

```bash
go install github.com/kgsaran/trackfw/cmd/trackfw@latest
```

### Verify installation

```bash
trackfw version
# trackfw v2.1.0
```

---

## 1. Initialize a project: `trackfw init`

The `init` command runs an interactive wizard that detects the project stack and generates the full governance structure.

```bash
trackfw init
```

The wizard asks about:

- **Project name** — used in generated templates
- **Project type** — frontend, backend, or fullstack
- **Backend stack** — Go, Java, Node.js, or Python
- **Backend framework** — Spring Boot, Gin, Express, FastAPI, etc.
- **Git hooks** — husky, lefthook, or none
- **CI/CD** — GitHub Actions or GitLab CI
- **AI agents** — Claude Code, Gemini CLI, Cursor, etc.

### What gets generated

After `init`, the following structure is created in the repository:

```
docs/
├── adr/              ← Architecture Decision Records
├── req/              ← Requirements
├── roadmaps/
│   ├── backlog/
│   ├── wip/
│   ├── blocked/
│   ├── done/
│   └── abandoned/
├── visao-projeto/
└── agents-working-context.md

trackfw.yaml          ← project configuration
scripts/
└── trackfw-validate.sh
CLAUDE.md             ← context for Claude Code (if selected)
.claude/commands/     ← slash commands for Claude Code
```

### Brownfield mode (legacy projects)

For projects that already exist without structured governance, use the `--brownfield` flag:

```bash
trackfw init --brownfield
```

This activates **lenient mode** for 30 days: governance violations are reported as warnings instead of errors, giving the team time to adapt processes without blocking existing CIs.

---

## 2. First architectural decision: `trackfw adr new`

ADRs (Architecture Decision Records) document significant technical decisions in the project.

```bash
trackfw adr new
```

The wizard prompts for:
- **Title** of the decision
- **Context** — why this decision was needed
- **Decision** — what was decided
- **Consequences** — positive and negative impacts
- **Alternatives** considered

### Expected output

```
created docs/adr/ADR-2026-06-13-use-postgresql-as-main-database.md
```

### Generated file

```markdown
---
status: Proposed
date: 2026-06-13
author: ""
---

# ADR: Use PostgreSQL as main database

| Status: Proposed | Date: 2026-06-13 |

## Context
<!-- Why this decision was needed -->

## Decision
<!-- What was decided -->

## Consequences
<!-- Positive and negative impacts -->

## Alternatives considered
<!-- Other options evaluated -->
```

### List ADRs

```bash
trackfw adr list
```

```
ADR-2026-06-13-use-postgresql-as-main-database.md    Proposed
```

---

## 3. First requirement: `trackfw req new`

REQs (Requirements) document business and technical needs. The wizard includes **contextual probes** — domain-specific questions (authentication, UI, persistence, etc.) that automatically generate ADR Drafts when needed.

```bash
trackfw req new
```

The wizard prompts for:
- **Title** of the requirement
- **Motivation** — why this requirement exists
- **Acceptance criteria** — how to know it's done
- **Linked ADR** — related architectural decision
- **Linked Roadmap** — implementation plan

If the title or motivation mentions known domains (authentication, database, deploy, etc.), the wizard presents additional domain-specific questions and may automatically create ADR Drafts.

### Expected output

```
created docs/req/REQ-2026-06-13-oauth-login.md
created docs/adr/ADR-2026-06-13-oauth-provider.md (Draft)

Next step: resolve Draft ADRs before creating the roadmap.
```

### List REQs

```bash
trackfw req list
```

```
REQ-2026-06-13-oauth-login.md    Open
```

---

## 4. First roadmap: `trackfw roadmap new`

Roadmaps detail the implementation plan for a REQ in microbatches (MLs).

```bash
trackfw roadmap new
# or linking directly to a REQ:
trackfw roadmap new --from-req docs/req/REQ-2026-06-13-oauth-login.md
```

### Available flags

| Flag | Description |
|------|-------------|
| `--title "Title"` | Roadmap title (without wizard) |
| `--req docs/req/REQ-*.md` | Path to linked REQ |
| `--from-req docs/req/REQ-*.md` | Create roadmap already linked to REQ |

### Generated file

```markdown
---
status: backlog
date: 2026-06-13
req: ""
squad: ""
---

# Roadmap: OAuth Login

> Created: 2026-06-13 | Status: backlog

## REQ: docs/req/REQ-2026-06-13-oauth-login.md

## Wave 1 — ...
```

---

## 5. Move roadmap between states

```bash
# Start implementation
trackfw roadmap move REQ-2026-06-13-oauth-login wip

# Complete
trackfw roadmap move REQ-2026-06-13-oauth-login done
```

Available states: `backlog` → `wip` → `done` (or `blocked` / `abandoned`)

---

## 6. Overview: `trackfw status` and `trackfw validate`

### Project status

```bash
trackfw status
```

```
trackfw — project status

📋 Backlog       2 roadmaps
🔄 WIP           1 roadmap
❌ Blocked       0 roadmaps
✅ Done          3 roadmaps

📄 ADRs          4   (Proposed: 2, Accepted: 1, Draft: 1)
📝 REQs          3   (Open: 2, Closed: 1)
```

### Consistency validation

```bash
trackfw validate
```

```
✓ No violations found.
```

If there are issues:

```
✗ 1 violation(s) found:

  [violation] REQ-2026-06-13-oauth-login.md is blocked by Draft ADR: ADR-2026-06-13-oauth-provider.md

⚠  1 warning(s):

  [warning] ROADMAP-2026-06-10-refactor-db.md in WIP for 9 days (stale)
```

---

## Next steps

- [Full commands reference](/en/guide/commands)
- [Using trackfw with AI agents](/en/guide/ai-agents)

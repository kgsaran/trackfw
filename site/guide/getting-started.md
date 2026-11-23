# Getting Started

This guide covers installing `trackfw` and the first steps to set up delivery governance in your project.

## Installation

### Homebrew (macOS / Linux)

```bash
brew tap kgsaran/trackfw
brew install trackfw
```

### npm

```bash
npm install -g trackfw
```

### pip

```bash
pip install trackfw
```

### Shell script (macOS / Linux / Git Bash on Windows)

```bash
curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
```

### PowerShell (Windows)

```powershell
irm https://github.com/kgsaran/trackfw/releases/latest/download/install.ps1 | iex
```

> `install.ps1` is published with every release.

### Verify installation

```bash
trackfw version
```

This prints the installed version (for example, `trackfw 11.8.1`).

---

## Quick start

The five commands below set up a fully governed project from scratch. They are pinned by an end-to-end test (`internal/commands/quickstart_e2e_test.go`) so they keep working as the product changes.

```bash
trackfw init --ai-tools claude --forge github
trackfw adr new "User Authentication Strategy"
trackfw req new "User Authentication" --adr ADR-$(date +%F)-user-authentication-strategy.md
trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip
trackfw validate
```

After running these commands, `trackfw validate` exits with rc=0 and prints `2 warning(s)`:

```
⚠  adr_req_has_content: "docs/adr/ADR-...-user-authentication-strategy.md" has mandatory
   section(s) with only model content: ## Context, ## Decision, ## Consequences,
   ## Alternatives Considered — fill in or set rules.adr_req_has_content: off
⚠  adr_req_has_content: "docs/req/REQ-...-user-authentication.md" has mandatory
   section(s) with only model content: ## Motivation, ## Acceptance Criteria — fill in or
   set rules.adr_req_has_content: off
2 warning(s)
```

The warnings appear because the ADR and REQ sections contain only template placeholders. In a TTY session, `adr new` and `req new` open short wizards — if you fill in all sections there, `validate` produces `✓ No violations found.` instead. You can also disable the check with `rules.adr_req_has_content: off` in `trackfw.yaml`.

---

## 1. Initialize a project: `trackfw init`

The `init` command runs an interactive wizard that detects the project stack and generates the full governance structure.

```bash
trackfw init
```

For the Quick start flow, pass flags directly:

```bash
trackfw init --ai-tools claude --forge github
```

The wizard (or flags) configures:

- **Project name** — used in generated templates
- **Project type** — frontend, backend, or fullstack
- **Backend stack** — Go, Java, Node.js, or Python
- **Backend framework** — Spring Boot, Gin, Express, FastAPI, etc.
- **Git hooks** — husky, lefthook, or none
- **CI/CD** — GitHub Actions or GitLab CI
- **AI agents** — Claude Code, OpenAI Codex, Gemini CLI, Google Antigravity, Cursor, GitHub Copilot, Windsurf, Amazon Q, OpenCode, and Kiro (10 targets).

### What gets generated

After `init`, the following structure is created in the repository:

```
docs/
├── adr/              ← Architecture Decision Records
├── req/              ← Requirements
└── roadmaps/
    ├── backlog/
    ├── analyzing/
    ├── wip/
    ├── blocked/
    ├── done/
    └── abandoned/

vault/
└── notes/
    └── index.md      ← knowledge vault index

trackfw.yaml          ← project configuration
scripts/
├── trackfw-validate.sh
├── trackfw-attention-signal.sh
├── trackfw-attention-cleanup.sh
├── trackfw-credential-guard.sh
└── trackfw-git-branch-guard.sh

CLAUDE.md             ← context for Claude Code (if selected)
.claude/
├── commands/
│   └── trackfw/      ← 9 slash commands for Claude Code
└── settings.json
AGENTS.md             ← persistent Codex instructions (if selected)
.agents/skills/       ← Codex governance workflows
.codex/agents/        ← Codex specialist subagents
```

> **Global writes:** When `--ai-tools claude` is selected with the default `--scope global`, `init` also writes 12 agents to `~/.claude/agents/` and 17 skills to `~/.claude/skills/`. Use `--scope project` to keep all files within the repository.

### Brownfield mode (legacy projects)

For projects that already exist without structured governance, use the `--brownfield` flag:

```bash
trackfw init --brownfield
```

**Without an existing `trackfw.yaml`:** `init` writes `governance_mode: lenient` and `lenient_until` (today + 30 days) to the generated `trackfw.yaml`. During the lenient period, governance violations are reported as warnings instead of errors.

**With an existing `trackfw.yaml`:** `init` warns `governance keys not modified — edit trackfw.yaml directly to enable lenient mode` and leaves the existing file unchanged. To enable lenient mode manually, add to `trackfw.yaml`:

```yaml
governance_mode: lenient
lenient_until: YYYY-MM-DD   # 30 days from today
```

---

## 2. First architectural decision: `trackfw adr new`

ADRs (Architecture Decision Records) document significant technical decisions in the project.

```bash
trackfw adr new "User Authentication Strategy"
```

Pass the title as an argument. In a TTY, omitting the title opens a wizard.

The wizard (or the argument) prompts for:
- **Title** of the decision
- **Context** — why this decision was needed
- **Decision** — what was decided
- **Consequences** — positive and negative impacts
- **Alternatives** considered

### Expected output

```
created docs/adr/ADR-2026-06-13-user-authentication-strategy.md
```

### Generated file

```markdown
---
status: Proposed
date: 2026-06-13
author: ""
---

# ADR: User Authentication Strategy

| Status: Proposed | Date: 2026-06-13 |

## Context
<!-- Why this decision was needed -->

## Decision
<!-- What was decided -->

## Consequences
<!-- Positive and negative impacts -->

## Alternatives Considered
<!-- Other options evaluated -->
```

> **Note:** The section headings (`## Context`, `## Decision`, `## Consequences`, `## Alternatives Considered`) are in English regardless of locale — they are literal markers used by `trackfw validate`.

### List ADRs

```bash
trackfw adr list
```

```
ADR-2026-06-13-user-authentication-strategy.md    Proposed
```

---

## 3. First requirement: `trackfw req new`

REQs (Requirements) document business and technical needs. The wizard includes **contextual probes** — domain-specific questions (authentication, UI, persistence, etc.) that automatically generate ADR Drafts when needed.

```bash
trackfw req new "User Authentication" --adr ADR-$(date +%F)-user-authentication-strategy.md
```

Pass the title as an argument. In a TTY, omitting the title opens a wizard.

The wizard (or the argument) prompts for:
- **Title** of the requirement
- **Motivation** — why this requirement exists
- **Acceptance criteria** — how to know it's done
- **Linked ADR** — related architectural decision

### Auto-generated roadmap

`req new` automatically creates a roadmap in `docs/roadmaps/backlog/`. You do not need to run `roadmap new` separately — it is only needed if you used `req new --no-roadmap`.

### Expected output

```
created docs/req/REQ-2026-06-13-user-authentication.md
✓ created docs/roadmaps/backlog/ROADMAP-2026-06-13-user-authentication.md
✓ linked REQ-2026-06-13-user-authentication.md → docs/roadmaps/backlog/ROADMAP-...
```

> **Note:** The section headings in the generated REQ file (`## Motivation`, `## Acceptance Criteria`) are in English — they are literal markers used by `trackfw validate`.

### List REQs

```bash
trackfw req list
```

```
REQ-2026-06-13-user-authentication.md    Open
```

---

## 4. Move roadmap to WIP: `trackfw roadmap move`

The roadmap was created in `backlog/` by `req new`. To start implementation, move it to `wip/`:

```bash
trackfw roadmap move ROADMAP-$(date +%F)-user-authentication.md wip
```

Use the `ROADMAP-` prefix with the `.md` extension, not the REQ filename.

### Expected output

```
✓ moved ROADMAP-2026-06-13-user-authentication.md → docs/roadmaps/wip
✓ synced REQ-2026-06-13-user-authentication.md → docs/roadmaps/wip/ROADMAP-...
```

### Generated roadmap structure

The roadmap generated by `req new` includes a Wave 0 threat model:

```markdown
---
status: wip
date: 2026-06-13
req: "docs/req/REQ-2026-06-13-user-authentication.md"
squad: ""
---

# Roadmap: User Authentication

> Created: 2026-06-13 | Status: wip

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pending
...

## Wave 1 — ...
```

> **Note:** `ML-0A` must be marked `✅ Done` before `roadmap move ... done` will succeed. Moving to `done` with a pending ML-0A produces: `Error: cannot move "..." to done: 1 blocker(s): ML-0A`.

### Available states

`backlog` → `analyzing` → `wip` → `done` (or `blocked` / `abandoned`)

---

## 5. Check consistency: `trackfw validate`

```bash
trackfw validate
```

After the Quick start flow, `validate` exits rc=0 with `2 warning(s)` (see the Quick start section above for the full output).

### If violations are found

```
✗ roadmap "ROADMAP-2026-06-13-implement-oauth.md" is in wip but has no linked REQ
✗ REQ-2026-06-13-oauth-login.md is blocked by Draft ADR: ADR-2026-06-13-oauth-provider.md
Error: 2 violation(s) found
```

Each violation prefixes the message with `✗`. Warnings use `⚠`.

---

## 6. Project overview: `trackfw status`

```bash
trackfw status
```

Displays an inventory of ADRs, REQs, and roadmaps by state, with WIP and blocked roadmaps listed by name. Output format and counts reflect the current project state.

---

---

## Windows

For detailed Windows setup — installation paths, supported shells (PowerShell 5.1, Git Bash, pwsh 7), measured support matrix, and known limitations — see the **Windows support (partial)** section in the [trackfw README on GitHub](https://github.com/kgsaran/trackfw#windows-support-partial).

---

## Next steps

- [Full commands reference](/guide/commands)
- [Using trackfw with AI agents](/guide/ai-agents)

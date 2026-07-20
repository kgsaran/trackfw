# trackfw — Project Vision

> Version: v2.14.0 | Date: 2026-06-24

---

## What is trackfw?

**trackfw** is an open-source, stack-agnostic CLI framework for governed software delivery.

It enforces a traceable chain from architectural decision to shipped code:

```
ADR → REQ → ROADMAP → backlog / wip / blocked / done / abandoned
```

Every piece of work is traceable back to a decision. Every decision is linked to a requirement. Every requirement is planned in a roadmap. No orphan work, no undocumented choices.

---

## The Problem

Most teams accumulate technical debt not because they lack tools, but because they lack **governance traceability**. Decisions are made in Slack. Requirements live in someone's head. Roadmaps drift from what was actually shipped.

Existing tools address parts of the problem in isolation:
- ADR tools manage decision records, but don't connect them to requirements or delivery.
- Kanban tools track work, but don't enforce that work is backed by a decision.
- CI tools validate code, but don't validate governance.

**trackfw closes the loop** — it is the connective tissue between why, what, and when.

---

## The Framework Chain

| Layer | Artifact | Purpose |
|---|---|---|
| Govern | `ADR` | Document the *why* behind an architectural or technical decision |
| Specify | `REQ` | Define *what* needs to be delivered, linked to an ADR |
| Plan | `ROADMAP` | Break the requirement into microbatches with acceptance criteria |
| Execute | `backlog → wip → done` | Track delivery state — folder is the source of truth |

### States

Roadmaps live in folders. Moving a file is moving state. No database, no SaaS dependency.

```
docs/roadmaps/
├── backlog/    # queued, not started
├── wip/        # actively being worked on (one at a time)
├── blocked/    # waiting on a dependency or decision
├── done/       # completed and validated
└── abandoned/  # discontinued (reason required in frontmatter)
```

---

## AI-native Governance

trackfw is the only governance CLI that treats AI agents as first-class actors in the delivery chain.

### `roadmap_namespacing: by_agent`

When your project uses AI agents as contributors, organize artifacts by agent:

```
docs/roadmaps/
├── claude/
│   ├── wip/
│   └── done/
├── gemini/
│   └── backlog/
```

`trackfw validate` and `trackfw context` are fully aware of this layout — no false positives, no blind spots.

### Why this matters (2026)

Engineering metrics platforms (LinearB, Faros AI) report AI agents increased throughput 30–40% but also change failure rate. trackfw's `trace_id_field` and `governance_mode: strict` provide the governance layer that ensures AI-generated work is traceable and validated before merge.

---

## The CLI

trackfw ships as a single binary with no runtime dependencies.

```bash
trackfw init                        # interactive wizard → scaffolds governance structure
trackfw adr new "title"             # creates a new ADR from template
trackfw req new "title"             # creates a new REQ, linked to an ADR
trackfw roadmap new "title"         # creates a roadmap in backlog/
trackfw roadmap move <name> wip     # moves roadmap between states
trackfw status                      # shows wip, blocked, recent done
trackfw validate                    # checks governance consistency
trackfw validate --json             # structured JSON output for programmatic CI integration
trackfw context                     # structured output of project state (REQs, Roadmaps, ADRs with counts)
trackfw serve                       # local governance dashboard (no cloud, no SaaS)
trackfw traceid                     # verifies bidirectional traceability REQ↔ROADMAP
```

### `trackfw validate` — the governance gate

The validate command is the heart of trackfw. It checks:

- Every roadmap in `wip/` has a linked REQ
- Every REQ has a linked ADR
- No roadmap is in `wip/` without an acceptance criteria block
- Plural `wip/` entries (more than one active roadmap) are flagged as a warning

**`governance_mode`** — configurable binary gate in `trackfw.yaml`:
- `strict` — any violation fails the pipeline (exit code 1)
- `lenient` — violations are reported but do not block CI

**15+ configurable rules** with severity levels (`off` / `warning` / `error`):

| Rule | What it checks |
|---|---|
| `req_has_adr` | Every REQ is linked to an ADR |
| `req_has_roadmap` | Every REQ has at least one ROADMAP |
| `blocked_has_req` | Every blocked roadmap references a REQ |
| `wip_limit` | No more than N roadmaps in wip simultaneously |
| `stale_wip` | Roadmaps in wip for longer than configured threshold |
| `adr_orphan` | ADRs not referenced by any REQ |
| `wip_acceptance` | Roadmaps in wip must have acceptance criteria |

**`trace_id_field`** — bidirectional REQ↔ROADMAP traceability with 5 automatic checks:

| Check | What it validates |
|---|---|
| `traceid_orphan_req` | REQ has a trace_id not referenced by any ROADMAP |
| `traceid_orphan_roadmap` | ROADMAP references a trace_id not found in any REQ |
| `traceid_state_mismatch` | REQ and ROADMAP states are inconsistent |
| `traceid_duplicate_req` | Multiple REQs share the same trace_id |
| `traceid_duplicate_roadmap` | Multiple ROADMAPs share the same trace_id |

**`--json` output** — machine-readable format for programmatic CI integration and dashboards.

This command is designed to run as a **pre-commit hook** and a **CI quality gate**.

---

## Stack-Agnostic, Stack-Aware

trackfw itself has no opinion on your stack. But `trackfw init` does.

The interactive wizard asks about your project's stack and generates the appropriate integration artifacts:

```
? Frontend stack?     → React / Vue / Angular / None
? Backend stack?      → Go / Java / Node / Python / None
? Package manager?    → npm / pnpm / yarn / bun / N/A
? Git hooks?          → husky / lefthook / none (auto-detected: husky via npx when Node.js present but lefthook unavailable)
```

After `init`, `trackfw update` automatically injects **attention hooks** into every detected AI agent CLI config. When an agent executes a tool call, `.trackfw-attention.json` is written instantly — the `trackfw serve` board shows a live banner without any agent behavioral change:

| CLI | Hook event | Config file |
|---|---|---|
| Claude Code | `PreToolUse[AskUserQuestion]` / `PostToolUse` | `.claude/settings.json` |
| Codex CLI | `PermissionRequest` / `PostToolUse` | `.codex/hooks.json` |
| Gemini CLI | `Notification[ToolPermission]` / `AfterTool` | `.gemini/settings.json` |
| Kiro | `PreToolUse` / `PostToolUse` | `.kiro/hooks/trackfw-attention.json` |
| GitHub Copilot | `preToolUse` / `postToolUse` | `.github/hooks/trackfw-attention.json` |
| Cursor | `preToolUse` / `postToolUse` | `.cursor/hooks.json` |
| Windsurf | instruction only (no generic PreToolUse) | `.windsurfrules` |

```
? CI system?          → GitHub Actions / GitLab CI / None
```

Based on your answers, `init` generates:

| Artifact | Purpose |
|---|---|
| `trackfw.yaml` | Project config (stack profile, governance_mode, rules) |
| `scripts/trackfw-validate.sh` | Stack-aware validation script |
| `.husky/pre-commit` or `lefthook.yml` | Git hook wiring |
| `.github/workflows/trackfw-gate.yml` | CI quality gate |

The governance structure itself (`docs/adr/`, `docs/req/`, `docs/roadmaps/`) is always identical — stack-agnostic. Only the integration layer is generated per stack.

---

## Extensibility — Generator Plugin Model

Generators are the stack-specific components of trackfw. The architecture is designed to be community-extensible:

- Core generators ship with trackfw (Go, Java, Node, Python, React, Vue, Angular)
- Community generators can be added as plugins
- The plugin model follows the same pattern as Prettier parsers or ESLint plugins — a generator is a named module that receives the `Config` struct and produces files

This means a Java/Maven shop and a Go/Makefile shop and a Python/Poetry shop all get governance structure that fits their workflow — without forking trackfw.

---

## Distribution

trackfw ships across three native CLIs. Shared behavior and intentional
runtime-specific exceptions are governed by the CLI parity contract.

| Channel | Package | Implementation | Installation |
|---|---|---|---|
| Direct | GitHub Releases | Go binary | `curl -sSfL .../install.sh \| sh` |
| Homebrew | `trackfw/tap/trackfw` | Go binary | `brew install trackfw/tap/trackfw` |
| Go | `github.com/trackfw/trackfw` | Go binary | `go install github.com/trackfw/trackfw/cmd/trackfw@latest` |
| npm | `trackfw` | Native Node.js (Node ≥ 18) | `npm install -g trackfw` |
| PyPI | `trackfw` | Native Python (Python ≥ 3.10) | `pip install trackfw` |

The Node.js and Python CLIs are native reimplementations, not wrappers around a
compiled binary. Their shared behavior and intentional Go-only exceptions are
defined by `docs/cli-parity.md` and enforced by release smoke tests.

---

## Design Principles

1. **Files are state** — moving a file between folders IS the state transition. No database, no API.
2. **Traceability is mandatory, not optional** — `validate` is a gate, not a suggestion.
3. **The framework is agnostic; the integration is aware** — governance structure never changes; generated hooks adapt to the stack.
4. **One active roadmap at a time** — parallel work without traceability is the root of most delivery chaos.
5. **Templates over convention** — every ADR, REQ, and ROADMAP is a markdown file with a predictable structure. Readable by humans and parseable by machines.
6. **Configurable by design** — every governance rule has a severity (`off` / `warning` / `error`). Adopt progressively, tighten over time.
7. **AI-agent aware** — governance structure natively supports multi-agent workflows (`roadmap_namespacing: by_agent`).

---

## What trackfw Is NOT

- Not a project management SaaS (no accounts, no cloud sync) — `trackfw serve` provides a local dashboard with no external dependencies
- Not a replacement for Git history (it complements, not duplicates)
- Not a task tracker (use GitHub Issues, Linear, Jira for tasks — trackfw governs the *why*)
- Not opinionated about how you write code — only about how you document decisions

---

## Current State (v2.7.0)

| Version | Feature | Status |
|---|---|---|
| v0.1 | CLI scaffold, validate, init wizard, stack generators | ✅ Done |
| v2.4 | JSON output, configurable rules (off/warning/error), governance_mode | ✅ Done |
| v2.5 | trace_id_field (bidirectional REQ↔ROADMAP), by_agent namespacing, salvaguarda one-sided | ✅ Done |
| v2.6 | req_has_adr / req_has_roadmap / blocked_has_req configurable | ✅ Done |
| v2.7 | `branch_has_wip_roadmap` rule (gate pré-trabalho); Node.js → husky fallback (Windows/corp); agent protocol in rules block | ✅ Done |
| v2.8 | Attention hooks auto-injected for 7 CLIs (Claude, Codex, Gemini, Kiro, Copilot, Cursor, Windsurf); permission-capable CLIs signal only when user action is required; 100% parity across Go, Node.js and Python CLIs | ✅ Done |
| vNext | GitHub Actions official, trackfw serve UX, multi-repo support | 🔄 Planned |

---

## Contributing

trackfw is designed for community contribution from day one. The generator plugin model means you can contribute support for a new stack without touching core logic.

Repository: `github.com/kgsaran/trackfw`
License: MIT

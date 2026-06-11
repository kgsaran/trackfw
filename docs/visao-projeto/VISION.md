# trackfw — Project Vision

> Version: 0.1 | Date: 2026-06-11

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
```

### `trackfw validate` — the governance gate

The validate command is the heart of trackfw. It checks:

- Every roadmap in `wip/` has a linked REQ
- Every REQ has a linked ADR
- No roadmap is in `wip/` without an acceptance criteria block
- Plural `wip/` entries (more than one active roadmap) are flagged as a warning

This command is designed to run as a **pre-commit hook** and a **CI quality gate**.

---

## Stack-Agnostic, Stack-Aware

trackfw itself has no opinion on your stack. But `trackfw init` does.

The interactive wizard asks about your project's stack and generates the appropriate integration artifacts:

```
? Frontend stack?     → React / Vue / Angular / None
? Backend stack?      → Go / Java / Node / Python / None
? Package manager?    → npm / pnpm / yarn / bun / N/A
? Git hooks?          → husky / lefthook / none
? CI system?          → GitHub Actions / GitLab CI / None
```

Based on your answers, `init` generates:

| Artifact | Purpose |
|---|---|
| `trackfw.yaml` | Project config (stack profile) |
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

trackfw ships as a **Go binary** — no runtime dependency, single file, cross-platform.

| Channel | Package | Installation |
|---|---|---|
| Direct | GitHub Releases | `curl -sSfL .../install.sh \| sh` |
| npm | `trackfw` | `npm install -g trackfw` |
| PyPI | `trackfw` | `pip install trackfw` |
| Homebrew | `trackfw/tap/trackfw` | `brew install trackfw/tap/trackfw` |

npm and PyPI packages are thin wrappers that download the correct platform binary. This is the same pattern used by esbuild, Biome, and Turbo.

---

## Design Principles

1. **Files are state** — moving a file between folders IS the state transition. No database, no API.
2. **Traceability is mandatory, not optional** — `validate` is a gate, not a suggestion.
3. **The framework is agnostic; the integration is aware** — governance structure never changes; generated hooks adapt to the stack.
4. **One active roadmap at a time** — parallel work without traceability is the root of most delivery chaos.
5. **Templates over convention** — every ADR, REQ, and ROADMAP is a markdown file with a predictable structure. Readable by humans and parseable by machines.

---

## What trackfw Is NOT

- Not a project management SaaS (no UI, no accounts, no cloud sync)
- Not a replacement for Git history (it complements, not duplicates)
- Not a task tracker (use GitHub Issues, Linear, Jira for tasks — trackfw governs the *why*)
- Not opinionated about how you write code — only about how you document decisions

---

## Roadmap

### v0.1 — Foundation (current)
- [x] CLI scaffold (init, adr, req, roadmap, status, validate)
- [x] Interactive stack wizard
- [x] Stack-specific generator: Go, Java, Node, Python, React
- [x] Git hook generators: husky, lefthook
- [x] CI generators: GitHub Actions, GitLab CI
- [x] `trackfw validate` governance gate
- [x] Install script

### v0.2 — Distribution
- [ ] GoReleaser pipeline (binaries for linux/darwin/windows amd64+arm64)
- [ ] npm wrapper package
- [ ] PyPI wrapper package
- [ ] Homebrew tap

### v0.3 — Plugin System
- [ ] Generator plugin interface
- [ ] `trackfw plugins list/add/remove`
- [ ] Community generator registry

### v0.4 — Intelligence
- [ ] `trackfw log` — history of state transitions
- [ ] `trackfw roadmap show <name>` — render roadmap progress in terminal
- [ ] Validate: detect stale `wip/` entries (in wip for > N days)
- [ ] `trackfw adr list` / `trackfw req list` with status summary

---

## Contributing

trackfw is designed for community contribution from day one. The generator plugin model means you can contribute support for a new stack without touching core logic.

Repository: `github.com/trackfw/trackfw`
License: MIT

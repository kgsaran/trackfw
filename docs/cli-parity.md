# CLI parity contract

Go is the behavioral reference. Node.js and Python must expose the same public
commands unless an exception is listed below.

Supported runtimes: Go 1.25+, Node.js 18+, and Python 3.10+.

| Command | Go | Node.js | Python | Contract |
|---|---:|---:|---:|---|
| `init` | yes | yes | yes | Creates governance structure and `trackfw.yaml`; `--identity-preset` selects an agent identity preset |
| `adr` | yes | yes | yes | `new`, `list` |
| `req` | yes | yes | yes | `new`, `list`, `move` |
| `roadmap` | yes | yes | yes | `new`, `move`, `list`, `show` |
| `validate` | yes | yes | yes | Text and `--json`; nonzero on violations |
| `status` | yes | yes | yes | Governance summary |
| `context` | yes | yes | yes | Markdown/JSON context |
| `log` | yes | yes | yes | Append/read transition log |
| `baseline` | yes | yes | yes | Persist accepted findings |
| `help` | yes | yes | yes | Single explicit help surface: `trackfw help` lists commands and config keys; `trackfw help <command>` shows that command's help; `trackfw help <key>` shows config key documentation; unknown topic exits non-zero with a suggestion when a close match exists. Native `--help` on root/subcommands is preserved separately by each runtime's framework (cobra/commander/argparse) |
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
| `branch` | yes | yes | yes | `new <type>/<slug>` — gates `git checkout -b` on the same `branch_has_wip_roadmap` matching logic `trackfw validate` already applies, moving the check before branch creation instead of after (see below) |
| `gemini` / `cursor` / `copilot` / `windsurf` / `amazonq` | yes | no | no | Historical Go-only compatibility aliases |
| `version` / `--version` | yes | yes | yes | Both print the same single line: `trackfw <semver>`, no `v` prefix — see "Version output" below |

## Version output

Both surfaces — the `version` subcommand and the `--version` flag — print **the same single line** to
stdout, in all three runtimes:

```
trackfw 5.0.0
```

Pinned literally:

| Element | Rule |
|---|---|
| Program name | Literal `trackfw`, then a single space |
| Version | SemVer `<major>.<minor>.<patch>`, **no `v` prefix**, no suffix |
| Line | Exactly one, terminated by `\n`, on **stdout** |
| `version` ≡ `--version` | Byte-identical to each other, within and across runtimes |

**No `v` prefix.** The `v` is a Git *tag* convention, not a version-string convention — SemVer states
that `v1.2.3` is not a semantic version. `npm/package.json` and `pypi/pyproject.toml` cannot carry it
(npm rejects it), and those manifests are the source of the string in two of the three runtimes.
Printing with `v` would force Node.js and Python to concatenate a prefix, creating two representations
of the same version inside one runtime.

**The Git tag stays `v<x.y.z>`.** That is where the prefix belongs and it does not change.
`scripts/install.sh` already strips it (`VERSION_BARE="${VERSION#v}"`) and is unaffected.

### Source of the string per runtime

| Runtime | Source |
|---|---|
| Go | `internal/version/version.go`, stored **without** the `v` |
| Node.js | `npm/package.json` |
| Python | `importlib.metadata`, with a literal fallback in `pypi/trackfw/__init__.py` |

In Go both surfaces consume `version.Version` — `internal/commands/version.go` for the subcommand and
the cobra `Version` field in `internal/commands/root.go` for the flag — so the stored value governs
both.

### Gate assertion — pinned, and why the old one was vacuous

The parity gate must apply **the same assertion to all three runtimes**:

```
^trackfw [0-9]+\.[0-9]+\.[0-9]+$
```

and must additionally compare the **bytes** of both surfaces across runtimes. The regex alone is not
sufficient evidence.

This is pinned because the previous gate hid the divergence instead of catching it.
`scripts/check-cli-parity.sh` asserted `^trackfw .+` for Go and Python — loose enough to accept
`trackfw v5.0.0` and `trackfw 5.0.0` equally, which is precisely why the `v` prefix survived every
audit — and used a **different regex for Node.js**
(`^([0-9]+\.){2}[0-9]+|^0\.0\.0-dev$`), which encoded that runtime's divergence as expected behaviour.
A per-runtime exemption in a parity gate makes the difference permanent and invisible.

### `-v` is reserved for verbose — never bound to `--version`

`-v` is **not** a shorthand for `--version` in any runtime. All three reject it with a **non-zero exit**.
Resolved by `REQ-2026-07-30-reservar-v-para-verbose-e-remover-atalho-de-versao-no-go`; previously it was
accepted **only by Go**, and nobody had decided that — cobra's `InitDefaultVersionFlag` registers
`--version` with the shorthand `v` whenever the `Version` field is set and the shorthand is free. The
flag was exposed by framework default, not by design.

**`-v` and `--verbose` are reserved for a future verbose mode. No runtime may bind them to any other
semantics.** In much of the ecosystem — `docker`, `kubectl`, `ansible`, `ssh`, `curl` — `-v` means
*verbose*, not *version*; and none of the three CLIs has `--verbose` today. Keeping `-v` bound to
version would burn the shorthand permanently, and freeing it later would be another breaking change.
`--version` and the `version` subcommand already cover the use case in all three.

**The reservation is contractual, not a surface.** No runtime accepts `-v` as a no-op. A flag that is
accepted but does nothing is worse than `unknown option`: the caller passes it, expects verbose output,
gets silence with no error, and cannot tell "reserved" from "broken".

Implementing the verbose semantics is **not** part of this reservation — deciding what becomes verbose
per command, and in what format, needs a concrete use case. It gets its own REQ when one exists.

#### What is *not* unified — measured, and deliberately left alone

After rejection, the three emit **different messages and exit codes**, because those are produced by the
frameworks. Baseline measured with an arbitrary unknown flag (`--zzz`):

| Runtime | Message | Exit |
|---|---|---|
| Go (cobra) | `Error: unknown flag: --zzz` | 1 |
| Node.js (commander) | `error: unknown option '--zzz'` | 1 |
| Python (argparse) | `trackfw: error: unrecognized arguments: --zzz` | **2** |

This divergence is **pre-existing and applies to every unknown flag**, not just `-v`. Argparse's exit 2
is the POSIX convention for a usage error.

The contract therefore requires only that `-v` **is not bound** and **exits non-zero** in all three. It
does **not** require a byte-identical message or an identical exit code: forcing that would mean
overriding the error handling of cobra, commander and argparse globally — a far larger change affecting
every command and every flag, which needs its own REQ if ever desired.

This boundary is written down on purpose. Without it, an implementer would chase byte-identity here,
fail, and most likely reach for a hack in one framework's error path.

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

## Canonical governance references

REQ frontmatter fields `adr:` and `roadmap:` use the same canonical reference
format in Go, Node.js, and Python: a complete path from the project root,
including the `.md` suffix.

Examples:

```yaml
adr: docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md
roadmap: docs/roadmaps/done/ROADMAP-2026-07-27-integridade-das-referencias-e-ciclo-de-vida-da-req.md
```

The validator checks the referenced path literally after normal path expansion
such as `~/`. It does not fall back to recursive basename matching. A basename
like `ROADMAP-001.md`, or a path that points to the wrong state directory such
as `docs/roadmaps/wip/X.md` when the file is in `done/`, is invalid even when a
file with the same basename exists elsewhere under `docs/roadmaps/`.

### `roadmap move` synchronizes the paired REQ reference

Because the reference is checked literally against the state directory, moving a roadmap invalidates
every REQ that points at it. `trackfw roadmap move` therefore rewrites those references as part of the
move. Without this, the command that exists to satisfy governance produces a state the validator
rejects — observed four times across two consecutive sessions, once per transition.

**Direction and timing.** Synchronization is **unidirectional**: the move knows the roadmap's new path
and fixes whoever points at it. It runs **after** a successful rename, at the same point in the flow
where the roadmap's own `status:` is already rewritten — never before, so a failed rename leaves no
dangling edit.

**Discovery source.** Scan `req_dir` for REQs whose `roadmap:` **basename** equals the moved roadmap's
basename. Cover both the flat layout (`req_dir/*.md`) and `by_agent`
(`req_dir/<agent>/<state>/*.md`), mirroring what the validator already scans.

Do **not** use the roadmap's own `req:` field for discovery. `trackfw roadmap new` writes `req: ""`, and
existing roadmaps carry a bare slug there with no path and no `.md`. Discovery must run in the inverse
direction.

**Which field is normative.** The **frontmatter** `roadmap:` is what the validator reads. `extractRefPath`
returns the first `Roadmap`-keyed value ending in `.md` and trims only quotes, not backticks — so the
body form `` Roadmap: `docs/roadmaps/wip/X.md` `` ends in a backtick, never matches, and is invisible to
the validator. The body line is updated anyway, **preserving its existing formatting including
backticks**, because a body that disagrees with the frontmatter misleads the human reader. An
implementation that updates only the body fixes nothing.

**Cardinality — every case pinned:**

| REQs pointing at the moved roadmap | Behaviour |
|---|---|
| Zero | No-op, **no output**, exit 0. A roadmap without a REQ is legitimate. |
| One | Rewrite both fields; one output line. |
| Several | Rewrite **all**; one output line each, sorted **lexicographically by REQ basename**. |
| Points at a **different** roadmap | Not touched. |
| Reference already correct | **No write at all** — byte-level idempotent. Moving twice changes nothing. |

**Order is pinned, not delegated to the filesystem.** Sort by REQ basename before emitting. An earlier
draft of this contract said "in `req_dir` scan order", which is not an order at all: Go's
`filepath.Glob` returns sorted results, while Node.js `fs.readdirSync` and Python `glob` guarantee
nothing across filesystems. Two runtimes would agree on macOS and diverge elsewhere — a divergence no
test on a single machine would catch. Reported by the ML-2B implementer rather than silently absorbed.

**Output, pinned literally.** One line per REQ actually rewritten, on **stdout**, after the existing
`✓ moved ...` line:

```
✓ synced <req-basename> → <new-roadmap-path>
```

**On failure.** A REQ that cannot be rewritten does **not** roll back the move — the roadmap has already
been renamed and reverting would create a worse inconsistency. Emit a diagnostic on **stderr** naming
the REQ, and exit non-zero:

```
trackfw roadmap move: failed to sync <req-basename>: <cause>
```

Remaining REQs are still attempted; the command reports the first failure's cause and exits non-zero
after processing all of them, so one unwritable file does not hide the rest.

### `req list` / `req move` — discovery layouts and conditional physical move

`req_dir` reuses the roadmap's own `roadmap_namespacing` field — there is no separate `req_namespacing`
key (see ADR-2026-08-04). `req list` and `req move <name> <status>` discover REQs by concatenating three
fixed, non-recursive globs (not mutually exclusive, all three are always scanned):

1. `req_dir/*.md` — flat legacy layout.
2. `req_dir/<state>/*.md` for each of the six governance states — per-state layout, no agent segment.
3. `req_dir/<agent>/<state>/*.md`, only when `roadmap_namespacing: by_agent` — by_agent layout, agents
   from `agents:` in config or, if unset, the first-level subdirectories of `req_dir`.

A REQ nested deeper than these three fixed patterns is invisible to both commands.

**`req move` mode is discriminated by where the file currently lives**, not by a flag:

- REQ found directly under `req_dir/` (flat) → **in-place**: only the `status:` frontmatter field (and
  the first `| Status: ... |` marker in the body, if present) is rewritten; the file is not moved and no
  folder is created. `<status>` is written verbatim — it accepts any string, including the free-form
  values (`Open`, `Done`, ...) existing flat REQs already carry. Existing flat REQs are never migrated to
  a state-subfolder layout automatically.
- REQ found under `req_dir/<state>/` or `req_dir/<agent>/<state>/` (a recognized state subfolder) →
  **physical move**: the file is relocated to `req_dir/<state-or-agent>/<new-status>/`, target directory
  created if missing, mirroring `trackfw roadmap move`. In this mode `<status>` **must** be one of the
  six governance state names (`backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`); any other
  value is rejected with `invalid state` — the free-form vocabulary from the flat mode does not apply
  here.
- Any other layout under `req_dir/` (unrecognized) → falls back to the in-place behavior above.

The transition is appended to `<req_dir>/.trackfw-log` — a log file separate from
`<roadmap_dir>/.trackfw-log`; `trackfw log` reads only the roadmap log, so REQ transitions do not appear
in `trackfw log` output.

## JSON Schema artifacts

`trackfw init` publishes `docs/schema/adr.schema.json`,
`docs/schema/req.schema.json`, and `docs/schema/roadmap.schema.json` as
cross-runtime helper artifacts for external agents and automation. They describe
the expected frontmatter object after a caller has extracted it from Markdown.

The Go, Node.js, and Python `trackfw validate` implementations do not load or
execute those JSON Schemas automatically. `trackfw validate` remains governed by
the internal validation rules documented in this contract, including
frontmatter presence, folder/status coherence, reference integrity, and
traceability checks.

## Validator `stale_wip` and inspection errors

The Go, Node.js, and Python validators share the same `stale_wip` contract:

- A roadmap's WIP age is measured from its latest transition into `wip/` in
  `docs/roadmaps/.trackfw-log`.
- Valid WIP-entry transitions include any log line for the current roadmap whose
  destination state is `wip`, such as `backlog → wip`, `analyzing → wip`, or
  `blocked → wip`.
- In `roadmap_namespacing: by_agent`, the roadmap identity includes the agent
  prefix exactly as written in the log, for example
  `zeus/ROADMAP-YYYY-MM-DD-<slug>.md`.
- If `.trackfw-log` is absent, or if the current roadmap has no parseable entry
  into `wip`, the backward-compatible fallback is the file `mtime`.
- Git commit time is not part of the cross-runtime contract for WIP age. It
  describes file edit history, not time spent in the WIP state.
- The default stale threshold remains 7 days and the default rule severity
  remains `warning` unless `rules.stale_wip` overrides it.
- Projects may override the threshold with `stale_wip_days` in `trackfw.yaml`.
  Values less than or equal to zero are ignored and fall back to the default.

```yaml
stale_wip_days: 14
rules:
  stale_wip: warning
```

Inspection failures must not degrade silently:

| Condition | Contract |
|---|---|
| Missing optional state directory such as `wip/`, `blocked/`, or `done/` | No finding; missing state directories are treated as empty states. |
| Permission denied, `ENOTDIR`, or walk/list failure for an existing configured directory | Emit a diagnostic for the owning rule, including the path and cause. Severity follows that rule's configured severity. |
| Expected file exists but cannot be `stat`ed or read | Emit a diagnostic for the owning rule and continue inspecting the remaining files. |
| Invalid support file or invalid transition-log line | Emit a diagnostic and use the documented fallback for the affected artifact when available. |

The implemented coverage is intentionally cross-runtime: Go, Node.js, and
Python test the `.trackfw-log` source of truth, configurable boundary behavior,
`mtime` fallback, and `ENOTDIR`/walk-error diagnostics for `wip/`.

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
Cursor, GitHub Copilot, Windsurf, Amazon Q, OpenCode, and Kiro.

### OpenCode agent representation (`opencode-agent`)

OpenCode (opencode.ai) is the tenth catalog target
(`REQ-2026-08-04-compatibilidade-com-opencode-opencode-ai-para-uso-de-modelos-open-source`).
Skills need no special handling — the OpenCode `SKILL.md` schema (`name`/`description`, optional
`license`/`compatibility`/`metadata`) is already identical to the shared `skill` representation.
Agents, however, use a dedicated `Render()` case, `"opencode-agent"`, that **reconstructs the
frontmatter from scratch** instead of reusing the default `subagent` case — the same pattern
already used for Antigravity's `agent-directory`. Confirmed experimentally against the real
OpenCode binary (1.18.13):

- **Frontmatter is rebuilt, not reused, because the source frontmatter hard-fails OpenCode's
  loader.** The canonical asset frontmatter carries `tools:` as a flat list of tool names
  (`tools: Agent, Read, Edit, Write, Bash, ...`). In OpenCode's agent schema `tools:` is a
  **reserved key** expecting a per-tool override object (e.g. `tools: { bash: false }`), not a
  list/string. Feeding the list verbatim does not just skip that one field — it makes OpenCode
  **refuse to load the entire project configuration** (`Configuration is invalid at
  .../agents/<file>.md`), reproduced against opencode 1.18.13. Reusing the existing `subagent`
  render path would therefore break every OpenCode project that installs a trackfw agent, not
  degrade gracefully.
- **`mode: subagent` is always fixed.** Without an explicit `mode:`, OpenCode defaults an agent to
  `mode: "all"` — selectable both as a subagent and as the primary/interactive persona in chat.
  trackfw agents must never be selectable as the primary persona (parity with their behavior in
  Claude Code, Cursor, and Gemini CLI), so `mode: subagent` is emitted unconditionally; it is never
  derived from the source asset.
- **`model:`, `tools:`, and `memory:` are omitted deliberately**, not mapped:
  - `tools:` — omitted because of the hard-fail above; there is no safe list-based value to emit.
  - `model:` — OpenCode expects `provider/model-id` (e.g. `anthropic/claude-sonnet-4-5`), while the
    catalog's `model:` field carries Claude Code aliases (`opus`, `sonnet`). Passing an alias
    through unmapped is accepted at load time but resolves to an invalid reference
    (`{"providerID": "opus", "modelID": ""}`) that fails at request time — a silent, worse
    fallback than omitting the field. Omitting `model:` lets OpenCode fall back to the model the
    user already configured (globally or per-agent) in `opencode.json`, which also matches this
    REQ's business motivation: routing trackfw agents to whatever open-source/local model
    (Ollama, LM Studio) the user already runs, instead of pinning every agent to Anthropic.
  - `memory:` — not part of OpenCode's schema; unknown non-reserved keys are silently absorbed
    into `options` rather than rejected, but it carries no meaning there, so it is left out.
  - Verified against the real `GET /agent` endpoint of a running `opencode serve`: the resolved
    JSON for an installed trackfw agent has `mode: "subagent"` and no `model` key at all (not
    `null` — absent), confirming the omission is honored end to end, not just at template level.

This representation is implemented identically in `internal/integrations/render.go` (Go, canonical
case), `npm/src/integrations/render.js`, and `pypi/trackfw/integrations/renderers.py`, and covered
by `TestRenderOpenCodeAgent` (Go) and their Node.js/Python equivalents.

The "Declared harness targets — pinned list" table further down this document already lists
`opencode` between `amazonq` and `kiro` (added in Wave 3 of the same REQ, alongside the
`harnessCatalogTargetOrder` / `_CATALOG_TARGET_ORDER` fix) — that entry is not duplicated here;
this section only documents the agent-representation decision that entry depends on.

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
Target/surface coverage is derived from the canonical integration catalog
(`internal/integrations/assets/catalog.json`): every surface whose `agents`
support level is not `unsupported` is exercised, using the default target name
for the default surface and `target=surface` for additional surfaces. This means
a new agent-capable catalog surface enters the gate without editing a manual
target list.
It runs as part of `make quality`, alongside `check-cli-parity.sh` and
`check-integration-assets.sh`.

`scripts/check-gates-falsify.sh` includes the P4 scenario
`identity-parity/catalog-target-missing`, which mutates a temporary catalog copy
to add an uncovered agent-capable surface and requires
`check-identity-parity.sh` to fail with a catalog coverage diagnostic.

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
2. Validates governance — REQ + roadmap in wip/ or done/ must exist
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
REQ and a roadmap in `wip/` or `done/` **always** aborts ship with exit code 1, regardless of
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

## `trackfw branch new`

`trackfw branch new <type>/<slug>` moves the `branch_has_wip_roadmap` governance gate — already
enforced by `trackfw validate` and `trackfw ship` (see "Regra `branch_has_wip_roadmap`" below) —
to **before** branch creation instead of after. It reuses the exact same matching logic those two
commands already apply; the command never implements a second version of the rule.

### Command surface

| Element | Value |
|---|---|
| Invocation | `trackfw branch new <type>/<slug>` |
| `<type>` | One of `feat`, `fix`, `refactor` — same vocabulary `trackfw ship` step 1 already validates |
| `<slug>` | Non-empty; matched against roadmaps in `wip/` and `done/` the same way `branch_has_wip_roadmap` does |
| `--dry-run` | Reports whether the branch would be created or blocked, without executing `git` |
| Exit 0 | Match found, branch created (or `--dry-run` reports "would create") |
| Exit non-zero, usage error | Malformed spec (missing `/`, empty slug, invalid `<type>`) |
| Exit non-zero, blocked | No matching roadmap in `wip/` nor `done/` — `git checkout -b` is **never** executed |
| Exit = Git's own code | Match found, `git checkout -b` ran and failed (e.g. branch already exists → Git's `128`) |

### Decision flow

```
1. Parse "<type>/<slug>" — <type> must be feat|fix|refactor, <slug> non-empty.
2. Normalize the slug and check whether any roadmap filename in wip/ or done/ contains it —
   the same BranchSlugMatchesRoadmap (Go) / branchSlugMatchesRoadmap (Node.js) /
   branch_slug_matches_roadmap (Python) function trackfw validate calls for
   branch_has_wip_roadmap. Not a reimplementation — the same function, imported.
3. No match: print the same governance orientation message trackfw validate already prints for
   this rule, exit non-zero, never invoke git.
4. --dry-run with a match: print "[dry-run] would create branch "<type>/<slug>" (git checkout -b
   <type>/<slug>)", exit 0, never invoke git.
5. Match, no --dry-run: run `git checkout -b <type>/<slug>` with inherited stdio.
```

### Shared matching logic — never duplicated

The slug-matching rule is implemented once per runtime and called from both places:

| Runtime | Shared function | Called by `trackfw validate` | Called by `trackfw branch new` |
|---|---|---|---|
| Go | `validator.BranchSlugMatchesRoadmap` | `validateBranchHasWIPRoadmap` (`internal/validator/validator.go`) | `runBranchNew` (`internal/commands/branch.go`) |
| Node.js | `validator.branchSlugMatchesRoadmap` | `npm/src/validator.js` | `runBranchNew` (`npm/src/branch/runner.js`) |
| Python | `_validator.branch_slug_matches_roadmap` | `pypi/trackfw/validator.py` | `run_branch_new` (`pypi/trackfw/commands/branch.py`) |

Because both call sites share the same function, the "no match" message is byte-identical in
both places — a project running `trackfw validate` and `trackfw branch new` never sees two
different explanations for the same governance gap. The governance-orientation and
no-matching-roadmap message builders (`BranchGovernanceOrientation` /
`BranchNoMatchingRoadmapMessage` in Go, and their Node.js/Python equivalents) are shared the
same way.

### Git output and exit code are propagated literally

`trackfw branch new` never reformats, wraps, or replaces `git checkout -b`'s own stdout, stderr,
or exit code. This was **not** true by default in two of the three runtimes and required an
explicit fix — see
`vault/notes/branch-new-exit-code-leak-vs-propagation-2026-08-04.md` for the full incident:

- **Go** originally leaked an extra stderr line, `exit status 128`, that Git itself never
  produces — an artifact of `exec.ExitError.Error()` being printed a second time by
  `root.go`'s `Execute()` (which prints any error returned from `RunE`, regardless of
  `SilenceErrors`). Fixed: `defaultGitCheckout` now calls `os.Exit(exitErr.ExitCode())` directly
  when the failure is a `*exec.ExitError`, so nothing propagates back through cobra to be
  printed a second time.
- **Node.js** originally translated any `git checkout -b` failure into a hardcoded exit code
  `1`, discarding Git's actual code (`128` for "branch already exists", but not always).
  Fixed: `defaultGitCheckout` (`npm/src/branch/runner.js`) now returns the real numeric exit
  code from `spawnSync`, and `runBranchNew` returns it unchanged.
- **Python** was correct from the first version — `_default_git_checkout`
  (`pypi/trackfw/commands/branch.py`) returns `subprocess.run(...).returncode` directly.

The net effect, confirmed empirically against real `git` subprocesses (not fakes) in all three
runtimes: for the "branch already exists" scenario, stdout, stderr, and exit code (`128`) are
byte-for-byte and numerically identical across Go, Node.js, and Python. No runtime prints an
extra diagnostic line, and none substitutes a fixed exit code for Git's own.

This matters because dependency-injected unit tests (all three runtimes inject a fake
`execGitCheckout`/`exec_git_checkout` for testability) never exercise the production wrapper —
the only way to verify "propagate literally" is to run a real `git` subprocess and compare, which
is exactly what `scripts/check-branch-new-parity.sh` (see below) does.

### Parity gate

`scripts/check-branch-new-parity.sh` covers three scenarios, each asserting stdout, stderr, and
exit code are byte-identical across all three runtimes:

1. **No match** — blocks, `git checkout -b` never runs.
2. **Match + `--dry-run`** — reports "would create", exit 0, never touches `git`.
3. **Match, real `git`, target branch already exists** — the only scenario that exercises the
   production `defaultGitCheckout` wrapper end-to-end rather than an injected fake; asserts
   Git's own diagnostic and exit code (`128`) are propagated unmodified in all three runtimes,
   and that no runtime leaks a Go-style `exit status N` artifact.

Wired into `make quality` via the `parity` target. `scripts/check-gates-falsify.sh` proves the
gate is non-vacuous (P4): a corrupted Node.js build that reformats the `blocked: ...` stderr
message is a scenario the gate is asserted to reject.

## `trackfw barrier`

`trackfw barrier <roadmap> --wave <n>` is the deterministic core of the wave-release barrier.
It is **stack-agnostic**: it never assumes a build tool, a test runner or a parity rule. Every
executable check comes from the roadmap itself. The agent-orchestration layer (specialist
inspections for code quality and security) lives in the `/trackfw:barrier` slash command, never
in the binary.

### Command surface

| Element | Value |
|---|---|
| Invocation | `trackfw barrier <roadmap> --wave <n>` |
| `<roadmap>` | Basename with or without `.md`, resolved against `wip/` then `done/` under `roadmap_dir` (both `flat` and `by_agent` layouts) |
| `--wave` | Wave **label**, required. Grammar `<integer>[-<suffix>]` — see "Wave label grammar" below. `2`, `2-bis`, `2-hotfix` are valid; the integer part must be ≥ 1. |
| `--json` | Emit the result document instead of the text report |
| Exit 0 | `status: "passed"` |
| Exit 1 | `status: "blocked"` — at least one check failed |
| Exit 2 | Usage/resolution error (roadmap not found, wave not found, malformed `--wave`) |

Exit code 2 is **not** `blocked`: a barrier that could not be evaluated is distinct from a
barrier that evaluated to a failure. The three runtimes must agree on this distinction.

**Exit-2 messages must be specific.** The message on `stderr` must name the unresolved entity —
the roadmap basename that could not be found, or the wave number that does not exist in the
roadmap. A generic parser error ("invalid choice", "unknown command") does not satisfy the
contract: it is indistinguishable from the CLI not implementing `barrier` at all, which makes any
exit-2 assertion vacuously true before implementation. This is the exact false positive found
while characterizing the contract in ML-1A; see
`vault/notes/barrier-contract-xfail-false-positive-2026-07-29.md`.

The two exit-2 messages are **pinned literally** — all three runtimes must emit these byte-for-byte
on `stderr`. `<roadmap-arg>` is the argument exactly as the user typed it, with no `.md`
normalization; `<roadmap-file>` is the resolved basename including `.md`:

```
trackfw barrier: roadmap "<roadmap-arg>" not found in wip/ nor done/ under <roadmap_dir>
trackfw barrier: wave <label> not found in roadmap "<roadmap-file>"
trackfw barrier: malformed wave heading at line <n>: "<token>" is not a valid wave label
trackfw barrier: invalid --wave "<value>" — not a valid wave label
```

Pinning the text matters because these messages are the only observable difference between "the
CLI does not implement barrier" and "barrier ran and could not resolve its input". A runtime that
paraphrases them satisfies its own tests while breaking cross-runtime equivalence.

The third message was added when the wave label grammar was introduced. Before that it was
**unpinned, and all three runtimes diverged**: Go said `%q is not a valid wave number`, Python said
`number {token!r} is not parseable`, and Node.js dumped the whole line without naming the cause at
all. `<token>` is the captured label, **never** the whole line — a caller must be able to tell which
token was rejected. `<n>` is 1-based.

The fourth message — an invalid `--wave` **argument**, as opposed to a malformed heading in the file —
was pinned for the same reason, one round later. Leaving it unpinned produced three texts again:
`invalid --wave "X" — not a valid wave label` (Go), `invalid --wave value: "X" (must be a valid wave
label, e.g. 1, 2-bis)` (Node.js), `malformed --wave value: "X" is not a valid wave label` (Python).
The pinned text is Go's. Both other runtimes align to it. `<value>` is the argument exactly as the
user typed it.

**JSON whitespace is normalized by the gate, key order is not.** `scripts/check-barrier.sh` strips
formatting differences before diffing (Go emits compact JSON, Node.js and Python emit spaced), so
`"wave":"1"` and `"wave": "1"` are equivalent for parity purposes. It deliberately does **not**
`sort_keys` — declaration order is part of the contract. Do not "fix" the spacing.

### Wave label grammar

A wave label is `<integer>[-<suffix>]`:

| Element | Rule |
|---|---|
| Integer part | One or more digits, value ≥ 1. Required. |
| Suffix | Optional. A single `-` followed by `[a-z0-9]+` — lowercase only. |

Valid: `1`, `2`, `2-bis`, `2-hotfix`, `10-a2`. Invalid: `X`, `2-BIS` (uppercase), `-bis` (no integer),
`2-` (empty suffix), `2-bis-ter` (two suffixes), `0` (integer < 1).

Regex, pinned: `^## Wave (\d+(?:-[a-z0-9]+)?) ` — the trailing space is part of rule 1 and is
preserved.

**Labels are distinct identities.** `--wave 2` matches `## Wave 2 ` and **never** `## Wave 2-bis `.
There is no prefix or fuzzy matching: a label either matches exactly or it does not.

**Ordering** — used only where waves must be listed or compared, never to infer that one wave gates
another:

1. Compare the integer parts numerically.
2. On a tie, a label with no suffix precedes a label with a suffix.
3. On a tie between two suffixes, compare the suffixes lexicographically.

So `2` < `2-bis` < `2-hotfix` < `3`.

**Why the suffix exists.** A corrective wave appended *after* an earlier wave was already executed and
committed needs a label that signals the correction without renumbering the following waves, which are
already cited in commit messages. Observed in the roadmap
`install-pula-artefato-desatualizado-em-vez-de-abortar` (PR #86): the cross-audit of Wave 2 required a
convergence wave, and the barrier rejected **all four waves** with `malformed wave heading`.

**A heading outside this grammar still aborts the whole document — intentionally.** Scoping the error
to the requested wave was considered and **rejected**: silently ignoring a malformed heading would
leave the MLs inside it **unaudited**, so a typo like `## Wave X — ...` would produce a green barrier
over unverified work. That is the same vacuity that ADR decision 13 forbids ("an ML must not pass for
having nothing to fail"), and it would also let a malformed roadmap read as "wave blocked", which ADR
decision 12 forbids. See ADR decision 16.

#### Detection is a full pre-pass — pinned

Two regexes are required, and **the order of operations matters more than the regexes**:

| Regex | Role |
|---|---|
| `^## Wave (\S+) ` | **Broad detector.** Decides "this line is a wave heading". |
| `^\d+(?:-[a-z0-9]+)?$` | **Strict validator**, applied to the token the broad detector captured. |

A line that matches the broad detector but fails the strict validator is a **malformed wave heading**
and aborts. Without the broad detector, a strict-only regex would simply not match `## Wave X — ...`,
the line would be treated as "not a wave heading at all", and the abort would silently disappear —
taking the regression test with it, since the heading would never be seen.

**The scan must visit every heading in the document before resolving the requested label, and must
not break early on a match.** This sentence is the contract, not an implementation hint. Both Node.js
and Python originally broke out of the loop as soon as the requested wave was found, so a malformed
heading **positioned after** the target wave was never reached: the barrier returned exit 1 `blocked`
instead of exit 2. Node.js was corrected in its first pass; Python's own regression test only covered
the "before" position and passed while the bug survived. Measured empirically, not reported:

| Malformed heading position | Expected | Go | Node.js | Python (before fix) |
|---|---|---|---|---|
| Before the target wave | exit 2 | exit 2 | exit 2 | exit 2 |
| **After** the target wave | **exit 2** | exit 2 | exit 2 | **exit 1 `blocked`** |

Any test of this behavior must cover **both positions**. A test at the "before" position alone is
vacuous with respect to the early-break bug.

#### Ordering has no call site — helper is optional

No runtime currently lists or compares waves; `--wave` resolution is exact-match only. The ordering
rule above stays **normative** — it applies the moment a listing surface appears — but implementing a
comparator is **optional** until then. Go has `compareWaveLabels` covered by unit tests, which proves
the rule is implementable; Node.js and Python correctly declined to add one rather than ship dead
code. Do not "fix" this asymmetry in either direction: adding dead comparators to two runtimes is not
parity, and deleting Go's loses the tested proof.

### States

| State | Meaning |
|---|---|
| `pending` | Check declared but not yet evaluated (only ever appears mid-run, and in `--json` when a preceding check aborted the run) |
| `running` | Check currently executing (text output only; never present in a final JSON document) |
| `passed` | Check evaluated and green |
| `blocked` | Check evaluated and red |

The wave-level `status` is `passed` only when **every** check is `passed`; otherwise `blocked`.

### Roadmap parsing rules (string-level — no heuristics)

These are literal parsing rules. All three runtimes must implement them identically.

1. **Wave heading.** A wave starts at a line matching `^## Wave <label> ` (H2, the literal word
   `Wave`, the **label**, then a space). The wave ends at the next `^## ` line or EOF. See
   "Wave label grammar" below — the label is not necessarily an integer.
2. **ML heading.** Inside a wave, an ML starts at a line matching `^### ML-` (H3). The ML ends
   at the next `^### ` or `^## ` line or EOF.
3. **ML completion.** An ML is complete when its body contains a line matching
   `^\*\*Status:\*\*` whose remainder contains `✅`. Any other marker (`⬜`, `🔄`, `❌`) is
   incomplete. Absence of a `**Status:**` line is incomplete.
4. **Acceptance evidence.** Inside an ML, the acceptance block starts at a line matching
   `^\*\*Critérios de aceite:\*\*` and ends at the next `^\*\*` line or at the ML boundary.
   Every line in that block matching `^- \[ \]` is unmet evidence. The ML has evidence only
   when the block exists, is non-empty, and contains zero `- [ ]` lines.
   **An ML with no acceptance block at all is `blocked`, not vacuously passed.**
5. **Wave gates.** Gates are declared per wave by a `**Gates da wave:**` line immediately
   followed by a fenced ```` ```bash ```` block. Each non-empty, non-comment line in that block
   is one gate command, executed from the repository root, in declaration order.
   A wave with no `**Gates da wave:**` block declares zero gates — that is legal and yields a
   `gates` check with `status: "passed"` and an empty `commands` array. The barrier **never**
   invents a gate.
6. **Malformed input.** A wave heading whose number is not parseable, an ML whose body cannot
   be delimited, or an unterminated fence is a usage error (exit 2) with an explicit message
   naming the offending line number — never a silent pass.

### Built-in checks

Evaluated in this fixed order; the run continues through all checks so the report is complete.

| `name` | Passes when |
|---|---|
| `mls_complete` | Wave contains ≥ 1 ML and every ML satisfies rule 3 |
| `acceptance_evidence` | Every ML in the wave satisfies rule 4 |
| `gates` | Every command from rule 5 exits 0 |
| `validate` | `trackfw validate --json` reports `violations: 0` |

`trackfw validate` is invoked in-process (Go/Node/Python each call their own validator), not by
shelling out to a `trackfw` binary that may not be on `PATH`.

### JSON document

```json
{
  "roadmap": "ROADMAP-2026-07-29-example.md",
  "wave": "2",
  "status": "blocked",
  "started_at": "2026-07-29T10:30:00Z",
  "finished_at": "2026-07-29T10:30:04Z",
  "checks": [
    {
      "name": "mls_complete",
      "status": "passed",
      "evidence": ["ML-2A: ✅", "ML-2B: ✅", "ML-2C: ✅"],
      "failures": []
    },
    {
      "name": "acceptance_evidence",
      "status": "blocked",
      "evidence": [],
      "failures": ["ML-2C: 2 unmet acceptance criteria"]
    },
    {
      "name": "gates",
      "status": "passed",
      "commands": ["make quality"],
      "evidence": ["make quality: exit 0"],
      "failures": []
    },
    {
      "name": "validate",
      "status": "passed",
      "evidence": ["0 violations, 0 warnings"],
      "failures": []
    }
  ],
  "failures": ["acceptance_evidence: ML-2C: 2 unmet acceptance criteria"]
}
```

Evidence and failure string formats are **pinned** — the three runtimes must emit these literally,
so that a diff of two runtimes' JSON output for the same fixture is empty:

| Check | `evidence` entry | `failures` entry |
|---|---|---|
| `mls_complete` | `<ML-id>: ✅` | `<ML-id>: not complete (status: <marker or "missing">)` |
| `acceptance_evidence` | `<ML-id>: <n> criteria met` | `<ML-id>: <n> unmet acceptance criteria` or `<ML-id>: no acceptance block` |
| `gates` | `<command>: exit 0` | `<command>: exit <code>` |
| `validate` | `<v> violations, <w> warnings` | `<v> violations, <w> warnings` |

Determinism contract:

- Key order is fixed as shown; `checks` is always in the built-in evaluation order.
- `evidence` and `failures` are always arrays, never `null`, never omitted.
- `commands` is present only on the `gates` check.
- Timestamps are RFC 3339 UTC with second precision.
- The top-level `failures` array is the concatenation of every check's `failures`, each prefixed
  with `<check-name>: `.

### Edge cases not reached by the eight mandated scenarios

These were surfaced while implementing the runtimes. They are pinned here because each is a point
where three independent implementations would otherwise drift silently — no contract test exercises
them, so the parity gate is the only thing that would catch it, and only much later.

| Case | Resolution |
|---|---|
| Acceptance block header present but body empty | Same as absent: check `blocked`, failure `<ML-id>: no acceptance block`. Rule 4 requires the block to be non-empty to count as evidence, so an empty block provides none. |
| Wave contains zero MLs | `mls_complete` is `blocked` with failure exactly `wave <n>: no ML found`. A wave with nothing in it must never release. |
| Wave heading with no title (`## Wave 1` with no trailing text) | Valid. Rule 6 makes only an *unparseable number* a usage error; the title is cosmetic. |
| Gate process terminated by a signal (no numeric exit code) | Recorded as `<command>: exit 1`. The format is defined only for numeric codes, and a signal kill is a failure. |

### `trackfw barrier` vs `/trackfw:barrier`

| | `trackfw barrier` (CLI) | `/trackfw:barrier` (slash command) |
|---|---|---|
| Nature | Deterministic, reproducible, exit-code driven | Orchestration checklist for `trackfw_architect` |
| Runs gates | Yes, only those declared in the roadmap | Delegates to the CLI |
| Invokes agents | **Never** | Yes — `code-quality` and `security` when applicable |
| Audits the diff | No | Yes, human/agent judgement |
| Git operations | **Never** | Only `trackfw_architect`, after a green barrier |

A green CLI barrier is **necessary but not sufficient** to release a wave. The specialist
inspections and diff audit are conditions the binary cannot evaluate.

## `trackfw update` vs `trackfw update harness`

Update is split by **scope**. The split exists because `trackfw update` today mutates global state
(`~/.claude` skill, global Codex deployments) as a side effect of being run inside a project — so a
user visiting twenty repositories re-runs the same global write twenty times, and a project-local
command silently reaches outside the repository.

| | `trackfw update` | `trackfw update harness` |
|---|---|---|
| Scope | The current repository only | The user's global harness (`~/.claude` and equivalents) |
| Requires `trackfw.yaml` / project cwd | Yes | **No** — runs from anywhere |
| Touches global state | **Never** | Yes, that is its only job |
| Typical frequency | Once per repository | Once per machine, per upgrade |

`trackfw update` covers: the trackfw rules block in agent config files, `scripts/trackfw-validate.sh`,
the CI workflow, project-level slash commands, and Git hooks. Any global mutation is removed from
its contract.

**One read-only exception, added for global-ADR discovery:** `trackfw update` inspects (never
writes) `~/.trackfw/adr` — if that directory exists and contains at least one `ADR-*.md`, `update`
surgically appends `~/.trackfw/adr` to the project's own `adr_dirs` in `trackfw.yaml`, idempotently,
preserving every other line/comment in the file byte-for-byte. If the global ADR dir doesn't exist,
is empty, or the entry is already present, `trackfw.yaml` is left untouched — this never fires "in
the dark" against an empty/missing global directory, and it never touches anything outside the
current project's own `trackfw.yaml`.

`trackfw update harness` covers: rules, agents and skills **already installed** in the user's home
directory.

### `trackfw.yaml` fields consumed by `update` and `sync` — single loader, `Update`/`Sync` namespaces

Since `REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis`, all
eleven fields below are read exclusively by the shared config loader (Go `config.Load`, Node.js
`loadConfig`, Python `load_config`) into two typed namespaces — `Update` (5 fields) and `Sync` (6
fields). No module outside the loader opens, reads, or parses `trackfw.yaml` in any of the three
runtimes; the five hand-rolled scanners that used to exist (`ReadUpdateConfig` in Go, the Node.js
`readUpdateConfig`/`readConfigField`, and Python's `_read_config_field`) were removed. The keys stay
**flat at the YAML root**, unchanged from before this refactor — only the internal representation
gained a namespace.

| Field (YAML key) | Namespace | Default (absent) | Consumed by |
|---|---|---|---|
| `hooks` | `Update` | `""` | `trackfw update` — selects which Git hook flavor (`husky`, `lefthook`, native) is (re)generated |
| `ci` | `Update` | `""` | `trackfw update` — selects which CI workflow template is (re)generated |
| `backend` | `Update` | `""` | `trackfw update` — backend stack used when regenerating `CLAUDE.md`/agent-config stack sections and stack-specific hook commands |
| `frontend` | `Update` | `""` | `trackfw update` — frontend stack used the same way as `backend` |
| `pkg_manager` | `Update` | `""` | `trackfw update` — package manager (`npm`, `yarn`, `pnpm`, …) used to compose the build/test commands written into generated hooks and `CLAUDE.md` |
| `linear_api_key` | `Sync` | `""` | `trackfw sync` (Linear) — read first, environment variable is the fallback (AC5 precedence, unchanged) |
| `linear_team_id` | `Sync` | `""` | `trackfw sync` (Linear) — same precedence as `linear_api_key` |
| `jira_base_url` | `Sync` | `""` | `trackfw sync` (Jira) — same precedence |
| `jira_email` | `Sync` | `""` | `trackfw sync` (Jira) — same precedence |
| `jira_token` | `Sync` | `""` | `trackfw sync` (Jira) — same precedence |
| `jira_project` | `Sync` | `""` | `trackfw sync` (Jira) — same precedence |

**Python's `update` did not read these five `Update` fields at all — closed, not a registered
exception.** Before this REQ, `trackfw update` in Go and Node.js decided which hooks/CI to generate
based on `hooks`/`ci`/`backend`/`frontend`/`pkg_manager`; the Python runtime had no reader for them
(`grep -rn pkg_manager pypi/trackfw` returned nothing) and silently produced a different observable
result. This was a functional gap, not a documented exception — it is closed as of this REQ: Python's
`update` now reads all five fields through the same loader and acts on them like Go and Node.js.

**Intentional exception — generated shell hooks keep their own `grep`/`sed` read.** The Git hooks
emitted by `scaffold.go:704,731` (Go), `hooks.js:77,104` (Node.js), and `init_gen.py:790,818`
(Python) extract `roadmap_dir` from `trackfw.yaml` with `grep '^roadmap_dir:' … | sed …` — a sixth,
deliberately separate parsing path. This is not the same defect class as the five scanners removed
above: those ran **inside the CLI binary itself**, where the shared loader was already available and
simply wasn't used. The generated shell runs as a Git hook **without the `trackfw` binary present in
the user's environment** (it fires on the user's `git commit`/`pre-push`, potentially before the CLI
is installed or on a machine that never installs it) — routing it through the loader would mean
shelling out to `trackfw` from inside a hook, which is not guaranteed to exist. It reads only
`roadmap_dir`, is intentionally minimal, and is not part of the `Update`/`Sync` namespaces above.

### `credential_guard.mode` — `trackfw.yaml` field consumed by `scripts/trackfw-credential-guard.sh`

Since `ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes`, a
nested `credential_guard:` mapping is read from `trackfw.yaml` by the shared config loader in all
three runtimes (`ProjectConfig.CredentialGuard.Mode` in Go, `cfg.credentialGuard.mode` in Node.js,
`cfg["credential_guard"]["mode"]` in Python), the same pattern already used for `link_fields`.

| Field (YAML key) | Type | Default (absent) | Consumed by |
|---|---|---|---|
| `credential_guard.mode` | `warn` \| `block` | `warn` | `scripts/trackfw-credential-guard.sh` — decides whether a detected JWT/AWS-key pattern only logs an attention signal (`warn`) or aborts the tool call with exit code 2 (`block`) |

```yaml
credential_guard:
  mode: block   # optional; default is "warn" when the whole key is absent
```

**Unrecognized `mode` value falls back to `warn` silently** — no fatal error, no stderr message,
consistent with how every other unrecognized-shape field in this parser behaves (e.g.
`roadmap_namespacing`, `forge`). This is a deliberate ML-1A design choice: `credential_guard` is a
single low-stakes enum, not worth a dedicated malformed-config error path shared byte-for-byte
across three YAML libraries (unlike `MalformedConfigMessage`, which exists specifically because
syntax errors must fail the same way in all three CLIs).

**`scripts/trackfw-credential-guard.sh`** (generated by `GenerateCredentialGuardScript` in Go,
`generateCredentialGuardScript` in Node.js/`hooks.js`, `_generate_credential_guard_script` in
Python/`init_gen.py` — byte-identical across the three, proven by
`internal/generators/credential_guard_test.go:TestCredentialGuardScript_ParityAcrossStacks`) reads
`credential_guard.mode` from `trackfw.yaml` itself via a plain `grep`/`sed` extraction — the same
"generated shell keeps its own reader" pattern documented above for `roadmap_dir`, and for the same
reason: the script runs as a CLI hook (`PreToolUse`/`PostToolUse`) potentially without the `trackfw`
binary available in that execution context.

**`warn` mode writes to a dedicated attention file, not the shared one.** When
`credential_guard.mode` is `warn` (the default) and a match is found, the script writes
`$ROADMAP_DIR/.trackfw-credential-guard.json` — a file distinct from
`$ROADMAP_DIR/.trackfw-attention.json`, which is owned exclusively by the pre-existing
`trackfw-attention-signal.sh`/`trackfw-attention-cleanup.sh` pair. Earlier in this ML the
credential-guard warning was written to the shared `.trackfw-attention.json` path; that was corrected
before this ML shipped because `trackfw-attention-cleanup.sh` deletes that path unconditionally
(`rm -f`), and — confirmed against the official Codex CLI hooks documentation
(<https://developers.openai.com/codex/hooks>, retrieved 2026-08-05) — a harness that runs multiple
matching hooks of the same event **concurrently** (Codex CLI does this: "Multiple matching command
hooks for the same event are launched concurrently") can run the cleanup hook (matcher `".*"`) and the
credential-guard hook (matcher `"Bash"`) for the same `PostToolUse[Bash]` invocation at the same time,
letting the cleanup's `rm -f` race the credential-guard's write and delete the warning it just wrote.
Using a dedicated, unshared path removes the race entirely regardless of a given CLI's concurrency
model (sequential or parallel) — no other generated script reads, writes or deletes
`.trackfw-credential-guard.json`. Proven by
`internal/generators/credential_guard_test.go:TestCredentialGuardScript_AttentionCleanupDoesNotDeleteIt`
(and the Node.js/Python equivalents in `npm/tests/credential_guard.test.js` and
`pypi/tests/test_credential_guard.py`), which runs the credential-guard script in `warn` mode followed
by `trackfw-attention-cleanup.sh` and asserts the dedicated file survives. `block` mode never writes
either attention file — it aborts the tool call directly via exit code 2.

As of ML-1A of
`ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`,
the script existed but was not yet wired into any CLI's `hooks.json`/`settings.json`. Wave 2 of the
same roadmap wires it CLI by CLI; ML-2A (Claude Code) and ML-2B (Codex) are done as of this writing —
see below for the Codex wiring specifics. The remaining CLIs (Gemini, Copilot, Cursor, Kiro) are
wired in their own MLs later in Wave 2; the final consolidated support table across all CLIs is
Wave 5 (ML-5A) scope.

#### Codex wiring (ML-2B) — `PreToolUse`/`PostToolUse` matcher `"Bash"`

`InjectCodexHooks` (Go: `internal/generators/agentfiles.go`; Node.js:
`npm/src/generators/hooks.js:injectCodexHooks`; Python:
`pypi/trackfw/generators/hooks.py:inject_codex_hooks`) writes three independent hook events into
`.codex/hooks.json`:

| Event | Matcher | Script | Purpose |
|---|---|---|---|
| `PermissionRequest` | `.*` | `trackfw-attention-signal.sh` | Pre-existing (ML-2A/earlier) — fires only when Codex is about to prompt for approval (shell escalation / managed-network approval); does **not** fire for every command |
| `PreToolUse` | `Bash` | `trackfw-credential-guard.sh` | New — fires for **every** Bash tool call, regardless of whether approval is required |
| `PostToolUse` | `.*` | `trackfw-attention-cleanup.sh` | Pre-existing |
| `PostToolUse` | `Bash` | `trackfw-credential-guard.sh` | New |

Confirmed against the official Codex CLI documentation
(<https://developers.openai.com/codex/hooks>, retrieved 2026-08-05):

- `PreToolUse` intercepts Bash, `apply_patch` file edits, MCP tool calls and other local function
  tools; `matcher` is applied to `tool_name` (canonical value `Bash` for the shell tool). This is
  distinct from `PermissionRequest`, which "runs when Codex is about to ask for approval... It
  doesn't run for commands that don't need approval" — confirming the ADR's premise that
  `PermissionRequest` alone is not a reliable interception point for a guard that must see every
  Bash invocation.
- **Divergence from the ADR's preliminary research**: hooks are **enabled by default** in current
  Codex CLI. The `[features]` key exists to **turn hooks off** (`[features] hooks = false`;
  `codex_hooks` is accepted as a deprecated alias for the same key) — not to opt them in as the ADR's
  preliminary research speculated. No config.toml injection was needed or added for this ML; the
  trackfw-generated `.codex/hooks.json` is picked up automatically by any Codex CLI version with
  hooks enabled (the default).
- `PreToolUse` blocking uses **exit code 2** (reason on `stderr`) or a
  `hookSpecificOutput.permissionDecision: "deny"` JSON response on stdout — the exit-code-2 path
  already matches `trackfw-credential-guard.sh`'s existing `block` mode behavior with no script
  changes required.
- The `hooks.json` top-level schema (`{"hooks": {"<Event>": [{"matcher": "...", "hooks": [{"type":
  "command", "command": "..."}]}]}}`) matches what `InjectCodexHooks`/`injectCodexHooks`/
  `inject_codex_hooks` already produced for `PermissionRequest`/`PostToolUse` before this ML — no
  format migration was needed, only new entries.

Merge/idempotency follows the same pattern established for Claude Code in ML-2A: a pre-existing
third-party entry for the same matcher (e.g. a hand-written `PreToolUse[matcher:"Bash"]` hook) is
merged into (not overwritten or duplicated by) the new `trackfw-credential-guard.sh` command — see
`mergeClaudeHookArray` (Go), the shared `mergeClaudeHookArray` (Node.js), and the new
`_merge_codex_hook_entry` helper (Python, added in this ML to bring Codex's Python injector to
matcher-merge parity with Go/Node — previously it only checked "is this exact command present
anywhere in the array", which would have produced sibling `{"matcher": "Bash", ...}` blocks instead
of merging into an existing one).

#### Gemini CLI wiring (ML-2C) — `BeforeTool`/`AfterTool` matcher `"run_shell_command"`

`InjectGeminiHooks` (Go: `internal/generators/agentfiles.go`; Node.js:
`npm/src/generators/hooks.js:injectGeminiHooks`; Python:
`pypi/trackfw/generators/hooks.py:inject_gemini_hooks`) writes four independent hook group entries into
`.gemini/settings.json`:

| Event | Matcher | Script | Purpose |
|---|---|---|---|
| `Notification` | `ToolPermission` | `trackfw-attention-signal.sh` | Pre-existing — fires only when Gemini CLI is about to prompt for permission; does **not** fire for every tool call |
| `BeforeTool` | `run_shell_command` | `trackfw-credential-guard.sh` | New — fires for **every** shell tool call, regardless of whether a permission prompt is needed |
| `AfterTool` | `*` | `trackfw-attention-cleanup.sh` | Pre-existing |
| `AfterTool` | `run_shell_command` | `trackfw-credential-guard.sh` | New |

Confirmed against the official Gemini CLI documentation
(<https://geminicli.com/docs/hooks/reference>, retrieved 2026-08-05 — fetched via `curl` and stripped
of markup; no WebFetch/WebSearch tool was available in this execution):

- `BeforeTool` "Fires before a tool is invoked. Used for argument validation, security checks, and
  parameter rewriting" — a real pre-execution interception point, distinct from `Notification`
  (`ToolPermission`), which the doc's own matcher-name-vs-lifecycle-event distinction implies only
  fires around permission prompts, not for every tool call (same limitation pattern already confirmed
  for Codex's `PermissionRequest` in ML-2B).
- `BeforeTool` supports `decision: "deny"` (alias `"block"`) and "Exit Code 2 (Block Tool): Prevents
  execution. Uses stderr as the reason" — already compatible with
  `trackfw-credential-guard.sh`'s existing `block` mode (exit 2 + stderr), no script change needed.
- The shell tool's canonical name is **`run_shell_command`**: "For `BeforeTool` and `AfterTool` events,
  the matcher field in your settings is compared against the name of the tool being executed. Built-in
  Tools: You can match any built-in tool (for example, `read_file`, `run_shell_command`)." Matcher is a
  regex evaluated against `tool_name` (doc: "Regex Support: Matchers support regular expressions").
- `AfterTool` "Fires after a tool executes. Used for result auditing, context injection, or hiding
  sensitive output from the agent" — confirms the pre-existing `AfterTool["*"]` wiring is indeed the
  post-execution equivalent already assumed by the code.
- The `settings.json` schema (`{"hooks": {"<Event>": [{"matcher": "...", "sequential": bool?, "hooks":
  [{"type": "command", "command": "..."}]}]}}`) matches what `InjectGeminiHooks`/`injectGeminiHooks`/
  `inject_gemini_hooks` already produced for `Notification`/`AfterTool` before this ML — no format
  migration was needed, only new group entries. The optional `sequential` field is not set by trackfw
  (defaults to unspecified/parallel-by-omission per the doc's `hooks` array default).
- Doc lists the tool-hook events as `Stable` (no preview/experimental marker found in the fetched
  reference page), unlike some other Gemini CLI hook categories — no minimum-version caveat is
  documented for `BeforeTool`/`AfterTool` specifically.

**Concurrency across matcher groups (explicitly investigated per this ML's brief):** the doc defines a
`sequential` field, but scoped **within one matcher group only** — "If `true`, hooks in this group run
one after another. If `false`, they run in parallel." It does not document ordering **between two
different matching groups** for the same event and the same `tool_name` (e.g. `AfterTool["*"]` vs.
`AfterTool["run_shell_command"]`, both matching a shell-tool call). No concurrency model is assumed
here for that case — it is left undocumented rather than guessed. This does not create the
Codex-style race found in ML-1A/ML-2B regardless of the real answer: `trackfw-credential-guard.sh`'s
`warn` mode writes only to its own dedicated `$ROADMAP_DIR/.trackfw-credential-guard.json`, a path no
other generated script (including `trackfw-attention-cleanup.sh`) reads, writes or deletes — so even if
Gemini CLI runs `AfterTool["*"]` and `AfterTool["run_shell_command"]` concurrently for the same call,
there is no shared file for them to race over.

Merge/idempotency follows the same `mergeClaudeHookArray`/`_merge_claude_hook_array` pattern as Claude
Code and Codex. The Python injector was rewritten in this ML to use the shared
`_merge_claude_hook_array` helper (already used by `inject_claude_hooks` in the same file) instead of
a bespoke "does any entry anywhere contain this command" check it previously used — the same class of
divergence ML-2A fixed in Go and ML-2B fixed in Python for Codex, which would otherwise append a
second `{"matcher": "run_shell_command", ...}` group instead of merging into an existing third-party
one. Side effect of that rewrite: the `name`/`timeout: 10000` fields the Python injector previously
wrote into Gemini hook entries (fields Go/Node never wrote) were dropped, so all three stacks now
produce the same entry shape (`{"matcher", "hooks": [{"type", "command"}]}`) — informational-only
fields were traded for exact cross-stack structural parity ahead of ML-3A's structural gate.

**Known gap, found during this ML — fixed in a dedicated follow-up commit right after ML-2C:**
`GenerateCredentialGuardScript` (Go) / `generateCredentialGuardScript` (Node.js) /
`_generate_credential_guard_script` (Python) — the functions that actually write
`scripts/trackfw-credential-guard.sh` to disk — were not called from any real command flow
(`trackfw init`/`discover`/`update`) in any of the three stacks; only tests invoked them directly.
Every hook wired so far (Claude Code, Codex, Gemini) pointed at a script that was never generated by
the CLI itself in normal usage. This pre-dated ML-2C (already true for ML-2A/ML-2B). **Fixed** in
commit `6b267c4` (`fix(hooks): conecta geracao do trackfw-credential-guard.sh aos fluxos reais`):
call sites added alongside `GenerateAttentionScripts`/equivalents in `internal/generators/scaffold.go`,
`update.go`, `internal/discover/discover.go` (Go), `npm/src/generators/init.js`,
`npm/src/commands/discover.js`/`update.js` (Node.js), and `pypi/trackfw/generators/hooks.py`
(`inject_hooks_detected`) + `init_gen.py`/`pypi/trackfw/commands/discover.py` (Python), including an
upgrade-scenario test (`trackfw update` backfilling the script for a pre-existing project). Confirmed
end-to-end by the orchestrator: `trackfw init` in a fresh directory generates the executable script
and the `Bash`-matcher wiring together.

#### GitHub Copilot wiring (ML-2D) — `.github/hooks/trackfw-attention.json` format correction + matcher `"bash"`

`InjectCopilotHooks` (Go: `internal/generators/agentfiles.go`; Node.js:
`npm/src/generators/hooks.js:injectCopilotHooks`; Python:
`pypi/trackfw/generators/hooks.py:inject_copilot_hooks`) writes a dedicated (overwritten wholesale, same
pattern as Kiro) `.github/hooks/trackfw-attention.json`.

**Format divergence found and corrected.** Before this ML, Go and Node.js emitted
`{"hooks": [{"event": "preToolUse", "run": "..."}, {"event": "postToolUse", "run": "..."}]}`, while
Python emitted `{"version": 1, "hooks": {"preToolUse": [{"type": "command", "bash": "...", "cwd": ".",
"timeoutSec": 10}], "postToolUse": [...]}}`. Confirmed against the official documentation
(<https://docs.github.com/en/copilot/reference/hooks-reference>, retrieved 2026-08-05 via `curl` of the
page's embedded Next.js `renderedPage` JSON, stripped of markup):

- "Repository-level hook files — `.github/hooks/*.json` in the repository root" — files use "JSON
  format with version 1", schema `{"version": 1, "hooks": {"<event>": [<entry>, ...]}}`.
- The documented Command-hook entry shape is `{"type": "command", "bash": "YOUR_BASH_COMMAND",
  "powershell": "YOUR_POWERSHELL_COMMAND", "cwd": "OPTIONAL/WORKING/DIRECTORY", "env": {"VAR": "VALUE"},
  "timeoutSec": 30}` — this is **exactly** the shape Python already used (`type`/`bash`/`cwd`/
  `timeoutSec`), confirming Python was correct. The `{"hooks": [{"event", "run"}]}` array-of-flat-objects
  shape Go/Node emitted does not match any format documented by GitHub and was not a legacy/deprecated
  variant found anywhere in the retrieved doc — it appears to have been an unverified guess baked in
  before this REQ. **Go and Node were aligned to Python's pre-existing (correct) format in this ML.**

**Matcher — real support confirmed, contrary to the pre-ML assumption that Copilot had none.** The
"Matcher filtering" table lists `preToolUse -> toolName` and `postToolUse -> toolName` — "Optional regex
tested against `toolName`... compiled as `^(?:PATTERN)$`... must match the entire tool name." A worked
example is shown inline on a `postToolUse` command entry: `{"type": "command", "matcher": "bash|edit",
"bash": "./scripts/log-tool.sh"}`. The per-field reference table for command hooks (`bash`/`command`/
`cwd`/`env`/`powershell`/`timeout`/`timeoutSec`/`type`) does not itself list `matcher` as a field, even
though the matcher-filtering section documents and shows it — this is treated as defensive evidence, not
a blocker: per the doc's own malformed-item handling ("If a hook configuration file... contains a
malformed hook item, only that item is dropped and logged"), if some Copilot version silently rejected
`matcher` as an unknown field, the whole entry (not just the field) would be the risk. `matcher: "bash"`
is used on both new `preToolUse`/`postToolUse` credential-guard entries to scope them to the shell tool,
as a hardening layer on top of — not a replacement for — `trackfw-credential-guard.sh`'s own raw-payload
JWT/AWS-key scan (ML-1A), which does not depend on any specific field name and would still work as a
no-op-when-no-match filter even if the matcher were ignored by a given Copilot version.

**Tool name casing depends on event-name casing (camelCase vs PascalCase), not fixed.** The doc: "Two
payload formats are supported, selected by the event name used in the hook configuration: camelCase
format... Fields use camelCase [and] `toolName` [carries] the runtime tool name" vs. "VS Code compatible
format — Configure the event name in PascalCase (for example, `SessionStart`). Fields use snake_case...
Payloads for PascalCase `PreToolUse` report `tool_name` as the Claude tool name (for example, `Bash`, not
`bash`)." The tool-name mapping table lists the shell tool's **runtime** name as `bash` (lowercase). Since
this wiring uses camelCase event keys (`preToolUse`/`postToolUse`, matching the pre-existing
signal/cleanup entries), `matcher: "bash"` (lowercase) is correct — using `"Bash"` (the PascalCase/Claude
name) would silently never match under this event-casing scheme. `trackfw-credential-guard.sh` was
inspected directly to confirm it does not depend on this distinction either way: it greps the *entire*
raw stdin payload for the JWT/AWS-key regex and a redirect-target heuristic, with no field-name lookup at
all — so the payload-shape choice affects only the matcher's scoping precision, never detection
correctness.

**Concurrency (explicitly investigated per this ML's brief) — the most definitive answer found across
all CLIs wired so far.** The doc states plainly: "If multiple hooks of the same type are configured,
they execute in order." Unlike Codex (confirmed concurrent, ML-2B) or Gemini (undocumented cross-group
model, ML-2C), Copilot hooks for the same event run **serially, in configuration order** — no race is
possible between `trackfw-attention-cleanup.sh` (index 0 in `postToolUse`) and
`trackfw-credential-guard.sh` (index 1) here even setting aside the ML-1A dedicated-file fix. Related
exit-code behavior worth flagging for anyone editing the script later: "Command `preToolUse` hooks are
fail-closed on errors — a crash or non-zero exit (including exit 2) denies the tool call, even if the
hook's stdout JSON reports `permissionDecision: "allow"`" — so any future bug causing
`trackfw-credential-guard.sh` to exit non-zero for reasons unrelated to a real "block" decision would
deny the tool call under Copilot, not just genuine `credential_guard.mode: block` detections. Timeouts,
by contrast, are always fail-open per the same section.

Idempotency: the file is regenerated wholesale on every call (same "dedicated file, safe overwrite"
pattern as Kiro's `trackfw-attention.json`) — no merge helper is needed because trackfw is the sole owner
of this filename and always emits the same two events/four entries.

Cross-stack structural parity (Go vs. Node.js vs. Python) is covered by
`internal/generators/copilot_hooks_parity_test.go` (`TestInjectCopilotHooks_StructuralParityAcrossStacks`),
which invokes each stack's real `injectCopilotHooks`/`inject_copilot_hooks` implementation as a
subprocess (Node via `node -e`-equivalent script, Python via `python3 -c`) and compares the resulting
JSON structurally (event keys, entry count, `bash`/`type`/`matcher` fields) rather than byte-for-byte,
since each stack's own JSON serializer is free to choose its own formatting.

#### Cursor wiring (ML-2E) — `.cursor/hooks.json`, `hooks.beforeShellExecution`/`hooks.afterShellExecution`

`InjectCursorHooks` (Go: `internal/generators/agentfiles.go`; Node.js:
`npm/src/generators/hooks.js:injectCursorHooks`; Python:
`pypi/trackfw/generators/hooks.py:inject_cursor_hooks`) merges into `.cursor/hooks.json`, which is
read-modify-write (not a dedicated/overwritten file, same pattern as Claude/Codex/Gemini).

**UPDATE (2026-08-06, follow-up ML — see `ROADMAP-2026-08-06-corrige-divergencia-de-versao-pypi-e-schema-legado-de-hooks-do-cursor.md`, ML-1B):
the "not a real event" finding below was time-bound and has since been superseded — Cursor's own docs
changed underneath it.** The paragraph immediately below (dated 2026-08-05) is kept for the historical
record of what was true at investigation time, but **no longer describes the current behavior**. See
"RESOLUTION" further down for the corrected, current state.

**Pre-existing format was not a real Cursor event, as of 2026-08-05 — historical, superseded below.**
At the time, the pre-existing attention-signal/cleanup wiring wrote to top-level
`preToolUse`/`postToolUse` arrays of `{"command": "..."}` objects. Confirmed against the official
documentation (<https://cursor.com/docs/agent/hooks>, retrieved 2026-08-05 via `curl -L` of the page's
embedded Next.js RSC payload, unescaped and grepped) that this did **not** correspond to any hook event
Cursor exposed at that time: the real config schema is `{"version": 1, "hooks": {"<eventName>": [<entry>,
...]}}`, and the full documented event list at the time was `sessionStart`, `sessionEnd`,
`beforeShellExecution`, `beforeMCPExecution`, `afterShellExecution`, `afterMCPExecution`,
`beforeReadFile`, `afterFileEdit`, `beforeSubmitPrompt`, `preCompact`, `stop`, `beforeTabFileRead`,
`afterTabFileEdit` — no generic `preToolUse`/`postToolUse` event was documented at all. That ML's brief
explicitly scoped fixing this out ("preserve the existing entries, do not migrate them, only add the new
hook in parallel"); it was recorded as a known, unresolved divergence for a future ML.

**RESOLUTION (2026-08-06) — `preToolUse`/`postToolUse`/`postToolUseFailure` are now real, documented
Cursor events; the legacy wiring has been migrated, not removed.** Re-fetching the hooks doc on
2026-08-06 (`https://cursor.com/docs/agent/hooks` now 308-redirects to `https://cursor.com/docs/hooks`;
fetched via plain `curl -sL`, no special headers needed this time, and parsed the same embedded Next.js
RSC JSON payload) shows Cursor added three new generic events since the 2026-08-05 snapshot:
`preToolUse` / `postToolUse` / `postToolUseFailure`, documented as "Generic tool use hooks (fires for all
tools)" — "Called before any tool execution. This is a generic hook that fires for all tool types (Shell,
Read, Write, MCP, Task, etc.). Use matchers to filter by specific tools." `preToolUse`'s documented input
is `{"tool_name": "Shell", "tool_input": {"command": "...", "working_directory": "..."}, "tool_use_id",
"cwd", "model", ...}`; `postToolUse`'s is the same shape plus `tool_output`/`duration`. This is
structurally identical to Claude Code's `PreToolUse`/`PostToolUse` payload (`tool_name`/`tool_input`),
which is exactly the shape `scripts/trackfw-attention-signal.sh` and `trackfw-attention-cleanup.sh`
already parse (`.tool_name`, `.tool_input.question // .tool_input.command`) — **no script changes were
needed**, only re-nesting the existing entries from the top-level array into `hooks.preToolUse` /
`hooks.postToolUse`. `InjectCursorHooks`/`injectCursorHooks`/`inject_cursor_hooks` now write to the
nested location and, for backward compatibility, migrate any known trackfw entry still present in a
pre-migration file's top-level `preToolUse`/`postToolUse` arrays into the new location, deleting the
top-level key once empty — any *unrelated* entry a user added there themselves (those keys were always
inert, since Cursor never actually read them) is left untouched, never deleted on a guess. The `matcher`
field for `preToolUse`/`postToolUse` filters by tool type (e.g. `"Shell|Read|Write"`) and is optional;
intentionally omitted here — the attention signal must fire for every tool use, not a filtered subset,
same reasoning as `beforeShellExecution`'s omitted matcher documented below.

**`beforeShellExecution` confirmed as the real, Bash-specific, pre-execution event.** From the doc's
"Hook Types" reference:

```json
// beforeShellExecution input
{
  "command": "<full terminal command>",
  "cwd": "<current working directory>",
  "sandbox": false
}

// Output
{
  "permission": "allow" | "deny" | "ask",
  "user_message": "<message shown in client>",
  "agent_message": "<message sent to agent>"
}
```

The event's own list entry describes it as "Control shell commands", distinct from
`beforeMCPExecution`/`afterMCPExecution` ("Control MCP tool usage") — this answers the investigation's
first question: yes, `beforeShellExecution` is a real, dedicated, pre-execution shell event, unrelated
to the (non-existent) generic `preToolUse`.

**`afterShellExecution` confirmed as the post-execution shell event — audit-only, no permission
response.** "Fires after a shell command executes; useful for auditing or collecting metrics from
command output." Input adds `output`/`duration` to the same base fields (`command`, `sandbox`); no
`permission`/`allow`/`deny`/`ask` output is documented for it (the command has already run — there is
nothing left to block). Wired here in parallel with `beforeShellExecution`, mirroring the
`PreToolUse`+`PostToolUse` pairing already used for the other CLIs in this wave, so a credential that
only surfaces in captured command *output* (not the command string itself) is still flagged.

**Exit code behavior confirmed to already match `trackfw-credential-guard.sh`'s existing contract — no
script change required.** Per the doc's "Exit code behavior" list: "Exit code `0` - Hook succeeded, use
the JSON output"; "Exit code `2` - Block the action (equivalent to returning `permission: \"deny\"`)";
"Other exit codes - Hook failed, action proceeds (fail-open by default)." A worked minimal example hook
in the same doc exits `0` with **no stdout output at all** (`cat > /dev/null; exit 0`), confirming that
an empty/absent JSON response on exit `0` is valid and does not error — the client defaults to
proceeding. `trackfw-credential-guard.sh` (ML-1A) already exits `2` for `credential_guard.mode: block`
detections (writing the reason to stderr) and exits `0` for everything else (`warn` mode included, which
writes its own warning to the dedicated `.trackfw-credential-guard.json` file, not stdout) — this is
**exactly** Cursor's `deny`/(implicit-)`allow` convention, so the script required zero modification to
be wired under Cursor. Emitting an explicit `{"permission": "allow", "agent_message": "..."}` JSON body
on `warn` (to additionally surface the warning inline to the agent, not just via the polled attention
file) was considered and rejected for this ML: the script is byte-for-byte shared across all six wired
CLIs (`internal/generators/credential_guard_parity_test.go`), and none of the other five parse or expect
JSON on the guard's stdout — adding Cursor-specific stdout output would require either payload-sniffing
logic (fragile, and every other CLI's investigation found the script is intentionally payload-shape
agnostic) or risk polluting stdout for CLIs that do inspect it. Exit-code-only is the simplest option
that is already fully correct per the documented contract.

**Matcher — real, but intentionally omitted here.** A worked example shows `beforeShellExecution`
entries support an optional `"matcher"` field: `{"command": "./scripts/approve-network.sh", "timeout":
30, "matcher": "curl|wget|nc"}`. Unlike the tool-name matchers used for Claude/Codex/Gemini/Copilot,
this `matcher` is a regex evaluated against the **command string itself** — the event is already
shell-specific, so there is no `tool_name` to filter on (this answers the investigation's third
question: no additional tool-type filtering is needed or possible at this level; the event boundary
already does that job). `trackfw-credential-guard.sh` must see every shell command to scan for
JWT/AWS-key patterns, so no `matcher` is set on the entries this ML adds.

**Concurrency — not documented on either retrieved page; not assumed.** Unlike Codex (confirmed
concurrent, ML-2B) or Copilot (confirmed serial-in-order, ML-2D), no statement about the execution
order/model of multiple hooks registered on the same event was found in the Cursor hooks reference
page as retrieved on 2026-08-05 or re-retrieved on 2026-08-06. Not a blocker: every event array this
wiring writes (`hooks.preToolUse`, `hooks.postToolUse`, `hooks.beforeShellExecution`,
`hooks.afterShellExecution`) only ever contains the single trackfw entry for that event, so there is no
same-event multi-hook race to reason about here regardless of Cursor's true concurrency model.

Idempotency and version handling: `version` is set to `1` only if the field is absent from a
pre-existing `.cursor/hooks.json` (a user-provided value, e.g. from a future schema bump, is never
overwritten); all four `hooks.*` arrays are merged via the same flat-array `{command}`-dedup helper
(`mergeSimpleCommandArray`/`hasEntry`/`_has_entry`) — re-running the injector never duplicates entries.
The one-time migration of legacy top-level `preToolUse`/`postToolUse` entries (2026-08-06 ML) is also
idempotent: once a known entry has been migrated out, re-running the injector finds nothing left to
migrate and is a no-op on the top-level keys.

#### Kiro wiring (ML-2F) — `.kiro/hooks/trackfw-attention.json` format correction + `PreToolUse`/`PostToolUse` matcher `"shell"`

`InjectKiroHooks` (Go: `internal/generators/agentfiles.go`; Node.js:
`npm/src/generators/hooks.js:injectKiroHooks`; Python:
`pypi/trackfw/generators/hooks.py:inject_kiro_hooks`) fully overwrites
`.kiro/hooks/trackfw-attention.json` — a dedicated file, owned exclusively by trackfw, with no user
content to preserve (same overwrite pattern documented for Kiro in the Copilot section above as a point
of comparison, and confirmed there as GitHub Copilot's own pattern too).

**Investigation resolved the ADR's open question affirmatively.** Confirmed against the official
documentation — <https://kiro.dev/docs/hooks/>, <https://kiro.dev/docs/hooks/types> and
<https://kiro.dev/docs/hooks/actions/> (all retrieved 2026-08-05, via `curl -L` of each page's embedded
Next.js RSC/HTML payload, since WebFetch/WebSearch were unavailable in this session) — that `PreToolUse`
is a real, distinct trigger, not limited to file/IDE events like `PostFileSave`. The "Available triggers"
table on the hooks overview page lists `PreToolUse`: "Before a tool is about to execute", Can block:
**Yes** — alongside `PostFileSave`/`PostFileCreate`/`PostFileDelete` (Can block: No) and
`PostToolUse`/`SessionStart`/`Stop`/`PostTaskExec` (Can block: No). The dedicated "Pre Tool Use" section
of `hooks/types` confirms: "Triggers when the agent is about to invoke a tool. Can validate and block
tool usage." This is unambiguous: `PreToolUse` intercepts tool invocations — including shell commands —
before execution, resolving the ADR's doubt in favor of implementing the wiring (not re-scoping).

**Pre-existing format was wrong on all three field names — corrected here, same file.** Before this ML,
all three stacks emitted `{"hooks": [{"name", "description", "event": "PreToolUse", "matcher":
{"tool_name": ".*"}, "action": {...}}]}`. None of `"event"` (as a top-level hook field), `matcher` as an
object, or the top-level payload missing `"version"` match the documented schema. The real schema, per
the "Hook file schema" example and the "Field reference" table on the hooks overview page:

```json
{
  "version": "v1",
  "hooks": [
    {
      "name": "example-hook",
      "trigger": "PostFileSave",
      "matcher": "\\.(ts|tsx)$",
      "action": { "type": "command", "command": "npx eslint --fix" }
    }
  ]
}
```

Field reference confirms: `version` (required, currently the string `"v1"`), `hooks[].trigger`
(required, "Event that fires the hook (PascalCase)"), `hooks[].matcher` (optional, "Regex pattern to
filter which events fire this hook. For `PreToolUse`/`PostToolUse`, matches tool name. For file events,
matches file path. Defaults to always-match."). There is no `"event"` field anywhere in this schema, and
`matcher` is always a scalar regex string, never an object. Because this file is fully owned/overwritten
by trackfw (unlike the Claude/Codex/Gemini/Cursor merge targets), this ML corrects **all** entries in the
file — including the pre-existing `trackfw-attention-signal`/`trackfw-attention-cleanup` hooks, which had
never used a valid field shape and (per the schema) would very likely never have fired in a real Kiro
installation — to `trigger`/scalar-`matcher`/`version: "v1"`, rather than leaving a known-invalid legacy
shape sitting next to newly-added, schema-correct entries in the same array. This mirrors the ML-2D
precedent for GitHub Copilot (also a fully-owned file, also realigned wholesale once the real schema was
confirmed), and differs from the ML-2E precedent for Cursor (a merge target with real user content,
where the legacy-but-wrong entries were deliberately left untouched and only documented).

**Matcher vocabulary and the shell tool's identifier.** The "Pre Tool Use" section of `hooks/types`
documents the `matcher` vocabulary for tool hooks precisely: built-in categories `read`/`write`/`shell`/
`web`/`spec` (`shell` = "all built-in shell command-related tools"), `*` for "all tools (built-in and
MCP)", `@mcp`/`@powers`/`@builtin` source prefixes (regex-matched), and canonical tool names with
aliases — explicitly worked example: `"execute_bash"` or `"shell"` — "Match shell command execution".
`.*` (a regex wildcard, previously emitted by all three stacks for the pre-existing signal/cleanup hooks)
does **not** appear anywhere in this vocabulary; `*` (a literal asterisk, "all tools") is the documented
match-everything value and is what this ML uses for the realigned signal/cleanup entries.
`trackfw-credential-guard-pre`/`-post` use `matcher: "shell"` (the broader category alias, matching every
built-in shell tool) rather than the single canonical id `"execute_bash"`, since the guard's purpose is
to see every shell invocation, not one specific tool identifier.

**Blocking contract — stricter than Claude Code/Codex/Gemini, already satisfied without a script
change.** Per `hooks/actions` (CLI tab): "If the command returns an exit code of `0` indicating success,
the stdout output of the command is added to the agent's context. If the command returns any other exit
code, the stderr output of the command is sent to the agent, and the agent is notified that the hook
returned an error. Additionally, in the case of the Pre Tool Use hook, the tool invocation is blocked."
Unlike Claude Code/Codex/Gemini (which key specifically on exit code `2`), Kiro blocks a `PreToolUse`
command hook on **any** non-zero exit code. `trackfw-credential-guard.sh` (ML-1A) was re-audited against
this stricter contract: every normal-operation exit path in the script is an explicit `exit 0`
(no-op/non-match/ephemeral-redirect/`warn` mode after logging) or `exit 2` (`block` mode) — there is no
code path that intentionally returns `exit 1` or any other non-zero value, so `warn` mode never
spuriously blocks a Kiro tool call. The only residual risk is an unguarded environment failure under the
script's `set -euo pipefail` (e.g. `mkdir -p` failing on a read-only filesystem) aborting with a
non-explicit exit code — this is a generic script-authoring risk shared by every trigger/CLI this hook is
wired into, not a hazard specific to Kiro's stricter any-non-zero-blocks semantics.

**STDIN payload.** `PreToolUse`/`PostToolUse` command actions receive JSON on stdin:
`{"hook_event_name", "cwd", "session_id", "tool_name", "tool_input"}` (confirmed by worked examples on
both `hooks/types` and the hooks overview page). `trackfw-credential-guard.sh` scans the raw payload text
for JWT/AWS-key patterns regardless of field names (ML-1A design decision), so it works unmodified under
this shape.

**Wired entries.** `PreToolUse`/`matcher: "shell"` and `PostToolUse`/`matcher: "shell"`, both pointing at
`scripts/trackfw-credential-guard.sh`, added alongside the schema-corrected
`trackfw-attention-signal` (`PreToolUse`/`matcher: "*"`) and `trackfw-attention-cleanup`
(`PostToolUse`/`matcher: "*"`) entries, in the same `hooks` array. Idempotent: the file is always fully
regenerated with the same four entries, so re-running the injector never duplicates or drifts.

#### Suporte por CLI — visão consolidada, escopo DE PROJETO (ML-5A, `ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`)

> Não confundir com a seção "Suporte por CLI — visão consolidada, escopo GLOBAL (ML-5A)" mais abaixo
> neste mesmo documento — mesmo rótulo `ML-5A`, mas de um roadmap diferente e posterior
> (`ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md`),
> que consolida o escopo GLOBAL (`trackfw update harness`), não o escopo de projeto documentado aqui.

Consolida, numa única tabela, o wiring já detalhado CLI a CLI acima (seções "wiring (ML-2x)") e no
gate estrutural (ML-3A, ver "Agent hooks por CLI ... — paridade estrutural" mais abaixo neste
documento). Nenhum dado novo é introduzido aqui — cada célula linka de volta para a seção que o
documenta com a fonte primária (doc oficial do CLI) e o teste que o comprova.

| CLI | Evento pré-execução | Evento pós-execução | Matcher/filtro | Bloqueio | Sabotagem e2e testada? |
|---|---|---|---|---|---|
| Claude Code | `PreToolUse` | `PostToolUse` | `matcher: "Bash"` (contra `tool_name`) | `exit 2` (`block`) | **Sim** — ML-4A |
| Codex ([ML-2B](#codex-wiring-ml-2b--pretoolusepostotooluse-matcher-bash)) | `PreToolUse` | `PostToolUse` | `matcher: "Bash"` (contra `tool_name`); distinto de `PermissionRequest` (só dispara em prompts de aprovação, não em todo comando) | `exit 2` + stderr, ou `hookSpecificOutput.permissionDecision: "deny"` | Não — doc oficial não expõe um exemplo de payload de stdin em runtime |
| Gemini CLI ([ML-2C](#gemini-cli-wiring-ml-2c--beforetoolaftertool-matcher-run_shell_command)) | `BeforeTool` | `AfterTool` | `matcher` regex `"run_shell_command"` (contra `tool_name`) | `exit 2` (`decision: "deny"`/`"block"`) | Não — doc oficial não expõe um exemplo de payload de stdin em runtime |
| GitHub Copilot ([ML-2D](#github-copilot-wiring-ml-2d--githubhookstrackfw-attentionjson-format-correction--matcher-bash)) | `preToolUse` | `postToolUse` | `matcher: "bash"` (contra `toolName`, regex ancorado `^(?:PATTERN)$`) | qualquer exit ≠ 0 bloqueia (`preToolUse` é fail-closed) | Não — doc confirma só o nome de um campo (`toolName`); payload completo de stdin não confirmado, formato depende do casing do evento (camelCase/PascalCase) |
| Cursor ([ML-2E](#cursor-wiring-ml-2e--cursorhooksjson-hooksbeforeshellexecutionhooksaftershellexecution)) | `beforeShellExecution` | `afterShellExecution` | nenhum — evento já é shell-specific (o guard precisa ver todo comando) | `exit 2` = `deny` (ou JSON `{"permission": "deny", ...}`) | **Sim** — ML-4A |
| Kiro ([ML-2F](#kiro-wiring-ml-2f--kirohookstrackfw-attentionjson-format-correction--pretoolusepostotooluse-matcher-shell)) | `PreToolUse` | `PostToolUse` | `matcher: "shell"` (categoria de tool, alcança todo tool de shell built-in) | qualquer exit ≠ 0 bloqueia `PreToolUse` — mais estrito que os demais CLIs (não só `exit 2`) | **Sim** — ML-4A |
| Windsurf | — | — | — | **Fora de escopo** — sem hook nativo pré-execução no CLI real, confirmado por `REQ-2026-06-20-attention-hooks-agent-clis.md` e pelo comentário já existente em `internal/generators/agentfiles.go` | N/A |

**Cobertura de sabotagem end-to-end: 3 de 6 CLIs** (Claude Code, Cursor, Kiro). Codex, Gemini CLI e
GitHub Copilot ficaram sem teste de sabotagem — não por omissão, mas porque a documentação oficial
recuperada para cada um deles (citada nas seções "wiring" respectivas) não expõe, com confiança
suficiente, um exemplo completo do payload JSON que chega via **stdin em runtime** ao hook (distinto do
formato do arquivo de configuração `hooks.json`/`settings.json`, esse sim confirmado para os 6). Ver
ML-4A no roadmap para o detalhamento CLI a CLI dessa decisão.
`trackfw-credential-guard.sh` é comprovadamente agnóstico a nomes de campo (varre o payload bruto
inteiro via regex), então a ausência de teste nesses 3 CLIs é uma lacuna de **evidência documentada**,
não uma lacuna de cobertura de detecção real.

##### Achados transversais (Waves 2-4)

1. **Race de concorrência do Codex.** A documentação oficial do Codex CLI
   (`developers.openai.com/codex/hooks`) confirma que hooks do mesmo evento com matchers diferentes
   batendo no mesmo `tool_name` rodam **concorrentemente** — no wiring do Codex,
   `PostToolUse[".*"]` (cleanup do attention-signal) e `PostToolUse["Bash"]` (credential-guard) colidem
   numa mesma chamada Bash, permitindo que o `rm -f` do cleanup apagasse o aviso do credential-guard
   escrito na mesma invocação. Corrigido decouplando o modo `warn` do credential-guard para um arquivo
   dedicado, `$ROADMAP_DIR/.trackfw-credential-guard.json`, que nenhum outro script gerado toca —
   elimina a race independentemente do modelo de concorrência de cada CLI (ver seção
   `credential_guard.mode` acima).
2. **Bug crítico: o script nunca era gerado em fluxo real.** `GenerateCredentialGuardScript`/
   `generateCredentialGuardScript`/`_generate_credential_guard_script` — as funções que de fato escrevem
   `scripts/trackfw-credential-guard.sh` em disco — não eram chamadas por nenhum comando real
   (`trackfw init`/`discover`/`update`) nos 3 stacks até a auditoria do ML-2C detectar o problema; todo
   o wiring feito em ML-2A/2B/2C apontava para um script que só existia se algo o gerasse manualmente
   por teste. **Corrigido em commit dedicado** logo após o ML-2C (Go: `scaffold.go:Scaffold`,
   `update.go:Update` + `runProjectTarget("agent-hooks")`, `discover.go:InstallGates`; Node.js:
   `init.js:scaffold`, `discover.js`, `update.js`; Python: `hooks.py:inject_hooks_detected` +
   `init_gen.py:scaffold` + `discover.py`) — confirmado end-to-end pelo orquestrador (`trackfw init`
   num diretório novo gera o script executável e o wiring com matcher `Bash`).
3. **Divergências de paridade Go/Node/Python corrigidas por CLI, durante as Waves 2-3:**
   - **Codex (ML-2B):** o merge do Python (`inject_codex_hooks`) só checava presença do comando em
     qualquer lugar do array, sem mesclar num matcher já existente — corrigido com o novo helper
     `_merge_codex_hook_entry`.
   - **Gemini CLI (ML-2C):** o Python (`inject_gemini_hooks`) usava checagem inline em vez do helper
     compartilhado `_merge_claude_hook_array`; reescrito para paridade real de merge. Efeito colateral:
     os campos `name`/`timeout: 10000`, que só o Python escrevia, foram removidos.
   - **GitHub Copilot (ML-2D):** Go e Node.js emitiam um formato inteiro incorreto
     (`{"hooks": [{"event", "run"}]}`, sem correspondência na doc oficial do GitHub); Python já usava o
     formato correto (`{"version": 1, "hooks": {"<event>": [...]}}`) — Go e Node.js foram realinhados a
     Python.
   - **Kiro (ML-2F):** os 3 stacks emitiam um schema legado incorreto (campo `event` em vez de
     `trigger`, `matcher` como objeto em vez de string, `version` ausente) — corrigido nos 3 stacks
     simultaneamente, já que o arquivo é 100% owned/overwritten pelo trackfw.
   - **ML-3A (gate estrutural):** a primeira execução do gate encontrou uma divergência adicional não
     capturada nos MLs acima — `_merge_codex_hook_entry` (Python) decorava as entradas do Codex com
     `timeout`/`statusMessage`, campos que Go/Node nunca escreveram; removido.
4. **RESOLVIDO em 2026-08-06 (ML-1B do `ROADMAP-2026-08-06-corrige-divergencia-de-versao-pypi-e-schema-legado-de-hooks-do-cursor.md`).**
   O item abaixo descreve o estado como estava nesta REQ (2026-08-05) — mantido para o histórico, mas
   **já corrigido**. Entre a investigação original e o ciclo seguinte, a documentação oficial do Cursor
   passou a documentar `preToolUse`/`postToolUse`/`postToolUseFailure` como eventos genéricos reais
   ("fires for all tool types"). O wiring legado foi migrado do nível raiz para
   `hooks.preToolUse`/`hooks.postToolUse` (schema real), preservando compatibilidade com arquivos
   pré-migração (entradas conhecidas do trackfw são migradas; entradas de usuário não relacionadas no
   nível raiz são preservadas intactas). Ver "Cursor wiring (ML-2E)" acima, seção "RESOLUTION
   (2026-08-06)", para a investigação e evidência completas. Descrição original do achado, para
   contexto histórico: o wiring do attention-signal/cleanup do Cursor usava um schema
   (`{"preToolUse": [...], "postToolUse": [...]}` no nível raiz) que não correspondia a nenhum evento
   documentado do Cursor real na época (a lista completa de eventos documentados em 2026-08-05 não
   incluía `preToolUse`/`postToolUse` genérico algum). Deixado intacto por instrução explícita do ML
   original (preservar, não migrar).
5. **Cobertura de teste de sabotagem end-to-end: 3 de 6 CLIs** (Claude Code, Cursor, Kiro) — ver tabela
   e nota acima; Codex, Gemini CLI e GitHub Copilot ficaram sem teste por falta de confiança suficiente
   no schema do payload de stdin em runtime documentado publicamente para cada um.

### States

Both commands report one state per target. These four strings are pinned:

| State | Meaning |
|---|---|
| `updated` | Target existed and was rewritten to the current template |
| `skipped` | Target existed and was already current, or is unmanaged and must not be overwritten |
| `missing` | Target is not installed. **Not an error** — see below |
| `failed` | Target exists but the write failed; carries a message |

**`missing` never installs.** A target that is not present is reported and left alone unless
`--install-missing` is passed explicitly. A `trackfw update harness` run on a machine where nothing
is installed reports every target as `missing` and exits **0** — "nothing to do" is a successful
outcome, not a usage error. Exit is non-zero only when at least one target is `failed`.

### Flags

| Flag | Applies to | Behaviour |
|---|---|---|
| `--dry-run` | both | Compute and report states without writing anything |
| `--json` | both | Emit the result document instead of the text report |
| `--targets` | both | Comma-separated subset of target ids; unknown id is a usage error |
| `--install-missing` | both | Allow `missing` targets to be installed instead of merely reported |

### JSON document

```json
{
  "scope": "harness",
  "dry_run": false,
  "targets": [
    {"id": "claude-skill", "state": "updated", "path": "~/.claude/skills/trackfw/SKILL.md"},
    {"id": "codex-agents", "state": "missing", "path": "~/.codex/agents"}
  ],
  "summary": {"updated": 1, "skipped": 0, "missing": 1, "failed": 0}
}
```

`scope` is `"project"` or `"harness"`. Key order is fixed as shown; `targets` follows the declared
target order, not filesystem order. `summary` always carries all four counters, including zeros.
`message` is present **only** when `state == "failed"`, positioned after `path`.

### Declared project targets — pinned list

`trackfw update` declares this fixed sequence of 5 ids, in this exact order:
`agent-rules`, `agent-hooks`, `codex-project-agents`, `validate-script`, `claude-commands`.

All three runtimes declare all five. A runtime that cannot manage a target still declares it and
reports its honest state — silently shortening the list makes the JSON incomparable across runtimes.

### `updated` vs `skipped` — the discriminator is content, not action

`updated` means the target's content **actually changed**. A target that already matches the current
template is `skipped`, even if the implementation rewrote the bytes. Deciding by "did I call write()"
instead of "did the content change" makes an idempotent re-run report `updated` in one runtime and
`skipped` in another for the same input — measured divergence between Go and Node.js in the first
Wave 6 round.

### Declared harness targets — pinned list

The harness target list is **not** derived at runtime; it is this fixed sequence of 27 ids, in this
exact order: `claude-skill`, `claude-credential-guard` (global-scope credential-guard wiring for
Claude Code — `ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md`,
ML-2A), `claude-agents`, `claude-skills`, `codex-credential-guard` (same wave, ML-2B — global-scope
credential-guard wiring for Codex CLI, `~/.codex/hooks.json`), `codex-agents`, `codex-skills`,
`gemini-credential-guard` (same wave, ML-2C — global-scope credential-guard wiring for Gemini CLI,
`~/.gemini/settings.json`, `BeforeTool`/`AfterTool[matcher:"run_shell_command"]`), `gemini-agents`,
`gemini-skills`, `antigravity-agents`, `antigravity-skills`, `cursor-credential-guard` (same wave,
ML-2D — global-scope credential-guard wiring for Cursor, `~/.cursor/hooks.json`,
`hooks.beforeShellExecution`/`hooks.afterShellExecution`, each entry a flat `{"command":"..."}`
object — no per-entry matcher, unlike Claude/Codex/Gemini's nested `{matcher,hooks:[{type,command}]}`
shape), `cursor-agents`, `cursor-skills`, `copilot-credential-guard` (same wave, ML-2E — global-scope
credential-guard wiring for GitHub Copilot, `~/.copilot/settings.json`,
`hooks.preToolUse`/`hooks.postToolUse[matcher:"bash"]` — see "GitHub Copilot global-scope wiring
(ML-2E)" below), `copilot-agents`, `copilot-skills`, `windsurf-agents`, `windsurf-skills`,
`amazonq-agents`, `amazonq-skills`, `opencode-agents`, `opencode-skills`, `kiro-credential-guard`
(same wave, ML-2F — global-scope credential-guard wiring for Kiro, a DEDICATED file at
`~/.kiro/hooks/trackfw-credential-guard.json` — see "Kiro global-scope wiring (ML-2F)" below),
`kiro-agents`, `kiro-skills`. Each `<tool>-credential-guard` id (where it exists) is always
positioned immediately BEFORE that tool's own `<tool>-agents`/`<tool>-skills` pair, never after —
`kiro-credential-guard` is the last credential-guard target of this wave (Windsurf has no native
hook mechanism and stays out per the ADR).

### GitHub Copilot global-scope wiring (ML-2E) — `~/.copilot/settings.json`, inline `hooks` field

**Investigation, confirmed 2026-08-06** against
`https://docs.github.com/en/copilot/reference/hooks-reference` (the `hooks-configuration` URL the ADR
originally cited 301-redirects here — same page used for the project-scope investigation, section
"Hooks locations"): the user/global scope offers two distinct mechanisms —

1. A **dedicated directory** of standalone hook files: "`*.json` files in the user-level hooks
   directory. By default this is `~/.copilot/hooks/` on macOS and Linux... If `COPILOT_HOME` is set,
   it is `$COPILOT_HOME/hooks/`" — structurally the user-scope analog of `.github/hooks/*.json`
   (dedicated, safe to overwrite wholesale, same as Kiro's own dedicated hook file at project scope).
2. An **inline `hooks` field in a general config file**: "Inline hooks block in user-level config —
   the hooks field at the top level of `~/.copilot/settings.json`."

This ML follows the roadmap's explicit instruction and targets option 2, `~/.copilot/settings.json`.
The doc confirms this file is **not** dedicated to hooks — it is Copilot CLI's general user config
file (holds other settings such as model choice), unlike `.github/hooks/trackfw-attention.json`
(project scope). So `copilot-credential-guard` **merges** into `root["hooks"]["preToolUse"/
"postToolUse"]` only, preserving every other top-level key — the same discipline
`claude-credential-guard`/`codex-credential-guard`/`gemini-credential-guard` already apply to their
own general `~/.claude/settings.json`/`~/.codex/hooks.json`/`~/.gemini/settings.json` files (Cursor is
the outlier: its `~/.cursor/hooks.json` is itself a dedicated hooks file, hence the `"version":1`
wrapper `cursor-credential-guard` adds).

**Entry shape — same as project scope, no divergence found.** "Hook configuration files use JSON
format with version 1" is stated without carving out an exception for the inline `hooks` field, and no
example anywhere in the doc shows a different command-entry shape for `settings.json` than for
standalone hook files. `copilot-credential-guard` therefore reuses the exact same command-entry shape
`InjectCopilotHooks` (agentfiles.go, project scope) already emits:
`{"type":"command","matcher":"bash","bash":"<absolute path>","cwd":".","timeoutSec":10}`, written under
`hooks.preToolUse`/`hooks.postToolUse`.

**One deliberate non-divergence from the doc's own dedicated-file examples: no top-level `"version"`
key added.** Every JSON example in the doc that shows `"version":1` at the root is an example of a
*dedicated* hooks file (`.github/hooks/*.json`, policy files) — none of them is an example of
`settings.json` itself. Since this code does not own every key of `settings.json` (it is a shared,
general config file), adding an unconfirmed top-level key would be an assumption beyond what the
source confirms; this mirrors how `claude-credential-guard`/`codex-credential-guard`/
`gemini-credential-guard` never add a `"version"` key to their own general settings files either.

**Codex hooks default-enabled, confirmed 2026-08-06 (ML-2B):** ROADMAP-2026-08-06's ADR flagged an
unresolved contradiction between two sources on whether Codex CLI hooks require
`[features] codex_hooks = true` as an explicit opt-in. Re-fetched directly from
`https://developers.openai.com/codex/hooks` on 2026-08-06: "Hooks are enabled by default. To turn
them off in `config.toml`, set: `[features] hooks = false`. Use `hooks` as the canonical feature key.
`codex_hooks` still works as a deprecated alias." `https://developers.openai.com/codex/config-
advanced` (same fetch date) has no conflicting requirement. This resolves the contradiction with high
confidence: no opt-in flag is needed for either project-scope (`.codex/hooks.json`,
`InjectCodexHooks`) or global-scope (`~/.codex/hooks.json`, `codex-credential-guard`) hook wiring —
`codex_hooks`/`hooks` is only ever used to turn hooks OFF.

### Kiro global-scope wiring (ML-2F) — `~/.kiro/hooks/trackfw-credential-guard.json`, dedicated file

**Format, confirmed 2026-08-06** against `https://kiro.dev/changelog/cli/2-13/` (re-fetched via
`curl -L`, same RSC/HTML retrieval method the project-scope `InjectKiroHooks` investigation used):
"Hooks placed in `~/.kiro/hooks/` now fire in every workspace automatically ... Workspace-level hooks
continue to work alongside global ones." This confirms `~/.kiro/hooks/` is a **directory of
one-file-per-hook**, the global-scope analog of the project-scope `.kiro/hooks/*.json` files — not a
single general settings file shared with other CLI config, unlike
`claude-credential-guard`/`codex-credential-guard`/`gemini-credential-guard`/`copilot-credential-guard`
(each of which merges into that tool's own general settings file). `kiro-credential-guard` therefore
writes a **dedicated** file, `~/.kiro/hooks/trackfw-credential-guard.json`, wholesale-overwritten on
every run (never merged) — same discipline as `claude-skill`
(`~/.claude/skills/trackfw/SKILL.md`), not the merge-and-preserve discipline of the settings-file
targets. Entry schema mirrors `InjectKiroHooks` (project scope) exactly: top-level
`{"version":"v1","hooks":[...]}`, each entry
`{"name","description","trigger","matcher","action":{"type":"command","command":<path>}}` — but
`command` here is the **absolute** path of `~/.trackfw/scripts/trackfw-credential-guard.sh` (a global
hook can fire from any project's cwd, unlike the project-scope wiring's relative
`scripts/trackfw-credential-guard.sh`), and the two hook names are
`trackfw-credential-guard-global-pre`/`trackfw-credential-guard-global-post` — deliberately distinct
from the project-scope names (`trackfw-credential-guard-pre`/`-post`), since this writes an entirely
different file and nothing in the changelog documents whether Kiro deduplicates same-named hooks
across scopes/files; the future project-scope dedup (Wave 3, ML-3A) matches on the script path, not
the hook name, same as every other tool's dedup.

**Kiro v3 caveat — no runtime version probe, documented instead.** The same changelog page states
global hooks are "Available in V3 (`kiro-cli --v3`)". Re-fetching that page and
`https://kiro.dev/docs/cli/` (2026-08-06) found `--v3` is a **launch-mode flag on the installed
binary**, not a value any `kiro`/`kiro-cli --version`-style command reports — neither page documents
any `--version` flag or output format at all. There is therefore no persistent, installed-version fact
to probe from a separate process: trackfw never invokes Kiro itself, and whether a given Kiro session
honors this file depends on how the user launches their *next* session (`kiro-cli --v3`), not on
anything on disk right now. `kiro-credential-guard` intentionally does **not** attempt a `kiro`/
`kiro-cli` subprocess version probe. It also does **not** put this caveat in the JSON `message` field:
the pinned contract (`TargetResult.Message`/`message` key, see "message only when present, last"
above and `TestUpdateHarnessCmd_JSONKeyOrderMatchesCliParityContract`) reserves `message` for `failed`
targets only — inventing a message on `updated` would violate that contract. The v3 prerequisite is
documented here and in the Go/Node/Python doc comments above
`harnessCredentialGuardTargetKiro`/`credentialGuardTargetKiro`/`_credential_guard_kiro_result`
instead; release notes pointing users at `trackfw update harness` should mention it too.

### Suporte por CLI — visão consolidada, escopo GLOBAL (ML-5A, `ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md`)

Consolida, numa única tabela, o wiring **global** (`trackfw update harness`) já detalhado CLI a CLI
nas seções acima ("Declared harness targets — pinned list", "GitHub Copilot global-scope wiring
(ML-2E)", "Kiro global-scope wiring (ML-2F)") e no gate estrutural dedicado ("Hooks GLOBAIS de
credential-guard ... — paridade estrutural (ROADMAP-2026-08-06, ML-4A)", mais abaixo neste
documento). Nenhum dado novo é introduzido aqui — cada célula reaproveita o que já foi confirmado com
fonte primária nas seções detalhadas por ML. **Não confundir com** a seção homônima "Suporte por CLI
— visão consolidada (ML-5A)" mais acima neste documento — aquela consolida o wiring **por-projeto**
de um roadmap anterior e não relacionado
(`ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`).

| CLI | Arquivo global | Merge ou overwrite total | Path do comando | Pré-requisito de versão |
|---|---|---|---|---|
| Claude Code | `~/.claude/settings.json` | Merge (`PreToolUse`/`PostToolUse[matcher:"Bash"]`, `mergeClaudeHookArray`) — ver "Declared harness targets — pinned list" (ML-2A) | Absoluto, `~/.trackfw/scripts/trackfw-credential-guard.sh` | Nenhum |
| Codex | `~/.codex/hooks.json` | Merge (`PreToolUse`/`PostToolUse[matcher:"Bash"]`) — ver "Declared harness targets — pinned list" (ML-2B) | Absoluto, mesmo script | Nenhum — investigação `codex_hooks` resolvida (hooks habilitados por padrão) |
| Gemini CLI | `~/.gemini/settings.json` | Merge (`BeforeTool`/`AfterTool[matcher:"run_shell_command"]`) — ver "Declared harness targets — pinned list" (ML-2C) | Absoluto, mesmo script | Nenhum |
| Cursor | `~/.cursor/hooks.json` | Merge (`hooks.beforeShellExecution`/`hooks.afterShellExecution`, entradas planas `{"command":...}`, sem `matcher`) — ver "Declared harness targets — pinned list" (ML-2D) | Absoluto, mesmo script | Nenhum |
| GitHub Copilot | `~/.copilot/settings.json` | Merge — inline `hooks.preToolUse`/`hooks.postToolUse[matcher:"bash"]` num arquivo de config geral compartilhado, **não** dedicado (diverge do escopo de projeto, que usa `.github/hooks/*.json` dedicado) — ver "GitHub Copilot global-scope wiring (ML-2E)" | Absoluto, mesmo script | Nenhum |
| Kiro | `~/.kiro/hooks/trackfw-credential-guard.json` | **Overwrite total** — arquivo dedicado, um-arquivo-por-hook em `~/.kiro/hooks/`, nunca merge — ver "Kiro global-scope wiring (ML-2F)" | Absoluto, mesmo script (hook names `-global-pre`/`-global-post`, distintos dos de projeto) | **v3** (`kiro-cli --v3`) — documentado, sem sonda de subprocesso possível (ver caveat acima) |
| Windsurf | — | — | — | **Fora de escopo** — sem hook nativo pré-execução, mesma razão da REQ original (PR #141) |

##### Achados transversais (Waves 1-4 deste roadmap)

1. **Modo sempre `warn` em escopo global, sem config adicional.** ML-1A decidiu não introduzir
   `~/.trackfw/config.yaml` para configurar `credential_guard.mode` no escopo global — complexidade
   não demandada; revisável se houver demanda real por `block` global. O script global reusa o
   conteúdo canônico do script de projeto, só muda o destino de escrita.
2. **Erro de autoria do roadmap no ML-2A (só Go listado), corrigido com follow-up de paridade.** O ML
   original só listava arquivos Go — violação da regra dura de paridade 3 CLIs do `CLAUDE.md`. O
   agente Go sinalizou a violação em vez de expandir escopo por conta própria; corrigido com um
   follow-up dedicado cobrindo Node.js/Python, com `check-update-parity.sh` confirmando os 22 ids
   idênticos nos 3 stacks. Todos os MLs seguintes (2B-2F) já exigiram os 3 stacks desde o início.
3. **Investigação do Codex resolvida: hooks habilitados por padrão, não opt-in.** Re-fetch de
   `developers.openai.com/codex/hooks` (2026-08-06) confirma que `[features] hooks = false`
   (`codex_hooks` como alias depreciado) só serve para DESLIGAR hooks — nunca é necessário como
   opt-in, nem para wiring de projeto nem global.
4. **Formato do Copilot em escopo global diverge do formato de projeto.** Escopo de projeto usa
   `.github/hooks/*.json`, um arquivo dedicado (overwrite total). Escopo global usa
   `~/.copilot/settings.json`, o arquivo de config geral do usuário do Copilot CLI (guarda outras
   chaves, ex. `model`) — logo exige merge preservando as demais chaves de topo, em vez de overwrite.
5. **Kiro sem sonda de versão v3 possível — pré-requisito documentado, não sondado.** `--v3` é uma
   flag de modo de lançamento do binário instalado, não um valor que algum comando `--version`
   reporte; não há fato de versão instalada persistente para sondar de um processo externo (trackfw
   nunca invoca o Kiro diretamente). Decisão: documentar o pré-requisito nos doc comments dos 3 stacks
   e em `docs/cli-parity.md`, sem tentar sondagem de subprocesso e sem usar `TargetResult.Message`
   (reservado a `state: failed`).
6. **Dedup por leitura (ML-3A) funcionando nos 6 CLIs, fail-open confirmado.** Cada um dos 6
   `InjectXHooks`/`injectXHooks`/`inject_x_hooks` de projeto lê (nunca escreve) o arquivo de hooks
   global correspondente antes de adicionar a entrada de credential-guard por-projeto; se a entrada
   global já existe, a entrada por-projeto é pulada (attention-signal/cleanup continuam normais).
   Qualquer falha ao resolver `$HOME`, ler ou parsear o arquivo global é tratada como "não instalado
   globalmente" — fail-open, nunca fail-closed silenciando o credential-guard por-projeto por erro de
   leitura. Coberto por `internal/generators/credential_guard_dedup_test.go` (Go, 9 testes) e
   equivalentes Node/Python.
7. **Gate de paridade estrutural novo (ML-4A) cobrindo os 6 arquivos globais.**
   `scripts/check-harness-hooks-parity.sh` — gate dedicado (não extensão de
   `check-agent-hooks-parity.sh`, entry points/fixtures diferentes) — roda `trackfw update harness
   --targets <6 ids>-credential-guard --install-missing` uma vez por runtime, cada um contra o seu
   próprio `$HOME` de fixture isolado, e compara estruturalmente os 6 arquivos resultantes (com
   normalização textual do path absoluto de fixture antes do `json.loads`). 12/12 `OK` (6 CLIs ×
   go-vs-node/go-vs-py). Prova negativa (P4) registrada em `check-gates-falsify.sh` (Cenário 45,
   corrompe o `matcher` do Kiro global). Ver "Hooks GLOBAIS de credential-guard ... — paridade
   estrutural (ROADMAP-2026-08-06, ML-4A)" mais abaixo para o detalhamento completo.

Each `<tool>-<kind>` target is a **roll-up over every catalog item** for that pair, not one row per
item; per-item granularity already exists via `trackfw agents update` and `trackfw skills update`.
Roll-up precedence: `failed` > `updated` > `skipped`; all-not-installed → `missing`.

`path` is rendered **tilde-abbreviated** (`~/.claude/agents`), never as an absolute path. Absolute
paths make the JSON machine-dependent and break byte-comparison across runtimes.

This list was pinned after the first implementation round produced three different answers — Go
declared 3 targets, Node.js and Python 19 — because the contract specified states, flags and key
order but left the target set to interpretation. Leaving a set unpinned is the same failure mode as
leaving a string unpinned.

**Parity auditing note:** compare these documents across runtimes with key order **preserved**
(`object_pairs_hook=OrderedDict` and `dumps` without `sort_keys`). Normalizing key order hides
declaration-order drift — that is exactly how the `gates` check divergence survived Wave 2 of the
barrier roadmap and had to be fixed later in ML-2E.

## `install` sobre artefato gerenciado desatualizado — skip, não erro fatal

Escopo desta seção: o preflight de `mutationInstall` no `IntegrationManager` dos três runtimes.
Afeta todo caller de `install` — `trackfw init --ai-tools`, `trackfw agents install`,
`trackfw skills install` e `trackfw update --install-missing`.

**Esta seção não altera o escopo de instalação.** As decisões D1 e D4 de
`ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md` permanecem em vigor:
`trackfw init --ai-tools`, sem TTY e sem `--scope`, instala em escopo **global**. O contrato
`trackfw update` vs `trackfw update harness` acima é escopado à **família `update`** e não impõe
fronteira projeto/global aos demais comandos.

### Problema

Um artefato `outdated` **e** `owned` (declarado no manifest com o mesmo claim, bytes correspondentes
a um template trackfw anterior) fazia o preflight de `install` retornar erro. Como `mutate` é um lote
atômico com rollback, o erro **aborta a operação inteira**: um harness global desatualizado impedia
`trackfw init --ai-tools gemini` de fazer o scaffold de um projeto novo, com

```
artifact "/home/<user>/.gemini/agents/trackfw-architect.md" is outdated; use update
```

### Contrato

| Estado do artefato | `owned` | `install` sem `--force` |
|---|---|---|
| `current` | qualquer | grava/no-op — inalterado |
| `outdated` | **sim** | **skip**: bytes preservados, lote continua, exit **0** |
| `outdated` | não | adoção — inalterado (`install` grava e assume o claim) |
| `modified` | qualquer | **erro** — inalterado, exige `--force` |

1. `outdated` + `owned` + sem `--force` → o artefato é **pulado**. Seus bytes são preservados, os
   demais itens do lote são aplicados e o exit code é **0**.
2. **`modified` continua sendo erro.** Bytes modificados são do usuário e nunca podem ser pulados
   silenciosamente. Não "simetrizar" os dois casos.
3. `install` não é caminho de upgrade — `update` é. Pular um artefato `owned`+`outdated` não perde
   informação alguma: seus bytes são um template trackfw anterior, não conteúdo do usuário.

### Superfície do sinal de skip

O observador opcional de skip é a **única** superfície sancionada para o sinal. Nenhum runtime deve
propagá-lo por outro caminho — em particular, o `mutate` do Node.js já retorna `this.inspect(plans)`,
e esse retorno **não** deve ser usado para comunicar skips, sob pena de divergência com Go e Python.

| Runtime | Assinatura | Quando ausente |
|---|---|---|
| Go | campo `Manager.OnSkip func(destination, reason string)` | nil → no-op |
| Node.js | `new IntegrationManager(dirs, { onSkip })` | `undefined` → no-op |
| Python | `IntegrationManager(root, on_skip=None)` | `None` → no-op |

O observador é chamado **uma vez por artefato pulado**, na fase de preflight, na ordem de
`resolved` — nunca duas vezes para o mesmo destino.

#### Valor de cada parâmetro — pinado

A primeira rodada de implementação pinou os **nomes** dos parâmetros e deixou os **valores** à
interpretação. Os três runtimes produziram três respostas para `reason`: a linha de aviso completa
(Go), a etiqueta `'outdated+owned'` (Node.js) e a etiqueta `"outdated"` (Python). Nome de parâmetro
não é contrato; valor é.

- **`destination`**: o caminho de exibição **tilde-abreviado** — exatamente a mesma string que
  aparece dentro de `reason`. Nunca o caminho absoluto.
- **`reason`**: a linha de aviso **completa e pronta para impressão**, sem `\n` terminal. Não é
  etiqueta, código nem categoria.

**Os callers NÃO devem compor, abreviar nem derivar o comando de remediação.** Um caller recebe
`reason` e escreve em stderr *verbatim*, sem acrescentar nada. Esta frase existe porque a primeira
rodada produziu **dois sites de composição dentro do mesmo runtime** (`init.js` e `integrations.js`
no Node.js), que podem divergir entre si sem que nenhum teste de paridade entre runtimes perceba.

#### Origem do comando de remediação — pinada

A remediação é derivada de **`plan.claim.scope`, por artefato**, dentro do manager.

Proibido derivá-la de: inferência sobre o caminho renderizado (`tilde.startsWith('~/')`) ou closure
sobre o escopo de nível de comando. Ambas acertam apenas enquanto o lote é de escopo uniforme — são
corretas por acidente, não por construção. Um lote de escopo misto produziria a remediação errada
para parte dos artefatos.

A abreviação tilde vive **no manager**. Não existe helper compartilhado utilizável em todos os
runtimes: o `tildeify` existe apenas em `npm/src/lib/update-engine.js`; o `update.go` usa constantes
hardcoded (`const displayPath = "~/.claude/..."`), não um helper; e em Python o `_tildeify` de
`commands/update_harness.py` é inalcançável de `integrations/manager.py` por import circular
(`integrations` → `commands` → `integrations`). Quando o helper não for importável sem ciclo,
**inline a lógica com a salvaguarda de `Clean`/barra dupla** — reimplementar sem ela reintroduz o bug
de `$HOME` com barra dupla corrigido no ML-6H.

#### `update --install-missing` não requer observador — intencional

Nenhum caller da família `update` precisa ligar o observador, e isso **não é omissão**. Verificado
em todos os call sites: `install` só é invocado para targets `not-installed` —
`internal/generators/update.go:502` e `:720` (ambos sob `case integrations.StateNotInstalled`),
`pypi/trackfw/commands/update_harness.py:222` (itera apenas `not_installed`), e
`npm/src/integrations/index.js:107` (único caller de `install` no Node, já recebe `onSkip` via
`execute`). Um artefato `not-installed` não pode ser `outdated` + `owned`, logo o branch de skip é
**inalcançável** por esses caminhos. Não "corrigir" ligando observadores ali.

### Aviso ao usuário — string pinada

Emitido em **stderr**, uma linha por artefato pulado, com o caminho **tilde-abreviado** (mesma regra
já pinada para `path` na seção de `update`; caminho absoluto torna a saída dependente da máquina e
quebra comparação byte-a-byte entre runtimes).

Escopo global:
```
warning: skipping outdated artifact ~/.gemini/agents/trackfw-architect.md; run 'trackfw update harness' to refresh it
```

Escopo de projeto:
```
warning: skipping outdated artifact .claude/agents/trackfw-architect.md; run 'trackfw update' to refresh it
```

O comando de remediação **varia por escopo do claim** — `update harness` para global, `update` para
projeto — porque indicar o comando errado manda o usuário a uma operação que não toca o artefato
citado. Em escopo de projeto o caminho é relativo à raiz do projeto, sem `./`.

Exit code é **0**. As linhas de sucesso por ferramenta (`✓ <tool> agents and skills`) continuam
sendo impressas: são por ferramenta, não por artefato, e a ferramenta foi de fato processada. O aviso
em stderr é a única indicação de skip.

### Implementação canônica

O runtime **Go** é a referência: o manager compõe a linha completa, derivando a remediação de
`item.plan.Claim.Scope` por artefato, e os callers apenas imprimem `reason`. Node.js e Python
convergem para essa forma. Go não deve ser alterado para se alinhar aos outros dois.

### Nota de teste

`npm/tests/agents-skills.test.js` continha `assert.throws(() => manager.install([plan]),
/outdated.*update/i)` — asserção que codificava o contrato antigo e é **invertida** por esta seção.
Go e Python não tinham cobertura equivalente; ambos passam a tê-la. Auditoria de paridade compara as
strings de aviso **byte-a-byte** entre os três runtimes.

## Regra `branch_has_wip_roadmap` — comportamento unificado nos 3 runtimes

A regra verifica que toda branch `feat/`, `fix/` ou `refactor/` possui um roadmap cujo nome
contém o slug da branch. Desde REQ-2026-07-26-robustez-dos-gates-de-governanca-e-paridade, a regra
procura o slug em **`wip/` e `done/`**, não apenas em `wip/`.

| Cenário | Comportamento esperado (Go / Node.js / Python) |
|---|---|
| Roadmap em `wip/` com slug da branch | Sem violação — comportamento original preservado |
| Roadmap em `done/` com slug da branch | Sem violação — permite encerrar o roadmap na própria branch (Definition of Done) |
| Nenhum roadmap em `wip/` nem em `done/` | Violação com mensagem "no roadmap is in wip/ nor done/" + orientação de remediação |
| Roadmap em `done/` com slug **diferente** da branch | Violação com mensagem "no matching roadmap in wip/ nor done/" — casamento de slug é obrigatório |

O casamento é feito por `normalizeBranchSlug(filename).contains(branchSlug)` (substring, não
igualdade), pois nomes de roadmap carregam prefixos de data (`ROADMAP-2026-07-27-<slug>.md`).

A resolução de diretórios (`wip/`, `done/`) é centralizada em `resolveStateDirs` (Go),
`resolveStateDirs` (Node.js) e `_resolve_state_dirs` (Python) — as variantes por agente
(`by_agent`) são suportadas via os mesmos wrappers `resolveWIPDirs`/`resolveDoneDirs`.

O ID da regra (`branch_has_wip_roadmap`) e o mecanismo de severidade configurável (`rules:`) são
preservados — a aceitação de `done/` não altera a config key nem o comportamento de `off`/`warning`.

`trackfw branch new` (ver "`trackfw branch new`" acima) aplica exatamente esta mesma regra **antes**
da branch existir, chamando a mesma função de matching (`BranchSlugMatchesRoadmap` /
`branchSlugMatchesRoadmap` / `branch_slug_matches_roadmap`) que `validateBranchHasWIPRoadmap`
chama aqui — não uma segunda implementação.

## Contrato de artefatos gerados (req, adr, roadmap, note)

Os quatro comandos de geração de artefatos produzem arquivos **byte-a-byte idênticos**
nos três runtimes para a mesma entrada. Isso inclui conteúdo e nome de arquivo.

### Frontmatter e formato — contrato explícito

#### `req new <title>`

Arquivo: `docs/req/REQ-YYYY-MM-DD-<slug>.md`

```
---
status: Open
date: YYYY-MM-DD
author: ""
adr: ""
roadmap: ""
---

# REQ: <title>

> Date: YYYY-MM-DD | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 
```

**Escopo local/global dos ADR drafts gerados por probe (Go+Node.js apenas):** no fluxo
interativo (TTY) que detecta domínios e gera ADR drafts a partir de probes, um único prompt
("Escopo dos ADRs desta REQ": Local, padrão, ou Global) é exibido antes do loop de probes —
a escolha vale para todos os ADR drafts gerados naquela sessão de `req new`, não é perguntada
por probe. `global` escreve os drafts em `~/.trackfw/adr/`; `local` (default) preserva o
comportamento anterior a esta feature. Sem TTY, nenhum prompt é exibido e o comportamento é
idêntico ao anterior (sempre `local`).

**Exceção Python-only, pré-existente e sem relação com o prompt acima:** `pypi` não implementa
o fluxo de detecção de domínios/probes/ADR-draft de `req new` — `req new` em Python só pede o
título (prompt simples se omitido) e nunca gera ADR drafts. Gap de paridade documentado, não
introduzido por esta feature; corrigi-lo exigiria portar o sistema de probes inteiro para
Python, fora do escopo desta REQ.

#### `adr new <title>`

Arquivo: `docs/adr/ADR-YYYY-MM-DD-<slug>.md`

**`--scope project|global`** (default `project`, os 3 CLIs): `project` preserva o
comportamento acima, byte a byte. `global` escreve em
`~/.trackfw/adr/ADR-YYYY-MM-DD-<slug>.md` — mesmo diretório-base de
`~/.trackfw/scripts/` (credential-guard) e `~/.trackfw/identity.json` — sem exigir
`trackfw.yaml`/raiz de projeto no cwd (mesmo padrão de `trackfw update harness`).
Conteúdo idêntico entre os dois escopos; só o diretório de destino muda.
`trackfw adr list` aceita o mesmo flag (`project` lista `adr_dirs[0]`, `global` lista
`~/.trackfw/adr/*.md`).

**Escopo desta feature, deliberadamente limitado:** `trackfw validate`/`status`/
`context` NÃO passam a varrer `~/.trackfw/adr` implicitamente — cada projeto
continua enxergando só os `adr_dirs` do seu próprio `trackfw.yaml`. Para um projeto
específico ver os ADRs globais em `validate`/`status`, adicione `~/.trackfw/adr` ao
`adr_dirs` desse projeto (expansão de `~` já suportada). O fluxo `req`→ADR draft
vinculado (`NewADRDraft`/`newADRDraft`/`new_adr_draft`, usado por `--from-req`)
também não ganha escopo global — um ADR nascido de uma REQ é inerentemente do
projeto onde a REQ vive.

**Exceção Python-only pré-existente, sem relação com `--scope`:** `pypi` já tinha
`--status <status>` e `--dir <path>` em `adr new` antes desta feature — Go/Node.js
não têm equivalente. `--dir` e `--scope global` são mutuamente exclusivos em Python
(erro claro se ambos forem passados); nos demais casos os dois flags continuam
funcionando como antes. `adr list` não existia em Python antes desta feature — foi
criado do zero, espelhando a saída de Go/Node.js (`nome-do-arquivo` alinhado a 60
colunas + status, ordem alfabética, `No ADRs found in <dir>` quando vazio).

```
---
status: Proposed
date: YYYY-MM-DD
author: ""
---

# ADR: <title>

> Date: YYYY-MM-DD | Status: Proposed

## Context
<!-- What is the situation that motivates this decision? -->

## Decision
<!-- What was decided? -->

## Consequences
<!-- What are the positive and negative consequences of this decision? -->

## Alternatives Considered
<!-- What other options were evaluated and why were they rejected? -->
```

#### `roadmap new <title>`

Arquivo: `docs/roadmaps/backlog/ROADMAP-YYYY-MM-DD-<slug>.md`

```
---
status: backlog
date: YYYY-MM-DD
req: ""
squad: ""
---

# Roadmap: <title>

> Created: YYYY-MM-DD | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: 

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — <title>
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes
```

O mesmo frontmatter é obrigatório para roadmaps criados por interfaces de agente,
incluindo o slash-command `/trackfw:roadmap`: `status`, `date`, `req` e `squad`.
O campo `req:` deve receber o caminho relativo completo da REQ selecionada, com
prefixo `docs/req/` e sufixo `.md`; basename solto e link Markdown não são
formato canônico.

Estados válidos para `roadmap move`, `roadmap list`, `roadmap show`, validação e
resolução de paths nos três runtimes:

```
backlog → analyzing → wip → blocked → done → abandoned
```

Ao mover um roadmap para `analyzing`, os três CLIs devem manter pasta,
frontmatter (`status: analyzing`), header (`| Status: analyzing`) e
`docs/roadmaps/.trackfw-log` sincronizados. Em `roadmap_namespacing: by_agent`,
o log preserva o prefixo do agente, por exemplo
`zeus/ROADMAP-YYYY-MM-DD-<slug>.md`.

#### `note new <title>`

Arquivo: `vault/notes/<slug>-YYYY-MM-DD.md` (slug antes da data, inverso do req/adr/roadmap).
Cria ou atualiza `vault/notes/index.md` com uma linha de link no formato `- [<slug>-YYYY-MM-DD](<slug>-YYYY-MM-DD.md)`.

```
---
title: "<title>"
tags: []
date: YYYY-MM-DD
related: []
---

# <title>

## Problem

<!-- Descreva o problema ou situação que motivou esta nota. -->

## Root cause

<!-- Qual foi a causa raiz identificada? -->

## Solution

<!-- Como foi resolvido ou mitigado? O que deve ser feito? -->
```

### Slug — normalização NFKD portável nos três runtimes

Os três runtimes usam a mesma semântica: NFKD decomposition → remoção de
combining marks (diacríticos) → lowercase → substituição de sequências
`[^a-z0-9]+` por hífen → colapso de hífens múltiplos → trim.

| Exemplo de título | Slug gerado (todos os runtimes) |
|---|---|
| `"Autenticação e Sessão"` | `autenticacao-e-sessao` |
| `"ADR Config (v2)"` | `adr-config-v2` |
| `"Minha Requisição #1"` | `minha-requisicao-1` |

Títulos com qualquer combinação de acentos (á é í ó ú), cedilha (ç), til (ã õ),
crase (à) e caracteres não-alfanuméricos produzem slugs idênticos nos três
runtimes. O gate `check-artifact-parity.sh` usa título acentuado
(`"Autenticação e Sessão"`) para validar esse comportamento.

### Data — hora local nos três runtimes

Todos os CLIs usam a data local (`date +%F` / `time.Now().Format("2006-01-02")` /
`datetime.date.today().isoformat()`) — nunca UTC. Geração cruzando meia-noite num
fuso horário avançado pode produzir datas distintas entre runtimes; o gate detecta
essa condição e falha explicitamente.

### Parity gate

`scripts/check-artifact-parity.sh` é o gate transversal que verifica esse contrato.
Para cada tipo de artefato (req, adr, roadmap, slash-command roadmap, note,
vault/notes/index.md), ele invoca os três runtimes com título posicional ASCII,
confirma que exatamente um arquivo foi gerado por runtime (vacuity guard), e faz
diff byte-a-byte acumulando todos os erros antes de sair — o diagnóstico nomeia
o tipo e os runtimes divergentes. O mesmo gate executa um ciclo E2E
`backlog → analyzing` em cada runtime nos layouts `flat` e `by_agent`,
verificando pasta, frontmatter, header, `.trackfw-log` e ausência de
`folder_status` no `validate --json`. Roda como parte de `make quality` (alvo
`parity`), antes de `check-gates-falsify.sh`.

Dois cenários negativos (P4) estão em `scripts/check-gates-falsify.sh`:

- **Cenário 7** — drift de **conteúdo**: corrompe o gerador de req do Node.js para
  emitir `status: OPEN`; asserta exit != 0 com `artifact parity drift: req (go vs node)`.
- **Cenário 8** — drift de **nome de arquivo**: compila binário Go corrompido que usa
  prefixo `RREQ-` em vez de `REQ-`; asserta exit != 0 com `arquivo ausente`.
  Os dois caminhos de comparação (conteúdo e nome) têm provas independentes.
- **Cenário 9** — drift de **slash-command roadmap**: corrompe o gerador de init
  do Node.js para emitir `status: backlogged` no `/trackfw:roadmap`; asserta
  exit != 0 com `artifact parity drift: slash_roadmap (go vs node)`.

## Scripts de attention hooks (`trackfw-attention-signal.sh` / `trackfw-attention-cleanup.sh`) — byte-idênticos

`trackfw discover --init` grava `scripts/trackfw-attention-signal.sh` e
`scripts/trackfw-attention-cleanup.sh` a partir de um literal-fonte embutido em
cada runtime (`internal/generators/scaffold.go`,
`npm/src/generators/hooks.js`, `pypi/trackfw/generators/init_gen.py`) — não são
arquivos estáticos compartilhados, cada runtime carrega sua própria cópia do
texto. Isso já divergiu silenciosamente uma vez (comentário "no-op fora da
raiz" em PT/EN/PT-diferente, presença/ausência de uma linha em branco após
`ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}`, e dois estilos equivalentes de
`sed` no cálculo de `TOOL_ESC`/`MSG_ESC`) sem nenhum gate detectar — ver
`docs/req/REQ-2026-08-04-scripts-de-attention-hooks-divergem-em-conteudo-entre-go-node-e-python-sem-gate-de-paridade.md`.
O texto canônico atual: comentário em inglês ("Script is intentionally a
no-op when executed outside the project root"), linha em branco presente após
o default de `ROADMAP_DIR` (e entre `TIMESTAMP=...` e `TOOL_ESC=...` no script
de signal), e `sed` de expressão única (`sed 'expr1; expr2'`, não
`sed -e expr1 -e expr2`).

### Parity gate

`scripts/check-attention-scripts-parity.sh` roda `discover --init` com os três
binários reais (Go compilado, Node.js, Python) num fixture vazio por runtime, e
faz `diff -u` byte-a-byte dos dois scripts gerados entre Go×Node e Go×Python —
falha com o diff explícito no diagnóstico se divergirem (P2, sem degradação
silenciosa) e tem um guard de vacuidade (P2) que reprova se algum runtime não
gerar os arquivos. Roda como parte de `make quality` (alvo `parity`), antes de
`check-gates-falsify.sh`.

A prova negativa (P4) está em `scripts/check-gates-falsify.sh` — corrompe o
comentário "no-op" do literal Python (`pypi/trackfw/generators/init_gen.py`)
numa cópia isolada do repositório e asserta que o gate reprova com o diff
explícito no diagnóstico.

## Agent hooks por CLI (`.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json`, `.github/hooks/trackfw-attention.json`, `.cursor/hooks.json`, `.kiro/hooks/trackfw-attention.json`) — paridade estrutural (ML-3A)

Cada `InjectXHooks`/`injectXHooks`/`inject_x_hooks` (`internal/generators/agentfiles.go`,
`npm/src/generators/hooks.js`, `pypi/trackfw/generators/hooks.py`), para os 6
CLIs da wave nativa cobertos pela
`docs/req/REQ-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`
(Claude Code, Codex, Gemini CLI, GitHub Copilot, Cursor, Kiro), é uma
implementação independente por stack — não um arquivo estático compartilhado.
Ao contrário dos dois scripts shell de attention hooks (seção acima), cada CLI
tem o seu **próprio schema JSON por design** (documentado CLI a CLI nas seções
"wiring (ML-2x)" acima) — então este gate não compara byte-a-byte, compara
**estruturalmente**: mesmas chaves presentes em cada nível, mesmos valores nas
chaves relevantes (comando/script referenciado, matcher, evento/trigger),
ordem de array significativa (pelo menos um CLI documenta execução em ordem —
ver "GitHub Copilot wiring (ML-2D)"), indentação/ordem de inserção de chaves
do serializador nunca é reportada como drift.

### Divergência pré-existente encontrada e corrigida por este ML

A primeira execução do gate reprovou de verdade contra o estado pós-Wave 2:
`pypi/trackfw/generators/hooks.py:_merge_codex_hook_entry` aceitava
`**extra_fields` e sempre escrevia `timeout=10` (+ `statusMessage` por hook) ao
criar uma entrada nova em `.codex/hooks.json` — campos que
`InjectCodexHooks` (Go) e `injectCodexHooks` (Node) nunca escreveram, que
`https://developers.openai.com/codex/hooks` não documenta como funcionais, e
dos quais nenhum teste (`pypi/tests/test_generators_init.py`,
`pypi/tests/test_codex.py`) dependia. Essa divergência é anterior a este ML
(introduzida no ML-2B, nunca detectada por falta de gate) — corrigida aqui
removendo a decoração `timeout`/`statusMessage` de Python, alinhando-o a
Go/Node, o mesmo movimento que o ML-2C já tinha feito para os campos
`name`/`timeout: 10000` que a versão anterior do Python escrevia nas entradas
do Gemini.

### Parity gate

`scripts/check-agent-hooks-parity.sh` roda `discover --init` uma vez por
runtime (Go compilado, Node.js, Python) — não uma vez por CLI — num fixture
que carrega, de uma vez, o marcador de detecção dos 6 CLIs
(`CLAUDE.md`/`AGENTS.md`/`GEMINI.md`/`.kiro/`/`.github/copilot-instructions.md`/
`.cursor/`, os mesmos marcadores lidos por
`InjectHooksDetected`/`injectHooksDetected`/`inject_hooks_detected`), o que
mantém o gate em ~3 execuções reais de CLI (isolamento por CLI mediria ~15s a
mais em `make quality` sem ganho de detecção: os guards de vacuidade abaixo já
cobrem o caso de um detector que passa a pular um CLI silenciosamente).

**`HOME` isolado por runtime (ROADMAP-2026-08-08 ML-4A, P3).** `run_discover_init` cria um
diretório vazio dedicado sob `$WORK` (`<fixture-dir>.home`) e passa `HOME="$home_dir"` para as 3
invocações — mesmo padrão de `check-update-parity.sh` (`run_update`/`run_init`/
`install_agent_*`). Antes desta correção o gate lia o `$HOME` **real** de quem o executava: numa
máquina onde o credential-guard já foi instalado globalmente (via `trackfw update harness`), o
dedup de projeto (ver "Agent hooks por CLI" e achado #6 acima) pulava silenciosamente a entrada de
credential-guard de projeto, e o guard de vacuidade `credential-guard-present` abaixo reprovava
os 6 CLIs × 3 runtimes de forma idêntica — um falso negativo ambiental, não uma regressão de
código. Ver `vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md`.
Efeito colateral aceito: este gate agora só exercita o caminho "sem guard global instalado"; o
caminho de dedup (entrada de projeto pulada) é coberto separadamente por
`internal/generators/credential_guard_dedup_test.go` (Go, 9 testes) e equivalentes Node/Python
(achado #6 acima), não por um gate shell.

Para cada um dos 6 arquivos de hook gerados, dois guards de vacuidade (P2) rodam
antes de qualquer diff: (1) o arquivo existe e não está vazio nos 3 runtimes;
(2) o arquivo referencia `scripts/trackfw-credential-guard.sh` pelo menos uma
vez em cada runtime — sem isso, uma regressão que removesse a entrada de
credential-guard identicamente nos 3 stacks ainda "passaria" numa comparação
cruzada pura, o oposto do que este ML existe para prevenir. Só então roda a
comparação estrutural (Go×Node e Go×Python, por CLI) via um comparador
`python3` inline (JSON parseado, diff recursivo por chave/índice de array,
sem `jq` — nenhum `scripts/check-*.sh` do projeto depende de `jq` nem nenhum
workflow o instala; `python3` já é uma dependência obrigatória do gate por
rodar o CLI Python). Falha nomeando o CLI, o par de stacks e o path JSON
divergente (ex.: `$.hooks.PreToolUse[0].matcher`). Roda como parte de
`make quality` (alvo `parity`), logo após
`check-attention-scripts-parity.sh` e antes de `check-gates-falsify.sh`.

A prova negativa (P4) está em `scripts/check-gates-falsify.sh` (Cenário 44) —
corrompe o `matcher` da entrada `trackfw-credential-guard-post` do wiring do
Kiro no literal Node.js (`npm/src/generators/hooks.js`, de `'shell'` para
`'execute_bash'`) numa cópia isolada do repositório e asserta que o gate
reprova apontando `$.hooks[3].matcher` no diagnóstico. **Gap conhecido, não
fechado por este ML:** o Cenário 44 falsifica apenas o comparador estrutural
(`compare_json`); o segundo guard de vacuidade (P2) —
`credential-guard-present`, o mesmo que capturou o falso negativo ambiental
corrigido acima — não tem prova negativa própria em `check-gates-falsify.sh`.
Reportado a `trackfw_architect`/Zeus como item em aberto; não corrigido aqui
por estar fora do escopo declarado deste ML (gate final + docs, não
hardening de P4).

## Hooks GLOBAIS de credential-guard (`~/.claude/settings.json`, `~/.codex/hooks.json`, `~/.gemini/settings.json`, `~/.cursor/hooks.json`, `~/.copilot/settings.json`, `~/.kiro/hooks/trackfw-credential-guard.json`) — paridade estrutural (ROADMAP-2026-08-06, ML-4A)

Sibling do gate de hooks por-projeto (seção anterior), para o escopo GLOBAL
introduzido por
`docs/adr/ADR-2026-08-06-hooks-de-credential-guard-em-escopo-global-via-trackfw-update-harness.md`:
`harnessCredentialGuardTarget<Tool>` (`internal/generators/update.go`),
`credentialGuardTarget<Tool>` (`npm/src/commands/update-harness.js`) e
`_credential_guard_<tool>_result` (`pypi/trackfw/commands/update_harness.py`)
são implementações independentes por stack para os mesmos 6 CLIs da wave
nativa, escritas via `trackfw update harness --targets <tool>-credential-guard
--install-missing` em `$HOME` em vez de num projeto. Nenhum dos dois gates
subsome o outro: o dedup do ML-3A (seção "Agent hooks por CLI" acima) LÊ o
arquivo global que este gate exercita, mas nunca o escreve.

### Parity gate

`scripts/check-harness-hooks-parity.sh` roda `update harness --targets
<todos os 6 ids>-credential-guard --install-missing` uma vez por runtime (Go
compilado, Node.js, Python), cada runtime contra o seu PRÓPRIO fixture de
`$HOME` isolado (nunca o `$HOME` real de quem roda o gate) — um `$HOME`
compartilhado entre os 3 runtimes foi descartado porque `--install-missing` é
merge idempotente: o segundo e o terceiro runtime a escrever no mesmo `$HOME`
reportariam `state: skipped` em vez de `state: updated`, enfraquecendo
silenciosamente a garantia central do gate (que cada stack, escrevendo do
zero, produz a mesma estrutura). Os mesmos dois guards de vacuidade (P2) do
gate por-projeto rodam antes de qualquer diff (arquivo existe e não está
vazio nos 3 runtimes; arquivo referencia `trackfw-credential-guard.sh` pelo
menos uma vez). A comparação estrutural reusa o mesmo comparador `python3`
inline do gate por-projeto (mesmo motivo: nenhum `jq`) — com uma etapa extra
de normalização textual ANTES do `json.loads`: cada um dos 6 arquivos embute
o path ABSOLUTO de `~/.trackfw/scripts/trackfw-credential-guard.sh` (um hook
global precisa resolver a partir do cwd de qualquer projeto, então um path
relativo não é opção), e como cada runtime roda contra o seu próprio `$HOME`
de fixture, esse absoluto diverge textualmente entre os 3 mesmo quando todos
resolvem corretamente — o gate substitui o path do `$HOME` de fixture de cada
runtime por um placeholder comum (`<HOME>`) no conteúdo bruto do arquivo
antes de parsear como JSON, então esse campo nunca é reportado como drift
falso. Falha nomeando o CLI, o par de stacks e o path JSON divergente (ex.:
`$.hooks[1].matcher`). Roda como parte de `make quality` (alvo `parity`),
logo após `check-agent-hooks-parity.sh` e antes de `check-gates-falsify.sh`.

A prova negativa (P4) está em `scripts/check-gates-falsify.sh` (Cenário 45) —
corrompe o `matcher` da entrada `trackfw-credential-guard-global-post` do
wiring GLOBAL do Kiro no literal Python
(`pypi/trackfw/commands/update_harness.py`, de `"shell"` para
`"execute_bash"`) numa cópia isolada do repositório e asserta que o gate
reprova apontando `$.hooks[1].matcher` no diagnóstico.

## Modo default do credential-guard GLOBAL — `warn` → `block` (ADR-2026-08-06 emenda 6, ROADMAP-2026-08-08 Wave 1)

**Supersede o achado transversal #1 da seção "Suporte por CLI — visão consolidada, escopo GLOBAL"
acima** ("Modo sempre `warn` em escopo global, sem config adicional"), que descrevia a decisão
original da ADR-2026-08-06. `ADR-2026-08-06` emenda 6 (2026-08-08) reverteu essa decisão: um guard
opt-in que nunca bloqueia por padrão é uma falsa sensação de proteção — o usuário que já rodou
`trackfw update harness --targets <tool>-credential-guard` demonstrou intenção explícita de ter o
mecanismo ativo.

`scripts/trackfw-credential-guard.sh` tem duas variantes de texto — **escopo de projeto** (gerado
por `trackfw init`/`discover`/`update` dentro do repositório) e **escopo global**
(`~/.trackfw/scripts/trackfw-credential-guard.sh`, gerado por `trackfw update harness`) — que
compartilham a mesma leitura de `credential_guard.mode` (`credentialGuardModeResolution` em Go,
equivalentes em Node/Python) mas divergem no **fallback** quando essa chave não está presente:

| Escopo | Constante (Go) | Fallback (`DEFAULT_MODE`) sem `trackfw.yaml`/sem a chave | `trackfw.yaml` com `credential_guard.mode` explícito no cwd |
|---|---|---|---|
| Projeto | `credentialGuardProjectTail` | `warn` — **inalterado** | Respeitado (`warn` ou `block`) — inalterado |
| Global | `credentialGuardGlobalTail` | `block` — **mudança de comportamento** (era `warn`) | Respeitado (`warn` ou `block`) — mesma leitura da variante de projeto |

Pontos pinados:

- **A leitura de `credential_guard.mode` é a mesma nos dois escopos** — o script global lê
  `trackfw.yaml` **do cwd de onde o hook disparou** (o projeto atual), não de um arquivo de config
  global (`~/.trackfw/config.yaml` continua não existindo — decisão mantida do ML-1A original: não
  vale a complexidade de uma segunda fonte de configuração só para isto). Um usuário que já
  configurou `credential_guard.mode: warn` explicitamente em `trackfw.yaml` **não vê nenhuma
  mudança de comportamento** com esta REQ.
- **Só o fallback (ausência de `trackfw.yaml`, ou `trackfw.yaml` sem a chave) muda**, e só no
  escopo global: passa de `warn` para `block` — abortar a tool call (exit 2) em vez de apenas
  logar um attention signal.
- **`ROADMAP_DIR` em escopo global permanece o caminho fixo `docs/roadmaps`** (sem ler
  `roadmap_dir:` de `trackfw.yaml`, já que não há garantia de o arquivo existir) e o attention
  signal só é gravado se esse diretório já existir — inalterado por esta REQ, documentado aqui só
  para não confundir com a resolução de `MODE`, que é independente.
- Implementado byte-idêntico nos 3 stacks: `credentialGuardProjectTail`/`credentialGuardGlobalTail`
  (Go, `internal/generators/scaffold.go`), as constantes homônimas em
  `npm/src/generators/hooks.js`, e `_CG_PROJECT_TAIL`/`_CG_GLOBAL_TAIL` (Python,
  `pypi/trackfw/generators/init_gen.py`) — cada bloco de resolução de `MODE` replicado como texto
  literal idêntico em vez de extraído para uma constante compartilhada concatenada, por causa da
  restrição do gate de paridade documentada em
  `vault/notes/credential-guard-parity-test-extractor-rejects-string-concatenation-2026-08-08.md`.

## Cobertura de matchers Read/Write/Edit do credential-guard por CLI (ADR-2026-08-06 emenda 7, ROADMAP-2026-08-08 Wave 2)

Antes desta REQ, o wiring por-projeto e global do credential-guard só interceptava o **shell tool**
de cada CLI (`Bash`/`apply_patch`/`run_shell_command`/etc. — ver seções "Codex wiring (ML-2B)",
"Gemini CLI wiring (ML-2C)" etc. acima). Um agente podia contornar o guard sem passar por um shell:
lendo um segredo com a ferramenta de leitura nativa (`Read`, `read_file`, `view`, ...) ou
materializando-o com a ferramenta de escrita/edição nativa (`Write`/`Edit`, `write_file`,
`create`/`edit`, `apply_patch`, ...). A ADR-2026-08-06 emenda 7 fecha essa lacuna: cada
`InjectXHooks`/`injectXHooks`/`inject_x_hooks` agora também registra o matcher nativo de
leitura e de escrita/edição de cada CLI apontando para o mesmo
`scripts/trackfw-credential-guard.sh`, sujeito ao mesmo dedup contra o wiring global
(`globalCredentialGuardInstalled<CLI>()`, seção "Agent hooks por CLI" acima) que já existia para o
shell tool.

| CLI | Matcher de leitura | Matcher de escrita/edição | Observação |
|---|---|---|---|
| Claude Code | `Read` | `Write\|Edit` | `PreToolUse`/`PostToolUse`, mesmo `mergeClaudeHookArray` já usado para `Bash` — `internal/generators/agentfiles.go:InjectClaudeHooks` |
| Codex | — (sem matcher de leitura) | `apply_patch` | **Limitação documentada, não workaround**: Codex não expõe um matcher de ferramenta de leitura interceptável — confirmado contra `https://learn.chatgpt.com/docs/hooks` (2026-08-08). Escrita/edição é coberta via `apply_patch` (aliases documentados Edit/Write) — `InjectCodexHooks` |
| Gemini CLI | `read_file\|read_many_files` | `write_file\|replace` | `BeforeTool`/`AfterTool`, mesmo padrão de matcher `\|` já usado para `run_shell_command` — `InjectGeminiHooks` |
| Kiro | `read` | `write` | Aliases de categoria de ferramenta documentados pelo Kiro; hooks dedicados `trackfw-credential-guard-read-pre`/`-read-post`/`-write-pre`/`-write-post`, mesmo `matcher: "shell"` da entrada de Bash generalizado para `"read"`/`"write"` — `InjectKiroHooks` |
| GitHub Copilot | `view` | `create\|edit` | Mapeamento `toolName` confirmado contra `https://docs.github.com/en/copilot/reference/hooks-reference`: `view -> Read`, `create -> Write`, `edit -> Edit` — mesma convenção de nome de ferramenta em minúsculo já usada para `bash` — `InjectCopilotHooks` |
| Cursor | `Read` (evento `preToolUse`/`postToolUse`) | `Write` (evento `preToolUse`/`postToolUse`) | Cursor não tem um evento shell-específico para leitura/escrita — usa os eventos genéricos `preToolUse`/`postToolUse` (distintos de `beforeShellExecution`/`afterShellExecution`, que só disparam para o shell tool), com `mergeCursorGuardMatcherEntry` filtrando por `toolName` — `InjectCursorHooks` |

Implementado identicamente nos 3 stacks (`internal/generators/agentfiles.go`,
`npm/src/generators/hooks.js`, `pypi/trackfw/generators/hooks.py`), verificado por
`scripts/check-agent-hooks-parity.sh` (escopo de projeto) e `scripts/check-harness-hooks-parity.sh`
(escopo global) — ver as duas seções "... paridade estrutural" acima para o detalhamento dos gates.
Cobertura ponta-a-ponta (matcher gerado → script efetivamente bloqueando/alertando um payload real
de tool call Read/Write) é testada em `npm/tests/credential_guard.test.js` e
`pypi/tests/test_credential_guard.py` (cenário (b) da REQ).

## Princípios de design de gates (P1–P4)

Todo gate de paridade e toda regra do validator devem seguir os quatro princípios documentados em
[`docs/gate-design-principles.md`](gate-design-principles.md): nenhum número mágico (P1), falha
explícita sem degradação silenciosa (P2), independência de ambiente (P3) e falsificabilidade
obrigatória (P4). O arquivo inclui os quatro defeitos reais que motivaram os princípios e o
checklist de aceite para gates novos.

A implementação de P4 é `scripts/check-gates-falsify.sh` — todo gate novo de paridade registra
ali sua prova negativa.

## Release rule

Changes to commands, options, exit codes, JSON fields, validation rules, or
generated artifact semantics require equivalent tests in all affected runtimes.

`internal/serve/static` is the canonical dashboard asset source. Copies packaged
by npm and PyPI must remain byte-identical and are checked in CI.

Plugin downloads use timeouts, size limits, and atomic replacement. Registry
entries do not currently publish checksums; adding signed SHA-256 metadata is a
required compatibility change before checksum verification can become mandatory.

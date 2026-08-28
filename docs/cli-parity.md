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
| `serve` | yes | yes | yes | Local dashboard |
| `agents` | yes | yes | yes | `list`, `install`, `uninstall`, `update` across supported AI CLIs |
| `skills` | yes | yes | yes | `list`, `install`, `uninstall`, `update` across supported AI CLIs |
| `note` | yes | yes | yes | `new <title>` — creates `vault/notes/<slug>-YYYY-MM-DD.md` and links in `index.md`; idempotent (fails on duplicate) |
| `ship` | yes | yes | yes | Governed `git commit + push + open PR/MR` for `feat`/`fix`/`refactor`/`chore`/`docs` branches; hard governance gate for `feat`/`fix`/`refactor` only — `chore`/`docs` skip it (see below) |
| `push` | yes | yes | yes | Governed `git push` for already-committed work — never commits, never opens a PR/MR; same branch vocabulary and governance gate as `ship` (see below) |
| `branch` | yes | yes | yes | `new <type>/<slug>` — for `feat`/`fix`/`refactor`, gates `git checkout -b` on the same `branch_has_wip_roadmap` matching logic `trackfw validate` already applies, moving the check before branch creation instead of after; `chore`/`docs` create the branch without that gate, mirroring the housekeeping exemption `trackfw ship`/`trackfw commit` already grant those types (see below). `prune [--apply]` — reports (and, with `--apply`, deletes) local branches already integrated into `origin/main` via the touched-files heuristic (see below); `--dry-run` behavior is the default, `--apply` is opt-in |
| `gemini` / `cursor` / `copilot` / `windsurf` / `amazonq` | yes | no | no | Historical Go-only compatibility aliases |
| `version` / `--version` | yes | yes | yes | Both print the same single line: `trackfw <semver>`, no `v` prefix — see "Version output" below |
| `changelog` | yes | yes | yes | Reads `CHANGELOG.md` at project root; no flags prints the first `## [...]` section (`Unreleased` or latest version); `--version <x.y.z>` prints a specific section (accepts an optional leading `v`); `--all` prints the entire file. Error messages byte-identical: `CHANGELOG.md not found — nothing to show`, `version "<x>" not found in CHANGELOG.md` |

## Version output

<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->

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

<!-- trackfw-contract: gate=scripts/check-release-tag-parity.sh partial=cobre os 4 arquivos-fonte de versão (inclusive o fallback duplo de pypi/trackfw/__init__.py) só como pré-condição de `release tag`, indiretamente; não testa a leitura em si (internal/version/version.go, npm/package.json, importlib.metadata) fora desse fluxo -->

| Runtime | Source |
|---|---|
| Go | `internal/version/version.go`, stored **without** the `v` |
| Node.js | `npm/package.json` |
| Python | `importlib.metadata`, with a literal fallback in `pypi/trackfw/__init__.py` |

In Go both surfaces consume `version.Version` — `internal/commands/version.go` for the subcommand and
the cobra `Version` field in `internal/commands/root.go` for the flag — so the stored value governs
both.

### Gate assertion — pinned, and why the old one was vacuous

<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->

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

<!-- trackfw-contract: gate=scripts/check-cli-parity.sh -->

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

<!-- trackfw-contract: gate=scripts/check-cli-parity.sh partial=a exigência positiva (-v não vincula e sai não-zero nos 3 runtimes) é coberta pelo mesmo gate da seção-mãe; a tabela de mensagem/exit code exatos por runtime para QUALQUER flag desconhecida (--zzz) é medida como baseline mas não verificada por nenhum gate -->

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

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh partial=regra note_orphan não comparada entre os 3 CLIs -->

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

## i18n locale keys — no orphan keys (ML-2A)

<!-- trackfw-contract: gap reason=a seção fixa fato falsificável (errors.notFound ausente e sem consumidor nos 3 CLIs) mas nenhum gate compara chaves de locale entre runtimes; ver REQ-2026-08-16-conformidade-estrutural-e-comportamental-de-i18n-entre-os-tres-clis -->

> ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md

`errors.notFound` existed in the Node.js and Python locale files
(`npm/src/i18n/locales/*.json`, `pypi/trackfw/i18n/locales/*.json`) but never in Go's
(`internal/i18n/locales/*.json`), and had **no consumer in any of the three runtimes** — orphan in
all three, not just missing in one. Removed from Node.js and Python rather than added to Go: a key
nobody reads is dead weight, and adding it to Go would have created a third copy of unused
content.

The removal is scoped to this one key. Auditing all three locale trees during this ML surfaced 31
further keys that diverge between stacks (present/absent, or textually different) — those are
**not** touched here; they are tracked by
`docs/req/REQ-2026-08-16-conformidade-estrutural-e-comportamental-de-i18n-entre-os-tres-clis.md`,
a REQ of its own. `docs/cli-parity.md` does not otherwise document i18n locale-key parity as a
contract; this section exists only to record that `errors.notFound` specifically is gone from all
three, not to claim the broader 31-key set is resolved.

## Site documentation drift (ML-2B) — out of this document's contract, registered anyway

<!-- trackfw-contract: none reason=a própria seção declara que docs/cli-parity.md não contrata prosa de documentação (site/guide), só comportamento de CLI; registrada apenas como paper trail de uma limpeza -->

`site/guide/commands.md` and `site/en/guide/commands.md` are pt/en mirrors of the same command
reference; `docs/cli-parity.md` does not contract them, because this document pins **CLI
behavior**, not documentation prose. ML-2B (same roadmap as the other items in this section)
removed `trackfw plugins` (no longer exists, ADR-2026-08-15) and added `changelog` and `commit`
(existed but were undocumented) to both files, verified against the real `trackfw --help` output
rather than `README.md`. Registered here only so the cleanup has a paper trail; it is not a new
clause of the parity contract.

## Canonical governance references

<!-- trackfw-contract: gap reason=a regra de match literal sem fallback por basename (extractRefPath) é comportamento falsificável do validador mas nenhum gate cross-CLI testa a rejeição de uma referência por basename ou por diretório de estado errado nos 3 runtimes -->

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

<!-- trackfw-contract: gate=scripts/check-roadmap-move-parity.sh -->

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

<!-- trackfw-contract: gap reason=nenhum gate cross-CLI exercita req list/req move — nem a descoberta por layout (flat/by_agent) nem a discriminação in-place-vs-physical-move são comparadas entre Go, Node.js e Python -->

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

<!-- trackfw-contract: gap reason=nenhum gate compara os 3 arquivos docs/schema/*.json publicados por init entre os 3 runtimes; check-artifact-parity.sh tem uma lista fixa de KINDS que não inclui os schemas -->

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

<!-- trackfw-contract: gap reason=o contrato de stale_wip (fonte .trackfw-log, fallback mtime, thresholds, severidade) e os diagnósticos de inspeção (ENOTDIR, arquivo ilegível) não têm gate cross-CLI; cobertura hoje é só por suíte de testes interna a cada runtime -->

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

<!-- trackfw-contract: gate=scripts/check-integration-cli-parity.sh partial=comportamento não-interativo sem --targets (abre seletor TTY / exige a flag) não é exercitado por este gate, que sempre passa --targets explícito -->

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

<!-- trackfw-contract: gate=scripts/check-identity-parity.sh partial=o target "opencode" entra na lista derivada do catálogo (support_level != unsupported) e tem seus artefatos comparados byte a byte nos 3 runtimes; as RAZÕES da representação (mode: subagent fixo, model:/tools:/memory: omitidos deliberadamente) são justificativa pinada que o diff de bytes não discrimina — os 3 runtimes concordando num valor errado ainda passaria -->

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

<!-- trackfw-contract: gap reason=nenhum gate exercita a tabela de resolução de --scope sem flag/sem TTY (D5/D6/D8: default global em install/update, erro obrigatório em uninstall) nos 3 runtimes; os gates de integração existentes sempre passam --scope explícito -->

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

<!-- trackfw-contract: gap reason=nenhum gate verifica que a escrita do Codex (install e o re-sync de update) cai em scope=project nos 3 runtimes; a seção mesma nota que não é alcançável pela flag pública --scope, então precisaria de fixture dedicada que hoje não existe -->

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

## `update` refusing unmanaged content — message names the remedy (item 8, ML-2C)

<!-- trackfw-contract: gap reason=a própria seção se autodeclara "Parity gap, not yet closed by a gate" — a paridade da mensagem foi verificada lendo o código-fonte dos 3 runtimes, não por um gate que roda os 3 binários e diffa stdout/stderr real -->

> ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md

**Behavior is unchanged and intentional, not a bug.** `update`'s preflight refuses to touch bytes
trackfw did not write, deliberately **ignoring `--force`** — overwriting a file the user created
or hand-edited outside of trackfw would be destructive. This applies even when the destination
path matches a claimed artifact but the on-disk bytes match no known trackfw template. Only
`install --force` may adopt/overwrite unmanaged content; `update --force` never does, by design.

The defect fixed here was diagnosability, not the refusal itself: the error named no remedy, and
`--force`'s own `--help` text promised "replace or remove modified managed artifacts" — leading a
user straight into trying the operation that had just failed.

**Message, byte-identical in all three runtimes:**

```
unmanaged artifact "<destination>" does not match a trackfw template — trackfw did not write these bytes.
Adopt it with: trackfw <kind> install --force --items <item> --targets <target> --scope <scope>
```

`<kind>`, `<item>`, `<target>`, `<scope>` are filled in from the plan's own `Claim` for that
artifact — never a generic placeholder. Canonical source:
`internal/integrations/manager.go:unmanagedArtifactError` (Go); mirrored in
`npm/src/integrations/manager.js:189` and `pypi/trackfw/integrations/manager.py:305-310`.

**`--force` help text**, corrected to stop over-promising what `update --force` does:

```
Replace a modified managed artifact; never adopts unmanaged bytes — use 'install --force' for that
```

**Non-regression:** `update --force` still refuses unmanaged bytes — never silently adopts them.
Covered per-stack by `TestManagerUpdateForceNeverAdoptsUnknownUnmanagedContent` (Go), `"install
force replaces unknown unmanaged content while update force never does"` (Node.js), and
`test_update_force_never_claims_unknown_unmanaged_file` (Python), plus an end-to-end scenario in
`check-gates-falsify.sh` that runs the real repro and asserts `exit != 0`.

**Root cause investigated, fix deferred.** The artifacts end up unmanaged-on-disk because
`Manager.mutate()` writes every batch's bytes to disk **before** persisting the manifest — an
interruption between the two loops leaves correctly-written files with no manifest entry. This is
a pattern, not an isolated incident. Fixing it needs detection (a `validate`/`doctor` rule) and/or
reordering persistence — out of scope for this ML; see
`vault/notes/integrations-manifest-write-precedes-persist-janela-de-registro-parcial-2026-08-16.md`.

**Parity gap, not yet closed by a gate.** The byte-identity claim above was verified by reading
the source of the three implementations, **not** by a gate that runs all three CLIs and diffs
their real stdout/stderr for this message. The three per-stack tests listed above assert the
message *within* each runtime; none of them assert equality *across* runtimes. Until such a gate
exists, a future edit to only one of the three copies would go undetected by `make quality`.
Recommendation, not implemented here: add a scenario to a `check-integrations-parity.sh`-style
script (or extend an existing one) that reproduces the unmanaged-artifact case against all three
binaries and three-way-diffs the message, following the same pattern already used by
`check-ship-parity.sh` and `check-branch-new-parity.sh`.

A related, still-open divergence found during the same investigation but **not** fixed here: the
error-wrapper prefix in `internal/integrations`/its Node.js/Python equivalents also diverges — Go
uses `Error:`, Python uses `trackfw agents update:`, and Node.js prints the raw source line of the
`throw` (a stack-trace leak on an expected error path). Same class of problem as the `ship`
stream/prefix divergence fixed above, but only `ship` was fixed by this roadmap; this one remains
open for a future ML.

## Non-interactive `--targets` error message (pre-existing, not part of the

<!-- trackfw-contract: gap reason=as duas strings divergentes (Go/Node vs Python) são pinadas literalmente e testadas cada uma dentro do próprio runtime; nenhum gate cross-CLI impede uma edição futura de afastar ainda mais os dois textos aqui fixados -->
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

<!-- trackfw-contract: gap reason=a divergência de exit code (1 em Go/Node, 2 em Python) é documentada como aceita e pré-existente, mas nenhum gate cross-CLI a verifica ativamente; uma mudança acidental do 2 do Python passaria despercebida -->

Go and Node exit `1` (the default for cobra/Node's uncaught-throw path) on
integration errors (invalid `--scope`, missing `--targets`, `uninstall`
without `--scope`, etc.). Python's `agents`/`skills` command handler
(`pypi/trackfw/integrations/command.py`) catches
`IntegrationError | OSError | ValueError` and exits `2`
(`raise SystemExit(2) from error`) — a pre-existing Python-CLI convention,
unrelated to and unaffected by the install-scope feature.

## Agent identity

<!-- trackfw-contract: gate=scripts/check-identity-parity.sh -->

Agent identity is a cross-runtime contract, not a per-distribution feature. The
Go, Node.js, and Python CLIs read the **same** configuration file and must
produce the same artifact bytes for the same input.

### Shared configuration

<!-- trackfw-contract: gate=scripts/check-identity-parity.sh partial=o fallback "sem identity.json" é testado byte a byte nos 3 runtimes (HOME_WITHOUT); schema_version inválido, agents vazio e entrada ausente para um id específico não são exercitados -->

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

<!-- trackfw-contract: gate=scripts/check-identity-parity.sh partial=cobre só o ponto 1 (slugs de preset hardcoded, renderizados e comparados byte a byte nos 3 runtimes); o modo custom (slugificação dinâmica) e a rejeição de entrada inválida/colisão de slugs (pontos 2 e 3) não são exercitados -->

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

<!-- trackfw-contract: gate=scripts/check-identity-parity.sh -->

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

<!-- trackfw-contract: gate=scripts/check-identity-parity.sh,scripts/check-gates-falsify.sh -->

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

<!-- trackfw-contract: gap reason=a própria seção declara que a UX do wizard "has no automated cross-CLI test yet" e depende só de revisão manual -->

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

<!-- trackfw-contract: gate=scripts/check-ship-parity.sh partial=roda com TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 e a maioria dos cenários usa --no-pr/--dry-run, então os passos 5–7 (commit real, push real, abertura de PR/MR) quase nunca executam ponta a ponta; --forge, resolução de forge e a tabela de adaptadores também não são exercitados (ver subseções) -->

`trackfw ship` runs a seven-step governed delivery sequence in all three runtimes:

```
1. Validates branch name — must match feat|fix|refactor|chore|docs/<slug>
2. Validates governance — REQ + roadmap in wip/ or done/ must exist for feat/fix/refactor branches
   (hard gate: not affected by lenient mode or per-rule severity); chore/docs branches skip this
   check entirely, mirroring `trackfw commit`
3. Detects pending squash-merges in other branches (advisory only)
4. Reviews what is staged (git status --short + git diff --cached --stat)
5. Commits with Conventional Commits format (-m is required)
6. Pushes to origin (adds -u if no upstream is configured yet)
7. Opens PR/MR via the resolved forge CLI, or prints a browser fallback URL if the CLI is absent
```

### Flags

<!-- trackfw-contract: gate=scripts/check-ship-parity.sh partial=--forge não é exercitado por nenhum cenário deste gate -->

| Flag | Type | Description |
|---|---|---|
| `-m` / `--message` | string | Commit message (Conventional Commits format required) |
| `--dry-run` | bool | Print what would be done without executing write commands; in step 7, also reports forge CLI availability and prints the fallback URL when CLI is absent |
| `--no-pr` | bool | Skip PR/MR creation after push (steps 1–6 still run) |
| `--forge` | string | Override forge detection (`github`, `gitlab`, `bitbucket`, `azure`) |

### Forge resolution and `forge:` field

<!-- trackfw-contract: gap reason=nenhum gate testa a precedência de resolução do forge (flag > trackfw.yaml > remote URL > CI file > manual) nem a mensagem impressa com a origem resolvida -->

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

<!-- trackfw-contract: gap reason=nenhum gate testa os padrões de URL de fallback ou o CLI/substantivo por forge (gh/glab/az/bitbucket) da tabela de adaptadores -->

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

<!-- trackfw-contract: gap reason=nenhum gate testa o passo 7 com o CLI de forge ausente (fallback de URL impresso, exit 0) nem o texto de --dry-run correspondente, byte a byte entre os 3 runtimes; check-ship-parity.sh desabilita comandos externos mas roda majoritariamente com --no-pr, sem chegar ao passo 7 -->

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

<!-- trackfw-contract: gap reason=nenhum gate fixa governance_mode: lenient em trackfw.yaml e confirma que ship ainda aborta — a diferença central desta seção (ship ignora lenient mode; validate não) nunca é exercitada; check-ship-parity.sh só prova o bloqueio com config default -->

`trackfw validate` respects `governance_mode: lenient` (configured in `trackfw.yaml`)
and per-rule severity overrides — in lenient mode, governance violations become warnings
and exit code is 0.

`trackfw ship` does **not** respect lenient mode or per-rule severity, for `feat`/`fix`/`refactor`
branches. The governance check in step 2 (`CheckShipGovernance`) is always a hard gate for those
three types: a branch without a linked REQ and a roadmap in `wip/` or `done/` **always** aborts
ship with exit code 1, regardless of `governance_mode` or `rules:` configuration. `chore`/`docs`
branches skip step 2 entirely, by branch type — see "Two independent exemption axes" under
`trackfw branch new` below for how this differs from the staged-content doc-only exemption.

**Why:** `ship` is a delivery gate, not an audit tool. Lenient mode exists for teams
still ramping up governance — it does not mean "ship without governance artifacts".

**Impact on users:** a team running `governance_mode: lenient` may see `trackfw validate`
pass (exit 0) but `trackfw ship` abort. This is intentional. The error message from step
2 explicitly mentions lenient mode to prevent confusion.

### Step 2 governance check — shared implementation, byte-identical output (ML-1B)

<!-- trackfw-contract: gate=scripts/check-ship-parity.sh -->

> ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md

`ship`'s step 2 (`CheckShipGovernance` in Go, `checkShipGovernance`/`check_ship_governance` in
Node.js/Python) no longer contains its own copy of the `branch_has_wip_roadmap` matching logic.
It delegates to the exact same validator functions `trackfw validate` and `trackfw branch new`
already use — `validator.validateBranchHasWIPRoadmap` / `validator.validateWIPHasREQ` in Go,
their Node.js (`npm/src/validator/index.js`) and Python (`pypi/trackfw/validator.py`)
equivalents. Before this fix, Node.js and Python reimplemented the rule from scratch, with their
own wording **and without ever scanning `done/`** — a branch with a matching roadmap only in
`done/` would pass in Go and incorrectly fail in Node.js/Python. This was a behavioral
divergence, not only a wording one.

**Error stream and prefix.** The message and stream are byte-identical across the three
runtimes: **stderr**, prefixed with `Error: `. This was already Go's actual behavior without any
code change — `internal/commands/root.go`'s `Execute()` wrapper prints `Error: <msg>` to stderr
for any `RunE` that does not opt out via `cmd.SilenceErrors` (unlike `branch.go`/`commit.go`,
which deliberately silence it to print without the prefix), and `ship.go` never set
`SilenceErrors`. The fix was entirely on the Node.js/Python side: `runShip`/`run_ship` gained a
`writeErr`/`write_err` parameter (default: writes `Error: <msg>\n` to stderr), used only for the
final one-line summary of each abort path; the detailed body (violation list, remediation hints,
the `Note: ...` block) still goes to **stdout** via `writeln`, exactly as Go already did through
`deps.out`.

Covered by `scripts/check-ship-parity.sh` (`feat-still-gated-non-regression` and
`invalid-branch-vocabulary` scenarios), which now runs a full three-way `diff -u` of stdout
**and** stderr for these cases, not a substring/exit-code-only assertion.

### Usage silencing

<!-- trackfw-contract: gap reason=nenhum gate verifica que a saída de uso (usage/help) é suprimida em erros de runtime (padrão de branch, gate de governança, nada staged, -m ausente) nos 3 runtimes -->

Runtime errors (branch pattern, governance gate, nothing staged, missing `-m`) set
`SilenceUsage = true` inside the command handler (Go/cobra) or return a non-zero exit
code directly from the runner function (Node.js/Python), so the usage text is never
printed for runtime errors. Parse-time errors (unknown flags) still show usage, because
they are raised by cobra/commander/argparse before the command handler runs.

### `ship --force-with-lease` — governed force-push (ML-1B)

<!-- trackfw-contract: gate=scripts/check-ship-force-parity.sh -->

> ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md,
> ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md

`--force-with-lease` pushes with `git push --force-with-lease` instead of a plain push — for the
post-rebase case, where a plain push is rejected as non-fast-forward. Raw `--force` is **never**
exposed as a flag in any of the 3 CLIs (Python additionally sets `allow_abbrev=False` on the
`ship` subparser, so `argparse` cannot silently resolve a bare `--force` to `--force-with-lease`
by prefix matching — the only other `--f...` flag on the parser).

The flag only runs step 6's push once a new gate (step 2.5, before any write) has confirmed the
branch already has an **open** PR/MR via the resolved forge CLI (`gh`, `glab`, or `az`) — the safe
path is always to open the PR first. This produces three distinct refusal classes, never
conflated:

```
no forge CLI available   → refuses, names gh/glab/az, never degrades to a permissive push
forge CLI available,
  zero open PR/MR         → refuses, names the branch, points at opening the PR/MR first
forge CLI available,
  query itself fails      → refuses, "could not verify ...", surfaces the CLI's own stderr text
forge CLI available,
  PR/MR confirmed open    → pushes with --force-with-lease, skips PR/MR creation in step 7
```

"Cannot verify" (the forge CLI errored, e.g. an auth failure) is a **separate** refusal from
"no open PR/MR" (the query succeeded and returned zero results) — conflating them would make a
`gh` authentication failure look like "no PR exists yet", nudging the caller to open a PR that
already exists.

When nothing is staged (the common post-rebase-with-conflicts-resolved case, where the index is
already clean), `--force-with-lease` pushes the existing local commits without requiring `-m` —
this exception does not apply without the flag; a plain `ship` with nothing staged still aborts
exactly as before.

**Stderr-text parity fix (ML-1B).** Building this gate found a real divergence: Go's
`exec.Command().Output()` error alone formats as the generic `"exit status N"`, discarding the
forge CLI's actual diagnostic text — while Node's `spawnSync` and Python's `subprocess.run`
already surfaced the real stderr. `defaultCheckPROpen` and `defaultGitExec`
(`internal/commands/ship.go`) now capture `cmd.Stderr` explicitly and use its trimmed text in the
refusal/error message, matching Node.js/Python byte-for-byte. This affects both the "cannot
verify" force-with-lease refusal and every `git commit`/`git push` failure message `ship` ever
prints, not only the force-with-lease path.

**Gate: `scripts/check-ship-force-parity.sh`** (registered in the `parity` Make target). Real bare
git remotes only — never a mocked `git`, per the project's standing gate-fixture convention (see
`check-branch-prune-parity.sh`). Five scenarios, each diffed byte-for-byte across the 3 runtimes
on stdout, stderr, and exit code:

- `no-forge-cli`, `forge-zero-pr`, `forge-unverifiable`, `forge-pr-open-pushes` — the four paths
  above; the success path is proved by the remote SHA actually changing
  (`git --git-dir=<bare> rev-parse <branch>` before/after), never by the printed message alone.
- `remote-advanced-lease-mismatch` — the semantic discriminant, stronger than inspecting the push
  argv string: a second clone pushes a legitimate commit to the branch after our clone last
  recorded its state (our clone's remote-tracking ref is pinned stale on purpose — fetch refspec
  restricted to `main`). The correct `--force-with-lease` refuses (real git safety semantics: the
  remote moved past what we last knew) and the other clone's commit survives untouched. A raw
  `--force` would push through unconditionally and destroy it — this is exactly what
  `scripts/check-gates-falsify.sh`'s P4 scenario (Cenário 73) sabotages via a single-literal
  change to `ship.go`'s push-arg construction on an isolated Go copy, and what this scenario
  catches: the sabotaged binary's push exits 0 and the other party's commit disappears from the
  remote.

### `trackfw release tag <version>` — governed release publication (ML-2A/ML-2B)

<!-- trackfw-contract: gate=scripts/check-release-tag-parity.sh -->

> ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md,
> ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md

`release tag` exists because `trackfw ship` only pushes branches — tagging a release is not a
branch operation, and `ship`'s governance gate ("REQ + roadmap in wip/") does not apply here. It
is a separate `release` command group (`trackfw release tag <version>`), not a `ship` flag.

**Nine refusal paths, all checked before any write, every one naming what to fix — this command
never guesses:**

1. **Dirty working tree** — refuses, lists the files via `git status --porcelain`, and names
   `trackfw commit` as the fix. **Never recommends `git stash`** — the git-branch-guard has
   blocked `stash` since ML-3A of this same roadmap, and the product would otherwise be
   recommending a command it refuses itself (the ML-2B coherence fix this gate's Scenario 1
   pins).
2. **Local default branch stale against `origin`** — the tag always targets
   `origin/<default>` (`main`/`master`, resolved the same way `ship` resolves it), never the
   branch currently checked out. If a local branch with that same name exists and diverges from
   `origin/<default>`, refuses naming `git pull`; if no such local branch exists, the check is
   skipped — this is what lets `release tag` run from any checked-out branch as long as the
   working tree is clean. **This local resolution is a cross-check candidate only — see the
   "commit-target ancoring" note below for what actually decides the published commit.**
3. **The 4 version files disagree with `<version>`** — `internal/version/version.go`,
   `npm/package.json`, `pypi/pyproject.toml`, and `pypi/trackfw/__init__.py` (checked twice: the
   `importlib.metadata` fallback and the `except`-block fallback, since both hold a version
   literal) — 5 checks in total. Refuses naming exactly which file/occurrence diverges and both
   values.
4. **`CHANGELOG.md` missing the version's section** — refuses unless a `## [<version>] -
   YYYY-MM-DD` heading exists; the matched section becomes the tag message
   (`changelog.FormatSection`/`format_section`/`formatSection` — the module `ship`-adjacent
   tooling already reuses, not duplicated).
5. **Tag already exists locally.**
6. **Tag already exists on `origin`.**
7. **No forge CLI available** — `release tag` currently only supports GitHub; refuses naming
   `gh` and the manual fallback (`git tag -a ... && git push origin ...`), which does not
   actually work if attempted — see the note on the guard below.
8. **Forge resolves to something other than GitHub** — refuses naming the resolved forge, the
   commit to tag, and points at the forge's web UI or an issue requesting support; deliberately
   does **not** suggest `git push origin <tag>`, since the `git-branch-guard`'s `case push)`
   blocks that push form unconditionally regardless of the reason it's being run for.
9. **Git identity missing** — refuses unless both `git config user.name` and `user.email` are
   set, naming both commands to fix it. The tag is always annotated, and an annotated tag
   requires a tagger identity.

**Commit-target ancoring — the forge decides the branch NAME unconditionally, a local ref only
cross-checks the SHA (ADR Emenda 1, 2026-08-19).** The barrier review (`docs/seguranca/2026-08-19-
revisao-do-push-forcado-e-do-release-tag.md`) found that resolving the tag's commit purely from
local refs — `git symbolic-ref refs/remotes/origin/HEAD` for the default branch name,
`git rev-parse origin/<base>` for its commit — is **not** an adequate anchor: both are gravable
inside the clone (`git symbolic-ref`, `git update-ref`), and the command's own internal
`git fetch origin --prune` does not reliably protect them — a `remote.origin.fetch` narrowed to
exclude the target branch leaves a forged or stale `origin/<base>` untouched by that fetch. The
fix: **the commit target comes from the forge**, via two GitHub GET calls (reusing the same `gh`
credential already used for the two POST calls below):

```
GET repos/{owner}/{repo}                      (gh api repos/{owner}/{repo})
  -> .default_branch                          — AUTHORITATIVE for the branch name, unconditionally.
                                                  No comparison against the local symref-derived
                                                  base exists: a fresh/shallow clone legitimately
                                                  has no origin/HEAD symref at all (the local
                                                  resolver then falls back to "main"), so refusing
                                                  on a name disagreement would be a false refusal
                                                  against a legitimate repo, not a security check.

GET repos/{owner}/{repo}/commits/{branch}     (gh api repos/{owner}/{repo}/commits/<forge's
                                                  default_branch>)
  -> .sha                                     — this is the `object` field in the POST git/tags
                                                  payload below. Cross-checked against a FRESH
                                                  local read of origin/<forge's default_branch>
                                                  (never against origin/<local symref-derived
                                                  base>, which may name a different branch) — that
                                                  local read is best-effort/non-fatal (absence must
                                                  not block reaching the forge), but when it DOES
                                                  resolve, disagreement with the forge is always a
                                                  refusal, never resolved by picking either side
```

The local symref-derived base is used ONLY for the pre-existing "local default branch stale
against origin" check (precondition 2 above, comparing two local refs unrelated to the forge) and
as an informational hint in the two forge-unreachable refusal paths (no forge CLI / unsupported
forge) — it plays no role in deciding what gets tagged. A repointed `origin/HEAD` symref is
therefore **neutralized, not refused**: the command silently uses the forge's real branch and
publishes against its sha, ignoring the repoint (Scenario 11 below).

The sha-divergence refusal message names both sides (`releaseTagCommitDivergesFmt` — kept
byte-identical across the 3 CLIs, same convention as every other refusal in this command).

`defaultBaseBranch`/`_default_base_branch` (shared with `ship`'s PR-body commit range) was also
fixed to strip the literal `refs/remotes/origin/` prefix instead of cutting at the last `/` —
the old logic broke a default branch name that itself contains a slash (e.g. `release/7.2`
resolved to `7.2`). Two consumers share this fix in each CLI: `release tag`'s own resolution and
`ship`'s PR body (`gitCommitsSince`/`buildPRBody`).

**Publication contract — two GitHub API calls, in order, second only on first's success (the
`object` field below is the forge-resolved sha from the GET call above, never a local one):**

```
POST git/tags   (gh api repos/{owner}/{repo}/git/tags   --input -)
  body: {tag, message, object: <forge-resolved commit sha>, type: "commit", tagger: {name, email, date}}
  -> returns the sha of the TAG OBJECT (not visible via any ref yet)

POST git/refs   (gh api repos/{owner}/{repo}/git/refs   --input -)
  body: {ref: "refs/tags/<tag>", sha: <the git/tags response's sha — NOT the commit sha>}
```

Both calls are required, and in this order: the first creates the tag object (carrying the
message/tagger) but nothing points at it yet; the second creates the ref pointing at that
object. **The second call alone — or a ref payload wired to the commit sha instead of the tag
object's sha — creates a *lightweight* tag**: the ref exists, `git describe`/`git tag -l` find
it, and the loss (no message, no tagger) is invisible until someone inspects the tag object
itself. This is exactly the regression the ADR names as the risk `release tag` exists to avoid,
since a plain `git push origin <tag>` from a lightweight local tag has the same failure mode and
the guard blocks that push form anyway. The second call is never issued if the first fails.

`{owner}`/`{repo}` are the literal placeholders `gh api` itself expands from the current
repository context — the remote URL is never hand-parsed for this.

**Content anchorage — version files and `CHANGELOG.md` read from the forge commit object, not
the working tree (ADR-2026-08-21, ML-2A).** The barrier review
(`docs/seguranca/2026-08-21-revisao-da-ancoragem-de-versao-e-mensagem.md`) found that reading
version and CHANGELOG from the local working tree allowed an attacker to publish a tag whose
message was written locally and never appeared in any auditable commit. The fix extends the same
sha-based authority already used for the commit target: version files (the 5 checks in P3 above)
and `CHANGELOG.md` (P4) are now read via `git show <objectSHA>:<path>`, where `objectSHA` is the
forge-resolved commit sha. Git objects are content-addressed — given a sha, the content is
cryptographically determined. The read is local (no extra API call), but the authority is the sha,
which comes from the forge. `readFile` was **removed** from the `releaseDeps` struct entirely (not
left unused): any future attempt to re-introduce a working-tree read fails to compile, making
silent fallback structurally impossible rather than merely guarded. If the requested object is
absent locally, the command refuses naming both the path and the sha — never falls back to the
working tree. **This resolves the PR-bump false-positive without exception**: since `objectSHA` is
the tip of the default branch *after* the PR merge, the bumped version files and the new CHANGELOG
section are already in it, so there is no divergence to tolerate. The gate name for this anchorage
is `check-release-tag-parity.sh` (Scenarios 15 and 16 added in ML-2B; Scenario 17 added in ML-4A).

**Gate: `scripts/check-release-tag-parity.sh`** (registered in the `parity` Make target). A real
bare git remote, local and offline, `$HOME`/`GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM` isolated at
both fixture-construction time and invocation time (the identity precondition means a developer
machine's real global `git config user.name` must never leak in). Publication always goes
through a local `gh` stub — no scenario, including success, ever reaches a real GitHub API. The
stub answers all four `gh api` calls the command makes (the two GET commit-target calls and the
two POST publish calls); only the two POST calls are recorded to the scenario's call log, so the
"refusal must never publish" assertions stay meaningful on every scenario that reaches
precondition 6 (forge resolution), where both GET calls happen before the identity check.
Seventeen scenarios (21 RT_LABELs — scenario 3 splits into 3a–3e), byte-diffed across the 3
runtimes on stdout, stderr, and exit code:

- Scenarios 1–9 exercise the nine refusal paths above (3 split into 3a–3e, one isolated
  mismatch per version-file check).
- Scenario 10 ("success") is the load-bearing one: it asserts the **SHA linkage** between the
  two `gh api` POST calls — the `git/refs` payload's `sha` must equal the `git/tags` response's
  returned sha (a fixed stub value, deliberately different from the commit sha), never the
  commit sha directly. This is what `scripts/check-gates-falsify.sh`'s Cenário 75 sabotages
  (single-literal change on `internal/commands/release.go`'s `refPayload` construction —
  `SHA: tagObj.SHA` reverted to `SHA: objectSHA` — in an isolated Go copy) and what this
  scenario's linkage assertion catches: the sabotaged binary still exits 0 and still prints a
  "Tag published" success message, but the ref it created points at the commit, not the tag
  object.
- **Scenarios 11–13 (ML-4B, ADR Emenda 1) exercise adversarial selection of the commit target**
  via the two different local refs the forge-anchoring code touches. 12 and 13 prove the sha
  cross-check refuses on divergence; 11 proves the opposite property on purpose — the branch NAME
  has no local-vs-forge comparison at all, so a repointed symref is neutralized, not refused:
  - **11 — repointed `origin/HEAD` symref**, locked against git's own auto-resync via
    `remote.origin.followRemoteHEAD never` (git ≥2.48 otherwise silently restores a repointed
    symref during the command's own internal `git fetch origin --prune`, making the scenario
    vacuous without this — see `vault/notes/git-fetch-self-heals-forged-origin-head-and-
    tracking-refs-2026-08-19.md`). The forge's `default_branch` (`main`, via the stub) disagrees
    with the repointed local base name (`chore/other`) — asserts **success** (exit 0), that
    stderr never mentions `chore/other`, and that stdout echoes the forge's real `main` commit
    sha, proving the repoint was silently ignored and both publish calls still fired.
  - **12 — `origin/main` forged via `git update-ref`, under a `remote.origin.fetch` refspec
    narrowed to exclude `main`** (without narrowing, the command's own fetch overwrites the
    forgery before it is ever read — same vault note). `refs/heads/main` is reset to the same
    forged sha via `git reset --hard` so the pre-existing local-branch-staleness check (a
    different, unrelated precondition) cannot discriminate this corruption instead of the new
    forge-comparison code. The forge's real tip (via the stub) disagrees — refused.
  - **13 — `remote.origin.fetch` narrowed with no active forgery**: a second, independent clone
    legitimately pushes one more commit to `main` on the bare remote; the fixture's narrowed
    refspec means its own internal fetch never learns about it, so `origin/main` stays naturally
    stale. The forge (via the stub) reports the advanced tip — refused. Mirrors
    `check-ship-force-parity.sh`'s "remote-advanced-lease-mismatch" narrowing technique.

  All three assert non-publication (no `git/tags`/`git/refs` request files) on top of the
  refusal. Scenario 12/13's underlying divergence check is what `scripts/check-gates-falsify.sh`'s
  Cenário 76 sabotages (single-literal `false &&` prefix neutering
  `if localSHA != "" && localSHA != commitObj.SHA {` in an isolated Go copy) — the sabotaged
  binary still resolves and prints the forge's sha correctly, but the refusal is gone.
- **Scenario 14 (ML-2A) — `refs/remotes/origin/main` absent**: proves that an absent local
  tracking ref (as opposed to an absent git object) must not block publishing. Uses the same
  pin+delete technique as Scenarios 12/13: `remote.origin.fetch` pinned to a decoy branch,
  `refs/remotes/origin/main` deleted. The forge stub returns the decoy commit's sha — distinct
  from `main`'s sha (self-discriminant: if the refspec ever stops isolating and `git fetch`
  repopulates `origin/main`, the cross-check refuses, flipping the "expected exit 0" assertion
  loudly red). Decoy commit carries the same tree as `main`, so P3/P4 (`git show <sha>:path`)
  succeed on valid objects.
- **Scenario 15 (ML-2B) — git object absent**: proves that when the forge returns a sha whose
  git objects do not exist locally, all 3 CLIs refuse naming both the path and the sha. Uses the
  same pin+delete technique (forgeLocalSHA = "" → cross-check skipped → reaches P3 → `git show
  FAKE_ABSENT_SHA:<path>` fails). FAKE_ABSENT_SHA (40 × 'a') is proven absent by a vacuity
  guard (`git cat-file -e`). No-publish guard: no `git/tags` POST may be reached.
- **Scenario 16 (ML-2B) — content-from-commit provenance**: proves BOTH that the legitimate
  PR-bump flow succeeds (case 2) and that the tag message content comes from the forge commit,
  not the working tree (case 3). Two-axis fixture: HEAD (local main) carries version 9.9.7 and
  CHANGELOG with `- head-only`; forge commit (decoy) carries version 9.9.9 and CHANGELOG with
  `- forge-only`. Both CHANGELOG carry a `## [9.9.9]` section so exit code alone cannot
  discriminate the two sources — only the section body can. Real binary reads from objectSHA →
  version 9.9.9 passes, message = "forge-only". Provenance proven by asserting the `message`
  field in the captured `git/tags` POST payload contains "forge-only" and not "head-only". The
  two-axis design makes each anchored read independently falsifiable: bypassing the version read
  (objectSHA → "HEAD") makes P3 see 9.9.7 ≠ 9.9.9 → exit non-zero; bypassing the CHANGELOG
  read (objectSHA → "HEAD") keeps exit 0 but payload message = "head-only" → provenance
  assertion fires. This second bypass is exactly what `scripts/check-gates-falsify.sh`'s Cenário
  87 sabotages (`deps.readCommittedFile(objectSHA, "CHANGELOG.md")` →
  `deps.readCommittedFile("HEAD", "CHANGELOG.md")` in an isolated Go copy).
- **Scenario 17 (ML-4A) — `refs/replace/` object-identity bypass**: proves that
  `--no-replace-objects` (first argument after `git` in all 3 CLIs) blocks the attack where an
  adversary writes `.git/refs/replace/<forge-sha>` = `<attacker-sha>` as a raw file (no git
  command; bypasses any branch guard). Without the flag, `git show <forge-sha>:CHANGELOG.md`
  follows the redirect and returns the attacker's content. Three-axis fixture: HEAD at 9.9.7
  with `- head-only`; forge commit (s17-decoy, pushed) at 9.9.9 with `- forge-only`; attacker
  commit (s17-attacker, LOCAL ONLY, never pushed) at 9.9.9 with `- refs-replace-forged`. Replace
  ref written as a raw file: `printf '%s\n' <attacker-sha> > .git/refs/replace/<forge-sha>`.
  Three vacuity guards: V1 (origin/main tracking ref gone — cross-check skipped, same as S14–16);
  V2 (attack genuine — raw `git show` without flag returns "refs-replace-forged"); V3 (fix works —
  `git --no-replace-objects show` returns "forge-only"). Post-run guard: replace ref still present
  and still active after all three CLI invocations, proving the fix suppresses a live redirect.
  Provenance assertions per runtime: message contains "forge-only" AND does NOT contain
  "refs-replace-forged". The per-runtime assertion makes the scenario correlated-revert-proof:
  all three stacks dropping the flag → each runtime fires independently; `assert_three_way` catches
  single-stack revert. `scripts/check-gates-falsify.sh`'s Cenário 158 sabotages
  (`"--no-replace-objects", "show"` → `"show"` in an isolated Go copy) and proves this provenance
  assertion catches the false negative.

## `trackfw push`

<!-- trackfw-contract: gate=scripts/check-push-parity.sh partial=roda com TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 e todos os cenários usam --dry-run; o push real (git push para o remoto) e a detecção de squash-merges com fetch real não são exercitados ponta a ponta (--force-with-lease tem gate próprio: scripts/check-push-force-parity.sh) -->

`trackfw push` pushes already-created commits without committing and without opening a PR/MR. It
runs the same branch-name validation and governance gate as `trackfw ship`, but stops after the
push — it never runs `git commit` and never opens a pull request.

```
1. Validates branch name — must match feat|fix|refactor|chore|docs/<slug>
2. Validates governance — REQ + roadmap in wip/ must exist for feat/fix/refactor branches
   (hard gate: not affected by lenient mode or per-rule severity); chore/docs branches skip
   this check, mirroring 'trackfw commit' and 'trackfw ship'
3. Detects pending squash-merges in other branches (advisory only)
4. Pushes to origin (adds -u if no upstream is configured yet)
```

**push never commits and never opens a PR/MR.** It does not accept `-m`. If you have not
committed yet, run `trackfw commit -m "..."` first.

Compositional vocabulary:
```
trackfw commit -m "..."   commits
trackfw push              pushes
trackfw ship -m "..."     commit + push + PR (composition)
```

| Flag | Go | Node.js | Python | Notes |
|------|-----|---------|--------|-------|
| `--dry-run` | yes | yes | yes | Print what would be done without executing write commands |
| `--force-with-lease` | yes | yes | yes | Governed force-push; requires an open PR/MR on the branch (verified via forge CLI) |

**Boundary with `ship` and `commit`:** `push` is not a partial `ship` with fewer steps — it is a
standalone command for the common case where `trackfw commit` was already run and only the push
step remains. `ship` always also commits (requires `-m`); `push` never commits. `commit` never
pushes; `push` never commits.

### `push --force-with-lease` — governed force-push (ML-4B)

<!-- trackfw-contract: gate=scripts/check-push-force-parity.sh -->

`push --force-with-lease` runs `git push --force-with-lease` instead of a plain push — for the
post-rebase case where a plain push is rejected. It only runs when the branch already has an open
PR/MR, verified via the resolved forge CLI (ConfigForge from `trackfw.yaml` `forge:` key, then
RemoteURL, then CI files — push has no `--forge` flag). Three distinct refusal classes:

| Refusal class | Condition | Message fragment |
|---|---|---|
| No forge CLI | gh/glab/az absent from PATH | `requires a forge CLI (gh, glab, or az)` |
| No open PR | PR list returns empty | `has no open pull/merge request. Open the PR/MR first` |
| Unverifiable | forge CLI exits non-zero | `could not verify whether branch` |

**Gate: `scripts/check-push-force-parity.sh`** (registered in the `parity` Make target). Real bare
origin per (scenario, runtime) — 5 scenarios total: no-forge-cli, forge-zero-pr,
forge-unverifiable, forge-pr-open-pushes (remote SHA changes, proved with `Pushed: ` marker),
remote-advanced-lease-mismatch (other clone advances remote, --force-with-lease refuses). Byte
comparison across Go, Node.js, Python via `assert_three_way`. `TRACKFW_DISABLE_EXTERNAL_COMMANDS`
is unset in this gate (push's forge lookup must never be suppressed). Cenário 163 in
`scripts/check-gates-falsify.sh` sabotages an isolated Go copy by inserting `open = true` right
before `if !open {` — `if false {` would leave `open` declared and unused, which does not compile —
and proves the gate detects the P4-push regression.

## `trackfw branch new`

<!-- trackfw-contract: gate=scripts/check-branch-new-parity.sh -->

`trackfw branch new <type>/<slug>` moves the `branch_has_wip_roadmap` governance gate — already
enforced by `trackfw validate` and `trackfw ship` (see "Regra `branch_has_wip_roadmap`" below) —
to **before** branch creation instead of after, for the `feat`/`fix`/`refactor` types. It reuses
the exact same matching logic those two commands already apply; the command never implements a
second version of the rule.

`chore` and `docs` are housekeeping types — already treated as roadmap-exempt, by **branch type**,
by `trackfw ship` and `trackfw commit` (see "Behavioural divergence from `trackfw validate`" above
and `trackfw commit`'s own governed-prefix exemption) — so `trackfw branch new` creates
`chore/<slug>` and `docs/<slug>` branches directly, without consulting `wip/`/`done/` at all. This
is what unblocks branches like `chore/release-x.y.z`, which by definition have no REQ/roadmap of
their own.

**Two independent exemption axes — do not conflate them.** `trackfw ship` grants roadmap
exemption along two separate axes that happen to overlap in practice but are checked differently:
(1) **staged-content doc-only** — every staged file is under `docs/`, `vault/`, or has a `.md`
extension (`allDocOnly`), regardless of branch name; prints `Governance: skipped (doc-only
change)`. (2) **branch-type housekeeping** — the branch itself is `chore/<slug>` or `docs/<slug>`,
regardless of what is staged (a `chore/` branch may stage code); prints `Governance: skipped
(chore/docs branch)`. `trackfw branch new`'s exemption for `chore`/`docs` types is the branch-type
axis (2) — it has no equivalent of axis (1), since there is nothing staged yet at branch-creation
time.

### Command surface

<!-- trackfw-contract: gate=scripts/check-branch-new-parity.sh -->

| Element | Value |
|---|---|
| Invocation | `trackfw branch new <type>/<slug>` |
| `<type>` | One of `feat`, `fix`, `refactor`, `chore`, `docs` |
| `<slug>` | Non-empty; for `feat`/`fix`/`refactor`, matched against roadmaps in `wip/` and `done/` the same way `branch_has_wip_roadmap` does. For `chore`/`docs`, no matching is performed. |
| `--dry-run` | Reports whether the branch would be created or blocked, without executing `git` |
| Exit 0 | `feat`/`fix`/`refactor` with a match, or any `chore`/`docs` spec — branch created (or `--dry-run` reports "would create") |
| Exit non-zero, usage error | Malformed spec (missing `/`, empty slug, invalid `<type>` — error message lists the full vocabulary: `feat, fix, refactor, chore, docs`) |
| Exit non-zero, blocked | `feat`/`fix`/`refactor` only: no matching roadmap in `wip/` nor `done/` — `git checkout -b` is **never** executed |
| Exit = Git's own code | Match found (or `chore`/`docs`), `git checkout -b` ran and failed (e.g. branch already exists → Git's `128`) |

### Decision flow

<!-- trackfw-contract: gate=scripts/check-branch-new-parity.sh -->

```
1. Parse "<type>/<slug>" — <type> must be feat|fix|refactor|chore|docs, <slug> non-empty.
2. For feat|fix|refactor only: normalize the slug and check whether any roadmap filename in
   wip/ or done/ contains it — the same BranchSlugMatchesRoadmap (Go) /
   branchSlugMatchesRoadmap (Node.js) / branch_slug_matches_roadmap (Python) function
   trackfw validate calls for branch_has_wip_roadmap. Not a reimplementation — the same
   function, imported. chore|docs skip this step entirely — matchSlug is never called.
3. No match (feat|fix|refactor only): print the same governance orientation message
   trackfw validate already prints for this rule, exit non-zero, never invoke git.
4. --dry-run with a match (or chore|docs): print "[dry-run] would create branch "<type>/<slug>"
   (git checkout -b <type>/<slug>)", exit 0, never invoke git.
5. Match (or chore|docs), no --dry-run: run `git checkout -b <type>/<slug>` with inherited
   stdio.
```

### Shared matching logic — never duplicated

<!-- trackfw-contract: gap reason=nenhum gate compara a mensagem de `trackfw validate` com a de `trackfw branch new` no mesmo fixture/runtime para provar que são byte-idênticas entre si — cada comando é validado isoladamente entre os 3 runtimes, não um contra o outro -->

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

<!-- trackfw-contract: gate=scripts/check-branch-new-parity.sh -->

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

<!-- trackfw-contract: gate=scripts/check-branch-new-parity.sh,scripts/check-gates-falsify.sh -->

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

## `trackfw branch prune`

<!-- trackfw-contract: gate=scripts/check-branch-prune-parity.sh -->

`trackfw branch prune [--apply]` automates the "one active branch at a time" check documented in
`CLAUDE.md` §1 ("Uma branch ativa por vez")
(`docs/req/REQ-2026-08-18-trackfw-branch-prune-apaga-branch-local-ja-integrada-com-deteccao-correta-de-squash-merge.md`).
It decides whether each local branch is safe to delete relative to `origin/main` — never a forge —
and reports the decision for **every** local branch, always, with a reason. It does not remove
human judgment from every case: a branch whose only remaining divergence is doc/config files is
flagged for manual review, never deleted automatically (see "The review_doc_config category"
below).

**`--dry-run` is the default.** Without `--apply`, nothing is ever deleted, even a branch decided
as clearly integrated — the command only reports. `--apply` is the explicit opt-in required to
actually run `git branch -d`/`-D` (see "Deletion: `-d` before `-D`" below).

### `git fetch origin --prune` — best-effort, non-blocking, always warned on failure

<!-- trackfw-contract: gate=scripts/check-branch-prune-parity.sh -->

Per `CLAUDE.md` §1 step 1, the command runs `git fetch origin --prune` before evaluating anything.
Unlike `trackfw ship`'s squash-merge check (`ship.go`), which **skips its check entirely** when
fetch fails, `branch prune` **keeps evaluating** against whatever `origin/main` ref is already
resolvable locally — offline is a legitimate use case, and skipping evaluation entirely would
regress AC6 (offline must still report, just never delete). Failure prints a warning naming the
fetch failure and only that; it never aborts the command.

A stale `origin/main` only ever makes the result **more conservative** — never less: it can *miss*
a branch that was in fact integrated since the last fetch (reporting it `keep` when a fresh fetch
would show it `delete`-worthy), but it can never report a branch as deletable that a fresh fetch
would show as pending. This holds because staleness only means `diverg` is computed against an
*older* (but still valid) snapshot of `origin/main`'s content — a merge landing later can only
ever remove divergence that a stale ref still sees, never introduce divergence that a fresh ref
would not have shown. Proven with a real-git fixture in all three CLIs (`TestRunBranchPrune_RealGitRepo_StaleOriginMain_IsConservativeNotWrong`
and Node/Python equivalents): a branch genuinely merged upstream by someone else is reported
`pending_work` (kept) while the local clone's `origin/main` is stale after a broken remote URL,
and becomes `content_identical` (deletable) the moment a real fetch succeeds — same branch, same
local commits, only the freshness of `origin/main` changed.

### Why not `git branch -d`, and why not a naive `git diff`

<!-- trackfw-contract: gate=scripts/check-branch-prune-parity.sh -->


`git branch -d` refuses by **ancestry** — with squash-merge as the project's merge strategy,
ancestry never exists, so `-d` refuses *every* integrated branch, teaching users to reach for
`-D`, which deletes without checking anything. A naive bidirectional
`git diff origin/main <branch> --stat` is only correct when `<branch>` is up to date with `main`;
on a **stale** branch it reflects how far `main` has moved, not whether the branch's own work
landed — the exact false positive `detectPendingSquashMerges` (`trackfw ship`, `ship.go:564`)
had before this REQ, which acknowledged a merged PR (#181) as having "unmerged changes" only
because a later PR (#182) had since advanced `main`. As of ML-2A, `detectPendingSquashMerges`
itself calls `evaluateBranchIntegration` (see below) instead of that naive diff — the false
positive is closed.

### The touched-files heuristic — the single shared decision function

<!-- trackfw-contract: gate=scripts/check-branch-prune-parity.sh partial=as linhas no_own_work e no_merge_base da tabela de decisão não são exercitadas cross-CLI pelo gate; só content_identical, review_doc_config e pending_work o são (cenários a, b, e, f) -->


```
mb      = git merge-base origin/main <branch>
touched = git diff --name-only -z mb <branch>                      (what the branch touched)
diverg  = git diff --name-only -z origin/main <branch> -- touched  (what still differs there)
```

| `touched` | `diverg` | Decision | Deletable |
|---|---|---|---|
| empty | — | `no_own_work` | yes — the `git branch -d` ancestry false negative |
| non-empty | empty | `content_identical` | yes — the naive-diff stale-but-integrated false positive |
| non-empty | proper subset of `touched`, all doc/config | `review_doc_config` | **no** — flagged for manual review (see below) |
| non-empty | equal to `touched` (all doc/config), or any non-doc/config file anywhere | `pending_work` | no — named in the report |
| (merge-base fails) | — | `no_merge_base` | no — refuses, unrelated history or bad ref |

### The `review_doc_config` category — flagged, never auto-deleted, requires a PROPER subset

<!-- trackfw-contract: gate=scripts/check-branch-prune-parity.sh -->


`CLAUDE.md` §1's own manual procedure treats a divergence limited to doc/config files (its
worked example: only `CLAUDE.md` diverges) as "housekeeping, apagar" — but that step assumes a
**human already reading the diff** before acting. A destructive command has no human in the loop
by construction, so this command deliberately narrows that step: a branch whose `diverg` is
**every file doc/config** (never mixed with even one non-doc/config file) is classified
`review_doc_config`, reported with action `review` (not `delete`, not `keep`'s plain wording), and
is **never** offered for deletion by `--apply` — regardless of how confident the classification is.

**`review_doc_config` additionally requires `diverg` to be a PROPER subset of `touched`**
(`len(diverg) < len(touched)`) — not merely "all doc/config". `diverg` is always a subset of
`touched` by construction (the second `git diff` is scoped `-- touched`), so a proper subset means
at least one file the branch touched *did* land in `main` — genuine partial integration, with
doc/config residue left over. When `diverg == touched`, **nothing** from the branch reached `main`
at all; that is `pending_work`, regardless of file type. This was corrected in ML-1C after an
audit found the original rule (any diverging file set that happened to be all doc/config)
misclassified a branch with brand-new, never-merged documentation as `review_doc_config` —
"probable housekeeping, confirm and delete manually" is materially wrong advice about real,
unmerged work, even though the branch itself was never at risk of being auto-deleted (the
failure-closed guarantee held; the *advice* was wrong).

"Doc/config" is decided by `isDocOrConfigPath` (Go), `isDocOrConfigPath` (Node.js),
`is_doc_or_config_path` (Python): the existing doc-path check (`docs/`, `vault/`, or a `.md`
extension) plus a conservative, best-effort list of non-runtime config extensions
(`.yaml`, `.yml`, `.json`, `.toml`, `.ini`, `.cfg`) and filenames (`.gitignore`,
`.gitattributes`, `.editorconfig`, `trackfw.yaml`, `LICENSE`). Misclassification here can never
cause a deletion — a file wrongly counted as doc/config only changes whether a *kept* branch is
reported as `review_doc_config` or `pending_work`; a single non-doc/config file anywhere in
`diverg`, or `diverg` equal to `touched`, keeps the branch in `pending_work`.

The report groups these branches into a summary line separate from the `--apply`/dry-run delete
summary: `N branch(es) need manual review (only doc/config diverges, never auto-deleted): <names>`.

Both `diff` calls use `-z` (NUL-separated, unquoted paths) — without it, a filename with a space
or non-ASCII byte would be mis-split by the pathspec on the second call, silently narrowing
`diverg` to nothing and deleting a branch with real pending work in that file.

This decision function — `evaluateBranchIntegration` (Go), `evaluateBranchIntegration` (Node.js),
`evaluate_branch_integration` (Python) — is the **single shared implementation**. `trackfw ship`'s
`detectPendingSquashMerges` (`ship.go`, `ship/runner.js`, `ship/runner.py`) calls it too (ML-2A,
REQ-2026-08-18) instead of maintaining its own bidirectional diff: for each remote candidate
returned by `git branch -r --no-merged origin/main`, it warns *only* when the decision is
`pending_work` — every other decision (`no_own_work`, `content_identical`, `review_doc_config`,
`no_merge_base`, `eval_error`) stays silent, the same posture the old naive check had on error
(skip, no warning). This is advisory-only in `ship` (never blocks the commit/push), unlike `branch
prune`, which is destructive; the two commands share the decision function but not its
consequences. Node.js imports `evaluateBranchIntegration`/`DECISION` from `branch/prune.js`;
Python late-imports `evaluate_branch_integration`/`BRANCH_PRUNE_DECISION_PENDING_WORK` from
`trackfw.commands.branch` inside `_detect_pending_squash_merges` (mirroring `commands/branch.py`'s
own existing late import of `ship/runner.py`, avoiding an import-time cycle between the two
modules); Go needs no import — both functions live in the same `commands` package.

### Always-kept branches — never evaluated for deletion, never candidates

<!-- trackfw-contract: gate=scripts/check-branch-prune-parity.sh partial=exclusão de branch com worktree checked-out em outro diretório não é exercitada cross-CLI pelo gate (só a branch atual via HEAD é, cenário c) -->


- **`main`** — the default branch itself. Evaluating it against `origin/main` would trivially
  report `no_own_work` (its own merge-base against itself is its own tip) and offer to delete the
  branch the user is meant to keep. Excluded by name before the heuristic ever runs — the
  highest-severity failure mode this command guards against.
- **The current branch** — via `git symbolic-ref --quiet --short HEAD` (empty on detached HEAD).
- **Any branch checked out in another worktree** — via `git worktree list --porcelain`, parsing
  the `branch refs/heads/<name>` line (not the human-readable format).

With `--apply`, current-branch and worktree status are **re-checked immediately before each
delete**, not just during the report phase — belt-and-suspenders against the branch changing
state mid-run.

### Deletion: `-d` before `-D`

<!-- trackfw-contract: gate=scripts/check-branch-prune-parity.sh partial=o fallback -d→-D só é exercitado cross-CLI no sentido squash (-d falha, cai para -D); o caminho onde -d sozinho basta (merge não-squash com ancestria) só tem prova por teste unitário isolado por runtime, não por comparação cross-CLI -->


`defaultDeleteBranch` (Go), `defaultDeleteBranch` (Node.js), `_default_delete_branch` (Python) try
`git branch -d <name>` first. When the branch also happens to have fast-forward ancestry with
`main` (a plain merge, not a squash), `-d` succeeds on its own — confirming the integration via
git's own independent ancestry check too, at no extra cost. It falls back to `git branch -D
<name>` only when `-d` refuses, the **expected** outcome for squash-merged branches (no ancestry
by construction, per the "Why not `git branch -d`" section above). All safety already lives in
`evaluateBranchIntegration` and the current-branch/worktree re-check immediately before this call
— which flag ultimately performs the deletion carries no additional safety meaning. Both codepaths
are proven with a real-git fixture in all three CLIs (a plain-merge branch deleted via `-d` alone;
a squash-merged branch where `-d` is first confirmed to fail, then `_default_delete_branch` falls
back to `-D` and succeeds).

### Offline / no remote — fails closed

<!-- trackfw-contract: gate=scripts/check-branch-prune-parity.sh partial=cenário d prova exit 1 e zero deleções quando origin/main não é resolvível, mas o gate não afirma o texto pinado "branch prune: origin/main not resolvable" — grep por "not resolvable" no script não retorna nada -->


The only ref this command consults is `origin/main`, checked once via
`git rev-parse --verify -q origin/main` **after** the best-effort fetch above. If it cannot be
resolved at all (no remote configured, or `origin/main` was never fetched even before this run),
the **whole command** refuses and deletes nothing — no fallback to a local `main`. The
human-readable reason goes to stdout; a bare `branch prune: origin/main not resolvable` goes to
stderr (mirroring `trackfw branch new`'s stdout/stderr split), exit 1.

### Command surface

<!-- trackfw-contract: gate=scripts/check-branch-prune-parity.sh -->


| Element | Value |
|---|---|
| Invocation | `trackfw branch prune [--apply]` |
| `--apply` | Actually delete branches decided as integrated. Default: report only, delete nothing (equivalent to an implicit `--dry-run`) |
| Exit 0 | Ran to completion — includes the case where `--apply` deleted zero branches |
| Exit 1 | `origin/main` unresolvable (offline with no prior fetch, no remote, never fetched), or local branch listing failed |
| Deletion | `git branch -d`, falling back to `-D` only when `-d` refuses (see above); only for branches decided `no_own_work` or `content_identical`, and never the current/worktree/default branch, and never `review_doc_config` |

### Parity gate

<!-- trackfw-contract: gate=scripts/check-branch-prune-parity.sh -->


`scripts/check-branch-prune-parity.sh` builds a **real** local bare repository as `origin` (no
mock of `git` — see `vault/notes/` precedent, Cenário 50 in `check-gates-falsify.sh`) and asserts
byte-identical stdout/stderr/exit code across all three runtimes for:

1. **Dry-run (default)** — reports two integrated branches (one squash-merged same-session, one
   stale-but-integrated after `main` advanced further — the AC2 discriminant) as deletable and a
   genuinely pending branch as kept, but deletes **nothing**; branch count unchanged.
2. **`--apply`** — deletes exactly the two integrated branches, keeps the pending branch and
   `main`.
3. **`--apply` with the integrated branch checked out as current** — never deletes it, reports
   the current-branch reason instead.
4. **Offline** (fresh repo, no remote) — refuses everything, exit 1, deletes nothing.
5. **Review, doc/config-only** — a branch whose only divergence is `README.md` is reported action
   `review` (never `delete`), the manual-review summary line is present, and dry-run deletes
   nothing.
6. **Stale `origin/main`, conservative** — a broken remote URL makes `git fetch origin --prune`
   fail inside the command; a branch genuinely merged upstream by an independent clone (invisible
   to the stale local `origin/main`) is reported `keep`, never `delete`; the fetch-failure warning
   is present.

Wired into `make quality` via the `parity` target.

## `trackfw barrier`

<!-- trackfw-contract: gate=scripts/check-barrier.sh -->


`trackfw barrier <roadmap> --wave <n>` is the deterministic core of the wave-release barrier.
It is **stack-agnostic**: it never assumes a build tool, a test runner or a parity rule. Every
executable check comes from the roadmap itself. The agent-orchestration layer (specialist
inspections for code quality and security) lives in the `/trackfw:barrier` slash command, never
in the binary.

### Command surface

<!-- trackfw-contract: gate=scripts/check-barrier.sh -->


| Element | Value |
|---|---|
| Invocation | `trackfw barrier <roadmap> --wave <n>` |
| `<roadmap>` | Basename with or without `.md`, resolved against `wip/` then `done/` under `roadmap_dir` (both `flat` and `by_agent` layouts) |
| `--wave` | Wave **label**, required. Grammar `<integer>[-<suffix>]` — see "Wave label grammar" below. `0`, `2`, `2-bis`, `2-hotfix` are valid; the integer part must be ≥ 0. |
| `--json` | Emit the result document instead of the text report |
| `--trust-local-gates` | Trust the local roadmap content for gate execution without comparing to `origin/main`. Used by the `/trackfw:barrier` slash command. See "Trust and `--trust-local-gates`" below. |
| Exit 0 | `status: "passed"` |
| Exit 1 | `status: "blocked"` — at least one check failed or untrusted (`not_evaluated`) |
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

<!-- trackfw-contract: gate=scripts/check-barrier.sh -->


A wave label is `<integer>[-<suffix>]`:

| Element | Rule |
|---|---|
| Integer part | One or more digits, value ≥ 0. Required. |
| Suffix | Optional. A single `-` followed by `[a-z0-9]+` — lowercase only. |

Valid: `0`, `1`, `2`, `2-bis`, `2-hotfix`, `10-a2`. `0` is the Wave 0 threat-model convention
(ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push,
ML-1A). Invalid: `X`, `2-BIS` (uppercase), `-bis` (no integer), `2-` (empty suffix), `2-bis-ter` (two
suffixes). Negative integers (`-1`) are already excluded by the regex, which has no sign — the
`>= 0` bound only rejects a malformed grammar match, it never has a negative integer to reject in
practice.

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

<!-- trackfw-contract: gate=scripts/check-barrier.sh -->


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

<!-- trackfw-contract: gap reason=a seção fixa fato falsificável hoje (Go tem compareWaveLabels coberto por teste unitário; Node.js e Python declinaram) e uma exigência positiva ("não fixe esta assimetria em nenhuma direção"), mas nenhum gate cross-CLI compara os três runtimes nesse ponto; regra de desempate da Emenda 1 aplicada — autodeclaração de "sem superfície hoje" não prevalece sobre fato falsificável presente -->


No runtime currently lists or compares waves; `--wave` resolution is exact-match only. The ordering
rule above stays **normative** — it applies the moment a listing surface appears — but implementing a
comparator is **optional** until then. Go has `compareWaveLabels` covered by unit tests, which proves
the rule is implementable; Node.js and Python correctly declined to add one rather than ship dead
code. Do not "fix" this asymmetry in either direction: adding dead comparators to two runtimes is not
parity, and deleting Go's loses the tested proof.

### States

<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=os estados pending e running só aparecem mid-run ou em documento abortado; os cenários do gate comparam apenas o documento JSON final (passed/blocked), nunca um snapshot mid-run; not_evaluated cobre o caso do gate não confiável e os cenários 14-17 do check-barrier.sh verificam os casos cross-CLI -->


| State | Meaning |
|---|---|
| `pending` | Check declared but not yet evaluated (only ever appears mid-run, and in `--json` when a preceding check aborted the run) |
| `running` | Check currently executing (text output only; never present in a final JSON document) |
| `passed` | Check evaluated and green |
| `blocked` | Check evaluated and red |
| `not_evaluated` | Check skipped because the roadmap is not trusted for gate execution (see "Trust and `--trust-local-gates`" below). Never `passed`. |

The wave-level `status` is `passed` only when **every** check is `passed`; otherwise `blocked`.
`not_evaluated` is not `passed` — a wave with an untrusted `gates` check reports `status: "blocked"`
and exits 1, so the architecture does not allow an unevaluated gate to release a wave silently.

### Trust and `--trust-local-gates`

<!-- trackfw-contract: gate=scripts/check-barrier.sh -->

`trackfw barrier` determines whether a roadmap's gates can be trusted before executing them. This
is the defense against the PR-vector: a contributor who opens a PR containing a hostile roadmap
can cause the maintainer to execute arbitrary shell commands by running `trackfw barrier` during
review. Decision: ADR-2026-08-23.

#### Discriminant (AC4, AC11)

<!-- trackfw-contract: gap reason=o discriminante origin/main é documentação de decisão de desenho (ML-2A); nenhum gate cross-CLI exercita especificamente o discriminante — isso é responsabilidade dos testes unitários Go (TestRoadmapTrustForGates_*) que não são cross-CLI -->

The discriminant is **git**: a roadmap whose content **differs from `origin/main`**, or that **does
not exist in `origin/main`**, is untrusted for gate execution. The comparison is byte-for-byte.

`HEAD` is NOT the discriminant. A roadmap added by a PR contributor is already committed on the
PR branch; a HEAD-comparison would mark it as trusted, closing usability without closing the
PR-vector.

#### `--trust-local-gates` flag (AC12, AC15)

<!-- trackfw-contract: gate=scripts/check-barrier.sh -->

The flag `--trust-local-gates` bypasses the trust check and executes gates from local content.
It is injected by the `/trackfw:barrier` slash command and is intended for the dominant flow:
the project architect evaluating a wave in their own repository during implementation, where the
roadmap is legitimately modified locally and not yet committed to `origin/main`.

The CLI direct (`trackfw barrier <roadmap> --wave <n>`) does NOT include the flag by default.
This is intentional: the PR-review flow — a maintainer running the barrier on a contributor's
roadmap — uses the direct CLI and gets protection without having to remember to ask for it.

**Rationale for this separation (AC5):** a flag required on every invocation becomes habit. A
guard that requires conscious opt-in for the dangerous case stays effective. The dominant
(safe) flow is frictionless; the rare (risky) flow requires one more word.

#### Fail-open cases (declared residuals)

<!-- trackfw-contract: gap reason=os casos fail-open são decisões de desenho declaradas (ML-2A); não há gate que simule ausência de remote ou falha de git invocation cross-CLI — seriam testes de integração dependentes de ambiente, não cross-CLI -->

The trust check is **fail-open** in the following cases. Gates execute as if trusted:

| Case | Reason | Residual |
|---|---|---|
| Roadmap is not inside a git repository | Cannot determine trust; test fixtures run in temp dirs | Declared in AC13 context: environment without git is considered safe enough |
| `origin/main` reference is not resolvable (no remote configured, not fetched) | Ambiguous — could be a fresh clone | Maintainer should fetch before running barrier on PRs |
| Any git invocation fails for reasons other than "path absent from origin/main" | Conservative: don't break normal usage for infrastructure issues | Log absence is the only way to detect this |

Gates are NOT fail-open when the path specifically **does not exist in `origin/main`** (exit 128,
"does not exist in" message from git). That case is the PR-vector: the roadmap was added by the
PR contributor and is not yet merged.

#### Pinned failure strings for `not_evaluated` (AC3, AC6, AC7)

<!-- trackfw-contract: gate=scripts/check-barrier.sh -->

When the trust check refuses gate execution, the `gates` check gets `status: "not_evaluated"` and
exactly one entry in `failures`. The `commands` array is still populated from `parseGates` so the
operator can see what would have been executed. The two pinned strings are:

```
gates not evaluated: roadmap is not committed in origin/main — pass --trust-local-gates to evaluate local gates
gates not evaluated: roadmap content differs from origin/main — pass --trust-local-gates to evaluate local gates
```

All three runtimes must emit these byte-for-byte. The text report symbol for `not_evaluated` is
`✗` (same as `blocked`) — only one symbol, the status string carries the distinction.

#### AC13 — The slash command lives in the repository (declared residual)

<!-- trackfw-contract: gap reason=AC13 é um residual declarado de segurança (ML-2A, ADR-2026-08-23 §4.1 e §4.2); não existe gate que valide essa mitigação — a proteção é code review do diff, não um script -->

The `/trackfw:barrier` slash command (`.claude/commands/trackfw/barrier.md`) is version-controlled
in the same repository. A hostile PR contributor can **edit the slash command** to include
`--trust-local-gates` and recover gate execution via the slash command interface.

This is a **declared and accepted residual**. The CLI cannot verify the provenance of the
`--trust-local-gates` flag: a flag from a hostile slash command, a hostile `Makefile`, a hostile
CI step, and the maintainer's own conscious invocation are all indistinguishable in `argv`.
Checking `.claude/commands/trackfw/barrier.md` against `origin/main` doesn't help — the CLI never
reads that file; the agent reads and executes it.

**Sibling surfaces in the same residual class** (named by the ML-4A barrier review, 2026-08-23):
version-controlled Claude hook scripts wired from the project's `.claude/settings.json`, project
`Makefile` targets, and any CI step. All of them run maintainer-side from a PR checkout without
passing through the CLI, and none is verifiable by the CLI for the same reason as the slash command.
The hook surface is **broader** than the gate one: it does not require running `trackfw barrier` at
all.

**The protection against this residual is the same as the protection against a hostile
`Makefile` or CI workflow in a PR: the maintainer reads the diff.** Editing the slash command
in a PR is an observable change in the diff, under the same code-review boundary that governs
all other infrastructure files. This residual is already named in ADR-2026-08-23 §4.1 and §4.2.

**Consequence for the `/trackfw:barrier` slash command:** the command includes a warning that
`--trust-local-gates` must NOT be passed when reviewing a third-party PR's roadmap. The
maintainer who wants to consciously evaluate a PR's gates can pass the flag explicitly to the
CLI, knowing what they accept.

### Roadmap parsing rules (string-level — no heuristics)

<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=regras 3, 4 e 5 são exercitadas cross-CLI pelos cenários isolated-check; a regra 6 (fence não-terminado, ML cujo corpo não pode ser delimitado como usage error nomeado) não tem cenário — grep por "unterminated"/"fence"/"cannot be delimited" no gate não retorna nada -->


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

<!-- trackfw-contract: gate=scripts/check-barrier.sh -->


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

<!-- trackfw-contract: gate=scripts/check-barrier.sh partial=o cenário 6 usa uma fixture 100% verde (todos os checks passed) e prova só que os 3 runtimes concordam ENTRE SI byte a byte; não afirma os textos pinados de evidence/failures ("<ML-id>: <n> criteria met", "<command>: exit 0") contra um valor esperado — grep por "criteria met"/"exit 0'" no script não retorna asserção correspondente, e nenhum cenário deste gate produz um failures[] não-vazio para comparar -->


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

<!-- trackfw-contract: gap reason=os quatro casos de borda (bloco de aceite vazio, wave sem MLs, wave sem título, processo morto por sinal) só têm cobertura em testes unitários por runtime (barrier_test.go/barrier.js/barrier.py); nenhum cenário de check-barrier.sh os exercita cross-CLI -->


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

<!-- trackfw-contract: gate=scripts/check-barrier.sh -->


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

<!-- trackfw-contract: gate=scripts/check-update-parity.sh -->


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

<!-- trackfw-contract: gap reason=nenhum gate cross-CLI exercita o efeito dos 12 campos Update/Sync (hooks, ci, backend, frontend, pkg_manager, agent_conventions, linear_*, jira_*) na saída gerada; há só testes unitários por runtime e a prova de falsificação do carregador unificado em check-gates-falsify.sh, que garante que o loader é chamado, não que os três runtimes reagem identicamente a um dado valor de campo -->


Since `REQ-2026-08-02-unificar-a-leitura-do-trackfw-yaml-em-um-unico-carregador-nos-tres-clis`, all
twelve fields below are read exclusively by the shared config loader (Go `config.Load`, Node.js
`loadConfig`, Python `load_config`) into two typed namespaces — `Update` (6 fields) and `Sync` (6
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
| `agent_conventions` | `Update` | `""` | Free-text, team-declared project conventions (test framework, architecture pattern, API style, linter, etc.) — never inferred automatically. When non-empty, `trackfwRulesBlock()`'s injection into every AI agent file (CLAUDE.md, AGENTS.md, GEMINI.md, etc.) gains a "### Project Conventions" section with this text verbatim; absent/empty produces byte-identical output to before this field existed. `trackfw discover` may print a best-effort test-framework suggestion for the user to add here manually — it never writes this key itself |
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

<!-- trackfw-contract: gate=internal/generators/credential_guard_test.go partial=TestCredentialGuardScript_ParityAcrossStacks compara byte a byte o script GERADO entre Go, Node e Python (que embute a leitura de mode via grep/sed), mas nenhum gate cross-CLI testa o fallback silencioso de um valor mode não reconhecido para "warn" nem a divergência real de comportamento warn vs block em runtime -->


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

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh partial=a estrutura do wiring (matcher/evento/schema por CLI) é comparada byte a byte entre os 3 runtimes; a própria seção declara cobertura de teste de sabotagem end-to-end em só 3 de 6 CLIs (Claude Code, Cursor, Kiro) — Codex, Gemini CLI e GitHub Copilot ficaram sem esse teste específico -->


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

<!-- trackfw-contract: gate=scripts/check-update-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-update-parity.sh -->


| Flag | Applies to | Behaviour |
|---|---|---|
| `--dry-run` | both | Compute and report states without writing anything |
| `--json` | both | Emit the result document instead of the text report |
| `--targets` | both | Comma-separated subset of target ids; unknown id is a usage error |
| `--install-missing` | both | Allow `missing` targets to be installed instead of merely reported |

### JSON document

<!-- trackfw-contract: gate=scripts/check-update-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-update-parity.sh partial=o gate garante que os 3 runtimes concordam entre si sobre o array de targets (comparação estrutural go-vs-node/go-vs-py), mas nenhum cenário afirma independentemente que a contagem/ordem bate com os 5 ids documentados — os 3 runtimes concordando com uma lista errada ainda passaria (mesmo padrão do achado "OpenCode agent representation" do lote 1) -->


`trackfw update` declares this fixed sequence of 5 ids, in this exact order:
`agent-rules`, `agent-hooks`, `codex-project-agents`, `validate-script`, `claude-commands`.

All three runtimes declare all five. A runtime that cannot manage a target still declares it and
reports its honest state — silently shortening the list makes the JSON incomparable across runtimes.

#### Declared residual — `codex-project-agents` is structurally outside the hash-comparison guarantee

<!-- trackfw-contract: gap reason=residual declarado (Wave 0 do ROADMAP-2026-08-27, Gap D): codex-project-agents bypassa runFileTarget e sempre reporta updated, logo esta fora do alcance do sandbox de dry-run por inclusao; nao ha gate porque nao ha comportamento a fixar — a secao documenta a AUSENCIA de garantia, e fixa-la exigiria reescrever o target, o que e escopo alheio a esta REQ -->

`codex-project-agents` does not use `runFileTarget` / `_run_file_target`. It calls
`codexProjectAgentsApply` (Go), `codexProjectAgentsTarget` (Node.js) or `_codex_project_agents_target`
(Python) directly. These functions resolve plans from a runtime catalog and return `updated`
unconditionally when any write occurs — they do NOT compute before/after content hashes. As a result:

- `codex-project-agents` reports `updated` even when the catalog content is byte-identical to what
  is already installed (false positive).
- `codex-project-agents` verification is via `manager.Inspect`, not content-hash diffing.
- The target's output paths are determined at runtime from the catalog, not from a static `relPaths`
  declaration, so they cannot be seeded into the dry-run sandbox — the target operates against the
  real project root even in dry-run mode (protected by the `if (!dryRun) manager.update()` guard in
  Node.js and the `opts.InstallMissing` guard in Go/Python, which limit the blast radius but do not
  provide full dry-run isolation).

This is a **declared and accepted residual** (Gap D, Wave 0 threat model,
`docs/seguranca/2026-08-27-modelo-de-ameaca-do-sandbox-por-inclusao.md` §R2). Closing it would
require either (a) a static relPaths declaration for every catalog artifact (impractical — the
catalog is runtime-resolved) or (b) a separate inspection-based dry-run path for this target.
Neither is in scope for the inclusion-sandbox ML (ML-1A of
`ROADMAP-2026-08-27-sandbox-do-update-dry-run-por-lista-de-inclusao-dos-destinos-declarados.md`).

### `--dry-run` sandbox — inclusion-based copy contract

<!-- trackfw-contract: gate=scripts/check-update-parity.sh -->

`--dry-run` creates a scratch directory seeded with **only the paths declared in each target's
`relPaths` list** (plus detection-signal seeds, see below). It does **not** walk the whole project
tree. This closes the class of failures where a broken symlink outside the declared set (e.g.
`.venv/bin/python → python3.13` deleted by Homebrew) caused `--dry-run` to abort.

**Invariant:** any file or directory outside the declared `relPaths` union has zero effect on
dry-run state. Broken symlinks outside the set are irrelevant by construction; broken symlinks
**inside** the set are treated as `missing` (hash returns null), not as errors.

**Detection-signal seeds (not counted in hash comparison):**

| Seed | Purpose |
|---|---|
| `trackfw.yaml` | `ReadAgentConventions` reads `agent_conventions` from it; absent → CLAUDE.md hash diverges between dry-run and real run (Gap E) |
| `.github/copilot-instructions.md` | `InjectHooksDetected` checks this to decide whether to write Copilot hooks; absent → hooks silently omitted (Gap C) |
| `.windsurfrules` | detection signal for Windsurf presence |
| `.amazonq/rules.md` | detection signal for Amazon Q presence |

**Gate coverage (Scenarios 9–14 of `check-update-parity.sh`):**

| Scenario | What is proved |
|---|---|
| 9 | Dangling symlink **outside** declared set — dry-run exits 0, state unaffected |
| 10 | Dangling symlink **inside** declared set — dry-run exits 0, target reported as `missing` |
| 11 (Gap E) | `agent_conventions` in `trackfw.yaml` — dry-run and real-run agree on CLAUDE.md state |
| 12 (Gap C) | `.github/copilot-instructions.md` present — Copilot hooks written in both dry-run and real run |
| 13 (Gap A/B) | `.windsurf/hooks.json` and `.amazonq/cli-agents/q_cli_default.json` appear in output paths |
| 14 (R-novo-1) | `.claude/commands/trackfw` already correct — dry-run and real-run both report `skipped` in all 3 runtimes |

**Falsification (Scenarios 175–176 of `check-gates-falsify.sh`):**

- **Direction A** — `add("trackfw.yaml")` removed from `buildSandboxInclusion` → dry-run reports
  `skipped` where real run reports `updated` (Gap E regression). Detected by Scenario 11 FAIL line
  `sandbox/gap-e/dry-vs-real/go`.
- **Direction B** — `copyProjectTree` body reverted to `filepath.WalkDir + os.ReadFile` →
  dry-run aborts on broken symlink outside declared set (CMDB regression). Detected by Scenario 9
  FAIL line `sandbox/dangling-outside-set/exit-zero/go`.

#### Declared residual — declared path or file within a declared directory is unreadable or a special file

<!-- trackfw-contract: gap reason=o comportamento de abort (em vez de failed-por-target) ao encontrar arquivo ilegivel (chmod 000, socket, fifo) em um caminho declarado ou dentro de um diretório declarado é semanticamente defensável (o arquivo é necessário para o target) e consistente nos 3 CLIs; não há comportamento a fixar, logo não há gate; documentado como R-novo-2 da barreira de 2026-08-27 -->

When a file in the declared `relPaths` set (or within a declared directory, after the R-novo-1 fix
that made `copyPath` recurse) is unreadable (e.g. `chmod 000`) or is a special file (socket, FIFO),
`copyPath` / `_copy_path` propagates the I/O error up through `copyProjectTree`, causing the
entire dry-run to abort rather than reporting `failed` for the individual target:

```
Error: preparing dry-run sandbox: sandbox: copying CLAUDE.md: open /tmp/.../CLAUDE.md: permission denied
```

This behaviour is the same in all three runtimes (Go, Node.js, Python) and is **semantically
correct**: the file is needed by the target, so the sandbox cannot be built without it. Treating it
as `failed` per-target would require the sandbox to proceed with a missing or substitute hash for a
file that may be central to the before/after comparison.

**Declared and accepted residual** (R-novo-2, barrier review 2026-08-27,
`docs/seguranca/2026-08-27-barreira-do-sandbox-por-inclusao.md` §3). The surface widened slightly
after the R-novo-1 fix: before, only a top-level declared path could trigger the abort; now, any
unreadable or special file **inside** a declared directory (e.g. a `chmod 000` file inside
`.claude/commands/trackfw`) also triggers it — matching Node.js behaviour, which was already
recursing and already had this surface.

### `updated` vs `skipped` — the discriminator is content, not action

<!-- trackfw-contract: gate=scripts/check-update-parity.sh -->


`updated` means the target's content **actually changed**. A target that already matches the current
template is `skipped`, even if the implementation rewrote the bytes. Deciding by "did I call write()"
instead of "did the content change" makes an idempotent re-run report `updated` in one runtime and
`skipped` in another for the same input — measured divergence between Go and Node.js in the first
Wave 6 round.

### Declared harness targets — pinned list

<!-- trackfw-contract: gate=scripts/check-update-parity.sh partial=mesmo padrão da lista de targets do projeto — os 3 runtimes concordando entre si sobre os 33 ids não prova que a lista bate com os 33 documentados; nenhum cenário afirma a contagem/ordem exata de forma independente -->


The harness target list is **not** derived at runtime; it is this fixed sequence of 33 ids, in this
exact order: `claude-skill`, `claude-credential-guard` (global-scope credential-guard wiring for
Claude Code — `ROADMAP-2026-08-06-hooks-de-credential-guard-como-escopo-global-cross-project-via-trackfw-update-harness.md`,
ML-2A), `claude-git-branch-guard` (global-scope git-branch-guard wiring for Claude Code —
`ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md`,
Wave 2/ML-2A — merges into the SAME `~/.claude/settings.json` hooks.PreToolUse/PostToolUse[matcher:
"Bash"] arrays claude-credential-guard already writes into, as a second, distinct command entry),
`claude-agents`, `claude-skills`, `codex-credential-guard` (same wave, ML-2B — global-scope
credential-guard wiring for Codex CLI, `~/.codex/hooks.json`), `codex-git-branch-guard` (same
`~/.codex/hooks.json` file), `codex-agents`, `codex-skills`,
`gemini-credential-guard` (same wave, ML-2C — global-scope credential-guard wiring for Gemini CLI,
`~/.gemini/settings.json`, `BeforeTool`/`AfterTool[matcher:"run_shell_command"]`),
`gemini-git-branch-guard` (same `~/.gemini/settings.json` file), `gemini-agents`,
`gemini-skills`, `antigravity-agents`, `antigravity-skills`, `cursor-credential-guard` (same wave,
ML-2D — global-scope credential-guard wiring for Cursor, `~/.cursor/hooks.json`,
`hooks.beforeShellExecution`/`hooks.afterShellExecution`, each entry a flat `{"command":"..."}`
object — no per-entry matcher, unlike Claude/Codex/Gemini's nested `{matcher,hooks:[{type,command}]}`
shape), `cursor-git-branch-guard` (same `~/.cursor/hooks.json` file), `cursor-agents`, `cursor-skills`,
`copilot-credential-guard` (same wave, ML-2E — global-scope
credential-guard wiring for GitHub Copilot, `~/.copilot/settings.json`,
`hooks.preToolUse`/`hooks.postToolUse[matcher:"bash"]` — see "GitHub Copilot global-scope wiring
(ML-2E)" below), `copilot-git-branch-guard` (same `~/.copilot/settings.json` file), `copilot-agents`,
`copilot-skills`, `windsurf-agents`, `windsurf-skills`,
`amazonq-agents`, `amazonq-skills`, `opencode-agents`, `opencode-skills`, `kiro-credential-guard`
(same wave, ML-2F — global-scope credential-guard wiring for Kiro, a DEDICATED file at
`~/.kiro/hooks/trackfw-credential-guard.json` — see "Kiro global-scope wiring (ML-2F)" below),
`kiro-git-branch-guard` (ROADMAP-2026-08-17 Wave 2/ML-2A — a SECOND, SEPARATE dedicated file,
`~/.kiro/hooks/trackfw-git-branch-guard.json`, never the same file as `kiro-credential-guard`: that
writer rewrites its document wholesale every run, never merges, so two wholesale writers sharing one
file would make both targets flap between each other's desired state forever — see "Kiro global-scope
git-branch-guard wiring (ROADMAP-2026-08-17 Wave 2/ML-2A)" below),
`kiro-agents`, `kiro-skills`. Each `<tool>-credential-guard`/`<tool>-git-branch-guard` id (where it
exists) is always positioned immediately BEFORE that tool's own `<tool>-agents`/`<tool>-skills` pair,
never after, and within a tool credential-guard always precedes git-branch-guard — `kiro-git-branch-
guard` is the last guard target of this wave (Windsurf has no native hook mechanism and stays out per
the ADR).

### Kiro global-scope git-branch-guard wiring (ROADMAP-2026-08-17 Wave 2/ML-2A) — `~/.kiro/hooks/trackfw-git-branch-guard.json`, dedicated file

<!-- trackfw-contract: gate=scripts/check-harness-hooks-parity.sh -->


Same `{"version":"v1","hooks":[...]}` schema as `kiro-credential-guard`'s own
`~/.kiro/hooks/trackfw-credential-guard.json` (see "Kiro global-scope wiring (ML-2F)" below), but a
**separate file**, with hook names `trackfw-git-branch-guard-global-pre`/`-global-post` instead of
`trackfw-credential-guard-global-pre`/`-global-post`. The other five CLIs (`claude`/`codex`/`gemini`/
`cursor`/`copilot`) reuse the exact same merge helpers their own `<tool>-credential-guard` targets
already use, passing `trackfw-git-branch-guard.sh`'s absolute path instead — both guards' entries
coexist as two distinct command entries in the same matcher's inner array (the merge helpers already
dedupe on exact command match and append a second, distinct command otherwise; they never overwrite
the first). Kiro alone splits into two files because `harnessCredentialGuardTargetKiro`/
`credentialGuardTargetKiro`/`_credential_guard_kiro_result` each rewrite their document WHOLESALE
every run — sharing that file with a second wholesale writer for git-branch-guard would make the two
targets flap between each other's desired 2-entry document on every subsequent run, breaking the
"missing never installs; unchanged content never rewrites" idempotency contract.

### GitHub Copilot global-scope wiring (ML-2E) — `~/.copilot/settings.json`, inline `hooks` field

<!-- trackfw-contract: gate=scripts/check-harness-hooks-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-harness-hooks-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-harness-hooks-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-update-parity.sh -->


Escopo desta seção: o preflight de `mutationInstall` no `IntegrationManager` dos três runtimes.
Afeta todo caller de `install` — `trackfw init --ai-tools`, `trackfw agents install`,
`trackfw skills install` e `trackfw update --install-missing`.

**Esta seção não altera o escopo de instalação.** As decisões D1 e D4 de
`ADR-2026-07-25-escopo-de-instalacao-selecionavel-para-agents-e-skills.md` permanecem em vigor:
`trackfw init --ai-tools`, sem TTY e sem `--scope`, instala em escopo **global**. O contrato
`trackfw update` vs `trackfw update harness` acima é escopado à **família `update`** e não impõe
fronteira projeto/global aos demais comandos.

### Problema

<!-- trackfw-contract: gate=scripts/check-update-parity.sh -->


Um artefato `outdated` **e** `owned` (declarado no manifest com o mesmo claim, bytes correspondentes
a um template trackfw anterior) fazia o preflight de `install` retornar erro. Como `mutate` é um lote
atômico com rollback, o erro **aborta a operação inteira**: um harness global desatualizado impedia
`trackfw init --ai-tools gemini` de fazer o scaffold de um projeto novo, com

```
artifact "/home/<user>/.gemini/agents/trackfw-architect.md" is outdated; use update
```

### Contrato

<!-- trackfw-contract: gate=scripts/check-update-parity.sh partial=cenários 6/7/8 exercitam cross-CLI só a linha outdated+owned da tabela (skip, bytes preservados, exit 0); a linha modified (erro, exige --force) não tem cenário no gate — grep por "modified"/"--force" no script não retorna teste correspondente -->


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

<!-- trackfw-contract: gate=scripts/check-update-parity.sh partial=o gate confirma a mensagem de skip byte a byte para um único artefato pulado; a exigência "chamado uma vez por artefato pulado, nunca duas vezes para o mesmo destino" precisaria de fixture com múltiplos artefatos pulados no mesmo lote, e nenhum cenário monta isso -->


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

<!-- trackfw-contract: gate=scripts/check-update-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-update-parity.sh partial=cenários 6 e 7 são cada um de escopo uniforme (s6 só global, s7 só projeto); a própria seção diz que as derivações proibidas só erram num lote de escopo MISTO — a fixture que discriminaria a derivação correta (por plan.claim.scope, por artefato) das proibidas (inferência do path renderizado, closure de escopo do comando) não existe no gate -->


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

<!-- trackfw-contract: gap reason=falsificável (rodar install sobre alvos not-installed e confirmar ausência de "skipping outdated artifact" em stderr), mas nenhum gate afirma essa ausência — os cenários 1-5 de check-update-parity.sh não verificam stderr vazio quanto a skip warnings; autodeclaração de "não é omissão" não prevalece sobre a falta de prova -->


Nenhum caller da família `update` precisa ligar o observador, e isso **não é omissão**. Verificado
em todos os call sites: `install` só é invocado para targets `not-installed` —
`internal/generators/update.go:502` e `:720` (ambos sob `case integrations.StateNotInstalled`),
`pypi/trackfw/commands/update_harness.py:222` (itera apenas `not_installed`), e
`npm/src/integrations/index.js:107` (único caller de `install` no Node, já recebe `onSkip` via
`execute`). Um artefato `not-installed` não pode ser `outdated` + `owned`, logo o branch de skip é
**inalcançável** por esses caminhos. Não "corrigir" ligando observadores ali.

### Aviso ao usuário — string pinada

<!-- trackfw-contract: gate=scripts/check-update-parity.sh partial=cenários 6 (global) e 7 (projeto) comparam a linha de warning inteira byte-a-byte entre os 3 runtimes e afirmam exit 0; a asserção de que as linhas de sucesso "✓ <tool> agents and skills" continuam sendo impressas não é verificada pelo gate -->


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

<!-- trackfw-contract: gate=scripts/check-update-parity.sh partial=a convergência atual de Node.js e Python para o formato do Go é a mesma prova byte-a-byte dos cenários 6/7; a diretriz "Go não deve ser alterado para se alinhar aos outros dois" é decisão de manutenção, não comportamento observável por gate -->


O runtime **Go** é a referência: o manager compõe a linha completa, derivando a remediação de
`item.plan.Claim.Scope` por artefato, e os callers apenas imprimem `reason`. Node.js e Python
convergem para essa forma. Go não deve ser alterado para se alinhar aos outros dois.

### Nota de teste

<!-- trackfw-contract: gate=scripts/check-update-parity.sh partial=a comparação byte-a-byte atual das strings de aviso (cenários 6/7) é a prova viva desta seção; a história do teste antigo invertido em npm/tests/agents-skills.test.js e a ausência prévia de cobertura equivalente em Go/Python são fatos de histórico de teste, não verificáveis por gate cross-CLI -->


`npm/tests/agents-skills.test.js` continha `assert.throws(() => manager.install([plan]),
/outdated.*update/i)` — asserção que codificava o contrato antigo e é **invertida** por esta seção.
Go e Python não tinham cobertura equivalente; ambos passam a tê-la. Auditoria de paridade compara as
strings de aviso **byte-a-byte** entre os três runtimes.

## Regra `branch_has_wip_roadmap` — comportamento unificado nos 3 runtimes

<!-- trackfw-contract: gate=scripts/check-validate-parity.sh partial=cobre as 3 linhas centrais da tabela via TRACKFW_BRANCH (roadmap em done/ com slug igual aceito, nenhum roadmap em wip/ nem done/ bloqueia, roadmap em done/ com slug diferente bloqueia); check-branch-new-parity.sh continua cobrindo só wip/ (cenário b) e ausência total (cenário a/f), sem cenário próprio de done/ — redundante com o bloco de check-validate-parity.sh, que exercita a mesma BranchSlugMatchesRoadmap; achado registrado (não corrigido, ver vault/notes/validate-branch-has-wip-roadmap-done-python-rule-null-2026-08-20.md): pypi/trackfw/validator.py's validate_branch_has_wip_roadmap retorna strings simples em vez do formato dict de _enrich_items, então o "rule"/"file" desta regra sai null em validate --json no Python (Go/Node.js tagueiam corretamente) — texto da mensagem é byte-idêntico nos 3, o gate compara por esse texto e pina a divergência de tag explicitamente -->


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

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh -->


Os quatro comandos de geração de artefatos produzem arquivos **byte-a-byte idênticos**
nos três runtimes para a mesma entrada. Isso inclui conteúdo e nome de arquivo.

### Frontmatter e formato — contrato explícito

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh -->

#### `req new <title>`

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh partial=cobre o template default gerado com título posicional (byte-a-byte, KIND="req"); o prompt de escopo local/global de ADR drafts via probe (Go+Node, requer TTY) e a ausência do fluxo de probes em Python são exceções documentadas sem gate -->


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

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh partial=cobre só o template default (--scope project implícito, KIND="adr", byte-a-byte); grep confirma zero ocorrência de "adr" combinado com "--scope"/"adr list" em qualquer check-*.sh — --scope global (diretório ~/.trackfw/adr/), `adr list` (project/global) e os flags Python-only --status/--dir não são exercitados por nenhum gate cross-CLI -->


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

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh partial=cobre template default, --title/--req e --from-req (KINDS roadmap/roadmap_flags/roadmap_from_req, incluindo o campo req: no formato exato "docs/req/<slug>.md" — linhas 272/388 do gate), o ciclo E2E backlog→analyzing em layout flat e by_agent, e uma asserção de conteúdo esperado (não só diff cross-stack) que os 3 KINDS acima contêm os literais `## Wave 0 — Threat Model`, `**Gates da wave:**` e `ML-0A` (AC14, ROADMAP-2026-08-22-wave-0-de-modelo-de-ameaca-no-harness-e-o-asset-do-arquiteto-ensina-trackfw-push, ML-2A — fecha a lacuna em que uma regressão sincronizada removendo Wave 0 dos 3 stacks passava despercebida no diff cross-stack sozinho, provado por check-gates-falsify.sh Cenário 166); as transições subsequentes da máquina de estados (analyzing→wip→blocked→done→abandoned) não são exercitadas por este gate -->


Arquivo: `docs/roadmaps/backlog/ROADMAP-YYYY-MM-DD-<slug>.md`

````
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

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** pending
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

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
````

`## Wave 0 — Threat Model` is prepended to every generated roadmap — both `roadmap new` (shown
above) and `roadmap new --from-req` (`## Wave 0` is always followed there by `## Wave 1 —
Implementation (derived from REQ criteria)`, never renumbered — `--from-req` has no separate
template subsection in this document; its body differs from the one shown above only in how the
implementation waves below Wave 0 are derived, one ML per REQ acceptance criterion, labeled
`ML-1A`, `ML-1B`, ... in criterion order). The Wave 0 ML is always labeled `ML-0A`, never `ML-1A` —
`ML-1A` is reserved for the first
implementation ML (or the first REQ-derived ML on the `--from-req` path). The `**Gates da wave:**`
block is a fixed, literal, non-interpolated `exit 1` placeholder (AC13,
docs/seguranca/2026-08-22-modelo-de-ameaca-da-wave-0-no-harness.md §2.1/§3 F5) — it fails closed
until the ML-0A author replaces the command with a project-specific evidence check; no REQ title,
slug or date is ever substituted into it, because `trackfw barrier` (see below) executes gate
commands via `sh -c` without sanitization. `trackfw barrier <roadmap> --wave 0` evaluates this
wave like any other (see "Wave label grammar" — the integer part accepts `0` specifically for this
convention).

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

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh partial=só o título acentuado ("Autenticação e Sessão") é exercitado pelo gate, conforme a própria seção declara; os outros dois exemplos da tabela ("ADR Config (v2)", "Minha Requisição #1") são apenas ilustrativos, sem prova por gate -->


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

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh -->


Todos os CLIs usam a data local (`date +%F` / `time.Now().Format("2006-01-02")` /
`datetime.date.today().isoformat()`) — nunca UTC. Geração cruzando meia-noite num
fuso horário avançado pode produzir datas distintas entre runtimes; o gate detecta
essa condição e falha explicitamente.

### Parity gate

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh -->


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

## CLAUDE.md — seção `## Architect responses` byte-idêntica nos 3 runtimes (ML-1A, ROADMAP-2026-08-21-regra-de-verbosidade-no-asset-do-arquiteto-e-nas-regras-semeadas)

<!-- trackfw-contract: gate=scripts/check-artifact-parity.sh -->

`trackfw init` / `trackfw discover --init` grava `CLAUDE.md` em cada runtime a partir de um
gerador embutido (`internal/generators/claudemd.go`, `npm/src/generators/init.js`,
`pypi/trackfw/generators/init_gen.py`). O `CLAUDE.md` completo **não** é byte-idêntico entre os
3 runtimes (Python inclui `## Architecture Directives (mandatory)` como seção de cabeçalho
separada; Go e Node.js não). A seção `## Architect responses`, acrescentada pelo ML-1A desta
ROADMAP, **deve** ser byte-idêntica nos 3 runtimes — ela define o protocolo de verbosidade do
agente Arquiteto e qualquer divergência silenciosa introduziria comportamento distinto por runtime.

O gate `check-artifact-parity.sh` isola e verifica apenas essa seção via `awk` (extrai de
`## Architect responses` até o próximo `## ` ou fim de arquivo) e compara byte a byte across
os 3 outputs de `init`. Um extrato vazio em qualquer runtime activa o vacuity guard antes da
comparação — evitando que a seção desapareça silenciosamente e a comparação byte a byte passe
por vacuidade (os 3 arquivos vazios concordam entre si).

Cenário P4 de falsificação em `scripts/check-gates-falsify.sh` (Cenário 84):
Node.js's `init.js` section header corrupted from `'## Architect responses'` to
`'## VERBOSITY_SECTION_REMOVED'` in an isolated npm copy; awk extraction finds no matching heading
→ vacuity guard fires `CLAUDE.md ## Architect responses missing or empty (node)`.

## Scripts de attention hooks (`trackfw-attention-signal.sh` / `trackfw-attention-cleanup.sh`) — byte-idênticos

<!-- trackfw-contract: gate=scripts/check-attention-scripts-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-attention-scripts-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh partial=o comparador estrutural garante que os 3 runtimes concordam entre si (mesmas chaves, mesmos valores) e que a ordem de array é significativa, mas não afirma independentemente que o valor bate com a string exata documentada por CLI — os 3 runtimes concordando com um mecanismo errado ainda passaria (mesmo padrão do achado "OpenCode agent representation" do lote 1) -->


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

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh -->


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

A prova negativa (P4) está em `scripts/check-gates-falsify.sh`, em dois
cenários que cobrem as duas camadas do gate: o Cenário 44 corrompe o
`matcher` da entrada `trackfw-credential-guard-post` do wiring do Kiro no
literal Node.js (`npm/src/generators/hooks.js`, de `'shell'` para
`'execute_bash'`) numa cópia isolada do repositório e asserta que o gate
reprova apontando `$.hooks[3].matcher` no diagnóstico — falsificando o
comparador estrutural (`compare_json`). O Cenário 46 cobre o segundo guard,
o de vacuidade (P2) `credential-guard-present` — o mesmo que capturou o
falso negativo ambiental corrigido acima: força as 3 funções de dedup
(`globalCredentialGuardInstalledClaude`/`_global_credential_guard_installed_claude`)
a sempre reportar "instalado", nas 3 cópias isoladas do source, o que
suprime a entrada de credential-guard do Claude de forma idêntica nos 3
stacks — o comparador estrutural continua satisfeito (nunca chega a rodar,
o gate sai antes, no guard de vacuidade) e só o guard `credential-guard-present`
reprova.

## Mecanismo de resolução de caminho dos hooks de projeto, por CLI

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh partial=o guard de vacuidade confirma que cada arquivo referencia trackfw-credential-guard.sh pelo menos uma vez, e o comparador estrutural confirma que os 3 runtimes concordam entre si sobre o comando emitido; nenhum gate afirma independentemente que a string emitida bate com o mecanismo exato documentado nesta tabela por CLI (env var vs shell substitution vs caminho relativo) -->


Decidido em
`docs/adr/ADR-2026-08-11-resolucao-de-caminho-dos-hooks-de-projeto-por-cli-mecanismo-especifico-do-fornecedor-sem-caminho-absoluto.md`
(pesquisa que sustenta a decisão:
`docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md`). Escopo: apenas hooks de **projeto**
(`.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json`, `.cursor/hooks.json`,
`.github/hooks/trackfw-attention.json`, `.kiro/hooks/*.json`) — os hooks de **escopo global**
(`trackfw update harness`, escritos em `~/.trackfw/...`) usam caminho absoluto por design e não
foram tocados por este mecanismo (caso distinto, fora do repo do usuário).

A falha de fundo é a mesma nos três CLIs alterados, mas o cwd contra o qual o `command` de hook
resolve **não** é idêntico entre eles — nem sempre a raiz do projeto, e por motivos diferentes: no
Claude Code o cwd do handler **acompanha os `cd` do agente durante a sessão** ("Handlers run in the
current directory"); no Codex CLI o cwd é fixo, mas é o cwd **da sessão** ("Commands run with the
session `cwd` as their working directory"), não necessariamente a raiz — o modo de falha é iniciar o
Codex a partir de um subdiretório, mais raro que o caso do Claude, mas real. Um caminho relativo puro
(`scripts/trackfw-*.sh`) só resolve corretamente se esse cwd coincidir com a raiz do projeto.

Dos três CLIs alterados, apenas **dois** foram provados quebrados por esse mecanismo: Claude Code
(bug em produção, corrigido em `0c66ecb`) e Codex CLI (verificação empírica no ML-3A, ver abaixo).
**Gemini CLI não foi provado quebrado** — a doc não afirma explicitamente que o cwd do hook deriva do
agente. Foi alterado mesmo assim por um argumento diferente, "mudança segura por construção": como
`$GEMINI_PROJECT_DIR` é documentado e usado em 100% dos exemplos oficiais de hook, resolve para a
raiz do projeto **independentemente** de o cwd derivar ou não — a mudança não pode piorar o
comportamento existente, então não era preciso provar o defeito para justificá-la (ADR §"Gemini CLI —
alterar, por argumento de assimetria"). Os outros três CLIs (Cursor, Copilot, Kiro) não foram
alterados — dois por já resolverem corretamente, um por mecanismo não verificável (ver a tabela e as
seções abaixo).

| CLI | Mecanismo | String emitida | Migração in-place? | Referência |
|---|---|---|---|---|
| Claude Code — credential-guard | placeholder de env var expandido em runtime | `$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh` | **Sim** — corrigido antes deste roadmap, em `0c66ecb` (v6.7.1); merge-based, matcher por ferramenta (`agentfiles.go:238–239`) | ADR §Decision, linha Claude Code |
| Claude Code — attention-signal/cleanup | placeholder de env var expandido em runtime | `$CLAUDE_PROJECT_DIR/scripts/trackfw-attention-{signal,cleanup}.sh` | **Sim** — estendido neste roadmap (ML-2A); merge-based, matcher `AskUserQuestion` (`agentfiles.go:213`, `:269`) — chamada de migração separada da do credential-guard, não a mesma | ADR §Decision, linha Claude Code |
| Codex CLI | substituição de shell (`$(...)`), aspas literais em torno da substituição | `"$(git rev-parse --show-toplevel)/scripts/trackfw-<script>.sh"` (JSON-escapado no arquivo gerado) | **Sim** — merge-based, mesmo motivo do Claude | ADR §Decision + §"Codex CLI — alterar, com dependência explícita de shell e git" |
| Gemini CLI | placeholder de env var expandido em runtime | `$GEMINI_PROJECT_DIR/scripts/trackfw-<script>.sh` | **Sim** — merge-based, mesmo motivo do Claude | ADR §Decision + §"Gemini CLI — alterar, por argumento de assimetria" |
| Cursor | nenhuma mudança — cwd de hooks de projeto já é fixo na raiz por design do fornecedor | caminho relativo puro, inalterado | Não precisa — não muda de string | ADR §"Cursor — não alterar" |
| GitHub Copilot CLI | nenhuma mudança — já usa o campo nativo `"cwd": "."` em todas as entradas | caminho relativo puro + `"cwd": "."`, inalterado | Não precisa — arquivo é regravado por inteiro a cada execução, não há entrada "antiga" a migrar | ADR §"GitHub Copilot CLI — não alterar; já estava correto" |
| Kiro | nenhuma mudança — mecanismo de resolução não verificável em doc primária (ver abaixo) | caminho relativo puro, inalterado | Não precisa — arquivo é regravado por inteiro a cada execução | ADR §"Kiro — não alterar (default de INDETERMINADO)" |

### Pré-condições do fix do Codex, descobertas empiricamente (não estão na doc do fornecedor)

<!-- trackfw-contract: gap reason=as três pré-condições (trust_level ausente, fora de repo git, submódulo/worktree, GIT_DIR/GIT_WORK_TREE) descrevem falhas silenciosas do mecanismo "$(git rev-parse --show-toplevel)" que o trackfw gera para o Codex; nenhum gate roda o hook do Codex fora de um repo git, dentro de submódulo, ou com GIT_DIR/GIT_WORK_TREE definidas para confirmar a degradação silenciosa descrita -->


Confirmadas no ML-3A rodando o `codex-cli` real (0.147.0), não em um shell isolado. Ver ADR Emenda 1
e `vault/notes/codex-hooks-de-projeto-so-rodam-em-projeto-trusted-2026-08-11.md`.

1. **O hook só roda se o projeto estiver marcado como `trusted`.** O Codex CLI só carrega hooks de
   **projeto** se aquele projeto estiver marcado como confiável em `~/.codex/config.toml`
   (`[projects."<path>"] trust_level = "trusted"`). Sem isso, os hooks do repositório são ignorados
   **em silêncio** — nenhum erro, nenhum log óbvio. Isso não muda a decisão (sem trust, nenhum hook
   roda, nem o antigo nem o novo), mas significa que o fix de caminho só produz efeito visível para
   o usuário final em projeto trusted. Para testes automatizados, o Codex expõe
   `--dangerously-bypass-hook-trust`; nunca escrever no `~/.codex/config.toml` real do usuário para
   "fazer um teste passar".
2. **`git rev-parse --show-toplevel` exige repositório git e depende de onde ele resolve a raiz do
   projeto — três casos, todos com a mesma consequência prática (guard não roda).**
   - **Fora de um repositório git**: o comando falha, a substituição vira vazia, o comando degrada
     para `/scripts/trackfw-credential-guard.sh` → o `trackfw-credential-guard.sh` **não executa**.
     Aceitável, pois o trackfw governa repositórios por definição.
   - **Dentro de um submódulo ou worktree**: o comando retorna a raiz *daquele* submódulo/worktree,
     que pode não ser onde o `scripts/` do trackfw vive.
   - **`GIT_DIR`/`GIT_WORK_TREE` definidas no ambiente do processo**: essas variáveis de ambiente,
     quando presentes, redirecionam para onde `git rev-parse --show-toplevel` resolve, produzindo o
     mesmo efeito prático dos dois casos acima — o hook resolve para uma raiz diferente da esperada
     e o `trackfw-credential-guard.sh` (controle de segurança) **pode deixar de executar em
     silêncio**, sem erro nem log óbvio.

   Em todos os três casos, a limitação é **conhecida e aceita, não corrigida por este roadmap**.
   O terceiro caso (`GIT_DIR`/`GIT_WORK_TREE`) foi identificado pela revisão de segurança em
   `docs/seguranca/2026-08-11-revisao-hooks-cwd.md` (Q3), que classificou o risco como aceitável e
   sem regressão contra a `main`, recomendando apenas o registro aqui.

   **[RECLASSIFICADO — ver §"Semântica de falha de hook por CLI — o que acontece quando o guard não
   roda (ROADMAP-2026-08-12, ML-3A)", item 6, abaixo neste mesmo arquivo]** O enquadramento acima
   ("limitação conhecida e aceita", "risco aceitável e sem regressão") descreve os três caminhos como
   degradação de **disponibilidade**. Com o veredito FAIL-OPEN do Codex confirmado empiricamente
   (`docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-codex.md`) e a Revisão ML-2B do parecer de
   segurança (`docs/seguranca/2026-08-12-semantica-de-falha-de-hook.md`), os três caminhos passam a
   ser **bypass silencioso de controle de segurança** — o texto histórico acima permanece por valor de
   registro, mas a classificação vigente é a da seção referenciada, não esta.

### Kiro — mecanismo de resolução não verificável em doc primária, mantido relativo

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh partial=o comparador estrutural confirma que o mecanismo do Kiro permaneceu inalterado (comando de hook comparado entre runtimes); o veredito INDETERMINADO em si — a impossibilidade de verificar o cwd do Kiro contra doc primária do fornecedor — é limitação de pesquisa documental, não testável por gate -->


Veredito `INDETERMINADO` em 2026-08-11 (`docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md`,
seção 5). As 4 páginas oficiais de hooks do Kiro consultadas nunca mencionam o diretório de trabalho
de execução da "Shell Command action" nem expõem uma env var de raiz de projeto:

- <https://kiro.dev/docs/hooks/>
- <https://kiro.dev/docs/hooks/types/>
- <https://kiro.dev/docs/hooks/actions/>
- <https://kiro.dev/docs/hooks/troubleshooting/>

Aplica-se o default do roadmap para `INDETERMINADO`: **não alterar** o mecanismo do Kiro. Sobrepor
este default exige evidência empírica direta (teste reproduzível no CLI real), nunca inferência a
partir de outro CLI.

### A heterogeneidade entre os 4 mecanismos é intencional, não divergência acidental

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh partial=a existência de 4 formas distintas por CLI é a mesma tabela já registrada na seção "Mecanismo de resolução..." acima, coberta pelo mesmo comparador estrutural (que confirma que os 3 runtimes concordam entre si, não que o valor bate com o mecanismo documentado); a justificativa de design (ordem de preferência, rejeição de mecanismo único) é rationale do ADR, não comportamento de CLI -->


Depois desta mudança existem **4 formas diferentes** de comando de hook entre os 6 CLIs
(`$CLAUDE_PROJECT_DIR/…`, `$GEMINI_PROJECT_DIR/…`, `"$(git rev-parse --show-toplevel)/…"`, e caminho
relativo puro para Cursor/Copilot/Kiro). Isso é deliberado: a regra geral, derivada no ADR
(§"Regra geral derivada"), é **preferir sempre o mecanismo nativo do fornecedor**, nesta ordem de
preferência:

1. Campo estruturado de working directory, quando existir (Copilot — `"cwd": "."`).
2. Placeholder/env var de raiz de projeto, expandido em runtime pelo próprio CLI (Claude, Gemini).
3. Substituição de shell (Codex) — último recurso, por introduzir pré-condições (shell, git).
4. Nenhuma mudança, quando o CLI já resolve contra a raiz por design (Cursor) ou quando o mecanismo
   não pode ser verificado (Kiro).

Caminho absoluto materializado no arquivo é proibido em todos os casos — os arquivos de settings são
versionados no repositório do usuário, e o path da máquina que rodou `trackfw init`/`update` não vale
para outro checkout. Sem esta nota, a leitura do código isoladamente (4 strings diferentes para
"o mesmo problema") convida a "corrigir" a heterogeneidade impondo um mecanismo único — o que o ADR
rejeita explicitamente em §"Alternatives Considered" ("Um único mecanismo para os 6 CLIs"), porque
forçaria `$(git rev-parse …)` em CLIs que já resolvem corretamente por meios próprios, adicionando
pré-condições sem defeito correspondente.

## Semântica de falha de hook por CLI — o que acontece quando o guard não roda (ROADMAP-2026-08-12, ML-3A)

<!-- trackfw-contract: none reason=cabeçalho de seção contendo só a citação de fontes (pesquisa documental/empírica, parecer de segurança); não é, em si, uma alegação de comportamento de CLI — as subseções abaixo carregam o conteúdo -->

> Fontes: `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-codex.md` (empírico, Codex CLI
> 0.147.0, inclusive a seção "ML-1C"), `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-varredura-documental.md`
> (documental, doc primária, 5 CLIs), `docs/seguranca/2026-08-12-semantica-de-falha-de-hook.md`
> (parecer de Hades — **consolidado a partir da seção "Revisão ML-2B", não da tabela original do
> parecer, que está marcada `[SUPERSEDIDA]` no próprio documento**).

### 1. Tabela por CLI — Caso A × Caso B × como se soube × severidade atual

<!-- trackfw-contract: none reason=tabela de semântica de falha de CLIs de terceiros (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, Kiro), sourced de doc do fornecedor e de medição direta do vendor CLI (não do trackfw) — insumo de risco para a decisão de arquitetura tratada nas seções seguintes, não comportamento do trackfw -->


- **Caso A** — o `command`/script do hook **não resolve** (ausente, caminho inválido, ou apagado em
  runtime): o processo nem chega a rodar.
- **Caso B** — o hook **roda** e sai com código != 0, distinguindo `exit 1` de `exit 2` quando a
  fonte permitir.

| CLI | Caso A | Caso B (`exit 1` vs `exit 2`) | Como se soube | Severidade atual |
|---|---|---|---|---|
| **Claude Code** | **FAIL-OPEN** — doc cita literalmente `No such file or directory` como exemplo do bucket não-bloqueante; a própria doc descreve o cenário de "policy hook" com caminho digitado errado ficando "silently disabled" | `exit 1` fail-open (citado nominalmente, sem JSON válido) · `exit 2` fail-closed em `PreToolUse`, blindado contra JSON contraditório | Documental — <https://code.claude.com/docs/en/hooks> | 🟡 — ver §5 (deixa de ser 🟢 na Revisão ML-2B) |
| **Codex CLI** | **FAIL-OPEN** — medido, `hook: PreToolUse Failed`, ferramenta prossegue | `exit 1` fail-open (medido, mesmo rótulo `Failed` do Caso A, confundidor de stderr fechado) · `exit 2` fail-closed (`hook: PreToolUse Blocked`, erro do router, ferramenta não executa) | **Empírico** — `docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-codex.md`, braços Caso A/B1/B2, `codex-cli 0.147.0`, com e sem `--dangerously-bypass-approvals-and-sandbox` | 🔴 — ver §5 |
| **Cursor** | **FAIL-OPEN por padrão** — doc agrupa "crash" com "timeout, invalid JSON" no mesmo enunciado de fail-open; não usa literalmente "script not found" (ressalva de interpretação registrada na fonte). Opt-in `failClosed: true` inverte por hook, não usado hoje pelo gerador do trackfw | Fail-open por padrão para qualquer não-2 (bucket único "Other exit codes", sem distinguir `exit 1` nominalmente) · `exit 2` fail-closed | Documental — <https://cursor.com/docs/hooks> | 🟡 — ver §5 |
| **Gemini CLI** | **INDETERMINADO** — buscado por "not found"/"ENOENT"/"no such file"/"spawn"/"invalid path" em 4 páginas de doc; nenhuma ocorrência que junte "exit codes" com "comando não iniciou". Tratado como pior caso (fail-open) neste documento | Não distingue `exit 1` de outros não-2 — qualquer código fora de `{0,2}` cai no bucket único `Other` = fail-open · `exit 2` = "System Block", fail-closed | Documental — <https://github.com/google-gemini/gemini-cli/blob/main/docs/hooks/reference.md> e `docs/hooks/index.md` | 🟡 — ver §5 |
| **GitHub Copilot CLI** | **Fail-closed para `preToolUse`** — doc agrupa "crash" com "non-zero exit" no mesmo enunciado de fail-closed (mesma ressalva de interpretação sobre "crash" vs "script not found" literal); exceção: timeout é sempre fail-open | Fail-closed para `preToolUse`, tanto `exit 2` quanto qualquer outro não-zero (exceto timeout); doc não distingue resultado entre `exit 1` e `exit 2` dentro de `preToolUse` (os dois negam) | Documental — <https://docs.github.com/en/copilot/reference/hooks-reference> | 🟢 qualificado — ver §5 (cobre ausência/crash, não cobre substituição de conteúdo) |
| **Kiro** | **INDETERMINADO** — nem a aba IDE nem a aba CLI de `hooks/actions/` discutem comando que não consegue rodar; buscado por "not found"/"ENOENT"/"127" nas 3 páginas de doc sem ocorrência | **Depende da superfície** — aba IDE: fail-closed para qualquer não-zero, sem distinguir `exit 1`/`exit 2`. Aba CLI: distingue como Claude — só `exit 2` bloqueia, `exit 1`/outros fail-open. Qual superfície o trackfw mira é pergunta em aberto, não resolvida por este ciclo | Documental — <https://kiro.dev/docs/hooks/actions/> (duas abas, textos diferentes na mesma seção) | Indeterminado — tratado como pior caso |

### 2. A distinção Caso A × Caso B

<!-- trackfw-contract: gap reason=o parágrafo pina o contrato de bloqueio do próprio trackfw (exit 2 + stderr, gerador em modo block — scaffold.go) e afirma que ele cobre só o Caso B (hook roda e decide); não há gate que rode o Caso A (comando ausente/caminho que não resolve) para provar que o trackfw de fato não tem nenhum mecanismo cobrindo esse caminho -->


O contrato de bloqueio conhecido do trackfw (`exit 2` + stderr — emitido pelo gerador em modo
`block`, `internal/generators/scaffold.go`, comentário "bloquear (`credential_guard.mode: block`,
exit 2)") cobre **apenas** o Caso B: o hook precisa **rodar** e **decidir** sair com 2. O Caso A —
comando ausente, caminho que não resolve, script apagado — é exatamente o que as três condições já
documentadas em "Pré-condições do fix do Codex" (acima, §"Mecanismo de resolução de caminho dos
hooks de projeto, por CLI") produzem: nenhuma delas passa pelo hook decidindo nada, porque o
processo do hook **nem chega a existir**. Medir só o Caso B e concluir "o CLI é fail-closed" responde
à pergunta errada — o contrato documentado é robusto onde se aplica, mas não cobre o caminho em que o
hook simplesmente não roda.

### 3. Discriminadores observáveis (economizam tempo de quem investigar)

<!-- trackfw-contract: none reason=discriminadores são mensagens de log do CLI de terceiro (Codex, Claude Code, Kiro), não strings emitidas pelo trackfw — auxílio de diagnóstico para quem investigar um incidente, não comportamento do trackfw a ser gateado -->


- **Codex** — nos logs de `codex exec`: `hook: PreToolUse Failed` (hook não rodou ou saiu fora do
  contrato de bloqueio — ferramenta **prossegue**) × `hook: PreToolUse Blocked` (`exit 2` exato —
  ferramenta **não** executa, com erro explícito `codex_core::tools::router: error=Command blocked by
  PreToolUse hook: ...`). **Só `exit 2` exato bloqueia** — testado com um confundidor fechado: script
  saindo `exit 1` com o **mesmo stderr** literal do braço `exit 2` (`"blocked by policy"`) continua
  fail-open (`hook: PreToolUse Failed`, marca do teste presente). O discriminador é especificamente o
  código de saída, não a presença de mensagem em stderr.
- **Claude Code** — `Failed with non-blocking status code: <mensagem do interpretador>` na primeira
  execução de um hook mal configurado é o sinal a observar; a doc do próprio fornecedor recomenda
  vigiar esse aviso especificamente em "policy hooks".
- **Kiro** — o discriminador relevante não está no comportamento do hook em si, mas em **qual
  superfície** (`hooks/actions/` aba IDE vs aba CLI) o trackfw de fato consome — a doc bifurca o
  comportamento nesse eixo, não em texto de log.

### 4. Hipótese refutada — registrada como tal, não apagada

<!-- trackfw-contract: none reason=registro de refutação de uma hipótese de ataque (mecanismo de cd) já descartada por medição própria do parecer de segurança (ML-1C); não é alegação de comportamento do trackfw pendente de gate, é histórico de investigação -->


O parecer original de segurança (`docs/seguranca/2026-08-12-semantica-de-falha-de-hook.md`, Pergunta
1/3, marcado `[REFUTADO]` inline) elevou o Codex a 🔴 com base no vetor: "o agente roda `mkdir x && cd
x && git init` e todas as chamadas subsequentes resolvem `git rev-parse --show-toplevel` para a raiz
aninhada, sem `scripts/` ali, reproduzindo o Caso A". Isso **não se reproduz** — medido diretamente
pelo ML-1C (`docs/pesquisa/2026-08-12-semantica-de-falha-de-hook-codex.md`, seção "ML-1C"), não
inferido da doc:

- **Experimento 1** (`cd` de shell explícito dentro do comando) e **Experimento 2** (parâmetro de
  working directory da própria chamada de ferramenta, que sobrevive à objeção "nada herdável" do
  Experimento 1 porque cada chamada do Codex já é um processo de shell novo) mediram, os dois, que o
  **cwd do hook é fixo na raiz da sessão** (`-C <fixture>`) — desacoplado tanto do `cd` de shell
  quanto do parâmetro de working directory da chamada de ferramenta que ele autoriza.
- Evidência de ponta a ponta: a própria expansão do `command` do hook (`$(git rev-parse
  --show-toplevel)/.codex/hooks/hook.sh`) continuou resolvendo o caminho a partir da raiz da sessão
  mesmo com a chamada de ferramenta mirando um subdiretório — se tivesse resolvido a partir do
  subdiretório, o hook teria sido rotulado `Failed` (caminho inexistente) e não haveria o segundo
  append no log; o segundo append ocorreu, provando a resolução pela raiz da sessão.

**Quem for reinventar essa hipótese deve consultar a medição do ML-1C antes** — o mecanismo de `cd`
não é o que sustenta a severidade do Codex hoje (ver §5).

### 5. O que sustenta a severidade hoje (medido × hipótese não medida)

<!-- trackfw-contract: gap reason=seção code-verifica fatos do próprio trackfw (script gerado sem proteção de escrita — os.WriteFile 0755 —, leitura de credential_guard.mode em runtime via grep do trackfw.yaml no cwd) que sustentam vias de ataque (Via 1/rm, achado 2.1/sobrescrita, downgrade de mode); nenhum gate ou teste automatizado exercita essas vias (sobrescrever o script, editar credential_guard.mode) para confirmar o resultado fail-open descrito -->


Com o mecanismo de `cd` refutado, a Revisão ML-2B (`docs/seguranca/2026-08-12-semantica-de-falha-de-hook.md`,
seção final) reavaliou alcançabilidade a partir de vetores que **não dependem do cwd se mover** —
escrita direta de arquivo, alcançável em qualquer CLI cujo agente tenha acesso a `Bash`/shell, não só
no Codex.

**Medido / código-verificado (confiança alta):**

- **`scripts/trackfw-credential-guard.sh` não tem proteção de escrita/deleção.** Gerado por
  `os.WriteFile(path, ..., 0755)` (`internal/generators/scaffold.go:797`,
  `GenerateCredentialGuardScript`) — sem imutabilidade, sem checksum registrado em nenhum lugar do
  repositório, sem verificação de integridade em `trackfw validate`/`doctor` hoje (confirmado por
  ausência de referência a `credential_guard` em `internal/validator/*.go`).
- **Via 1 — apagar o script (`rm`).** Composição de dois fatos, cada um já estabelecido
  separadamente: a ausência de proteção acima (código-verificado) + o Caso A medido no ML-1A
  (script ausente → `hook: PreToolUse Failed` → ferramenta prossegue → FAIL-OPEN). A cadeia completa
  `rm` → hook → tool-call não foi rodada como experimento único contínuo, mas isso mediria o mesmo
  estado ("arquivo ausente") por um caminho diferente — não é hipótese nova sobre o comportamento do
  Codex.
- **Achado 2.1 — sobrescrever o conteúdo do script (mais forte que `rm`).** Trocar o script por um
  que sai `exit 0` (`echo '#!/bin/bash' > scripts/trackfw-credential-guard.sh && echo 'exit 0' >>
  scripts/trackfw-credential-guard.sh`) é estritamente pior que apagar: produz `hook: PreToolUse
  Completed` — o **mesmo rótulo de sucesso normal** medido no controle positivo do ML-1A — zero sinal
  distinguível de execução legítima. E o próprio comando de escrita não contém JWT nem chave AWS, não
  casando com o padrão de detecção do credential-guard.
- **Downgrade de `credential_guard.mode` via `trackfw.yaml`.** O modo (`warn`/`block`) é lido em
  **runtime**, por `grep` simples do `trackfw.yaml` no cwd (`internal/generators/scaffold.go:1005`,
  `credentialGuardModeResolution`) — não embutido no script. Uma edição de uma linha derruba o
  controle para não-bloqueante sem tocar no script em absoluto (bit-idêntico), derrotando até uma
  eventual verificação de integridade que cubra só o script. Mesmo mecanismo no escopo global
  (`~/.trackfw/scripts/trackfw-credential-guard.sh`, `internal/generators/scaffold.go:1047-1051`).

**Hipótese não medida, rotulada como tal (não usada para sustentar severidade sozinha):**

- **Via 2 — gitfile redirecionando `.git`.** Mecanicamente plausível (mesmo princípio dos caminhos de
  submódulo/worktree já documentados abaixo), mas depende de também controlar `core.worktree` do
  `gitdir` alvo — via de dois passos, não medida.

**Consequência para o ranking por CLI:** as vias de escrita direta (Via 1, achado 2.1) não dependem de
nenhum mecanismo específico do Codex — são escrita comum de arquivo, alcançável em qualquer CLI com
`Bash`/shell. Claude e Gemini, antes tratados como 🟢 "efetivo por inalcançabilidade" (Caso A só via
`$CLAUDE_PROJECT_DIR`/`$GEMINI_PROJECT_DIR` vazio, degradando para `/scripts/...` não plantável),
**sobem para 🟡** — essas vias não dependem da env var estar vazia, atuam diretamente no caminho real
do script. Cursor permanece 🟡 (`failClosed: true`, se adotado, cobre `rm` mas não a sobrescrita —
script presente, roda, sai `exit 0` não é crash/timeout/JSON inválido). Copilot é o único CLI que
muda de forma qualificada: fail-closed nativo captura ausência/crash (Via 1), mas **não** o achado
2.1 (script presente, executa, sai 0 — não há "falha" para o Copilot detectar) — fica 🟢 apenas para
esse subconjunto, não para "substituição". **Codex permanece 🔴** — não por ser estruturalmente pior
nesta classe de vetor, mas por ser o único CLI onde o vetor original, a Via 1 e o achado 2.1 foram
todos, em algum grau, verificados ou código-verificados neste ciclo.

### 6. Reclassificação retroativa — referência cruzada com "Pré-condições do fix do Codex"

<!-- trackfw-contract: gap reason=reclassifica os três caminhos já documentados do mecanismo do Codex (fora de repo git, submódulo/worktree, GIT_DIR/GIT_WORK_TREE) de "degradação de disponibilidade" para "bypass silencioso de controle de segurança" — mesma lacuna de gate já registrada na seção "Pré-condições do fix do Codex" acima: nenhum cenário executa esses três caminhos -->


Os três caminhos já documentados acima (§"Pré-condições do fix do Codex, descobertas
empiricamente") — fora de repositório git, dentro de submódulo/worktree, e `GIT_DIR`/`GIT_WORK_TREE`
redirecionando a resolução — estavam registrados como **degradação de disponibilidade**: "o guard às
vezes não roda, o agente perde uma verificação, mas o pior caso é ruído". Com o veredito FAIL-OPEN do
Codex confirmado (§1) e o Caso A definido como o que esses três caminhos produzem (§2), esse
enquadramento está errado — **o guard deixar de rodar não é neutro; é o controle de negação sendo
desligado sem nenhum sinal que o usuário normalmente observa.** Os três caminhos passam de "limitação
conhecida / disponibilidade" para **bypass silencioso de controle de segurança**, na mesma classe de
risco da Via 1/achado 2.1 acima (§5). Achado adicional de detectabilidade: o gate de *trust* do Codex
(hooks de projeto só carregam em projeto `trusted`, ver a mesma seção acima) é **pior** que o Caso A
nesse eixo — o Caso A ao menos produz `hook: PreToolUse Failed` nos logs; um projeto não confiável não
produz nenhum evento de hook, zero sinal.

### 7. Escopo do que foi medido — não avaliado

<!-- trackfw-contract: none reason=a própria seção é uma declaração de escopo do que NÃO foi medido (sessão interativa do Codex, prova empírica dedicada para Claude/Cursor/Copilot/Kiro/Gemini) — registra fronteiras de uma investigação, não uma alegação positiva de comportamento de CLI a ser gateada -->


Confirmado apenas para `codex exec` (modo não-interativo), tanto com
`--dangerously-bypass-approvals-and-sandbox` quanto com sandbox restrito real (`-s
workspace-write`) — não é artefato do bypass. **Não avaliado:** sessão interativa do Codex com
aprovação humana no loop; e prova empírica dedicada não foi feita para Claude, Cursor, Copilot ou
Kiro (só documental) nem para o Gemini (`INDETERMINADO` tratado como pior caso, sem verificação
empírica considerada necessária no momento deste ciclo — decisão reavaliável se surgir mitigação que
dependa da semântica real). Nenhuma mudança de código foi feita neste ML — mitigação (wrapper
`test -x`, controle positivo em `validate`/`doctor`, `failClosed: true` no Cursor, verificação de
integridade de script/config) permanece **avaliada, não implementada**, cabendo a Zeus decidir se abre
REQ nova a partir do parecer de Hades.

## Controle positivo do credential-guard: o que a regra `credential_guard_hook_resolvable` cobre, e o que não cobre (ROADMAP-2026-08-12-mitigacao-do-fail-open, Wave 1/2/3/3-bis + Barreira B1)

<!-- trackfw-contract: gate=scripts/check-validate-parity.sh partial=bloco ROADMAP-2026-08-20 ML-3A/ML-4B e ROADMAP-2026-08-21 ML-2A e ROADMAP-2026-08-22 ML-2A e ML-4A: cobre 18 casos CG (claude-absent/claude-present/cursor-absent/cursor-present/claude-noexec/claude-notype/claude-relativo/copilot-relativo-present/claude-pwd/claude-pwd-quoted/claude-absoluto/claude-git-toplevel/claude-outra-var/cursor-pwd/claude-tilde/claude-tilde-quoted/claude-pwd-braced/claude-sh-c-pwd) e 2 casos GBG (gbg-claude-relativo/gbg-cursor-relativo-present) byte-identicos nos 3 CLIs; Cenário 80 prova nao-vacuidade do cross-CLI de deteccao; Cenários 159/160 provam as duas direcoes do discriminante de falso-positivo (acusar de menos e acusar de mais para Copilot); Cenário 164 prova direcao-A da classificacao por ancoragem ($PWD suprimido); Cenário 165 prova direcao-B (caminho absoluto acusado — o falso-positivo caro desta entrega); ML-4A adicionou: ~/... sem aspas=classe1 (silencio), "~/..." com aspas=classe2 (acusar), ${PWD}/...=classe2, sh -c "$PWD/..."=mensagem-PWD (Contains vs HasPrefix); nao exerce todas as 6 entradas de credentialGuardHookFiles (Codex/Gemini/Kiro dependem de cobertura unitaria interna por runtime) -->

> Fontes: `internal/validator/validator_credential_guard.go` (implementação, os 3 CLIs têm
> equivalente em `npm/src/` e `pypi/trackfw/`), `docs/adr/ADR-2026-08-12-defesa-do-credential-guard-vive-no-escopo-global-controle-que-mora-onde-o-agente-escreve-nao-e-controle.md`
> (decisão de arquitetura), `docs/req/REQ-2026-08-12-credential-guard-de-escopo-global-como-caminho-padrao-consentimento-explicito-verificacao-da-premissa-de-sandbox-e-a-via-do-credential-guard-mode.md`
> (follow-up ainda não implementado). Continuação direta de §"Semântica de falha de hook por CLI"
> (acima) — aquela seção **mediu** o problema (fail-open em 4 de 6 CLIs); esta seção documenta **o
> que foi entregue em resposta**, e o que não foi.

Depois de reverter as outras três mitigações avaliadas (ML-3C, ver abaixo), esta regra é a **única
coisa que sobrou no escopo de projeto**. O risco real de documentação é alguém ler esta seção como
"o incidente está mitigado" — não está; leia a subseção 2 com o mesmo peso que a 1.

### 1. O que a regra faz

<!-- trackfw-contract: gate=scripts/check-validate-parity.sh partial=bloco ML-3A cobre resolução de caminho para Claude ($CLAUDE_PROJECT_DIR/) e Cursor (relativo puro); as outras 4 formas (Codex/Gemini/GitHub/Kiro) têm paridade de wiring coberta pelo Cenário 44 mas não gate cross-CLI de existência/executabilidade -->


`credential_guard_hook_resolvable` (default `error`, configurável por `rules:` no `trackfw.yaml`,
como as demais regras do validador — `applyRule`/`applyRuleTagged`, `internal/validator/validator.go:120,136`):

- Para cada arquivo de hook de **projeto** que **existir** — a lista fechada `credentialGuardHookFiles`
  em `validator_credential_guard.go:28-35`: `.claude/settings.json`, `.codex/hooks.json`,
  `.gemini/settings.json`, `.cursor/hooks.json`, `.github/hooks/trackfw-attention.json`,
  `.kiro/hooks/trackfw-attention.json` — varre recursivamente o JSON já decodificado
  (`collectCredentialGuardCommands`) e coleta todo valor-string que referencia
  `trackfw-credential-guard.sh`, independentemente do nome do campo (`command`, `bash`,
  `action.command` — varredura por valor, não por schema, decisão de design registrada no
  comentário de `collectCredentialGuardCommands`).
- **Resolve o caminho** usando exatamente as 3 formas que o trackfw emite hoje (ver acima,
  §"Mecanismo de resolução de caminho dos hooks de projeto, por CLI"): `$CLAUDE_PROJECT_DIR/…` /
  `$GEMINI_PROJECT_DIR/…` substituído pela raiz do projeto; `"$(git rev-parse --show-toplevel)/…"`
  substituído pela raiz do projeto com as aspas literais removidas; caminho relativo puro
  (Cursor/Copilot/Kiro) resolvido contra a raiz do projeto.
- Verifica que o script resolvido **existe** (`os.Stat`) e é **executável** (`info.Mode()&0111 != 0`).
- Mensagem de violação acionável: arquivo de hook, CLI, caminho resolvido, e a ação (`trackfw update`
  regenera o script).
- Hook ausente ou JSON inválido é **pulado em silêncio** — não é responsabilidade desta regra garantir
  a existência ou a forma do arquivo de hook.
- Comando que não casa nenhuma das 3 formas de prefixo é **ignorado de propósito** —
  `resolveCredentialGuardHookPath` retorna `ok=false` e o chamador não trata isso como violação.
  Não é função desta regra adivinhar wiring próprio do usuário fora dos formatos que o trackfw gera.
- **Ausência de entrada de guard não é violação** — é o estado legítimo de quem usa só o guard
  **global** (`~/.trackfw/`, fora do escopo desta regra; o dedup `globalCredentialGuardInstalled*()`
  pula as entradas de projeto de propósito).
- Provada não-vácua pelo Cenário 47 de `scripts/check-gates-falsify.sh` (ML-2A): desabilitar a regra
  faz o cenário falhar.

### 2. O que ela NÃO cobre — com o mesmo destaque

<!-- trackfw-contract: gap reason=cada afirmação de não-cobertura é comportamento falsificável do trackfw (ex.: substituir o script por "exit 0" e confirmar que a regra não acusa; editar credential_guard.mode e confirmar que a regra não verifica) — nenhum gate ou teste, cross-CLI ou por runtime, exercita esses cenários adversariais; o achado 2.1 citado é só argumento de código-lido, não um teste executado -->


- **Verifica no momento em que `validate` roda, não no momento da invocação do hook.** Entre um
  `validate` verde e a próxima chamada de ferramenta, o script pode ser apagado ou sobrescrito sem
  que nada dispare uma nova checagem. O controle é **pontual**, não contínuo.
- **Detecta ausência, não sobrescrita.** Um script substituído por `exit 0`
  (`echo '#!/bin/bash' > scripts/trackfw-credential-guard.sh; echo 'exit 0' >> scripts/trackfw-credential-guard.sh`)
  continua existindo e continua executável — `os.Stat` e o teste de bit `0111` passam os dois. A
  regra fica **muda**. Pior: no Codex isso produz `hook: PreToolUse Completed`, o **mesmo rótulo de
  sucesso normal** de uma execução legítima — zero sinal observável em qualquer um dos dois lugares
  (validador e log de hook). Ver §"Semântica de falha de hook por CLI", §5, "Achado 2.1".
- **Não cobre downgrade de `credential_guard.mode` no `trackfw.yaml`.** O modo (`warn`/`block`) é lido
  em **runtime**, por leitura direta do `trackfw.yaml` no cwd (`internal/generators/scaffold.go:1005`,
  `credentialGuardModeResolution`) — não embutido no script nem verificado por esta regra. Uma edição
  de uma linha de YAML derruba o controle para não-bloqueante sem tocar no script em absoluto
  (bit-idêntico ao script legítimo).
- **Comando com formato de caminho desconhecido é ignorado de propósito.** Evita falso positivo em
  wiring próprio do usuário fora dos 3 formatos que o trackfw gera hoje — mas, como consequência
  direta, a regra **não cobre wiring que o trackfw não gerou**, mesmo que aponte para um script
  ausente.

### 3. A decisão de arquitetura — escopo global é a defesa real

<!-- trackfw-contract: gap reason=a tabela de mitigações rejeitadas afirma um fato falsificável de output gerado ("failClosed: true não usado hoje pelo gerador do trackfw") — grep confirma zero ocorrência de "failClosed" em todo internal/generators/*.go e em qualquer check-*.sh; nenhum gate afirma essa ausência, e o comparador estrutural não discrimina "os 3 runtimes omitem a chave" de "os 3 runtimes deveriam omitir a chave" -->


Depois de avaliar três mitigações adicionais de escopo de **projeto**, a decisão registrada em
`docs/adr/ADR-2026-08-12-defesa-do-credential-guard-vive-no-escopo-global-controle-que-mora-onde-o-agente-escreve-nao-e-controle.md`
foi: **a defesa real vive no escopo global** (`~/.trackfw/`, fora do repositório em que o agente
trabalha, `trackfw update harness`). O escopo de projeto — incluindo a regra desta seção — passa a
ser tratado como **conveniência de configuração, não controle de segurança**.

As três mitigações avaliadas e **rejeitadas**, cada uma com seu motivo (não reabrir sem ler o ADR
primeiro):

| Mitigação | Rejeitada porque |
|---|---|
| **`failClosed: true`** (opt-in nativo do Cursor) — chegou a ser **implementada** nos ML-3A/3B e depois **revertida** no ML-3C desta mesma branch | Cobre só 1 de 6 CLIs (só o Cursor tem opt-in nativo); e **brica clone fresco** — repositório com `.cursor/hooks.json` já commitado, antes do `init` gerar o script, trava toda chamada de shell dentro do Cursor. Mesma classe de *bricking* que o item wrapper foi adiado por evitar — enviar um e adiar o outro pelo mesmo argumento era a incoerência que a Barreira B1 corrigiu. |
| **Wrapper** (`sh -c 'test -x <script> && exec <script> || exit 2'`) | Mesma classe de *bricking*, confirmada no código: o script é **gerado** por `trackfw init`/`update harness` (`internal/generators/scaffold.go:779-837`), não faz parte do binário — um clone fresco, antes do `init`, travaria toda chamada de ferramenta. E cobre só a via de **deleção**; um script substituído por `exit 0` passa no `test -x` e sai `0` (não cobre sobrescrita). |
| **Verificação de integridade** de conteúdo do script e de `credential_guard.mode` | É a única que cobriria sobrescrita **e** downgrade de config — mas exige um valor de referência guardado **fora** do arquivo gerado, ou seja, exige exatamente o escopo global como pré-requisito para ter onde guardar a referência de forma confiável. Se o escopo global é pré-requisito da solução, é mais simples e mais forte usá-lo diretamente como a solução, em vez de construir integridade em cima dele. |

O guard global por padrão fica como **REQ própria** (`REQ-2026-08-12-credential-guard-de-escopo-global-...`),
ainda **não implementada** neste ciclo.

### 4. Riscos aceitos — sem suavizar

<!-- trackfw-contract: gap reason=cada risco listado é comportamento falsificável e atual do trackfw (ex.: credential_guard.mode continua rebaixável por uma linha de trackfw.yaml, independentemente de escopo projeto/global — internal/generators/scaffold.go:1047-1051) e nenhum gate adversarial exercita esse downgrade; a premissa "agente restrito ao workspace" é explicitamente marcada como NÃO medida pela própria seção -->


Copiados do ADR, na íntegra, sem atenuação:

- **Quem não instala o guard global fica sem defesa** contra as vias de deleção e sobrescrita do
  script. O guard global tem pré-condição de instalação e **não é o default hoje** — até virar
  default, a decisão é uma **intenção**, não uma proteção. Esta é a lacuna mais importante do ADR.
- **`credential_guard.mode` no `trackfw.yaml` continua rebaixável por uma linha de YAML**,
  independentemente do escopo do script — no escopo de projeto ou no global
  (`~/.trackfw/scripts/trackfw-credential-guard.sh`, `internal/generators/scaffold.go:1047-1051`, mesmo
  mecanismo de leitura em runtime). Esta via **não é fechada** pela decisão de preferir o escopo
  global, e precisa entrar na REQ de "global por padrão".
- **A premissa "agente restrito ao workspace" NÃO foi medida.** Não foi verificado se o agente
  alcança `~/.trackfw/` nos ambientes reais em que o trackfw roda. A premissa vale para sandboxes que
  restringem escrita fora do projeto — **não universalmente**. Um agente sem sandbox alcança `$HOME`.
  Tratar como hipótese a verificar antes de considerar o escopo global uma defesa forte, não como
  fato estabelecido.

### 5. Referências cruzadas

<!-- trackfw-contract: none reason=são apenas referências cruzadas a outras seções deste mesmo documento e a uma REQ de follow-up; nenhuma alegação nova de comportamento de CLI -->


- §"Semântica de falha de hook por CLI — o que acontece quando o guard não roda" (acima, mesmo
  documento) — mede o problema que esta seção responde; §5 e §6 daquela seção documentam as vias de
  sobrescrita e o achado 2.1 citados na subseção 2 acima.
- §"Mecanismo de resolução de caminho dos hooks de projeto, por CLI" (acima) — as 3 formas de
  prefixo que `resolveCredentialGuardHookPath` resolve.
- `docs/req/REQ-2026-08-12-credential-guard-de-escopo-global-como-caminho-padrao-consentimento-explicito-verificacao-da-premissa-de-sandbox-e-a-via-do-credential-guard-mode.md`
  — follow-up que precisa fechar: guard global como default, verificação da premissa de sandbox, e a
  via de `credential_guard.mode`.

## Hooks GLOBAIS de credential-guard (`~/.claude/settings.json`, `~/.codex/hooks.json`, `~/.gemini/settings.json`, `~/.cursor/hooks.json`, `~/.copilot/settings.json`, `~/.kiro/hooks/trackfw-credential-guard.json`) — paridade estrutural (ROADMAP-2026-08-06, ML-4A)

<!-- trackfw-contract: gate=scripts/check-harness-hooks-parity.sh -->


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

<!-- trackfw-contract: gate=scripts/check-harness-hooks-parity.sh -->


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

<!-- trackfw-contract: gap reason=nenhum gate ou teste roda de fato o script gerado (scripts/trackfw-credential-guard.sh) sem trackfw.yaml/sem a chave credential_guard.mode para confirmar o fallback DEFAULT_MODE=block no escopo global vs DEFAULT_MODE=warn no escopo de projeto; check-harness-hooks-parity.sh só compara a estrutura do wiring JSON, não a execução do script nem a leitura de MODE em runtime — grep por DEFAULT_MODE/credential_guard.mode nos check-*.sh só retorna a regra distinta credential_guard_mode_downgrade (seção "Ancoragem no HEAD", fora deste lote) -->


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

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh partial=os matchers de leitura/escrita são comparados estruturalmente (os 3 runtimes concordam entre si) pelo mesmo comparador do wiring de Bash, com o mesmo limite: não afirma independentemente que o valor bate com a tabela documentada por CLI; a cobertura ponta-a-ponta (matcher → script bloqueando/alertando um payload real) é testada só em npm/tests/credential_guard.test.js e pypi/tests/test_credential_guard.py — a seção omite menção ao equivalente em Go, que existe (internal/generators/credential_guard_test.go, credential_guard_sabotage_test.go) mas não é cross-CLI — não há gate ou suíte que compare os 3 runtimes entre si para essa ponta-a-ponta -->


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

<!-- trackfw-contract: none reason=documenta princípios de design dos próprios gates de paridade (meta-processo de qualidade), não um comportamento de CLI a ser verificado por gate -->


Todo gate de paridade e toda regra do validator devem seguir os quatro princípios documentados em
[`docs/gate-design-principles.md`](gate-design-principles.md): nenhum número mágico (P1), falha
explícita sem degradação silenciosa (P2), independência de ambiente (P3) e falsificabilidade
obrigatória (P4). O arquivo inclui os quatro defeitos reais que motivaram os princípios e o
checklist de aceite para gates novos.

A implementação de P4 é `scripts/check-gates-falsify.sh` — todo gate novo de paridade registra
ali sua prova negativa.

## Release rule

<!-- trackfw-contract: gate=scripts/check-static-assets.sh partial=a regra geral ("mudanças exigem testes equivalentes nos runtimes afetados") é princípio de processo, não uma alegação isolada de comportamento de CLI; só a paridade byte-a-byte dos assets estáticos do dashboard (internal/serve/static vs cópias em npm/pypi) é diretamente verificada por gate -->


Changes to commands, options, exit codes, JSON fields, validation rules, or
generated artifact semantics require equivalent tests in all affected runtimes.

`internal/serve/static` is the canonical dashboard asset source. Copies packaged
by npm and PyPI must remain byte-identical and are checked in CI.

## Plugin subsystem — removed (ADR-2026-08-15)

<!-- trackfw-contract: gate=scripts/check-unknown-command-parity.sh -->


`trackfw plugins {list,add,remove}` and all plugin download/execution code were
removed by
[`ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-terceiro.md`](adr/ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-terceiro.md).
trackfw no longer downloads, installs, manages, or executes third-party code.
This also closes the previously-documented exception where Python never had
plugin installation: with the removal, the three CLIs converge on the same
behavior and the exception is no longer needed. The command row above has been
deleted rather than marked `no`.

Removal included the argument-dispatch fallback that used to turn ANY
unrecognized top-level command into an attempt to execute a `trackfw-<name>`
binary found on `PATH` (`internal/commands/root.go`'s old `RunPlugin` call). An
unrecognized command is now always an "unknown command" error — see the next
section for the canonical, cross-CLI message this produces.

## Unknown top-level command — canonical message

<!-- trackfw-contract: gate=scripts/check-unknown-command-parity.sh -->

`trackfw <unrecognized>` produces, in **all three CLIs**, on **stderr**, **exit
code 1**:

```
Error: unknown command "x" for "trackfw"
Did you mean "validate"?
Run 'trackfw --help' for usage.
```

The `Did you mean "..."?` line is present only when a close-enough command
exists; with no eligible candidate, only the first and third lines are
printed. Before this contract, the three frameworks' default behavior diverged
in every dimension — text, quoting, exit code, and whether a suggestion was
offered at all:

```
GO (cobra)       exit 1  Error: unknown command "x" for "trackfw"
                          Run 'trackfw --help' for usage.
NODE (commander) exit 1  error: unknown command 'x'
                          (Did you mean validate?)
PY (argparse)    exit 2  trackfw: error: argument COMMAND: invalid choice: 'x' (choose from ...)
```

None of the three frameworks' built-in suggestion mechanisms are used to
produce the canonical message — cobra's `Command.SuggestionsFor`, commander's
`suggestSimilar` (Damerau-Levenshtein, threshold 3, similarity ratio) and
argparse's `invalid choice` listing all use different distance functions and
thresholds, which would make the three CLIs disagree on whether/what to
suggest for the same typo. Instead, all three reimplement the same plain
(no-transposition) Levenshtein distance and the same suggestion-picking rule:

- a candidate is eligible when its case-insensitive Levenshtein distance to
  the typed text is `<= 2`, **or** it is a case-insensitive prefix match;
- among eligible candidates, the one with the lowest distance wins, ties
  broken alphabetically — a single, deterministic suggestion.

Implementations: `internal/commands/root.go` (`suggestCommand`,
`levenshteinDistance`), `npm/src/lib/unknown-command.js` (`suggestCommand`,
`levenshteinDistance`), `pypi/trackfw/unknown_command.py` (`suggest_command`,
`levenshtein_distance`). The Python entry point additionally overrides
`ArgumentParser.error` narrowly — only for the top-level "invalid choice on
COMMAND" message — specifically to change this one error's exit code from
argparse's default `2` to `1` without touching the exit code of any other
argparse error (missing/invalid flags, `unrecognized arguments: ...`, etc.),
which stay `2` as before, consistent with the pre-existing, deliberately
unpinned exit-code divergence for unknown *flags* noted below.

Verified byte-for-byte across the three runtimes, plus the falsification
vector (a real `trackfw-vaildate` executable placed on `PATH`, which the
removal above must never invoke) by `scripts/check-unknown-command-parity.sh`
(added to `make parity`).

### Candidate set — "completion" is Go-only, and deliberately excluded

<!-- trackfw-contract: gap reason=nenhum cenário de check-unknown-command-parity.sh testa um typo próximo de "completion" (grep confirma zero ocorrências da palavra no script) para confirmar que a lista de candidatos exclui "completion" nos 3 CLIs -->

Cobra auto-registers a `completion` subcommand that has no Node.js/Python
equivalent (shell-completion script generator, framework built-in, unrelated
to plugins). If the suggestion algorithm used cobra's own command list, a typo
near "completion" would suggest it only in Go, breaking parity in an edge
case. All three implementations instead use an explicit candidate list — in Go
this is `root.Commands()` filtered to exclude `"completion"` by name, matching
the fixed list that Node.js (`program.commands`) and Python
(`subparsers.choices`) already have.

### Bare invocation (`trackfw` with no argument) — unified

<!-- trackfw-contract: gate=scripts/check-unknown-command-parity.sh -->

`trackfw` **with no argument at all** behaves identically across the three runtimes: **exit
`0`**, help printed to **stdout**, **stderr empty**. This used to diverge — Go exited `0`
with help on stdout while Node.js's default commander behavior exited `1` with help on
stderr — and was documented as a known gap when the plugin subsystem was removed (commit
`f9202f4`).

**Decision (ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md,
ML-1C):** Go's behavior is canonical — a bare `trackfw` invocation is a legitimate request for
help, not an error, so it exits `0` with the help text on stdout. Node.js's default commander
root-command-with-no-`.action()` behavior was overridden to match.

Non-regression, measured alongside the fix: an actually unknown command (e.g. `trackfw zzz`)
is unaffected by this change and keeps the contract from "Unknown top-level command — canonical
message" above — **exit `1`**, message on **stderr**, identical across the three runtimes.


## Detecção de adulteração do credential-guard: as duas regras novas, e o que elas não veem (ROADMAP-2026-08-12-deteccao-de-adulteracao)

<!-- trackfw-contract: none reason=cabeçalho de citação ao ADR e aos arquivos de implementação, sem alegação de comportamento própria — o conteúdo factual está nas subseções abaixo -->

> Decisão: `docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md`
> (ler as **3 emendas**). Implementação: `internal/validator/validator_credential_guard_integrity.go`
> + equivalentes em `npm/src/validator/index.js` e `pypi/trackfw/validator.py`. Continuação de
> §"Semântica de falha de hook por CLI" e §"Controle positivo do credential-guard".

### As três vias, e quem cobre cada uma

<!-- trackfw-contract: gate=scripts/check-gates-falsify.sh partial=os Cenários 47/49/50 provam não-vacuidade de cada uma das 3 regras (hook_resolvable/script_integrity/mode_downgrade), mas por desenho comentado nos próprios cenários ("por que só o CLI Go") testam só o CLI Go — paridade de severidade/âncora entre os 3 runtimes depende só de suítes de teste unitário internas por stack, sem gate cross-CLI -->

| Via | Regra | Âncora | Severidade |
|---|---|---|---|
| **Deleção** do script | `credential_guard_hook_resolvable` (ciclo anterior) | existência + bit de execução | `error` |
| **Sobrescrita** do script | `credential_guard_script_integrity` | **template do binário** | **`warning`** |
| **Downgrade** de `credential_guard.mode` | `credential_guard_mode_downgrade` | **git `HEAD`**, direcional | `error` |

### Por que a âncora é POR ALVO

<!-- trackfw-contract: gap reason=a distinção de âncora por alvo (script→binário, mode→HEAD) é rationale de arquitetura; nenhum gate exercita o caso que discriminaria as duas (ex.: HEAD divergente do disco só para o script, provando que script_integrity de fato ignora HEAD) -->

- **Script → template do binário.** O script é **concatenação pura de constantes, sem interpolação
  por projeto** — o binário **é** a referência canônica, e não depende de `HEAD`, de commit nem de
  `$HOME`. Isto **derruba** o argumento do ADR superseded (*"integridade exige o escopo global"*)
  para este alvo.
- **`mode` → `HEAD`, comparação semântica e direcional** (`block` no `HEAD` → não-`block` no disco).
  É valor **autoral**, sem forma canônica: o binário não tem o que comparar. **Não** é byte-diff do
  `trackfw.yaml` — o arquivo muda o tempo todo por motivo legítimo, e regra ruidosa acaba desligada.

### Por que `warning` no script — e o que destravaria `error`

<!-- trackfw-contract: gate=scripts/check-gates-falsify.sh partial=a severidade default "warning" é lida do código-fonte pelo comentário do Cenário 49; os fixtures desse cenário fixam `rules: credential_guard_script_integrity: error` explicitamente "para poder usar assert_fails_with" — nenhum gate confirma que o default REAL (sem override) resolve para warning -->

O script **não carrega marcador de versão**. A regra, portanto, **não consegue distinguir** *drift*
legítimo (usuário não rodou `trackfw update` depois de um bump) de adulteração real. Afirmar
adulteração seria errado em metade dos casos, então a mensagem é **causalmente neutra** e a severidade
é `warning`.

**Embutir versão/hash no template** é o que permitiria elevar para `error` — trabalho futuro, ainda
não feito.

### 🔴 O que estas regras NÃO veem — mesmo destaque

<!-- trackfw-contract: gap reason=cada bullet é um limite de comportamento falsificável (regra não vê adversário que commita a mudança, não vê ausência de HEAD, warning não muda exit code do validate) — nenhum gate exercita esses cenários adversariais; é autodeclaração de limite, não prova -->

- **Verificam quando o `validate` roda**, não no momento da invocação do hook. Entre um `validate`
  verde e a próxima chamada de ferramenta, tudo pode mudar.
- **Adversário que commita a alteração** não é detectado por `mode_downgrade` — o `HEAD` passa a
  conter a mudança. O que resta é o **rastro auditável** (o diff existe, aparece no PR).
- **Sem `HEAD` não há comparação:** repositório sem commits, arquivo não versionado. A regra fica em
  **silêncio** — decisão deliberada, e a convenção deste projeto (não violar por ausência).
- **`warning` não muda o exit code** do `validate`. Em CI, o `validate` **passa** com a divergência do
  script; o sinal está no output, não no código de saída. Quem quiser que falhe precisa configurar
  `rules: credential_guard_script_integrity: error`.
- **Nada disso é prevenção.** Foi **medido** que não há prevenção técnica possível, no escopo do
  trackfw, contra um agente induzido com escrita irrestrita ao workspace.

### 🔴 Dois limites descobertos na revisão final (ML-3B), ainda abertos

<!-- trackfw-contract: gap reason=os dois limites (auto-silenciamento da regra do mode via edição não commitada + `rules: off` simultâneos; cobertura de deleção condicional ao wiring existente) são comportamento falsificável do trackfw sem gate ou teste que os exercite -->

1. **A regra do `mode` pode se auto-silenciar.** `ruleSeverity()` lê `rules:` **sempre do
   `trackfw.yaml` em disco**, nunca do `HEAD`. Logo, uma **única edição não commitada** que rebaixe
   `credential_guard.mode` **e** defina `credential_guard_mode_downgrade: off` derrota a detecção —
   sem commit nenhum. É mais forte que os limites já aceitos pelo ADR (que pressupunham o adversário
   **commitando**). **REQ de follow-up:** ancorar `rules:` no `HEAD` para estas regras.
2. **A cobertura de deleção é condicional ao wiring.** Se o script **e** a entrada de hook
   correspondente forem removidos **juntos**, as três regras ficam em silêncio
   (`internal/validator/validator_credential_guard.go:106-108`) — não há entrada registrada apontando
   para script ausente, então não há o que acusar.

### Cópia local do template no validador — dívida conhecida, coberta

<!-- trackfw-contract: gate=internal/validator/validator_credential_guard_integrity_external_test.go,npm/tests/credential_guard_integrity.test.js,pypi/tests/test_credential_guard_integrity.py partial=cada stack tem seu próprio teste MatchesGenerator (Go: TestCredentialGuardScriptReference_MatchesGenerator; Node: "CREDENTIAL_GUARD_SCRIPT_REFERENCE é byte-idêntico ao que generateCredentialGuardScript emite"; Python: test_credential_guard_integrity.py:82) provando que a cópia local do PRÓPRIO validador bate com o gerador REAL do PRÓPRIO stack — mas são 3 testes unitários isolados, nunca um gate cross-CLI que rode os 3 juntos e compare as 3 cópias entre si; o Cenário 48 de check-gates-falsify.sh prova que os 3 GERADORES emitem o mesmo script (via check-attention-scripts-parity.sh), o que fecha a lacuna transitivamente quando somado aos 3 testes MatchesGenerator, mas nenhum gate único afirma essa cadeia completa de uma vez -->

`internal/validator` **não pode importar** `internal/generators` (ciclo de import), então o template
existe **também** numa cópia local do validador, em cada stack. A alternativa de injeção de provider
foi rejeitada: as funções do gerador **imprimem linha de sucesso** a cada chamada, o que corromperia a
mensagem de sucesso do `validate` fixada pelo Cenário 29.

**Mitigação:** teste de paridade que executa o **gerador real** e falha se as cópias derivarem —
verificado injetando deriva no literal, com mensagem acionável e contagem de bytes. É mais forte que
o `TestCredentialGuardScript_ParityAcrossStacks` pré-existente, que **nunca executa** Node nem Python
(ver ADR Emenda 2).


## Ancoragem no `HEAD` da severidade das regras de credential-guard (ROADMAP-2026-08-12-ancorar-rules-no-head)

<!-- trackfw-contract: none reason=cabeçalho de citação ao ADR, sem alegação de comportamento própria — o conteúdo factual está nas subseções abaixo -->

> ADR (4 emendas): `docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-head-e-disco.md`

### O problema que isto fecha

<!-- trackfw-contract: none reason=narrativa histórica do problema motivador, sem alegação de comportamento de CLI atual verificável -->

As regras de credential-guard podiam ser **desligadas pela mesma edição não commitada que deveriam
denunciar** — `ruleSeverity()` lia `rules:` só do disco. Sem commit, **sem rastro**. Era pior que os
limites que o ADR de detecção aceita, onde o adversário commita e sobra o diff revisável.

### O que mudou

<!-- trackfw-contract: gate=scripts/check-gates-falsify.sh partial=os Cenários 51/52/54 (internal/validator) provam a resolução pela mais estrita entre HEAD e disco, o carve-out do baseline não suprimindo as 3 regras de guard, e o ambiente do git limpo de GIT_* — mas todos rodam só contra o CLI Go (mesmo padrão dos Cenários 47/49/50, "por que só o CLI Go"); a linha "severidade das outras ~38 regras inalterada" tem cenário próprio (53) também só-Go -->

| Aspecto | Comportamento |
|---|---|
| **Severidade** das 3 regras de guard | resolvida pela **mais estrita entre `HEAD` e disco** |
| Severidade das outras ~38 regras | **inalterada** — só disco, byte-idêntico ao anterior |
| **Baseline** (`.trackfw-baseline.json`) | **não suprime** violações das 3 regras de guard |
| Invocações de `git` do validador | ambiente do processo filho **sem nenhuma variável `GIT_*`**, ancoradas com `git -C <root>` |

As três regras: `credential_guard_hook_resolvable`, `credential_guard_script_integrity`,
`credential_guard_mode_downgrade`.

### 🔴 Consequência de migração — quebra silenciosa se não for lida

<!-- trackfw-contract: gap reason=a "quebra do nada" ao rodar `trackfw update` num projeto com baseline tolerando essas regras é comportamento falsificável — nenhum gate monta esse fixture end-to-end (baseline pré-existente + update + confirmação de violação reaparecendo) -->

Um projeto que hoje **tolera** uma violação dessas regras via `.trackfw-baseline.json` passa a ter
uma violação **não suprimível**. É **intencional** — é o carve-out funcionando — mas aparece como
*"quebrou do nada"* num `trackfw update`.

**Saída legítima e auditável:** desligar a regra com `rules:` **commitado**, ou corrigir a causa.

### Por que o filtro de ambiente é por PREFIXO, não por lista

<!-- trackfw-contract: gate=scripts/check-gates-falsify.sh partial=o Cenário 54 prova os dois vetores (GIT_DIR/GIT_WORK_TREE e GIT_CONFIG_COUNT=abc) contra o binário real, com controle autodiscriminante provando que cada vetor é genuíno — mas roda só contra o CLI Go; Node.js/Python têm filtro equivalente (npm/src/validator/git-exec.js, pypi/trackfw/validator.py) sem gate cross-CLI comparando os 3 -->

A primeira correção usou uma **denylist de 8 nomes** ("variáveis que redirecionam o repositório") e
**não fechava o problema**. O vetor real é **fazer o `git` falhar por qualquer motivo** — todo call
site trata falha como *"sem âncora, silêncio"*, ou seja, **falha aberta**.

Contraexemplo que quebrou a denylist: **`GIT_CONFIG_COUNT=abc`** — não redireciona nada, apenas faz o
`git` sair 128 por config malformada.

Qualquer lista fechada envelhece: o git ganha variáveis novas a cada versão, e **basta uma** que faça
o processo falhar. Detalhe em
`vault/notes/validador-git-env-bypass-filtre-por-prefixo-2026-08-12.md`.

### O que continua aberto

<!-- trackfw-contract: gap reason=os dois itens (governance_mode: lenient convertendo tudo em warning; ausência de HEAD caindo no disco) são limitações declaradas e falsificáveis sem gate ou teste que as exercite e confirme -->

- **`governance_mode: lenient`** converte **toda** a saída do `validate` em warning, inclusive a
  destas regras. **Não** foi fechado — *blast radius* é o validador inteiro e há caso de uso legítimo
  (onboarding, com `lenient_until`). **REQ própria.** Enquanto existir, o problema está **reduzido,
  não resolvido**.
- **Sem `HEAD` não há âncora** — repositório sem commits, `trackfw.yaml` não versionado: a resolução
  cai no disco e o buraco existe.
- **Fora do validador**, o mesmo padrão de `exec.Command("git", ...)` sem ambiente limpo existe em
  `internal/forge/resolve.go`, `internal/commands/branch.go`, `internal/commands/ship.go`. **Não são
  controles de segurança**, e não foram alterados.

## Git branch guard por runtime (ML-1A, ROADMAP-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md)

<!-- trackfw-contract: gate=scripts/check-harness-hooks-parity.sh,scripts/check-agent-hooks-parity.sh partial=GUARDS="credential-guard git-branch-guard" prova paridade estrutural do wiring project-scope (discover --init) nos 8 CLIs nativos, incluindo Windsurf e Amazon Q desde ROADMAP-2026-08-20/ML-1B (claude/codex/gemini/cursor/copilot/kiro/windsurf/amazonq via check-agent-hooks-parity.sh), e do wiring global/harness-scope (update harness) nos 6 CLIs que têm target de harness (claude/codex/gemini/cursor/copilot/kiro via check-harness-hooks-parity.sh) — Windsurf não tem par de targets de harness por impossibilidade estrutural: não tem mecanismo de hook global nativo (decisão registrada no próprio comentário de harnessCatalogTargetOrder, internal/generators/update.go) e nunca terá — sem artefato global para gatear. Amazon Q não tem par de targets de harness por pendência de implementação: caminhos ~/.aws/amazonq/ existem no catálogo (catalog.json linha 44), mas os harness targets nunca foram implementados nos 3 CLIs — a ausência não é permanente (grep confirma zero ocorrências de "windsurf"/"amazonq" pareadas com credential-guard/git-branch-guard em HarnessTargetIDs/buildHarnessTargetIDs nos 3 CLIs) -->

> REQ: `docs/req/REQ-2026-08-14-bloqueio-tecnico-de-comandos-git-brutos-por-subagente-via-deny-hooks-nos-7-runtimes-suportados.md`

Bloqueia tecnicamente `git commit`/`git push`/`git checkout -b` brutos (não via `trackfw
commit`/`trackfw ship`/`trackfw branch new`) por subagente. Mesmo padrão do credential-guard: um
script canônico Go (`internal/generators/scaffold.go`, const `gitBranchGuardScript` +
`GenerateGitBranchGuardScript`/`GenerateGlobalGitBranchGuardScript`) escreve
`scripts/trackfw-git-branch-guard.sh` (projeto) / `~/.trackfw/scripts/trackfw-git-branch-guard.sh`
(global). **Estado final (Wave 1-4 concluídas):** o script existe, é gerado por `trackfw init`/
`trackfw update`/`trackfw update harness` nos 3 CLIs, e está ligado (wiring de hook/deny real) nos 7
runtimes.

| Runtime | Mecanismo | Isolamento do arquiteto |
|---|---|---|
| Claude Code | hook `PreToolUse` | **deny global** (decisão 2026-08-14, ver abaixo) |
| Codex CLI | hook `PreToolUse`/`Bash` (mecanismo estável usado, não Rules) | deny global |
| Gemini CLI | hook `PreToolUse`/`BeforeTool` (exit 2) | deny global |
| GitHub Copilot | `--deny-tool` granular | deny global |
| Cursor | hook `beforeShellExecution` (via exit code 2, sem camada de deny estático adicional) | deny global |
| Windsurf | hook `pre_run_command` em `.windsurf/hooks.json` | deny global |
| Amazon Q Developer | hook `preToolUse` + `deniedCommands` regex em `.amazonq/cli-agents/q_cli_default.json` | deny global |

**Decisão sobre isolamento do arquiteto (2026-08-14, usuário, via AskUserQuestion pós-Wave 3):** a
diferenciação "arquiteto livre, especialista bloqueado" **não foi implementada em nenhum runtime**
nesta REQ — o deny é global para todos os agentes, inclusive o arquiteto/orquestrador (Zeus ou
equivalente), em todos os 7 runtimes. Motivo: (a) nenhum runtime tinha essa diferenciação de fato
implementada quando checado — mesmo os 3 runtimes com suporte nativo a subagentes (Claude via hook
`subagent_name`, Gemini via subagentes nativos, Amazon Q via custom agents) exigiriam trabalho
adicional fora do escopo desta REQ; (b) coerente com a regra já existente no CLAUDE.md de que o
arquiteto não deveria commitar código de implementação mesmo — ele passa a usar `trackfw commit`/
`trackfw branch new`/`trackfw ship` como qualquer outro agente. Confirmado ao vivo nesta sessão: o
próprio arquiteto (Zeus) foi bloqueado tentando `git commit` bruto neste mesmo repositório após
`trackfw update` regenerar os hooks locais. Se o isolamento por subagente vier a ser necessário, é
uma REQ nova.

### Contrato de payload do script (`gitBranchGuardScript`)

<!-- trackfw-contract: gate=scripts/check-attention-scripts-parity.sh,scripts/check-gates-falsify.sh partial=check-attention-scripts-parity.sh prova a byte-identidade do script entre os 3 CLIs; os Cenários 60-69/74 de check-gates-falsify.sh exercitam os 3 formatos de entrada e o padrão casado contra esse script canônico único, mas nenhum cenário testa especificamente o campo `tool_info.command_line` (formato Windsurf) nem `$TRACKFW_GIT_COMMAND` como fallback -->

O script suporta 3 formatos de entrada, nesta ordem de precedência — cobre os contratos divergentes
dos 7 runtimes sem precisar de uma variante de script por runtime:

1. **Argumentos de linha de comando** (`$1..$N`) — o comando git cru passado como argv.
2. **Payload JSON via stdin** — tenta `.tool_input.command`, `.command` e `.hook_input.command`,
   nessa ordem; usa `jq` quando disponível, com fallback grep/sed (sem exigir `jq` no PATH, mesmo
   espírito de `credentialGuardDetectionCore`).
3. **Texto cru via stdin**, ou `$TRACKFW_GIT_COMMAND` como último fallback.

Padrão casado: `^git (commit|push|checkout -b|switch -c)\b`, aceitando flags globais antes do
subcomando (`git -C . commit`, `git --no-pager push`). `switch -c`/`-C`/`--create` (a forma
alternativa a `checkout -b` para criar branch — item 2 do
ROADMAP-2026-08-16-higiene-sete-debitos-acumulados-da-entrega-de-plugins-e-da-release-7-0-0.md,
ML-1A) é reconhecida varrendo **todos** os tokens após o subcomando `switch`, não só o primeiro,
cobrindo `git switch --track -c feat/x`. Sem match: allow silencioso (`exit 0`, sem output).

Com match, o script emite **os dois formatos de decisão simultaneamente** — `{"decision":"block",
"reason":"..."}` no stdout (consumido por Claude/Gemini) **e** `exit 2` (consumido por
Codex/Windsurf/Cursor por exit-code) — em vez de uma variante por runtime dentro do script. Essa é
uma simplificação deliberada do ML-1A: o formato `permission: "deny"` específico do Cursor, se
necessário, fica a cargo do wiring da Wave 3 em cima da mesma saída, não deste script.

Mensagem de bloqueio por subcomando (todas referenciam CLAUDE.md §1):

| Subcomando bloqueado | Orientação |
|---|---|
| `checkout -b` | `trackfw branch new <type>/<slug>` |
| `switch -c`/`-C`/`--create` | `trackfw branch new <type>/<slug>` |
| `commit` | `trackfw commit -m '<mensagem>'` (comando novo, Wave 2 deste roadmap) |
| `push` | `trackfw ship` |

### Caminhos confirmados — Windsurf e Amazon Q (apolo-tf, 2026-08-14, correção pós-auditoria do ML-3A)

<!-- trackfw-contract: gate=scripts/check-agent-hooks-parity.sh partial=prova paridade estrutural (identidade semântica via diff JSON recursivo, não byte-idêntica — ver seção "Campos mínimos do custom agent Amazon Q" logo abaixo) dos 2 arquivos de project-scope entre Go/Node.js/Python desde ROADMAP-2026-08-20/ML-1B; check-harness-hooks-parity.sh não se aplica a estes caminhos (arquivos de project-scope, não de ~/.<tool> global-scope) e nunca poderia comparar algo que não existe nesse escopo para nenhum dos dois CLIs -->

A primeira implementação do wiring (ML-3A) escreveu caminhos/formatos **inventados** para Windsurf e
Amazon Q, sinalizados no próprio comentário de código como não confirmados contra documentação
oficial. Uma verificação posterior confirmou que ambos estavam estruturalmente errados — corrigido
nos 3 CLIs (Go `internal/generators/agentfiles.go`, Node `npm/src/generators/hooks.js`, Python
`pypi/trackfw/generators/hooks.py`), com paridade estrutural confirmada via
`check-agent-hooks-parity.sh` (ROADMAP-2026-08-20/ML-1B — a menção anterior a
`check-harness-hooks-parity.sh` nesta frase era falsa e nunca poderia ter sido verdadeira: os dois
caminhos aqui são de project-scope, e check-harness-hooks-parity.sh só compara arquivos de
global-scope em `~/.<tool>/...`, que não existem para Windsurf/Amazon Q — ver a nota na seção "Git
branch guard por runtime" acima).

| Runtime | Caminho errado (ML-3A original) | Caminho correto (confirmado) |
|---|---|---|
| Windsurf | `.windsurf/hooks/trackfw-git-branch-guard.json` (arquivo dedicado, schema `{"version":1,"hooks":[{"name":...,"trigger":...,"action":{...}}]}`) | `.windsurf/hooks.json` (arquivo único, schema `{"hooks":{"pre_run_command":[{"command":"...","show_output":bool}]}}` — merge idempotente no array do evento) — ver https://docs.devin.ai/desktop/cascade/hooks |
| Amazon Q Developer | `.amazonq/settings.json` (`hooks`/`toolsSettings` na raiz) | `.amazonq/cli-agents/q_cli_default.json` (arquivo de **custom agent** nomeado — `hooks`/`toolsSettings` são campos de nível superior de um agent, não de um settings.json compartilhado) — ver https://docs.aws.amazon.com/amazonq/latest/qdeveloper-ug/command-line-custom-agents-configuration.html |

O nome de arquivo `q_cli_default.json` foi escolhido (em vez de um nome arbitrário) por ser a
convenção mais próxima de "ativação automática sem flag manual" para um custom agent do Amazon Q CLI
— com a ressalva de que há um bug conhecido e não contornado
(github.com/aws/amazon-q-developer-cli#2922) onde esse override por nome de arquivo nem sempre é
respeitado pela CLI.

O contrato de payload do `gitBranchGuardScript` (seção acima) também foi ajustado: o campo
`tool_info.command_line` (formato real do payload `pre_run_command` do Windsurf) foi adicionado à
cadeia de tentativas de extração do comando via stdin JSON, ao lado dos campos genéricos já
existentes (`.tool_input.command`, `.command`, `.hook_input.command`).

### Campos mínimos do custom agent Amazon Q — Go como canônico (2026-08-20, ML-1A-bis)

<!-- trackfw-contract: gap reason=a decisão de alinhar Node/Python ao conjunto mínimo do Go é por assimetria de risco, não por verificação contra a doc viva da AWS ou um `q chat --agent` real; nenhum gate cross-CLI confirma o schema real do Amazon Q, só a byte-identidade entre os 3 CLIs (ML-1B) -->

Investigação do ML-1A (roadmap `ROADMAP-2026-08-20-gates-para-os-tres-contratos-de-maior-risco.md`)
mediu uma 6ª divergência real entre os 3 CLIs: Node e Python escreviam, na criação do
`.amazonq/cli-agents/q_cli_default.json`, 6 campos que o Go **deliberadamente omite** (doc comment
de `InjectAmazonQHooks`, `internal/generators/agentfiles.go`) — `prompt`, `mcpServers`,
`toolAliases`, `allowedTools`, `resources`, `useLegacyMcpJson`.

**Decisão: o Go é o canônico.** Node (`npm/src/generators/hooks.js`) e Python
(`pypi/trackfw/generators/hooks.py`) foram alinhados a ele — o conjunto mínimo escrito na criação
passou a ser só `name`, `description` e `tools`. O motivo é assimetria de risco, citada no próprio
comentário do Go: um campo extra que o schema real não espera arrisca falhar a validação do agente;
um campo opcional ausente normalmente não. Entre as duas, escrever de menos é o lado seguro.

**Limite explícito, para não virar "confirmado" por engano:** essa escolha **não** foi verificada
contra a documentação viva da AWS (`command-line-custom-agents-configuration.html`) nem contra um
`q chat --agent` real — é a mesma ressalva que o comentário do Go já carregava ("verify this
defaults set against the live doc ... before treating it as final") e que segue sem ser feita. O
que ML-1A-bis resolve é a **divergência entre os 3 CLIs**, não a **correção contra o schema real do
Amazon Q**.

**Contrato de merge preservado:** o alinhamento é só sobre o que uma instalação **nova** cria. Um
arquivo `q_cli_default.json` pré-existente com qualquer um dos 6 campos (ex.: `mcpServers`
configurado manualmente pelo usuário) não é tocado — os 3 injectors usam
merge-só-se-ausente (`setdefault`/`hasOwnProperty`/`_, exists := root[k]`), então nenhum campo já
presente é removido por este lote.

### Fix de robustez do `match_subcommand` (2026-08-14, ML-4A, achado por teste manual E2E)

<!-- trackfw-contract: gate=scripts/check-gates-falsify.sh partial=os 3 bugs foram achados por teste manual E2E, não por gate automatizado; o bug 1 (comando encadeado) é superado e coberto indiretamente pelos Cenários 60/61 (segmentação quote-aware); grep confirma que não há cenário dedicado para o bug 2 (basename vs caminho absoluto, ex. `/usr/bin/git commit`) -->

O teste manual end-to-end (via subagente `apolo-tf`) revelou 3 bugs reais na lógica de detecção,
todos com a mesma causa raiz — o script fazia busca de padrão livre na string inteira do comando,
sem respeitar limites reais de segmento (`;`, `&&`, `||`, `|`, quebra de linha) nem exigir que `git`
fosse o primeiro token de um segmento:

1. **Falso negativo, comando encadeado**: `git status; git push origin HEAD` não bloqueava o `push`.
2. **Falso negativo, path absoluto**: `/usr/bin/git commit` não bloqueava (comparação de igualdade
   exata, sem normalizar por basename).
3. **Falso positivo crítico**: `trackfw commit -m "..."` era bloqueado sempre que a mensagem
   mencionasse "git commit"/"git push" em qualquer lugar da string — reproduzido ao vivo nesta
   sessão contra o próprio arquiteto.

Fix (byte-idêntico nos 3 CLIs): `match_subcommand` divide o comando em segmentos reais e exige que o
**basename** do primeiro token de cada segmento seja `git` antes de inspecionar o subcomando. Um bug
adicional foi descoberto durante o port para Node/Python: usar `| while read` (pipe, cria subshell)
faz `return`/`exit` dentro do loop não propagar para a função chamadora — corrigido trocando para o
padrão heredoc (`done <<EOF...EOF`) que o Go já usava, evitando o subshell. Ver vault notes
`git-branch-guard-pipe-into-while-loses-return-status-2026-08-14.md` e
`git-branch-guard-self-blocking-quote-unaware-splitter-2026-08-14.md`.

### Segmentação quote-aware — resolve o falso-positivo de linha de mensagem (ML-1A, 2026-08-16)

<!-- trackfw-contract: gate=scripts/check-gates-falsify.sh -->

A limitação acima **foi resolvida**: `match_subcommand` passou a segmentar o comando através de
`quote_aware_split` (scanner char-a-char em `awk`) + `strip_heredoc_bodies`, em vez do `sed` cego
por separador. `;`/`&&`/`||`/`|`/quebra-de-linha deixam de ser tratados como separador de comando
**enquanto dentro de uma string entre aspas simples ou duplas**, e o corpo de blocos heredoc é
removido antes da segmentação — cobrindo exatamente o caso antes documentado como limitação (linha
de mensagem de commit, ou tabela em texto, que **começa** com `git <sub>` depois de uma quebra de
linha real dentro de uma string citada ou de um heredoc). Ver
`vault/notes/git-branch-guard-quote-aware-segmentation-2026-08-16.md` e
`vault/notes/git-branch-guard-falso-positivo-em-linha-de-mensagem-de-commit-2026-08-16.md`.

**O que continua garantido, não regredido:** aspas não fechadas até o fim da entrada nunca
"vazam" o texto seguinte como comando novo (mesma semântica do shell real); um heredoc mal-formado
(sem linha terminadora) devolve o texto original intocado, preservando a segmentação de linha real
— lado seguro, mais restritivo é preferível a esconder um comando atrás de um heredoc quebrado;
um separador real **fora** de aspas, mesmo logo após uma aspa de fechamento (`git commit -m "x";
git push`), continua sendo tratado como separador e o `push` continua bloqueado.

Cenários de falsificação novos (baseline + detecção) provando os dois lados — falso-positivo
eliminado e bloqueio real preservado — estão nos Cenários 60/61 de `scripts/check-gates-falsify.sh`.
Nenhum runtime tem garantia hermética de shell completo (o parser continua sendo um scanner, não um
interpretador de shell); essa ressalva geral permanece registrada na REQ vinculada.

### Gate de paridade do `trackfw commit` (`scripts/check-commit-parity.sh`, ML-4A)

<!-- trackfw-contract: gate=scripts/check-commit-parity.sh -->

Criado para fechar a lacuna que só `check-branch-new-parity.sh` cobria antes — verifica que as
mensagens de bloqueio/aviso do novo comando `trackfw commit` são byte-idênticas entre os 3 CLIs em 3
cenários (commit em `main`/`master`, commit em `feat/` sem roadmap em `wip/`, commit em branch
não-governada). Registrado em `make quality`/`parity`. Encontrou um bug real de ordenação de output
no Python (`pypi/trackfw/commands/commit.py`): `sys.stdout.write()` sem `flush()` antes de um
`subprocess.run` com stdio herdado inverte a ordem da saída quando stdout não é um TTY — corrigido
com `out.flush()`.

### Bloqueio da classe destrutiva de working tree + mensagem de raio de alcance (ML-3A, ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md / REQ-2026-08-19-guard-nao-bloqueia-comandos-destrutivos-de-working-tree-em-repo-compartilhado-por-agentes.md)

<!-- trackfw-contract: gate=scripts/check-gates-falsify.sh partial=Cenário 74 confirma par baseline+detecção por comando para todas as linhas da tabela de bloqueio/liberação (stash, reset --hard, clean -f, restore, checkout --/.); a alegação de "raio de alcance" (nada antes do comando bloqueado chega a executar em comando composto) não tem cenário dedicado provando ausência de efeito colateral — é propriedade do mecanismo de hook do runtime (inspeção antes da execução), não testada isoladamente por este gate -->

> REQ: `docs/req/REQ-2026-08-19-guard-nao-bloqueia-comandos-destrutivos-de-working-tree-em-repo-compartilhado-por-agentes.md`

`git worktree list` confirma que subagentes despachados em paralelo pelo mesmo agente
orquestrador compartilham **um único worktree físico** — um `git stash`/`git reset --hard`/`git
clean -f` disparado por um agente afeta o trabalho não commitado de **todos** os outros. O guard
(mesmo script canônico `gitBranchGuardScript`, seção acima) passou a reconhecer também essa classe,
com o mesmo padrão de precedência de casamento por segmento (`match_subcommand`).

**Bloqueado**, mensagem sempre nomeando a alternativa segura:

| Subcomando bloqueado | Discriminante | Alternativa citada na mensagem |
|---|---|---|
| `git stash` (bare) / `stash push` / `stash save` | qualquer subcomando de stash exceto `list`/`show` | `git stash list`/`git stash show` (leitura) + `trackfw branch new` para guardar trabalho em progresso |
| `git stash clear` / `stash drop` | idem | idem |
| `git reset --hard` | `--hard` em qualquer posição de token | `git reset --soft`/`--mixed` (`git reset --soft HEAD~1` é o contorno padrão do `trackfw ship`) |
| `git clean -f`/`-fd`/`-x`/`-X`/`--force` | qualquer token de força, exceto quando `-n`/`--dry-run` também presente | `git clean -n`/`--dry-run` |
| `git restore <path>` sem `--staged`, **ou** `git restore --staged --worktree <path>` (ou `-W`) | argumento posicional presente e (`--staged` ausente **ou** `--worktree`/`-W` presente) | `git restore --staged` **sozinho** (não toca o working tree) |
| `git checkout -- <path>` / `git checkout .` | `--` em qualquer posição de token, ou `.` como token isolado | `git checkout <branch>`/`git switch <branch>` |

**Liberado, verificado por cenário (o risco dominante é super-bloquear, não sub-bloquear — o
próprio guard já registra esse julgamento na regra de `git branch`):**

- `git stash list` / `git stash show` (leitura).
- `git reset` **sem** `--hard` — inclui `--soft` e `--mixed` (inclusive sem flag, que é `--mixed`
  implícito). `git reset --soft HEAD~1` é o contorno padrão para reempurrar trabalho já commitado
  via `trackfw ship`, então bloquear `reset` inteiro inviabilizaria o próprio trilho governado.
- `git clean -n` / `--dry-run` (nunca apaga nada) — inclusive quando `-n` aparece **junto** com um
  token de força (`git clean -n -f`): `-n` vence, git nunca apaga nada com dry-run presente, e o
  discriminante é testado com essa combinação, não só `-n` isolado.
- `git restore --staged <path>` **sozinho** (sem `--worktree`/`-W`) — mexe só no index, nunca no
  working tree. `--staged --worktree`/`-W` juntos restauram **os dois**, então continuam bloqueados
  mesmo com `--staged` presente — a REQ licencia só "`--staged` sozinho", não "qualquer `--staged`".
- `git checkout <branch>` / `git switch <branch>` — distinguir nome de branch de caminho sem `--`
  é genuinamente ambíguo; adivinhar produziria falso-positivo, então só a forma explícita de
  caminho (`--`/`.`) bloqueia. **Declarado, não fechado:** `git checkout ./src/foo.go` (caminho sem
  `--` nem `.` sozinho) também fica livre por essa mesma decisão — é a mesma ambiguidade
  branch-vs-caminho que a REQ pede para não adivinhar, não uma lacuna nova.

**Mensagem de raio de alcance:** o guard inspeciona a string do comando inteiro antes de executar
qualquer parte dele, então um comando composto (`cat > f <<EOF ... EOF && git commit ...`) é
recusado **por inteiro antes de qualquer parte rodar** — nenhum efeito colateral do que veio antes
do comando bloqueado chega a acontecer. Toda mensagem de recusa (das classes acima e das
pré-existentes de `checkout -b`/`switch -c`/`branch`/`worktree add -b`/`commit`/`push`) agora
declara explicitamente essa propriedade ("Nada antes deste comando foi executado (comando composto
é bloqueado por inteiro)"). A mensagem de `push` passa também a citar `trackfw release tag` ao lado
de `trackfw ship`, como caminho governado alternativo (Wave 2 do mesmo roadmap).

Evasões já conhecidas do guard (prefixo `env`/`command`, flag fora da primeira posição de token,
`git${IFS}stash`) continuam cobertas para os subcomandos novos pelo mesmo `match_subcommand` — não
é uma checagem separada por classe.

**Regra de `stash` é deny-by-default, declarado:** só `list`/`show` estão na allowlist de leitura —
`stash pop`/`apply`/`branch`/`create`/`store` **também bloqueiam**, não só `push`/`save`/`clear`/
`drop`. A própria REQ nomeia `stash pop` como o caminho de recuperação de um `stash` já feito; a
decisão aqui foi bloquear a classe inteira por padrão em vez de abrir uma allowlist maior — se isso
for restritivo demais na prática, é ajuste de allowlist num ML futuro, não reabertura desta REQ.

**Gate de prova negativa (P4):** Cenário 74 de `scripts/check-gates-falsify.sh` — um par
baseline+detecção **por comando**, cobrindo as duas direções: o comando bloqueado escapando
(rótulo do `case` corrompido) e o comando liberado sendo capturado por engano (discriminante de
liberação corrompido ou alargado para casar qualquer token, ex.: `--hard` virando um curinga que
faria `git reset --soft` bloquear incorretamente). `git reset --soft`/`--mixed`, `git stash
list`/`show`, `git clean -n`, `git restore --staged` e `git checkout <branch>` são provados livres
tanto antes quanto — onde corrompidos — expostos como dependentes do literal exato, não de uma
lacuna acidental.

### `update-ref`/`worktree remove --force`/`git rm -f` entram no bloqueio destrutivo (ML-4B, ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md, corretivo do veredito BLOQUEAR do `hades-tf`)

<!-- trackfw-contract: gate=scripts/check-release-tag-parity.sh partial=Cenários 11-14 provam a origem do commit-alvo; a própria seção declara "não fechado" para os 3 comandos novos: nenhum par baseline+detecção dedicado em check-gates-falsify.sh para update-ref/worktree remove --force/rm -f no padrão do Cenário 74 — a literal do script está correta (verificado por testes de integridade existentes), mas sem prova de falsificação por comando -->

> ADR: `docs/adr/ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md`, Emenda 1

`git update-ref` foi o **mecanismo** que tornou alcançável o exploit descrito na Emenda 1 do ADR
(forjar `refs/remotes/origin/<base>` localmente, sem push, para desviar o commit-alvo de
`trackfw release tag`) — a correção do próprio `release tag` (ancorar o commit-alvo no forge)
fecha esse desvio para esse comando específico, mas o guard também passou a bloquear
`update-ref` na origem, já que não há forma de leitura equivalente a liberar seletivamente
(a subcommand inteira é escrita).

`git worktree remove --force`/`-f` e `git rm -f`/`--force` entram pela mesma classe já
estabelecida em ML-3A (destrutivo, irreversível, worktree compartilhado entre subagentes):

| Subcomando bloqueado | Discriminante | Alternativa citada na mensagem |
|---|---|---|
| `git update-ref` | qualquer forma (sem exceção — sem forma de leitura a liberar) | `trackfw release tag` (o commit-alvo agora vem do forge, não de refs locais) |
| `git worktree remove -f`/`--force` | token de força em qualquer posição | `git worktree remove` sem force (recusa sozinho quando há algo não commitado) |
| `git rm -f`/`--force` | token de força em qualquer posição; sem carve-out para `--cached` (destrancar do index sem `-f` já é liberado por não precisar de force) | mesma classe de `git clean -f`/`git reset --hard` já bloqueados |

**Gate:** `scripts/check-release-tag-parity.sh` ganhou 4 cenários (11-14) sobre a origem do alvo — os 11-13 adversariais e o 14 cobrindo a ausência da ref de tracking local — provando
que a seleção do commit-alvo é ancorada no forge e recusa nomeando a divergência quando um ref
local (symref repontado, `origin/<base>` forjado via `update-ref` sob refspec estreitado, ou
`remote.origin.fetch` estreitado deixando o ref local desatualizado) diverge do forge —
sabotados de propósito pelo Cenário 76 de `scripts/check-gates-falsify.sh`. **Declarado, não
fechado:** este ML não adicionou pares baseline+detecção dedicados no próprio
`check-gates-falsify.sh` para `update-ref`/`worktree remove --force`/`rm -f` (o padrão que o
Cenário 74 estabeleceu para `stash`/`reset --hard`/`clean -f`/`restore`/`checkout --`) — a
literal do script está correta (byte-idêntica entre `gitBranchGuardScript`,
`gitBranchGuardScriptReference`, `GIT_BRANCH_GUARD_SCRIPT`/`GIT_BRANCH_GUARD_SCRIPT_REFERENCE`
em Node e `_GIT_BRANCH_GUARD_SH`/`_GIT_BRANCH_GUARD_SCRIPT_REFERENCE` em Python, verificado pelos
testes de integridade existentes), mas a prova de falsificação por comando fica para um ML
futuro se a cobertura do Cenário 74 for considerada insuficiente para esta classe.

## `trackfw <skills|agents> third-party` — instalação de skills de terceiro via URL (ADR-2026-08-15, ML-3A/ML-3B)

<!-- trackfw-contract: none reason=cabeçalho de citação ao ADR, sem alegação de comportamento própria — o conteúdo factual está nas subseções abaixo -->

### O comando, em duas fases

<!-- trackfw-contract: gate=scripts/check-thirdparty-parity.sh -->

`third-party` nunca instala em um passo. O fluxo é deliberadamente quebrado em duas invocações
separadas para forçar um ponto de revisão humana entre "buscar da rede" e "gravar no sistema de
arquivos do projeto/agente":

1. **`trackfw <skills|agents> third-party fetch <url> [--slug <slug>]`** — busca o conteúdo bruto (raw) da URL,
   roda o checker de markers (ver D3 abaixo) e grava um registro de **quarentena** em
   `.trackfw/thirdparty-quarantine/<checksum_sha256>.json`, onde `checksum_sha256` é o hash SHA-256
   dos bytes **brutos** recebidos da rede, antes de qualquer normalização (D6). Nada é instalado
   nesta fase. `fetch` é o único subcomando que efetivamente toca a rede.
2. **Aprovação externa (fora do CLI, D10.2)** — nenhum subcomando de `third-party` escreve em
   `.trackfw/thirdparty-provenance.json`. Um aprovador humano ou um agente de revisão dedicado
   (ex.: `hades-tf`) inspeciona o conteúdo em quarentena e, se aprovar, grava a entrada de
   provenance diretamente, keyed pelo destino **relativo à raiz do projeto** (não pelo destino
   absoluto do manifest — ver nota de paridade abaixo).
3. **`trackfw <skills|agents> third-party install --checksum <sha256> --targets <t1,t2> [--apply-to <agentes>]`** —
   consome o registro de quarentena pelo checksum informado, verifica a aprovação
   (`VerifyApproval`) contra `.trackfw/thirdparty-provenance.json`, normaliza o conteúdo
   (`NormalizeThirdPartyContent` = `TrimSpace(raw) + "\n"`) e só então grava o artefato de skill no
   destino resolvido, registrando um `Claim` no manifest com `origin: "thirdparty"` (D11) e,
   quando `--apply-to` for usado, injetando uma referência no(s) arquivo(s) de agente indicado(s)
   (rastreada em `.trackfw/thirdparty-references.json`, D9 schema 3, para sobreviver a
   `agents update`).

**`fetch` nunca escreve fora de `.trackfw/thirdparty-quarantine/`; `install` nunca faz fetch de
rede.** Essa separação é a barreira de revisão — ver "Limitação do critério de markers (D3)" abaixo
para o que ela NÃO garante.

### Os 3 schemas JSON (D9)

<!-- trackfw-contract: gate=scripts/check-thirdparty-parity.sh -->

| Schema | Caminho | Keyed por | Escrito por |
|---|---|---|---|
| Quarentena | `.trackfw/thirdparty-quarantine/<checksum_sha256>.json` | nome de arquivo = checksum SHA-256 dos bytes brutos | `third-party fetch` |
| Provenance | `.trackfw/thirdparty-provenance.json` (`entries: {...}`, `schema_version: 2`) | destino **relativo à raiz do projeto** (não o destino absoluto usado nas chaves do `integrations-manifest.json`) | `checksum_sha256`/`approved_by`/etc: aprovador externo (D10.2). `installed_sha256`: `third-party install`, ver abaixo (D2-bis) |
| References | `.trackfw/thirdparty-references.json` | id do agente-alvo → lista de skills de terceiro referenciadas | `third-party install --apply-to` |

Os 3 schemas são cobertos byte-idênticos entre Go/Node/Python por `scripts/check-thirdparty-parity.sh`
(Parte B), incluindo o round-trip completo via `third-party install` real (não apenas fixtures
hand-authored).

**Nota de paridade crítica — domínio de chave absoluto vs. relativo:** o `integrations-manifest.json`
usa como chave o destino **absoluto** resolvido (`Manager.resolve()` em Go e equivalentes em
Node/Python). Os 3 schemas de `.trackfw/thirdparty-*` usam o destino **relativo à raiz do projeto**
(o valor pré-`resolve()`). Uma implementação inicial que usasse o destino absoluto do manifest como
chave de busca em `thirdparty-provenance.json` nunca encontraria a entrada — falso-positivo
sistemático do lado (i) da regra abaixo. Confirmado empiricamente contra o comando real (não por
inspeção do ADR), nos 3 CLIs.

### Escopo default diverge do catálogo (exceção D4)

<!-- trackfw-contract: gate=scripts/check-thirdparty-parity.sh -->

Ao contrário dos itens de catálogo (agentes, skills nativas), cujo escopo default é `global`,
`third-party install` usa escopo default **`project`** (ADR-2026-07-25 D1 mantém `global` como
default de catálogo; `third-party` é uma exceção documentada, não uma divergência). Motivo: um
artefato de terceiro instalado por engano em escopo `global` vazaria para todos os projetos que
compartilham o home do usuário, ampliando o raio de um erro de revisão. `--scope` continua
aceitando `global` explicitamente quando desejado.

### D4-bis — `--scope global` continua permitido, mas com aviso e confirmação próprios (ML-4C)

<!-- trackfw-contract: gap reason=grep confirma zero ocorrência de "scope global"/"yes-global-scope-unverified" em check-thirdparty-parity.sh — o aviso, a mensagem de recusa e a flag `--yes-global-scope-unverified` não têm nenhum gate cross-CLI -->

Achado do `hades-tf`, decisão de KG (2026-08-15): `--scope global` tira o artefato do perímetro de
`thirdparty_artifact_has_provenance` — a regra só lê o manifest do **projeto**
(`validator_thirdparty_provenance.go:99`), e um artefato em `~/` não está no git, então não há o que
detectar (coerente com ADR-2026-08-12). Isso **não** deixa de ser permitido, mas deixa de ser
silencioso:

- `install` imprime, sempre que `--scope global` for resolvido (antes de qualquer outra saída), um
  aviso citando explicitamente que a instalação **nunca será verificada por `trackfw validate`**.
- A confirmação para `--scope global` deixa de colapsar em `--yes-i-trust-this-source` (que já é
  obrigatória em modo não-interativo, ver AC1) — nova flag própria,
  **`--yes-global-scope-unverified`**, é exigida adicionalmente. As duas flags têm propósitos
  distintos: uma confirma confiança na origem do conteúdo, a outra confirma que o usuário entende que
  esta instalação específica nunca passará por `trackfw validate`.
- Aviso e mensagem de recusa são **texto idêntico** nos 3 CLIs (constantes dedicadas em cada porte,
  nunca inline, para não divergirem silenciosamente em uma edição futura).

### `trackfw validate` — regra `thirdparty_artifact_has_provenance` (D2)

<!-- trackfw-contract: gate=scripts/check-thirdparty-parity.sh partial=a branch (i) (violação por Claim.origin sem entrada de provenance) tem mensagem comparada byte-a-byte pela Parte C; a branch (ii) tem o caminho de não-falso-positivo coberto (D2-bis, instalação legítima produz zero violações nos 3 CLIs) — mas a DETECÇÃO real da branch (ii) (installed_sha256 divergente após adulterar o arquivo instalado) não é exercitada cross-CLI: grep não encontra fixture de tamper/append no script; a cobertura desse caminho é só testes unitários por stack (branch_ii_quarantine_deletion_still_detects_tamper e equivalentes), nunca comparados entre os 3 CLIs via gate -->

Bidirecional, e **nunca faz fetch de rede** (D6 — a regra só lê `.trackfw/` e os artefatos já
instalados no disco):

- **Branch (i)** — todo destino cujo `Claim.origin == "thirdparty"` no manifest precisa ter uma
  entrada correspondente em `thirdparty-provenance.json` (chave = destino relativo à raiz). Faltando,
  é violação (mensagem inclui "D2 branch i" e o destino absoluto, para localização rápida).
- **Branch (ii)** — toda entrada de provenance é verificada contra o conteúdo real instalado,
  comparando `sha256(arquivo instalado)` diretamente contra `entry.installed_sha256` (ver "Dois
  hashes, dois domínios" abaixo). Se `installed_sha256` não bater (ou estiver ausente/vazio, o que
  nunca combina com o hash de um arquivo real), é violação. A regra **não lê mais o registro de
  quarentena** — sua ausência não é mais considerada erro por esta ramificação (ver D2-bis abaixo).

#### D2-bis — dois hashes, dois domínios (`schema_version: 2`, `installed_sha256`)

<!-- trackfw-contract: gate=scripts/check-thirdparty-parity.sh -->

A entrada de proveniência carrega **dois** hashes SHA-256, em domínios diferentes, e confundi-los
foi um bug real encontrado na auditoria do ML-3A:

| Campo | Domínio | Quando é calculado | Quem escreve | Para que serve |
|---|---|---|---|---|
| `checksum_sha256` | bytes **brutos** buscados da rede, antes de qualquer normalização (D6) | no `fetch` | aprovador externo (D10.2), como âncora do que ele efetivamente revisou | vínculo de aprovação D8c/TOCTOU (`verifyApproval`) |
| `installed_sha256` | bytes **normalizados** (`normalize_third_party_content` = `TrimSpace(raw) + "\n"`) | no `install`, pelo mesmo código que grava o arquivo de destino | `third-party install` (único escritor de código deste campo) | detecção de adulteração pós-install / branch (ii) de `thirdparty_artifact_has_provenance` |

**Por que dois campos, e não um:** o arquivo instalado é **sempre** o conteúdo normalizado, nunca o
bruto — então `sha256(arquivo instalado)` só pode ser comparado de forma correta contra um hash que
também esteja no domínio normalizado. Comparar `sha256(arquivo instalado)` direto contra
`checksum_sha256` (a leitura literal do texto original de D2) produz **falso-positivo em toda
instalação legítima** cujo conteúdo bruto não fosse já exatamente `TrimSpace + uma única quebra de
linha` — o caso comum de qualquer markdown com linha em branco final. Coberto por um teste de
regressão load-bearing nos 3 CLIs (`branch_ii_legitimate_install_does_not_false_positive` /
equivalentes), que mantém `checksum_sha256` e `installed_sha256` com valores **deliberadamente
diferentes** na fixture, para provar que a ramificação (ii) usa o campo certo.

**Por que não usar o registro de quarentena como ponte (a solução do ML-3A, tecnicamente correta e
substituída):** o ML-3A resolveu a mesma divergência de domínio lendo o `content_base64` da
quarentena, normalizando-o e comparando contra o arquivo instalado. Funcionava, mas tornava um
artefato de **estágio** (`.trackfw/thirdparty-quarantine/`, com nome, forma e propósito de diretório
temporário) dependência obrigatória de um **gate permanente**: apagar ou colocar esse diretório no
`.gitignore` faria `validate` falhar para sempre, sem caminho de recuperação (`validate` nunca faz
fetch de rede, D6). `installed_sha256` remove essa dependência — a quarentena continua sendo escrita
e commitada (valor de auditoria/reconstrução), mas sua ausência deixou de ser erro da ramificação
(ii). Coberto pelos testes `branch_ii_quarantine_deletion_does_not_break_clean_install` /
`branch_ii_quarantine_deletion_still_detects_tamper` (e equivalentes Node/Python), que apagam
`.trackfw/thirdparty-quarantine/` inteiro antes de validar.

**Quem escreve `installed_sha256`:** apenas `third-party install`, depois que o `manager.install`
já gravou o arquivo com sucesso — nunca antes, nunca em caso de falha do install. Ele carrega a
entrada de proveniência já existente (escrita pelo aprovador), preserva todos os demais campos
intocados, e grava de volta só com `installed_sha256` preenchido. `checksum_sha256` nunca é tocado
por este código — é sempre o valor que o aprovador gravou.

**Ordem de campos e paridade byte-a-byte:** `installed_sha256` entra **logo após**
`checksum_sha256` na ordem canônica de campos (`url`, `checksum_sha256`, `installed_sha256`,
`installed_at`, `approved_by`, `review_reference`, `scope`, `marker_override`) — a mesma ordem nos 3
CLIs (Go: ordem de declaração do struct `ProvenanceEntry`; Python:
`provenance.py:_ENTRY_FIELD_ORDER`; Node: construção explícita do objeto em
`commands/thirdparty.js`, deliberadamente **não** via spread de object, que apenas apenderia o campo
no fim). `scripts/check-thirdparty-parity.sh` compara `thirdparty-provenance.json` byte a byte entre
os 3 CLIs após um install real — essa ordem é o que garante que a comparação passe.
- Claims sem `origin` (manifests legados, escritos antes deste campo existir) ou com
  `origin == ""` são tratados como catálogo e nunca são flagados — retrocompatibilidade explícita,
  testada nos 3 CLIs.

**🔴 Limitação documentada (D11), mesmo destaque das demais seções deste documento:** esta regra só
enxerga artefatos instalados através do próprio fluxo `third-party` do trackfw. Um arquivo de
terceiro copiado manualmente para dentro do projeto, sem passar por `fetch`/`install`, não tem
`Claim.origin == "thirdparty"` no manifest e portanto é invisível para esta regra — ela não é uma
varredura de conteúdo do repositório, é uma auditoria de proveniência do que o próprio trackfw
gravou.

### Limitação do critério de markers (D3) — o que ele NÃO cobre

<!-- trackfw-contract: gate=scripts/check-thirdparty-parity.sh partial=a não-cobertura de homoglifos é comprovada cross-CLI (Parte A do gate, caso cyrillic_pass, que DEVE passar) — mas paráfrase, indireção, fragmentação e conteúdo auto-modificável são limitações declaradas sem nenhum gate ou fixture que demonstre a evasão descrita -->

O checker de markers (`checkMarkers`/`check_markers`/equivalentes) procura por um conjunto fechado
de padrões textuais suspeitos (heading H1–H6 com determinado formato, blocos de código cercados por
` ``` `/`~~~`, comentários HTML, etc.) no conteúdo bruto buscado por `fetch`, antes de permitir que
ele entre em quarentena. Isso é uma **tripwire, não um filtro contra um adversário competente** —
precisa ser lido explicitamente assim por qualquer agente ou humano que dependa dele:

- **NÃO cobre paráfrase** — o mesmo conteúdo malicioso reescrito sem os padrões exatos passa direto.
- **NÃO cobre indireção** — instruções que remetem a outro arquivo/URL não são seguidas nem
  inspecionadas.
- **NÃO cobre fragmentação** — conteúdo dividido em pedaços que só formam um padrão suspeito quando
  concatenados escapa da checagem linha-a-linha.
- **NÃO cobre homoglifos** — um heading escrito com caracteres cirílicos visualmente idênticos ao
  ASCII (ex. а/a) não bate no padrão ASCII esperado; isso é intencional (ver caso de teste cirílico
  em `markers_test.go`/equivalentes, que deve **passar**, i.e. não ser sinalizado) — não é um bug,
  é a fronteira documentada do que o critério tenta pegar.
- **NÃO cobre reivindicações semânticas sem heading** — texto que instrui um agente sem usar
  nenhum dos formatos de heading reconhecidos não aciona o checker.
- **NÃO cobre conteúdo auto-modificável** — nada impede que o conteúdo aprovado seja substituído
  depois por outro conteúdo igualmente inocente-parecendo, exceto a checagem de checksum feita por
  `thirdparty_artifact_has_provenance` (D2 branch ii) no momento de `validate`, que é uma auditoria
  pós-fato, não uma prevenção.

A defesa real contra um adversário competente é a barreira humana entre `fetch` e a aprovação de
provenance (D10.2) — o checker de markers só existe para pegar o caso óbvio e barato, reduzindo o
volume que chega à revisão humana.

### D3-ter — fence não fechado e comentário HTML deixaram de conceder imunidade (ML-4C)

<!-- trackfw-contract: gate=scripts/check-thirdparty-parity.sh -->

Dois achados da barreira da Wave 4 (o do fence pelos **dois** auditores independentemente; o do
comentário HTML só pelo `hades-tf`), ambos reproduzidos pelo arquiteto nos 3 CLIs antes da correção,
e um terceiro achado do `hefesto-tf` (casefold divergente). Nenhum dos três estava listado acima como
lacuna declarada de D3 — eram bugs no passo de normalização, não evasões aceitas:

- **(a) Fence sem fechamento.** Antes: um arquivo iniciado por ``` sem fechamento até EOF fazia o
  line-scanner descartar **o documento inteiro** como "dentro do fence", escondendo qualquer marcador
  depois do abridor — `["\`\`\`", "## Git authority", ...]` retornava `[]`. Correção: fence **sem**
  fechamento não é fence para este critério — as linhas voltam a ser escaneadas normalmente. Fence
  **fechado** continua concedendo imunidade (a emenda original de D3, que impede o próprio parecer de
  segurança de se recusar, é preservada).
- **(b) Comentário HTML.** Antes: o passo 1 **apagava** o comentário inteiro antes de escanear, o que
  contradizia a própria justificativa escrita do passo ("um agente LLM lê comentário HTML no fluxo de
  tokens") — `<!-- ## Git authority -->` passava limpo. Correção: o passo 1 passa a **neutralizar**
  (remover só os delimitadores `<!--`/`-->`), mantendo o conteúdo interno no fluxo escaneado.
- **(c) Casefold divergente.** Go/Node usavam lowercase simples (`strings.ToLower`/
  `toLowerCase()`); Python usava `str.casefold()` (casefold Unicode completo). Sem exploit conhecido
  contra os 6 marcadores ASCII, mas era uma divergência silenciosa e não testada num passo de
  normalização de segurança. Unificado: os 3 CLIs usam lowercase simples agora (Python trocou de
  `casefold()` para `lower()`).

**Não-regressão testada explicitamente:** rodar o checker contra o próprio
`docs/seguranca/2026-08-15-skills-de-terceiro-via-url.md` (que lista os 6 marcadores dentro de um
fence **fechado**) continua retornando zero marcadores nos 3 CLIs — as correções (a)/(b) não afetam
conteúdo dentro de um fence fechado.

### D6-bis — a query string da URL de origem é redigida antes de persistir (ML-4C)

<!-- trackfw-contract: gap reason=grep confirma zero ocorrência de "RedactURL"/"redact_url"/"[redacted]" em qualquer script de scripts/*.sh — a redação de query string/userinfo antes de persistir em quarentena/provenance não tem nenhum gate cross-CLI que a exercite -->

Achado do `hades-tf`: a URL completa (com query string) era gravada **verbatim** em
`.trackfw/thirdparty-quarantine/<checksum>.json` e, potencialmente, em
`.trackfw/thirdparty-provenance.json` — ambos versionados. Uma URL pré-assinada carrega token na
query, que viraria segredo permanente no histórico do git.

Correção: `RedactURL`/`redactURL`/`redact_url` (implementado uma vez por CLI, em
`internal/thirdparty/markers.go`, `npm/src/thirdparty/markers.js`, `pypi/trackfw/thirdparty/markers.py`)
grava `esquema://host/caminho` com a query (e o userinfo, se houver) substituídos pelo literal
`[redacted]`. Aplicado no ponto de construção do registro de quarentena (`NewQuarantineEntry`) e,
como defesa em profundidade, no ponto de escrita da proveniência (`WriteProvenance`) — mesmo sem
nenhum comando neste código escrevendo `ProvenanceEntry.URL` hoje (D10.2: o aprovador externo grava a
entrada diretamente). A URL completa continua sendo usada **só em memória**, para o `fetch` em si; a
redigida é a que vai para os dois arquivos. Verificação de integridade usa checksum, nunca a URL —
nada quebra.

### Gate de paridade (`scripts/check-thirdparty-parity.sh`, ML-3A)

<!-- trackfw-contract: gate=scripts/check-thirdparty-parity.sh partial=a própria seção já declara o limite da Parte D (só StateNotInstalled, StateModified sem gate) -->

Registrado em `make quality`/`parity`. Cobre, nos 3 CLIs:

- Parte A — presença do corpus de casos de markers (heading H1/H6, fence com crase/til,
  **fence não fechada** [ML-4C: agora RECUSA, não passa — D3-ter(a)], **fechador mais curto que o
  abridor** [idem], **fence indentada**, fullwidth NFKC, homoglifo cirílico [deve passar],
  comentário HTML [ML-4C: agora RECUSA — D3-ter(b), neutralizado em vez de apagado], casefold
  simples vs. casefold Unicode completo [ML-4C: novo caso, D3-ter(c)], não-regressão contra o
  próprio parecer de segurança [ML-4C: novo caso], prosa comum, espaços múltiplos, fence seguida de
  heading real) nos 3 conjuntos de testes — os 2 primeiros casos em negrito são onde uma
  implementação via regex divergiria de um line-scanner explícito; Go usa line-scanner porque RE2
  não suporta backreferences, e Node/Python replicam o mesmo algoritmo (não usam backreferences)
  para não divergir nesses casos.
- Parte B — round-trip completo dos 3 schemas D9 via `third-party install` real (stdout, conteúdo
  instalado, `thirdparty-references.json`, claim do manifest com `origin=thirdparty`) byte/semântica
  idêntica entre os 3 CLIs.
- Parte C — mensagem de violação da regra D2 branch (i) byte-idêntica (normalizada por caminho
  absoluto do diretório temporário) entre os 3 CLIs, via `trackfw validate --json`.
- Parte D — mensagem de remediação D10.1 (`--apply-to` contra agente em escopo divergente/ausente/
  modificado) byte-idêntica entre os 3 CLIs, extraída apenas da linha `cannot attach reference:...`
  — comparar a saída bruta de erro top-level não foi possível porque Node/Python não envolvem o
  entrypoint do CLI em um handler top-level equivalente ao de Go/cobra, produzindo stack
  trace/traceback estruturalmente diferentes; essa divergência é pré-existente e de escopo do
  projeto inteiro (não específica de `third-party`), documentada como fora de escopo deste ML.

**🔴 Bug real encontrado por este gate, corrigido no ML-3A:** `pypi/trackfw/commands/thirdparty.py`
usava `f"...{agent_id!r}..."` (repr do Python → aspas simples, ex. `'backend'`) nas 4 mensagens de
erro que citam `agent_id`, enquanto Go usa `%q` (aspas duplas) e Node usa aspas duplas explícitas no
template literal. Corrigido para `f"...\"{agent_id}\"..."` nas 4 ocorrências (não só nas 2 cobertas
pela Parte D) para manter o arquivo internamente consistente com o contrato de paridade auditado.
Nenhum teste pré-existente assumia a forma antiga (aspas simples), portanto nenhum teste foi editado
para essa correção.

**🔴 Lacuna conhecida, não coberta por este gate:** a Parte D só exercita a mensagem D10.1 para o
caso "agente não instalado" (`StateNotInstalled`). O caso irmão "agente modificado manualmente fora
do trackfw" (`StateModified`) tem sua própria mensagem D10.1 e não é comparado entre os 3 CLIs por
este script.

## `trackfw serve` — endereço de escuta, `--host` e aviso de exposição (REQ-2026-08-16, ML-1A/1B/1C)

<!-- trackfw-contract: gate=scripts/check-serve-address-parity.sh -->

Gate: `scripts/check-serve-address-parity.sh` (alvo `parity`) + Cenário 59 de
`check-gates-falsify.sh`. O gate prova o contrato **por escuta real** — sobe cada runtime, mede com
`lsof`, mata — e nunca por leitura de fonte. Um teste que só procurasse a string `"127.0.0.1"` no
código passaria mesmo com o bind efetivo quebrado.

**Origem:** `serve` escutava em todas as interfaces sem autenticação no Go e no Python;
`/api/chain` devolvia a cadeia de governança inteira para qualquer dispositivo da rede.

### Contrato pinado

<!-- trackfw-contract: gate=scripts/check-serve-address-parity.sh -->

| item | valor, idêntico nos 3 runtimes |
|---|---|
| host padrão | `127.0.0.1` — **loopback IPv4**, nunca wildcard |
| opt-in de exposição | flag `--host`, e **nada mais** (sem env, sem `trackfw.yaml`) |
| help da flag | `Host to bind to (loopback only by default; use 0.0.0.0 to expose on the network)` |
| aviso ao expor | `WARNING: trackfw serve is binding to <host>:<port> — the governance chain (ADRs, REQs, roadmaps) will be readable without authentication by any device that can reach it.` |
| destino do aviso | **stderr**, emitido antes do bind |
| `--host ::1` | escuta em `[::1]` nos 3 |

A linha do aviso é comparada **byte a byte** entre os 3 pelo gate, normalizando só a porta.

### URL impressa (e aberta no browser)

<!-- trackfw-contract: gate=scripts/check-serve-address-parity.sh partial=o gate testa o bind real (lsof) para o caso default e a URL impressa para `--host ::1` (colchetes, regra 2) e `--host 0.0.0.0` (não-localhost, regra 3) — mas não há asserção sobre o TEXTO da URL impressa no caso default (regra 1, loopback IPv4 → "http://localhost:<porta>"); o gate confirma o bind com lsof nesse caso, não a string da URL -->

Regra idêntica nos 3, implementada em `DisplayURL` (Go), `displayUrl` (Node) e `_display_url`
(Python):

1. host `localhost` ou **loopback IPv4** (`127.0.0.0/8`) → `http://localhost:<porta>`, para não
   alterar a saída do caso comum;
2. **literal IPv6** → `http://[<host>]:<porta>`, com colchetes (RFC 3986);
3. qualquer outro → `http://<host>:<porta>`.

A classificação IPv4/IPv6 é por **sintaxe do literal** (presença de `:`), **não** por família
decodificada. `net.IP.To4()` do Go é não-nil para `::ffff:127.0.0.1`, enquanto `net.isIPv6()` do
Node e `ipaddress.IPv6Address` do Python classificam o mesmo literal como IPv6 — decodificar
produziria `localhost` no Go e colchetes nos outros dois, uma divergência de 3 vias em lógica de
convenção compartilhada.

### Exceções intencionais — divergências que **não** são violação do contrato

<!-- trackfw-contract: gap reason=as alegações empíricas ("curl localhost devolve 200 nos 3 com bind IPv4"; `--host ::ffff:127.0.0.1`/`127.0.0.2` não bindam em nenhum dos 3) são medições declaradas — grep confirma zero ocorrência de "curl"/"ffff:127"/"127.0.0.2" em check-serve-address-parity.sh, sem gate que as reproduza -->

- **Porta padrão diverge:** Go `4080`, Node e Python `8080`. Pré-existente a esta REQ, fora do seu
  escopo, e **deliberadamente não corrigida aqui** para não misturar mudança de interface com uma
  correção de segurança urgente.
- **Prefixo da linha de URL diverge:** Go `trackfw serve — listening on <url>`, Node
  `trackfw serve: <url>`, Python `trackfw dashboard: <url>`. O gate compara a **URL**, não a linha.
- **Mensagem de erro de bind diverge:** cada runtime propaga o erro da própria stdlib.
- **`--host ::ffff:127.0.0.1` e `--host 127.0.0.2`:** **nenhum** dos 3 consegue escutar nesses
  endereços, logo o impacto de segurança é nulo; o que diverge é só o texto do erro.
- **Loopback dual-stack não é objetivo.** Um listener não cobre `127.0.0.1` e `::1` ao mesmo tempo, e
  o wildcard `:porta` é exatamente o defeito corrigido. Medido: `curl localhost` devolve 200 nos 3
  com bind IPv4, porque o cliente faz fallback.

### 🔴 O que este contrato **não** garante

<!-- trackfw-contract: gate=scripts/check-serve-address-parity.sh partial=a degradação do gate quando `lsof` falta (pula a exclusão de wildcard em vez de passar em silêncio) está no código real do gate (HAVE_LSOF); a ausência de autenticação no `serve` é fato de arquitetura declarado, não uma alegação testável por gate -->

**Não há autenticação no `serve`.** Expor com `--host` deixa a cadeia de governança legível por
qualquer um que alcance a porta — o aviso é a única mitigação, e por ir a stderr **não protege uso
não-interativo** (Makefile, Dockerfile, CI redirecionam stderr para log). Se a exposição vier a ser
intencional, é REQ própria. Ver o apêndice "Barreira ML-2A" em
`docs/seguranca/2026-08-16-vazamento-de-stack-no-cli-node.md`.

O gate degrada para teste de conexão quando falta `lsof`, e nesse modo **pula** a exclusão de
wildcard em vez de passar em silêncio — o braço de detecção do Cenário 59 então deixa de reprovar e
a falsificação fica vermelha. A vacuidade se denuncia sozinha em vez de virar falso verde.

## `trackfw doctor` — detecção de artefato fora do manifesto (REQ-2026-08-17, ADR-2026-08-18, ML-2A/ML-2B/ML-2C)

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->

Gate: **`scripts/check-doctor-parity.sh`** (alvo `parity`) + Cenários 71 e 72 de
`check-gates-falsify.sh`. O gate compara as **três saídas reais** (`diff -u` byte a byte, stdout e
stderr, nas duas superfícies — relatório de texto e `--json`) contra fixtures construídas por
**install-e-mutar** real, nunca por bytes de template hardcoded.

**Origem:** antes da inversão de ordem de persistência do ADR-2026-08-18, uma escrita
interrompida entre gravar os bytes de um artefato e persistir a entrada correspondente no
manifesto podia deixar o disco e o manifesto dessincronizados sem nenhum comando para detectar
ou remediar isso.

**ML-2C (correção da barreira ML-3A):** a auditoria de segurança
(`docs/seguranca/2026-08-18-revisao-do-doctor-e-da-inversao.md`) encontrou um terceiro estado —
`!Registered && StateModified` — caindo em silêncio fora do `switch` de `ClassifyDoctor`. É
precisamente o estado que faz o preflight de `agents install` recusar o mesmo destino com
`unmanaged artifact`: o `doctor` respondia "no mismatches found" ao usuário cujo `install` acabara
de recusar. Ver `vault/notes/doctor-classifydoctor-silences-tampering-when-manifest-entry-removed-2026-08-19.md`.

### As três classes, e por que não podem ser fundidas

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->

| classe | condição | remédio |
|---|---|---|
| `unregistered-write` | conteúdo em disco bate byte a byte com o template atual do catálogo, **mas o manifesto não tem entrada nenhuma** para o destino | adota — `install --force`, seguro porque o conteúdo já é o que o install escreveria de qualquer forma |
| `unknown-content` | conteúdo em disco **não** bate com o template do catálogo **e** o manifesto não tem entrada nenhuma para o destino | ambíguo — nomeia literalmente a recusa: `agents install` recusará este destino com `unmanaged artifact`; se o arquivo é do usuário, remover ou mover; se é do trackfw e derivou do template, `install --force` substitui |
| `hand-modified` | o manifesto **tem** entrada e esta claim é dona dela, **mas o hash em disco divergiu** do que o manifesto registrou | avisa da perda — `install --force` sobrescreve a edição manual |

`unregistered-write` e `unknown-content` usam o mesmo discriminante — `Registered` (existe
**alguma** entrada para o destino, de qualquer claim), não `Managed` (esta claim exata é dona da
entrada) — ver o doc comment de `ClassifyDoctor` (`internal/integrations/doctor.go`). Um destino
registrado sob uma claim **diferente** deve ficar em silêncio, nunca virar falso-positivo, seja de
"escrita não registrada" (Cenário 71) ou de "conteúdo desconhecido" (Cenário 72) — isso quase
entrou como bug no ML-2A para a primeira classe, e é o risco simétrico que o ML-2C precisou fechar
para a segunda.

`unknown-content` é **genuinamente ambíguo** entre duas causas que um sinal só-de-hash não
distingue: um arquivo que simplesmente não é do trackfw ocupando um destino do catálogo, ou um
artefato órfão do trackfw (escrito antes de uma interrupção deixar a entrada do manifesto
ausente, ADR-2026-08-18) cujos bytes derivaram depois que o template do catálogo evoluiu. Por
isso o remédio nomeia a recusa literalmente em vez de escolher um lado.

### Cenários do gate (a–f), cada um em texto e `--json`

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->

- **(a) baseline limpo** — nada instalado, os 3 relatam `no mismatches found` / `[]`.
- **(b) unregistered-write** — install real seguido de remoção cirúrgica da entrada do manifesto
  (bytes em disco intocados).
- **(c) hand-modified** — install real seguido de um byte anexado ao artefato em disco (hash do
  manifesto fica obsoleto).
- **(d) unknown-content** — conteúdo alheio escrito exatamente no destino que o catálogo usaria,
  **sem nunca instalar** (zero entrada no manifesto, conteúdo não bate template) — reproduz
  `Registered=false, State=modified`. **Reinterpretado pelo ML-2C** (antes chamado
  "alien-file-not-flagged", esperava `no mismatches found`): é exatamente o estado que
  `ClassifyDoctor` deixava de reportar; o fixture não mudou, só a expectativa — os 3 CLIs agora
  relatam exatamente um finding `unknown-content` cujo remédio cita `unmanaged artifact`
  literalmente.
- **(e) registrado sob claim diferente, conteúdo atual** — install real seguido de retargetar o
  `item` da claim da entrada do manifesto (hash intocado) — reproduz `Registered=true,
  Managed=false, State=current` e prova que os 3 CLIs ficam em silêncio (o discriminante do
  Cenário 71).
- **(f) registrado sob claim diferente, conteúdo divergido** — igual a (e), mas **também** com um
  byte anexado ao artefato em disco, reproduzindo `Registered=true, Managed=false,
  State=modified`. Adicionado pelo ML-2C: é o **único** fixture do gate capaz de discriminar o
  `!Registered` correto de um `!Managed` incorreto na classe `unknown-content` — (e) sozinho não
  alcança esse `case` porque seu `State` permanece `current`. Prova que os 3 CLIs ficam em silêncio
  (o discriminante do Cenário 72).

### Restrições duras do fixture (cada uma já custou um ciclo nesta série)

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->

1. **`HOME` redirecionado** para um diretório temporário por cenário — `doctor` varre o escopo
   **global** além do de projeto; sem isso o gate leria o `~/.trackfw` real de quem o executa.
2. **Install-e-mutar, nunca bytes de template hardcoded** — hardcode apodrece em silêncio na
   próxima mudança de template do catálogo.
3. **Identidade fixada explicitamente** (`identity.json` escrito em `$HOME` antes do install) —
   sem isso, o fallback zero-value de `identity.Load` faz os 3 CLIs concordarem por construção,
   fechando a paridade vacuamente independente de a renderização sensível a identidade estar
   correta.
4. **Edição do manifesto via `python3`, nunca `sed -i`** — divergência BSD vs GNU já custou uma
   falha de CI só-em-Linux nesta série.
5. **Resolução física do caminho do fixture (`pwd -P`)** — no macOS, `/tmp` e `$TMPDIR` são
   symlinks para `/private/...`; a resolução de cwd do Go só é física após `EvalSymlinks`
   explícito, enquanto Node e Python são sempre físicos. Sem normalizar o `project`/`home` do
   fixture com `pwd -P` antes de instalar, o manifesto fica gravado com a chave não-canônica do Go
   e toda leitura via Node/Python falha o lookup — reportando "não registrado" para **qualquer**
   artefato, independentemente do que foi de fato instalado. Mesmo fix que
   `scripts/check-thirdparty-parity.sh` já aplica.

### Dois defeitos reais de paridade que este gate encontrou e corrigiu no produto (não no gate)

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->

- **`--json` com zero achados:** o Go emitia `null` (slice nil serializada por `encoding/json`)
  onde Node e Python sempre emitem `[]`. Corrigido inicializando `ClassifyDoctor` com
  `[]DoctorFinding{}` (`internal/integrations/doctor.go`) — a forma canônica escolhida é a que
  Node/Python já usavam.
- **Linha em branco final do relatório de texto:** o Go deixava uma linha em branco à direita do
  último finding; Node e Python já normalizavam isso (`.replace(/\n$/, '')` /
  `.rstrip("\n")`). Corrigido em `printDoctorReport` (`internal/commands/doctor.go`) para não
  emitir a quebra final — a forma canônica é a que os outros dois já usavam.

Nenhuma divergência de **nomes de campo** no `--json` foi encontrada: `finding`, `claim`
(`target`/`surface`/`scope`/`kind`/`item`), `destination`, `remedy` já casavam nos 3 CLIs.

---

## `trackfw doctor` — cobertura de artefatos de scaffold (ADR-2026-08-27, ML-1A)

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->

Cobertura adicional integrada ao `trackfw doctor` em ML-1A: os artefatos de scaffold (scripts de
hook, slash commands do Claude) são comparados contra o template que o binário instalado geraria,
usando o `trackfw.yaml` do próprio projeto. Nenhuma entrada é gravada no manifesto — propriedade
por caminho, não por manifesto (ADR-2026-08-27).

### As três classes de finding

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->

| classe | condição | remédio |
|---|---|---|
| `scaffold-divergent` | artefato de scaffold existe em disco mas o conteúdo difere do template que o binário atual geraria | `trackfw update` — a mensagem é neutra quanto à culpa (AC16): não há stamp de versão no artefato, então nem o binário nem o projeto podem ser identificados como o lado defasado |
| `scaffold-missing` | artefato de scaffold que deveria existir está ausente do disco | `trackfw update` |
| `scaffold-wrong-mode` | artefato de scaffold existe com conteúdo correto, mas o bit de execução do owner está ausente (`mode & 0o100 == 0`) em um artefato que deve ser executável | `trackfw update` — o `update` restaura o conteúdo **e** o modo (ver AC9 abaixo) |

As três classes têm `claim` zerado (`kind`, `item`, `target`, `surface`, `scope` = string vazia)
— artefatos de scaffold nunca têm entrada no manifesto.

### Propriedade por caminho — artefatos cobertos pelos 3 CLIs

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->

Os seguintes artefatos são verificados pelo Go e pelo Node.js (sempre que `trackfw.yaml` existe):

| artefato | condicional |
|---|---|
| `scripts/trackfw-validate.sh` | sempre (conteúdo renderizado a partir do `trackfw.yaml` do projeto — AC12) |
| `scripts/trackfw-attention-signal.sh` | sempre |
| `scripts/trackfw-attention-cleanup.sh` | sempre |
| `scripts/trackfw-credential-guard.sh` | sempre |
| `scripts/trackfw-git-branch-guard.sh` | sempre |
| `.claude/commands/trackfw/<cmd>.md` (9 arquivos) | somente se `.claude/commands/trackfw/` já existir (AC14: `discover --init` não escreve slash commands — ausência legítima) |
| `.github/workflows/trackfw-gate.yml` | somente se `ci: github-actions` no `trackfw.yaml` (AC13) |
| `.gitlab-ci-trackfw.yml` | somente se `ci: gitlab-ci` no `trackfw.yaml` (AC13) |

### validate.sh — pertencimento a conjunto (set-membership, escopado)

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh -->

**Decisão arquitetural (2026-08-27):** `scripts/trackfw-validate.sh` é aceito pelo `doctor` quando
seu conteúdo corresponde a **qualquer** dos templates de runtime conhecidos — pertencimento a
conjunto, não igualdade a um único template.

A divergência de bytes entre os runtimes para este arquivo é **pré-existente, intencional e
documentada**: Go/Node produzem uma forma `#!/usr/bin/env sh` cfg-dependente (com blocos de
build por backend/frontend); Python produz uma forma simples `#!/usr/bin/env bash` agnóstica de
backend (`_VALIDATE_SCRIPT_CONTENT`). Aceitar qualquer das formas conhecidas elimina o
falso-positivo que ocorria quando o doctor de um runtime julgava um arquivo escrito por outro.

| forma conhecida | conteúdo | runtime de origem |
|---|---|---|
| Go/Node form | `buildValidateScript(cfg)` renderizado a partir do `trackfw.yaml` do projeto | Go (`internal/generators/scaffold.go`) · Node.js (`npm/src/generators/init.js`) |
| Python form | `#!/usr/bin/env bash\nset -euo pipefail\ntrackfw validate\n` (`_VALIDATE_SCRIPT_CONTENT`) | Python (`pypi/trackfw/generators/init_gen.py`) |

**O que continua sendo acusado:** arquivo que não casa com **nenhuma** das formas conhecidas —
editado à mão, corrompido, ou gerado por uma versão futura que alterou o template sem atualizar
o conjunto. A cobertura não diminui; apenas o critério deixa de ser runtime-específico para este
artefato.

**Escopo da exceção: apenas `scripts/trackfw-validate.sh`.** Todos os demais artefatos de scaffold
(`trackfw-attention-signal.sh`, `trackfw-attention-cleanup.sh`, `trackfw-credential-guard.sh`,
`trackfw-git-branch-guard.sh`, slash commands, CI workflows) têm bytes pinados entre os runtimes
por gate e continuam usando igualdade a template único.

**Parity liability:** `pypi/trackfw/integrations/scaffold_doctor.py` mantém um mirror de
`_build_go_node_validate_script` que replica a lógica de `buildValidateScript(cfg)`. Se Go ou
Node alterarem o template, esse mirror deve ser atualizado no mesmo commit. Os testes de
pertencimento nos 3 runtimes (`TestValidateScriptMembership` em Go,
`scaffold_doctor_membership.test.js` em Node, `test_scaffold_doctor_membership.py` em Python)
detectam a deriva antes que chegue à main.

### Cobertura por runtime — tabela completa

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh partial=exclusão de CI workflow no Python não coberta cross-CLI por gate único (fixtures não usam ci: para evitar divergência legítima Python-vs-Go/Node; ver "CI workflow exclusion" abaixo) -->

| artefato | Go | Node.js | Python |
|---|---|---|---|
| `scripts/trackfw-validate.sh` | sim — **set-membership** (Go/Node form ∪ Python form) | sim — **set-membership** | sim — **set-membership** |
| `scripts/trackfw-attention-signal.sh` | sim | sim | sim |
| `scripts/trackfw-attention-cleanup.sh` | sim | sim | sim |
| `scripts/trackfw-credential-guard.sh` | sim | sim | sim |
| `scripts/trackfw-git-branch-guard.sh` | sim | sim | sim |
| `.claude/commands/trackfw/<cmd>.md` | sim (AC14) | sim (AC14) | sim (AC14) |
| `.github/workflows/trackfw-gate.yml` | sim (AC13: `ci: github-actions`) | sim (AC13) | não — ver abaixo |
| `.gitlab-ci-trackfw.yml` | sim (AC13: `ci: gitlab-ci`) | sim (AC13) | não — ver abaixo |

**CI workflow exclusion — Python (principled):** o `update` do Python não inclui `ci-workflow` em
`PROJECT_TARGET_IDS` (ver `pypi/trackfw/commands/update.py`). O remedy de qualquer finding do
doctor é `trackfw update`; se o `update` do Python não gerencia o caminho, o remedy seria
enganoso. A exclusão dos CI workflows é fundamentada em propriedade, não em conveniência.

### Estado `scaffold-wrong-mode` — bit de execução ausente (REQ-2026-08-28)

<!-- trackfw-contract: gate=scripts/check-doctor-parity.sh,scripts/check-gates-falsify.sh -->

Adicionado em REQ-2026-08-28 (ROADMAP-2026-08-28-doctor-compara-o-bit-de-execucao-dos-artefatos-de-scaffold).
Três estados são agora distintos para artefatos de scaffold executáveis: conteúdo correto + bit
presente (silêncio), conteúdo errado (`scaffold-divergent`, não importa o modo), conteúdo correto
+ bit ausente (`scaffold-wrong-mode`, AC3).

**Quais artefatos têm `execBit=true` (os únicos que podem receber `scaffold-wrong-mode`):**

| artefato | execBit | razão |
|---|---|---|
| `scripts/trackfw-validate.sh` | `true` | gerado com `0755` nos 3 runtimes |
| `scripts/trackfw-attention-signal.sh` | `true` | idem |
| `scripts/trackfw-attention-cleanup.sh` | `true` | idem |
| `scripts/trackfw-credential-guard.sh` | `true` | idem |
| `scripts/trackfw-git-branch-guard.sh` | `true` | idem |
| `.claude/commands/trackfw/*.md` (9 arquivos) | `false` | markdown 0644 — nunca executável (AC4/AC11) |
| `.github/workflows/trackfw-gate.yml` | `false` | YAML 0644 — nunca executável (AC4/AC11) |
| `.gitlab-ci-trackfw.yml` | `false` | YAML 0644 — nunca executável (AC4/AC11) |

**AC10 — máscara de bit, não igualdade:** o teste é `mode & 0o100 != 0`, não `mode == 0755`.
Modos umask-narrowed como `0750` ou `0700` têm o bit de execução do owner e são aceitos. Um
arquivo em `0755` que perdeu o bit para `0644` não tem o bit e é acusado.

**AC5 — guarda de plataforma Windows:** no Windows (`CurrentGOOS == "windows"`) o bit de execução
não é representável em NTFS. A verificação de modo é suprimida inteiramente — `scaffold-wrong-mode`
nunca é emitido nesta plataforma. O doctor imprime uma nota informando a supressão. O gate
`check-doctor-parity.sh` não cobre Windows (os CI runners do projeto são Linux/macOS); testes
unitários específicos de plataforma no Go cobrem a guarda via `generators.CurrentGOOS`.

**AC9 — `trackfw update` restaura o modo em arquivos existentes:** `os.WriteFile` / `writeFileSync`
aplicam `perm` somente no evento `O_CREATE`; em arquivo existente (`O_TRUNC`) o conteúdo é
reescrito mas o inode mode não é tocado. Cada runtime adiciona uma chamada explícita de Chmod
**após** a escrita para restaurar `0755` mesmo quando o arquivo já existia:

| runtime | chamada | local |
|---|---|---|
| Go | `os.Chmod(path, 0755)` | `generateValidateScript` em `internal/generators/scaffold.go` |
| Node.js | `fs.chmodSync(path, 0o755)` | `generateValidateScript` em `npm/src/generators/init.js` |
| Python | `os.chmod(path, 0o755)` | `_generate_validate_script` em `pypi/trackfw/generators/init_gen.py` — já era incondicional antes de AC9 |

**Falsificação nas três direções (gate `check-gates-falsify.sh`, Cenários 179–181):**

| cenário | sabotagem | o que o gate detecta |
|---|---|---|
| 179 (direção A) | `execBit &&` → `false  &&` em `scaffold_doctor.go:324` — a condição de modo nunca dispara | `check-doctor-parity.sh` reprova: cenário (p) `scaffold-wrong-mode-detected` não encontra `[scaffold-wrong-mode]` no Go |
| 180 (direção B) | `execBit &&` → `true   &&` — discriminante AC11 silenciado: todos os artefatos têm o bit verificado, incluindo os 0644 | doctor Go invocado diretamente em fixture com slash commands (`--targets validate-script,agent-hooks,claude-commands`): 9 `scaffold-wrong-mode` falsos positivos em `.claude/commands/trackfw/*.md` |
| 181 (direção C) | `os.Chmod` removido de `generateValidateScript` — `update` reescreve o conteúdo mas não restaura o bit | `cmp -s` confirma que `apply()` rodou (conteúdo restaurado); `test ! -x` confirma que o bit permanece ausente após o update |

---

## `agent_models` — versão de modelo por tier com composição por alvo

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

O campo `agent_models` em `trackfw.yaml` permite configurar a versão do modelo por tier de agente
(`sonnet`, `opus`, `haiku`). Quando presente, o CLI compõe um model ID completo para o target
`claude`; todos os outros targets permanecem com seu alias canônico de tier — essa fronteira é
um GATE (ADR-2026-08-21), não um detalhe de implementação.

### Motivo: cota, não custo

<!-- trackfw-contract: none reason=este subtítulo é rationale de arquitetura (por que o campo existe), não uma alegação de comportamento de CLI; o comportamento consequente (composição e fronteira de namespace) é coberto pelos demais cenários do gate -->

O campo existe para que equipes que têm acesso a modelos específicos por cota (ex.: acesso
antecipado ao `claude-opus-5` antes do GA) possam fixar essa versão no projeto sem depender do
alias genérico de tier (`opus`). Não é um mecanismo de controle de custo; o campo não altera o
tier de nenhum agente — só especifica a versão dentro do tier.

### Formato

<!-- trackfw-contract: none reason=este subtítulo documenta a sintaxe YAML do campo; a verificação de que os 3 CLIs lêem e aplicam corretamente esse formato está nos Cenários 1–4 do gate -->

```yaml
agent_models:
  sonnet: "4.6"     # compõe para claude-sonnet-4-6 no target claude
  opus: "5"         # compõe para claude-opus-5 no target claude
```

### As três regras de composição (aplicadas nos 3 CLIs identicamente)

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

1. **Versão com ponto(s):** `"4.6"` → `claude-sonnet-4-6` (ponto substituído por traço).
2. **Major sem minor:** `"5"` → `claude-opus-5` (sem sufixo `-0`; minor omitido é omitido no ID).
3. **Escape hatch — valor literal:** se o valor contiver traço ou qualquer caractere não-numérico
   além de ponto (ex.: `"claude-sonnet-4-5-20250929"`), ele é usado literalmente como model ID,
   sem prefixo nem substituição. O predicado `isVersionString` (regex `^[0-9]+(\.[0-9]+)*$`)
   decide entre composição e passagem literal.

### Fronteira de namespace (regra mais importante)

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

Apenas o target `claude` recebe composição. A condição de guarda em
`internal/integrations/render.go` é:

```go
} else if targetID == "claude" && len(agentModels) > 0 {
```

Targets que usam o `case default:` do switch de representação (Gemini, Cursor, Kiro, Copilot,
Windsurf) **não** recebem o model ID composto. Targets com cases dedicados que retornam cedo
(Codex via `custom-agent-toml`, Antigravity via `agent-directory`, OpenCode via
`opencode-agent`, AmazonQ via `cli-agent-json`) também não são afetados.

**Consequência de uma violação:** um agente Gemini receberia `model: claude-sonnet-4-6` no lugar
de `model: sonnet` — seu SDK não reconhece o prefixo `claude-` e rejeita o modelo na inicialização.
A causa fica a duas camadas de distância do sintoma, tornando o diagnóstico custoso.

### Comportamento quando `agent_models` está ausente

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

Sem `agent_models` no `trackfw.yaml`, o comportamento é idêntico ao de hoje: cada agente recebe
o alias canônico do seu tier (`sonnet`, `opus`, etc.) sem nenhuma composição. Nenhum default
implícito é aplicado; o campo é completamente opcional.

### Cenários cobertos pelo gate (`check-agent-models-parity.sh`)

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

| Caso | Descrição | Axis de comparação |
|------|-----------|-------------------|
| 1 — Composição | `agent_models: {opus: "5", sonnet: "4.6"}` → agente architect recebe `model: claude-opus-5`, backend recebe `model: claude-sonnet-4-6` | Cross-runtime (Go = Node = Python) |
| 2 — Sem vazamento | Com `agent_models` configurado, targets não-Claude (Codex, Gemini) **não** mudam em relação ao baseline sem `agent_models` | Por-runtime (baseline vs candidate, independente entre runtimes) |
| 3 — Config ausente | Sem `agent_models`, o target `claude`/backend mantém o alias `sonnet` sem composição | Cross-runtime |
| 4 — Escape hatch | `sonnet: "claude-sonnet-4-5-20250929"` é passado literalmente como `model: claude-sonnet-4-5-20250929` | Cross-runtime |

O Caso 2 é verificado **por-runtime** (não cross-runtime) porque um vazamento correlacionado
identicamente nos 3 runtimes passaria numa verificação cross-runtime — o problema ainda existiria
e o usuário só o descobriria em produção.

### Nota de implementação: três guards no switch de representação

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

O switch em `render.go` tem três camadas de proteção que somadas garantem a fronteira:

1. Cases dedicados que `return` cedo (Codex, Antigravity, OpenCode, AmazonQ) — nunca alcançam o bloco de composição.
2. Guard `targetID == "cursor"` dentro do `default:` — Cursor tem seu próprio `if` antes do `else if` de composição.
3. Guard `targetID == "claude" && len(agentModels) > 0` — o load-bearing literal que protege todos os demais targets que caem no `default:` (Gemini, Kiro, Copilot, Windsurf).

A remoção da condição `targetID == "claude" &&` do guard 3 é o único vetor de vulnerabilidade
não redundante — o Cenário 86 de `check-gates-falsify.sh` o falsifica diretamente.

## Resolução por escopo de `agent_models` — fonte da configuração por escopo (ML-1A, ADR-2026-08-23)

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

O escopo escolhe o arquivo, exclusivamente — sem merge, sem fallback entre escopos (ADR-2026-08-23, AC13):

| Escopo | Arquivo lido | Precedência |
|--------|-------------|-------------|
| `global` | `~/.trackfw/trackfw.yaml` | Exclusivo: o cwd não é consultado para o valor |
| `project` | `./trackfw.yaml` do cwd | Exclusivo: o arquivo global não é consultado |

### Contratos de escopo global

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

#### Dois cwds, mesmo pin global: saída byte-idêntica

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

`agents install --scope global` invocado de dois cwds diferentes (um sem `trackfw.yaml`, outro com `agent_models` distinto) deve produzir arquivos de agente byte-idênticos em `~/.claude/agents/`. O valor vem sempre de `~/.trackfw/trackfw.yaml`, jamais do cwd.

#### Aviso "configurado no projeto, não no global"

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

Quando `~/.trackfw/trackfw.yaml` existe mas não tem `agent_models` E o `trackfw.yaml` do cwd tem `agent_models`, o comando emite para stderr (byte-idêntico nos 3 CLIs):

```
trackfw: agents global: agent_models configurado em trackfw.yaml do projeto mas não vale para escopo global. Mova a chave para ~/.trackfw/trackfw.yaml.
```

O valor do projeto **não é aplicado**: o tier canônico (`model: sonnet`, `model: opus`) é usado.

#### Aviso "não configurado"

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

Quando `~/.trackfw/trackfw.yaml` está ausente ou não tem `agent_models` E o cwd também não tem `agent_models`, o comando emite para stderr (byte-idêntico nos 3 CLIs):

```
trackfw: agents global: agent_models não configurado em ~/.trackfw/trackfw.yaml — usando tier canônico. Configure em ~/.trackfw/trackfw.yaml para pinar versões.
```

#### Config global malformada: não fatal (AC12)

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

Se `~/.trackfw/trackfw.yaml` existe mas contém YAML inválido, o comando:
- **Não faz exit 1** — diferente do `trackfw.yaml` de projeto, que é fatal.
- Emite o aviso abaixo para stderr (byte-idêntico nos 3 CLIs).
- Usa o tier canônico como fallback.

```
trackfw: aviso: "~/.trackfw/trackfw.yaml" tem YAML malformado — config global de modelo ignorada; usando tier canônico.
```

### Contrato de escopo de projeto (não-regressão)

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

`agents install --scope project` lê `agent_models` do `trackfw.yaml` do cwd, mesmo que `~/.trackfw/trackfw.yaml` esteja presente com valores diferentes. A presença do arquivo global não altera o comportamento de escopo de projeto.

### Cenários do gate (Cases 6–10 de `check-agent-models-parity.sh`)

<!-- trackfw-contract: gate=scripts/check-agent-models-parity.sh -->

| Case | Fixture | Verificação |
|------|---------|-------------|
| 6 — Dois cwds, pin global | home com `agent_models`, cwd-a vazio, cwd-b com `sonnet: "9.9"` distinto | Saída byte-idêntica entre cwds; vacuity guard: `model: claude-opus-5` |
| 7 — Pin só no projeto | `~/.trackfw/trackfw.yaml` sem `agent_models`, cwd com `agent_models` | Aviso "wrong place" em stderr; `model: sonnet` (não composto) |
| 8 — Pin em lugar nenhum | Sem `~/.trackfw/trackfw.yaml`, sem `trackfw.yaml` no cwd | Aviso "not configured" em stderr; `model: sonnet` |
| 9 — Escopo de projeto (não-regressão) | Global com `sonnet: "4.6"`, projeto com `sonnet: "9.9"` | Vacuity guard: `model: claude-sonnet-9-9`; cross-runtime byte-idêntico |
| 10 — Config global malformada | `~/.trackfw/trackfw.yaml` com YAML inválido | Exit 0; aviso malformed em stderr; `model: sonnet` |


---

## `trackfw audit-surface`

<!-- trackfw-contract: gate=scripts/check-audit-surface.sh -->

`trackfw audit-surface <ref> [--base <base>] [--json]` reports the executable surface of a git ref **without checking it out** — all file reads go through `git show <ref>:<path>`.

REQ: `docs/req/REQ-2026-08-26-checkout-de-pr-executa-hook-versionado-sem-que-nada-avise-o-mantenedor.md`
ADR: `docs/adr/ADR-2026-08-26-superficie-executavel-de-um-checkout-de-pr-e-auditada-por-comando-dedicado-nao-por-regra-de-validate.md`
Implemented: Wave 1 / ML-1A (2026-08-27, apolo-tf)

| Aspect | Contract |
|---|---|
| Invocation | `trackfw audit-surface <ref>` |
| `--base <base>` | Optional. Base ref for Makefile/CI diff. |
| `--json` | Emit report as JSON. |
| Runtimes scanned | All 8 project-scope runtimes: claude, codex, gemini, copilot, cursor, kiro, windsurf, amazonq — always, even when absent at the ref. |
| Unit of reporting | `(event, matcher, raw_command, script_path, script_digest)` — any component changing is a surface change (AC14). |
| Absence | Reported as `absent [runtime] wiring-file` — absence is information, not exclusion (AC13). |
| Instruction files | Reported as `instruction [present|absent] path` — distinct label from shell scripts (AC15). |
| Slash commands | Reported as `slash-command path` (AC15). |
| False positive guard | The command opens ONLY the 8 exact wiring-file paths. It never greps file content for hook-path strings. `docs/cli-parity.md` and `internal/generators/agentfiles.go` are never opened (AC16, by construction). |
| No judgment | The command names what executes; it never judges whether a script is hostile (AC5, AC6). |
| Byte-identical | Text output and `--json` are byte-identical across all 3 CLIs. |

### Text output format

<!-- trackfw-contract: gate=scripts/check-audit-surface.sh -->

```
trackfw audit-surface: N hook tuple(s) at REF
[blank line]
hook [runtime] wiring-file event/matcher raw-command <digest>
...
absent [runtime] wiring-file
...
instruction [present|absent] path
...
slash-command path
...
lifecycle [present|absent] file key [command]
```

`<digest>` values for hook tuples:
- `sha256:<hex>` — script resolved and hashed
- `not-found` — resolved path does not exist at the ref
- `unresolvable` — command genuinely cannot be resolved to a file path (pipe, builtin, `-c` inline string)
- `symlink-><target>|sha256:<hex>` — script is a git symlink; target content hashed (F2 fix)
- `symlink-><target>|not-found` — symlink target absent at the ref
- `symlink-><target>|not-supported` — absolute symlink target (not followed without checkout)

`normalizeCommand` resolves: bare paths, paths with arguments, interpreter-prefix forms
(`bash x.sh`, `python3 x.py`, etc.), and `$CLAUDE_PROJECT_DIR/`-prefixed paths.
Recognised script extensions: `.sh .bash .zsh .py .js .rb .pl .fish`

`lifecycle` lines:
- `lifecycle [present] file key` for presence-only entries (e.g., `.vscode/tasks.json`)
- `lifecycle [present] file key command` when a command was extracted
- `lifecycle [absent] file key` when absent
Lifecycle inventory: `package.json` (discovered: root first, then `npm/package.json`),
`.husky/pre-commit`, `.vscode/tasks.json` (presence/absence only).

### JSON output format

<!-- trackfw-contract: gate=scripts/check-audit-surface.sh -->

```json
{
  "ref": "...",
  "base": "...",
  "hook_wiring": [
    {
      "runtime": "...",
      "wiring_file": "...",
      "present": true,
      "tuples": [
        {
          "event": "...",
          "matcher": "...",
          "raw_command": "...",
          "script_path": "...",
          "script_digest": "sha256:..."
        }
      ]
    }
  ],
  "instruction_files": [
    {"path": "...", "kind": "agent-config|slash-command", "present": true}
  ],
  "lifecycle_hooks": [
    {"file": "...", "key": "...", "command": "...", "present": true}
  ]
}
```

Note: `base` field is omitted from JSON across all 3 CLIs when not provided.

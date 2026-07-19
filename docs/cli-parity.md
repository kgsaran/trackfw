# CLI parity contract

Go is the behavioral reference. Node.js and Python must expose the same public
commands unless an exception is listed below.

Supported runtimes: Go 1.25+, Node.js 18+, and Python 3.10+.

| Command | Go | Node.js | Python | Contract |
|---|---:|---:|---:|---|
| `init` | yes | yes | yes | Creates governance structure and `trackfw.yaml` |
| `adr` | yes | yes | yes | `new`, `list` |
| `req` | yes | yes | yes | `new`, `list` |
| `roadmap` | yes | yes | yes | `new`, `move`, `list`, `show` |
| `validate` | yes | yes | yes | Text and `--json`; nonzero on violations |
| `status` | yes | yes | yes | Governance summary |
| `context` | yes | yes | yes | Markdown/JSON context |
| `log` | yes | yes | yes | Append/read transition log |
| `baseline` | yes | yes | yes | Persist accepted findings |
| `help` | yes | yes | yes | Configuration key documentation |
| `configure` | yes | yes | yes | Generate configuration |
| `discover` | yes | yes | yes | Inspect existing repository |
| `update` | yes | yes | yes | Refresh managed artifacts |
| `metrics` | yes | yes | yes | Delivery metrics |
| `sync` | yes | yes | yes | Jira/Linear synchronization |
| `plugins` | yes | yes | yes | Plugin operations supported by runtime |
| `serve` | yes | yes | yes | Local dashboard |
| `agents` | yes | yes | yes | `list`, `install`, `uninstall`, `update` across supported AI CLIs |
| `skills` | yes | yes | yes | `list`, `install`, `uninstall`, `update` across supported AI CLIs |
| `gemini` / `cursor` / `copilot` / `windsurf` / `amazonq` | yes | no | no | Historical Go-only compatibility aliases |
| `version` / `--version` | yes | yes | yes | Prints `trackfw <version>` |

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
Cursor, GitHub Copilot, Windsurf, Amazon Q, and Kiro.

Lifecycle state is one of `not-installed`, `current`, `outdated`, or `modified`.
Ownership and SHA-256 are stored per project or global scope. Update and
uninstall preserve modified files unless `--force` is explicit; uninstall never
removes an unmanaged file or a shared artifact that still has another claim.
Legacy surfaces are inspected by `list` and selected explicitly for mutations,
for example `--surface antigravity=legacy-cli`. Known legacy templates can be
adopted safely; unknown content is never adopted by `update`, even with force.

The standalone `gemini`, `cursor`, `copilot`, `windsurf`, and `amazonq` names
exist only in the Go distribution for historical compatibility. They are not
part of the cross-runtime contract; use `agents` and `skills` in new automation.

## Release rule

Changes to commands, options, exit codes, JSON fields, validation rules, or
generated artifact semantics require equivalent tests in all affected runtimes.

`internal/serve/static` is the canonical dashboard asset source. Copies packaged
by npm and PyPI must remain byte-identical and are checked in CI.

Plugin downloads use timeouts, size limits, and atomic replacement. Registry
entries do not currently publish checksums; adding signed SHA-256 metadata is a
required compatibility change before checksum verification can become mandatory.

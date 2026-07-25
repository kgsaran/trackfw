# trackfw

Native Python distribution of the trackfw governance CLI.

Project documentation, installation instructions, and the CLI parity contract:
https://github.com/kgsaran/trackfw

## Agent identity

The ten installed agents (`architect`, `backend`, `frontend`, `qa`, `infra`,
`security`, `dba`, `ux`, `code-quality`, `data`) can be given real names.

```bash
trackfw init --identity-preset greek
```

Accepted values are the ten themed presets plus two opt-outs:
`greek`, `norse`, `potter`, `thrones`, `chaves`, `pioneers`, `starwars`,
`tolkien`, `turma`, `egyptian`, `neutral`, `none`.

Sample mapping (three of the ten presets):

| Agent | `greek` | `starwars` | `egyptian` |
|---|---|---|---|
| architect | Zeus | Yoda | Thoth |
| backend | Apolo | Han | Rá |
| frontend | Afrodite | Leia | Ísis |
| qa | Ártemis | Ahsoka | Hórus |
| infra | Ares | Chewbacca | Ptah |
| security | Hades | Vader | Anúbis |
| dba | Poseidon | R2-D2 | Seshat |
| ux | Atena | Padmé | Bastet |
| code-quality | Hefesto | Obi-Wan | Maat |
| data | Métis | C-3PO | Osíris |

The remaining presets are `norse` (Odin, Thor, Freya…), `potter` (Dumbledore,
Snape, Luna…), `thrones` (Tyrion, Jon, Arya…), `chaves` (Girafales, Madruga,
Chiquinha…), `pioneers` (Turing, Ritchie, Berners-Lee…), `tolkien` (Gandalf,
Aragorn, Arwen…), and `turma` (Franjinha, Cebolinha, Magali…).

### Custom mode and your nickname

Running `trackfw init` without the flag opens the wizard, which additionally
offers **name them one by one** — you type all ten display names yourself.
Invalid names are rejected inline with an error, never silently corrected, and
two names resolving to the same identifier are rejected as well. The wizard
also asks for an optional nickname for you, used by the agents when addressing
you.

### `agents install` also runs the wizard

`trackfw init` is not the only entry point: `trackfw agents install`, the
natural path in a project that is already governed, offers the same wizard.
It appears **only** when all of the following hold: the command is `agents`
(never `skills` — skills have no identity), stdin is a TTY, and either no
`~/.trackfw/identity.json` exists yet or `--identity` was passed to force
reconfiguration. With an identity already configured and no `--identity`,
the command asks nothing — it prints `identity: N custom agent(s)` and
installs directly.

```bash
trackfw agents install --targets claude                          # offers the wizard on first run, then installs; installs directly if identity is already set
trackfw agents install --targets claude --identity                # force reconfiguration
trackfw agents install --targets claude --identity-preset chaves  # non-interactive
```

Two new flags exist only on `agents install` (`skills install` never
registers them): `--identity` (bool, forces reconfiguration) and
`--identity-preset <preset>` (same ten presets plus `neutral` and `none`; an
invalid value errors out listing the valid ones).

In **name them one by one** mode, each field is labeled by the agent's
specialty from the catalog, never by its technical id (e.g. `Architect —
Architecture, ADRs and governed coordination`). Before anything is written to
disk — for a preset or for custom names — a confirmation screen lists all ten
`specialty → name` pairs plus your nickname; answering no returns to preset
selection without writing anything.

### Shared configuration

Identity lives in a single global file read by the Go, npm, and PyPI CLIs
alike:

```json
// ~/.trackfw/identity.json
{
  "schema_version": 1,
  "user_nickname": "Kleber",
  "agents": {
    "architect": { "display_name": "Zeus", "slug": "zeus" }
  }
}
```

The install path is unchanged (`~/.claude/agents/trackfw-architect.md`); only
the frontmatter and the first line of the body carry the identity:

```markdown
---
name: zeus-tf
description: Zeus — Principal software architect for system design, ADRs and governed multi-agent coordination.
model: opus
---

Você é Zeus. Trate o usuário como Kleber.

# Architect
...
```

### The `-tf` suffix

The `name` always ends in `-tf`. Two agents sharing a `name` in the same
directory make Claude Code load *"only one of them, chosen by filesystem read
order rather than a documented precedence"* — silent shadowing the user cannot
detect. The suffix is part of the technical identifier only; the agent still
presents itself as **Zeus**.

### Invoking

- `@agent-zeus-tf` works — explicit mention resolves against `name`.
- "ask Zeus to…" works — natural-language routing reads `description`.
- "who are you?" answers "Sou Zeus" — the body loads after selection.

### Cost and non-regression

The agent never reads the configuration at runtime. Identity is materialized
into the artifact at install time, so the per-interaction cost is essentially
zero: the `description` is substituted rather than extended, and the body grows
by tens of tokens loaded only after the agent is already selected.

Without `~/.trackfw/identity.json`, the generated artifacts are byte for byte
identical to the current ones. The feature is opt-in and regresses nothing.

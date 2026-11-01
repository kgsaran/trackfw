---
domain: gitignore
problem: negation-override-by-append
date: 2026-11-01
roadmap: ROADMAP-2026-11-01-memoria-de-agente-cai-em-subpastas-quando-o-cwd-do-despacho-nao-e-a-raiz-do-repositorio.md
severity: MEDIUM
---

# `.gitignore` append after `!`-negation overrides the opt-out (A-1 / #646 ML-2A)

## Root cause

`hasGitIgnoreAgentMemoryRule` (scaffold.go) skips `!`-prefixed lines with `continue` — they
are neither counted as "present" nor as "absent." The intent (stated in the function comment) is
"not re-appended over an explicit opt-out." But because the function returns `false`, the caller
appends the pattern *after* the negation line. Under gitignore's last-match-wins semantics, the
appended pattern wins.

```
# Before generateGitIgnore:
!lib/.claude/agent-memory/

# After generateGitIgnore appends the trackfw block:
!lib/.claude/agent-memory/
...
*/**/.claude/agent-memory/     ← last match wins
```

`git check-ignore -v lib/.claude/agent-memory/zeus/test.md` flips from NOT IGNORED to IGNORED.

## Measured evidence (ML-2A red-team)

```
# Before:
git check-ignore -v lib/.claude/agent-memory/zeus/test.md → NOT IGNORED

# After appending */**/.claude/agent-memory/:
git check-ignore -v lib/.claude/agent-memory/zeus/test.md
  .gitignore:3:*/**/.claude/agent-memory/    lib/.claude/agent-memory/zeus/test.md
```

## What the comment says vs. what the code does

| claim | reality |
|---|---|
| "the rule is not re-appended over an explicit opt-out" | the function returns false for `!` lines → rule IS appended |
| "Lines starting with `!` are skipped" | correct — but skipping ≠ blocking the append |

## Fix

When iterating lines, if a `!`-prefixed line is a subdirectory or superset of
`.claude/agent-memory/`, treat the rule as "intentionally absent" and do not append.
Alternatively: only skip `!` lines that are an exact negation of the agent-memory pattern
(`!*/**/.claude/agent-memory/`) and document all other negation patterns as not recognized.

The conservative fix is: return `true` (rule "present") if any `!`-line contains
`agent-memory` in the path, preventing any append that could override it.

## When this triggers

Consumer project that tracks agent memory in a specific subdirectory (e.g., `lib/.claude/`)
and has added `!lib/.claude/agent-memory/` to opt out of a global ignore rule. After
`trackfw init` or `trackfw update`, that memory is silently ignored by git.

## Related

- P-4 of Wave-0 docs/seguranca/2026-11-01-wave0-memoria-de-agente-em-subpasta.md
- A-1 in docs/seguranca/2026-11-01-redteam-memoria-de-agente-em-subpasta.md

---
name: project_git_add_A_with_pathspec_equals_pathspec
description: git add -A <dir> stages exactly the same files as git add <dir> — blocking -A with pathspec is pure false positive (AC3)
metadata:
  type: project
---

`git add -A docs/` and `git add docs/` produce identical staging results (measured 2026-10-17 with `git add -n` in temp repo). The `-A` flag widens scope only when NO pathspec is given; with a pathspec, it has no additional effect.

**Why:** The guard rule for implicit staging must be scope-based, not flag-based. Flag-only detection (`t == "-A"`) blocks legitimate `git add -A docs/roadmaps/wip/specific-file.md` patterns while passing `git add -A` (bare, whole-tree).

**How to apply:** In the `case "add":` implementation, the blocking condition is:
- No positionals and no interactive flags (`-p`/`-i`/`-n`) → BLOCK
- Positional is `.`, `..`, `:/`, `:(top)` → BLOCK
- Positional is any other path (file or directory) → PASS (regardless of `-A`/`-u` flags)

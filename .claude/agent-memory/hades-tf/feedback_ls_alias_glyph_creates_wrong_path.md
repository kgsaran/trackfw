---
name: ls-alias-glyph-creates-wrong-path
description: ls aliased to eza prints icon glyphs; $(ls -d ...) captures glyph as part of path, mkdir creates directories inside the real repo
metadata:
  type: feedback
---

Never use `$(ls …)` or `ls -d` with command substitution to capture directory paths — the shell alias for `ls` is eza (or similar), which prepends icon glyphs (e.g., U+F115). The captured string includes the glyph, so `mkdir -p "$VAR/subdir"` creates a path under the real repo root when cwd is there.

**Why:** measured 2026-11-10 ML-4A. `$(ls -d "$SCRATCH/consumer5."*)` returned `<U+F115> /private/tmp/.../consumer5.7TFxdq`. Running `mkdir -p "$TESTDIR5/docs/adr"` with cwd=real repo created `<repo>/<glyph> /private/tmp/.../consumer5.7TFxdq/docs/adr`, a nested structure inside the real repo. Required `rm -rf` and disclosure in the parecer.

**How to apply:** always use explicit, hardcoded paths or `mktemp -d` and capture the return value DIRECTLY (e.g., `TESTDIR=$(mktemp -d "$SCRATCH/consumer.XXXXXX")`). Never use `ls` to find a directory when you already have the path. Use `/bin/ls` if you must, never the aliased `ls`.

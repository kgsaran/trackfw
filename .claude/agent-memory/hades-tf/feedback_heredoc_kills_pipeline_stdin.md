---
name: heredoc-kills-pipeline-stdin
description: A heredoc on a command that also receives a pipe overwrites stdin — use -c argument instead
metadata:
  type: feedback
---

In bash: `cmd1 | python3 - "$arg" << 'PYEOF'...PYEOF` — the heredoc replaces stdin of python3, so the pipe data from cmd1 is **lost**. python3 reads the heredoc as its script but receives no data.

**Rule:** Never combine a pipe and a heredoc on the same command when the pipe provides data.

**Fix:** Use `python3 -c '...'` with the code as a -c argument, letting stdin carry the pipe data.

**Why:** In ML-1F (2026-09-13), `_text_files_from_repo()` used `git ls-files --eol | python3 - << 'PYEOF'`. The function returned 0 files in self-test, causing vacuity guard to fire. Switching to `python3 -c '...'` fixed it immediately.

**How to apply:** When writing a filter function that processes stdin from a pipe using python3 (or awk/etc), always use the -c argument for the script code, not a heredoc.

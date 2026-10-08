---
name: feedback_depth_ignored_local_clone_use_file_protocol
description: git clone --depth N is silently ignored on local filesystem paths; use file:// to get a genuine shallow clone
metadata:
  type: feedback
---

`git clone --depth 1 /path/to/repo dest/` ignores `--depth` when the source is a local filesystem path. The result is a full clone. Git does print a warning to stderr (`--depth is ignored in local clones; use file:// instead`), but a casual scan of tool output may miss it — confirm with `git rev-parse --is-shallow-repository`.

**Why:** Git treats local paths as hardlink copies, not real clones, and depth semantics don't apply. The `--depth` flag only works when the transport layer is involved (SSH, HTTPS, or the `file://` pseudo-protocol, which forces socket transport even for local paths).

**How to apply:** When building test scenarios that require a genuine shallow clone (e.g., verifying `git rev-parse --is-shallow-repository` returns `true`, or testing behavior when a ref's history is truncated), always use:

```bash
git clone --depth 1 "file://$ORIGINAL_REPO_PATH" "$DEST"
# verify:
git -C "$DEST" rev-parse --is-shallow-repository  # must print "true"
```

A shallow clone via `file://` confirms that `git ls-tree` operates on tree objects (not history), which is critical when testing that `ls-tree` works even without full history — confirmed: exit 0, correct results with depth=1.

Measured: 2026-10-08, ML-0A Wave 0 for REQ-2026-10-08 (req_done_open_criteria fork detection).

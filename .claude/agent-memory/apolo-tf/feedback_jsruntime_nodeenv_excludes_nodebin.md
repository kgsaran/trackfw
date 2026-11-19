---
name: jsruntime-nodeenv-excludes-nodebin
description: nodeEnvNoTrackfw must NOT include the node binary dir; on macOS Homebrew, node and trackfw share /opt/homebrew/bin
metadata:
  type: feedback
---

When writing a Go test that calls node via absolute path and needs to prove ENOENT for
a missing binary, `nodeEnvNoTrackfw` must NOT include `filepath.Dir(nodePath)` in PATH.

**Why:** On macOS with Homebrew, both `node` and `trackfw` live in `/opt/homebrew/bin`.
Any env that includes the node binary's dir effectively includes the real `trackfw` binary
too. `spawnSync("trackfw")` finds it and exits 0 → arm passes for the wrong reason.

**How to apply:** Use only `/usr/bin:/bin` (+ sbin on Linux) for the minimal PATH.
Go calls node via its absolute path so PATH doesn't need to include the node binary dir.

See vault note: [[jsruntime-test-node-bin-dir-and-windows-bat-trap-2026-11-19]]

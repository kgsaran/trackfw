---
name: windows-spawnSync-needs-exe-not-bat
description: node spawnSync without shell:true won't find .bat files on Windows — use helper-process TestMain pattern with .exe copy
metadata:
  type: feedback
---

Node's `spawnSync("binary", args)` without `{shell: true}` calls CreateProcess directly on
Windows. CreateProcess only finds `.exe` and `.com`, NOT `.bat`. A `.bat` file fake binary
is invisible to spawnSync with the default `shell: false`.

**How to apply:** Use the **helper-process pattern** when writing a fake binary for node spawnSync tests:

1. Add `TestMain(m *testing.M)` to the test package. When `TRACKFW_FAKE_GUARD=1`, the
   test binary acts as the fake binary (read stdin, write to TRACKFW_STDIN_CAPTURE, exit 2
   on secret, exit 0 otherwise) — exits before `m.Run()` so no tests run.

2. `writeFakeTrackfw` copies `os.Executable()` to temp dir as `trackfw` (Unix) or
   `trackfw.exe` (Windows). The binary is already a `.exe` on Windows.

3. `nodeEnv` adds `TRACKFW_FAKE_GUARD=1` and `TRACKFW_FAKE_SECRET` to the subprocess env.

**Why:** The Windows CI job in quality.yml runs `go test ./...` with node (setup-node@v7),
so JS runtime tests DO run on Windows CI. A .bat fake that works on Unix will fail silently
on Windows — arm1 and arm3 would pass for the wrong reason (ENOENT instead of block).

See vault note: [[jsruntime-test-node-bin-dir-and-windows-bat-trap-2026-11-19]]

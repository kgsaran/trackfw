# JS runtime test: node bin dir is also trackfw dir on macOS; .bat not found without shell

**Date:** 2026-11-19
**REQ:** REQ-2026-11-17
**ML:** ML-5E2

## Root cause 1 — macOS: Homebrew puts node AND trackfw in the same dir

On macOS with Homebrew, both `node` and `trackfw` live in `/opt/homebrew/bin/`.

When a Go test builds a `nodeEnvNoTrackfw` to prove ENOENT, it must NOT include
`filepath.Dir(nodePath)` in PATH. Any env that includes the node binary's dir
effectively includes the real `trackfw` binary too.

**Measured:** `PATH=/opt/homebrew/bin:/usr/bin:/bin node driver.js` → spawnSync("trackfw") 
finds the real `trackfw`, exits 0, test passes for the wrong reason.

**Fix:** `nodeEnvNoTrackfw` uses ONLY `/usr/bin:/bin` (and `/usr/sbin:/sbin` on Linux).
Go calls node via its absolute path (`/opt/homebrew/bin/node`), so PATH need not contain
the node binary's directory.

## Root cause 2 — Windows: .bat fake trackfw not found by spawnSync without shell:true

Node's `spawnSync("trackfw", args)` without `{shell: true}` calls `execvp`/`CreateProcess`
directly. On Windows, `CreateProcess` only finds `.exe` and `.com` extensions, NOT `.bat`.

A `.bat` file as the fake `trackfw` binary works fine in the shell (`cmd.exe`) but is 
invisible to `spawnSync` with the default `shell: false`.

**Fix:** Use the **helper-process pattern**:
- `TestMain` checks `TRACKFW_FAKE_GUARD=1`. If set, reads stdin, writes to
  `TRACKFW_STDIN_CAPTURE`, exits 2 on `TRACKFW_FAKE_SECRET`, exits 0 otherwise.
- `writeFakeTrackfw` copies `os.Executable()` to temp dir as `trackfw` (Unix) or
  `trackfw.exe` (Windows).
- `nodeEnv` adds `TRACKFW_FAKE_GUARD=1` and `TRACKFW_FAKE_SECRET` to the subprocess env.

The test binary acts as itself when TRACKFW_FAKE_GUARD is NOT set, and as the fake binary
when it IS set. No shell needed. Works on Windows because it's a real `.exe`.

## Related

- Applies to any Go test that needs a fake binary invocable by node's spawnSync
- The Windows CI job in quality.yml (line 146) runs `go test ./...` with node from
  setup-node@v7 (line 169), so opencodeplugin tests DO run on Windows CI

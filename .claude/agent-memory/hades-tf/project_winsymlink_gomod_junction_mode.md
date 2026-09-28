---
name: project_winsymlink_gomod_junction_mode
description: go.mod directive (< 1.23 vs >= 1.23) determines junction ModeSymlink vs ModeIrregular in Go; standalone probe with go 1.21 gives opposite of production
metadata:
  type: project
---

Junction (`mklink /J`, `IO_REPARSE_TAG_MOUNT_POINT`) mode in Go depends on the `go` directive in `go.mod`, not Go toolchain version or architecture.

**Why:** Go 1.23 introduced `GODEBUG=winsymlink`. The default is set by go.mod:
- `go < 1.23` → `winsymlink=0` → junction = `ModeSymlink`
- `go >= 1.23` → `winsymlink=1` → junction = `ModeIrregular`

trackfw uses `go 1.25.2` → `winsymlink=1` → junction = `ModeIrregular`.

**A/B measured on VM (go1.27.0 windows/arm64):** same binary, same junction:
- go.mod `go 1.21`: junction = `ModeSymlink=true` (0x80001b6, `L...`)
- go.mod `go 1.25`: junction = `ModeIrregular=true` (0x801b6, `?...`)

**How to apply:** Always measure within the product package (`go test` inside the module), never with a standalone sonda with a separate go.mod. A sonda with `go 1.21` measures the opposite behavior.

**Fix: `ModeSymlink | ModeIrregular`** — correct under both winsymlink settings. No Go bump reopens the hole.

**Source:** `$(go env GOROOT)/src/os/types_windows.go` Go 1.27.0, lines 177-231.
**Vault note:** `vault/notes/winsymlink-gomod-governa-mode-de-juncao-2026-09-27.md`
**Parecer:** `docs/seguranca/2026-09-27-predicado-da-contencao-de-juncao-no-windows.md`
**REQ:** REQ-2026-09-27 | Issue #444

---
name: project_release_assets_mutable_goreleaser
description: trackfw release assets are mutable via gh release upload --clobber; .immutable=false; no Sigstore/cosign signing; checksums.txt provides transport integrity only
metadata:
  type: project
---

Release assets for `github.com/kgsaran/trackfw` are mutable.

Measured 2026-10-30 during ML-0A Wave 0 (REQ-2026-10-30 #637):

```
$ gh api repos/kgsaran/trackfw/releases/tags/v11.2.0 --jq '.immutable'
false
$ gh release upload --help 2>&1 | grep clobber
  --clobber   Delete and re-upload existing assets of the same name
```

`signs:` is absent from `.goreleaser.yaml`. `checksums.txt` guarantees transport integrity (SHA-256 of each asset) but not authenticity against a compromised write-access credential.

**Why:** `releases/latest/download/install.sh` download in consumer CI templates means any new release (or a clobber of an existing release asset) reaches all consumers that pin binaries but not the script. AC3 in REQ-2026-10-30 pins the script to the tag, limiting blast radius to a single version's asset — but does not close the mutability window.

**How to apply:** Any future recommendation to use `releases/latest` for script or binary downloads must weigh this. If the project ever adopts Sigstore/cosign, this note is obsolete for signing purposes, but `--clobber` still exists.

**Related:** [[project_mlscope_barrier_ml2a_verdict]] — same infrastructure, different attack vector

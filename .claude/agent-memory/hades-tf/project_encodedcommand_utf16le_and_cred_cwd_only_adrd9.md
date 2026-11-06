---
name: encodedcommand-utf16le-cred-cwd-only-adrd9
description: EncodedCommand requires UTF-16LE base64 (not UTF-8); credential guard cwd-only declared residual in ADR D9
metadata:
  type: project
---

**EncodedCommand encoding:** `-EncodedCommand` / `-enc` requires UTF-16LE base64. UTF-8 base64 (e.g. `Z2l0IHB1c2gK` for "git push") runs as garbled Unicode in PS and fails — want=0 not want=2. Correct UTF-16LE base64 for `git push`: `ZwBpAHQAIABwAHUAcwBoAA==`. Measured 2026-11-05 on pwsh 7.

**Why:** R3 rows in parecer previously used UTF-8 base64 — want column was wrong (a command that doesn't run is not a bypass). The advisor caught this same error class twice before (R-C8-dbl-bs, `$"git push"` bare).

**How to apply:** whenever constructing an `-EncodedCommand` test vector, generate with `printf 'cmd' | iconv -f UTF-8 -t UTF-16LE | base64`. Never use plain UTF-8 base64.

---

**Credential guard cwd-only (ADR D9):** project-scope credential guard only reads `trackfw.yaml` from cwd (no walk-up). Git-branch guard walks up. Asymmetry is intentional, declared in ADR D9 of `ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md`: "git-branch sobe até achar trackfw.yaml; credential só olha o cwd … Os dois preservados como estão." From `proj/sub/` without its own yaml, project-scope guard returns rc=0 (default warn). Global-scope guard defaults to block and fills the gap.

**Why:** same symptom as #662 but different cause (discovery, not parse). Regra Dura requires this distinction to be written. Finding classified as R-cred-2 (pre-declared residual).

**How to apply:** report as R-cred-2 with ADR D9 citation (porta fiel D9 preserves the asymmetry; the ADR does not explicitly declare it as a security residual). Classify as CORRIGIR only if ADR D9 is superseded or if README makes an unconditional promise of walk-up behavior.

---
name: req-adr-prefix-grant-live-bypass
description: req:/adr: in buildMLScopeOrchestrationSet grants arbitrary paths — go.mod reproduced live; fixed by p.8 in REQ-2026-10-29, same REQ as ml_scope union
metadata:
  type: project
---

`buildMLScopeOrchestrationSet` (`barrier.go:954-966`) adds `req:` and `adr:` frontmatter values
directly to the orchestration set without any prefix or path validation.

**Finding (reproduced live 2026-10-29):** `req: "go.mod"` grants `go.mod` as orchestration —
confirmed with `trackfw barrier` on a test roadmap.

**Severity:** medium — committer can grant any path by setting `req:` to any value; does not
require a separate REQ under Regra Dura because the fix (p.8 prescription) is in-scope for
REQ-2026-10-29 ml_scope union, same function (`buildMLScopeOrchestrationSet`), Wave 1 ML-1A.

**Variants measured:**
- `req: "go.mod"` → grants `go.mod` (top-level, not under req_dir)
- `req: "docs/req-x/f.md"` → sibling directory passes HasPrefix `"docs/req"` without slash guard
- `req: "docs/req/../../go.mod"` → traversal; if checked raw before clean, passes prefix check
- `req: "./docs/req/REQ-x.md"` → `path.Clean` → `docs/req/REQ-x.md` → must be ACCEPTED
- trailing slash in `cfg.REQDir = "docs/req/"` → `"docs/req//"` → drops all legit REQs (FP)

**Fix (p.8):** for each `req:`/`adr:` value:
1. `cleaned = path.Clean(filepath.ToSlash(raw))`
2. Reject if abs or starts with `..`
3. `req:` — accept only if `strings.HasPrefix(cleaned, path.Clean(filepath.ToSlash(cfg.REQDir))+"/")`
4. `adr:` — accept only if `strings.HasPrefix(cleaned, path.Clean(filepath.ToSlash(dir))+"/")`
   for some `dir` in `cfg.ADRDirs`
5. Discard silently otherwise.

**Corpus impact:** 14 roadmaps with `req:` outside `docs/req/` in HEAD (all done/ or abandoned/).
If reopened under Regra Dura, they lose the grant and require manual correction.

**Why:** same function as the ml_scope union feature; fixing in the same ML avoids a parallel REQ
on the same line (Regra Dura de Causa Raiz).

**How to apply:** when reviewing any ml_scope change touching `buildMLScopeOrchestrationSet`,
verify the prefix validation is present and that `path.Clean` is called before the HasPrefix check,
not after.

---
name: project-provenance-key-filepath-rel-read-mismatch
description: Go-only thirdparty_artifact_has_provenance validate rule builds its lookup key with filepath.Rel (native separator) against a JSON file whose keys are always written with "/" — a read-side bug, not a write-side one, found during the separator-portability REQ
metadata:
  type: project
---

Found 2026-09-01 during ML-0A (Wave 0) of
`ROADMAP-2026-09-01-caminho-dentro-de-artefato-versionado-usa-sempre-barra.md`
(REQ `REQ-2026-08-30-caminho-portavel-montado-com-separador-do-sistema-vaza-para-dentro-de-artefato-versionado.md`).

`internal/validator/validator_thirdparty_provenance.go:142` computes
`provenanceKey, _ := filepath.Rel(root, destination)` and looks it up in
`prov.Entries[provenanceKey]`, where `prov` is loaded from
`.trackfw/thirdparty-provenance.json`. The keys in that file are always written
with `/` — `internal/integrations/render.go:821`
(`ResolveThirdPartySkillDestination`) builds them as
`baseDir + "/thirdparty/" + slug + ".md"`, explicit concatenation, never
`filepath.Join`. On Windows, `filepath.Rel` returns a `\`-separated string, so
the lookup key never matches the `/`-separated key on disk — a legitimately
approved third-party artifact would show as "no provenance entry" on a Windows
`trackfw validate` run. Not reproduced live (would need a Windows machine or a
cross-compiled `filepath.Rel` — Go's implementation is platform-conditional at
compile time, not just runtime-configurable), so this is inferred from reading
the code, not measured.

This rule (`thirdparty_artifact_has_provenance`) is Go-only by design (documented
in `docs/cli-parity.md` as intentionally git-anchored, not ported to Node/Python),
so there's no cross-runtime parity gap here — just a latent bug in the Go
implementation itself.

**Why this matters beyond this one bug:** it's a *write is already correct, read
never tolerates the other grammar* pattern — the opposite of the REQ's headline
bug (`roadmap move` writes native separator into REQ frontmatter). Worth checking
for this same shape (correct writer + naive reader doing its own path-join for
lookup) anywhere else a JSON/frontmatter is keyed or valued by a path.

**How to apply:** if touching this area again, don't assume "the writer is
already using `/`" means the whole feature is safe — check every reader that
independently reconstructs a path for comparison/lookup, not just the writer.
Unfixed at time of finding; not yet handed to Wave 1 partitioning (architect's
call whether it enters this REQ's scope or becomes its own follow-up, since it
touches a file outside the REQ's original list).

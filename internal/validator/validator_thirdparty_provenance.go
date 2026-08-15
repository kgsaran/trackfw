package validator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/thirdparty"
)

// thirdPartyOrigin is the Claim.Origin value that marks a manifest artifact
// as installed by `third-party install` (ADR-2026-08-15 D11), rather than
// by the catalog. Kept in sync with the literal used in
// internal/commands/integrations_thirdparty.go — duplicated as a constant
// here (not imported) because that command lives in a different package
// and this is the only value this rule needs from it.
const thirdPartyOrigin = "thirdparty"

// validateThirdPartyArtifactHasProvenance implements the
// "thirdparty_artifact_has_provenance" rule (ADR-2026-08-15 D2), the real,
// git-anchored enforcement behind the TRACKFW_ORCHESTRATOR_SESSION
// guardrail (D2 is explicit that the env var is not a security control).
// It NEVER performs a network fetch (D6) — every check below reads only
// files already on disk (and, per this project's convention, versioned in
// the repository): .trackfw/integrations-manifest.json,
// .trackfw/thirdparty-provenance.json and
// .trackfw/thirdparty-quarantine/<checksum>.json.
//
// Two branches, both fatal (error, not warning — D2 does not appear in
// ruleDefaults, so it falls through to the "error" default in
// ruleSeverity):
//
//  1. A manifest artifact carries a claim with Origin == "thirdparty" but
//     .trackfw/thirdparty-provenance.json has no entry keyed by that
//     artifact's destination — nobody ever recorded who approved it.
//  2. A provenance entry exists, but its installed_sha256 cannot be
//     reconciled against what is actually on disk at the declared
//     destination — the artifact was tampered with after approval, or
//     installed outside the fetch/install flow.
//
// Branch 2 originally compared checksum_sha256 (D6: sha256 of the RAW bytes
// fetched, before normalization) against sha256(installed file) directly —
// literally following ADR-2026-08-15 D2's own text. That was WRONG: the
// installed file is always NormalizeThirdPartyContent(raw) —
// internal/integrations/render.go's normalizeMarkdown, TrimSpace(raw)+"\n"
// — which is generally NOT the identity function. For any legitimately
// fetched-and-approved artifact whose raw bytes were not already exactly
// trimmed with a single trailing newline (extremely common — e.g. any file
// with a trailing blank line), that literal comparison reported a false
// "checksum mismatch" on every validate run. manifest.Hash does not rescue
// this either: it is contentHash of the NORMALIZED content, the same
// domain as the installed file, not the raw domain checksum_sha256 lives
// in.
//
// A first fix (ML-3A) bridged the two domains via the quarantine record
// (.trackfw/thirdparty-quarantine/<checksum>.json): read the raw content
// back out of quarantine, normalize it, and compare against the installed
// file. That was correct but made a STAGING artifact
// (.trackfw/thirdparty-quarantine/, named and shaped like a directory
// meant to be pruned) a hard, unrecoverable dependency of this PERMANENT
// gate — delete or gitignore that directory and validate fails forever,
// with no way to reconstruct it (D6 forbids network access inside
// validate). ADR-2026-08-15 D2-bis replaces it:
//
//   - ProvenanceEntry gained InstalledSHA256 (thirdparty.ProvenanceEntry,
//     internal/thirdparty/provenance.go) — sha256 of the NORMALIZED bytes,
//     computed by executeThirdPartyInstall at install time, by the exact
//     code path that writes the destination file. checksum_sha256 is
//     UNCHANGED — it remains the raw-bytes approval anchor for D8c/
//     VerifyApproval, untouched by this rule.
//   - Branch (ii) now compares sha256(installed file) directly against
//     entry.InstalledSHA256 — both sides already in the normalized domain,
//     no bridge artifact needed.
//   - The quarantine record is still written and committed by `fetch`
//     (audit/reconstruction value), but its ABSENCE is no longer an error
//     for this rule — it is not read here at all anymore.
//
// This still never performs a network fetch and still only reads files
// already committed to the repository (D6).
func validateThirdPartyArtifactHasProvenance() ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	// Same rationale as validateGuardHookResolvable in
	// validator_credential_guard.go: os.Getwd() can return a symlinked path
	// (macOS /tmp -> /private/tmp) while manifest destinations are written
	// with the physical path by the integrations Manager, and Node/Python's
	// cwd resolution returns the physical path directly. Resolving symlinks
	// here keeps this rule's messages byte-identical across the 3 stacks.
	if resolvedRoot, symErr := filepath.EvalSymlinks(root); symErr == nil {
		root = resolvedRoot
	}

	manifest, err := integrations.LoadManifest(root)
	if err != nil {
		return nil, fmt.Errorf("thirdparty_artifact_has_provenance: %w", err)
	}

	var thirdPartyDestinations []string
	for destination, artifact := range manifest.Artifacts {
		for _, claim := range artifact.Claims {
			if claim.Origin == thirdPartyOrigin {
				thirdPartyDestinations = append(thirdPartyDestinations, destination)
				break
			}
		}
	}
	if len(thirdPartyDestinations) == 0 {
		return nil, nil
	}
	// Deterministic order — map iteration above is random, and this rule's
	// output must be stable across runs (and byte-identical across the 3
	// CLIs, which do not share Go's map iteration order).
	sort.Strings(thirdPartyDestinations)

	prov, err := thirdparty.LoadProvenance(root)
	if err != nil {
		return nil, fmt.Errorf("thirdparty_artifact_has_provenance: %w", err)
	}

	var msgs []string
	for _, destination := range thirdPartyDestinations {
		// Provenance keys are NOT the manifest's absolute destination —
		// verified empirically against the real install command
		// (internal/commands/integrations_thirdparty.go): VerifyApproval
		// and UpsertProvenanceEntry are called with rt.destination, the
		// project-root-relative (or "~/"-prefixed, global-scope) string
		// ResolveThirdPartySkillDestination returns, BEFORE
		// Manager.resolve() joins it against root to produce the absolute
		// path stored as the manifest key. Every claim reached here came
		// from the PROJECT manifest (root/.trackfw/integrations-manifest.json),
		// so its scope is always "project" (a global-scope claim would live
		// in the home manifest instead, which this rule intentionally never
		// reads — see this file's package doc: git-anchored detection
		// cannot reach ~/.trackfw/ regardless). filepath.Rel inverts
		// Manager.resolve's filepath.Join(root, relative) exactly.
		provenanceKey, relErr := filepath.Rel(root, destination)
		if relErr != nil {
			msgs = append(msgs, fmt.Sprintf(
				"thirdparty_artifact_has_provenance: %q could not be expressed relative to %q (%v) — cannot look up "+
					"its provenance entry",
				destination, root, relErr,
			))
			continue
		}
		entry, ok := prov.Entries[provenanceKey]
		if !ok {
			msgs = append(msgs, fmt.Sprintf(
				"thirdparty_artifact_has_provenance: %q is claimed as a third-party artifact but has no entry in "+
					".trackfw/thirdparty-provenance.json — obtain a favorable hades-tf review and record an approved "+
					"provenance entry for this destination before this can pass validate (D2 branch i)",
				destination,
			))
			continue
		}

		installed, readErr := os.ReadFile(destination)
		if readErr != nil {
			msgs = append(msgs, fmt.Sprintf(
				"thirdparty_artifact_has_provenance: %q is claimed as a third-party artifact with an approved "+
					"provenance entry, but the destination file could not be read (%v)",
				destination, readErr,
			))
			continue
		}

		if sha256Hex(installed) != entry.InstalledSHA256 {
			msgs = append(msgs, fmt.Sprintf(
				"thirdparty_artifact_has_provenance: %q — installed content does not match installed_sha256 %s "+
					"recorded in .trackfw/thirdparty-provenance.json — the artifact was modified after approval or "+
					"installed outside the fetch/install flow (D2 branch ii, D2-bis)",
				destination, entry.InstalledSHA256,
			))
		}
	}

	return msgs, nil
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

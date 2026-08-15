package validator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
//  2. A provenance entry exists, but its checksum_sha256 cannot be
//     reconciled against what is actually on disk at the declared
//     destination — the artifact was tampered with after approval, or
//     installed outside the fetch/install flow.
//
// Branch 2 needs a comment about a real imprecision in
// ADR-2026-08-15 D2, found while implementing this rule (reported here,
// not silently routed around — see this ML's delivery report and
// docs/roadmaps/.trackfw-attention.json at the time this was written):
// D2's own text says the branch fires when checksum_sha256 "não bate com o
// SHA-256 do conteúdo instalado no destino declarado". Taken completely
// literally that would mean comparing checksum_sha256 (which D6 defines as
// sha256 of the RAW bytes fetched, before normalization) against
// sha256(bytes of the installed file). But the installed file is always
// NormalizeThirdPartyContent(raw) — internal/integrations/render.go's
// normalizeMarkdown, TrimSpace(raw)+"\n" — which is generally NOT the
// identity function. For any legitimately fetched-and-approved artifact
// whose raw bytes were not already exactly trimmed with a single trailing
// newline (extremely common — e.g. any file with a trailing blank line),
// that literal comparison would report a false "checksum mismatch" on
// every validate run. manifest.Hash does not rescue this either: it is
// contentHash of the NORMALIZED content, the same domain as the installed
// file, not the raw domain checksum_sha256 lives in.
//
// Resolution implemented below: use the quarantine record
// (.trackfw/thirdparty-quarantine/<checksum>.json) as the git-anchored
// bridge between the two domains — it persists after install (nothing
// deletes it) and is not gitignored, so it is exactly as auditable as the
// provenance entry itself. The check becomes:
//
//   - the quarantine record for checksum_sha256 must exist (fail-closed,
//     D8f) and must be internally self-consistent
//     (sha256(base64-decoded content) == checksum_sha256, guarding against
//     a hand-edited quarantine file);
//   - the destination file's bytes must equal
//     NormalizeThirdPartyContent(quarantine content) byte-for-byte — this
//     is the actual drift/tamper check, comparing like domains.
//
// This still never performs a network fetch and still only reads files
// already committed to the repository (D6). It is a deviation from D2's
// literal phrasing, flagged for architect review rather than implemented
// silently.
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

		quarantineEntry, qErr := thirdparty.ReadQuarantine(root, entry.ChecksumSHA256)
		if qErr != nil {
			msgs = append(msgs, fmt.Sprintf(
				"thirdparty_artifact_has_provenance: %q has a provenance entry for checksum %s, but "+
					".trackfw/thirdparty-quarantine/%s.json could not be read (%v) — the quarantine record is "+
					"required to verify the approval against the installed content (D2 branch ii, fail-closed per D8f)",
				destination, entry.ChecksumSHA256, entry.ChecksumSHA256, qErr,
			))
			continue
		}

		rawContent, decodeErr := quarantineEntry.DecodeContent()
		if decodeErr != nil {
			msgs = append(msgs, fmt.Sprintf(
				"thirdparty_artifact_has_provenance: %q — quarantine record for checksum %s has an undecodable "+
					"content_base64 (%v)",
				destination, entry.ChecksumSHA256, decodeErr,
			))
			continue
		}
		if sha256Hex(rawContent) != entry.ChecksumSHA256 {
			msgs = append(msgs, fmt.Sprintf(
				"thirdparty_artifact_has_provenance: %q — quarantine record for checksum %s is not "+
					"self-consistent (recomputed checksum does not match its own filename); the record may have "+
					"been hand-edited",
				destination, entry.ChecksumSHA256,
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

		expected := normalizeThirdPartyForValidation(rawContent)
		if string(installed) != string(expected) {
			msgs = append(msgs, fmt.Sprintf(
				"thirdparty_artifact_has_provenance: %q — installed content does not match the checksum %s "+
					"approved in .trackfw/thirdparty-provenance.json (verified via its quarantine record) — the "+
					"artifact was modified after approval or installed outside the fetch/install flow (D2 branch ii)",
				destination, entry.ChecksumSHA256,
			))
		}
	}

	return msgs, nil
}

// normalizeThirdPartyForValidation replicates
// internal/integrations/render.go's normalizeMarkdown (TrimSpace + a
// single trailing newline) byte-for-byte, without importing that package's
// full render surface for one line of logic — same rationale as
// thirdparty.Checksum's doc comment about not reusing manager.go's
// contentHash when the signature does not fit cleanly.
func normalizeThirdPartyForValidation(content []byte) []byte {
	return []byte(strings.TrimSpace(string(content)) + "\n")
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

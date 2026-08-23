package commands

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/thirdparty"
	"github.com/spf13/cobra"
)

// thirdPartyProvenanceRule is the name of the trackfw validate rule that is
// the real (git-anchored) enforcement behind the orchestrator-session
// guardrail below — implemented in ML-3A of this roadmap, not here. Named
// as a constant so the guardrail message and any future test asserting its
// wording never drift apart silently.
const thirdPartyProvenanceRule = "thirdparty_artifact_has_provenance"

// thirdPartyFetch is the indirection used to invoke thirdparty.Fetch,
// mirroring the identityWizardRunner/promptInstallScopeRunner package-var
// pattern already used in this package (integrations_flags.go). Tests
// substitute this instead of hitting real network, keeping fetch-command
// tests compatible with TRACKFW_DISABLE_EXTERNAL_COMMANDS=1.
var thirdPartyFetch = thirdparty.Fetch

// thirdPartySlugPattern is the allowed shape of a third-party artifact slug
// (used to build its destination filename, ADR D5). Deliberately
// conservative: lowercase alnum plus "._-", 1-64 chars, must start
// alphanumeric. Rejects "/", "..", NUL and anything else that could escape
// the "thirdparty/" subdirectory Manager.resolve is expected to confine it
// to.
var thirdPartySlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// thirdPartyChecksumPattern validates --checksum is a well-formed SHA-256
// hex digest before it is used to build filesystem paths (quarantine
// lookup) or sliced (CatalogVersion below) — a short or malformed value
// must fail loudly here, not panic on a slice bound or silently mismatch a
// different quarantine record.
var thirdPartyChecksumPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func newIntegrationsThirdPartyCmd(kind integrations.ItemKind) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "third-party",
		Short: fmt.Sprintf("Fetch and install third-party %s content under a two-phase quarantine gate", kind),
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newThirdPartyFetchCmd(kind), newThirdPartyInstallCmd(kind))
	return cmd
}

// checkOrchestratorGuardrail implements D2: TRACKFW_ORCHESTRATOR_SESSION is
// a guardrail against accidental invocation from a plain terminal, never a
// security control — it is trivially set by anyone with shell access. The
// real enforcement is the thirdPartyProvenanceRule check in `trackfw
// validate` (ML-3A), which is git-anchored per ADR-2026-08-12. This message
// must never present the env var as prevention.
func checkOrchestratorGuardrail() error {
	if os.Getenv("TRACKFW_ORCHESTRATOR_SESSION") != "" {
		return nil
	}
	return fmt.Errorf(
		"refused: TRACKFW_ORCHESTRATOR_SESSION is not set. This is a guardrail against accidental "+
			"invocation from a plain terminal, not a security control — it does not resist anyone who "+
			"already has shell access. The real enforcement is the %q rule checked by `trackfw validate`, "+
			"which detects any third-party artifact committed without a matching, checksum-linked "+
			"provenance entry. If this is an orchestrated agent session, set TRACKFW_ORCHESTRATOR_SESSION=1",
		thirdPartyProvenanceRule,
	)
}

func thirdPartyEntryKind(kind integrations.ItemKind) string {
	if kind == integrations.KindAgents {
		return "agent"
	}
	return "skill"
}

// --- Fase 1: fetch ---

type thirdPartyFetchOptions struct {
	targets      []string
	forceMarkers bool
}

func newThirdPartyFetchCmd(kind integrations.ItemKind) *cobra.Command {
	opts := thirdPartyFetchOptions{}
	cmd := &cobra.Command{
		Use:   "fetch <url>",
		Short: "Download third-party content into quarantine for review (never installs)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeThirdPartyFetch(cmd, kind, args[0], &opts)
		},
	}
	cmd.Flags().StringSliceVar(&opts.targets, "targets", nil, "target CLIs this artifact is intended for (recorded for review only; confirmed again at install)")
	cmd.Flags().BoolVar(&opts.forceMarkers, "force-thirdparty-markers", false, "override refusal on boundary-redefinition markers (D3); recorded, never silent")
	return cmd
}

func executeThirdPartyFetch(cmd *cobra.Command, kind integrations.ItemKind, rawURL string, opts *thirdPartyFetchOptions) error {
	if err := checkOrchestratorGuardrail(); err != nil {
		return err
	}
	raw, err := thirdPartyFetch(rawURL)
	if err != nil {
		return err
	}
	matched := thirdparty.CheckMarkers(raw)
	if len(matched) > 0 && !opts.forceMarkers {
		return fmt.Errorf(
			"refused: content matches boundary-redefinition marker(s) %v (D3); pass --force-thirdparty-markers "+
				"to quarantine it anyway (recorded in marker_check, never installed without approval)",
			matched,
		)
	}

	manager, err := integrationsManager()
	if err != nil {
		return err
	}
	entry := thirdparty.NewQuarantineEntry(rawURL, raw, matched, thirdPartyEntryKind(kind), opts.targets)
	if err := thirdparty.WriteQuarantine(manager.ProjectRoot, entry); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "quarantined: %s\n", thirdparty.QuarantinePath(manager.ProjectRoot, entry.ChecksumSHA256))
	fmt.Fprintf(cmd.OutOrStdout(), "checksum: %s\n", entry.ChecksumSHA256)
	if len(matched) > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "warning: marker check failed (matched=%v); --force-thirdparty-markers was used\n", matched)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "next: obtain a favorable hades-tf review, record its provenance entry keyed by the resolved "+
		"destination(s), then run `%s third-party install --checksum %s --targets <t1,t2> [--apply-to <agent-id,...>]`\n",
		kind, entry.ChecksumSHA256)
	return nil
}

// --- Fase 2: install ---

// thirdPartyGlobalScopeWarning and thirdPartyGlobalScopeRefusal are the
// D4-bis literal strings for --scope global: printed/returned verbatim by
// all 3 CLIs (the roadmap's AC requires identical wording), so they are
// named constants here rather than inlined, mirroring
// thirdPartyProvenanceRule's rationale above.
const thirdPartyGlobalScopeWarning = "warning: --scope global installs outside the project tree; this artifact will NEVER be verified by `trackfw validate` (the \"" + thirdPartyProvenanceRule + "\" rule only scans the project's own manifest — an artifact under a home directory is invisible to it, per ADR-2026-08-12)."
const thirdPartyGlobalScopeRefusal = "install to --scope global requires --yes-global-scope-unverified as its own explicit confirmation (D4-bis), distinct from --yes-i-trust-this-source: it confirms you understand `trackfw validate` will never verify this installation"

type thirdPartyInstallOptions struct {
	checksum         string
	slug             string
	targets          []string
	applyTo          []string
	scope            string
	yesITrust        bool
	yesGlobalScopeOK bool
}

func newThirdPartyInstallCmd(kind integrations.ItemKind) *cobra.Command {
	opts := thirdPartyInstallOptions{}
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Consume a quarantined artifact into its resolved destination(s), requiring a prior checksum-linked approval",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return executeThirdPartyInstall(cmd, kind, &opts)
		},
	}
	cmd.Flags().StringVar(&opts.checksum, "checksum", "", "SHA-256 checksum of the quarantined artifact (required)")
	cmd.Flags().StringVar(&opts.slug, "slug", "", "destination file slug (default: derived from the quarantined URL)")
	cmd.Flags().StringSliceVar(&opts.targets, "targets", nil, "target CLIs to install the skill file into (required)")
	cmd.Flags().StringSliceVar(&opts.applyTo, "apply-to", nil, "catalog agent item IDs whose rendered file gets a reference to this artifact (optional; never inferred silently — AC3)")
	cmd.Flags().StringVar(&opts.scope, "scope", "", "installation scope: project or global (default: project — D4)")
	cmd.Flags().BoolVar(&opts.yesITrust, "yes-i-trust-this-source", false, "required in non-interactive mode (AC1)")
	cmd.Flags().BoolVar(&opts.yesGlobalScopeOK, "yes-global-scope-unverified", false, "required, in addition to --yes-i-trust-this-source, for --scope global: confirms this installation will never be verified by `trackfw validate` (D4-bis)")
	return cmd
}

// resolveThirdPartyScope is deliberately separate from resolveScope
// (integrations_flags.go): third-party's default is "project" (D4), the
// opposite of the catalog's "global" default (ADR-2026-07-25 D1), and
// resolveScope's existing tests assert the catalog default — touching it
// would risk that contract. Same flag-set precedence discipline: an
// explicit --scope is detected via cmd.Flags().Changed, never by comparing
// opts.scope's value against "project" (that comparison cannot distinguish
// an explicit `--scope project` from the flag's unset zero value).
func resolveThirdPartyScope(cmd *cobra.Command, opts *thirdPartyInstallOptions) error {
	if cmd.Flags().Changed("scope") {
		if opts.scope != "project" && opts.scope != "global" {
			return fmt.Errorf("invalid --scope %q: use project or global", opts.scope)
		}
		return nil
	}
	opts.scope = "project"
	return nil
}

// deriveSlug produces a filesystem-safe slug from a quarantined artifact's
// source URL when --slug is not given.
func deriveSlug(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("cannot derive slug from URL %q: %w", rawURL, err)
	}
	base := path.Base(parsed.Path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ToLower(base)
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	slug := strings.Trim(b.String(), "-._")
	if slug == "" || !thirdPartySlugPattern.MatchString(slug) {
		return "", fmt.Errorf("cannot derive a safe slug from URL %q; pass --slug explicitly", rawURL)
	}
	return slug, nil
}

type resolvedThirdPartyTarget struct {
	targetID    string
	surfaceID   string
	destination string
}

func executeThirdPartyInstall(cmd *cobra.Command, kind integrations.ItemKind, opts *thirdPartyInstallOptions) error {
	if err := checkOrchestratorGuardrail(); err != nil {
		return err
	}
	if opts.checksum == "" {
		return fmt.Errorf("install requires --checksum")
	}
	if !thirdPartyChecksumPattern.MatchString(opts.checksum) {
		return fmt.Errorf("invalid --checksum %q: expected a 64-character lowercase SHA-256 hex digest", opts.checksum)
	}
	if len(opts.targets) == 0 {
		return fmt.Errorf("install requires --targets")
	}
	if err := resolveThirdPartyScope(cmd, opts); err != nil {
		return err
	}
	// D4-bis — print the warning as early as possible, before any other
	// output, so it is visible even if a later step aborts the command.
	if opts.scope == "global" {
		fmt.Fprintln(cmd.OutOrStdout(), thirdPartyGlobalScopeWarning)
	}

	manager, err := integrationsManager()
	if err != nil {
		return err
	}

	entry, err := thirdparty.ReadQuarantine(manager.ProjectRoot, opts.checksum)
	if err != nil {
		return err
	}
	content, err := entry.DecodeContent()
	if err != nil {
		return err
	}
	// D8c / TOCTOU close: the quarantine record's filename IS its checksum,
	// but that alone does not prove content_base64 hasn't been edited in
	// place since approval. Recompute over the decoded bytes and require
	// both the record's own field and the caller-supplied --checksum to
	// agree with it.
	recomputed := thirdparty.Checksum(content)
	if recomputed != opts.checksum || entry.ChecksumSHA256 != opts.checksum {
		return fmt.Errorf("refused: quarantined content for %q no longer matches checksum %s (TOCTOU guard, D8c)", entry.URL, opts.checksum)
	}

	slug := opts.slug
	if slug == "" {
		slug, err = deriveSlug(entry.URL)
		if err != nil {
			return err
		}
	} else if !thirdPartySlugPattern.MatchString(slug) {
		return fmt.Errorf("invalid --slug %q: use lowercase alphanumerics, '.', '_' or '-'", slug)
	}

	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return err
	}

	resolvedTargets := make([]resolvedThirdPartyTarget, 0, len(opts.targets))
	for _, targetID := range opts.targets {
		dest, surfaceID, err := integrations.ResolveThirdPartySkillDestination(catalog, targetID, opts.scope, slug)
		if err != nil {
			return err
		}
		resolvedTargets = append(resolvedTargets, resolvedThirdPartyTarget{targetID: targetID, surfaceID: surfaceID, destination: dest})
	}

	// D5/AC3 preconditions for --apply-to are validated here, BEFORE any
	// write happens (including the skill file below). Validating only
	// after `manager.Install` of the skill file — as an earlier version of
	// this command did — left a partial state on the precondition failure
	// path: skill file written, registry entry upserted, but no reference
	// actually injected, and no flag on this command to force past it.
	// Fail everything up front instead.
	tpAgentModels, tpWarnMsg := config.ResolveAgentModels(opts.scope, manager.HomeDir, manager.ProjectRoot)
	if tpWarnMsg != "" {
		fmt.Fprintln(os.Stderr, tpWarnMsg)
	}
	var ident identity.Config
	if len(opts.applyTo) > 0 {
		ident, err = identity.Load(manager.HomeDir)
		if err != nil {
			return err
		}
		for _, agentID := range opts.applyTo {
			if _, ok := catalog.Item(integrations.KindAgents, agentID); !ok {
				return fmt.Errorf("unknown agent item %q", agentID)
			}
			for _, rt := range resolvedTargets {
				agentPlans, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
					Kind: integrations.KindAgents, Targets: []string{rt.targetID}, Items: []string{agentID},
					Scope: opts.scope, Identity: ident, ProjectRoot: manager.ProjectRoot,
					AgentModels: tpAgentModels,
				})
				if err != nil {
					return err
				}
				if len(agentPlans) == 0 {
					return fmt.Errorf("target %s has no supported agents surface for item %q", rt.targetID, agentID)
				}
				inspection, err := manager.Inspect(agentPlans[0])
				if err != nil {
					return err
				}
				// ADR imprecision found and resolved here (reported, not
				// silently worked around): D5/D8 never say which scope the
				// referencing agent artifact must be at, and D4 gives
				// third-party a DIFFERENT default scope (project) than the
				// catalog default (global, ADR-2026-07-25 D1) — so in the
				// common case there is no `.claude/agents/trackfw-<id>.md`
				// to inject into, only `~/.claude/agents/...`, and a
				// project-relative skill path injected into a global
				// (cross-project) agent file would be broken for every
				// other project sharing that home-scoped file. Resolution:
				// require the agent to already be installed, owned, and
				// NOT hand-modified at the SAME scope as the skill; fail
				// loudly with the exact remediation instead of silently
				// skipping or installing at a mismatched scope (AC3
				// forbids silent decisions).
				switch {
				case !inspection.Managed || inspection.State == integrations.StateNotInstalled:
					return fmt.Errorf(
						"cannot attach reference: agent %q is not installed at --scope %s for target %s; run "+
							"`trackfw agents install --scope %s --targets %s --items %s` first",
						agentID, opts.scope, rt.targetID, opts.scope, rt.targetID, agentID,
					)
				case inspection.State == integrations.StateModified:
					return fmt.Errorf(
						"cannot attach reference: agent %q at --scope %s for target %s was modified outside trackfw; run "+
							"`trackfw agents update --scope %s --targets %s --items %s --force` first",
						agentID, opts.scope, rt.targetID, opts.scope, rt.targetID, agentID,
					)
				}
			}
		}
	}

	// AC1 — always show content and every resolved destination before
	// writing anything, in both interactive and non-interactive mode.
	fmt.Fprintf(cmd.OutOrStdout(), "URL: %s\nChecksum: %s\n\n--- content ---\n%s\n--- end content ---\n\n", entry.URL, opts.checksum, string(content))
	fmt.Fprintf(cmd.OutOrStdout(), "Resolved destination(s) (scope=%s):\n", opts.scope)
	for _, rt := range resolvedTargets {
		fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", rt.targetID, rt.destination)
	}

	if !integrationsStdinIsTTY() {
		if !opts.yesITrust {
			return fmt.Errorf("install requires --yes-i-trust-this-source in non-interactive mode (AC1)")
		}
	} else if !opts.yesITrust {
		confirmed := false
		if err := huh.NewConfirm().
			Title("Install this third-party content at the destination(s) shown above?").
			Value(&confirmed).
			Run(); err != nil {
			return err
		}
		if !confirmed {
			return fmt.Errorf("install cancelled")
		}
	}

	// D4-bis — global scope requires ITS OWN explicit confirmation, beyond
	// --yes-i-trust-this-source: that flag only confirms trust in the
	// content's source, while --yes-global-scope-unverified confirms the
	// user understands `trackfw validate` will never verify this specific
	// installation (the two are orthogonal — decision by KG, 2026-08-15,
	// superseding the ML-3A choice to collapse both into
	// --yes-i-trust-this-source).
	if opts.scope == "global" && !opts.yesGlobalScopeOK {
		return fmt.Errorf(thirdPartyGlobalScopeRefusal)
	}

	// D8c — the TOCTOU-closing approval check, verified per resolved
	// destination (provenance is keyed by destination, not by checksum
	// alone — a checksum approved for one destination is not automatically
	// approved for another).
	for _, rt := range resolvedTargets {
		if err := thirdparty.VerifyApproval(manager.ProjectRoot, opts.checksum, rt.destination); err != nil {
			return fmt.Errorf("not approved for %s: %w", rt.destination, err)
		}
	}
	// D3 — a failed marker check is not fatal to fetch (--force-thirdparty-markers
	// already overrode that refusal), but install requires the approver to
	// have knowingly recorded marker_override in the provenance entry for
	// each destination — an unset override here would let a forced fetch
	// slip through install without anyone having acknowledged the failed
	// check at approval time.
	if entry.MarkerCheck.Result == "fail" {
		prov, err := thirdparty.LoadProvenance(manager.ProjectRoot)
		if err != nil {
			return err
		}
		for _, rt := range resolvedTargets {
			if !prov.Entries[rt.destination].MarkerOverride {
				return fmt.Errorf(
					"refused: %s failed the D3 boundary marker check (matched=%v) and its provenance entry lacks marker_override=true",
					rt.destination, entry.MarkerCheck.MatchedMarkers,
				)
			}
		}
	}

	// Write the skill file(s) — always through Manager, never os.WriteFile
	// raw. Claim.Kind is always KindSkills regardless of which lifecycle
	// (`skills`/`agents`) invoked this command: the artifact always lives
	// under "skills/thirdparty/" per D5, and detectNameCollision in
	// manager.go only scans KindAgents ".md" siblings for frontmatter
	// "name:" collisions — routing third-party skills through KindAgents
	// would risk spurious collisions against arbitrary third-party
	// frontmatter.
	normalized := integrations.NormalizeThirdPartyContent(content)
	plans := make([]integrations.PlannedArtifact, 0, len(resolvedTargets))
	for _, rt := range resolvedTargets {
		plans = append(plans, integrations.PlannedArtifact{
			Claim: integrations.Claim{
				Target: rt.targetID, Surface: rt.surfaceID, Scope: opts.scope,
				Kind: integrations.KindSkills, Item: "thirdparty-" + slug,
				Origin: "thirdparty", // D11 — marks this destination for thirdparty_artifact_has_provenance
			},
			Destination:    rt.destination,
			Content:        normalized,
			CatalogVersion: "thirdparty:" + opts.checksum[:12],
			SupportLevel:   "native",
		})
	}
	if err := manager.Install(plans, false); err != nil {
		return err
	}

	// D2-bis — record InstalledSHA256 (SHA-256 of the NORMALIZED bytes,
	// i.e. sha256(normalized) above) on each destination's existing
	// provenance entry, now that the install actually succeeded. This is
	// computed by this exact code path, the same one that just wrote
	// `normalized` to disk via manager.Install — see provenance.go's
	// ProvenanceEntry doc for why this must be a distinct field from
	// ChecksumSHA256 (the raw-bytes approval anchor, written externally by
	// the approver and never touched here). Only InstalledSHA256 changes;
	// every other field the approver wrote (url, checksum_sha256,
	// approved_by, review_reference, scope, marker_override) is preserved
	// verbatim by loading the entry first and mutating only this one field.
	// rt.destination is already the provenance key: the
	// project-root-relative (pre-Manager.resolve()) string
	// ResolveThirdPartySkillDestination returns — the same value
	// VerifyApproval was just called with above, NOT the absolute manifest
	// destination (see this file's doc / docs/cli-parity.md "Nota de
	// paridade crítica" for why those two domains must not be confused).
	installedSHA256 := thirdparty.Checksum(normalized)
	for _, rt := range resolvedTargets {
		prov, err := thirdparty.LoadProvenance(manager.ProjectRoot)
		if err != nil {
			return err
		}
		provEntry := prov.Entries[rt.destination]
		provEntry.InstalledSHA256 = installedSHA256
		if err := thirdparty.UpsertProvenanceEntry(manager.ProjectRoot, rt.destination, provEntry); err != nil {
			return err
		}
	}

	// D5 — attach references to the requested catalog agent artifacts, at
	// the SAME scope as the skill file just installed. Preconditions
	// (installed, owned, not hand-modified, at this exact scope) were
	// already validated above, before the skill file was written — this
	// loop only performs writes.
	if len(opts.applyTo) > 0 {
		for _, agentID := range opts.applyTo {
			for _, rt := range resolvedTargets {
				if err := integrations.UpsertThirdPartyReference(manager.ProjectRoot, rt.targetID, agentID, integrations.ThirdPartyReference{
					Slug: slug, Destination: rt.destination, URL: entry.URL,
				}); err != nil {
					return err
				}

				// Re-render now so the on-disk artifact reflects the new
				// reference immediately — not only on the next `agents
				// update`. BuildPlans picks up the registry entry just
				// written via ApplyThirdPartyReferences (plan.go).
				agentPlans, err := integrations.BuildPlans(catalog, integrations.PlanRequest{
					Kind: integrations.KindAgents, Targets: []string{rt.targetID}, Items: []string{agentID},
					Scope: opts.scope, Identity: ident, ProjectRoot: manager.ProjectRoot,
					AgentModels: tpAgentModels,
				})
				if err != nil {
					return err
				}
				if err := manager.Update(agentPlans, false); err != nil {
					return err
				}
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "installed: %d destination(s)\n", len(plans))
	return nil
}

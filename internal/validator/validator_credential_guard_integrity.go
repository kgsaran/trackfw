package validator

import (
	"fmt"
	"os"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
)

// ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A.
// ADR: docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-
// resposta-e-deteccao-ancorada-no-git.md (Emenda 1: âncora POR ALVO, decidida na Barreira B0).
//
// Two independent rules, two independent anchors, two independent severities — deliberately not
// merged into one rule, because each has its own configurability need (rules: <name>: <severity>
// in trackfw.yaml operates per rule name) and the ADR mandates different defaults for each:
//
//   - credential_guard_script_integrity: anchor is the BINARY'S OWN TEMPLATE (this trackfw
//     build's credentialGuardScriptReference). Covers overwrite of
//     scripts/trackfw-credential-guard.sh. Severity default "warning" (see ruleDefaults in
//     validator.go) — the script carries no version marker, so this rule cannot distinguish
//     legitimate drift (user hasn't run `trackfw update` since a template bump) from real
//     tampering (ADR Emenda 3). The message is causally neutral for that reason: it never asserts
//     tampering.
//   - credential_guard_mode_downgrade: anchor is the git HEAD copy of trackfw.yaml, compared
//     SEMANTICALLY and DIRECTIONALLY (block at HEAD -> non-block on disk), never byte-diffed.
//     Severity default "error" (falls through to ruleSeverity's default — no ruleDefaults entry
//     needed): unlike the script, `credential_guard.mode` carries no drift-prone generated form —
//     a legitimate relax (block -> warn) that hasn't been committed yet is the one false-positive
//     case considered, and it is accepted: the fix in that case is `git commit`, which is exactly
//     the auditable trail this whole detection direction exists to force (ADR Decision point 5:
//     the mode downgrade is "a mais diffável" of the three vias — that is the point being spent
//     here, not hedged away with a warning severity).

// validateCredentialGuardScriptIntegrity is the "credential_guard_script_integrity" rule: compares
// the on-disk scripts/trackfw-credential-guard.sh against the template this trackfw binary would
// generate. Silent (no violation, no error) when the script does not exist — that absence is
// credential_guard_hook_resolvable's job, not this rule's; duplicating it here would double-report
// the same underlying condition under two rule names.
func validateCredentialGuardScriptIntegrity() ([]string, error) {
	const relPath = "scripts/trackfw-credential-guard.sh"

	content, err := readRegularFile(relPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio,
		// ML-1C: was `return nil, fmt.Errorf(...)` — hades-tf's ML-1B barrier found this was the
		// SERIOUS case among the *_script_integrity family: it ABORTED the entire `trackfw
		// validate` run (exit 1, non-JSON stdout, every other rule's result lost with it) on the
		// first unreadable script, in Go project-scope only. A consumer of `trackfw validate --json`
		// in CI that parses stdout as JSON gets neither JSON nor a diagnosable message — the exact
		// contract break this whole REQ exists to close. Reported as a violation of THIS rule
		// instead, matching validateGitBranchGuardScriptIntegrity's sibling fix
		// (validator_git_branch_guard.go) and the config-file read-error branches
		// (validateGuardHookResolvable). readRegularFile (regularfile.go) also closes the FIFO
		// hang hades-tf found for config files — this script read shares the same primitive.
		return []string{fmt.Sprintf(
			"%s could not be read — trackfw cannot tell whether this script matches the template "+
				"it should; fix the file, or run `trackfw update` to regenerate it",
			relPath,
		)}, nil
	}

	if string(content) == credentialGuardScriptReference {
		return nil, nil
	}

	return []string{fmt.Sprintf(
		"%s content diverges from the template this version of trackfw generates — "+
			"if you did not edit this file by hand, run `trackfw update` to regenerate it",
		relPath,
	)}, nil
}

// credentialGuardModeBlockLookbehindLines mirrors the shell script's own resolution of
// credential_guard.mode (credentialGuardModeResolution in internal/generators/scaffold.go, `grep
// -A 5 '^credential_guard:'`): the block value key is found on the same line as, or within the 5
// lines following, a line that starts with "credential_guard:". Deliberately the SAME lightweight
// line-scan the shipped script itself uses to read this value — not a full YAML parser — so this
// rule's notion of "what credential_guard.mode resolves to" matches what actually runs at hook
// time, rather than diverging on some YAML edge case a real parser would handle differently.
const credentialGuardModeBlockLookbehindLines = 5

// extractCredentialGuardMode scans content for a top-level "credential_guard:" line and returns
// the value of the "mode:" key found within it (own line, or one of the next
// credentialGuardModeBlockLookbehindLines lines). ok is false when no "credential_guard:" line
// exists at all, OR when it exists but no "mode:" key is found within the lookbehind window — both
// cases mean "no explicit mode value to reason about", which the two callers below treat
// identically (no anchor / not a block value).
func extractCredentialGuardMode(content string) (mode string, ok bool) {
	lines := strings.Split(content, "\n")

	start := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "credential_guard:") {
			start = i
			break
		}
	}
	if start == -1 {
		return "", false
	}

	end := start + 1 + credentialGuardModeBlockLookbehindLines
	if end > len(lines) {
		end = len(lines)
	}

	for _, l := range lines[start:end] {
		trimmed := strings.TrimSpace(l)
		if !strings.Contains(trimmed, "mode:") {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "mode:"))
		if h := strings.Index(rest, "#"); h >= 0 {
			rest = strings.TrimSpace(rest[:h])
		}
		rest = strings.Trim(rest, `"'`)
		return rest, true
	}

	return "", false
}

// headTrackfwYAML returns the content of trackfw.yaml as committed at HEAD, resolved relative to
// the current working directory (not necessarily the git toplevel — `trackfw validate` can run
// from a subdirectory). ok is false whenever there is no usable anchor: not a git worktree, no
// commits yet, or trackfw.yaml not tracked at HEAD — every one of these is a "no anchor, stay
// silent" case per this rule's contract, never an error.
func headTrackfwYAML() (content string, ok bool) {
	if !isGitWorktree(".") {
		return "", false
	}
	if err := gitCommand(".", "rev-parse", "--verify", "HEAD").Run(); err != nil {
		// No commits yet.
		return "", false
	}

	out, err := gitCommand(".", "show", "HEAD:./trackfw.yaml").Output()
	if err != nil {
		// Not tracked at HEAD (new/untracked file, or trackfw.yaml doesn't exist at HEAD).
		return "", false
	}
	return string(out), true
}

// validateCredentialGuardModeDowngrade is the "credential_guard_mode_downgrade" rule: fires only
// when credential_guard.mode was explicitly "block" at HEAD and the current on-disk trackfw.yaml
// no longer resolves to "block" (explicit "warn", an unrecognized value, or the key/file missing
// altogether — all of which the shipped script itself would fall back to "warn" for, per
// credentialGuardModeResolution's DEFAULT_MODE="warn" for the project variant).
//
// Silent (no violation) whenever HEAD is not "block": that is "no anchor to detect a downgrade
// from", not "nothing wrong" — see the file-level comment above for why this is the correct
// reading of "trackfw.yaml sem a chave credential_guard.mode -> silêncio" (it is about the anchor
// side, HEAD; the disk side lacking the key is exactly the downgrade this rule exists to catch,
// and is never treated as "no anchor").
func validateCredentialGuardModeDowngrade() ([]string, error) {
	headContent, ok := headTrackfwYAML()
	if !ok {
		return nil, nil
	}

	headMode, _ := extractCredentialGuardMode(headContent)
	if headMode != "block" {
		return nil, nil
	}

	diskContent, err := readRegularFile("trackfw.yaml")
	if err != nil {
		if !os.IsNotExist(err) {
			// Non-ENOENT read failure (permission denied, FIFO/socket swapped in, ...) used to
			// abort `trackfw validate` entirely via fmt.Errorf — the same defect ML-1C found and
			// fixed for the *_script_integrity family, missed here because 1A-1C fixed specific
			// FUNCTIONS, not every raw read in the guard family's files (ROADMAP-2026-09-06-fecha-
			// o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1G).
			//
			// Fixed-text message, NOT inspectionDiagnostic (which interpolates the raw OS error
			// string) — this rule is in Node's CREDENTIAL_GUARD_ANCHORED_RULES and its sibling
			// functions in this file/validator_git_branch_guard.go (validateGuardHookResolvable,
			// *_script_integrity) all use a fixed remedy message specifically so the 3 runtimes
			// stay byte-identical: `%v`/`err.message`/`str(e)` differ per OS AND per runtime for
			// the exact same failure (verified live: "open trackfw.yaml: permission denied" vs
			// "EACCES: permission denied, open 'trackfw.yaml'" vs "[Errno 13] Permission denied:
			// 'trackfw.yaml'"). An earlier version of this fix used inspectionDiagnostic, which
			// would have made a LATENT 3-CLI divergence observable for the first time — the same
			// class of self-review catch ML-1C made about its own Node message before declaring
			// done. Caught before commit by a second review pass, not by the parity gate.
			return []string{credentialGuardModeDowngradeReadFailureMessage()}, nil
		}
		// trackfw.yaml deleted entirely while HEAD had mode: block — this IS the downgrade.
		return []string{credentialGuardModeDowngradeMessage()}, nil
	}

	diskMode, _ := extractCredentialGuardMode(string(diskContent))
	if diskMode == "block" {
		return nil, nil
	}

	return []string{credentialGuardModeDowngradeMessage()}, nil
}

func credentialGuardModeDowngradeMessage() string {
	return "trackfw.yaml sets credential_guard.mode: block at the git HEAD commit, but the " +
		"current file does not resolve to block — if this was intentional, commit the change; " +
		"otherwise investigate before treating the credential guard as active"
}

// credentialGuardModeDowngradeReadFailureMessage is the fixed-text remedy for a non-ENOENT read
// failure (permission denied, FIFO/socket, ...) on trackfw.yaml when HEAD had mode: block —
// deliberately NOT using inspectionDiagnostic (raw OS error text), for the same byte-identity
// reason as every other guard-family "could not be read" message. ROADMAP-2026-09-06-fecha-o-fail-
// open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1G.
func credentialGuardModeDowngradeReadFailureMessage() string {
	return "trackfw.yaml could not be read — trackfw cannot tell whether credential_guard.mode " +
		"is still block; fix the file, or run `trackfw update` to regenerate it"
}

// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1A.
// ADR: docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-
// estrita-entre-head-e-disco.md.
//
// Achado do ML-3B anterior: a severidade destas 3 regras (rules: <nome>: off|warning em
// trackfw.yaml) é lida do disco por ruleSeverity(), igual a todas as ~38 outras regras do
// validador — o que significa que uma edição NÃO COMMITADA de trackfw.yaml pode desligar a regra
// que denunciaria essa mesma edição, sem deixar rastro. credentialGuardAnchoredRules e
// credentialGuardRuleSeverity existiam só para fechar esse canal para estas 3 regras — as
// demais ~38 continuavam passando por diskRuleSeverity, inalteradas.
//
// ROADMAP-2026-09-17-leniencia-sem-prazo-e-eterna-e-a-severidade-por-regra-vem-do-arquivo-que-o-
// pr-edita, ML-1A: a ancoragem de HEAD era vácua em CI (HEAD == disco para edições COMMITADAS —
// medido na Wave 0). Substituído por ancoragem em origin/main (ver originMainTrackfwYAML,
// loadOriginMainAnchor, currentOriginMain abaixo) generalizada para TODAS as regras.
// credentialGuardRuleSeverity foi removida (sem caller de produção pós-generalização).

// credentialGuardAnchoredRules lists the 3 rule names that require the .trackfw-baseline.json
// carve-out in filterBaselineTagged (validator.go) — this is the ONLY remaining purpose of this
// variable after ROADMAP-2026-09-17 ML-1A generalized severity anchoring to ALL rules via
// origin/main (currentOriginMain).
//
// This is DISTINCT from ruleSeverity() routing, which was the variable's original role but is now
// handled uniformly for all rules by loadOriginMainAnchor(). filterBaselineTagged keeps a
// per-rule carve-out for these 3 specifically because .trackfw-baseline.json is .gitignore'd
// (never versioned), so there is no origin/main copy to compare against — the only closure for
// the baseline channel is to exclude these rule names from baseline tolerance entirely.
var credentialGuardAnchoredRules = map[string]bool{
	"credential_guard_hook_resolvable":  true,
	"credential_guard_script_integrity": true,
	"credential_guard_mode_downgrade":   true,
}

// originMainAnchorState is the four-state discriminant for origin/main's trackfw.yaml read,
// loaded once per Validate* call by loadOriginMainAnchor() and stored in currentOriginMain.
// The zero value (originAnchorNotSet) is the "not yet loaded by any Validate* call" sentinel
// used as a sane default when ruleSeverity() is called outside of Validate* (e.g., in tests
// that call it directly): falls through to diskRuleSeverity, unchanged from pre-ML-1A behavior.
//
// ADR-2026-09-17, Adendo: four states, not three — rows 1 and 2 in the table below are
// "disk only, silent" but for DIFFERENT reasons, and only row 3 is the adversary-reachable
// fail-closed case.
//
//	| state              | detection                          | behavior              |
//	|--------------------|------------------------------------|----------------------|
//	| originAnchorNoGit  | not a git worktree                 | disk only, silent    |
//	| originAnchorNoRemote | git, but no "origin" remote      | disk only, silent    |
//	|                    |   (not adversary-reachable via     |                      |
//	|                    |    editing trackfw.yaml alone)     |                      |
//	| originAnchorRefUnreadable | origin exists, rev-parse --verify origin/main fails | FAIL CLOSED |
//	| originAnchorFileAbsent | ref ok, trackfw.yaml absent     | disk only, warning   |
//	| originAnchorOK     | ref ok, file present               | stricter-wins        |
//
// Rows 1 and 2 rationale for "silent": neither can be manufactured by committing a change to
// trackfw.yaml — they require modifying the git metadata or the remote configuration, which is
// outside any PR's write surface. The same reasoning already written at credentialGuardRuleSeverity
// (Decision point 4 of ADR-2026-08-12) applies here, now covering all rules.
type originMainAnchorState int

const (
	originAnchorNotSet         originMainAnchorState = iota // zero value: no Validate* called yet
	originAnchorNoGit                                       // not a git worktree
	originAnchorNoRemote                                    // git, no "origin" remote
	originAnchorRefUnreadable                               // origin exists, origin/main ref unreadable — FAIL CLOSED
	originAnchorFileAbsent                                  // ref ok, trackfw.yaml absent — disk only, warning
	originAnchorOK                                          // fully loaded — stricter of origin/main vs disk wins
)

// originMainAnchorDirs holds the parsed governance directory paths from origin/main's
// trackfw.yaml. Used by ML-2B (scope-redirect guard) to compare against disk dirs.
// Non-nil only when state == originAnchorOK.
type originMainAnchorDirs struct {
	reqDir     string
	roadmapDir string
	adrDirs    []string
}

// originMainAnchor holds the result of a single loadOriginMainAnchor() call. Set at the top of
// ValidateUnfiltered() and validateUnfilteredTagged(); read by ruleSeverity(). Not goroutine-safe
// (acceptable: trackfw is a CLI tool, one validate call at a time).
type originMainAnchor struct {
	state originMainAnchorState
	rules map[string]string     // non-nil only when state == originAnchorOK
	dirs  *originMainAnchorDirs // non-nil only when state == originAnchorOK; ML-2B scope-redirect guard
}

// currentOriginMain is the package-level anchor result, set at the top of each Validate* call.
// Zero value (originAnchorNotSet) → diskRuleSeverity fallback for all rules.
var currentOriginMain originMainAnchor

// originMainTrackfwYAML reads origin/main:./trackfw.yaml and returns the content plus the
// four-state discriminant. Uses gitCommand (validator_git_exec.go) for env isolation.
//
// State discrimination matches the quality.yml:545-570 precedent (Braço 1/2/3):
//   - originAnchorNoGit / originAnchorNoRemote: disk only, silent — not adversary-reachable
//   - originAnchorRefUnreadable: origin exists but rev-parse --verify origin/main fails → FAIL CLOSED
//   - originAnchorFileAbsent: ref ok but ls-tree shows file absent → disk only, warning
//   - originAnchorOK: content returned
// deriveOriginDefaultBranch enumerates refs/remotes/origin/ and returns the short ref name
// (e.g. "origin/main") most likely to be the default branch.
//
// ML-1B (Defect 4): the old implementation hardcoded "origin/main", causing repos whose default
// branch is "master", "trunk", or "develop" to permanently land in originAnchorRefUnreadable.
//
// Resolution order:
//  1. "origin/main"   — most common GitHub default
//  2. "origin/master" — legacy default
//  3. The single non-HEAD ref, if exactly one exists — unambiguous regardless of name
//  4. Otherwise → "", false (caller emits originAnchorRefUnreadable with fallback declared in msg)
//
// Does NOT use "git symbolic-ref refs/remotes/origin/HEAD" (written by git clone, not by
// actions/checkout) or "git ls-remote" (network dependency). Uses only local refs.
func deriveOriginDefaultBranch() (refName string, ok bool) {
	out, err := gitCommand(".", "for-each-ref", "--format=%(refname:short)", "refs/remotes/origin/").Output()
	if err != nil {
		return "", false
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		// Filter both the short form ("origin", output of %(refname:short) for
		// refs/remotes/origin/HEAD) and the long form ("origin/HEAD", kept as a guard
		// against git version variation). Neither is a real branch — both are the HEAD
		// pointer written by `git clone` and should not be counted as branch candidates.
		if line == "" || line == "origin" || line == "origin/HEAD" {
			continue
		}
		branches = append(branches, line)
	}
	for _, preferred := range []string{"origin/main", "origin/master"} {
		for _, b := range branches {
			if b == preferred {
				return preferred, true
			}
		}
	}
	if len(branches) == 1 {
		return branches[0], true // unambiguous single branch — use it regardless of name
	}
	return "", false
}

func originMainTrackfwYAML() (content string, state originMainAnchorState) {
	if !isGitWorktree(".") {
		return "", originAnchorNoGit
	}

	// State 2: no origin remote — disk only, silent.
	// Not adversary-reachable: editing trackfw.yaml cannot remove the "origin" remote entry
	// from .git/config. Same reasoning as headTrackfwYAML's "no commits yet" accepted limit.
	if err := gitCommand(".", "remote", "get-url", "origin").Run(); err != nil {
		return "", originAnchorNoRemote
	}

	// State 3: origin exists — derive default branch from local refs/remotes/origin/.
	// ML-1B (Defect 4): hardcoded "origin/main" replaced by deriveOriginDefaultBranch so that
	// repos whose default branch is "master", "trunk", or any other name are not permanently
	// stuck in originAnchorRefUnreadable.
	// When no usable ref is found (nothing fetched yet, or multiple ambiguous refs with no
	// main/master), we fall through to originAnchorRefUnreadable — the same fail-closed state
	// as before, but now reached only when the fetch hasn't run, not because of the branch name.
	ref, found := deriveOriginDefaultBranch()
	if !found {
		return "", originAnchorRefUnreadable
	}

	// State 4: ref ok but file absent — new file in this PR.
	lsOut, err := gitCommand(".", "ls-tree", ref, "--", "./trackfw.yaml").Output()
	if err != nil || len(strings.TrimSpace(string(lsOut))) == 0 {
		return "", originAnchorFileAbsent
	}

	// State 5: ref and file present — extract content.
	out, err := gitCommand(".", "show", ref+":./trackfw.yaml").Output()
	if err != nil {
		// ls-tree listed the file but show failed — treat as ref unreadable (fail safe).
		return "", originAnchorRefUnreadable
	}
	return string(out), originAnchorOK
}

// loadOriginMainAnchor reads origin/main:./trackfw.yaml and returns the anchor result to be
// stored in currentOriginMain at the top of each Validate* call.
// ML-2B: also parses req_dir/roadmap_dir/adr_dirs into anchor.dirs so that scopeRedirectViolations
// can compare them against the disk config without a second git-show call.
func loadOriginMainAnchor() originMainAnchor {
	content, state := originMainTrackfwYAML()
	if state != originAnchorOK {
		return originMainAnchor{state: state}
	}
	reqDir, roadmapDir, adrDirs := config.ParseDirsFromContent(content)
	return originMainAnchor{
		state: originAnchorOK,
		rules: config.ParseRulesFromContent(content),
		dirs: &originMainAnchorDirs{
			reqDir:     reqDir,
			roadmapDir: roadmapDir,
			adrDirs:    adrDirs,
		},
	}
}

// originMainRefUnreadableMessage is the fail-closed violation message emitted when origin/main
// exists as a remote but its ref is unreadable (origin/main ref not present locally — typically
// because the workflow is missing the `git fetch origin` step added by ML-1A).
// DISTINCT from originMainFileAbsentMessage: ref-unreadable is a failure; file-absent is expected
// for a PR that adds trackfw.yaml for the first time.
func originMainRefUnreadableMessage() string {
	return "severity anchor unavailable: no refs found under refs/remotes/origin/ — " +
		"ensure the workflow runs a git fetch for the default branch " +
		"(e.g. git fetch --depth=1 --no-tags origin +refs/heads/main:refs/remotes/origin/main) " +
		"before trackfw validate; " +
		"rule severities that are lower in trackfw.yaml than their built-in defaults " +
		"are overridden to prevent bypass"
}

// originMainAnchorWeakeningMessage is the per-rule violation emitted when the severity anchor
// is unavailable (originAnchorRefUnreadable) AND the disk's trackfw.yaml sets a lower severity
// for a rule than its built-in default. This closes the bypass even when origin/main is absent:
// the weakening attempt is reported regardless of whether the rule has findings in this repo.
//
// ML-1B (Defect 3): separated from the blanket anchor violation so that repos that never touch
// rules: are not affected, while repos that DO try to weaken rules are still flagged.
func originMainAnchorWeakeningMessage(ruleName, diskSev, defaultSev string) string {
	return "severity anchor unavailable and trackfw.yaml weakens rule " + ruleName +
		" below built-in default (" + defaultSev + " → " + diskSev + "): " +
		"commit the fetch step (trackfw-gate.yml / trackfw-validate.yml) to activate the anchor, " +
		"or remove the rules: override to restore default severity"
}

// originMainFileAbsentMessage is the informational warning emitted when origin/main is readable
// but trackfw.yaml is absent there — expected for a PR that adds the file for the first time.
// DISTINCT from originMainRefUnreadableMessage: this is not a failure, just a note.
func originMainFileAbsentMessage() string {
	return "origin/main:./trackfw.yaml not found — rule severities resolved from disk only " +
		"(expected for a PR that adds trackfw.yaml for the first time; not a failure)"
}

// credentialGuardSeverityRank orders severities from least to most strict, for the "mais estrita
// vence" comparison in loadOriginMainAnchor / ruleSeverity (validator.go). Any string other than
// "off"/"warning" — this only ever means "error" in practice, but applyRule/applyRuleTagged
// already treat every unrecognized value as their `default:` (error) branch, so this mirrors that
// same fallback rather than introducing a stricter contract than the rest of the file has.
func credentialGuardSeverityRank(s string) int {
	switch s {
	case "off":
		return 0
	case "warning":
		return 1
	default:
		return 2
	}
}

// credentialGuardStricterSeverity returns whichever of a, b ranks higher per
// credentialGuardSeverityRank ("error" > "warning" > "off"). Ties resolve to a (arbitrary but
// deterministic — callers here never rely on tie-breaking, both sides are only ever compared once).
func credentialGuardStricterSeverity(a, b string) string {
	if credentialGuardSeverityRank(a) >= credentialGuardSeverityRank(b) {
		return a
	}
	return b
}

// credentialGuardDefaultSeverity is the same "ruleDefaults > error" fallback diskRuleSeverity uses
// once trackfw.yaml's rules: key for name is known to be absent — factored out so
// ruleSeverity (validator.go) can apply it identically to both the disk side (via diskRuleSeverity,
// which already does this) and the origin/main side (config.ParseRulesFromContent only returns what
// rules: itself contains). Also used in the originAnchorRefUnreadable fail-closed path: returns the
// rule's built-in default without consulting the disk's rules: block at all, preventing bypass via
// a committed `rules: {<name>: off}` when the anchor cannot be verified.
func credentialGuardDefaultSeverity(name string) string {
	if d, ok := ruleDefaults[name]; ok {
		return d
	}
	return "error"
}

// credentialGuardRuleSeverity was removed in ROADMAP-2026-09-17 ML-1A: its role (stricter-of-two
// severity comparison) is now performed by ruleSeverity() (validator.go) for ALL rules via
// currentOriginMain / loadOriginMainAnchor(), reusing the same helpers below. The 3 rules this
// function previously routed are no longer special in ruleSeverity(); they retain their special
// status only in filterBaselineTagged() via credentialGuardAnchoredRules.

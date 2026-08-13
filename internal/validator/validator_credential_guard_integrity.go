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

	content, err := os.ReadFile(relPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("credential_guard_script_integrity: reading %s: %w", relPath, err)
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

	diskContent, err := os.ReadFile("trackfw.yaml")
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("credential_guard_mode_downgrade: reading trackfw.yaml: %w", err)
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

// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1A.
// ADR: docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-
// estrita-entre-head-e-disco.md.
//
// Achado do ML-3B anterior: a severidade destas 3 regras (rules: <nome>: off|warning em
// trackfw.yaml) é lida do disco por ruleSeverity(), igual a todas as ~38 outras regras do
// validador — o que significa que uma edição NÃO COMMITADA de trackfw.yaml pode desligar a regra
// que denunciaria essa mesma edição, sem deixar rastro. credentialGuardAnchoredRules e
// credentialGuardRuleSeverity abaixo existem só para fechar esse canal para estas 3 regras — as
// demais ~38 continuam passando por diskRuleSeverity, inalteradas.

// credentialGuardAnchoredRules lists the rule names whose severity ruleSeverity() (validator.go)
// resolves via credentialGuardRuleSeverity instead of the ordinary disk-only diskRuleSeverity path
// every other rule uses. Also consulted by filterBaselineTagged (validator.go) for the
// .trackfw-baseline.json carve-out — a SEPARATE, independent closure for a separate channel: see
// that function's doc comment for why the baseline channel cannot be closed by anchoring in HEAD.
var credentialGuardAnchoredRules = map[string]bool{
	"credential_guard_hook_resolvable":  true,
	"credential_guard_script_integrity": true,
	"credential_guard_mode_downgrade":   true,
}

// credentialGuardSeverityRank orders severities from least to most strict, for the "mais estrita
// vence" comparison in credentialGuardRuleSeverity. Any string other than "off"/"warning" — this
// only ever means "error" in practice, but applyRule/applyRuleTagged already treat every
// unrecognized value as their `default:` (error) branch, so this mirrors that same fallback rather
// than introducing a stricter contract than the rest of the file has.
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
// credentialGuardRuleSeverity can apply it identically to both the disk side (via
// diskRuleSeverity, which already does this) and the HEAD side (which has no equivalent helper of
// its own, since config.ParseRulesFromContent only returns what rules: itself contains).
func credentialGuardDefaultSeverity(name string) string {
	if d, ok := ruleDefaults[name]; ok {
		return d
	}
	return "error"
}

// credentialGuardRuleSeverity resolves the severity of one of the 3 credentialGuardAnchoredRules
// as the MAIS ESTRITA (stricter) of two independently-resolved severities — HEAD and disk — never
// disk alone. This is the mechanism M4 chosen by the ADR: direcional, not "ignore disk and use
// HEAD only" (see ADR §Decision point 1 and the parecer's §2 for why direcional matters — the
// common case, HEAD not mentioning the rule at all, must resolve to the default, i.e. the
// strictest possible value, or disk would silently win back every time).
//
// Sem HEAD (not a git worktree, no commits yet, or trackfw.yaml not tracked at HEAD —
// headTrackfwYAML's 3 "no anchor" cases): falls back to disk alone, same as every other rule. ADR
// Decision point 4: an accepted limit, not an adversary-triggerable bypass — none of those 3
// conditions can be reached by an uncommitted edit to trackfw.yaml alone.
func credentialGuardRuleSeverity(name string) string {
	diskSeverity := diskRuleSeverity(name)

	headContent, ok := headTrackfwYAML()
	if !ok {
		return diskSeverity
	}

	headRules := config.ParseRulesFromContent(headContent)
	headSeverity, ok := headRules[name]
	if !ok {
		// No rules: <name> at HEAD — the common case for virtually every repository today.
		// Resolves to the default (already the strictest value diskRuleSeverity itself would
		// fall back to), so disk can only ever equal or lose this comparison, never win it.
		headSeverity = credentialGuardDefaultSeverity(name)
	}

	return credentialGuardStricterSeverity(headSeverity, diskSeverity)
}

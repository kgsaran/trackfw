// Package guard contains pattern tests for the 12 credential types (AC2, ML-1A).
// All credential literals are built by concatenation so this source file never
// contains a matchable credential that would trigger the credential guard hook.
// ADR D3, REQ AC3: tokens are assembled at runtime; no source literal matches credPatterns.
package guard

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// credPatternTestVector describes one AC2 test case.
type credPatternTestVector struct {
	name     string
	input    string // constructed at init time, never a literal credential
	wantHit  bool   // true: credScanInput must return a non-empty type name
	wantType string // when wantHit=true, the expected type prefix in the name
}

func buildPatternVectors() []credPatternTestVector {
	// --- positive vectors (one per pattern) ---
	// JWT (3-part base64url)
	jwt := "eyJ" + "hbGciOiJIUzI1NiJ9" + "." + "eyJzdWIiOiJ4In0" + "." + "abc123"

	// AWS access key
	awsKey := "AKIA" + "0123456789ABCDEF"

	// GitHub classic PAT
	ghpToken := "ghp_" + strings.Repeat("A", 36)

	// GitHub OAuth
	ghoToken := "gho_" + strings.Repeat("B", 36)

	// GitHub user-to-server
	ghuToken := "ghu_" + strings.Repeat("C", 36)

	// GitHub server-to-server
	ghsToken := "ghs_" + strings.Repeat("D", 36)

	// GitHub refresh
	ghrToken := "ghr_" + strings.Repeat("E", 36)

	// GitHub fine-grained PAT (82 alphanum/underscore chars after prefix)
	githubPAT := "github_pat_" + strings.Repeat("A1", 41)

	// Anthropic API key (sk-ant- prefix, 20+ chars body)
	skAnt := "sk-ant-" + strings.Repeat("a1", 15)

	// OpenAI project key (sk-proj- prefix, 20+ chars body)
	skProj := "sk-proj-" + strings.Repeat("b2", 15)

	// OpenAI API key (sk- prefix, 48+ alphanum chars) — longer than sk-ant and sk-proj
	skBase := "sk-" + strings.Repeat("C3", 25)

	// Google API key
	aiza := "AIza" + strings.Repeat("0A", 18) // 36 chars after prefix

	// Slack token (xoxb-)
	slackB := "xoxb-" + strings.Repeat("1A-", 7) + "1A"

	// Slack token (xoxp-)
	slackP := "xoxp-" + strings.Repeat("2B-", 7) + "2B"

	// PEM private key header
	pem := "-----BEGIN " + "RSA PRIVATE KEY" + "-----"

	// --- false positive / negative vectors ---
	// sk-ant prefix too short (19 chars body — below 20 minimum)
	skAntShort := "sk-ant-" + strings.Repeat("x", 19)

	// sk-proj prefix too short
	skProjShort := "sk-proj-" + strings.Repeat("y", 19)

	// sk- base too short (47 chars — below 48 minimum)
	skBaseShort := "sk-" + strings.Repeat("z", 47)

	// GitHub token too short
	ghpShort := "ghp_" + strings.Repeat("A", 35)

	// JSON \n (backslash + n) immediately before sk-ant- prefix
	// In raw JSON the newline is two chars: \ + n, so 'n' is a word char.
	// The explicit boundary (?:^|[^A-Za-z0-9_]|\\[bfnrt]) must match the JSON newline.
	jsonNLSKAnt := `\n` + "sk-ant-" + strings.Repeat("a1", 15)

	// Plain text that is not a credential
	plainText := "hello world, print(1)"

	return []credPatternTestVector{
		// Positive: each known type produces a hit
		{"JWT positive", jwt, true, "JWT"},
		{"AWS access key positive", awsKey, true, "AWS access key"},
		{"GitHub ghp_ positive", ghpToken, true, "GitHub token"},
		{"GitHub gho_ positive", ghoToken, true, "GitHub token"},
		{"GitHub ghu_ positive", ghuToken, true, "GitHub token"},
		{"GitHub ghs_ positive", ghsToken, true, "GitHub token"},
		{"GitHub ghr_ positive", ghrToken, true, "GitHub token"},
		{"GitHub fine-grained PAT positive", githubPAT, true, "GitHub fine-grained PAT"},
		{"Anthropic key positive", skAnt, true, "Anthropic API key"},
		{"OpenAI project key positive", skProj, true, "OpenAI project key"},
		{"OpenAI API key positive", skBase, true, "OpenAI"},
		{"Google API key positive", aiza, true, "Google API key"},
		{"Slack xoxb positive", slackB, true, "Slack token"},
		{"Slack xoxp positive", slackP, true, "Slack token"},
		{"PEM private key positive", pem, true, "PEM private key"},

		// JSON \\n boundary: sk-ant- after JSON-encoded newline must hit
		{"Anthropic key after JSON newline", jsonNLSKAnt, true, "Anthropic API key"},

		// Negative: too-short variants must not hit
		{"Anthropic key too short", skAntShort, false, ""},
		{"OpenAI project key too short", skProjShort, false, ""},
		{"OpenAI API key too short", skBaseShort, false, ""},
		{"GitHub ghp_ too short", ghpShort, false, ""},

		// Negative: plain text must not hit
		{"plain text no credential", plainText, false, ""},
	}
}

// TestCredScanInput_PatternVectors checks positive and negative vectors for all 12
// credential types (AC2). Positive: credScanInput returns a type name containing wantType.
// Negative: credScanInput returns "". JSON \\n boundary: Anthropic key after JSON newline
// is detected via the explicit boundary (?:^|[^A-Za-z0-9_]|\\[bfnrt]).
func TestCredScanInput_PatternVectors(t *testing.T) {
	vectors := buildPatternVectors()
	for _, v := range vectors {
		v := v
		t.Run(v.name, func(t *testing.T) {
			t.Parallel()
			got := credScanInput(v.input)
			if v.wantHit {
				if got == "" {
					t.Errorf("expected match containing %q, got empty", v.wantType)
				} else if v.wantType != "" && !strings.Contains(got, v.wantType) {
					t.Errorf("expected match containing %q, got %q", v.wantType, got)
				}
			} else {
				if got != "" {
					t.Errorf("expected no match, got %q", got)
				}
			}
		})
	}
}

// TestCredScanInput_PlainCommandNoBlock verifies that an ordinary command
// (print(1)) produces no hit, confirming rc=0 behavior for benign payloads.
// Assertion: plain command → credScanInput returns "" → no credential detected.
func TestCredScanInput_PlainCommandNoBlock(t *testing.T) {
	got := credScanInput("print(1)")
	if got != "" {
		t.Errorf("plain command: expected no match, got %q", got)
	}
}

// credEphemeralTable is index-aligned with credPatterns: len(credEphemeralTable) == len(credPatterns).
// Each tok is built by concatenation so no source literal is a matchable credential in a tool payload.
// Structural invariants checked by TestCredEphemeralExemption_AllPatterns:
//   (a) len matches credPatterns;
//   (b) for each i, the first j where credPatterns[j].re.MatchString(tok[i]) is true equals i
//       (ensures the token exercises exactly the intended pattern, not an earlier one that
//       subsumes it — catches silent mis-testing on reorder or new entry).
var credEphemeralTable = []struct {
	label string
	tok   string
}{
	// 0 JWT
	{"JWT", "eyJ" + "hbGciOiJIUzI1NiJ9" + "." + "eyJzdWIiOiJ4In0" + "." + "abc123"},
	// 1 AWS access key
	{"AWS_access_key", "AKIA" + "0123456789ABCDEF"},
	// 2 GitHub ghp_
	{"ghp_", "ghp_" + strings.Repeat("A", 36)},
	// 3 GitHub gho_
	{"gho_", "gho_" + strings.Repeat("B", 36)},
	// 4 GitHub ghu_
	{"ghu_", "ghu_" + strings.Repeat("C", 36)},
	// 5 GitHub ghs_
	{"ghs_", "ghs_" + strings.Repeat("D", 36)},
	// 6 GitHub ghr_
	{"ghr_", "ghr_" + strings.Repeat("E", 36)},
	// 7 GitHub fine-grained PAT (82 alphanum/underscore chars)
	{"github_pat_", "git" + "hub_pat_" + strings.Repeat("A1", 41)},
	// 8 Anthropic API key
	{"sk-ant-", "sk-ant-" + strings.Repeat("a1", 15)},
	// 9 OpenAI project key
	{"sk-proj-", "sk-proj-" + strings.Repeat("b2", 15)},
	// 10 OpenAI API key (48+ alphanum after sk-; no sk-ant- or sk-proj- substring)
	{"sk-base", "sk-" + strings.Repeat("C3", 25)},
	// 11 Google API key
	{"AIza", "AIza" + strings.Repeat("0A", 18)},
	// 12 Slack token (xoxb-)
	{"xoxb-", "xoxb-" + strings.Repeat("1A-", 7) + "1A"},
	// 13 PEM private key header (spaces in token are fine in a JSON string)
	{"PEM", "-----BEGIN " + "RSA PRIVATE KEY" + "-----"},
}

// TestCredEphemeralExemption_AllPatterns asserts the F2 shape-based ephemeral exemption
// through RunCredential for every entry in credPatterns (all 14, including the 5 "GitHub token"
// sub-patterns that share a name).
//
// Two arms per entry, exercised in a project with mode: block:
//   arm A: "echo <tok> > /dev/null"  → rc=0 (ephemeral exemption fires)
//   arm B: "echo <tok> > out.txt"    → rc=2 (non-ephemeral redirect; exemption does not fire)
//
// Structural assertions run first:
//   (a) len(credEphemeralTable) == len(credPatterns) — table must be kept in sync
//   (b) for each i, the first j where credPatterns[j].re.MatchString(tok[i]) equals i —
//       prevents a token from silently exercising the wrong (earlier) pattern
//
// PEM note: the PEM private key header token (credEphemeralTable row 13) contains
// spaces but no shell metacharacters; credIsSimpleCmd accepts it, the exemption fires normally (rc=0).
// The header is built from fragments — no source literal here matches the PEM pattern (ADR D3, REQ AC3).
//
// Assertion: credCmdHasTypeMatch correctly matches all sub-patterns sharing a type name,
// ensuring the ephemeral exemption fires for every pattern variant, not just the first.
//
// No t.Parallel(): runCred calls os.Chdir, which is process-global.
func TestCredEphemeralExemption_AllPatterns(t *testing.T) {
	// Structural assertion (a): table length must match credPatterns.
	if got, want := len(credEphemeralTable), len(credPatterns); got != want {
		t.Fatalf("credEphemeralTable has %d entries, credPatterns has %d — keep them in sync", got, want)
	}

	// Structural assertion (b): each token must be matched first by credPatterns[i],
	// not by any earlier pattern.
	for i, entry := range credEphemeralTable {
		firstMatch := -1
		for j, p := range credPatterns {
			if p.re.MatchString(entry.tok) {
				firstMatch = j
				break
			}
		}
		if firstMatch != i {
			t.Errorf("credEphemeralTable[%d] label=%q: token matched by credPatterns[%d], want credPatterns[%d]",
				i, entry.label, firstMatch, i)
		}
	}

	// Bail early if structural checks failed — behavioral arms would be misleading.
	if t.Failed() {
		return
	}

	dir := makeCredProjectDir(t, "credential_guard:\n  mode: block\nroadmap_dir: docs/roadmaps\n")

	for i, entry := range credEphemeralTable {
		i, entry := i, entry
		t.Run(entry.label+"/arm_A_devnull", func(t *testing.T) {
			payload := `{"tool_input":{"command":"echo ` + entry.tok + ` > /dev/null"}}`
			code, _, errOut := runCred(t, dir, payload)
			if code != 0 {
				t.Errorf("credEphemeralTable[%d] label=%q: arm A (>/dev/null): expected rc=0 (ephemeral exemption), got %d (stderr=%q)",
					i, entry.label, code, errOut)
			}
		})
		t.Run(entry.label+"/arm_B_file", func(t *testing.T) {
			payload := `{"tool_input":{"command":"echo ` + entry.tok + ` > out.txt"}}`
			code, _, errOut := runCred(t, dir, payload)
			if code != 2 {
				t.Errorf("credEphemeralTable[%d] label=%q: arm B (>out.txt): expected rc=2 (block mode, non-ephemeral), got %d (stderr=%q)",
					i, entry.label, code, errOut)
			}
		})
	}
}

// TestFixturesSemTokenLiteral is the AC3 census gate (ADR D3, ML-1B).
//
// It reads the two source files that formerly held credential literals and applies
// every pattern in credPatterns.  The test fails if any pattern matches — which means
// a full credential was re-introduced as a source literal.  This is the gate that
// prevents regression: once it passes, CI re-runs it on every commit.
//
// Files checked:
//   - this file (credential_patterns_test.go)
//   - internal/generators/credential_guard_test.go
//   - internal/guard/credential_test.go (clean at ML-1B; included to catch regressions from ML-1A)
//
// Assertion: no source literal in the three files matches any credential pattern in
// credPatterns; tokens in those files are assembled at runtime from fragments (ADR D3, REQ AC3).
func TestFixturesSemTokenLiteral(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed — cannot locate source files")
	}
	guardDir := filepath.Dir(thisFile)
	internalDir := filepath.Dir(guardDir)

	filesToCheck := []string{
		thisFile, // credential_patterns_test.go
		filepath.Join(internalDir, "generators", "credential_guard_test.go"),
		filepath.Join(guardDir, "credential_test.go"),
	}

	for _, path := range filesToCheck {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("os.ReadFile(%s): %v", path, err)
			}
			if len(content) == 0 {
				t.Fatalf("file is empty: %s — census cannot prove absence", path)
			}
			for _, p := range credPatterns {
				if p.re.Match(content) {
					t.Errorf("pattern %q matched in %s — file contains a credential literal; build fragments instead (ADR D3, REQ AC3)", p.name, path)
				}
			}
		})
	}
}

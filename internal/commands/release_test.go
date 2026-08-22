package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// test helpers
// ────────────────────────────────────────────────────────────────────────────

const (
	releaseTestVersion = "9.9.9"
	releaseTestTag      = "v9.9.9"
	releaseTestSHA      = "abc123def456"
)

func validReleaseVersionFiles(version string) map[string]string {
	return map[string]string{
		"internal/version/version.go": fmt.Sprintf("package version\n\nvar Version = %q\n", version),
		"npm/package.json":            fmt.Sprintf(`{"name":"trackfw","version":%q}`, version),
		"pypi/pyproject.toml":         fmt.Sprintf("[project]\nname = \"trackfw\"\nversion = %q\n", version),
		"pypi/trackfw/__init__.py": fmt.Sprintf(
			"try:\n    from importlib.metadata import version\n    __version__ = version(\"trackfw\") or %q\nexcept Exception:\n    __version__ = %q\n",
			version, version,
		),
		"CHANGELOG.md": fmt.Sprintf("# Changelog\n\n## [%s] - 2026-08-19\n\n### Added\n- x\n", version),
	}
}

// mockReleaseGit routes execGit calls by joined args prefix; unmatched calls return "", nil.
type mockReleaseGit struct {
	responses map[string]string // exact joined-args -> stdout
	errors    map[string]error  // exact joined-args -> error (checked before responses)
	calls     [][]string
}

func newMockReleaseGit() *mockReleaseGit {
	return &mockReleaseGit{
		responses: map[string]string{
			"status --porcelain":                           "",
			"fetch origin --prune":                         "",
			"symbolic-ref refs/remotes/origin/HEAD":         "refs/remotes/origin/main",
			"rev-parse origin/main":                         releaseTestSHA,
			"remote get-url origin":                         "https://github.com/kgsaran/trackfw.git",
			"config user.name":                              "Test User",
			"config user.email":                             "test@example.com",
			"ls-remote --tags origin refs/tags/" + releaseTestTag: "",
		},
		errors: map[string]error{
			"rev-parse -q --verify refs/heads/main":         errors.New("no such branch"),
			"rev-parse -q --verify refs/tags/" + releaseTestTag: errors.New("no such tag"),
		},
	}
}

func (m *mockReleaseGit) exec(args ...string) (string, error) {
	call := make([]string, len(args))
	copy(call, args)
	m.calls = append(m.calls, call)
	key := strings.Join(args, " ")
	if err, ok := m.errors[key]; ok {
		return "", err
	}
	if out, ok := m.responses[key]; ok {
		return out, nil
	}
	return "", nil
}

func makeReleaseDeps(t *testing.T, fileOverrides map[string]string) (releaseDeps, *mockReleaseGit) {
	t.Helper()
	files := validReleaseVersionFiles(releaseTestVersion)
	for k, v := range fileOverrides {
		files[k] = v
	}
	g := newMockReleaseGit()
	d := releaseDeps{
		execGit: g.exec,
		// readCommittedFile reads from the files map, keyed by path (ignoring sha — tests control
		// both the sha the forge mock returns and the content in the files map; the sha parameter
		// is available for assertions in individual tests via custom overrides).
		readCommittedFile: func(sha, path string) (string, error) {
			content, ok := files[path]
			if !ok {
				return "", fmt.Errorf("object %s:%s not found", sha, path)
			}
			return content, nil
		},
		out:          &bytes.Buffer{},
		configForge:  "",
		repoDir:      "",
		availFn:      func(string) bool { return true },
		execForgeAPI: defaultMockReleaseForgeAPI,
	}
	return d, g
}

// defaultMockReleaseForgeAPI answers the four gh api calls release tag makes:
//   - repos/{owner}/{repo}                       -> default_branch: "main" (agrees with the
//     fixture's symref-derived base, so no divergence fires by default)
//   - repos/{owner}/{repo}/commits/main           -> sha: releaseTestSHA (agrees with the
//     fixture's local origin/main, so no divergence fires by default)
//   - repos/{owner}/{repo}/git/tags  (POST)       -> sha: "tagobjectsha000"
//   - repos/{owner}/{repo}/git/refs  (POST)       -> {}
func defaultMockReleaseForgeAPI(name string, args []string, stdin string) (string, error) {
	if len(args) >= 2 {
		switch {
		case strings.Contains(args[1], "git/tags"):
			return `{"sha":"tagobjectsha000"}`, nil
		case strings.Contains(args[1], "git/refs"):
			return `{}`, nil
		case strings.Contains(args[1], "/commits/"):
			return fmt.Sprintf(`{"sha":%q}`, releaseTestSHA), nil
		case args[1] == "repos/{owner}/{repo}":
			return `{"default_branch":"main"}`, nil
		}
	}
	return `{}`, nil
}

// ────────────────────────────────────────────────────────────────────────────
// Precondition 1 — clean working tree
// ────────────────────────────────────────────────────────────────────────────

func TestReleaseTag_DirtyTree_Aborts(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	g.responses["status --porcelain"] = " M some/file.go\n"
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error for dirty working tree")
	}
	if !strings.Contains(err.Error(), "working tree is not clean") {
		t.Errorf("error = %q, want mention of dirty working tree", err.Error())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Precondition 2 — default branch up to date with origin
// ────────────────────────────────────────────────────────────────────────────

func TestReleaseTag_FetchFails_Aborts(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	g.errors["fetch origin --prune"] = errors.New("could not connect")
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when fetch fails")
	}
	if !strings.Contains(err.Error(), "could not fetch origin") {
		t.Errorf("error = %q, want mention of fetch failure", err.Error())
	}
}

func TestReleaseTag_LocalMainStale_Aborts(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	delete(g.errors, "rev-parse -q --verify refs/heads/main") // local main exists
	g.responses["rev-parse -q --verify refs/heads/main"] = ""
	g.responses["rev-parse refs/heads/main"] = "stalesha000"
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error for stale local main")
	}
	if !strings.Contains(err.Error(), "not up to date with origin/main") {
		t.Errorf("error = %q, want mention of stale local main", err.Error())
	}
}

func TestReleaseTag_LocalMainMatchesOrigin_NotBlocked(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	delete(g.errors, "rev-parse -q --verify refs/heads/main")
	g.responses["rev-parse -q --verify refs/heads/main"] = ""
	g.responses["rev-parse refs/heads/main"] = releaseTestSHA
	if err := runReleaseTag(releaseTestVersion, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReleaseTag_NoLocalMain_NotBlocked(t *testing.T) {
	// Default mock: rev-parse -q --verify refs/heads/main errors (no local branch) — must
	// not block; release tag always targets origin/main's tip directly.
	d, _ := makeReleaseDeps(t, nil)
	if err := runReleaseTag(releaseTestVersion, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Precondition 3 — the 4 version files must all match
// ────────────────────────────────────────────────────────────────────────────

func TestReleaseTag_VersionFileMismatch_NamesWhichFile(t *testing.T) {
	cases := []struct {
		name  string
		path  string
		label string
	}{
		{"go", "internal/version/version.go", "internal/version/version.go"},
		{"npm", "npm/package.json", "npm/package.json"},
		{"pyproject", "pypi/pyproject.toml", "pypi/pyproject.toml"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := validReleaseVersionFiles(releaseTestVersion)
			switch tc.path {
			case "internal/version/version.go":
				files[tc.path] = `package version

var Version = "0.0.1"
`
			case "npm/package.json":
				files[tc.path] = `{"name":"trackfw","version":"0.0.1"}`
			case "pypi/pyproject.toml":
				files[tc.path] = "[project]\nversion = \"0.0.1\"\n"
			}
			d, _ := makeReleaseDeps(t, files)
			err := runReleaseTag(releaseTestVersion, d)
			if err == nil {
				t.Fatalf("expected error for mismatched %s", tc.path)
			}
			if !strings.Contains(err.Error(), tc.label) {
				t.Errorf("error = %q, want it to name %q", err.Error(), tc.label)
			}
			if !strings.Contains(err.Error(), "0.0.1") || !strings.Contains(err.Error(), releaseTestVersion) {
				t.Errorf("error = %q, want both the found and expected versions", err.Error())
			}
		})
	}
}

func TestReleaseTag_InitPyTryFallbackMismatch_Aborts(t *testing.T) {
	files := validReleaseVersionFiles(releaseTestVersion)
	files["pypi/trackfw/__init__.py"] = fmt.Sprintf(
		"try:\n    from importlib.metadata import version\n    __version__ = version(\"trackfw\") or \"0.0.1\"\nexcept Exception:\n    __version__ = %q\n",
		releaseTestVersion,
	)
	d, _ := makeReleaseDeps(t, files)
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil || !strings.Contains(err.Error(), "importlib.metadata fallback") {
		t.Fatalf("expected error naming the try-block fallback, got: %v", err)
	}
}

func TestReleaseTag_InitPyExceptFallbackMismatch_Aborts(t *testing.T) {
	files := validReleaseVersionFiles(releaseTestVersion)
	files["pypi/trackfw/__init__.py"] = fmt.Sprintf(
		"try:\n    from importlib.metadata import version\n    __version__ = version(\"trackfw\") or %q\nexcept Exception:\n    __version__ = \"0.0.1\"\n",
		releaseTestVersion,
	)
	d, _ := makeReleaseDeps(t, files)
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil || !strings.Contains(err.Error(), "except fallback") {
		t.Fatalf("expected error naming the except-block fallback, got: %v", err)
	}
}

func TestReleaseTag_VPrefixArg_NormalizedAgainstBareFileVersions(t *testing.T) {
	d, _ := makeReleaseDeps(t, nil)
	if err := runReleaseTag("v"+releaseTestVersion, d); err != nil {
		t.Fatalf("unexpected error passing 'v%s': %v", releaseTestVersion, err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Precondition 4 — CHANGELOG.md must have the version's section
// ────────────────────────────────────────────────────────────────────────────

func TestReleaseTag_ChangelogMissingSection_Aborts(t *testing.T) {
	files := validReleaseVersionFiles(releaseTestVersion)
	files["CHANGELOG.md"] = "# Changelog\n\n## [1.0.0] - 2020-01-01\n\n### Added\n- x\n"
	d, _ := makeReleaseDeps(t, files)
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error for missing CHANGELOG section")
	}
	if !strings.Contains(err.Error(), releaseTestVersion) || !strings.Contains(err.Error(), "not found in CHANGELOG.md") {
		t.Errorf("error = %q, want it to name the missing version and CHANGELOG.md", err.Error())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Precondition 5 — tag must not already exist, local or remote
// ────────────────────────────────────────────────────────────────────────────

func TestReleaseTag_LocalTagExists_Aborts(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	delete(g.errors, "rev-parse -q --verify refs/tags/"+releaseTestTag)
	g.responses["rev-parse -q --verify refs/tags/"+releaseTestTag] = releaseTestSHA
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error for pre-existing local tag")
	}
	if !strings.Contains(err.Error(), releaseTestTag) || !strings.Contains(err.Error(), "already exists locally") {
		t.Errorf("error = %q, want it to name the tag and 'already exists locally'", err.Error())
	}
}

func TestReleaseTag_RemoteTagExists_Aborts(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	g.responses["ls-remote --tags origin refs/tags/"+releaseTestTag] = releaseTestSHA + "\trefs/tags/" + releaseTestTag
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error for pre-existing remote tag")
	}
	if !strings.Contains(err.Error(), releaseTestTag) || !strings.Contains(err.Error(), "already exists on origin") {
		t.Errorf("error = %q, want it to name the tag and 'already exists on origin'", err.Error())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Precondition 6 — forge CLI available, GitHub only
// ────────────────────────────────────────────────────────────────────────────

func TestReleaseTag_NoForgeCLI_Aborts(t *testing.T) {
	d, _ := makeReleaseDeps(t, nil)
	d.availFn = func(string) bool { return false }
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when gh is unavailable")
	}
	if !strings.Contains(err.Error(), "requires the GitHub CLI (gh)") {
		t.Errorf("error = %q, want mention of missing gh CLI", err.Error())
	}
	if !strings.Contains(err.Error(), "git tag -a "+releaseTestTag) {
		t.Errorf("error = %q, want manual-tag orientation naming %s", err.Error(), releaseTestTag)
	}
}

func TestReleaseTag_UnsupportedForge_Aborts(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	g.responses["remote get-url origin"] = "git@gitlab.com:kgsaran/trackfw.git"
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error for non-GitHub forge")
	}
	if !strings.Contains(err.Error(), "currently only supports GitHub") || !strings.Contains(err.Error(), "gitlab") {
		t.Errorf("error = %q, want mention of GitHub-only support and resolved forge gitlab", err.Error())
	}
}

func TestReleaseTag_ManualForge_Aborts(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	g.responses["remote get-url origin"] = "git@example.internal:kgsaran/trackfw.git"
	d.repoDir = "" // no CI file detection either → manual
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error for manual (unresolved) forge")
	}
	if !strings.Contains(err.Error(), `resolved forge: "manual"`) {
		t.Errorf("error = %q, want it to name the manual resolution", err.Error())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Commit target ancored on the forge (ADR-2026-08-19, Emenda 1) — the forge's
// default_branch/commit sha are authoritative; local refs are cross-checked only, never trusted.
// ────────────────────────────────────────────────────────────────────────────

func TestReleaseTag_LocalSymrefDiverges_ForgeNameWinsWithoutRefusal(t *testing.T) {
	// Simulates a repointed local symbolic-ref: the local, symref-derived base resolves to
	// "chore/other" (an attacker-writable, purely local operation), while the forge reports
	// "main". The forge's branch name is authoritative unconditionally — no local-vs-forge name
	// comparison exists, because a fresh/shallow clone legitimately has no local opinion on the
	// default branch at all (defaultBaseBranch falls back to "main" with no symref present, which
	// must not be indistinguishable from — nor treated as — a forgery). The repoint is neutralized:
	// the command resolves and publishes using the forge's branch (origin/main) and sha, ignoring
	// the repointed symref entirely.
	d, g := makeReleaseDeps(t, nil)
	g.responses["symbolic-ref refs/remotes/origin/HEAD"] = "refs/remotes/origin/chore/other"
	g.responses["rev-parse origin/chore/other"] = "shaonchoreother00"
	g.errors["rev-parse -q --verify refs/heads/chore/other"] = errors.New("no such branch")

	var tagsBody string
	d.execForgeAPI = func(name string, args []string, stdin string) (string, error) {
		if len(args) >= 2 && strings.Contains(args[1], "git/tags") {
			tagsBody = stdin
			return `{"sha":"tagobjectsha000"}`, nil
		}
		return defaultMockReleaseForgeAPI(name, args, stdin)
	}

	if err := runReleaseTag(releaseTestVersion, d); err != nil {
		t.Fatalf("unexpected error: %v — a repointed local symref must not block publish, only the forge's own sha cross-check does", err)
	}
	if !strings.Contains(tagsBody, releaseTestSHA) {
		t.Errorf("tag object payload = %q, want it to use the forge's commit sha %q for origin/main, ignoring the repointed symref (shaonchoreother00)", tagsBody, releaseTestSHA)
	}
}

func TestReleaseTag_NoLocalSymref_ForgeBranchNameUsedWithoutFalseRefusal(t *testing.T) {
	// Simulates a fresh/shallow clone that has no origin/HEAD symref at all — defaultBaseBranch
	// falls back to its "main" default. If the repo's REAL default branch is "master" (as reported
	// by the forge), the forge's name must still be used without any refusal: there is no local
	// opinion to disagree with the forge here, just an absent one. Regression guard for treating
	// "no symref" and "a genuinely repointed symref" as the same signal — they are not.
	d, g := makeReleaseDeps(t, nil)
	g.errors["symbolic-ref refs/remotes/origin/HEAD"] = errors.New("ref refs/remotes/origin/HEAD is not a symbolic ref")
	delete(g.responses, "rev-parse origin/main")
	g.errors["rev-parse origin/main"] = errors.New("unknown revision")
	g.responses["rev-parse origin/master"] = releaseTestSHA

	d.execForgeAPI = func(name string, args []string, stdin string) (string, error) {
		if len(args) >= 2 && args[1] == "repos/{owner}/{repo}" {
			return `{"default_branch":"master"}`, nil
		}
		return defaultMockReleaseForgeAPI(name, args, stdin)
	}

	if err := runReleaseTag(releaseTestVersion, d); err != nil {
		t.Fatalf("unexpected error: %v — an absent local symref (fresh/shallow clone) must not be treated as a divergence against the forge's real default branch", err)
	}
}

func TestReleaseTag_ForgeCommitDiverges_RefusesNamingDivergence(t *testing.T) {
	// Simulates a forged/stale origin/main tracking ref (e.g. via git update-ref, or a narrowed
	// remote.origin.fetch that never learned the real tip): local origin/main resolves to a sha
	// the forge does not report as its tip. Must refuse — the forge sha is never published
	// silently past a disagreeing local ref.
	d, g := makeReleaseDeps(t, nil)
	g.responses["rev-parse origin/main"] = "forgedlocalsha000"
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when local origin/main diverges from the forge's tip commit")
	}
	if !strings.Contains(err.Error(), "forgedlocalsha000") || !strings.Contains(err.Error(), releaseTestSHA) {
		t.Errorf("error = %q, want it to name both the local and forge shas", err.Error())
	}
	if !strings.Contains(err.Error(), "diverges") {
		t.Errorf("error = %q, want it to name the divergence explicitly", err.Error())
	}
}

func TestReleaseTag_ForgeCommitSHA_UsedForPublish_NeverLocal(t *testing.T) {
	// Even when local origin/main cannot be resolved at all (symref repointed to a branch this
	// clone has no tracking ref for), the command must still resolve and publish using the
	// forge's sha — never block on the missing local cross-check candidate.
	d, g := makeReleaseDeps(t, nil)
	g.responses["symbolic-ref refs/remotes/origin/HEAD"] = "refs/remotes/origin/main"
	delete(g.responses, "rev-parse origin/main")
	g.errors["rev-parse origin/main"] = errors.New("unknown revision")

	var tagsBody string
	d.execForgeAPI = func(name string, args []string, stdin string) (string, error) {
		if len(args) >= 2 && strings.Contains(args[1], "git/tags") {
			tagsBody = stdin
			return `{"sha":"tagobjectsha000"}`, nil
		}
		return defaultMockReleaseForgeAPI(name, args, stdin)
	}

	if err := runReleaseTag(releaseTestVersion, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(tagsBody, releaseTestSHA) {
		t.Errorf("tag object payload = %q, want it to use the forge's commit sha %q", tagsBody, releaseTestSHA)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Git identity
// ────────────────────────────────────────────────────────────────────────────

func TestReleaseTag_NoGitIdentity_Aborts(t *testing.T) {
	d, g := makeReleaseDeps(t, nil)
	g.responses["config user.name"] = ""
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when git user.name is unset")
	}
	if !strings.Contains(err.Error(), "git config user.name") {
		t.Errorf("error = %q, want mention of git config user.name", err.Error())
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Success path — verifies the annotated-tag publish sequence
// ────────────────────────────────────────────────────────────────────────────

func TestReleaseTag_Success_PublishesAnnotatedTag(t *testing.T) {
	d, _ := makeReleaseDeps(t, nil)

	var calls []struct {
		name string
		args []string
		body string
	}
	d.execForgeAPI = func(name string, args []string, stdin string) (string, error) {
		// Only the two POST publish calls are recorded — the GET calls resolving
		// default_branch/commit sha are answered but not part of what this test asserts on.
		if len(args) >= 2 && (strings.Contains(args[1], "git/tags") || strings.Contains(args[1], "git/refs")) {
			call := struct {
				name string
				args []string
				body string
			}{name, args, stdin}
			calls = append(calls, call)
		}
		return defaultMockReleaseForgeAPI(name, args, stdin)
	}

	buf := &bytes.Buffer{}
	d.out = buf

	if err := runReleaseTag(releaseTestVersion, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(calls) != 2 {
		t.Fatalf("expected 2 gh api calls, got %d", len(calls))
	}

	// First call: creates the tag object.
	if !strings.Contains(calls[0].args[1], "git/tags") {
		t.Errorf("first call endpoint = %q, want git/tags", calls[0].args[1])
	}
	var tagPayload map[string]interface{}
	if err := json.Unmarshal([]byte(calls[0].body), &tagPayload); err != nil {
		t.Fatalf("could not parse first call body: %v", err)
	}
	if tagPayload["tag"] != releaseTestTag {
		t.Errorf("tag payload[tag] = %v, want %q", tagPayload["tag"], releaseTestTag)
	}
	if tagPayload["object"] != releaseTestSHA {
		t.Errorf("tag payload[object] = %v, want %q", tagPayload["object"], releaseTestSHA)
	}
	if tagPayload["type"] != "commit" {
		t.Errorf("tag payload[type] = %v, want commit", tagPayload["type"])
	}
	if body, _ := tagPayload["message"].(string); !strings.Contains(body, releaseTestVersion) {
		t.Errorf("tag payload[message] = %q, want it to contain the CHANGELOG section for %s", body, releaseTestVersion)
	}
	tagger, _ := tagPayload["tagger"].(map[string]interface{})
	if tagger["name"] != "Test User" || tagger["email"] != "test@example.com" {
		t.Errorf("tagger = %+v, want name/email from git config", tagger)
	}

	// Second call: creates the ref, using the object sha from the first call's response.
	if !strings.Contains(calls[1].args[1], "git/refs") {
		t.Errorf("second call endpoint = %q, want git/refs", calls[1].args[1])
	}
	var refPayload map[string]interface{}
	if err := json.Unmarshal([]byte(calls[1].body), &refPayload); err != nil {
		t.Fatalf("could not parse second call body: %v", err)
	}
	if refPayload["ref"] != "refs/tags/"+releaseTestTag {
		t.Errorf("ref payload[ref] = %v, want refs/tags/%s", refPayload["ref"], releaseTestTag)
	}
	if refPayload["sha"] != "tagobjectsha000" {
		t.Errorf("ref payload[sha] = %v, want the tag object sha returned by the first call", refPayload["sha"])
	}

	if !strings.Contains(buf.String(), releaseTestTag) {
		t.Errorf("stdout = %q, want it to mention the published tag", buf.String())
	}
}

func TestReleaseTag_TagObjectCallFails_AbortsBeforeRefCall(t *testing.T) {
	d, _ := makeReleaseDeps(t, nil)
	var refCalled bool
	d.execForgeAPI = func(name string, args []string, stdin string) (string, error) {
		if len(args) >= 2 && strings.Contains(args[1], "git/tags") {
			return "", errors.New("401 Unauthorized")
		}
		if len(args) >= 2 && strings.Contains(args[1], "git/refs") {
			refCalled = true
		}
		return defaultMockReleaseForgeAPI(name, args, stdin)
	}
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when the tag object call fails")
	}
	if refCalled {
		t.Fatal("git/refs must never be called when git/tags failed — would create an orphan ref")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// ML-2A: Object anchoring — P3/P4 read from the commit-target, not the
// working tree. See ADR-2026-08-21-release-tag-le-versao-e-changelog-do-
// commit-ancorado.md.
// ────────────────────────────────────────────────────────────────────────────

// TestReleaseTag_ObjectAbsent_VersionFile_RefusesNamingShAndPath verifies that
// when readCommittedFile cannot find a version file at objectSHA, the command
// refuses with a message that names both the path and the sha — and never
// reaches the publish step.
func TestReleaseTag_ObjectAbsent_VersionFile_RefusesNamingShAndPath(t *testing.T) {
	d, _ := makeReleaseDeps(t, nil)
	var publishCalled bool
	d.execForgeAPI = func(name string, args []string, stdin string) (string, error) {
		if len(args) >= 2 && strings.Contains(args[1], "git/tags") {
			publishCalled = true
		}
		return defaultMockReleaseForgeAPI(name, args, stdin)
	}
	// Make the version file absent at the commit sha.
	d.readCommittedFile = func(sha, path string) (string, error) {
		if path == "internal/version/version.go" {
			return "", fmt.Errorf("path 'internal/version/version.go' does not exist in '%s'", sha)
		}
		// All other files succeed with valid content.
		files := validReleaseVersionFiles(releaseTestVersion)
		if content, ok := files[path]; ok {
			return content, nil
		}
		return "", fmt.Errorf("object %s:%s not found", sha, path)
	}

	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when version file object is absent")
	}
	if !strings.Contains(err.Error(), "internal/version/version.go") {
		t.Errorf("error = %q, want it to name the missing path", err.Error())
	}
	if !strings.Contains(err.Error(), releaseTestSHA) {
		t.Errorf("error = %q, want it to name the commit sha %q", err.Error(), releaseTestSHA)
	}
	if !strings.Contains(err.Error(), "refuses to run") {
		t.Errorf("error = %q, want it to be a refusal message", err.Error())
	}
	if publishCalled {
		t.Fatal("git/tags must never be called when a version file object is absent")
	}
}

// TestReleaseTag_ObjectAbsent_Changelog_RefusesNamingShaAndPath verifies that
// when readCommittedFile cannot find CHANGELOG.md at objectSHA, the command
// refuses naming both the path and the sha, and never publishes.
func TestReleaseTag_ObjectAbsent_Changelog_RefusesNamingShaAndPath(t *testing.T) {
	d, _ := makeReleaseDeps(t, nil)
	var publishCalled bool
	d.execForgeAPI = func(name string, args []string, stdin string) (string, error) {
		if len(args) >= 2 && strings.Contains(args[1], "git/tags") {
			publishCalled = true
		}
		return defaultMockReleaseForgeAPI(name, args, stdin)
	}
	d.readCommittedFile = func(sha, path string) (string, error) {
		if path == "CHANGELOG.md" {
			return "", fmt.Errorf("path 'CHANGELOG.md' does not exist in '%s'", sha)
		}
		files := validReleaseVersionFiles(releaseTestVersion)
		if content, ok := files[path]; ok {
			return content, nil
		}
		return "", fmt.Errorf("object %s:%s not found", sha, path)
	}

	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when CHANGELOG.md object is absent")
	}
	if !strings.Contains(err.Error(), "CHANGELOG.md") {
		t.Errorf("error = %q, want it to name CHANGELOG.md", err.Error())
	}
	if !strings.Contains(err.Error(), releaseTestSHA) {
		t.Errorf("error = %q, want it to name the commit sha %q", err.Error(), releaseTestSHA)
	}
	if !strings.Contains(err.Error(), "refuses to run") {
		t.Errorf("error = %q, want it to be a refusal message", err.Error())
	}
	if publishCalled {
		t.Fatal("git/tags must never be called when CHANGELOG.md object is absent")
	}
}

// TestReleaseTag_TagMessageFromCommit_NotFromHypotheticalLocal proves that the
// tag payload message is sourced from readCommittedFile (the commit-anchored
// content), not from any alternative local source. The readCommittedFile mock
// returns CHANGELOG content with a body line that only appears in the committed
// blob, while a "hypothetical local" version would have a different body line.
// The test asserts the tag payload contains the committed body, not the other.
func TestReleaseTag_TagMessageFromCommit_NotFromHypotheticalLocal(t *testing.T) {
	d, _ := makeReleaseDeps(t, nil)

	// committedBody is a unique body line that only readCommittedFile can deliver.
	const committedBody = "- from-commit-object-anchor\n"
	const localOnlyBody = "- from-working-tree-NOT-anchored\n"

	// readCommittedFile returns committed-blob content for CHANGELOG.md.
	files := validReleaseVersionFiles(releaseTestVersion)
	d.readCommittedFile = func(sha, path string) (string, error) {
		if path == "CHANGELOG.md" {
			return fmt.Sprintf("# Changelog\n\n## [%s] - 2026-08-21\n\n### Added\n%s", releaseTestVersion, committedBody), nil
		}
		if content, ok := files[path]; ok {
			return content, nil
		}
		return "", fmt.Errorf("object %s:%s not found", sha, path)
	}

	var tagPayloadBody string
	d.execForgeAPI = func(name string, args []string, stdin string) (string, error) {
		if len(args) >= 2 && strings.Contains(args[1], "git/tags") {
			tagPayloadBody = stdin
		}
		return defaultMockReleaseForgeAPI(name, args, stdin)
	}

	if err := runReleaseTag(releaseTestVersion, d); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(tagPayloadBody, "from-commit-object-anchor") {
		t.Errorf("tag payload = %q, want it to contain the committed-blob body line", tagPayloadBody)
	}
	if strings.Contains(tagPayloadBody, "from-working-tree-NOT-anchored") {
		t.Errorf("tag payload = %q, must NOT contain the local-only body line (anchoring failure)", tagPayloadBody)
	}
	_ = localOnlyBody // documented here as the "not this" counterpart
}

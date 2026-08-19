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
		readFile: func(path string) (string, error) {
			content, ok := files[path]
			if !ok {
				return "", fmt.Errorf("file not found: %s", path)
			}
			return content, nil
		},
		out:         &bytes.Buffer{},
		configForge: "",
		repoDir:     "",
		availFn:     func(string) bool { return true },
		execForgeAPI: func(name string, args []string, stdin string) (string, error) {
			if len(args) >= 2 && strings.Contains(args[1], "git/tags") {
				return `{"sha":"tagobjectsha000"}`, nil
			}
			return `{}`, nil
		},
	}
	return d, g
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
		call := struct {
			name string
			args []string
			body string
		}{name, args, stdin}
		calls = append(calls, call)
		if strings.Contains(args[1], "git/tags") {
			return `{"sha":"tagobjectsha000"}`, nil
		}
		return `{}`, nil
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
		if strings.Contains(args[1], "git/tags") {
			return "", errors.New("401 Unauthorized")
		}
		refCalled = true
		return `{}`, nil
	}
	err := runReleaseTag(releaseTestVersion, d)
	if err == nil {
		t.Fatal("expected error when the tag object call fails")
	}
	if refCalled {
		t.Fatal("git/refs must never be called when git/tags failed — would create an orphan ref")
	}
}

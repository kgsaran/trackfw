package forge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func tmpDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func mkFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte{}, 0644); err != nil {
		t.Fatalf("mkFile %s: %v", name, err)
	}
}

func mkDir(t *testing.T, dir string, parts ...string) {
	t.Helper()
	target := filepath.Join(append([]string{dir}, parts...)...)
	if err := os.MkdirAll(target, 0755); err != nil {
		t.Fatalf("mkDir %v: %v", parts, err)
	}
}

func assertResolution(t *testing.T, res Resolution, wantForge, wantSource string) {
	t.Helper()
	if res.Forge != wantForge {
		t.Errorf("Forge: want %q, got %q", wantForge, res.Forge)
	}
	if res.Source != wantSource {
		t.Errorf("Source: want %q, got %q", wantSource, res.Source)
	}
}

// ---------------------------------------------------------------------------
// Precedence
// ---------------------------------------------------------------------------

func TestResolve_FlagWinsOverConfig(t *testing.T) {
	res, err := Resolve(Input{FlagForge: "github", ConfigForge: "gitlab"})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "github", "flag")
}

func TestResolve_FlagWinsOverRemote(t *testing.T) {
	res, err := Resolve(Input{
		FlagForge: "github",
		RemoteURL: "https://gitlab.com/org/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "github", "flag")
}

func TestResolve_FlagWinsOverCI(t *testing.T) {
	dir := tmpDir(t)
	mkFile(t, dir, ".gitlab-ci.yml")
	res, err := Resolve(Input{FlagForge: "bitbucket", RepoDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "bitbucket", "flag")
}

func TestResolve_ConfigWinsOverRemote(t *testing.T) {
	res, err := Resolve(Input{
		ConfigForge: "bitbucket",
		RemoteURL:   "https://github.com/org/repo.git",
	})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "bitbucket", "config")
}

func TestResolve_ConfigWinsOverCI(t *testing.T) {
	dir := tmpDir(t)
	mkFile(t, dir, ".gitlab-ci.yml")
	res, err := Resolve(Input{ConfigForge: "azure", RepoDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "azure", "config")
}

func TestResolve_RemoteWinsOverCI(t *testing.T) {
	dir := tmpDir(t)
	mkFile(t, dir, ".gitlab-ci.yml")
	res, err := Resolve(Input{
		RemoteURL: "https://github.com/org/repo.git",
		RepoDir:   dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "github", "remote")
}

// ---------------------------------------------------------------------------
// SSH and HTTPS equivalence for the 4 known hosts
// ---------------------------------------------------------------------------

var knownHosts = []struct {
	forge    string
	httpsURL string
	sshURL   string
}{
	{
		forge:    "github",
		httpsURL: "https://github.com/org/repo.git",
		sshURL:   "git@github.com:org/repo.git",
	},
	{
		forge:    "gitlab",
		httpsURL: "https://gitlab.com/org/repo.git",
		sshURL:   "git@gitlab.com:org/repo.git",
	},
	{
		forge:    "bitbucket",
		httpsURL: "https://bitbucket.org/org/repo.git",
		sshURL:   "git@bitbucket.org:org/repo.git",
	},
	{
		forge:    "azure",
		httpsURL: "https://dev.azure.com/org/project/_git/repo",
		sshURL:   "git@ssh.dev.azure.com:v3/org/project/repo",
	},
}

func TestResolve_SSH_HTTPS_Equivalence(t *testing.T) {
	for _, tc := range knownHosts {
		t.Run(tc.forge+"_https", func(t *testing.T) {
			res, err := Resolve(Input{RemoteURL: tc.httpsURL})
			if err != nil {
				t.Fatal(err)
			}
			assertResolution(t, res, tc.forge, "remote")
		})
		t.Run(tc.forge+"_ssh", func(t *testing.T) {
			res, err := Resolve(Input{RemoteURL: tc.sshURL})
			if err != nil {
				t.Fatal(err)
			}
			assertResolution(t, res, tc.forge, "remote")
		})
	}
}

// TestResolve_AzureSSHHost verifies that ssh.dev.azure.com (the Azure DevOps SSH hostname)
// is also recognised as Azure, matching the HTTPS form dev.azure.com.
func TestResolve_AzureSSHHost(t *testing.T) {
	res, err := Resolve(Input{RemoteURL: "ssh://git@ssh.dev.azure.com/v3/org/project/repo"})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "azure", "remote")
}

// ---------------------------------------------------------------------------
// Azure *.visualstudio.com
// ---------------------------------------------------------------------------

func TestResolve_AzureDevAzureCom(t *testing.T) {
	res, err := Resolve(Input{RemoteURL: "https://dev.azure.com/org/project/_git/repo"})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "azure", "remote")
}

func TestResolve_AzureVisualStudioCom(t *testing.T) {
	res, err := Resolve(Input{RemoteURL: "https://foo.visualstudio.com/DefaultCollection/_git/repo"})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "azure", "remote")
}

// ---------------------------------------------------------------------------
// Self-hosted / unknown host + CI desempate
// ---------------------------------------------------------------------------

func TestResolve_SelfHosted_GitlabCI(t *testing.T) {
	dir := tmpDir(t)
	mkFile(t, dir, ".gitlab-ci.yml")
	res, err := Resolve(Input{
		RemoteURL: "https://git.empresa.com.br/org/repo.git",
		RepoDir:   dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "gitlab", "ci")
}

func TestResolve_SelfHosted_GithubWorkflows(t *testing.T) {
	dir := tmpDir(t)
	mkDir(t, dir, ".github", "workflows")
	res, err := Resolve(Input{
		RemoteURL: "https://git.empresa.com.br/org/repo.git",
		RepoDir:   dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "github", "ci")
}

func TestResolve_SelfHosted_NoCI_IsManual(t *testing.T) {
	dir := tmpDir(t)
	res, err := Resolve(Input{
		RemoteURL: "https://git.empresa.com.br/org/repo.git",
		RepoDir:   dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "manual", "none")
}

// ---------------------------------------------------------------------------
// No remote (new repo) — must not panic
// ---------------------------------------------------------------------------

func TestResolve_NoRemote_CIDecides(t *testing.T) {
	dir := tmpDir(t)
	mkFile(t, dir, ".gitlab-ci.yml")
	res, err := Resolve(Input{RemoteURL: "", RepoDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "gitlab", "ci")
}

func TestResolve_NoRemote_NoCI_IsManual(t *testing.T) {
	dir := tmpDir(t)
	res, err := Resolve(Input{RemoteURL: "", RepoDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	assertResolution(t, res, "manual", "none")
}

// ---------------------------------------------------------------------------
// Manual is a valid result, never an error
// ---------------------------------------------------------------------------

func TestResolve_Manual_IsNotAnError(t *testing.T) {
	res, err := Resolve(Input{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if res.Forge != "manual" {
		t.Errorf("want manual, got %q", res.Forge)
	}
	if res.Source != "none" {
		t.Errorf("want none, got %q", res.Source)
	}
}

// ---------------------------------------------------------------------------
// Invalid forge value → error with list of valid values
// ---------------------------------------------------------------------------

func TestResolve_InvalidFlagForge_Error(t *testing.T) {
	_, err := Resolve(Input{FlagForge: "notaforge"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "notaforge") {
		t.Errorf("error should mention the invalid value; got: %s", msg)
	}
	for _, v := range ValidForges {
		if !strings.Contains(msg, v) {
			t.Errorf("error should list valid value %q; got: %s", v, msg)
		}
	}
}

func TestResolve_InvalidConfigForge_Error(t *testing.T) {
	_, err := Resolve(Input{ConfigForge: "svn"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "svn") {
		t.Errorf("error should mention the invalid value; got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// SSH long form (ssh://)
// ---------------------------------------------------------------------------

func TestExtractHost_SSHLongForm(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{"ssh://git@github.com/org/repo.git", "github.com"},
		{"ssh://git@gitlab.com/org/repo.git", "gitlab.com"},
		{"ssh://git@bitbucket.org/org/repo.git", "bitbucket.org"},
	}
	for _, tc := range cases {
		got := extractHost(tc.url)
		if got != tc.want {
			t.Errorf("extractHost(%q) = %q; want %q", tc.url, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// extractHost edge cases
// ---------------------------------------------------------------------------

func TestExtractHost_Empty(t *testing.T) {
	if got := extractHost(""); got != "" {
		t.Errorf("extractHost(\"\") = %q; want \"\"", got)
	}
}

func TestExtractHost_HTTPS_WithCredentials(t *testing.T) {
	got := extractHost("https://user:pass@github.com/org/repo.git")
	if got != "github.com" {
		t.Errorf("got %q, want github.com", got)
	}
}

package forge

import (
	"testing"
)

// ---------------------------------------------------------------------------
// helpers locais (sem redeclarar tmpDir/mkFile/mkDir/assertResolution de resolve_test.go)
// ---------------------------------------------------------------------------

// spyFn retorna uma availFn que registra as chamadas e devolve ret.
func spyFn(calls *[]string, ret bool) func(string) bool {
	return func(name string) bool {
		*calls = append(*calls, name)
		return ret
	}
}

func availTrue(_ string) bool  { return true }
func availFalse(_ string) bool { return false }

// ---------------------------------------------------------------------------
// TestNewAdapter — nouns e CLIName
// ---------------------------------------------------------------------------

func TestNewAdapter_Nouns(t *testing.T) {
	cases := []struct {
		forge    string
		wantNoun string
		wantCLI  string
	}{
		{"github", "Pull Request", "gh"},
		{"gitlab", "Merge Request", "glab"},
		{"azure", "Pull Request", "az"},
		{"bitbucket", "Pull Request", ""},
	}
	for _, tc := range cases {
		a := NewAdapter(tc.forge, availTrue)
		if a.Noun != tc.wantNoun {
			t.Errorf("forge=%s: Noun=%q want %q", tc.forge, a.Noun, tc.wantNoun)
		}
		if a.CLIName != tc.wantCLI {
			t.Errorf("forge=%s: CLIName=%q want %q", tc.forge, a.CLIName, tc.wantCLI)
		}
	}
}

// ---------------------------------------------------------------------------
// TestNewAdapter — disponibilidade via availFn injetada
// ---------------------------------------------------------------------------

func TestNewAdapter_AvailFn_True(t *testing.T) {
	for _, forge := range []string{"github", "gitlab", "azure"} {
		a := NewAdapter(forge, availTrue)
		if !a.Available {
			t.Errorf("forge=%s: Available should be true when availFn returns true", forge)
		}
	}
}

func TestNewAdapter_AvailFn_False(t *testing.T) {
	for _, forge := range []string{"github", "gitlab", "azure"} {
		a := NewAdapter(forge, availFalse)
		if a.Available {
			t.Errorf("forge=%s: Available should be false when availFn returns false", forge)
		}
	}
}

// ---------------------------------------------------------------------------
// TestNewAdapter — bitbucket nunca chama availFn
// ---------------------------------------------------------------------------

func TestNewAdapter_Bitbucket_NeverCallsAvailFn(t *testing.T) {
	var calls []string
	a := NewAdapter("bitbucket", spyFn(&calls, true))
	if a.Available {
		t.Error("bitbucket: Available deve ser sempre false")
	}
	if len(calls) != 0 {
		t.Errorf("bitbucket: availFn não deve ser chamada; chamada com %v", calls)
	}
}

func TestNewAdapter_GitHub_CallsAvailFnWithGh(t *testing.T) {
	var calls []string
	NewAdapter("github", spyFn(&calls, true))
	if len(calls) != 1 || calls[0] != "gh" {
		t.Errorf("github: esperado availFn chamada com [\"gh\"]; got %v", calls)
	}
}

func TestNewAdapter_GitLab_CallsAvailFnWithGlab(t *testing.T) {
	var calls []string
	NewAdapter("gitlab", spyFn(&calls, false))
	if len(calls) != 1 || calls[0] != "glab" {
		t.Errorf("gitlab: esperado availFn chamada com [\"glab\"]; got %v", calls)
	}
}

func TestNewAdapter_Azure_CallsAvailFnWithAz(t *testing.T) {
	var calls []string
	NewAdapter("azure", spyFn(&calls, false))
	if len(calls) != 1 || calls[0] != "az" {
		t.Errorf("azure: esperado availFn chamada com [\"az\"]; got %v", calls)
	}
}

// ---------------------------------------------------------------------------
// TestNewAdapter — forge desconhecido
// ---------------------------------------------------------------------------

func TestNewAdapter_UnknownForge(t *testing.T) {
	a := NewAdapter("unknown-forge", availTrue)
	if a.Available {
		t.Error("forge desconhecido: Available deve ser false")
	}
	if a.CLIName != "" {
		t.Errorf("forge desconhecido: CLIName deve ser vazio; got %q", a.CLIName)
	}
}

// ---------------------------------------------------------------------------
// TestFallbackURL
// ---------------------------------------------------------------------------

func TestFallbackURL(t *testing.T) {
	branch := "feat/my-feature"
	cases := []struct {
		forge     string
		remoteURL string
		wantURL   string
	}{
		// GitHub — HTTPS
		{
			"github",
			"https://github.com/org/repo.git",
			"https://github.com/org/repo/compare/feat/my-feature?expand=1",
		},
		// GitHub — SSH
		{
			"github",
			"git@github.com:org/repo.git",
			"https://github.com/org/repo/compare/feat/my-feature?expand=1",
		},
		// GitHub — self-hosted
		{
			"github",
			"https://git.company.com/org/repo.git",
			"https://git.company.com/org/repo/compare/feat/my-feature?expand=1",
		},
		// GitLab — HTTPS
		{
			"gitlab",
			"https://gitlab.com/org/repo.git",
			"https://gitlab.com/org/repo/-/merge_requests/new?merge_request[source_branch]=feat/my-feature",
		},
		// GitLab — SSH
		{
			"gitlab",
			"git@gitlab.com:org/repo.git",
			"https://gitlab.com/org/repo/-/merge_requests/new?merge_request[source_branch]=feat/my-feature",
		},
		// GitLab — self-hosted
		{
			"gitlab",
			"https://gitlab.company.com/org/repo.git",
			"https://gitlab.company.com/org/repo/-/merge_requests/new?merge_request[source_branch]=feat/my-feature",
		},
		// Bitbucket — HTTPS
		{
			"bitbucket",
			"https://bitbucket.org/org/repo.git",
			"https://bitbucket.org/org/repo/pull-requests/new?source=feat/my-feature",
		},
		// Bitbucket — SSH
		{
			"bitbucket",
			"git@bitbucket.org:org/repo.git",
			"https://bitbucket.org/org/repo/pull-requests/new?source=feat/my-feature",
		},
		// Azure — HTTPS
		{
			"azure",
			"https://dev.azure.com/org/project/_git/repo",
			"https://dev.azure.com/org/project/_git/repo/pullrequestcreate?sourceRef=feat/my-feature",
		},
		// Azure — SSH (normalização ssh.dev.azure.com → dev.azure.com)
		{
			"azure",
			"git@ssh.dev.azure.com:v3/org/project/repo",
			"https://dev.azure.com/org/project/_git/repo/pullrequestcreate?sourceRef=feat/my-feature",
		},
		// Azure — self-hosted
		{
			"azure",
			"https://azdo.company.com/org/project/_git/repo",
			"https://azdo.company.com/org/project/_git/repo/pullrequestcreate?sourceRef=feat/my-feature",
		},
	}
	for _, tc := range cases {
		a := NewAdapter(tc.forge, availFalse)
		got := a.FallbackURL(tc.remoteURL, branch)
		if got != tc.wantURL {
			t.Errorf("forge=%s remote=%q\n  got:  %s\n  want: %s", tc.forge, tc.remoteURL, got, tc.wantURL)
		}
	}
}

// ---------------------------------------------------------------------------
// TestFallbackURL — casos de borda
// ---------------------------------------------------------------------------

func TestFallbackURL_EmptyRemote(t *testing.T) {
	a := NewAdapter("github", availFalse)
	if got := a.FallbackURL("", "main"); got != "" {
		t.Errorf("remote vazio: esperado \"\"; got %q", got)
	}
}

func TestFallbackURL_UnknownForge(t *testing.T) {
	a := NewAdapter("unknown", availFalse)
	// URL base pode ser derivada mas sem padrão de sufixo → retorna ""
	got := a.FallbackURL("https://example.com/org/repo.git", "main")
	if got != "" {
		t.Errorf("forge desconhecido: esperado \"\"; got %q", got)
	}
}

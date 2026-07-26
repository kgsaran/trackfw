// Package forge implements forge detection and resolution for trackfw ship.
// Precedence (highest to lowest):
//  1. --forge flag (explicit override)
//  2. forge: field in trackfw.yaml
//  3. Host extracted from git remote get-url origin
//  4. CI configuration files detected in the repo root
//  5. manual — never an error; push is performed and the PR URL is printed
package forge

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"os"
)

// ValidForges lists the accepted values for the --forge flag and forge: config field.
var ValidForges = []string{"github", "gitlab", "bitbucket", "azure"}

// Resolution holds the resolved forge and the source that decided it.
type Resolution struct {
	Forge  string // "github", "gitlab", "bitbucket", "azure", or "manual"
	Source string // "flag", "config", "remote", "ci", or "none"
}

// Input holds all data needed for forge resolution, enabling full injection in tests
// (no subprocess calls inside Resolve itself).
type Input struct {
	FlagForge   string // value from --forge flag; empty if not provided
	ConfigForge string // value from trackfw.yaml forge: field; empty if not set
	RemoteURL   string // output of "git remote get-url origin"; empty if unavailable
	RepoDir     string // repository root; used to look for CI indicator files
}

// Resolve determines the active forge using the precedence defined in the ADR:
//  1. FlagForge
//  2. ConfigForge
//  3. Host extracted from RemoteURL (known hosts only)
//  4. CI files in RepoDir (.gitlab-ci.yml → gitlab; .github/workflows/ → github)
//  5. Resolution{Forge: "manual", Source: "none"} — never returns an error for this case
//
// Returns an error only when an explicitly provided forge value (flag or config)
// is not in ValidForges.
func Resolve(in Input) (Resolution, error) {
	// 1. Explicit flag wins everything.
	if in.FlagForge != "" {
		if err := validateForge(in.FlagForge); err != nil {
			return Resolution{}, err
		}
		return Resolution{Forge: in.FlagForge, Source: "flag"}, nil
	}

	// 2. Config field.
	if in.ConfigForge != "" {
		if err := validateForge(in.ConfigForge); err != nil {
			return Resolution{}, err
		}
		return Resolution{Forge: in.ConfigForge, Source: "config"}, nil
	}

	// 3. Remote URL — only for known hosts.
	if in.RemoteURL != "" {
		if forge := forgeFromRemoteURL(in.RemoteURL); forge != "" {
			return Resolution{Forge: forge, Source: "remote"}, nil
		}
	}

	// 4. CI files — desempate for self-hosted / unknown host.
	if in.RepoDir != "" {
		if forge := forgeFromCI(in.RepoDir); forge != "" {
			return Resolution{Forge: forge, Source: "ci"}, nil
		}
	}

	// 5. Manual — not an error.
	return Resolution{Forge: "manual", Source: "none"}, nil
}

// ResolveFromRepo is the production entry point. It runs "git remote get-url origin"
// in repoDir to obtain the remote URL, then delegates to Resolve.
func ResolveFromRepo(flagForge, configForge, repoDir string) (Resolution, error) {
	return Resolve(Input{
		FlagForge:   flagForge,
		ConfigForge: configForge,
		RemoteURL:   gitRemoteURL(repoDir),
		RepoDir:     repoDir,
	})
}

// validateForge returns an error when forge is not in ValidForges.
func validateForge(forge string) error {
	for _, v := range ValidForges {
		if forge == v {
			return nil
		}
	}
	return fmt.Errorf("invalid forge %q: accepted values are %s",
		forge, strings.Join(ValidForges, ", "))
}

// forgeFromRemoteURL extracts the host from rawURL and maps it to a forge name.
// Returns "" for unknown or self-hosted hosts (those fall through to CI detection).
func forgeFromRemoteURL(rawURL string) string {
	host := extractHost(rawURL)
	return hostToForge(host)
}

// extractHost returns the lowercase hostname from HTTPS, SSH (git@) or ssh:// URLs.
func extractHost(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)

	switch {
	// SSH short form: git@github.com:org/repo.git
	case strings.HasPrefix(rawURL, "git@"):
		rest := rawURL[4:] // strip "git@"
		if idx := strings.IndexByte(rest, ':'); idx >= 0 {
			return strings.ToLower(rest[:idx])
		}

	// SSH long form: ssh://git@github.com/org/repo.git
	case strings.HasPrefix(rawURL, "ssh://"):
		rest := rawURL[6:] // strip "ssh://"
		if idx := strings.IndexByte(rest, '@'); idx >= 0 {
			rest = rest[idx+1:] // strip user@
		}
		if idx := strings.IndexByte(rest, '/'); idx >= 0 {
			return strings.ToLower(rest[:idx])
		}
		return strings.ToLower(rest)

	// HTTPS / HTTP
	case strings.HasPrefix(rawURL, "https://") || strings.HasPrefix(rawURL, "http://"):
		rawURL = strings.TrimPrefix(rawURL, "https://")
		rawURL = strings.TrimPrefix(rawURL, "http://")
		// strip optional user:pass@
		if idx := strings.IndexByte(rawURL, '@'); idx >= 0 {
			rawURL = rawURL[idx+1:]
		}
		if idx := strings.IndexByte(rawURL, '/'); idx >= 0 {
			return strings.ToLower(rawURL[:idx])
		}
		return strings.ToLower(rawURL)
	}

	return ""
}

// hostToForge maps a known hostname to its forge identifier.
// Returns "" for unknown / self-hosted hosts.
//
// Azure DevOps uses two distinct hostnames:
//   - HTTPS: dev.azure.com
//   - SSH:   ssh.dev.azure.com (and possibly other *.dev.azure.com subdomains)
//   - Legacy: *.visualstudio.com
func hostToForge(host string) string {
	switch {
	case host == "github.com":
		return "github"
	case host == "gitlab.com":
		return "gitlab"
	case host == "bitbucket.org":
		return "bitbucket"
	case host == "dev.azure.com",
		strings.HasSuffix(host, ".dev.azure.com"),
		strings.HasSuffix(host, ".visualstudio.com"):
		return "azure"
	}
	return ""
}

// forgeFromCI inspects CI indicator files in repoDir:
//   - .gitlab-ci.yml → gitlab
//   - .github/workflows/ (directory) → github
//
// Priority: GitLab before GitHub (alphabetical order of the specs; only one is
// usually present).
func forgeFromCI(repoDir string) string {
	if _, err := os.Stat(filepath.Join(repoDir, ".gitlab-ci.yml")); err == nil {
		return "gitlab"
	}
	if fi, err := os.Stat(filepath.Join(repoDir, ".github", "workflows")); err == nil && fi.IsDir() {
		return "github"
	}
	return ""
}

// gitRemoteURL runs "git remote get-url origin" in repoDir.
// Returns "" on any error (no remote, not a git repo, git not found, etc.).
func gitRemoteURL(repoDir string) string {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

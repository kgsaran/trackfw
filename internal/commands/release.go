package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/changelog"
	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/forge"
	"github.com/spf13/cobra"
)

// releaseDeps holds injectable dependencies for `trackfw release tag`, mirroring the
// shipDeps pattern in ship.go so tests never execute real git/gh commands.
type releaseDeps struct {
	// execGit runs a git command and returns (trimmed-stdout, error). Read-only and
	// write commands both go through this — release tag never uses "add ." semantics.
	execGit func(args ...string) (string, error)

	// readFile reads a file relative to repoDir (the 4 version files + CHANGELOG.md).
	readFile func(path string) (string, error)

	out io.Writer

	// configForge is the forge: value from trackfw.yaml (injected; production: config.Load().Forge).
	configForge string
	// repoDir is the repo root, used for CI file detection during forge resolution.
	repoDir string
	// availFn injects CLI availability check for forge.NewAdapter. nil uses the production default.
	availFn func(string) bool

	// execForgeAPI runs a forge CLI command that reads a JSON body from stdin and returns
	// captured stdout (the JSON response). Used for the two `gh api` calls that publish the
	// annotated tag. nil uses defaultExecForgeAPI.
	execForgeAPI func(name string, args []string, stdin string) (string, error)
}

// Named refusal message formats — kept as constants so the ML-2B parity gate has a single
// place to compare byte-for-byte across the 3 CLIs. Every precondition refusal names what to
// fix, per the ADR's decision that release tag prefers refusing over guessing.
// See ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md.
const (
	releaseTagDirtyTreeFmt = "trackfw release tag refuses to run: working tree is not clean.\n%s\nCommit or stash your changes before tagging a release."

	releaseTagFetchFailedFmt = "trackfw release tag refuses to run: could not fetch origin (%s). Check your network/credentials and retry."

	releaseTagLocalBranchStaleFmt = "trackfw release tag refuses to run: local %q is not up to date with origin/%s (local %s, remote %s). Run: git pull"

	releaseTagVersionMismatchFmt = "trackfw release tag refuses to run: %s has version %q, expected %q. Update it to match before tagging."

	releaseTagChangelogMissingFmt = "trackfw release tag refuses to run: %s. Add a \"## [%s] - YYYY-MM-DD\" section to CHANGELOG.md before tagging."

	releaseTagExistsLocalFmt = "trackfw release tag refuses to run: tag %q already exists locally. Delete it first (git tag -d %s) or choose a different version."

	releaseTagExistsRemoteFmt = "trackfw release tag refuses to run: tag %q already exists on origin. Choose a different version."

	releaseTagNoForgeCLIFmt = "trackfw release tag requires the GitHub CLI (gh) to publish the tag. No forge CLI is available for this repository — install and authenticate gh, or push the tag manually: git tag -a %s -m \"<CHANGELOG.md section>\" %s && git push origin %s"

	releaseTagUnsupportedForgeFmt = "trackfw release tag currently only supports GitHub (resolved forge: %q). Publishing tag %s on this forge is not implemented yet — commit to tag: %s. Create %s through your forge's web UI, or open an issue requesting support for this forge."

	releaseTagNoGitIdentityMsg = "trackfw release tag refuses to run: git config user.name and user.email must be set to create an annotated tag (git config user.name \"Your Name\" && git config user.email you@example.com)."
)

// releaseVersionFile describes one location where the project version is recorded, and how
// to extract it. All 5 checks (4 files — pypi/trackfw/__init__.py holds 2 occurrences) must
// agree with the version requested on the CLI before release tag proceeds.
type releaseVersionFile struct {
	label   string // used in refusal messages — names exactly what diverges
	path    string // relative to repoDir
	extract func(content string) (string, error)
}

var releaseVersionFiles = []releaseVersionFile{
	{"internal/version/version.go", "internal/version/version.go", extractGoVersion},
	{"npm/package.json", "npm/package.json", extractNpmVersion},
	{"pypi/pyproject.toml", "pypi/pyproject.toml", extractPyprojectVersion},
	{"pypi/trackfw/__init__.py (importlib.metadata fallback)", "pypi/trackfw/__init__.py", extractInitTryVersion},
	{"pypi/trackfw/__init__.py (except fallback)", "pypi/trackfw/__init__.py", extractInitExceptVersion},
}

var goVersionRE = regexp.MustCompile(`Version\s*=\s*"([^"]+)"`)

func extractGoVersion(content string) (string, error) {
	m := goVersionRE.FindStringSubmatch(content)
	if m == nil {
		return "", fmt.Errorf(`could not find Version = "..." in internal/version/version.go`)
	}
	return m[1], nil
}

func extractNpmVersion(content string) (string, error) {
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return "", fmt.Errorf("could not parse npm/package.json: %w", err)
	}
	if pkg.Version == "" {
		return "", fmt.Errorf(`npm/package.json has no "version" field`)
	}
	return pkg.Version, nil
}

var pyprojectVersionRE = regexp.MustCompile(`(?m)^version\s*=\s*"([^"]+)"`)

func extractPyprojectVersion(content string) (string, error) {
	m := pyprojectVersionRE.FindStringSubmatch(content)
	if m == nil {
		return "", fmt.Errorf(`could not find version = "..." in pypi/pyproject.toml`)
	}
	return m[1], nil
}

// initTryVersionRE matches the fallback in `__version__ = version("trackfw") or "7.1.0"`.
var initTryVersionRE = regexp.MustCompile(`or\s+"([^"]+)"`)

func extractInitTryVersion(content string) (string, error) {
	m := initTryVersionRE.FindStringSubmatch(content)
	if m == nil {
		return "", fmt.Errorf("could not find the importlib.metadata fallback version in pypi/trackfw/__init__.py")
	}
	return m[1], nil
}

// initExceptVersionRE matches the except-block's `__version__ = "7.1.0"` — distinct from the
// try-block line above, which never starts with `__version__ = "` directly (it starts with
// `__version__ = version(...)`).
var initExceptVersionRE = regexp.MustCompile(`__version__\s*=\s*"([^"]+)"`)

func extractInitExceptVersion(content string) (string, error) {
	m := initExceptVersionRE.FindStringSubmatch(content)
	if m == nil {
		return "", fmt.Errorf("could not find the except-block fallback version in pypi/trackfw/__init__.py")
	}
	return m[1], nil
}

func newReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Governed release operations",
	}
	cmd.AddCommand(newReleaseTagCmd())
	return cmd
}

func newReleaseTagCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tag <version>",
		Short: "Create and publish an annotated release tag",
		Long: `trackfw release tag creates and publishes an annotated git tag for a release.

It exists because 'trackfw ship' only pushes branches — tag is not a branch operation, and
ship's governance gate ("REQ + roadmap in wip/") does not apply to release. See
ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md.

Every precondition below refuses with a message naming what to fix — this command never
guesses:
  1. Working tree must be clean.
  2. The default branch (main/master), if checked out locally, must be up to date with origin.
  3. The 4 version files must all match <version> exactly.
  4. CHANGELOG.md must have a "## [<version>] - YYYY-MM-DD" section.
  5. The tag must not already exist, locally or on origin.
  6. The GitHub CLI (gh) must be available and authenticated — release tag currently only
     supports GitHub; other forges are refused with instructions to push the tag manually.

On success, it publishes the tag via two GitHub API calls (POST git/tags then POST git/refs),
preserving the annotation — a plain 'git push origin <tag>' loses it if the tag was created
without -a, and the git-branch-guard blocks that push form anyway.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			deps := releaseDeps{
				execGit:      defaultGitExec,
				readFile:     defaultReleaseReadFile,
				out:          cmd.OutOrStdout(),
				configForge:  config.Load().Forge,
				repoDir:      ".",
				availFn:      nil,
				execForgeAPI: defaultExecForgeAPI,
			}
			return runReleaseTag(args[0], deps)
		},
	}
	return cmd
}

// defaultReleaseReadFile reads a file relative to the current working directory (the repo
// root, in production use).
func defaultReleaseReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// defaultExecForgeAPI runs a forge CLI command (gh api ...) feeding stdin and capturing
// stdout, so the JSON response can be parsed. On failure, surfaces the CLI's real stderr text
// instead of exec's generic "exit status N" — same reasoning as defaultGitExec/
// defaultCheckPROpen in ship.go.
func defaultExecForgeAPI(name string, args []string, stdin string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			if exitErr, ok := err.(*exec.ExitError); ok {
				msg = fmt.Sprintf("%s %s exited with %d", name, strings.Join(args, " "), exitErr.ExitCode())
			} else {
				msg = err.Error()
			}
		}
		return strings.TrimSpace(stdout.String()), errors.New(msg)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// normalizeReleaseVersion strips an optional leading "v"/"V", matching
// changelog.FindVersion's own normalization so both agree on what "the version" means.
func normalizeReleaseVersion(v string) string {
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		return v[1:]
	}
	return v
}

// runReleaseTag implements `trackfw release tag <version>`. Every precondition below is
// checked before any write — the risk this command carries (per the roadmap) is publishing a
// wrong tag to a public repository, so it always refuses rather than guesses.
func runReleaseTag(versionArg string, deps releaseDeps) error {
	version := normalizeReleaseVersion(strings.TrimSpace(versionArg))
	tagName := "v" + version

	// ─── Precondition 1: clean working tree ──────────────────────────────────
	statusOut, err := deps.execGit("status", "--porcelain")
	if err != nil {
		return fmt.Errorf("could not determine working tree status: %w", err)
	}
	if strings.TrimSpace(statusOut) != "" {
		return fmt.Errorf(releaseTagDirtyTreeFmt, statusOut)
	}

	// ─── Precondition 2: default branch up to date with origin ──────────────
	if _, err := deps.execGit("fetch", "origin", "--prune"); err != nil {
		return fmt.Errorf(releaseTagFetchFailedFmt, err)
	}

	base := defaultBaseBranch(deps.execGit)

	objectSHA, err := deps.execGit("rev-parse", "origin/"+base)
	if err != nil {
		return fmt.Errorf("could not resolve origin/%s: %w", base, err)
	}
	objectSHA = strings.TrimSpace(objectSHA)

	if _, err := deps.execGit("rev-parse", "-q", "--verify", "refs/heads/"+base); err == nil {
		localSHA, lerr := deps.execGit("rev-parse", "refs/heads/"+base)
		if lerr == nil {
			localSHA = strings.TrimSpace(localSHA)
			if localSHA != objectSHA {
				return fmt.Errorf(releaseTagLocalBranchStaleFmt, base, base, localSHA, objectSHA)
			}
		}
	}

	// ─── Precondition 3: the 4 version files must all match ─────────────────
	for _, vf := range releaseVersionFiles {
		content, rerr := deps.readFile(vf.path)
		if rerr != nil {
			return fmt.Errorf("trackfw release tag refuses to run: could not read %s: %w", vf.path, rerr)
		}
		got, eerr := vf.extract(content)
		if eerr != nil {
			return fmt.Errorf("trackfw release tag refuses to run: %w", eerr)
		}
		if got != version {
			return fmt.Errorf(releaseTagVersionMismatchFmt, vf.label, got, version)
		}
	}

	// ─── Precondition 4: CHANGELOG.md has the version's section ─────────────
	changelogContent, err := deps.readFile("CHANGELOG.md")
	if err != nil {
		return fmt.Errorf("trackfw release tag refuses to run: could not read CHANGELOG.md: %w", err)
	}
	sections, err := changelog.ParseSections(changelogContent)
	if err != nil {
		return fmt.Errorf("trackfw release tag refuses to run: could not parse CHANGELOG.md: %w", err)
	}
	section, err := changelog.FindVersion(sections, version)
	if err != nil {
		return fmt.Errorf(releaseTagChangelogMissingFmt, err.Error(), version)
	}
	tagMessage := changelog.FormatSection(section)

	// ─── Precondition 5: tag must not already exist, local or remote ────────
	if _, err := deps.execGit("rev-parse", "-q", "--verify", "refs/tags/"+tagName); err == nil {
		return fmt.Errorf(releaseTagExistsLocalFmt, tagName, tagName)
	}
	remoteTagOut, _ := deps.execGit("ls-remote", "--tags", "origin", "refs/tags/"+tagName)
	if strings.TrimSpace(remoteTagOut) != "" {
		return fmt.Errorf(releaseTagExistsRemoteFmt, tagName)
	}

	// ─── Precondition 6: forge CLI available — GitHub only, for now ─────────
	remoteURL, _ := deps.execGit("remote", "get-url", "origin")
	remoteURL = strings.TrimSpace(remoteURL)

	resolution, resErr := forge.Resolve(forge.Input{
		ConfigForge: deps.configForge,
		RemoteURL:   remoteURL,
		RepoDir:     deps.repoDir,
	})
	if resErr != nil {
		return resErr
	}

	if resolution.Forge != "github" {
		return fmt.Errorf(releaseTagUnsupportedForgeFmt, resolution.Forge, tagName, objectSHA, tagName)
	}

	adapter := forge.NewAdapter(resolution.Forge, deps.availFn)
	if !adapter.Available {
		return fmt.Errorf(releaseTagNoForgeCLIFmt, tagName, objectSHA, tagName)
	}

	// ─── Tagger identity ──────────────────────────────────────────────────
	name, _ := deps.execGit("config", "user.name")
	email, _ := deps.execGit("config", "user.email")
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if name == "" || email == "" {
		return fmt.Errorf("%s", releaseTagNoGitIdentityMsg)
	}

	// ─── Publish: two gh api calls, preserving the annotation ───────────────
	// Reference implementation validated in production (v7.1.0):
	//   POST git/tags  -> sha of the tag OBJECT (not visible via a ref yet)
	//   POST git/refs  -> refs/tags/<tag> pointing at that object's sha
	// Both required: the first alone creates the object but no ref points at it; a plain
	// `git push origin <tag>` from a lightweight local tag would lose the annotation.
	tagPayload, merr := json.Marshal(struct {
		Tag     string `json:"tag"`
		Message string `json:"message"`
		Object  string `json:"object"`
		Type    string `json:"type"`
		Tagger  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  string `json:"date"`
		} `json:"tagger"`
	}{
		Tag:     tagName,
		Message: tagMessage,
		Object:  objectSHA,
		Type:    "commit",
		Tagger: struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  string `json:"date"`
		}{Name: name, Email: email, Date: time.Now().UTC().Format(time.RFC3339)},
	})
	if merr != nil {
		return fmt.Errorf("trackfw release tag: could not build tag object payload: %w", merr)
	}

	tagResp, err := deps.execForgeAPI("gh", []string{"api", "repos/{owner}/{repo}/git/tags", "--method", "POST", "--input", "-"}, string(tagPayload))
	if err != nil {
		return fmt.Errorf("trackfw release tag: gh api failed creating the tag object: %w", err)
	}

	var tagObj struct {
		SHA string `json:"sha"`
	}
	if jerr := json.Unmarshal([]byte(tagResp), &tagObj); jerr != nil || tagObj.SHA == "" {
		return fmt.Errorf("trackfw release tag: could not parse the tag object response from gh api: %s", tagResp)
	}

	refPayload, merr := json.Marshal(struct {
		Ref string `json:"ref"`
		SHA string `json:"sha"`
	}{Ref: "refs/tags/" + tagName, SHA: tagObj.SHA})
	if merr != nil {
		return fmt.Errorf("trackfw release tag: could not build ref payload: %w", merr)
	}

	if _, err := deps.execForgeAPI("gh", []string{"api", "repos/{owner}/{repo}/git/refs", "--method", "POST", "--input", "-"}, string(refPayload)); err != nil {
		return fmt.Errorf("trackfw release tag: gh api failed creating the tag ref: %w", err)
	}

	fmt.Fprintf(deps.out, "Tag published: %s\n", tagName)
	fmt.Fprintf(deps.out, "  tag object: %s\n", tagObj.SHA)
	fmt.Fprintf(deps.out, "  commit:     %s\n", objectSHA)
	fmt.Fprintf(deps.out, "\nrelease tag complete.\n")
	return nil
}

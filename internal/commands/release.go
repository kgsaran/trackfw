package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

	// readCommittedFile reads a file from a specific commit object (git show <sha>:<path>)
	// and returns the content byte-for-byte, WITHOUT trimming — callers rely on verbatim
	// content (CHANGELOG sections, version strings). Absent object → error naming what is
	// missing; never falls back to the working tree.
	readCommittedFile func(sha, path string) (string, error)

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
	releaseTagDirtyTreeFmt = "trackfw release tag refuses to run: working tree is not clean.\n%s\nCommit your changes (trackfw commit) before tagging a release."

	releaseTagFetchFailedFmt = "trackfw release tag refuses to run: could not fetch origin (%s). Check your network/credentials and retry."

	releaseTagLocalBranchStaleFmt = "trackfw release tag refuses to run: local %q is not up to date with origin/%s (local %s, remote %s). Run: git pull"

	releaseTagVersionMismatchFmt = "trackfw release tag refuses to run: %s has version %q, expected %q. Update it to match before tagging."

	releaseTagChangelogMissingFmt = "trackfw release tag refuses to run: %s. Add a \"## [%s] - YYYY-MM-DD\" section to CHANGELOG.md before tagging."

	releaseTagExistsLocalFmt = "trackfw release tag refuses to run: tag %q already exists locally. Delete it first (git tag -d %s) or choose a different version."

	releaseTagExistsRemoteFmt = "trackfw release tag refuses to run: tag %q already exists on origin. Choose a different version."

	releaseTagNoForgeCLIFmt = "trackfw release tag requires the GitHub CLI (gh) to publish the tag. No forge CLI is available for this repository — install and authenticate gh, or push the tag manually: git tag -a %s -m \"<CHANGELOG.md section>\" %s && git push origin %s"

	releaseTagUnsupportedForgeFmt = "trackfw release tag currently only supports GitHub (resolved forge: %q). Publishing tag %s on this forge is not implemented yet — commit to tag: %s. Create %s through your forge's web UI, or open an issue requesting support for this forge."

	releaseTagNoGitIdentityMsg = "trackfw release tag refuses to run: git config user.name and user.email must be set to create an annotated tag (git config user.name \"Your Name\" && git config user.email you@example.com)."

	// releaseTagCommitDivergesFmt fires when a LOCAL ref (origin/<forge's default branch>'s
	// resolved sha) disagrees with what the forge itself reports for that same branch. This ref
	// is writable inside the clone (git update-ref) — the forge is the only source that is not —
	// so a disagreement is refused, never silently resolved by picking one side. The BRANCH NAME
	// itself comes from the forge unconditionally (no local-vs-forge name check — see the call
	// site) since a fresh/shallow clone legitimately has no local opinion on it at all. See
	// ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md, Emenda 1.
	releaseTagCommitDivergesFmt = "trackfw release tag refuses to run: local origin/%s (%s) diverges from the forge's %s tip (%s). A local ref can be stale or forged — investigate before retrying: git fetch origin --prune"

	releaseTagForgeAPIDefaultBranchFailedFmt = "trackfw release tag: gh api failed resolving the repository's default branch from the forge: %w"

	releaseTagForgeAPICommitFailedFmt = "trackfw release tag: gh api failed resolving the forge's tip commit for %s: %w"

	// releaseTagObjectAbsentFmt fires when git show <sha>:<path> fails (object not present
	// locally after the fetch that Precondition 2 already ran). Names the path and the sha so
	// the user knows exactly what is missing. Never falls back to the working tree — that would
	// undo the anchoring that this ML exists to provide. See ADR-2026-08-21-release-tag-le-
	// versao-e-changelog-do-commit-ancorado.md.
	releaseTagObjectAbsentFmt = "trackfw release tag refuses to run: could not read %s at commit %s: %s"
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
				execGit:           defaultGitExec,
				readCommittedFile: defaultReleaseReadCommittedFile,
				out:               cmd.OutOrStdout(),
				configForge:       config.Load().Forge,
				repoDir:           ".",
				availFn:           nil,
				execForgeAPI:      defaultExecForgeAPI,
			}
			return runReleaseTag(args[0], deps)
		},
	}
	return cmd
}

// defaultReleaseReadCommittedFile reads a file from a specific commit object (git show
// <sha>:<path>) and returns the content verbatim — stdout is NOT trimmed because callers rely
// on byte-exact content (CHANGELOG sections, version strings with newlines). On any failure
// (object absent, sha unknown) the error surfaces git's real stderr; there is no fallback to
// the working tree. See ADR-2026-08-21-release-tag-le-versao-e-changelog-do-commit-ancorado.md.
func defaultReleaseReadCommittedFile(sha, path string) (string, error) {
	cmd := exec.Command("git", "--no-replace-objects", "show", sha+":"+path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			if exitErr, ok := err.(*exec.ExitError); ok {
				msg = fmt.Sprintf("git show %s:%s exited with %d", sha, path, exitErr.ExitCode())
			} else {
				msg = err.Error()
			}
		}
		return "", errors.New(msg)
	}
	return stdout.String(), nil
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
	// base is symref-derived — a LOCAL, gravable ref (git symbolic-ref). It is used below only
	// as (a) the value the forge's default_branch must agree with, and (b) input to the
	// local-branch-staleness check, which is unrelated to the forge and unaffected by it.
	if _, err := deps.execGit("fetch", "origin", "--prune"); err != nil {
		return fmt.Errorf(releaseTagFetchFailedFmt, err)
	}

	base := defaultBaseBranch(deps.execGit)

	// localSHA (origin/<base>'s local tracking ref) is best-effort and non-fatal here: it is a
	// cross-check candidate against the forge, never the source of the commit target. A failure
	// to resolve it (e.g. a symref repointed to a branch this clone has no tracking ref for)
	// must not block reaching the forge resolution below — the forge's answer is what decides.
	localSHA := ""
	if out, lerr := deps.execGit("rev-parse", "origin/"+base); lerr == nil {
		localSHA = strings.TrimSpace(out)

		// Local-branch-staleness check: unrelated to the forge — both sides here are local
		// (refs/heads/<base> vs the origin/<base> ref just resolved above) — kept exactly as
		// before.
		if _, verr := deps.execGit("rev-parse", "-q", "--verify", "refs/heads/"+base); verr == nil {
			localBranchSHA, lberr := deps.execGit("rev-parse", "refs/heads/"+base)
			if lberr == nil {
				localBranchSHA = strings.TrimSpace(localBranchSHA)
				if localBranchSHA != localSHA {
					return fmt.Errorf(releaseTagLocalBranchStaleFmt, base, base, localBranchSHA, localSHA)
				}
			}
		}
	}

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
		// No forge to ask — localSHA is shown purely as an informational hint for the manual
		// fallback text below; the command never publishes on this path.
		return fmt.Errorf(releaseTagUnsupportedForgeFmt, resolution.Forge, tagName, localSHA, tagName)
	}

	adapter := forge.NewAdapter(resolution.Forge, deps.availFn)
	if !adapter.Available {
		// Same reasoning as above: no forge CLI to ask, localSHA is informational only.
		return fmt.Errorf(releaseTagNoForgeCLIFmt, tagName, localSHA, tagName)
	}

	// ─── The commit-target comes from the forge, never from a local ref ─────
	// The forge's default_branch is authoritative for the BRANCH NAME — unconditionally, with no
	// refusal if it disagrees with the local symref-derived base (a fresh/shallow clone may have
	// no origin/HEAD symref at all, defaultBaseBranch then falls back to "main"; refusing on that
	// mismatch would be a false refusal against a legitimate repo, not a security check). Only the
	// forge's SHA is cross-checked against a local ref — resolved fresh, keyed to the forge's own
	// branch name, never to the (possibly-forged) local base. See ADR-2026-08-19-caminho-
	// governado-para-push-forcado-e-tag-de-release.md, Emenda 1.
	repoInfoResp, err := deps.execForgeAPI("gh", []string{"api", "repos/{owner}/{repo}"}, "")
	if err != nil {
		return fmt.Errorf(releaseTagForgeAPIDefaultBranchFailedFmt, err)
	}
	var repoInfo struct {
		DefaultBranch string `json:"default_branch"`
	}
	if jerr := json.Unmarshal([]byte(repoInfoResp), &repoInfo); jerr != nil || repoInfo.DefaultBranch == "" {
		return fmt.Errorf("trackfw release tag: could not parse default_branch from the forge's repository response: %s", repoInfoResp)
	}

	// forgeLocalSHA is resolved fresh against the forge's own branch name — deliberately NOT
	// reusing localSHA above, which was keyed to the symref-derived base and may name a different
	// branch (stale symref, or a fresh clone with no symref at all). Best-effort/non-fatal, same
	// reasoning as localSHA: absence must not block reaching the publish step below.
	forgeLocalSHA := ""
	if out, lerr := deps.execGit("rev-parse", "origin/"+repoInfo.DefaultBranch); lerr == nil {
		forgeLocalSHA = strings.TrimSpace(out)
	}

	commitResp, err := deps.execForgeAPI("gh", []string{"api", "repos/{owner}/{repo}/commits/" + repoInfo.DefaultBranch}, "")
	if err != nil {
		return fmt.Errorf(releaseTagForgeAPICommitFailedFmt, repoInfo.DefaultBranch, err)
	}
	var commitObj struct {
		SHA string `json:"sha"`
	}
	if jerr := json.Unmarshal([]byte(commitResp), &commitObj); jerr != nil || commitObj.SHA == "" {
		return fmt.Errorf("trackfw release tag: could not parse the forge's commit response for %s: %s", repoInfo.DefaultBranch, commitResp)
	}

	if forgeLocalSHA != "" && forgeLocalSHA != commitObj.SHA {
		return fmt.Errorf(releaseTagCommitDivergesFmt, repoInfo.DefaultBranch, forgeLocalSHA, repoInfo.DefaultBranch, commitObj.SHA)
	}

	// objectSHA is now authoritative — resolved from the forge, cross-checked (not sourced)
	// against the local ref above.
	objectSHA := commitObj.SHA

	// ─── Precondition 3: version files in the commit-target must all match ──
	// Content is read from objectSHA via git show, NOT from the working tree. Objects are
	// content-addressed: given a sha that comes from the forge, the content is cyptographically
	// determined — a local edit that was not committed cannot influence the tag message.
	// Absent object → refuse naming sha+path; never fall back to local. See ADR-2026-08-21.
	for _, vf := range releaseVersionFiles {
		content, rerr := deps.readCommittedFile(objectSHA, vf.path)
		if rerr != nil {
			return fmt.Errorf(releaseTagObjectAbsentFmt, vf.path, objectSHA, rerr.Error())
		}
		got, eerr := vf.extract(content)
		if eerr != nil {
			return fmt.Errorf("trackfw release tag refuses to run: %w", eerr)
		}
		if got != version {
			return fmt.Errorf(releaseTagVersionMismatchFmt, vf.label, got, version)
		}
	}

	// ─── Precondition 4: CHANGELOG.md in the commit-target has the version's section ──
	// Same anchoring as P3: content comes from objectSHA, never from the working tree.
	changelogContent, err := deps.readCommittedFile(objectSHA, "CHANGELOG.md")
	if err != nil {
		return fmt.Errorf(releaseTagObjectAbsentFmt, "CHANGELOG.md", objectSHA, err.Error())
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

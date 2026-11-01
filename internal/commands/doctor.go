package commands

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/identity"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var jsonOut bool
	var remote bool

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Detect artifacts on disk missing from the manifest, distinguishing hand-modified artifacts from unknown content",
		Long: `trackfw doctor sweeps every catalog-managed agents/skills destination, in
both project and global scope, and reports three distinct disk/manifest
mismatches with different remedies — they are never merged:

  unregistered-write   on-disk content matches the current catalog template
                        exactly, but the manifest has no entry for it. This
                        is what the pre-ADR-2026-08-18 write order could
                        leave behind if interrupted: the bytes are trackfw's
                        own, only the manifest record is missing. Safe to
                        adopt.

  unknown-content       on-disk content matches neither the catalog template
                        NOR any manifest entry. This state is genuinely
                        ambiguous: it could be a file that is not trackfw's
                        at all occupying a catalog destination, or an
                        orphaned trackfw artifact whose bytes drifted once
                        the catalog moved on. It is exactly the state that
                        makes "agents install" refuse this destination with
                        "unmanaged artifact" — the remedy names that refusal
                        instead of picking a side: remove or move the file
                        if it is yours; if it is trackfw's and it drifted,
                        "install --force" replaces it with the current
                        template.

  hand-modified         the manifest owns the destination, but its on-disk
                        hash no longer matches what the manifest recorded —
                        the file was edited after trackfw wrote it. Adopting
                        overwrites that edit; it is a human decision, never
                        automatic.

A destination registered under a claim other than the one being inspected is
never reported, regardless of its content — that ambiguity belongs to a
different item, not this one, and is out of scope for doctor. Each finding
prints a ready-to-copy remediation command; doctor never writes anything
itself.

--remote adds a second, OPTIONAL modality (ADR-2026-09-02): it checks this
repository's GitHub branch protection (required_status_checks, enforce_admins)
and, locally, whether core.hooksPath has been neutralized (e.g. /dev/null).
Without --remote, doctor behaves exactly as it always has — offline, fast, no
credential required. With --remote and no usable credential (or a credential
without admin access to the repository, or a forge other than GitHub), the
result is "not-evaluated" — a third state, distinct from both a clean check
and a finding, because "could not verify" collapsed into "ok" is the single
most expensive lie this command could tell.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			findings, err := runDoctor(remote)
			if err != nil {
				return err
			}
			if jsonOut {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(findings)
			}
			printDoctorReport(cmd, findings)
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOut, "json", false, "Emit findings as a JSON array instead of the text report")
	cmd.Flags().BoolVar(&remote, "remote", false, "Also check GitHub branch protection (required_status_checks, enforce_admins) and local core.hooksPath neutralization. Requires network + a GitHub credential with admin access; absence of either is reported as not-evaluated, never as a pass (ADR-2026-09-02).")
	return cmd
}

func runDoctor(remote bool) ([]integrations.DoctorFinding, error) {
	catalog, err := integrations.LoadCatalog()
	if err != nil {
		return nil, err
	}
	manager, err := integrationsManager()
	if err != nil {
		return nil, err
	}
	ident, err := identity.Load(manager.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("doctor: identidade invalida: %w", err)
	}
	catalogFindings, err := integrations.RunDoctor(catalog, manager, ident, config.Load().AgentModels)
	if err != nil {
		return nil, err
	}
	// Scaffold coverage (ADR-2026-08-27): compare scaffold artifacts on disk against
	// the templates the current binary would generate, using the project's own
	// trackfw.yaml. No manifest entry is written or read (AC3).
	scaffoldFindings, err := generators.RunScaffoldDoctor(manager.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("doctor: scaffold scan: %w", err)
	}
	findings := append(catalogFindings, scaffoldFindings...)
	// Stray agent-memory scan (REQ-2026-11-01, #646): find .claude/agent-memory/ directories
	// outside the repository root — harness residue from dispatch in a subdirectory cwd.
	strayFindings, err := scanStrayAgentMemory(manager.ProjectRoot)
	if err != nil {
		return nil, fmt.Errorf("doctor: stray agent-memory scan: %w", err)
	}
	findings = append(findings, strayFindings...)
	// Remote modality (ADR-2026-09-02, ML-3A): opt-in only — never runs without --remote, so
	// doctor's offline default is unchanged.
	if remote {
		findings = append(findings, runDoctorRemote(defaultDoctorRemoteDeps(manager.ProjectRoot))...)
	}
	return findings, nil
}

// shellQuotePOSIX wraps s in POSIX single quotes, escaping any embedded single-quote
// character so the result is safe to embed verbatim in a shell command.
//
// POSIX single-quoting rules: a single-quoted string cannot contain a literal single
// quote, so each ' is terminated ('...' → '...'), the literal ' emitted as '\” (that
// is: close-quote, escaped-quote, reopen-quote), and quoting resumed.
//
// Examples:
//
//	plain    → 'plain'
//	my dir   → 'my dir'
//	it's     → 'it'\''s'
//	$()      → '$()'          (no shell expansion inside single quotes)
func shellQuotePOSIX(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// scanStrayAgentMemory walks the git repository rooted at start and returns one
// DoctorFinding per .claude/agent-memory/ directory found outside the root. The
// root .claude/agent-memory/ is legitimate and never reported.
//
// Rules:
//   - root is resolved via `git rev-parse --show-toplevel` (not os.Getwd), so the
//     scan is correct even when `trackfw doctor` is invoked from a subdirectory;
//   - if start is not inside a git repository, or git is unavailable, returns nil;
//   - .git/ and node_modules/ are skipped (SkipDir);
//   - a subdirectory that contains a .git entry (file or dir) is a nested repo and
//     is skipped — its .claude/agent-memory/ is legitimate for agents of that repo;
//   - symlinks and Windows junctions (ModeSymlink|ModeIrregular) are not followed;
//   - permission errors on directories skip that subtree, not the whole scan;
//   - file count inside the stray is recursive over non-directory entries (not ReadDir,
//     which would count subdirectory entries as "files" for otherwise-empty dirs);
//   - a permission error reading a subdirectory inside the stray is treated as
//     "contents not fully readable" — the finding does not suggest rm -rf.
//
// The remedy always uses the absolute path in POSIX single quotes (shellQuotePOSIX).
// The finding message distinguishes: empty (harness residue), unreadable (inspect before
// removing), and has files (lost agent memory). The doctor never removes anything.
func scanStrayAgentMemory(start string) ([]integrations.DoctorFinding, error) {
	// Resolve git root from start — independent of the process cwd.
	gitCmd := exec.Command("git", "rev-parse", "--show-toplevel")
	gitCmd.Dir = start
	topOut, err := gitCmd.Output()
	if err != nil {
		// Not a git repo or git unavailable — no findings, no error.
		return nil, nil
	}
	gitRoot := strings.TrimRight(string(topOut), "\r\n")
	// Resolve symlinks so filepath.EvalSymlinks comparisons are stable (macOS: /tmp → /private/tmp).
	if resolved, resolveErr := filepath.EvalSymlinks(gitRoot); resolveErr == nil {
		gitRoot = resolved
	}

	var findings []integrations.DoctorFinding

	_ = filepath.WalkDir(gitRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Permission error or unreadable entry — skip the subtree if it is a directory.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if path == gitRoot {
			return nil
		}

		// Never follow symlinks or Windows junctions (go.mod ≥ 1.23: junctions = ModeIrregular).
		if d.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		name := d.Name()

		// Skip well-known large/irrelevant directory trees.
		if name == ".git" || name == "node_modules" {
			return filepath.SkipDir
		}

		// Skip nested git repositories entirely — their .claude/agent-memory/ is
		// the legitimate root for agents dispatched in that repo, not a stray.
		// Check must happen HERE (before entering the directory's children) because
		// .claude/ is lexicographically before .git/, so the .claude subtree would
		// be visited before the .git entry is encountered as a child.
		if _, lstatErr := os.Lstat(filepath.Join(path, ".git")); lstatErr == nil {
			return filepath.SkipDir
		}

		// Detect .claude/agent-memory/ at any depth.
		if name == "agent-memory" && filepath.Base(filepath.Dir(path)) == ".claude" {
			grandparent := filepath.Dir(filepath.Dir(path))
			// Resolve symlinks for a stable comparison with gitRoot.
			if resolved, resolveErr := filepath.EvalSymlinks(grandparent); resolveErr == nil {
				grandparent = resolved
			}
			if grandparent == gitRoot {
				// This is the repository root's .claude/agent-memory/ — legitimate.
				// Continue walking inside it to find nested strays.
				return nil
			}

			// STRAY: count non-directory entries recursively (ReadDir would count
			// subdirectory entries, misreporting an otherwise-empty stray as "has files").
			// A permission error reading a subdirectory is tracked separately; that state
			// suppresses both empty- and files-branch remedies (the content is unknown).
			fileCount := 0
			unreadable := false
			_ = filepath.WalkDir(path, func(_ string, innerD fs.DirEntry, innerErr error) error {
				if innerErr != nil {
					unreadable = true
					return nil
				}
				if innerD.Type()&(os.ModeSymlink|os.ModeIrregular) != 0 {
					return nil
				}
				if !innerD.IsDir() {
					fileCount++
				}
				return nil
			})

			rel, relErr := filepath.Rel(gitRoot, path)
			if relErr != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)

			var remedy string
			if unreadable {
				remedy = "contents not fully readable (permission denied reading subdirectory); inspect directory before removing: " + shellQuotePOSIX(path)
			} else if fileCount == 0 {
				remedy = "empty directory (harness residue from dispatch in a subdirectory cwd); remove with: rm -rf " + shellQuotePOSIX(path)
			} else {
				remedy = fmt.Sprintf("%d file(s) of lost agent memory (never loaded — harness read from dispatch cwd path); archive to .claude/agent-memory/ before removing: rm -rf %s", fileCount, shellQuotePOSIX(path))
			}
			findings = append(findings, integrations.DoctorFinding{
				FindingKind: integrations.DoctorStrayAgentMemory,
				Destination: rel,
				Remedy:      remedy,
			})
			// Don't recurse further inside the stray — we already counted its contents.
			return filepath.SkipDir
		}

		return nil
	})

	return findings, nil
}

func printDoctorReport(cmd *cobra.Command, findings []integrations.DoctorFinding) {
	out := cmd.OutOrStdout()
	if len(findings) == 0 {
		fmt.Fprintln(out, "trackfw doctor: no mismatches found -- disk matches the manifest for every catalog-managed artifact and all scaffold templates.")
		return
	}
	// Counted explicitly by kind (never an if/else fallback into the last
	// bucket) so a new class introduced later fails loudly here instead
	// of silently inflating another kind's count.
	var unregistered, handModified, unknownContent, scaffoldDivergent, scaffoldMissing, scaffoldWrongMode int
	var requiredChecksMissing, enforceAdminsDisabled, hooksPathNeutralized, notEvaluated int
	var workflowDuplicated int
	var strayAgentMemory int
	for _, finding := range findings {
		switch finding.FindingKind {
		case integrations.DoctorUnregisteredWrite:
			unregistered++
		case integrations.DoctorHandModified:
			handModified++
		case integrations.DoctorUnknownContent:
			unknownContent++
		case integrations.DoctorScaffoldDivergent:
			scaffoldDivergent++
		case integrations.DoctorScaffoldMissing:
			scaffoldMissing++
		case integrations.DoctorScaffoldWrongMode:
			scaffoldWrongMode++
		case integrations.DoctorRequiredStatusChecksMissing:
			requiredChecksMissing++
		case integrations.DoctorEnforceAdminsDisabled:
			enforceAdminsDisabled++
		case integrations.DoctorHooksPathNeutralized:
			hooksPathNeutralized++
		case integrations.DoctorNotEvaluated:
			notEvaluated++
		case integrations.DoctorScaffoldWorkflowDuplicated:
			workflowDuplicated++
		case integrations.DoctorStrayAgentMemory:
			strayAgentMemory++
		}
	}
	fmt.Fprintf(out, "trackfw doctor: %d finding(s) -- %d unregistered-write, %d hand-modified, %d unknown-content, %d scaffold-divergent, %d scaffold-missing, %d scaffold-wrong-mode, %d required-status-checks-missing, %d enforce-admins-disabled, %d hooks-path-neutralized, %d not-evaluated, %d scaffold-workflow-duplicated, %d stray-agent-memory\n\n",
		len(findings), unregistered, handModified, unknownContent, scaffoldDivergent, scaffoldMissing, scaffoldWrongMode, requiredChecksMissing, enforceAdminsDisabled, hooksPathNeutralized, notEvaluated, workflowDuplicated, strayAgentMemory)
	// One blank line BETWEEN findings, none trailing after the last one — matches Node's
	// `lines.join('\n').replace(/\n$/, '')` and Python's `"\n".join(lines).rstrip("\n")`
	// (npm/src/commands/doctor.js, pypi/trackfw/commands/doctor.py). A naive per-finding
	// "\n\n" suffix leaves a trailing blank line only Go would emit — a real byte-level
	// divergence on the text surface caught by scripts/check-doctor-parity.sh (ML-2B).
	for i, finding := range findings {
		if i > 0 {
			fmt.Fprintln(out)
		}
		fmt.Fprintf(out, "[%s] %s\n  remedy: %s\n", finding.FindingKind, finding.Destination, finding.Remedy)
	}
}

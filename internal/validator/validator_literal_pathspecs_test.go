package validator

// TestMdBasenamesInGitTree_LiteralPathspecDirName asserts ML-3C conclusion:
// mdBasenamesInGitTreeWithError passes --literal-pathspecs so that magic tokens in
// roadmap_dir (e.g. ":(exclude)") are never interpreted by git and never silently
// suppress existing paths, which would cause D3 to miss a real done/ tree and accept
// a roadmap as "moved by this branch" without verifying the base.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMdBasenamesInGitTree_LiteralPathspecDirName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("NTFS reserva ':' em nome de arquivo — o diretório ':(exclude)rm' não pode existir, então o vetor RN1 não existe nesta plataforma")
	}
	cases := []struct {
		dirPrefix string
	}{
		{":(exclude)rm/done"},
		{":(icase)rm/done"},
	}

	const roadmapFile = "ROADMAP-2026-10-01-x.md"

	for _, tc := range cases {
		tc := tc
		t.Run(tc.dirPrefix, func(t *testing.T) {
			run := func(dir string, args ...string) {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = dir
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v in %s: %s", args, dir, out)
				}
			}

			bareDir := t.TempDir()
			run(bareDir, "init", "--bare", bareDir)

			seed := t.TempDir()
			run(seed, "init", "-b", "main")
			run(seed, "config", "user.email", "test@test.com")
			run(seed, "config", "user.name", "test")

			// Directory name contains pathspec magic characters (literal on disk).
			full := filepath.Join(seed, filepath.FromSlash(tc.dirPrefix))
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatalf("MkdirAll %s: %v", full, err)
			}
			if err := os.WriteFile(filepath.Join(full, roadmapFile), []byte("# test\n"), 0o644); err != nil {
				t.Fatalf("WriteFile in %s: %v", tc.dirPrefix, err)
			}

			run(seed, "add", ".")
			run(seed, "commit", "--allow-empty", "-m", "init")
			run(seed, "remote", "add", "origin", bareDir)
			run(seed, "push", "origin", "HEAD:main")

			workDir := t.TempDir()
			run(workDir, "init", "-b", "main")
			run(workDir, "config", "user.email", "test@test.com")
			run(workDir, "config", "user.name", "test")
			run(workDir, "remote", "add", "origin", bareDir)
			run(workDir, "fetch", "origin")

			chdir(t, workDir)

			got, err := mdBasenamesInGitTreeWithError("origin/main", tc.dirPrefix)
			if err != nil {
				t.Fatalf("mdBasenamesInGitTreeWithError(%q, %q) returned error: %v", "origin/main", tc.dirPrefix, err)
			}
			if !got[roadmapFile] {
				t.Errorf("mdBasenamesInGitTreeWithError(%q, %q) = %v; want set containing %q",
					"origin/main", tc.dirPrefix, got, roadmapFile)
			}
		})
	}
}

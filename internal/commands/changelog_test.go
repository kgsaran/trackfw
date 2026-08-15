package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const changelogFixture = `# Changelog

Todas as mudanças notáveis deste projeto serão documentadas neste arquivo.

## [Unreleased]

### Added
- feature em desenvolvimento

## [6.10.0] - 2026-08-14

### Added
- comando trackfw changelog
`

// chdirToProjectWithChangelog isola cwd em um diretório de fixture (t.TempDir())
// contendo um CHANGELOG.md, restaurando o cwd original ao fim do teste. Mesmo
// padrão de internal/commands/adr_test.go (setHomeAndCwd).
func chdirToProjectWithChangelog(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()

	origCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if content != "" {
		if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(content), 0o644); err != nil {
			t.Fatalf("setup CHANGELOG.md: %v", err)
		}
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(origCwd)
	})
	return dir
}

// runChangelogCmd executa "trackfw changelog <args>" e retorna o stdout
// capturado. O comando usa fmt.Print (não cmd.OutOrStdout()) — mesmo padrão
// de internal/commands/status.go — por isso a captura é feita substituindo
// os.Stdout, não via root.SetOut.
func runChangelogCmd(t *testing.T, args []string) (string, error) {
	t.Helper()

	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs(append([]string{"changelog"}, args...))
	runErr := root.Execute()

	_ = w.Close()
	os.Stdout = origStdout

	out, readErr := io.ReadAll(r)
	if readErr != nil {
		t.Fatalf("io.ReadAll: %v", readErr)
	}
	return string(out), runErr
}

func TestChangelogCmdNoFlagsPrintsFirstSection(t *testing.T) {
	chdirToProjectWithChangelog(t, changelogFixture)

	out, err := runChangelogCmd(t, nil)
	if err != nil {
		t.Fatalf("trackfw changelog: erro inesperado: %v", err)
	}
	want := "## [Unreleased]\n\n### Added\n- feature em desenvolvimento\n"
	if out != want {
		t.Errorf("trackfw changelog: saída = %q, esperava %q", out, want)
	}
}

func TestChangelogCmdVersionExisting(t *testing.T) {
	chdirToProjectWithChangelog(t, changelogFixture)

	out, err := runChangelogCmd(t, []string{"--version", "6.10.0"})
	if err != nil {
		t.Fatalf("trackfw changelog --version 6.10.0: erro inesperado: %v", err)
	}
	want := "## [6.10.0] - 2026-08-14\n\n### Added\n- comando trackfw changelog\n"
	if out != want {
		t.Errorf("trackfw changelog --version 6.10.0: saída = %q, esperava %q", out, want)
	}
}

func TestChangelogCmdVersionMissing(t *testing.T) {
	chdirToProjectWithChangelog(t, changelogFixture)

	_, err := runChangelogCmd(t, []string{"--version", "999.0.0"})
	if err == nil {
		t.Fatal("trackfw changelog --version 999.0.0: esperava erro, obteve nil")
	}
	want := `version "999.0.0" not found in CHANGELOG.md`
	if err.Error() != want {
		t.Errorf("trackfw changelog --version 999.0.0: mensagem de erro = %q, esperava %q", err.Error(), want)
	}
}

func TestChangelogCmdAllPrintsEntireFile(t *testing.T) {
	chdirToProjectWithChangelog(t, changelogFixture)

	out, err := runChangelogCmd(t, []string{"--all"})
	if err != nil {
		t.Fatalf("trackfw changelog --all: erro inesperado: %v", err)
	}
	if out != changelogFixture {
		t.Errorf("trackfw changelog --all: saída divergente do fixture")
	}
}

func TestChangelogCmdMissingFile(t *testing.T) {
	chdirToProjectWithChangelog(t, "")

	_, err := runChangelogCmd(t, nil)
	if err == nil {
		t.Fatal("trackfw changelog: esperava erro, obteve nil")
	}
	want := "CHANGELOG.md not found — nothing to show"
	if err.Error() != want {
		t.Errorf("trackfw changelog: mensagem de erro = %q, esperava %q", err.Error(), want)
	}
}

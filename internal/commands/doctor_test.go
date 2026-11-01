package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/kgsaran/trackfw/internal/integrations"
)

// TestDoctorReportsNoFindingsOnEmptyProject proves the command's silent
// path: a project with nothing installed must report zero findings, never a
// false positive from the sweep itself.
func TestDoctorReportsNoFindingsOnEmptyProject(t *testing.T) {
	integrationCommandFixture(t)

	cmd := newDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	if !strings.Contains(out.String(), "no mismatches found") {
		t.Fatalf("expected no-findings message, got: %s", out.String())
	}
}

// TestDoctorDistinguishesUnregisteredWriteFromHandModified reproduces both
// classes end-to-end through the real command surface (not the
// ClassifyDoctor unit) and asserts the report names the remedy for each.
func TestDoctorDistinguishesUnregisteredWriteFromHandModified(t *testing.T) {
	project, _ := integrationCommandFixture(t)

	installCmd := newAgentsCmd()
	installCmd.SetArgs([]string{"install", "--items", "backend", "--targets", "claude", "--scope", "project"})
	if err := installCmd.Execute(); err != nil {
		t.Fatalf("agents install failed: %v", err)
	}

	// Case 1: unregistered write — simulate the pre-ADR-2026-08-18 crash
	// window by removing this destination's manifest entry, leaving on-disk
	// bytes that still match the current catalog render untouched. The
	// destination is read back from the manifest itself (not reconstructed
	// from the catalog) to avoid a spurious mismatch from /var vs
	// /private/var symlink resolution on macOS.
	manifestFile := integrations.ManifestPath(project)
	raw, err := os.ReadFile(manifestFile)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	artifacts, _ := manifest["artifacts"].(map[string]interface{})
	if len(artifacts) != 1 {
		t.Fatalf("expected exactly one manifest artifact, manifest: %s", raw)
	}
	var destination string
	for key := range artifacts {
		destination = key
	}
	delete(artifacts, destination)
	tampered, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestFile, tampered, 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	report := out.String()
	if !strings.Contains(report, "["+string(integrations.DoctorUnregisteredWrite)+"]") {
		t.Fatalf("expected unregistered-write finding, got: %s", report)
	}
	if !strings.Contains(report, "agents install --force") {
		t.Fatalf("expected remedy naming agents install --force, got: %s", report)
	}
	if strings.Contains(report, "["+string(integrations.DoctorHandModified)+"]") {
		t.Fatalf("must not report hand-modified for this case: %s", report)
	}

	// Restore normal management, then simulate a hand edit after install —
	// distinct class, distinct remedy wording (must still be install
	// --force, but the two finding kinds must not collapse).
	installCmd = newAgentsCmd()
	installCmd.SetArgs([]string{"install", "--items", "backend", "--targets", "claude", "--scope", "project", "--force"})
	if err := installCmd.Execute(); err != nil {
		t.Fatalf("agents install --force (re-adopt) failed: %v", err)
	}
	if err := os.WriteFile(destination, []byte("edited by a human"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd = newDoctorCmd()
	out.Reset()
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	report = out.String()
	if !strings.Contains(report, "["+string(integrations.DoctorHandModified)+"]") {
		t.Fatalf("expected hand-modified finding, got: %s", report)
	}
	if strings.Contains(report, "["+string(integrations.DoctorUnregisteredWrite)+"]") {
		t.Fatalf("must not report unregistered-write for this case: %s", report)
	}
}

// TestDoctorReportsUnknownContentForNeverInstalledDestination reproduces
// end-to-end (real command surface, not the ClassifyDoctor unit) the state
// ML-3A's audit found silently unreported: a destination the catalog would
// use for this claim, carrying content that was never installed by trackfw
// (no manifest entry) and that does not match the catalog template either.
// The remedy must name the preflight refusal literally.
func TestDoctorReportsUnknownContentForNeverInstalledDestination(t *testing.T) {
	project, _ := integrationCommandFixture(t)

	// Learn the exact destination `agents install` would use, read-only,
	// without ever installing — mirrors scripts/check-doctor-parity.sh's
	// scenario (d)/(f) fixture construction.
	listCmd := newAgentsCmd()
	listCmd.SetArgs([]string{"list", "--items", "backend", "--targets", "claude", "--scope", "project", "--json"})
	var listOut bytes.Buffer
	listCmd.SetOut(&listOut)
	if err := listCmd.Execute(); err != nil {
		t.Fatalf("agents list failed: %v", err)
	}
	var listPayload struct {
		Deployments []struct {
			Destination string `json:"destination"`
		} `json:"deployments"`
	}
	if err := json.Unmarshal(listOut.Bytes(), &listPayload); err != nil {
		t.Fatalf("agents list --json did not decode: %v\n%s", err, listOut.String())
	}
	if len(listPayload.Deployments) != 1 {
		t.Fatalf("expected exactly one deployment row, got %d: %+v", len(listPayload.Deployments), listPayload.Deployments)
	}
	destination := listPayload.Deployments[0].Destination
	if !strings.HasPrefix(destination, "/") {
		destination = project + "/" + destination
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("nobody installed this through trackfw"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := newDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor failed: %v", err)
	}
	report := out.String()
	if !strings.Contains(report, "["+string(integrations.DoctorUnknownContent)+"]") {
		t.Fatalf("expected unknown-content finding, got: %s", report)
	}
	if !strings.Contains(report, "unmanaged artifact") {
		t.Fatalf("expected remedy to name the preflight refusal literally, got: %s", report)
	}
	if strings.Contains(report, "["+string(integrations.DoctorUnregisteredWrite)+"]") {
		t.Fatalf("must not report unregistered-write for this case: %s", report)
	}
	if strings.Contains(report, "["+string(integrations.DoctorHandModified)+"]") {
		t.Fatalf("must not report hand-modified for this case: %s", report)
	}
}

// TestDoctorJSONOutputIsValidAndFindingLess proves --json emits a decodable
// array with the same zero-findings shape as the text report.
func TestDoctorJSONOutputIsValidAndFindingLess(t *testing.T) {
	integrationCommandFixture(t)

	cmd := newDoctorCmd()
	cmd.SetArgs([]string{"--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor --json failed: %v", err)
	}
	var findings []integrations.DoctorFinding
	if err := json.Unmarshal(out.Bytes(), &findings); err != nil {
		t.Fatalf("doctor --json did not emit a decodable array: %v\n%s", err, out.String())
	}
	if len(findings) != 0 {
		t.Fatalf("findings = %d, want 0: %+v", len(findings), findings)
	}
}

// gitInitForDoctor initialises a git repo in dir with a minimal isolated config.
// Returns a git run helper bound to that config.
func gitInitForDoctor(t *testing.T, dir string) func(...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	gitcfg := filepath.Join(dir, ".test-gitconfig")
	_ = os.WriteFile(gitcfg, []byte("[user]\n\temail = test@trackfw\n\tname = trackfw test\n[commit]\n\tgpgsign = false\n"), 0600)
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+gitcfg,
			"GIT_CONFIG_SYSTEM=/dev/null",
			"GIT_TERMINAL_PROMPT=0",
			"HOME="+dir,
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	return run
}

// isSymlinkPrivilegeError reports true when err is the "process lacks privilege to create
// a symlink" failure — WinError 1314 on Windows without Developer Mode, or generic
// permission-denied on any platform. Matches by error, not by GOOS.
// Pattern is identical to the symlink_helper_test.go copies in discover/ and generators/.
func isSymlinkPrivilegeError(err error) bool {
	if os.IsPermission(err) {
		return true
	}
	var e syscall.Errno
	return errors.As(err, &e) && e == 1314
}

// TestScanStrayAgentMemory_SubpastaComMemoriaVazia asserts that a .claude/agent-memory/
// in a subdirectory with no files is reported as 1 finding with 0 files (harness residue).
//
// Reconciliation (ML-1C, AC4): afirma que um diretório stray vazio produz 1 achado, com
// a mensagem nomeando "harness residue" e sem arquivos — contra a contagem com ReadDir que
// reportaria 1 entrada (o subdiretório) como "memória perdida".
func TestScanStrayAgentMemory_SubpastaComMemoriaVazia(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	gitInitForDoctor(t, project)

	// Create stray: sub/.claude/agent-memory/zeus-tf/ (empty subdir, 0 files)
	strayDir := filepath.Join(project, "sub", ".claude", "agent-memory", "zeus-tf")
	if err := os.MkdirAll(strayDir, 0755); err != nil {
		t.Fatal(err)
	}

	findings, err := scanStrayAgentMemory(project)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.FindingKind != integrations.DoctorStrayAgentMemory {
		t.Errorf("FindingKind = %q, want stray-agent-memory", f.FindingKind)
	}
	if !strings.Contains(f.Remedy, "harness residue") {
		t.Errorf("expected 'harness residue' in remedy for empty stray, got: %q", f.Remedy)
	}
	if strings.Contains(f.Remedy, "1 file") {
		t.Errorf("empty stray must not report file count > 0, got: %q", f.Remedy)
	}
}

// TestScanStrayAgentMemory_SubpastaComDoisArquivos asserts that a stray with 2 files
// reports the correct file count in the finding message.
//
// Reconciliation (ML-1C, AC4): afirma que 2 arquivos no stray são contados corretamente
// e o achado diferencia memória perdida de resíduo vazio.
func TestScanStrayAgentMemory_SubpastaComDoisArquivos(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	gitInitForDoctor(t, project)

	strayDir := filepath.Join(project, "sub", ".claude", "agent-memory", "zeus-tf")
	if err := os.MkdirAll(strayDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"user_role.md", "feedback.md"} {
		if err := os.WriteFile(filepath.Join(strayDir, name), []byte("memory"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	findings, err := scanStrayAgentMemory(project)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Remedy, "2 file") {
		t.Errorf("expected '2 file' in remedy, got: %q", findings[0].Remedy)
	}
}

// TestScanStrayAgentMemory_ApenasRaizNenhumAchado asserts that a project with only
// the root .claude/agent-memory/ produces no stray-agent-memory findings.
//
// Reconciliation (ML-1C, AC4): afirma que a memória legítima da raiz nunca é reportada
// como stray — é o contra-braço do teste de subpasta.
func TestScanStrayAgentMemory_ApenasRaizNenhumAchado(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	gitInitForDoctor(t, project)

	// Create only the legitimate root memory.
	rootDir := filepath.Join(project, ".claude", "agent-memory", "zeus-tf")
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(rootDir, "user_role.md"), []byte("memory"), 0644)

	findings, err := scanStrayAgentMemory(project)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for root-only memory, got %d: %+v", len(findings), findings)
	}
}

// TestScanStrayAgentMemory_RepositorioAninhado asserts that a .claude/agent-memory/ inside
// a nested git repository is NOT reported as stray (it is legitimate for that inner repo).
//
// Reconciliation (ML-1C, AC4): afirma que o filtro de repositório aninhado funciona —
// contra-braço: sem o filtro, o teste falha (achado indevido para repo aninhado).
func TestScanStrayAgentMemory_RepositorioAninhado(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	gitInitForDoctor(t, project)

	nestedDir := filepath.Join(project, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatal(err)
	}

	t.Run("GitDirNested", func(t *testing.T) {
		// Sub-test A: .git/ directory (plain nested repo from `git init`)
		if _, err := exec.LookPath("git"); err != nil {
			t.Skip("git not available")
		}
		gitcfg := filepath.Join(project, ".test-gitconfig")
		_ = os.WriteFile(gitcfg, []byte("[user]\n\temail = test@trackfw\n\tname = trackfw test\n[commit]\n\tgpgsign = false\n"), 0600)
		cmd := exec.Command("git", "init", "-q", nestedDir)
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+gitcfg,
			"GIT_CONFIG_SYSTEM=/dev/null",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init nested: %v\n%s", err, out)
		}
		// Place memory inside the nested repo.
		memDir := filepath.Join(nestedDir, ".claude", "agent-memory", "agent-x")
		if err := os.MkdirAll(memDir, 0755); err != nil {
			t.Fatal(err)
		}
		findings, err := scanStrayAgentMemory(project)
		if err != nil {
			t.Fatalf("scan error: %v", err)
		}
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for nested-repo memory, got %d: %+v", len(findings), findings)
		}
	})

	t.Run("GitFileSubmodule", func(t *testing.T) {
		// Sub-test B: .git as a regular file (submodule pointer).
		submodDir := filepath.Join(project, "submod")
		if err := os.MkdirAll(submodDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Write a .git file (submodule format).
		if err := os.WriteFile(filepath.Join(submodDir, ".git"), []byte("gitdir: ../.git/modules/submod\n"), 0644); err != nil {
			t.Fatal(err)
		}
		memDir := filepath.Join(submodDir, ".claude", "agent-memory", "agent-x")
		if err := os.MkdirAll(memDir, 0755); err != nil {
			t.Fatal(err)
		}
		findings, err := scanStrayAgentMemory(project)
		if err != nil {
			t.Fatalf("scan error: %v", err)
		}
		for _, f := range findings {
			if strings.HasPrefix(f.Destination, "submod/") {
				t.Errorf("submodule memory reported as stray: %+v", f)
			}
		}
	})
}

// TestScanStrayAgentMemory_NodeModulesNaoReportado asserts that .claude/agent-memory/
// inside node_modules/ is NOT reported (node_modules is skipped).
//
// Reconciliation (ML-1C, AC4): afirma que node_modules/ é ignorado pela varredura —
// evita relatórios sobre memória em dependências instaladas.
func TestScanStrayAgentMemory_NodeModulesNaoReportado(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	gitInitForDoctor(t, project)

	nmDir := filepath.Join(project, "node_modules", "x", ".claude", "agent-memory")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}

	findings, err := scanStrayAgentMemory(project)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for node_modules stray, got %d: %+v", len(findings), findings)
	}
}

// TestScanStrayAgentMemory_SymlinkNaoSeguido asserts that a symlink pointing to a
// directory containing .claude/agent-memory/ outside the root is not followed.
//
// Reconciliation (ML-1C, AC4): afirma que filepath.WalkDir nunca segue symlinks de
// diretório por construção da stdlib (DirEntry.Type() retorna ModeSymlink e IsDir()
// retorna false para a entrada do link; o check !d.IsDir() na linha seguinte já retorna nil).
// O guard explícito ModeSymlink|ModeIrregular em scanStrayAgentMemory (outer walk) é
// redundante para o comportamento deste teste — medido: o teste permanece verde com o
// guard removido. Residual não testado: o inner-walk guard (linha ~224) protege
// a contagem de arquivos dentro de um stray de não contar arquivos-link; não há teste
// que o exercite diretamente.
func TestScanStrayAgentMemory_SymlinkNaoSeguido(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	gitInitForDoctor(t, project)

	// External directory with agent-memory.
	external := t.TempDir()
	extMem := filepath.Join(external, ".claude", "agent-memory")
	if err := os.MkdirAll(extMem, 0755); err != nil {
		t.Fatal(err)
	}

	linkPath := filepath.Join(project, "linked")
	if err := os.Symlink(external, linkPath); err != nil {
		if isSymlinkPrivilegeError(err) {
			t.Skipf("symlink guard not exercised: symlink creation requires Developer Mode or elevated process: %v", err)
		}
		t.Fatalf("symlink: %v", err)
	}

	findings, err := scanStrayAgentMemory(project)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for symlinked external dir, got %d: %+v", len(findings), findings)
	}
}

// TestDoctorStrayAgentMemory_ViaCommand verifies the full doctor command path: exit code
// unchanged (returns nil), and the summary line includes the stray-agent-memory count.
//
// Reconciliation (ML-1C, AC4): afirma que o exit code do doctor não muda com achados de
// stray (RunE retorna nil), e que o relatório contém "1 stray-agent-memory".
func TestDoctorStrayAgentMemory_ViaCommand(t *testing.T) {
	project, _ := integrationCommandFixture(t)
	gitInitForDoctor(t, project)

	// Create one stray (empty).
	strayDir := filepath.Join(project, "sub", ".claude", "agent-memory", "zeus-tf")
	if err := os.MkdirAll(strayDir, 0755); err != nil {
		t.Fatal(err)
	}

	cmd := newDoctorCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	// Must return nil (exit code unchanged).
	if err := cmd.Execute(); err != nil {
		t.Fatalf("doctor returned error: %v", err)
	}
	report := out.String()
	if !strings.Contains(report, "1 stray-agent-memory") {
		t.Errorf("expected '1 stray-agent-memory' in summary, got:\n%s", report)
	}
}

// TestShellQuotePOSIX verifies the POSIX single-quote escaping function.
//
// Reconciliation (ML-3A, AC7 / A-2): afirma que shellQuotePOSIX produz saída correta
// para os casos de interesse: caminho vazio, caminho sem metacaracteres, espaço,
// aspa simples embutida, e expansão de subshell — todos devem ser opacos ao shell.
func TestShellQuotePOSIX(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "''"},
		{"plain", "'plain'"},
		{"my dir", "'my dir'"},
		{"it's", "'it'\\''s'"},
		{"a$(touch PWNED)", "'a$(touch PWNED)'"},
	}
	for _, c := range cases {
		got := shellQuotePOSIX(c.in)
		if got != c.want {
			t.Errorf("shellQuotePOSIX(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestScanStrayAgentMemory_PathsWithSpaceAndQuote verifies that the remedy produced for
// a stray inside a directory with a space or single-quote in the name can be executed
// safely via sh -c: it removes exactly the target and leaves a sibling called "my" intact.
//
// Reconciliation (ML-3A, AC7 / A-2): afirma que shellQuotePOSIX é aplicado ao caminho
// absoluto no remédio — sem as aspas, rm -rf interpreta o espaço como separador de tokens
// e remove o irmão "my" (contra-braço: remover a chamada a shellQuotePOSIX faz o teste
// do irmão reprovar).
func TestScanStrayAgentMemory_PathsWithSpaceAndQuote(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh -c not available on Windows")
	}

	project, _ := integrationCommandFixture(t)
	gitInitForDoctor(t, project)

	// Stray 1: "my dir/.claude/agent-memory/zeus-tf/" (empty — 0 files).
	myDirStray := filepath.Join(project, "my dir", ".claude", "agent-memory", "zeus-tf")
	if err := os.MkdirAll(myDirStray, 0755); err != nil {
		t.Fatal(err)
	}

	// Sibling "my/" must survive: without quoting, rm -rf tokenises "my dir" as
	// two tokens and deletes "my" first.
	siblingMy := filepath.Join(project, "my")
	if err := os.MkdirAll(siblingMy, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siblingMy, "sentinel.txt"), []byte("sentinel"), 0644); err != nil {
		t.Fatal(err)
	}

	// Stray 2: "don't/.claude/agent-memory/zeus-tf/" with one file.
	quoteStray := filepath.Join(project, "don't", ".claude", "agent-memory", "zeus-tf")
	if err := os.MkdirAll(quoteStray, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(quoteStray, "mem.md"), []byte("memory"), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := scanStrayAgentMemory(project)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(findings), findings)
	}

	// Run each remedy via sh -c with cwd = separate TempDir (not project) to prove the
	// path in the command is absolute, not relative.
	shDir := t.TempDir()
	for _, f := range findings {
		idx := strings.Index(f.Remedy, "rm -rf ")
		if idx < 0 {
			t.Fatalf("no 'rm -rf' in remedy: %q", f.Remedy)
		}
		rmCmd := f.Remedy[idx:]
		cmd := exec.Command("sh", "-c", rmCmd)
		cmd.Dir = shDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("sh -c %q: %v\n%s", rmCmd, err, out)
		}
	}

	// "my dir" stray must be gone.
	myDirAgentMem := filepath.Join(project, "my dir", ".claude", "agent-memory")
	if _, err := os.Stat(myDirAgentMem); !os.IsNotExist(err) {
		t.Errorf("'my dir' stray should have been removed, stat: %v", err)
	}

	// Sibling "my" must still exist.
	if _, err := os.Stat(filepath.Join(siblingMy, "sentinel.txt")); err != nil {
		t.Errorf("sibling 'my/' should be preserved after quoted rm, got: %v", err)
	}

	// "don't" stray must be gone.
	quoteAgentMem := filepath.Join(project, "don't", ".claude", "agent-memory")
	if _, err := os.Stat(quoteAgentMem); !os.IsNotExist(err) {
		t.Errorf("\"don't\" stray should have been removed, stat: %v", err)
	}
}

// TestScanStrayAgentMemory_UnreadableSubdir verifies that a stray whose subdirectory
// has permission 000 (making its contents unreadable) is not reported as "empty" and
// does not suggest rm -rf.
//
// Reconciliation (ML-3A, AC7 / A-3): afirma que innerErr != nil na inner WalkDir
// marca o achado como "contents not fully readable" — sem "empty" na mensagem e sem
// sugestão de rm -rf — contra o comportamento anterior que ignorava o erro e declarava
// 0 arquivos ("empty directory … remove with: rm -rf").
func TestScanStrayAgentMemory_UnreadableSubdir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 000 not effective on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("chmod 000 has no effect when running as root")
	}

	project, _ := integrationCommandFixture(t)
	gitInitForDoctor(t, project)

	// Create stray: sub/.claude/agent-memory/zeus-tf/
	strayDir := filepath.Join(project, "sub", ".claude", "agent-memory", "zeus-tf")
	if err := os.MkdirAll(strayDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create hidden/ subdir with chmod 000, containing secret.md.
	hiddenDir := filepath.Join(strayDir, "hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "secret.md"), []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(hiddenDir, 0); err != nil {
		t.Fatal(err)
	}
	// Restore permissions in Cleanup so TempDir RemoveAll can clean up.
	t.Cleanup(func() {
		_ = os.Chmod(hiddenDir, 0755)
	})

	findings, err := scanStrayAgentMemory(project)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}

	remedy := findings[0].Remedy
	if strings.Contains(strings.ToLower(remedy), "empty") {
		t.Errorf("remedy must not say 'empty' for unreadable subdir: %q", remedy)
	}
	if strings.Contains(remedy, "rm -rf") {
		t.Errorf("remedy must not suggest 'rm -rf' for unreadable subdir: %q", remedy)
	}
	if !strings.Contains(strings.ToLower(remedy), "inspect") {
		t.Errorf("remedy must say 'inspect' for unreadable subdir: %q", remedy)
	}
}

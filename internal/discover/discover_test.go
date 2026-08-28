package discover

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/integrations"
	"github.com/kgsaran/trackfw/internal/version"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("TRACKFW_DISABLE_EXTERNAL_COMMANDS", "1")
	os.Exit(m.Run())
}

func TestScan_Empty(t *testing.T) {
	dir := t.TempDir()
	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.GovernanceScore != 0 {
		t.Errorf("expected score 0 for empty dir, got %d", r.GovernanceScore)
	}
	if r.ADRCount != 0 || r.REQCount != 0 || r.RoadmapCount != 0 {
		t.Error("expected all counts 0 for empty dir")
	}
}

func TestScan_Flat(t *testing.T) {
	dir := t.TempDir()
	// cria estrutura flat
	mustMkdir(t, dir, "docs/adr")
	mustMkdir(t, dir, "docs/req")
	mustMkdir(t, dir, "docs/roadmaps/wip")
	mustMkdir(t, dir, "docs/roadmaps/done")
	mustWriteFile(t, filepath.Join(dir, "docs/adr/ADR-001.md"), "# ADR")
	mustWriteFile(t, filepath.Join(dir, "docs/req/REQ-001.md"), "# REQ")
	mustWriteFile(t, filepath.Join(dir, "docs/roadmaps/done/ROADMAP-001.md"), "# R")
	// hook e CI
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "")
	mustMkdir(t, dir, ".github/workflows")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.RoadmapNamespacing != "flat" {
		t.Errorf("expected flat, got %s", r.RoadmapNamespacing)
	}
	if r.REQDir != "docs/req" {
		t.Errorf("expected docs/req, got %s", r.REQDir)
	}
	if r.ADRCount != 1 {
		t.Errorf("expected 1 ADR, got %d", r.ADRCount)
	}
	if r.RoadmapCount != 1 {
		t.Errorf("expected 1 roadmap, got %d", r.RoadmapCount)
	}
	if r.HookFramework != "lefthook" {
		t.Errorf("expected lefthook, got %s", r.HookFramework)
	}
	if r.CISystem != "github-actions" {
		t.Errorf("expected github-actions, got %s", r.CISystem)
	}
}

func TestScan_ByAgent(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, dir, "docs/adr/zeus")
	mustMkdir(t, dir, "docs/requisições")
	mustMkdir(t, dir, "docs/roadmaps/zeus/wip")
	mustMkdir(t, dir, "docs/roadmaps/apolo/done")
	mustWriteFile(t, filepath.Join(dir, "docs/adr/zeus/ADR-001.md"), "# ADR")
	mustWriteFile(t, filepath.Join(dir, "docs/requisições/REQ-001.md"), "# REQ")
	mustWriteFile(t, filepath.Join(dir, "docs/roadmaps/zeus/wip/ROADMAP-001.md"), "# R")
	mustWriteFile(t, filepath.Join(dir, "docs/roadmaps/apolo/done/ROADMAP-002.md"), "# R")
	// hook e CI
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "")
	mustMkdir(t, dir, ".github/workflows")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.RoadmapNamespacing != "by_agent" {
		t.Errorf("expected by_agent, got %s", r.RoadmapNamespacing)
	}
	if r.REQDir != "docs/requisições" {
		t.Errorf("expected docs/requisições, got %s", r.REQDir)
	}
	if len(r.Agents) != 2 {
		t.Errorf("expected 2 agents, got %v", r.Agents)
	}
	if r.RoadmapCount != 2 {
		t.Errorf("expected 2 roadmaps, got %d", r.RoadmapCount)
	}
	if r.HookFramework != "lefthook" {
		t.Errorf("expected lefthook, got %s", r.HookFramework)
	}
	if r.CISystem != "github-actions" {
		t.Errorf("expected github-actions, got %s", r.CISystem)
	}
}

func TestScan_CMDBLike(t *testing.T) {
	dir := t.TempDir()
	// simula a estrutura CMDB com 6 agentes
	for _, agent := range []string{"zeus", "apolo", "afrodite", "artemis", "ares", "atena"} {
		for _, state := range []string{"wip", "done"} {
			mustMkdir(t, dir, "docs/roadmaps/"+agent+"/"+state)
		}
		mustMkdir(t, dir, "docs/adr/"+agent)
	}
	mustMkdir(t, dir, "docs/requisições")
	// hook e CI
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "")
	mustMkdir(t, dir, ".github/workflows")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.RoadmapNamespacing != "by_agent" {
		t.Errorf("expected by_agent")
	}
	if len(r.Agents) != 6 {
		t.Errorf("expected 6 agents, got %d: %v", len(r.Agents), r.Agents)
	}
	if r.REQDir != "docs/requisições" {
		t.Errorf("expected docs/requisições, got %s", r.REQDir)
	}
	if len(r.ADRDirs) != 6 {
		t.Errorf("expected 6 ADR dirs, got %d", len(r.ADRDirs))
	}
	if r.HookFramework != "lefthook" {
		t.Errorf("expected lefthook, got %s", r.HookFramework)
	}
	if r.CISystem != "github-actions" {
		t.Errorf("expected github-actions, got %s", r.CISystem)
	}
}

func TestScan_HookAndCI(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "")
	mustMkdir(t, dir, ".github/workflows")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.HookFramework != "lefthook" {
		t.Errorf("expected lefthook, got %s", r.HookFramework)
	}
	if r.CISystem != "github-actions" {
		t.Errorf("expected github-actions, got %s", r.CISystem)
	}
}

// TestGenerateYAML_ForgePresent verifies that GenerateYAML emits "forge: github"
// when the DiscoveryResult has Forge set to a valid value.
func TestGenerateYAML_ForgePresent(t *testing.T) {
	r := DiscoveryResult{
		ADRDirs:            []string{"docs/adr"},
		REQDir:             "docs/req",
		RoadmapDir:         "docs/roadmaps",
		RoadmapNamespacing: "flat",
		HookFramework:      "none",
		CISystem:           "none",
		Forge:              "github",
	}
	yaml := GenerateYAML(r)
	if !containsSubstr(yaml, "forge: github") {
		t.Errorf("expected 'forge: github' in YAML, got:\n%s", yaml)
	}
	// forge must appear after ci:
	ciIdx := findSubstr(yaml, "ci:")
	forgeIdx := findSubstr(yaml, "forge:")
	if ciIdx < 0 || forgeIdx < 0 || forgeIdx <= ciIdx {
		t.Errorf("forge: must appear after ci: in YAML (ciIdx=%d, forgeIdx=%d)", ciIdx, forgeIdx)
	}
}

// TestGenerateYAML_ForgeAbsent verifies that GenerateYAML omits the "forge:" key
// entirely when Forge is empty (not detected).
func TestGenerateYAML_ForgeAbsent(t *testing.T) {
	r := DiscoveryResult{
		ADRDirs:            []string{"docs/adr"},
		REQDir:             "docs/req",
		RoadmapDir:         "docs/roadmaps",
		RoadmapNamespacing: "flat",
		HookFramework:      "none",
		CISystem:           "none",
		Forge:              "", // not detected
	}
	yaml := GenerateYAML(r)
	if containsSubstr(yaml, "forge:") {
		t.Errorf("expected 'forge:' key to be absent in YAML when Forge is empty, got:\n%s", yaml)
	}
}

// TestScan_ForgeFromCI verifies that Scan detects the forge via .gitlab-ci.yml when
// there is no git remote (temp dir is not a git repo so remote returns empty).
func TestScan_ForgeFromCI(t *testing.T) {
	dir := t.TempDir()
	// Write .gitlab-ci.yml but no .github/workflows — CI detection picks gitlab.
	mustWriteFile(t, filepath.Join(dir, ".gitlab-ci.yml"), "stages:\n  - test\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Forge != "gitlab" {
		t.Errorf("expected Forge='gitlab' from .gitlab-ci.yml, got %q", r.Forge)
	}
	// The generated YAML must include the forge key.
	yaml := GenerateYAML(r)
	if !containsSubstr(yaml, "forge: gitlab") {
		t.Errorf("expected 'forge: gitlab' in generated YAML, got:\n%s", yaml)
	}
}

// TestScan_NoForge verifies that Scan leaves Forge empty when no CI files exist
// and the directory is not a git repository (no remote URL available).
func TestScan_NoForge(t *testing.T) {
	dir := t.TempDir()
	// Clean temp dir: no .gitlab-ci.yml, no .github/workflows, no git repo.

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.Forge != "" {
		t.Errorf("expected empty Forge for dir with no CI signals, got %q", r.Forge)
	}
	// The generated YAML must not include the forge key.
	yaml := GenerateYAML(r)
	if containsSubstr(yaml, "forge:") {
		t.Errorf("expected 'forge:' key to be absent in YAML when forge not detected, got:\n%s", yaml)
	}
}

func TestInstallGates_Lefthook_GithubActions(t *testing.T) {
	dir := t.TempDir()
	// cria lefthook.yml (sem entrada trackfw)
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "# lefthook config\n")
	// cria .github/workflows (vazio — sem workflow existente)
	mustMkdir(t, dir, ".github/workflows")

	r := DiscoveryResult{
		HookFramework: "lefthook",
		CISystem:      "github-actions",
	}

	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	// validate script criado
	scriptPath := filepath.Join(dir, "scripts", "trackfw-validate.sh")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("validate script not found: %v", err)
	}
	if !containsSubstr(string(content), "trackfw validate") {
		t.Error("validate script should contain 'trackfw validate'")
	}

	// lefthook.yml atualizado com entrada trackfw
	hookContent, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if !containsSubstr(string(hookContent), "trackfw") {
		t.Error("lefthook.yml should contain trackfw entry")
	}

	// CI workflow criado
	workflowPath := filepath.Join(dir, ".github", "workflows", "trackfw-validate.yml")
	if _, err := os.Stat(workflowPath); err != nil {
		t.Errorf("CI workflow not found: %v", err)
	}
}

func TestInstallGates_Idempotente(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), "# trackfw already here\n")
	mustMkdir(t, dir, ".github/workflows")
	// workflow já existe
	mustWriteFile(t, filepath.Join(dir, ".github/workflows/trackfw-validate.yml"), "# existing\n")

	r := DiscoveryResult{
		HookFramework: "lefthook",
		CISystem:      "github-actions",
	}

	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	// lefthook não deve ter sido modificado (já contém trackfw)
	hookContent, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if string(hookContent) != "# trackfw already here\n" {
		t.Error("lefthook.yml should not be modified when trackfw is already present")
	}

	// workflow não deve ter sido sobrescrito
	wfContent, _ := os.ReadFile(filepath.Join(dir, ".github/workflows/trackfw-validate.yml"))
	if string(wfContent) != "# existing\n" {
		t.Error("existing CI workflow should not be overwritten")
	}
}

// TestInstallGates_GeraAttentionScripts confirma que `discover --init` (via InstallGates)
// gera scripts/trackfw-attention-signal.sh e scripts/trackfw-attention-cleanup.sh no
// rootDir, com o mesmo conteúdo produzido por `trackfw init` (generators.GenerateAttentionScripts),
// executáveis (0755), e que a segunda execução é idempotente (conteúdo inalterado).
func TestInstallGates_GeraAttentionScripts(t *testing.T) {
	dir := t.TempDir()

	r := DiscoveryResult{}
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	signalPath := filepath.Join(dir, "scripts", "trackfw-attention-signal.sh")
	cleanupPath := filepath.Join(dir, "scripts", "trackfw-attention-cleanup.sh")

	signalInfo, err := os.Stat(signalPath)
	if err != nil {
		t.Fatalf("attention signal script not found: %v", err)
	}
	if signalInfo.Mode().Perm() != 0755 {
		t.Errorf("attention signal script mode = %v, want 0755", signalInfo.Mode().Perm())
	}

	cleanupInfo, err := os.Stat(cleanupPath)
	if err != nil {
		t.Fatalf("attention cleanup script not found: %v", err)
	}
	if cleanupInfo.Mode().Perm() != 0755 {
		t.Errorf("attention cleanup script mode = %v, want 0755", cleanupInfo.Mode().Perm())
	}

	signalGot, err := os.ReadFile(signalPath)
	if err != nil {
		t.Fatalf("reading signal script: %v", err)
	}
	cleanupGot, err := os.ReadFile(cleanupPath)
	if err != nil {
		t.Fatalf("reading cleanup script: %v", err)
	}

	// Compara byte-a-byte com o que `trackfw init` produziria via
	// generators.GenerateAttentionScripts num diretório de referência independente.
	refDir := t.TempDir()
	if err := generators.GenerateAttentionScripts(refDir); err != nil {
		t.Fatalf("GenerateAttentionScripts (reference) error: %v", err)
	}
	signalWant, err := os.ReadFile(filepath.Join(refDir, "scripts", "trackfw-attention-signal.sh"))
	if err != nil {
		t.Fatalf("reading reference signal script: %v", err)
	}
	cleanupWant, err := os.ReadFile(filepath.Join(refDir, "scripts", "trackfw-attention-cleanup.sh"))
	if err != nil {
		t.Fatalf("reading reference cleanup script: %v", err)
	}

	if string(signalGot) != string(signalWant) {
		t.Error("discover --init attention signal script differs from trackfw init output")
	}
	if string(cleanupGot) != string(cleanupWant) {
		t.Error("discover --init attention cleanup script differs from trackfw init output")
	}

	// Idempotência: rodar novamente não deve corromper nem alterar os arquivos.
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates (2nd run) error: %v", err)
	}
	signalGot2, err := os.ReadFile(signalPath)
	if err != nil {
		t.Fatalf("reading signal script after 2nd run: %v", err)
	}
	if string(signalGot2) != string(signalGot) {
		t.Error("attention signal script changed after re-running InstallGates (not idempotent)")
	}
	cleanupGot2, err := os.ReadFile(cleanupPath)
	if err != nil {
		t.Fatalf("reading cleanup script after 2nd run: %v", err)
	}
	if string(cleanupGot2) != string(cleanupGot) {
		t.Error("attention cleanup script changed after re-running InstallGates (not idempotent)")
	}
}

// TestInstallGates_GeraCredentialGuardScript confirma que `discover --init` (via
// InstallGates) gera scripts/trackfw-credential-guard.sh no rootDir, com o mesmo
// conteúdo produzido por `trackfw init` (generators.GenerateCredentialGuardScript),
// executável (0755) — regressão do bug em que o gerador existia mas nunca era
// chamado por nenhum fluxo real (só por testes que o chamavam diretamente).
func TestInstallGates_GeraCredentialGuardScript(t *testing.T) {
	dir := t.TempDir()

	r := DiscoveryResult{}
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	guardPath := filepath.Join(dir, "scripts", "trackfw-credential-guard.sh")

	guardInfo, err := os.Stat(guardPath)
	if err != nil {
		t.Fatalf("credential guard script not found: %v", err)
	}
	if guardInfo.Mode().Perm() != 0755 {
		t.Errorf("credential guard script mode = %v, want 0755", guardInfo.Mode().Perm())
	}

	guardGot, err := os.ReadFile(guardPath)
	if err != nil {
		t.Fatalf("reading credential guard script: %v", err)
	}

	refDir := t.TempDir()
	if err := generators.GenerateCredentialGuardScript(refDir); err != nil {
		t.Fatalf("GenerateCredentialGuardScript (reference) error: %v", err)
	}
	guardWant, err := os.ReadFile(filepath.Join(refDir, "scripts", "trackfw-credential-guard.sh"))
	if err != nil {
		t.Fatalf("reading reference credential guard script: %v", err)
	}
	if string(guardGot) != string(guardWant) {
		t.Error("discover --init credential guard script differs from trackfw init output")
	}

	// Idempotência.
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates (2nd run) error: %v", err)
	}
	guardGot2, err := os.ReadFile(guardPath)
	if err != nil {
		t.Fatalf("reading credential guard script after 2nd run: %v", err)
	}
	if string(guardGot2) != string(guardGot) {
		t.Error("credential guard script changed after re-running InstallGates (not idempotent)")
	}
}

func TestGenerateYAML(t *testing.T) {
	r := DiscoveryResult{
		ADRDirs:            []string{"docs/adr/zeus", "docs/adr/apolo"},
		REQDir:             "docs/requisições",
		RoadmapDir:         "docs/roadmaps",
		RoadmapNamespacing: "by_agent",
		Agents:             []string{"zeus", "apolo"},
	}
	yaml := GenerateYAML(r)
	if !containsStr(yaml, "docs/requisições") {
		t.Error("YAML should contain req_dir with Portuguese path")
	}
	if !containsStr(yaml, "by_agent") {
		t.Error("YAML should contain by_agent")
	}
	if !containsStr(yaml, "governance_mode: lenient") {
		t.Error("YAML should contain lenient mode")
	}
}

// TestInstallLefthook_SemPackageJSON — projeto sem package.json → lefthook.yml criado
func TestInstallLefthook_SemPackageJSON(t *testing.T) {
	dir := t.TempDir()
	// sem package.json, sem lefthook.yml existente

	if err := installLefthook(dir, io.Discard); err != nil {
		t.Fatalf("installLefthook error: %v", err)
	}

	cfgPath := filepath.Join(dir, "lefthook.yml")
	content, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("lefthook.yml not created: %v", err)
	}
	if !containsSubstr(string(content), "trackfw-validate") {
		t.Errorf("lefthook.yml should contain trackfw-validate entry, got: %s", content)
	}
}

// TestInstallLefthook_Idempotente — lefthook já contém trackfw → não adiciona duplicata
func TestInstallLefthook_Idempotente(t *testing.T) {
	dir := t.TempDir()
	original := "pre-commit:\n  commands:\n    trackfw-validate:\n      run: scripts/trackfw-validate.sh\n"
	mustWriteFile(t, filepath.Join(dir, "lefthook.yml"), original)

	if err := installLefthook(dir, io.Discard); err != nil {
		t.Fatalf("installLefthook error: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	// o conteúdo não deve ter sido expandido — ainda é o original
	if string(content) != original {
		t.Errorf("lefthook.yml should remain unchanged, got: %s", content)
	}
}

// TestInstallHusky_ComPackageJSON — projeto com package.json → .husky/pre-commit criado
// O exec de npm/npx pode falhar no ambiente de teste; o importante é o arquivo .husky/pre-commit.
func TestInstallHusky_ComPackageJSON(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)

	if err := installHusky(dir, io.Discard); err != nil {
		t.Fatalf("installHusky error: %v", err)
	}

	huskyHook := filepath.Join(dir, ".husky", "pre-commit")
	content, err := os.ReadFile(huskyHook)
	if err != nil {
		t.Fatalf(".husky/pre-commit not created: %v", err)
	}
	if !containsSubstr(string(content), "trackfw-validate.sh") {
		t.Errorf(".husky/pre-commit should contain trackfw-validate.sh, got: %s", content)
	}
}

// TestInstallHook_DefaultSemPackageJSON — via installHook com framework "none" sem package.json.
// Quando node está no PATH, o fallback é husky via npx (.husky/pre-commit).
// Quando node não está, o fallback é lefthook (lefthook.yml).
func TestInstallHook_DefaultSemPackageJSON(t *testing.T) {
	dir := t.TempDir()
	// sem package.json → fallback depende da presença de node no PATH

	if err := installHook("none", dir, io.Discard); err != nil {
		t.Fatalf("installHook error: %v", err)
	}

	huskyHook := filepath.Join(dir, ".husky", "pre-commit")
	lefthookCfg := filepath.Join(dir, "lefthook.yml")

	huskyExists := fileExists(huskyHook)
	lefthookExists := fileExists(lefthookCfg)

	if !huskyExists && !lefthookExists {
		t.Error("expected either .husky/pre-commit (node fallback) or lefthook.yml (lefthook fallback) to be created")
	}
}

// TestInstallHook_DefaultComPackageJSON — via installHook com framework "none" com package.json
func TestInstallHook_DefaultComPackageJSON(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)

	if err := installHook("none", dir, io.Discard); err != nil {
		t.Fatalf("installHook error: %v", err)
	}

	huskyHook := filepath.Join(dir, ".husky", "pre-commit")
	if _, err := os.Stat(huskyHook); err != nil {
		t.Errorf(".husky/pre-commit should have been created: %v", err)
	}
}

// TestInstallHuskyNPX_SemPackageJSON — installHuskyNPX cria .husky/pre-commit mesmo sem package.json.
// O npx pode falhar no CI (warning não-bloqueante), mas o arquivo de hook deve ser criado.
func TestInstallHuskyNPX_SemPackageJSON(t *testing.T) {
	dir := t.TempDir()
	// sem package.json — installHuskyNPX deve criar .husky/pre-commit de qualquer forma

	if err := installHuskyNPX(dir, io.Discard); err != nil {
		t.Fatalf("installHuskyNPX error: %v", err)
	}

	huskyHook := filepath.Join(dir, ".husky", "pre-commit")
	content, err := os.ReadFile(huskyHook)
	if err != nil {
		t.Fatalf(".husky/pre-commit not created: %v", err)
	}
	if !containsSubstr(string(content), "trackfw-validate.sh") {
		t.Errorf(".husky/pre-commit should contain trackfw-validate.sh, got: %s", content)
	}
}

// TestScan_SuggestedTestFramework_Jest verifica que a presença de jest.config.js
// sugere "jest".
func TestScan_SuggestedTestFramework_Jest(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "jest.config.js"), "module.exports = {}\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "jest" {
		t.Errorf("expected 'jest', got %q", r.SuggestedTestFramework)
	}
}

// TestScan_SuggestedTestFramework_JestTS verifica jest.config.ts também sugere "jest".
func TestScan_SuggestedTestFramework_JestTS(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "jest.config.ts"), "export default {}\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "jest" {
		t.Errorf("expected 'jest', got %q", r.SuggestedTestFramework)
	}
}

// TestScan_SuggestedTestFramework_Vitest verifica que a presença de vitest.config.js
// sugere "vitest".
func TestScan_SuggestedTestFramework_Vitest(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "vitest.config.js"), "export default {}\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "vitest" {
		t.Errorf("expected 'vitest', got %q", r.SuggestedTestFramework)
	}
}

// TestScan_SuggestedTestFramework_VitestTS verifica vitest.config.ts também sugere "vitest".
func TestScan_SuggestedTestFramework_VitestTS(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "vitest.config.ts"), "export default {}\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "vitest" {
		t.Errorf("expected 'vitest', got %q", r.SuggestedTestFramework)
	}
}

// TestScan_SuggestedTestFramework_PytestIni verifica que pytest.ini sugere "pytest".
func TestScan_SuggestedTestFramework_PytestIni(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "pytest.ini"), "[pytest]\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "pytest" {
		t.Errorf("expected 'pytest', got %q", r.SuggestedTestFramework)
	}
}

// TestScan_SuggestedTestFramework_PyprojectToml verifica que pyproject.toml com seção
// [tool.pytest.ini_options] sugere "pytest".
func TestScan_SuggestedTestFramework_PyprojectToml(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "pyproject.toml"), "[tool.pytest.ini_options]\naddopts = \"-ra\"\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "pytest" {
		t.Errorf("expected 'pytest', got %q", r.SuggestedTestFramework)
	}
}

// TestScan_SuggestedTestFramework_PyprojectTomlSemPytest verifica que um pyproject.toml
// SEM seção [tool.pytest...] não sugere "pytest".
func TestScan_SuggestedTestFramework_PyprojectTomlSemPytest(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "pyproject.toml"), "[tool.black]\nline-length = 88\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "" {
		t.Errorf("expected '', got %q", r.SuggestedTestFramework)
	}
}

// TestScan_SuggestedTestFramework_SetupCfg verifica que setup.cfg com [tool:pytest]
// sugere "pytest".
func TestScan_SuggestedTestFramework_SetupCfg(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "setup.cfg"), "[tool:pytest]\ntestpaths = tests\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "pytest" {
		t.Errorf("expected 'pytest', got %q", r.SuggestedTestFramework)
	}
}

// TestScan_SuggestedTestFramework_GoTest verifica que go.mod + *_test.go em qualquer
// lugar do repositório sugere "go test".
func TestScan_SuggestedTestFramework_GoTest(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.22\n")
	mustMkdir(t, dir, "internal/foo")
	mustWriteFile(t, filepath.Join(dir, "internal/foo/foo_test.go"), "package foo\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "go test" {
		t.Errorf("expected 'go test', got %q", r.SuggestedTestFramework)
	}
}

// TestScan_SuggestedTestFramework_GoModSemTestFile verifica que go.mod SEM nenhum
// *_test.go NÃO sugere "go test".
func TestScan_SuggestedTestFramework_GoModSemTestFile(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "go.mod"), "module example.com/foo\n\ngo 1.22\n")

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "" {
		t.Errorf("expected '', got %q", r.SuggestedTestFramework)
	}
}

// TestScan_SuggestedTestFramework_Nenhum verifica que a ausência de todos os
// arquivos-gatilho resulta em SuggestedTestFramework vazio (nunca erro).
func TestScan_SuggestedTestFramework_Nenhum(t *testing.T) {
	dir := t.TempDir()

	r, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r.SuggestedTestFramework != "" {
		t.Errorf("expected '', got %q", r.SuggestedTestFramework)
	}
}

// TestWriteCIWorkflow_PinsGoInstallToBinaryVersion proves that the second install
// mechanism (`go install .../cmd/trackfw@latest`, written by InstallGates into
// trackfw-validate.yml) is pinned to the version of the binary that generated it — the
// same defect class the Wave 0 correction (ROADMAP-2026-08-28) named against the
// install.sh mechanism, applied to this second surface. Asserts against
// version.Version directly (not a hardcoded literal): if version.Version changes, this
// assertion must still pass without editing the test, proving the pin is not hardcoded
// in the generator either.
func TestWriteCIWorkflow_PinsGoInstallToBinaryVersion(t *testing.T) {
	dir := t.TempDir()
	r := DiscoveryResult{CISystem: "github-actions"}
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "trackfw-validate.yml"))
	if err != nil {
		t.Fatalf("trackfw-validate.yml not found: %v", err)
	}

	wantPin := "go install github.com/kgsaran/trackfw/cmd/trackfw@v" + version.Version
	if !containsSubstr(string(content), wantPin) {
		t.Errorf("trackfw-validate.yml does not contain pinned install line %q, got:\n%s", wantPin, content)
	}
	if containsSubstr(string(content), "trackfw/cmd/trackfw@latest") {
		t.Errorf("trackfw-validate.yml still contains unpinned '@latest', got:\n%s", content)
	}
}

// TestWriteCIWorkflow_VersionNotHardcoded falsifies the specific regression the ADR
// warns against (roadmap Wave 0 section 3, "install.sh"/scaffold.go falsification
// table applied to this surface): a template with `@v7.3.0` typed literally into the
// generator source would pass every other test in this file today, because
// version.Version happens to equal "7.3.0" right now. Stubbing version.Version to a
// value that never appears in the source is the only way to prove the pin tracks the
// variable, not a literal.
func TestWriteCIWorkflow_VersionNotHardcoded(t *testing.T) {
	orig := version.Version
	version.Version = "9.9.9-stub"
	defer func() { version.Version = orig }()

	dir := t.TempDir()
	r := DiscoveryResult{CISystem: "github-actions"}
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "trackfw-validate.yml"))
	if err != nil {
		t.Fatalf("trackfw-validate.yml not found: %v", err)
	}

	if !containsSubstr(string(content), "trackfw@v9.9.9-stub") {
		t.Errorf("stubbed version.Version did not propagate to the template, got:\n%s", content)
	}
	if containsSubstr(string(content), "7.3.0") {
		t.Errorf("template still contains the real version 7.3.0 after stubbing — pin looks hardcoded, got:\n%s", content)
	}
}

// TestWriteCIWorkflow_MatchesGeneratorTemplateByteForByte proves the file InstallGates
// writes is byte-identical to generators.BuildDiscoverGitHubActionsWorkflowContent —
// the same function scaffold doctor uses to detect drift (AC10/AC11). If this ever
// diverges, doctor would either never fire (false negative) or always fire (false
// positive) against a project just generated by the current binary.
func TestWriteCIWorkflow_MatchesGeneratorTemplateByteForByte(t *testing.T) {
	dir := t.TempDir()
	r := DiscoveryResult{CISystem: "github-actions"}
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".github", "workflows", "trackfw-validate.yml"))
	if err != nil {
		t.Fatalf("trackfw-validate.yml not found: %v", err)
	}

	want := generators.BuildDiscoverGitHubActionsWorkflowContent()
	if string(content) != want {
		t.Errorf("trackfw-validate.yml diverges from generators.BuildDiscoverGitHubActionsWorkflowContent:\ngot:\n%s\nwant:\n%s", content, want)
	}
}

// TestWriteCIWorkflow_IdempotentSameBinary proves that regenerating trackfw-validate.yml
// twice with the same binary produces byte-identical output — the "regressed too far
// the other way" side of the AC10/AC11 falsification pair from ROADMAP-2026-08-28
// section 3 (a template that changes on every run would make doctor noisy forever).
func TestWriteCIWorkflow_IdempotentSameBinary(t *testing.T) {
	first := generators.BuildDiscoverGitHubActionsWorkflowContent()
	second := generators.BuildDiscoverGitHubActionsWorkflowContent()
	if first != second {
		t.Errorf("BuildDiscoverGitHubActionsWorkflowContent is not idempotent across calls:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestRunScaffoldDoctor_DiscoverWorkflow_NoMismatchAfterGenerate proves AC11: a project
// whose trackfw-validate.yml was just written by the current binary is reported clean
// by scaffold doctor — no scaffold-divergent finding for a file the binary itself wrote.
func TestRunScaffoldDoctor_DiscoverWorkflow_NoMismatchAfterGenerate(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "trackfw.yaml"), "backend: go\n")
	r := DiscoveryResult{CISystem: "github-actions"}
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	findings, err := generators.RunScaffoldDoctor(dir)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor error: %v", err)
	}
	for _, f := range findings {
		if f.Destination == generators.DiscoverGitHubActionsWorkflowPath {
			t.Errorf("expected no finding for %s right after generation, got: %+v", generators.DiscoverGitHubActionsWorkflowPath, f)
		}
	}
}

// TestRunScaffoldDoctor_DiscoverWorkflow_DivergentWhenPinManuallyChanged proves AC10:
// scaffold doctor accuses trackfw-validate.yml as scaffold-divergent when its pin no
// longer matches the version the current binary would generate — the exact scenario
// ROADMAP-2026-08-28 exists to make visible (a stale pin must not be silently invisible
// in any of the two install-mechanism artifacts, matching the pre-existing coverage of
// trackfw-gate.yml).
func TestRunScaffoldDoctor_DiscoverWorkflow_DivergentWhenPinManuallyChanged(t *testing.T) {
	dir := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "trackfw.yaml"), "backend: go\n")
	r := DiscoveryResult{CISystem: "github-actions"}
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	workflowPath := filepath.Join(dir, ".github", "workflows", "trackfw-validate.yml")
	original, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("reading generated workflow: %v", err)
	}
	tampered := strings.Replace(string(original), "trackfw@v"+version.Version, "trackfw@v0.0.1-stale", 1)
	if tampered == string(original) {
		t.Fatalf("tamper substitution did not change content — test setup broken")
	}
	if err := os.WriteFile(workflowPath, []byte(tampered), 0644); err != nil {
		t.Fatalf("writing tampered workflow: %v", err)
	}

	findings, err := generators.RunScaffoldDoctor(dir)
	if err != nil {
		t.Fatalf("RunScaffoldDoctor error: %v", err)
	}
	found := false
	for _, f := range findings {
		if f.Destination == generators.DiscoverGitHubActionsWorkflowPath {
			found = true
			if f.FindingKind != integrations.DoctorScaffoldDivergent {
				t.Errorf("expected DoctorScaffoldDivergent, got %v", f.FindingKind)
			}
		}
	}
	if !found {
		t.Errorf("expected a scaffold-divergent finding for %s with a stale pin, got none in: %+v", generators.DiscoverGitHubActionsWorkflowPath, findings)
	}
}

// TestWriteCIWorkflow_NeverWritesThroughLiveSymlink is the corrective
// falsifier for the symlink-follow arbitrary-write reported by hades-tf's
// final barrier review (2026-08-28): `trackfw discover --init` with a LIVE
// symlink already at .github/workflows/trackfw-validate.yml pointing OUTSIDE
// the project must not overwrite the file the symlink points to. Before the
// fix, writeCIWorkflow decided presence with fileExists (os.Stat, follows
// symlinks), treated the symlink as "already installed" only by accident of
// its target existing, and os.WriteFile followed the link straight through.
func TestWriteCIWorkflow_NeverWritesThroughLiveSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "trackfw.yaml"), "backend: go\n")

	victim := filepath.Join(outside, "vitima.txt")
	const originalContent = "CONTEUDO ORIGINAL DA VITIMA\n"
	mustWriteFile(t, victim, originalContent)

	workflowPath := filepath.Join(dir, ".github", "workflows", "trackfw-validate.yml")
	mustMkdir(t, dir, ".github/workflows")
	if err := os.Symlink(victim, workflowPath); err != nil {
		t.Fatal(err)
	}

	r := DiscoveryResult{CISystem: "github-actions"}
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	got, err := os.ReadFile(victim)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != originalContent {
		t.Fatalf("symlink-follow arbitrary write: victim file outside the project was overwritten.\nwant: %q\ngot:  %q", originalContent, got)
	}
	linkInfo, err := os.Lstat(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected %s to remain a symlink (untouched), got mode %v", workflowPath, linkInfo.Mode())
	}
}

// TestWriteCIWorkflow_NeverWritesThroughDanglingSymlink is the same
// falsifier for the dangling-symlink variant: the link target does not
// exist yet, so a naive os.Stat-based idempotency guard reports "not
// present" and os.WriteFile CREATES the file at the attacker-chosen path.
func TestWriteCIWorkflow_NeverWritesThroughDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	mustWriteFile(t, filepath.Join(dir, "trackfw.yaml"), "backend: go\n")

	danglingTarget := filepath.Join(outside, "does-not-exist-yet")
	workflowPath := filepath.Join(dir, ".github", "workflows", "trackfw-validate.yml")
	mustMkdir(t, dir, ".github/workflows")
	if err := os.Symlink(danglingTarget, workflowPath); err != nil {
		t.Fatal(err)
	}

	r := DiscoveryResult{CISystem: "github-actions"}
	if err := InstallGates(r, dir, io.Discard); err != nil {
		t.Fatalf("InstallGates error: %v", err)
	}

	if _, err := os.Lstat(danglingTarget); !os.IsNotExist(err) {
		t.Fatalf("dangling-symlink arbitrary write: %s was created outside the project (stat err=%v)", danglingTarget, err)
	}
}

// helpers

func mustMkdir(t *testing.T, base, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(base, rel), 0755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstr(s, sub))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// findSubstr returns the index of the first occurrence of sub in s, or -1.
func findSubstr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

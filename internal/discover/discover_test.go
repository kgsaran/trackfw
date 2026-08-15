package discover

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
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

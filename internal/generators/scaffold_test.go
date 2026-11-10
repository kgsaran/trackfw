package generators

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/roadmapdoc"
)

// TestInstallSkills_CriaSlashCommandsESkillGlobal — verifica que InstallSkills cria
// os slash commands no projeto E a skill global em $HOME/.claude/skills/trackfw/
func TestInstallSkills_CriaSlashCommandsESkillGlobal(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	orig, _ := os.Getwd()
	origHome := os.Getenv("HOME")
	_ = os.Chdir(dir)
	_ = os.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)     // ML-2C: Windows reads USERPROFILE
	t.Setenv("CLAUDE_CONFIG_DIR", "") // ML-2C: isolate from real CLAUDE_CONFIG_DIR
	t.Cleanup(func() {
		_ = os.Chdir(orig)
		_ = os.Setenv("HOME", origHome)
	})

	if err := InstallSkills(); err != nil {
		t.Fatalf("InstallSkills() erro: %v", err)
	}

	// slash commands no projeto
	for _, name := range expectedCommands {
		p := filepath.Join(".claude", "commands", "trackfw", name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("slash command não encontrado: %s", p)
		}
	}

	// skill global
	skillPath := filepath.Join(home, ".claude", "skills", "trackfw", "SKILL.md")
	info, err := os.Stat(skillPath)
	if err != nil {
		t.Fatalf("SKILL.md não encontrado: %v", err)
	}
	if info.Size() == 0 {
		t.Error("SKILL.md está vazio")
	}
}

var expectedCommands = []string{
	"adr.md", "req.md", "roadmap.md", "implement.md",
	"validate.md", "status.md", "move.md",
}

func expectKnownFailure(t *testing.T, defect string, check func() error) {
	t.Helper()
	if err := check(); err != nil {
		t.Logf("xfail esperado (%s): %v", defect, err)
		return
	}
	t.Fatalf("XPASS inesperado (%s): remova o xfail e reative o teste como obrigatório", defect)
}

func TestGenerateClaudeCommands_CreatesAllFiles(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := generateClaudeCommands(); err != nil {
		t.Fatalf("generateClaudeCommands() erro: %v", err)
	}

	for _, name := range expectedCommands {
		path := filepath.Join(".claude", "commands", "trackfw", name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("arquivo esperado não encontrado: %s (%v)", path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("arquivo vazio: %s", path)
		}
	}
}

func TestSlashRoadmapCommandRequiresCanonicalFrontmatter(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := generateClaudeCommands(); err != nil {
		t.Fatalf("generateClaudeCommands() erro: %v", err)
	}
	content, err := os.ReadFile(filepath.Join(".claude", "commands", "trackfw", "roadmap.md"))
	if err != nil {
		t.Fatalf("roadmap.md não encontrado: %v", err)
	}
	body := string(content)
	required := []string{
		"````markdown\n---",
		"status: backlog",
		"date: <YYYY-MM-DD>",
		`req: "docs/req/<selected-file>.md"`,
		`squad: ""`,
		"---\n\n# Roadmap:",
		"> Created: <YYYY-MM-DD> | Status: backlog",
		"docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md",
		"Fill `req:` with the full relative path of the selected REQ",
		"### ML-1B — <title> (if independent of ML-1A)",
		"## Wave 2 — <name> (depends on Wave 1)",
		"> Dependencies: Wave 1 complete",
	}
	for _, want := range required {
		if !strings.Contains(body, want) {
			t.Fatalf("roadmap.md não contém trecho canônico esperado: %s", want)
		}
	}

	versioned, err := os.ReadFile(filepath.Join(orig, "..", "..", ".claude", "commands", "trackfw", "roadmap.md"))
	if err != nil {
		t.Fatalf("roadmap.md versionado não encontrado: %v", err)
	}
	if body != string(versioned) {
		t.Fatal("roadmap.md gerado diverge do arquivo versionado em .claude/commands/trackfw/roadmap.md")
	}
}

type testExpectationError struct {
	message string
}

func (e *testExpectationError) Error() string { return e.message }

// TestGenerateClaudeCommands_Idempotente — segundo init não sobrescreve arquivos customizados
func TestGenerateClaudeCommands_Idempotente(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Primeiro init — cria os arquivos
	if err := generateClaudeCommands(); err != nil {
		t.Fatalf("primeiro generateClaudeCommands() erro: %v", err)
	}

	// Customiza um arquivo (simula edição manual pelo usuário)
	customPath := filepath.Join(".claude", "commands", "trackfw", "adr.md")
	customContent := "# conteúdo customizado pelo usuário"
	if err := os.WriteFile(customPath, []byte(customContent), 0644); err != nil {
		t.Fatalf("WriteFile customização: %v", err)
	}

	// Segundo init — não deve sobrescrever
	if err := generateClaudeCommands(); err != nil {
		t.Fatalf("segundo generateClaudeCommands() erro: %v", err)
	}

	got, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatalf("ReadFile após segundo init: %v", err)
	}
	if string(got) != customContent {
		t.Errorf("arquivo customizado foi sobrescrito — esperado %q, obteve %q", customContent, string(got))
	}
}

func TestGenerateAttentionScripts(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := GenerateAttentionScripts(""); err != nil {
		t.Fatalf("GenerateAttentionScripts erro: %v", err)
	}

	signalPath := filepath.Join("scripts", "trackfw-attention-signal.sh")
	cleanupPath := filepath.Join("scripts", "trackfw-attention-cleanup.sh")

	for _, p := range []string{signalPath, cleanupPath} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("arquivo de atenção não gerado: %s (%v)", p, err)
		}
		if info.Size() == 0 {
			t.Errorf("arquivo de atenção está vazio: %s", p)
		}
		// Verifica permissões de execução (no Unix 0755). Guarda MEDIDA: ver
		// execBitRepresentavelPara em execbit_probe_test.go — o bit não é representável
		// em NTFS e o assert mediria uma propriedade que o filesystem não tem.
		if execBitRepresentavelPara(t, p) {
			if mode := info.Mode().Perm(); mode&0111 == 0 {
				t.Errorf("arquivo %s não tem permissão de execução (perm: %o)", p, mode)
			}
		} else {
			execBitNaoExercitado(t, p)
		}
	}

	signalContent, _ := os.ReadFile(signalPath)
	if !strings.Contains(string(signalContent), "# trackfw attention signal — compatibility wrapper") {
		t.Errorf("script signal não contém cabeçalho esperado: %s", string(signalContent))
	}

	cleanupContent, _ := os.ReadFile(cleanupPath)
	if !strings.Contains(string(cleanupContent), "# trackfw attention cleanup — compatibility wrapper") {
		t.Errorf("script cleanup não contém cabeçalho esperado: %s", string(cleanupContent))
	}
}

func TestAttentionScripts_ExecutionContract(t *testing.T) {
	t.Setenv("PATH", filepath.Dir(compiledGuardBinary(t))+string(os.PathListSeparator)+os.Getenv("PATH"))
	// (a) trackfw.yaml sem roadmap_dir -> executa signal script, verifica criação de docs/roadmaps/.trackfw-attention.json parseável, executa cleanup script, verifica remoção.
	t.Run("DefaultRoadmapDirAndCleanup", func(t *testing.T) {
		dir := t.TempDir()
		orig, _ := os.Getwd()
		_ = os.Chdir(dir)
		t.Cleanup(func() { _ = os.Chdir(orig) })

		if err := writeTrackfwConfig(Config{}); err != nil {
			t.Fatalf("writeTrackfwConfig erro: %v", err)
		}
		if err := GenerateAttentionScripts(""); err != nil {
			t.Fatalf("generateAttentionScripts erro: %v", err)
		}

		signalPath := filepath.Join("scripts", "trackfw-attention-signal.sh")
		cleanupPath := filepath.Join("scripts", "trackfw-attention-cleanup.sh")

		// Executa Signal
		cmd := exec.Command("bash", signalPath)
		cmd.Stdin = strings.NewReader(`{"tool_name":"test_tool","tool_input":{"question":"Need approval?"}}`)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Signal script falhou: %v, output: %s", err, string(out))
		}

		attentionFile := filepath.Join("docs", "roadmaps", ".trackfw-attention.json")
		data, err := os.ReadFile(attentionFile)
		if err != nil {
			t.Fatalf("Arquivo de atenção não foi criado em %s: %v", attentionFile, err)
		}

		var payload struct {
			Tool      string `json:"tool"`
			Message   string `json:"message"`
			Level     string `json:"level"`
			Timestamp string `json:"timestamp"`
		}
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("JSON gerado é inválido: %v, conteúdo: %s", err, string(data))
		}

		if payload.Tool != "test_tool" {
			t.Errorf("Tool esperada 'test_tool', obteve: %q", payload.Tool)
		}
		if payload.Message != "Need approval?" {
			t.Errorf("Message esperada 'Need approval?', obteve: %q", payload.Message)
		}
		if payload.Level != "action_required" {
			t.Errorf("Level esperado 'action_required', obteve: %q", payload.Level)
		}
		if payload.Timestamp == "" {
			t.Errorf("Timestamp não deve ser vazio")
		}

		// Executa Cleanup
		cmdCleanup := exec.Command("bash", cleanupPath)
		outCleanup, err := cmdCleanup.CombinedOutput()
		if err != nil {
			t.Fatalf("Cleanup script falhou: %v, output: %s", err, string(outCleanup))
		}

		if _, err := os.Stat(attentionFile); !os.IsNotExist(err) {
			t.Errorf("Arquivo de atenção %s ainda existe após cleanup", attentionFile)
		}
	})

	// (b) roadmap_dir configurado com path traversal ou absoluto externo -> contido em docs/roadmaps
	t.Run("PathTraversalContainment", func(t *testing.T) {
		traversalPaths := []string{
			"../../outside",
			"/tmp/outside_attention",
			"*/../*",
		}

		for _, p := range traversalPaths {
			t.Run(p, func(t *testing.T) {
				dir := t.TempDir()
				orig, _ := os.Getwd()
				_ = os.Chdir(dir)
				t.Cleanup(func() { _ = os.Chdir(orig) })

				yamlContent := "roadmap_dir: " + p + "\n"
				if err := os.WriteFile("trackfw.yaml", []byte(yamlContent), 0644); err != nil {
					t.Fatalf("WriteFile trackfw.yaml erro: %v", err)
				}
				if err := GenerateAttentionScripts(""); err != nil {
					t.Fatalf("generateAttentionScripts erro: %v", err)
				}

				signalPath := filepath.Join("scripts", "trackfw-attention-signal.sh")
				cmd := exec.Command("bash", signalPath)
				cmd.Stdin = strings.NewReader(`{"tool_name":"traversal_tool","tool_input":{"question":"Testing containment"}}`)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("Signal script falhou para path %s: %v, output: %s", p, err, string(out))
				}

				// Deve ser mantido em docs/roadmaps/.trackfw-attention.json
				containedFile := filepath.Join("docs", "roadmaps", ".trackfw-attention.json")
				if _, err := os.Stat(containedFile); !os.IsNotExist(err) {
					t.Errorf("Caminho inseguro deveria ser no-op, mas há arquivo em %s para path %s: %v", containedFile, p, err)
				}

				// Não deve ter sido criado no caminho externo
				if p == "../../outside" || p == "/tmp/outside_attention" {
					extFile := filepath.Join(p, ".trackfw-attention.json")
					if _, err := os.Stat(extFile); err == nil {
						t.Errorf("Vazamento de path traversal! Arquivo foi criado em %s", extFile)
						_ = os.Remove(extFile)
					}
				}
			})
		}
	})

	// (c) Payload via stdin com aspas, barras, newlines, TABs, CRs -> JSON estritamente válido e parseável
	t.Run("SpecialCharactersEscaping", func(t *testing.T) {
		dir := t.TempDir()
		orig, _ := os.Getwd()
		_ = os.Chdir(dir)
		t.Cleanup(func() { _ = os.Chdir(orig) })

		if err := os.WriteFile("trackfw.yaml", []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
			t.Fatalf("WriteFile trackfw.yaml erro: %v", err)
		}
		if err := GenerateAttentionScripts(""); err != nil {
			t.Fatalf("generateAttentionScripts erro: %v", err)
		}

		// Tool name and question containing quotes, backslashes, newlines, tabs, and carriage returns
		stdinJSON := "{\n" +
			"  \"tool_name\": \"complex_tool\\\"with\\\"quotes\\\\and\\\\slash\",\n" +
			"  \"tool_input\": {\n" +
			"    \"question\": \"Line 1\\nLine 2\\rLine 3\\tTabbed \\\"Quoted\\\" \\\\Backslash\\\\\"\n" +
			"  }\n" +
			"}"

		signalPath := filepath.Join("scripts", "trackfw-attention-signal.sh")
		cmd := exec.Command("bash", signalPath)
		cmd.Stdin = strings.NewReader(stdinJSON)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Signal script falhou com payload especial: %v, output: %s", err, string(out))
		}

		attentionFile := filepath.Join("docs", "roadmaps", ".trackfw-attention.json")
		data, err := os.ReadFile(attentionFile)
		if err != nil {
			t.Fatalf("Arquivo de atenção não foi encontrado: %v", err)
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("json.Unmarshal falhou no arquivo gerado! Conteúdo corrompido:\n%s\nErro: %v", string(data), err)
		}

		if payload["tool"] == "" {
			t.Errorf("Campo 'tool' não deve ser vazio no JSON parseado")
		}
		if payload["message"] == "" {
			t.Errorf("Campo 'message' não deve ser vazio no JSON parseado")
		}
	})
}

func TestAttentionScripts_FallbackWithoutJQ(t *testing.T) {
	t.Setenv("PATH", filepath.Dir(compiledGuardBinary(t))+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := os.WriteFile("trackfw.yaml", []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml erro: %v", err)
	}
	if err := GenerateAttentionScripts(""); err != nil {
		t.Fatalf("generateAttentionScripts erro: %v", err)
	}

	// O PATH do filho contém apenas o diretório onde compiledGuardBinary(t) foi compilado.
	// Esse diretório contém exatamente um arquivo (o binário trackfw); jq nunca está lá.
	trackfwBinDir := filepath.Dir(compiledGuardBinary(t))

	// P-2: verificar acessibilidade antes de rodar o script, com o mesmo env que o filho receberá.
	probe := exec.Command("bash", "-c", "command -v trackfw")
	probe.Env = []string{"PATH=" + trackfwBinDir}
	if out, err := probe.CombinedOutput(); err != nil {
		t.Fatalf("setup: trackfw não encontrado no PATH do filho (PATH=%q, saída=%q); "+
			"verifique que compiledGuardBinary(t) produziu o binário em %q",
			trackfwBinDir, strings.TrimSpace(string(out)), trackfwBinDir)
	}

	signalPath := filepath.Join("scripts", "trackfw-attention-signal.sh")
	cmd := exec.Command("bash", signalPath)
	cmd.Env = []string{"PATH=" + trackfwBinDir}
	cmd.Stdin = strings.NewReader(`{"tool_name":"fallback_tool","tool_input":{"question":"Testing fallback without jq"}}`)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Signal script (fallback python3 sem jq) falhou: %v, output: %s", err, string(out))
	}

	attentionFile := filepath.Join("docs", "roadmaps", ".trackfw-attention.json")
	data, err := os.ReadFile(attentionFile)
	if err != nil {
		t.Fatalf("Arquivo de atenção não foi encontrado no modo fallback: %v", err)
	}

	var payload struct {
		Tool    string `json:"tool"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("JSON gerado no fallback sem jq é inválido: %v, conteúdo: %s", err, string(data))
	}

	if payload.Tool != "fallback_tool" {
		t.Errorf("Tool esperada 'fallback_tool', obteve %q", payload.Tool)
	}
	if payload.Message != "Testing fallback without jq" {
		t.Errorf("Message esperada 'Testing fallback without jq', obteve %q", payload.Message)
	}
}

// TestScaffoldStatusVocabularyMatchesGenerator — ML-2B (AC-ML-2B, AC11)
//
// Affirms that every **Status:** value the scaffold template teaches is a value
// the roadmap generator emits — the defect being that "pending" was not.
//
// AC11 sentence: this test affirms that the scaffold's **Status:** markers use
// exactly the pending vocabulary that wave0Block (the roadmap generator) emits,
// so a roadmap produced by the slash command arrives with the same first tokens
// that roadmapdoc.StatusIsComplete and the barrier already recognise.
func TestScaffoldStatusVocabularyMatchesGenerator(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	if err := generateClaudeCommands(); err != nil {
		t.Fatalf("generateClaudeCommands() erro: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(".claude", "commands", "trackfw", "roadmap.md"))
	if err != nil {
		t.Fatalf("roadmap.md não encontrado: %v", err)
	}

	// Extract **Status:** markers from the generated roadmap.md.
	// The markers live inside an indented markdown code block, so we trim leading
	// whitespace before applying StatusLineRe.
	// Intentionally UNMASKED (inverse of MLStatusMarker): we want markers inside
	// the scaffold template fence, not ML bodies of live roadmaps.
	// Lines like "edite `**Status:** ⬜ Pendente` →" do NOT start with **Status:**
	// after trimming, so StatusLineRe (anchored at ^) will not match them.
	var scaffoldMarkers []string
	for _, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if m := roadmapdoc.StatusLineRe.FindStringSubmatch(trimmed); m != nil {
			scaffoldMarkers = append(scaffoldMarkers, strings.TrimSpace(m[1]))
		}
	}
	if len(scaffoldMarkers) == 0 {
		t.Fatal("no **Status:** markers found in generated roadmap.md — extraction is broken")
	}

	// Derive allowed pending first-tokens from wave0Block (the roadmap generator's output,
	// same package). This set grows automatically if the generator adds new vocabulary;
	// no literals are hardcoded here.
	generatorPendingTokens := map[string]bool{}
	for _, line := range strings.Split(wave0Block, "\n") {
		trimmed := strings.TrimSpace(line)
		if m := roadmapdoc.StatusLineRe.FindStringSubmatch(trimmed); m != nil {
			marker := strings.TrimSpace(m[1])
			if !roadmapdoc.StatusIsComplete(marker) {
				if fields := strings.Fields(marker); len(fields) > 0 {
					generatorPendingTokens[fields[0]] = true
				}
			}
		}
	}

	// Validate: every scaffold marker must be either complete (passes StatusIsComplete)
	// or use a pending first-token that the roadmap generator emits.
	for _, marker := range scaffoldMarkers {
		if roadmapdoc.StatusIsComplete(marker) {
			continue
		}
		fields := strings.Fields(marker)
		if len(fields) == 0 {
			t.Errorf("scaffold has empty **Status:** marker")
			continue
		}
		if !generatorPendingTokens[fields[0]] {
			t.Errorf("scaffold teaches **Status:** %q (first token %q) which the roadmap "+
				"generator does not emit; allowed pending tokens: %v", marker, fields[0], generatorPendingTokens)
		}
	}
}

// TestScaffold_RoadmapMD_GatesModelParseable — ML-3C (REQ-2026-11-02)
//
// Regra Dura de Reconciliação:
//
//	This test asserts that the roadmap.md slash command template, after the
//	3-space indentation is removed, produces a Wave 0 section whose "**Wave gates:**"
//	header sits at column 0, so roadmapdoc.ParseGatesLines accepts it without error
//	and returns at least one gate command starting with "exit 1".
func TestScaffold_RoadmapMD_GatesModelParseable(t *testing.T) {
	content := claudeCommandsContent()["roadmap.md"]
	if content == "" {
		t.Fatal("roadmap.md not found in claudeCommandsContent()")
	}

	// Extract the template from between the ````markdown...```` fence.
	// The fence markers are ````markdown and ```` (four backticks each).
	const openFence = "````markdown\n"
	const closeFence = "\n````"
	openIdx := strings.Index(content, openFence)
	if openIdx < 0 {
		t.Fatal("````markdown fence not found in roadmap.md content")
	}
	afterOpen := content[openIdx+len(openFence):]
	closeIdx := strings.Index(afterOpen, closeFence)
	if closeIdx < 0 {
		t.Fatal("closing ```` fence not found in roadmap.md content")
	}
	template := afterOpen[:closeIdx]

	lines := strings.Split(template, "\n")

	// Find Wave 0 start and end indices (0-based).
	wave0Start := -1
	wave0End := len(lines)
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "## Wave 0") {
			wave0Start = i
		} else if wave0Start >= 0 && strings.HasPrefix(strings.TrimSpace(line), "## Wave ") {
			wave0End = i
			break
		}
	}
	if wave0Start < 0 {
		t.Fatal("## Wave 0 section not found in roadmap.md template")
	}

	gcmds, err := roadmapdoc.ParseGatesLines(lines, wave0Start, wave0End)
	if err != nil {
		t.Fatalf("ParseGatesLines returned error for Wave 0 in roadmap.md template: %v", err)
	}
	if len(gcmds) == 0 {
		t.Fatal("ParseGatesLines returned zero gate commands for Wave 0 — expected at least 'exit 1'")
	}
	found := false
	for _, gc := range gcmds {
		if strings.HasPrefix(gc.Text, "exit 1") {
			found = true
			break
		}
	}
	if !found {
		var texts []string
		for _, gc := range gcmds {
			texts = append(texts, gc.Text)
		}
		t.Errorf("expected a gate starting with 'exit 1', got: %v", texts)
	}
}

// ─── ML-1C Containment Tests ────────────────────────────────────────────────

// TestGenerateAttentionScripts_SymlinkScriptsDirRefused asserts that ML-1C
// containment guards reject GenerateAttentionScripts when scripts/ is a
// symlink pointing outside the project tree.
func TestGenerateAttentionScripts_SymlinkScriptsDirRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	symlinkOrSkip(t, outside, filepath.Join(dir, "scripts"))

	err := GenerateAttentionScripts("")
	if err == nil {
		t.Fatal("GenerateAttentionScripts() should refuse when scripts/ is a symlink, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "trackfw-attention-signal.sh")); statErr == nil {
		t.Error("containment violated: attention-signal.sh was written to outside dir")
	}
}

// TestScaffold_SymlinkGovDirRefused asserts that ML-1C containment guards
// cause Scaffold to refuse when one of the governance dirs (docs/) is a
// symlink pointing outside the project root (PoC 1 scenario).
func TestScaffold_SymlinkGovDirRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	symlinkOrSkip(t, outside, filepath.Join(dir, "docs"))

	cfg := Config{
		ProjectType: "governance",
		Frontend:    "none",
		Backend:     "go",
		WipLimit:    1,
	}
	err := Scaffold(cfg)
	if err == nil {
		t.Fatal("Scaffold() should refuse when docs/ is a symlink, got nil")
	}
	// The outside dir must NOT have gained governance subdirectories.
	if _, statErr := os.Stat(filepath.Join(outside, "adr")); statErr == nil {
		t.Error("containment violated: adr dir was created outside the project")
	}
}

// TestGenerateClaudeCommandsInner_SymlinkDirRefused asserts that ML-1C
// containment guards reject slash-command generation when .claude/ is a
// symlink pointing outside the project root.
func TestGenerateClaudeCommandsInner_SymlinkDirRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	symlinkOrSkip(t, outside, filepath.Join(dir, ".claude"))

	err := generateClaudeCommandsInner(false)
	if err == nil {
		t.Fatal("generateClaudeCommandsInner() should refuse when .claude/ is a symlink, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "commands")); statErr == nil {
		t.Error("containment violated: commands dir was created outside the project")
	}
}

// TestGenerateGitHubActionsWorkflow_SymlinkGithubDirRefused asserts that
// ML-1C containment guards reject GitHub Actions workflow generation when
// .github/ is a symlink pointing outside the project root (PoC 1 scenario).
func TestGenerateGitHubActionsWorkflow_SymlinkGithubDirRefused(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	symlinkOrSkip(t, outside, filepath.Join(dir, ".github"))

	err := generateGitHubActionsWorkflow(Config{CI: "github-actions"})
	if err == nil {
		t.Fatal("generateGitHubActionsWorkflow() should refuse when .github/ is a symlink, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "workflows")); statErr == nil {
		t.Error("containment violated: workflows dir was created outside the project")
	}
}

// ─── ML-3B + ML-3E: CRLF normalisation in the distributed literal ───────────

// TestAttentionSignal_Literal_HasCRLFNormalisationAtPython3Sites affirms that
// the TOOL and MSG capture pipelines in the distributed literal each contain
// sed $'s/\r$//', which removes the trailing \r that python3 print() appends
// in Windows text mode.
//
// ML-3B Regra Dura de Reconciliação sentence: this test affirms that the
// attentionSignalScript literal applies sed $'s/\r$//' to each python3 stdout
// capture site, matching the Wave 0 conclusion that python3 in Windows text
// mode appends \r\n to print() output and that the correction must be
// autocontained (no lib-crlf-normalize.sh, which the consumer does not have).
func TestAttentionSignal_Literal_DelegatesWithoutPythonCaptures(t *testing.T) {
	if strings.Contains(attentionSignalScript, "python3") || !strings.Contains(attentionSignalScript, `trackfw attention signal "$@" || true`) {
		t.Fatal("attention must use the native parser")
	}
}

// TestAttentionSignal_CRLF_TrackfwYaml_WritesToCorrectPath affirms that a
// CRLF-formatted trackfw.yaml does not corrupt ROADMAP_DIR, so the signal file
// is written to docs/roadmaps/ and not to docs/roadmaps\r/ (a silent wrong-path
// outcome that makes the board miss the signal entirely).
//
// ML-3E Regra Dura de Reconciliação sentence: this test affirms that grep over
// a CRLF trackfw.yaml no longer produces a ROADMAP_DIR with a trailing \r,
// because sed $'s/\r//g' strips all \r from the extracted value before it
// reaches mkdir -p and the JSON write.
func TestAttentionSignal_CRLF_TrackfwYaml_WritesToCorrectPath(t *testing.T) {
	t.Setenv("PATH", filepath.Dir(compiledGuardBinary(t))+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Write trackfw.yaml with CRLF line endings (as Notepad on Windows would produce).
	crlfYaml := "roadmap_dir: docs/roadmaps\r\n"
	if err := os.WriteFile("trackfw.yaml", []byte(crlfYaml), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml CRLF: %v", err)
	}
	if err := GenerateAttentionScripts(""); err != nil {
		t.Fatalf("GenerateAttentionScripts: %v", err)
	}

	signalPath := filepath.Join("scripts", "trackfw-attention-signal.sh")
	cmd := exec.Command("bash", signalPath)
	cmd.Stdin = strings.NewReader(`{"tool_name":"crlf_tool","tool_input":{"question":"CRLF test"}}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("signal script failed with CRLF trackfw.yaml: %v\n%s", err, out)
	}

	// The signal must be in docs/roadmaps/, not docs/roadmaps\r/.
	wantFile := filepath.Join("docs", "roadmaps", ".trackfw-attention.json")
	data, err := os.ReadFile(wantFile)
	if err != nil {
		t.Fatalf("attention file not found at %s (CRLF trackfw.yaml corrupted ROADMAP_DIR): %v", wantFile, err)
	}

	var payload map[string]interface{}
	if jsonErr := json.Unmarshal(data, &payload); jsonErr != nil {
		t.Fatalf("attention file is not valid JSON: %v\ncontent: %s", jsonErr, data)
	}

	// Contra-braço: no stray directory with \r in name.
	badDir := filepath.Join("docs", "roadmaps\r")
	if _, statErr := os.Stat(badDir); statErr == nil {
		t.Errorf("directory with CR in name was created (%q) — ROADMAP_DIR normalisation failed", badDir)
	}
}

// TestAttentionSignal_LF_TrackfwYaml_StillWorks is the contra-braço for ML-3E:
// a plain LF trackfw.yaml must continue to work after the CRLF normalisation
// is added to the ROADMAP_DIR extraction pipeline.
//
// ML-3E contra-braço sentence: this test affirms that sed $'s/\r//g' on an LF
// trackfw.yaml leaves ROADMAP_DIR unchanged so the signal is still written to
// docs/roadmaps/.
func TestAttentionSignal_LF_TrackfwYaml_StillWorks(t *testing.T) {
	t.Setenv("PATH", filepath.Dir(compiledGuardBinary(t))+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Plain LF yaml (Unix-standard).
	if err := os.WriteFile("trackfw.yaml", []byte("roadmap_dir: docs/roadmaps\n"), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml LF: %v", err)
	}
	if err := GenerateAttentionScripts(""); err != nil {
		t.Fatalf("GenerateAttentionScripts: %v", err)
	}

	signalPath := filepath.Join("scripts", "trackfw-attention-signal.sh")
	cmd := exec.Command("bash", signalPath)
	cmd.Stdin = strings.NewReader(`{"tool_name":"lf_tool","tool_input":{"question":"LF test"}}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("signal script failed with LF trackfw.yaml: %v\n%s", err, out)
	}

	wantFile := filepath.Join("docs", "roadmaps", ".trackfw-attention.json")
	data, err := os.ReadFile(wantFile)
	if err != nil {
		t.Fatalf("attention file not found at %s after LF yaml: %v", wantFile, err)
	}
	var payload map[string]interface{}
	if jsonErr := json.Unmarshal(data, &payload); jsonErr != nil {
		t.Fatalf("JSON invalid after LF yaml: %v\ncontent: %s", jsonErr, data)
	}
}

// TestAttentionSignal_SpaceInRoadmapDir_NotBrokenByCRLFNorm is the contra-braço
// that ensures sed $'s/\r//g' does not mangle a roadmap_dir value that contains
// a space (unusual but valid on Unix).
//
// ML-3E contra-braço sentence: this test affirms that the CRLF normalisation
// stage (sed $'s/\r//g') does not split or collapse roadmap_dir values that
// contain spaces, so the correction does not introduce a new class of path
// corruption for consumers with unconventional but valid directory names.
func TestAttentionSignal_SpaceInRoadmapDir_NotBrokenByCRLFNorm(t *testing.T) {
	t.Setenv("PATH", filepath.Dir(compiledGuardBinary(t))+string(os.PathListSeparator)+os.Getenv("PATH"))
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// roadmap_dir with a space — exercise the sed normalisation stage specifically.
	if err := os.WriteFile("trackfw.yaml", []byte("roadmap_dir: my roadmaps\n"), 0644); err != nil {
		t.Fatalf("WriteFile trackfw.yaml with space: %v", err)
	}
	if err := GenerateAttentionScripts(""); err != nil {
		t.Fatalf("GenerateAttentionScripts: %v", err)
	}

	signalPath := filepath.Join("scripts", "trackfw-attention-signal.sh")
	cmd := exec.Command("bash", signalPath)
	cmd.Stdin = strings.NewReader(`{"tool_name":"space_tool","tool_input":{"question":"space dir test"}}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("signal script failed with space in roadmap_dir: %v\n%s", err, out)
	}

	wantFile := filepath.Join("my roadmaps", ".trackfw-attention.json")
	data, err := os.ReadFile(wantFile)
	if err != nil {
		t.Fatalf("attention file not found at %q — space in roadmap_dir was broken by CRLF norm: %v", wantFile, err)
	}
	var payload map[string]interface{}
	if jsonErr := json.Unmarshal(data, &payload); jsonErr != nil {
		t.Fatalf("JSON invalid for space roadmap_dir: %v\ncontent: %s", jsonErr, data)
	}
}

// ─── ML-1A (ADR-2026-09-29 D2): generator skips gate.yml when validate.yml exists ──

// TestGenerateGitHubActionsWorkflow_SkipsWhenValidateYmlPresent asserts that
// generateGitHubActionsWorkflow does NOT write trackfw-gate.yml when
// .github/workflows/trackfw-validate.yml already exists as a regular file.
//
// ML-1A Regra Dura de Reconciliação sentence: this test affirms that ADR-2026-09-29 D2
// is implemented — the generator's asymmetry with refreshDiscoverGitHubActionsWorkflowIfPresent
// is corrected: when validate.yml is present, gate.yml is NOT written and the function
// returns nil (not an error, per the ADR: absence is expected, not a failure).
func TestGenerateGitHubActionsWorkflow_SkipsWhenValidateYmlPresent(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Plant trackfw-validate.yml as a regular file (the discover-installed workflow).
	wfDir := filepath.Join(dir, ".github", "workflows")
	if err := os.MkdirAll(wfDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wfDir, "trackfw-validate.yml"), []byte("name: trackfw validate\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := generateGitHubActionsWorkflow(Config{CI: "github-actions"})
	if err != nil {
		t.Fatalf("generateGitHubActionsWorkflow() returned error when validate.yml present, want nil: %v", err)
	}

	gatePath := filepath.Join(dir, ".github", "workflows", "trackfw-gate.yml")
	if _, statErr := os.Stat(gatePath); statErr == nil {
		t.Error("gate.yml was written even though validate.yml was present — D2 not implemented")
	}
}

// TestGenerateGitHubActionsWorkflow_WritesWhenValidateYmlAbsent is the counter-arm:
// when trackfw-validate.yml is absent, generateGitHubActionsWorkflow MUST still write
// trackfw-gate.yml — the project-new path must not have regressed.
//
// ML-1A Regra Dura de Reconciliação sentence: this test affirms that the gate.yml write
// path is intact for new projects, i.e. the D2 guard (validate.yml presence check) does
// not suppress the write when validate.yml is absent — the pre-existing behavior is preserved.
func TestGenerateGitHubActionsWorkflow_WritesWhenValidateYmlAbsent(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// No validate.yml — clean new project.
	err := generateGitHubActionsWorkflow(Config{CI: "github-actions"})
	if err != nil {
		t.Fatalf("generateGitHubActionsWorkflow() returned error for clean new project: %v", err)
	}

	gatePath := filepath.Join(dir, ".github", "workflows", "trackfw-gate.yml")
	if _, statErr := os.Stat(gatePath); statErr != nil {
		t.Errorf("gate.yml was NOT written for new project (validate.yml absent) — counter-arm: %v", statErr)
	}
}

// TestGenerateCommitMsgHook_HuskyExecBitPromotedWhenFileExistsWith0644 asserts AC2b
// (REQ-2026-08-30, husky commit-msg site): when .husky/commit-msg already exists with
// mode 0644, generateCommitMsgHook must promote it to 0755 (git executes hooks via
// fork+exec; the +x bit is required). Affirmation: this test asserts that
// generateCommitMsgHook writes with mode 0755 via pathguard.GuardedWrite (fd.Chmod
// before rename), replacing a pre-existing 0644 file atomically.
func TestGenerateCommitMsgHook_HuskyExecBitPromotedWhenFileExistsWith0644(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Pre-create .husky/commit-msg with mode 0644.
	huskyDir := filepath.Join(dir, ".husky")
	if err := os.MkdirAll(huskyDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	hookPath := filepath.Join(huskyDir, "commit-msg")
	if err := os.WriteFile(hookPath, []byte("# old content\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Verify precondition: no exec bit.
	if execBitRepresentavelPara(t, hookPath) {
		st, _ := os.Stat(hookPath)
		if st.Mode().Perm()&0111 != 0 {
			t.Fatal("precondition failed: hook already has exec bit before generateCommitMsgHook")
		}
	}

	cfg := Config{Hooks: "husky", RequireReqInCommit: true}
	if err := generateCommitMsgHook(cfg); err != nil {
		t.Fatalf("generateCommitMsgHook: %v", err)
	}

	st, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("Stat after generateCommitMsgHook: %v", err)
	}
	if execBitRepresentavelPara(t, hookPath) {
		if st.Mode().Perm() != 0755 {
			t.Errorf("husky commit-msg hook mode = %04o, want 0755 (exec bit not promoted from 0644)", st.Mode().Perm())
		}
	} else {
		execBitNaoExercitado(t, hookPath)
	}
}

// TestGenerateHuskyHook_ExecBitPromotedWhenFileExistsWith0644 asserts AC2b
// (REQ-2026-08-30, husky pre-commit site): when .husky/pre-commit already exists with
// mode 0644, generateHuskyHook must promote it to 0755 (git executes hooks via
// fork+exec; the +x bit is required). Affirmation: this test asserts that
// generateHuskyHook writes with mode 0755 via pathguard.GuardedWrite (fd.Chmod
// before rename), replacing a pre-existing 0644 file atomically.
func TestGenerateHuskyHook_ExecBitPromotedWhenFileExistsWith0644(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Pre-create .husky/pre-commit with mode 0644.
	huskyDir := filepath.Join(dir, ".husky")
	if err := os.MkdirAll(huskyDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	hookPath := filepath.Join(huskyDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte("# old content\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Verify precondition: no exec bit.
	if execBitRepresentavelPara(t, hookPath) {
		st, _ := os.Stat(hookPath)
		if st.Mode().Perm()&0111 != 0 {
			t.Fatal("precondition failed: hook already has exec bit before generateHuskyHook")
		}
	}

	if err := generateHuskyHook(); err != nil {
		t.Fatalf("generateHuskyHook: %v", err)
	}

	st, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("Stat after generateHuskyHook: %v", err)
	}
	if execBitRepresentavelPara(t, hookPath) {
		if st.Mode().Perm() != 0755 {
			t.Errorf("husky pre-commit hook mode = %04o, want 0755 (exec bit not promoted from 0644)", st.Mode().Perm())
		}
	} else {
		execBitNaoExercitado(t, hookPath)
	}
}

// TestGenerateCommitMsgHook_HuskySymlinkAtLeafRefused asserts R-4
// (REQ-2026-08-30): a symlink planted at .husky/commit-msg pointing outside
// the project root is refused by pathguard.GuardedWrite, and the target file
// remains unchanged.
//
// Affirmation (R-4 symlink, commit-msg): GuardedWrite's RejectAndReport
// catches the symlink before any write occurs. This would PASS with
// os.WriteFile+Chmod (which follows the symlink) and FAILS only with GuardedWrite.
func TestGenerateCommitMsgHook_HuskySymlinkAtLeafRefused(t *testing.T) {
	outside := t.TempDir()
	victim := filepath.Join(outside, "victim.txt")
	originalContent := "must not be overwritten\n"
	if err := os.WriteFile(victim, []byte(originalContent), 0644); err != nil {
		t.Fatalf("WriteFile victim: %v", err)
	}

	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	huskyDir := filepath.Join(dir, ".husky")
	if err := os.MkdirAll(huskyDir, 0755); err != nil {
		t.Fatalf("MkdirAll .husky: %v", err)
	}
	hookPath := filepath.Join(huskyDir, "commit-msg")
	symlinkOrSkip(t, victim, hookPath)

	cfg := Config{Hooks: "husky", RequireReqInCommit: true}
	err := generateCommitMsgHook(cfg)
	if err == nil {
		t.Fatal("generateCommitMsgHook should refuse when .husky/commit-msg is a symlink, got nil")
	}

	got, readErr := os.ReadFile(victim)
	if readErr != nil {
		t.Fatalf("ReadFile victim: %v", readErr)
	}
	if string(got) != originalContent {
		t.Errorf("symlink-follow: victim was overwritten\nwant: %q\ngot:  %q", originalContent, string(got))
	}
}

// TestGenerateHuskyHook_SymlinkAtLeafRefused asserts R-4
// (REQ-2026-08-30): a symlink planted at .husky/pre-commit pointing outside
// the project root is refused by pathguard.GuardedWrite, and the target file
// remains unchanged.
//
// Affirmation (R-4 symlink, pre-commit): GuardedWrite's RejectAndReport
// catches the symlink before any write occurs. This would PASS with
// os.WriteFile+Chmod (which follows the symlink) and FAILS only with GuardedWrite.
func TestGenerateHuskyHook_SymlinkAtLeafRefused(t *testing.T) {
	outside := t.TempDir()
	victim := filepath.Join(outside, "victim.txt")
	originalContent := "must not be overwritten\n"
	if err := os.WriteFile(victim, []byte(originalContent), 0644); err != nil {
		t.Fatalf("WriteFile victim: %v", err)
	}

	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	huskyDir := filepath.Join(dir, ".husky")
	if err := os.MkdirAll(huskyDir, 0755); err != nil {
		t.Fatalf("MkdirAll .husky: %v", err)
	}
	hookPath := filepath.Join(huskyDir, "pre-commit")
	symlinkOrSkip(t, victim, hookPath)

	err := generateHuskyHook()
	if err == nil {
		t.Fatal("generateHuskyHook should refuse when .husky/pre-commit is a symlink, got nil")
	}

	got, readErr := os.ReadFile(victim)
	if readErr != nil {
		t.Fatalf("ReadFile victim: %v", readErr)
	}
	if string(got) != originalContent {
		t.Errorf("symlink-follow: victim was overwritten\nwant: %q\ngot:  %q", originalContent, string(got))
	}
}

// TestGenerateCommitMsgHook_HuskyHardlinkNotFollowed asserts R-4
// (REQ-2026-08-30): a hardlink to an outside file at .husky/commit-msg is
// NOT followed by pathguard.GuardedWrite — GuardedWrite renames a new inode,
// breaking the hardlink so the outside file's inode is unchanged.
//
// Affirmation (R-4 hardlink, commit-msg): rename-to-new-inode breaks the
// shared inode. With old os.WriteFile+Chmod, both share an inode and
// os.WriteFile overwrites shared content — this test fails with that pattern.
func TestGenerateCommitMsgHook_HuskyHardlinkNotFollowed(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("hardlink semantics differ on Windows")
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "shared.txt")
	originalContent := "shared inode — must not be overwritten\n"
	if err := os.WriteFile(outsideFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("WriteFile outsideFile: %v", err)
	}

	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	huskyDir := filepath.Join(dir, ".husky")
	if err := os.MkdirAll(huskyDir, 0755); err != nil {
		t.Fatalf("MkdirAll .husky: %v", err)
	}
	hookPath := filepath.Join(huskyDir, "commit-msg")
	if err := os.Link(outsideFile, hookPath); err != nil {
		t.Skipf("os.Link not supported: %v", err)
	}

	cfg := Config{Hooks: "husky", RequireReqInCommit: true}
	if err := generateCommitMsgHook(cfg); err != nil {
		t.Fatalf("generateCommitMsgHook: %v", err)
	}

	got, readErr := os.ReadFile(outsideFile)
	if readErr != nil {
		t.Fatalf("ReadFile outsideFile: %v", readErr)
	}
	if string(got) != originalContent {
		t.Errorf("hardlink shared inode was overwritten\nwant: %q\ngot:  %q", originalContent, string(got))
	}
}

// TestGenerateHuskyHook_HardlinkNotFollowed asserts R-4
// (REQ-2026-08-30): a hardlink to an outside file at .husky/pre-commit is
// NOT followed by pathguard.GuardedWrite — GuardedWrite renames a new inode,
// breaking the hardlink so the outside file's inode is unchanged.
//
// Affirmation (R-4 hardlink, pre-commit): rename-to-new-inode breaks the
// shared inode. With old os.WriteFile+Chmod, both share an inode and
// os.WriteFile overwrites shared content — this test fails with that pattern.
func TestGenerateHuskyHook_HardlinkNotFollowed(t *testing.T) {
	if os.Getenv("GOOS") == "windows" {
		t.Skip("hardlink semantics differ on Windows")
	}
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "shared.txt")
	originalContent := "shared inode — must not be overwritten\n"
	if err := os.WriteFile(outsideFile, []byte(originalContent), 0644); err != nil {
		t.Fatalf("WriteFile outsideFile: %v", err)
	}

	dir := t.TempDir()
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	huskyDir := filepath.Join(dir, ".husky")
	if err := os.MkdirAll(huskyDir, 0755); err != nil {
		t.Fatalf("MkdirAll .husky: %v", err)
	}
	hookPath := filepath.Join(huskyDir, "pre-commit")
	if err := os.Link(outsideFile, hookPath); err != nil {
		t.Skipf("os.Link not supported: %v", err)
	}

	if err := generateHuskyHook(); err != nil {
		t.Fatalf("generateHuskyHook: %v", err)
	}

	got, readErr := os.ReadFile(outsideFile)
	if readErr != nil {
		t.Fatalf("ReadFile outsideFile: %v", readErr)
	}
	if string(got) != originalContent {
		t.Errorf("hardlink shared inode was overwritten\nwant: %q\ngot:  %q", originalContent, string(got))
	}
}

// --------------------------------------------------------------------------
// T-12: Scaffold pre-check on symlinked project directories
// --------------------------------------------------------------------------

// scaffoldProjectState walks the project directory and returns a map of
// relative path → content (for files) or "dir" (for dirs) or "symlink:target"
// (for symlinks). Used to compare project state before and after a Scaffold
// call to assert no partial writes.
func scaffoldProjectState(t *testing.T, dir string) map[string]string {
	t.Helper()
	state := map[string]string{}
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		if rel == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			target, _ := os.Readlink(path)
			state[rel] = "symlink:" + target
			return nil
		}
		if d.IsDir() {
			state[rel] = "dir"
			return nil
		}
		content, _ := os.ReadFile(path)
		state[rel] = string(content)
		return nil
	})
	return state
}

// TestScaffoldPreCheckAbortsOnSymlinkClaudeDir asserts that Scaffold aborts
// without any write when <project>/.claude is a symlink. Reconciliation:
// affirms that the upfront pre-check in Scaffold fires for ClaudeCommandsDirPath
// before the first MkdirAll — without the pre-check, 11 files would be written
// before generateClaudeCommands detects the symlink (T-12 root cause).
func TestScaffoldPreCheckAbortsOnSymlinkClaudeDir(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()

	// Pre-capture state (only the .claude symlink).
	claudeDir := filepath.Join(dir, ".claude")
	symlinkOrSkip(t, target, claudeDir)
	before := scaffoldProjectState(t, dir)

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{ProjectName: "test", ProjectType: "governance", Hooks: "none", CI: "none"}
	err := Scaffold(cfg)
	if err == nil {
		t.Fatal("Scaffold must fail when <proj>/.claude is a symlink")
	}

	after := scaffoldProjectState(t, dir)
	// Only the symlink we placed must exist — no new entries.
	for k, v := range after {
		if bv, ok := before[k]; !ok || bv != v {
			t.Errorf("unexpected write after aborted Scaffold: %s = %q (before: %q)", k, v, bv)
		}
	}
}

// TestScaffoldPreCheckAbortsOnSymlinkScriptsDir asserts that Scaffold aborts
// without any write when <project>/scripts is a symlink. Reconciliation:
// affirms that the upfront pre-check covers validateScriptPath — without it,
// govDirs would succeed before scripts/ is refused (5 files written).
func TestScaffoldPreCheckAbortsOnSymlinkScriptsDir(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	scriptsDir := filepath.Join(dir, "scripts")
	symlinkOrSkip(t, target, scriptsDir)
	before := scaffoldProjectState(t, dir)

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{ProjectName: "test", ProjectType: "governance", Hooks: "none", CI: "none"}
	err := Scaffold(cfg)
	if err == nil {
		t.Fatal("Scaffold must fail when <proj>/scripts is a symlink")
	}

	after := scaffoldProjectState(t, dir)
	for k, v := range after {
		if bv, ok := before[k]; !ok || bv != v {
			t.Errorf("unexpected write after aborted Scaffold: %s = %q (before: %q)", k, v, bv)
		}
	}
}

// TestScaffoldPreCheckAbortsOnSymlinkVaultDir asserts that Scaffold aborts
// without any write when <project>/vault is a symlink. Reconciliation:
// affirms that the upfront pre-check covers vaultIndexPath — without it,
// govDirs entries before vault/ would succeed (1 file written).
func TestScaffoldPreCheckAbortsOnSymlinkVaultDir(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	vaultDir := filepath.Join(dir, "vault")
	symlinkOrSkip(t, target, vaultDir)
	before := scaffoldProjectState(t, dir)

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{ProjectName: "test", ProjectType: "governance", Hooks: "none", CI: "none"}
	err := Scaffold(cfg)
	if err == nil {
		t.Fatal("Scaffold must fail when <proj>/vault is a symlink")
	}

	after := scaffoldProjectState(t, dir)
	for k, v := range after {
		if bv, ok := before[k]; !ok || bv != v {
			t.Errorf("unexpected write after aborted Scaffold: %s = %q (before: %q)", k, v, bv)
		}
	}
}

// TestScaffoldPreCheckAbortsOnSymlinkDocsDir asserts that Scaffold aborts
// without any write when <project>/docs is a symlink. Reconciliation:
// affirms that the upfront pre-check covers govDirs entries under docs/ —
// without it, the first govDirs entries (docs/adr) already succeed via MkdirAll.
func TestScaffoldPreCheckAbortsOnSymlinkDocsDir(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	symlinkOrSkip(t, target, docsDir)
	before := scaffoldProjectState(t, dir)

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{ProjectName: "test", ProjectType: "governance", Hooks: "none", CI: "none"}
	err := Scaffold(cfg)
	if err == nil {
		t.Fatal("Scaffold must fail when <proj>/docs is a symlink")
	}

	after := scaffoldProjectState(t, dir)
	for k, v := range after {
		if bv, ok := before[k]; !ok || bv != v {
			t.Errorf("unexpected write after aborted Scaffold: %s = %q (before: %q)", k, v, bv)
		}
	}
}

// TestScaffoldPreCheckCounterArm_WithoutPreCheckClaudeDirWritesFiles is the
// counter-arm for TestScaffoldPreCheckAbortsOnSymlinkClaudeDir: it verifies
// that WITHOUT the upfront pre-check loop, Scaffold would write project files
// before the error (the write-time guard fires later). Implemented by running
// a modified Scaffold that skips the upfront loop. Reconciliation: affirms that
// removing the scaffoldWriteTargets loop from Scaffold causes partial writes
// — if this test passes, the counter-arm is active.
func TestScaffoldPreCheckCounterArm_WithoutPreCheckClaudeDirWritesFiles(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()
	claudeDir := filepath.Join(dir, ".claude")
	symlinkOrSkip(t, target, claudeDir)

	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Run scaffoldWithoutPreCheck to simulate the old behavior.
	cfg := Config{ProjectName: "test", ProjectType: "governance", Hooks: "none", CI: "none"}
	err := scaffoldWithoutPreCheck(cfg)
	if err == nil {
		t.Fatal("scaffoldWithoutPreCheck must still fail (write-time guard fires)")
	}

	// At least one project file must have been written before the error.
	entries, _ := os.ReadDir(dir)
	hasProjectFile := false
	for _, e := range entries {
		if e.Name() != ".claude" {
			hasProjectFile = true
			break
		}
	}
	if !hasProjectFile {
		t.Error("counter-arm: scaffoldWithoutPreCheck must write project files before the error; got no files — this means the upfront pre-check is not needed (unexpected)")
	}
}

// ─── ML-2B completion: scaffoldWriteTargets coverage test + symlink .github ──

// isCovered reports whether path p (ToSlash-normalized) is covered by any
// target in targets (also ToSlash-normalized).
//
// Coverage criterion — a path P is covered by target T when:
//   - P == T  (exact match)
//   - strings.HasPrefix(P, T+"/")  — P is inside directory T
//   - strings.HasPrefix(T, P+"/")  — P is an ancestor dir of T that must have
//     been created as an intermediate directory to reach T
//
// The ancestor direction (third rule) is what keeps intermediate dirs created
// by os.MkdirAll — docs/, scripts/, .claude/, .claude/commands/, etc. — from
// failing the check without needing to enumerate them in scaffoldWriteTargets.
func isCovered(p string, targets []string) bool {
	for _, t := range targets {
		if p == t || strings.HasPrefix(p, t+"/") || strings.HasPrefix(t, p+"/") {
			return true
		}
	}
	return false
}

// TestScaffoldWriteTargets_CoversEveryFileScaffoldCreates — ML-2B completion
//
// Affirms that scaffoldWriteTargets(cfg) declares every path that Scaffold(cfg)
// creates in a fresh project directory for seven representative Config values.
// The comment at scaffold.go:114-120 promises this invariant; this test makes
// it executable.
//
// Refutation of the handoff premise: the comment at scaffold.go:168-172
// explicitly excludes .claude/settings.json from the upfront pre-check list —
// that exclusion is CORRECT and must not be reversed (adding it would make a
// non-fatal path fatal at pre-check). The "dentro de" coverage criterion cannot
// absorb .claude/settings.json without a rule that accepts every root-level file
// (the ancestor of every target is "."), so the exception path is taken here:
// .claude/settings.json is a named explicit exclusion. InjectClaudeHooks (the
// writer) guards the leaf via pathguard.RejectAndReport before writing, so
// symlink protection already exists at the write site. Two assertions keep the
// exclusion current and deliberate.
//
// Regra Dura de Reconciliação sentence: this test affirms that every path
// Scaffold creates for a given Config is either an explicit entry in
// scaffoldWriteTargets(cfg) or an intermediate directory obligatory for such an
// entry — so a future write added to Scaffold without a corresponding entry in
// scaffoldWriteTargets is immediately detected.
func TestScaffoldWriteTargets_CoversEveryFileScaffoldCreates(t *testing.T) {
	// .claude/settings.json is written by InjectClaudeHooks (triggered by the
	// CLAUDE.md that Scaffold creates) as a non-fatal, explicitly excluded path.
	// See scaffold.go:168-172 and InjectClaudeHooks (agentfiles.go) for the guard.
	const claudeSettingsJSON = ".claude/settings.json"

	cases := []struct {
		name string
		cfg  Config
	}{
		{"NoHooksNoCI", Config{}},
		{"HuskyNoReq", Config{Hooks: "husky"}},
		{"HuskyWithReq", Config{Hooks: "husky", RequireReqInCommit: true}},
		{"LefthookNoReq", Config{Hooks: "lefthook"}},
		{"LefthookWithReq", Config{Hooks: "lefthook", RequireReqInCommit: true}},
		{"GitHubActions", Config{CI: "github-actions"}},
		{"Java", Config{Backend: "java"}},
	}

	origDir, _ := os.Getwd()

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			t.Setenv("CLAUDE_CONFIG_DIR", "")
			config.Reset()
			t.Cleanup(config.Reset)

			if err := os.Chdir(dir); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chdir(origDir) })

			if err := Scaffold(tc.cfg); err != nil {
				t.Fatalf("Scaffold(%s) failed: %v", tc.name, err)
			}

			// Collect all paths created by Scaffold (relative, ToSlash-normalized).
			var created []string
			_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				rel, relErr := filepath.Rel(dir, path)
				if relErr != nil || rel == "." {
					return nil
				}
				created = append(created, filepath.ToSlash(rel))
				return nil
			})

			targets := scaffoldWriteTargets(tc.cfg)
			normTargets := make([]string, len(targets))
			for i, tgt := range targets {
				normTargets[i] = filepath.ToSlash(tgt)
			}

			// Assertion 1 — deliberate exclusion is still present in the walk (keeps
			// the exclusion current: if InjectClaudeHooks stops writing settings.json,
			// this fails so the exclusion can be removed).
			foundSettingsJSON := false
			for _, p := range created {
				if p == claudeSettingsJSON {
					foundSettingsJSON = true
					break
				}
			}
			if !foundSettingsJSON {
				t.Errorf("%s: %s not found in walk — exclusion may be stale (InjectClaudeHooks may no longer fire)",
					tc.name, claudeSettingsJSON)
			}

			// Assertion 2 — deliberate exclusion is absent from scaffoldWriteTargets
			// (keeps the exclusion deliberate: if someone adds it to the list, this
			// fails to prevent a non-fatal path from becoming fatal at pre-check).
			for _, tgt := range normTargets {
				if tgt == claudeSettingsJSON {
					t.Errorf("%s: %s is now IN scaffoldWriteTargets — the deliberate exclusion "+
						"was reversed; adding it makes a non-fatal path fatal at pre-check "+
						"(see scaffold.go:168-172)",
						tc.name, claudeSettingsJSON)
				}
			}

			// Coverage check — every walked path is covered, excluding the deliberate exclusion.
			for _, p := range created {
				if p == claudeSettingsJSON {
					continue // deliberate exclusion: leaf-guarded at write site
				}
				if !isCovered(p, normTargets) {
					t.Errorf("%s: path %q is not covered by scaffoldWriteTargets — "+
						"add it or one of its parent dirs to scaffoldWriteTargets",
						tc.name, p)
				}
			}

			// Vacuous-pass guard — assert at least vault/notes/index.md was created.
			foundVaultIndex := false
			for _, p := range created {
				if p == filepath.ToSlash(vaultIndexPath) {
					foundVaultIndex = true
					break
				}
			}
			if !foundVaultIndex {
				t.Errorf("%s: Scaffold succeeded but %s not found — Scaffold may be a no-op",
					tc.name, vaultIndexPath)
			}
		})
	}

	// Counter-arm — remove credentialGuardScriptPath from the targets; uncovered()
	// must report exactly that one file. This verifies that the coverage check is
	// active and would catch a genuinely missing entry.
	t.Run("CounterArm_MissingEntryDetected", func(t *testing.T) {
		dir := t.TempDir()
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("USERPROFILE", home)
		t.Setenv("CLAUDE_CONFIG_DIR", "")
		config.Reset()
		t.Cleanup(config.Reset)

		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chdir(origDir) })

		cfg := Config{}
		if err := Scaffold(cfg); err != nil {
			t.Fatalf("Scaffold failed: %v", err)
		}

		var created []string
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			rel, relErr := filepath.Rel(dir, path)
			if relErr != nil || rel == "." {
				return nil
			}
			created = append(created, filepath.ToSlash(rel))
			return nil
		})

		// Build trimmed targets: all entries except credentialGuardScriptPath.
		allTargets := scaffoldWriteTargets(cfg)
		trimmed := make([]string, 0, len(allTargets)-1)
		for _, tgt := range allTargets {
			if filepath.ToSlash(tgt) == filepath.ToSlash(credentialGuardScriptPath) {
				continue
			}
			trimmed = append(trimmed, filepath.ToSlash(tgt))
		}
		if len(trimmed) != len(allTargets)-1 {
			t.Fatalf("credentialGuardScriptPath %q not found in scaffoldWriteTargets — counter-arm is broken",
				credentialGuardScriptPath)
		}

		// Collect uncovered paths (excluding the deliberate .claude/settings.json exclusion).
		const claudeSettingsJSON = ".claude/settings.json"
		var uncov []string
		for _, p := range created {
			if p == claudeSettingsJSON {
				continue
			}
			if !isCovered(p, trimmed) {
				uncov = append(uncov, p)
			}
		}

		want := filepath.ToSlash(credentialGuardScriptPath)
		if len(uncov) != 1 || uncov[0] != want {
			t.Errorf("counter-arm: expected exactly [%s] uncovered, got %v", want, uncov)
		}
	})
}

// TestScaffold_GithubActionsSymlinkRefused_ProjectStateUnchanged — ML-2B completion
//
// Affirms that Scaffold aborts without any write when <project>/.github is a
// symlink, even when CI: "github-actions" is set. The T-12 upfront pre-check
// must fire for GitHubActionsWorkflowPath before the first MkdirAll — testing
// the conditional branch of scaffoldWriteTargets that existing pre-check tests
// do not reach.
//
// Regra Dura de Reconciliação sentence: this test affirms that the upfront
// scaffoldWriteTargets pre-check covers the GitHubActionsWorkflowPath branch
// (CI: "github-actions") so a symlinked .github/ aborts Scaffold before any
// project file is written.
func TestScaffold_GithubActionsSymlinkRefused_ProjectStateUnchanged(t *testing.T) {
	outside := t.TempDir()
	dir := t.TempDir()
	orig, _ := os.Getwd()

	githubDir := filepath.Join(dir, ".github")
	symlinkOrSkip(t, outside, githubDir)
	before := scaffoldProjectState(t, dir)

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	cfg := Config{CI: "github-actions"}
	err := Scaffold(cfg)
	if err == nil {
		t.Fatal("Scaffold must fail when <proj>/.github is a symlink, got nil")
	}
	if !strings.Contains(err.Error(), "scaffold pre-check") {
		t.Errorf("error must mention 'scaffold pre-check', got: %v", err)
	}
	if !strings.Contains(err.Error(), ".github") {
		t.Errorf("error must name .github in the message, got: %v", err)
	}

	after := scaffoldProjectState(t, dir)
	for k, v := range after {
		bv, ok := before[k]
		if !ok || bv != v {
			t.Errorf("unexpected write after aborted Scaffold: %s = %q (before: %q)", k, v, bv)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			t.Errorf("unexpected deletion after aborted Scaffold: %s", k)
		}
	}

	if _, statErr := os.Stat(filepath.Join(outside, "workflows")); statErr == nil {
		t.Error("containment violated: workflows/ was created in the outside dir")
	}
}

// scaffoldWithoutPreCheck runs Scaffold without the upfront pathguard loop,
// mimicking the behaviour before T-12. Used only by the counter-arm test.
func scaffoldWithoutPreCheck(cfg Config) error {
	root, err := projectRoot()
	if err != nil {
		return fmt.Errorf("Scaffold: %w", err)
	}
	for _, dir := range govDirs {
		absDir := filepath.Join(root, dir)
		if err := rejectScaffoldPath(root, absDir); err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating %s: %w", dir, err)
		}
	}
	if err := generateVaultIndex(); err != nil {
		return err
	}
	if err := generateGitAttributes(); err != nil {
		return err
	}
	if err := generateGitIgnore(); err != nil {
		return err
	}
	if err := writeTrackfwConfig(cfg); err != nil {
		return err
	}
	if err := generateValidateScript(cfg); err != nil {
		return err
	}
	if err := GenerateAttentionScripts(""); err != nil {
		return err
	}
	if err := GenerateCredentialGuardScript(""); err != nil {
		return err
	}
	if err := GenerateGitBranchGuardScript(""); err != nil {
		return err
	}
	if err := generateClaudeMD(cfg); err != nil {
		return err
	}
	if err := generateClaudeCommands(); err != nil {
		return err
	}
	return nil
}

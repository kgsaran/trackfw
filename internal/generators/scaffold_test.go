package generators

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
		"```markdown\n   ---",
		"status: backlog",
		"date: <YYYY-MM-DD>",
		`req: "docs/req/<arquivo-selecionado>.md"`,
		`squad: ""`,
		"---\n\n   # Roadmap:",
		"> Created: <YYYY-MM-DD> | Status: backlog",
		"docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md",
		"Preencha `req:` com o caminho relativo completo da REQ selecionada",
		"### ML-1B — <título> (se independente de ML-1A)",
		"## Wave 2 — <nome> (depende de Wave 1)",
		"> Dependências: Wave 1 completa",
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
	if !strings.Contains(string(signalContent), "# trackfw attention signal — PreToolUse/BeforeTool hook") {
		t.Errorf("script signal não contém cabeçalho esperado: %s", string(signalContent))
	}

	cleanupContent, _ := os.ReadFile(cleanupPath)
	if !strings.Contains(string(cleanupContent), "# trackfw attention cleanup — PostToolUse/AfterTool hook") {
		t.Errorf("script cleanup não contém cabeçalho esperado: %s", string(cleanupContent))
	}
}

func TestAttentionScripts_ExecutionContract(t *testing.T) {
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
				if _, err := os.Stat(containedFile); err != nil {
					t.Errorf("Arquivo de atenção esperadamente contido não foi encontrado em %s para path %s: %v", containedFile, p, err)
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

	// Criar diretório temporário para PATH customizado sem jq
	fakeBinDir := t.TempDir()

	// Utilitários necessários para o script rodar sem jq: bash, python3, date, grep, sed, tr, mkdir, printf, cat, rm
	requiredBins := []string{"bash", "python3", "python", "date", "grep", "sed", "tr", "mkdir", "printf", "cat", "rm"}
	for _, bin := range requiredBins {
		binPath, err := exec.LookPath(bin)
		if err == nil {
			// Marginal corrigido: distingue "sem privilégio" (continua sem o
			// binário — setup best-effort) de "falhou por outro motivo" (falha
			// o teste, já que o source existe e o target não existe ainda).
			if symlinkErr := os.Symlink(binPath, filepath.Join(fakeBinDir, bin)); symlinkErr != nil {
				if !isSymlinkPrivilegeError(symlinkErr) {
					t.Fatalf("os.Symlink(%q, fakeBinDir/%s): %v", binPath, bin, symlinkErr)
				}
				// Sem privilégio: prossegue sem este binário no fakeBinDir.
			}
		}
	}

	signalPath := filepath.Join("scripts", "trackfw-attention-signal.sh")
	cmd := exec.Command("bash", signalPath)
	cmd.Env = []string{
		"PATH=" + fakeBinDir,
	}
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

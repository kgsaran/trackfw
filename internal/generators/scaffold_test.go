package generators

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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

	if err := generateAttentionScripts(); err != nil {
		t.Fatalf("generateAttentionScripts() erro: %v", err)
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
		// Verifica permissões de execução (no Unix 0755)
		if mode := info.Mode().Perm(); mode&0111 == 0 {
			t.Errorf("arquivo %s não tem permissão de execução (perm: %o)", p, mode)
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
		if err := generateAttentionScripts(); err != nil {
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
				if err := generateAttentionScripts(); err != nil {
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
		if err := generateAttentionScripts(); err != nil {
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
	if err := generateAttentionScripts(); err != nil {
		t.Fatalf("generateAttentionScripts erro: %v", err)
	}

	// Criar diretório temporário para PATH customizado sem jq
	fakeBinDir := t.TempDir()

	// Utilitários necessários para o script rodar sem jq: bash, python3, date, grep, sed, tr, mkdir, printf, cat, rm
	requiredBins := []string{"bash", "python3", "python", "date", "grep", "sed", "tr", "mkdir", "printf", "cat", "rm"}
	for _, bin := range requiredBins {
		path, err := exec.LookPath(bin)
		if err == nil {
			_ = os.Symlink(path, filepath.Join(fakeBinDir, bin))
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


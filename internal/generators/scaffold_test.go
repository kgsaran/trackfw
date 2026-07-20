package generators

import (
	"os"
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

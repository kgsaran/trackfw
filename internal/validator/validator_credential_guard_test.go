package validator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// guardEntryClaudeSettings monta um .claude/settings.json mínimo com uma entrada de
// credential-guard apontando para scriptCmd (valor bruto do campo "command").
func guardEntryClaudeSettings(scriptCmd string) string {
	return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {"command": "` + scriptCmd + `", "type": "command"}
        ]
      }
    ]
  }
}
`
}

// TestCredentialGuardHookResolvable_DisparaScriptAusente — a regra dispara quando existe uma
// entrada de guard mas o script referenciado não existe no caminho resolvido.
func TestCredentialGuardHookResolvable_DisparaScriptAusente(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh`))
	// Nota: scripts/trackfw-credential-guard.sh NÃO é criado — ausência proposital.

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "does not exist") || !hasViolation(msgs, ".claude/settings.json") {
		t.Errorf("esperado violation de script ausente, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_DisparaScriptNaoExecutavel — a regra dispara quando o script
// existe mas não tem permissão de execução.
func TestCredentialGuardHookResolvable_DisparaScriptNaoExecutavel(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh`))
	scriptPath := filepath.Join(dir, "scripts", "trackfw-credential-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0644); err != nil { // sem bit +x
		t.Fatalf("write script: %v", err)
	}

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "not executable") {
		t.Errorf("esperado violation de script não executável, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_NaoDisparaSemEntrada — ausência de entrada de guard é estado
// legítimo (guard global instalado) e nunca deve violar.
func TestCredentialGuardHookResolvable_NaoDisparaSemEntrada(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// .claude/settings.json só com attention-signal/cleanup, sem credential-guard — o próprio
	// arquivo real deste repositório está nesse estado (guard global instalado).
	writeFile(t, dir, ".claude/settings.json", `{
  "hooks": {
    "PostToolUse": [
      {"matcher": "AskUserQuestion", "hooks": [{"command": "scripts/trackfw-attention-cleanup.sh", "type": "command"}]}
    ],
    "PreToolUse": [
      {"matcher": "AskUserQuestion", "hooks": [{"command": "scripts/trackfw-attention-signal.sh", "type": "command"}]}
    ]
  }
}
`)

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado zero violations sem entrada de guard, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_NaoDisparaFormatoDesconhecido — um comando que referencia o
// script mas não casa nenhuma das 3 formas de prefixo conhecidas não deve gerar violação (não é
// função desta regra adivinhar wiring próprio do usuário).
func TestCredentialGuardHookResolvable_NaoDisparaFormatoDesconhecido(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`$SOME_OTHER_VAR/scripts/trackfw-credential-guard.sh`))

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado zero violations com formato de prefixo desconhecido, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_ResolveAspasLiteraisDoCodex — a forma do Codex
// ("$(git rev-parse --show-toplevel)/…" com aspas literais no valor) resolve corretamente.
func TestCredentialGuardHookResolvable_ResolveAspasLiteraisDoCodex(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	codexHooks := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": ".*", "hooks": [{"command": "\"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh\"", "type": "command"}]}
    ]
  }
}
`
	writeFile(t, dir, ".codex/hooks.json", codexHooks)

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "does not exist") || !hasViolation(msgs, ".codex/hooks.json") {
		t.Errorf("esperado violation resolvendo a forma do Codex, obteve: %v", msgs)
	}

	scriptPath := filepath.Join(dir, "scripts", "trackfw-credential-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	msgs, err = validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado zero violations com script existente e executável, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_ResolveCaminhoRelativoPuro — a forma do Cursor/Copilot/Kiro
// (caminho relativo puro, sem prefixo) resolve contra a raiz do projeto.
func TestCredentialGuardHookResolvable_ResolveCaminhoRelativoPuro(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".cursor/hooks.json", `{
  "version": 1,
  "hooks": {
    "beforeShellExecution": [{"command": "scripts/trackfw-credential-guard.sh"}]
  }
}
`)

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "does not exist") || !hasViolation(msgs, ".cursor/hooks.json") {
		t.Errorf("esperado violation resolvendo caminho relativo puro, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_Configuravel — a regra é configurável por rules: no
// trackfw.yaml (off/warning/error), com default error.
func TestCredentialGuardHookResolvable_Configuravel(t *testing.T) {
	buildDir := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh`))
		return dir
	}

	t.Run("default_error", func(t *testing.T) {
		dir := buildDir(t)
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, _, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if !hasViolation(violations, "trackfw-credential-guard.sh") {
			t.Errorf("sem config (default error) deve gerar violation, obteve: %v", violations)
		}
	})

	t.Run("warning", func(t *testing.T) {
		dir := buildDir(t)
		writeFile(t, dir, "trackfw.yaml", "rules:\n  credential_guard_hook_resolvable: warning\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "trackfw-credential-guard.sh") {
			t.Errorf("com rules:warning não deve haver violation, obteve: %v", violations)
		}
		if !hasWarning(warnings, "trackfw-credential-guard.sh") {
			t.Errorf("com rules:warning deve haver warning, obteve: %v", warnings)
		}
	})

	t.Run("off", func(t *testing.T) {
		dir := buildDir(t)
		writeFile(t, dir, "trackfw.yaml", "rules:\n  credential_guard_hook_resolvable: off\n")
		chdir(t, dir)
		t.Cleanup(config.Reset)

		violations, warnings, err := ValidateUnfiltered()
		if err != nil {
			t.Fatalf("ValidateUnfiltered() erro: %v", err)
		}
		if hasViolation(violations, "trackfw-credential-guard.sh") || hasWarning(warnings, "trackfw-credential-guard.sh") {
			t.Errorf("com rules:off não deve haver violation nem warning, obteve violations=%v warnings=%v", violations, warnings)
		}
	})
}

// TestCredentialGuardHookResolvable_ArquivoAusenteEhPulado — arquivo de hook que não existe é
// pulado em silêncio, não é violação.
func TestCredentialGuardHookResolvable_ArquivoAusenteEhPulado(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado zero violations sem nenhum arquivo de hook, obteve: %v", msgs)
	}
}

// kiroHooksFixture monta um .kiro/hooks/trackfw-attention.json com a forma real emitida por
// InjectKiroHooks (internal/generators/agentfiles.go:632-701) — inclui campos "name"/"description"
// que também mencionam "trackfw-credential-guard" (sem ".sh") ao lado da entrada real
// action.command "scripts/trackfw-credential-guard.sh", para provar que o walker por valor não
// gera falso positivo a partir desses campos vizinhos.
func kiroHooksFixture() string {
	return `{
  "version": "v1",
  "hooks": [
    {
      "name": "trackfw-credential-guard-pre",
      "description": "Blocks/warns on possible plaintext credential materialization before a shell command executes",
      "trigger": "PreToolUse",
      "matcher": "shell",
      "action": {"type": "command", "command": "scripts/trackfw-credential-guard.sh"}
    },
    {
      "name": "trackfw-credential-guard-post",
      "description": "Warns on possible plaintext credential materialization after a shell command executes",
      "trigger": "PostToolUse",
      "matcher": "shell",
      "action": {"type": "command", "command": "scripts/trackfw-credential-guard.sh"}
    }
  ]
}
`
}

// TestCredentialGuardHookResolvable_KiroNaoGeraFalsoPositivoComCamposVizinhos — os campos "name"/
// "description" do Kiro citam "trackfw-credential-guard" (sem ".sh") ao lado de action.command;
// com o script presente e executável, a regra não deve violar a partir desses campos vizinhos.
func TestCredentialGuardHookResolvable_KiroNaoGeraFalsoPositivoComCamposVizinhos(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".kiro/hooks/trackfw-attention.json", kiroHooksFixture())
	scriptPath := filepath.Join(dir, "scripts", "trackfw-credential-guard.sh")
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("esperado zero violations com script Kiro presente e executável, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_KiroDisparaScriptAusente — sanity check simétrico: com o
// mesmo fixture Kiro, script ausente deve violar (prova que o teste acima não é vácuo).
func TestCredentialGuardHookResolvable_KiroDisparaScriptAusente(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".kiro/hooks/trackfw-attention.json", kiroHooksFixture())

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "does not exist") || !hasViolation(msgs, ".kiro/hooks/trackfw-attention.json") {
		t.Errorf("esperado violation de script ausente no fixture Kiro, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_CaminhoResolvidoEhFisicoNaoSimlink — o caminho absoluto
// embutido na mensagem usa o diretório FÍSICO (pós-resolução de symlink), igual a
// process.cwd()/os.getcwd() em Node/Python — não o caminho que os.Getwd() do Go retornaria via
// atalho de $PWD quando o diretório é acessado através de um symlink (ex.: /tmp -> /private/tmp
// no macOS). Sem isso, a mensagem diverge byte-a-byte entre os 3 stacks quando o projeto vive sob
// um diretório symlinked.
func TestCredentialGuardHookResolvable_CaminhoResolvidoEhFisicoNaoSimlink(t *testing.T) {
	dir := t.TempDir()
	physicalDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh`))

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	expected := filepath.Join(physicalDir, "scripts", "trackfw-credential-guard.sh")
	if !hasViolation(msgs, expected) {
		t.Errorf("esperado o caminho físico %q na mensagem, obteve: %v", expected, msgs)
	}
}

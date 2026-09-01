package validator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	// O bit de execução não é representável em NTFS: no Windows info.Mode()&0111
	// é 0 para todo arquivo, inclusive após os.Chmod(0o755). Este teste afirma
	// que a regra DISPARA quando o bit falta, o que só tem sentido onde o bit
	// existe — fixamos a plataforma em vez de pular, para que a garantia
	// continue verificada em qualquer host.
	CurrentGOOS = "linux"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

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

// TestCredentialGuardHookResolvable_DisparaFormaRelativaAntigaEmClaude — AC1: Claude settings com
// a forma relativa antiga ("scripts/...") e o script PRESENTE e executável deve gerar violação
// "bare relative path" (ROADMAP-2026-08-21 ML-1B, requiresVarOrShellPrefix).
func TestCredentialGuardHookResolvable_DisparaFormaRelativaAntigaEmClaude(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// Forma relativa antiga — idêntica à que o CMDB tinha antes do `trackfw update` (REQ-2026-08-17).
	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`scripts/trackfw-credential-guard.sh`))
	// Script PRESENTE e executável — prova que a violação não vem da ausência do script,
	// mas da forma do comando.
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
	if !hasViolation(msgs, "bare relative path") || !hasViolation(msgs, ".claude/settings.json") {
		t.Errorf("esperado violation de forma relativa antiga em Claude, obteve: %v", msgs)
	}
	if !hasViolation(msgs, "trackfw update") {
		t.Errorf("esperado menção a `trackfw update` na mensagem (AC4), obteve: %v", msgs)
	}
}

// ---------------------------------------------------------------------------
// Testes ML-1A — classificação por ancoragem (ADR-2026-08-22)
// ---------------------------------------------------------------------------

// guardEntryCodexHooks monta um .codex/hooks.json mínimo com uma entrada de credential-guard
// usando o valor bruto scriptCmd no campo "command".
func guardEntryCodexHooks(scriptCmd string) string {
	return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {"command": ` + scriptCmd + `, "type": "command"}
        ]
      }
    ]
  }
}
`
}

// guardEntryGeminiSettings monta um .gemini/settings.json mínimo com uma entrada de
// credential-guard usando o valor bruto scriptCmd no campo "command".
func guardEntryGeminiSettings(scriptCmd string) string {
	return `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {"command": "` + scriptCmd + `", "type": "command"}
        ]
      }
    ]
  }
}
`
}

// TestClassifyHookAnchorage_Classe1_Ancorado — classifyHookAnchorage retorna classe 1 para todas
// as formas ancoradas (variáveis de projeto, forma do Codex, caminho absoluto, ~/… sem aspas).
func TestClassifyHookAnchorage_Classe1_Ancorado(t *testing.T) {
	cases := []struct {
		raw       string
		wasQuoted bool
	}{
		{"$CLAUDE_PROJECT_DIR/scripts/trackfw-credential-guard.sh", false},
		{"$GEMINI_PROJECT_DIR/scripts/trackfw-credential-guard.sh", false},
		{"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh", false},
		{"/opt/scripts/trackfw-credential-guard.sh", false},
		{"/absolute/path/to/guard.sh", false},
		// ~/… sem aspas: tilde expande para $HOME em qualquer shell POSIX — ancorado.
		{"~/scripts/trackfw-credential-guard.sh", false},
		{"~/.trackfw/scripts/trackfw-credential-guard.sh", false},
	}
	for _, tc := range cases {
		got := classifyHookAnchorage(tc.raw, tc.wasQuoted)
		if got != hookAnchorageClassAnchored {
			t.Errorf("classifyHookAnchorage(%q, wasQuoted=%v) = %d, quero hookAnchorageClassAnchored (%d)", tc.raw, tc.wasQuoted, got, hookAnchorageClassAnchored)
		}
	}
}

// TestClassifyHookAnchorage_Classe2_CwdDependent — classifyHookAnchorage retorna classe 2 para
// formas dependentes do cwd ($PWD/…, ${PWD}/…, ./…, ../…, relativo puro, "~/…" aspeado).
func TestClassifyHookAnchorage_Classe2_CwdDependent(t *testing.T) {
	cases := []struct {
		raw       string
		wasQuoted bool
	}{
		{"$PWD/scripts/trackfw-credential-guard.sh", false},
		{"${PWD}/scripts/trackfw-credential-guard.sh", false},
		{"./scripts/trackfw-credential-guard.sh", false},
		{"../scripts/trackfw-credential-guard.sh", false},
		{"scripts/trackfw-credential-guard.sh", false},
		{"sh scripts/trackfw-credential-guard.sh", false},
		// "~/…" com aspas: tilde NÃO expande dentro de aspas duplas — classe 2.
		{"~/scripts/trackfw-credential-guard.sh", true},
		{"~/.trackfw/scripts/trackfw-credential-guard.sh", true},
	}
	for _, tc := range cases {
		got := classifyHookAnchorage(tc.raw, tc.wasQuoted)
		if got != hookAnchorageClassCwdDependent {
			t.Errorf("classifyHookAnchorage(%q, wasQuoted=%v) = %d, quero hookAnchorageClassCwdDependent (%d)", tc.raw, tc.wasQuoted, got, hookAnchorageClassCwdDependent)
		}
	}
}

// TestClassifyHookAnchorage_Classe3_Indecidivel — classifyHookAnchorage retorna classe 3 para
// variáveis próprias do usuário que o validador não pode resolver.
func TestClassifyHookAnchorage_Classe3_Indecidivel(t *testing.T) {
	cases := []struct {
		raw       string
		wasQuoted bool
	}{
		{"$SOME_OTHER_VAR/scripts/trackfw-credential-guard.sh", false},
		{"$MY_CUSTOM_DIR/guard.sh", false},
		{"$UNDEFINED/trackfw-credential-guard.sh", false},
	}
	for _, tc := range cases {
		got := classifyHookAnchorage(tc.raw, tc.wasQuoted)
		if got != hookAnchorageClassUndecidable {
			t.Errorf("classifyHookAnchorage(%q, wasQuoted=%v) = %d, quero hookAnchorageClassUndecidable (%d)", tc.raw, tc.wasQuoted, got, hookAnchorageClassUndecidable)
		}
	}
}

// TestStripOuterQuotesForClassify — remove aspas duplas envolventes, não toca em outros valores.
func TestStripOuterQuotesForClassify(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{`"$PWD/scripts/guard.sh"`, `$PWD/scripts/guard.sh`},
		{`"$(git rev-parse --show-toplevel)/scripts/guard.sh"`, `$(git rev-parse --show-toplevel)/scripts/guard.sh`},
		{`$CLAUDE_PROJECT_DIR/scripts/guard.sh`, `$CLAUDE_PROJECT_DIR/scripts/guard.sh`},
		{`scripts/guard.sh`, `scripts/guard.sh`},
		{`"`, `"`},           // string de 1 char
		{`""`, ``},           // string vazia entre aspas
		{`"abc`, `"abc`},     // aspas de abertura sem fechamento
	}
	for _, c := range cases {
		got := stripOuterQuotesForClassify(c.raw)
		if got != c.want {
			t.Errorf("stripOuterQuotesForClassify(%q) = %q, quero %q", c.raw, got, c.want)
		}
	}
}

// TestCredentialGuardHookResolvable_DisparaPwdEmClaude — AC2: Claude settings com $PWD/…
// e script presente deve gerar violação com mensagem explicando que $PWD não ancora.
func TestCredentialGuardHookResolvable_DisparaPwdEmClaude(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`$PWD/scripts/trackfw-credential-guard.sh`))
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
	if !hasViolation(msgs, "$PWD path") || !hasViolation(msgs, ".claude/settings.json") {
		t.Errorf("AC2: esperado violation de $PWD em Claude, obteve: %v", msgs)
	}
	if !hasViolation(msgs, "current working directory") {
		t.Errorf("AC2: mensagem deve explicar que $PWD não ancora, obteve: %v", msgs)
	}
	if !hasViolation(msgs, "trackfw update") {
		t.Errorf("AC2: mensagem deve citar `trackfw update`, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_DisparaPwdEntreAspasEmClaude — achado D.3: "$PWD/…" entre
// aspas também acusado após strip de aspas externas (classifyHookAnchorage opera no valor puro).
func TestCredentialGuardHookResolvable_DisparaPwdEntreAspasEmClaude(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// O JSON precisa de escape: o valor do campo "command" é "$PWD/scripts/...".
	// guardEntryClaudeSettings concatena diretamente o scriptCmd no JSON; para o valor
	// conter aspas duplas literais no JSON, precisamos escapar com \".
	const scriptCmdJSON = `\"$PWD/scripts/trackfw-credential-guard.sh\"`
	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(scriptCmdJSON))

	// Valida que o JSON foi gerado corretamente (fixture malformado dá falso negativo silencioso).
	rawJSON, readErr := os.ReadFile(filepath.Join(dir, ".claude/settings.json"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var parsedCheck interface{}
	if jsonErr := json.Unmarshal(rawJSON, &parsedCheck); jsonErr != nil {
		t.Fatalf("fixture JSON inválido: %v — saída: %s", jsonErr, rawJSON)
	}

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
	if !hasViolation(msgs, "$PWD path") || !hasViolation(msgs, ".claude/settings.json") {
		t.Errorf("D.3: esperado violation de $PWD entre aspas em Claude, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_CaminhoAbsolutoSilencioso — classe 1: caminho absoluto não
// deve gerar violação (wiring legítimo, ancora independentemente do cwd). O falso-positivo que
// reprova a entrega (ADR-2026-08-22).
func TestCredentialGuardHookResolvable_CaminhoAbsolutoSilencioso(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`/opt/scripts/trackfw-credential-guard.sh`))

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("classe 1 (absoluto) deve ser silenciosa, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_OutraVarSilenciosa — classe 3: $OUTRA_VAR/… não deve gerar
// violação (indecidível; silêncio declarado no ADR-2026-08-22).
func TestCredentialGuardHookResolvable_OutraVarSilenciosa(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`$MY_CUSTOM_DIR/scripts/trackfw-credential-guard.sh`))

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("classe 3 ($OUTRA_VAR) deve ser silenciosa, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_CodexFormaSilenciosa — forma do Codex com aspas e git
// rev-parse (classe 1) deve continuar silenciosa após strip de aspas externas.
func TestCredentialGuardHookResolvable_CodexFormaSilenciosa(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	// O valor JSON do Codex já inclui as aspas externas: "\"$(git rev-parse ...)\""
	// guardEntryCodexHooks recebe o valor JSON bruto (incluindo as aspas para o JSON).
	codexHooks := `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": ".*",
        "hooks": [
          {"command": "\"$(git rev-parse --show-toplevel)/scripts/trackfw-credential-guard.sh\"", "type": "command"}
        ]
      }
    ]
  }
}
`
	writeFile(t, dir, ".codex/hooks.json", codexHooks)
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
		t.Errorf("forma Codex (classe 1 com aspas) deve ser silenciosa, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_DisparaPwdEmCodex — AC2 para Codex: $PWD/… também acusado.
func TestCredentialGuardHookResolvable_DisparaPwdEmCodex(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".codex/hooks.json", `{
  "hooks": {
    "PreToolUse": [
      {"matcher": ".*", "hooks": [{"command": "$PWD/scripts/trackfw-credential-guard.sh", "type": "command"}]}
    ]
  }
}
`)

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "$PWD path") || !hasViolation(msgs, ".codex/hooks.json") {
		t.Errorf("AC2 Codex: esperado violation de $PWD, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_DisparaPwdEmGemini — AC2 para Gemini: $PWD/… também acusado.
func TestCredentialGuardHookResolvable_DisparaPwdEmGemini(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".gemini/settings.json", guardEntryGeminiSettings(`$PWD/scripts/trackfw-credential-guard.sh`))

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "$PWD path") || !hasViolation(msgs, ".gemini/settings.json") {
		t.Errorf("AC2 Gemini: esperado violation de $PWD, obteve: %v", msgs)
	}
}

// TestHookValueWasQuoted — hookValueWasQuoted detecta aspas externas.
func TestHookValueWasQuoted(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{`"$PWD/scripts/guard.sh"`, true},
		{`"~/scripts/guard.sh"`, true},
		{`~/scripts/guard.sh`, false},
		{`$PWD/scripts/guard.sh`, false},
		{`"`, false},  // 1 char
		{`""`, true},  // string vazia aspeada
		{`"abc`, false},
	}
	for _, c := range cases {
		got := hookValueWasQuoted(c.raw)
		if got != c.want {
			t.Errorf("hookValueWasQuoted(%q) = %v, quero %v", c.raw, got, c.want)
		}
	}
}

// TestCwdDependentReason_PwdEmQualquerPosicao — cwdDependentReason retorna mensagem do $PWD
// quando $PWD aparece em qualquer posição (inclusive dentro de wrapper sh -c), não só no prefixo.
func TestCwdDependentReason_PwdEmQualquerPosicao(t *testing.T) {
	pwdCases := []string{
		"$PWD/scripts/guard.sh",
		"${PWD}/scripts/guard.sh",
		`sh -c "$PWD/scripts/guard.sh"`,
		`env FOO=x $PWD/scripts/guard.sh`,
	}
	for _, raw := range pwdCases {
		reason := cwdDependentReason(raw)
		if !strings.Contains(reason, "$PWD path") {
			t.Errorf("cwdDependentReason(%q) deve conter '$PWD path', obteve: %q", raw, reason)
		}
	}
	bareCases := []string{
		"./scripts/guard.sh",
		"../scripts/guard.sh",
		"scripts/guard.sh",
		"~/scripts/guard.sh", // ~/… aspeado cai aqui (sem $PWD)
	}
	for _, raw := range bareCases {
		reason := cwdDependentReason(raw)
		if !strings.Contains(reason, "bare relative path") {
			t.Errorf("cwdDependentReason(%q) deve conter 'bare relative path', obteve: %q", raw, reason)
		}
	}
}

// TestCredentialGuardHookResolvable_TildeSemAspas_Silencioso — ML-4A: ~/… sem aspas é classe 1
// (tilde expande para $HOME — ancorado). Não deve gerar violação (falso-positivo da barreira ML-3A).
func TestCredentialGuardHookResolvable_TildeSemAspas_Silencioso(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`~/scripts/trackfw-credential-guard.sh`))

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("ML-4A: ~/… sem aspas (classe 1) deve ser silencioso, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_TildeComAspas_Acusado — ML-4A: "~/…" aspeado é classe 2
// (tilde NÃO expande dentro de aspas duplas). Deve gerar violação com mensagem "bare relative path".
func TestCredentialGuardHookResolvable_TildeComAspas_Acusado(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	const scriptCmdJSON = `\"~/scripts/trackfw-credential-guard.sh\"`
	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(scriptCmdJSON))

	rawJSON, readErr := os.ReadFile(filepath.Join(dir, ".claude/settings.json"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var parsedCheck interface{}
	if jsonErr := json.Unmarshal(rawJSON, &parsedCheck); jsonErr != nil {
		t.Fatalf("fixture JSON inválido: %v — saída: %s", jsonErr, rawJSON)
	}

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "bare relative path") {
		t.Errorf("ML-4A: \"~/…\" aspeado (classe 2) deve ser acusado com 'bare relative path', obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_PwdChaveado_Acusado — ML-4A: ${PWD}/… é classe 2 (mesma
// semântica de $PWD/… — cwd-dependent). Deve gerar violação com mensagem do $PWD.
func TestCredentialGuardHookResolvable_PwdChaveado_Acusado(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(`${PWD}/scripts/trackfw-credential-guard.sh`))

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "$PWD path") {
		t.Errorf("ML-4A: ${PWD}/… deve ser acusado com mensagem do $PWD, obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_ShCPwd_MensagemPwd — ML-4A: sh -c "$PWD/…" deve ser acusado
// com a mensagem do $PWD (não "bare relative path"), pois $PWD está presente no comando.
func TestCredentialGuardHookResolvable_ShCPwd_MensagemPwd(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	const scriptCmdJSON = `sh -c \"$PWD/scripts/trackfw-credential-guard.sh\"`
	writeFile(t, dir, ".claude/settings.json", guardEntryClaudeSettings(scriptCmdJSON))

	rawJSON, readErr := os.ReadFile(filepath.Join(dir, ".claude/settings.json"))
	if readErr != nil {
		t.Fatalf("ReadFile: %v", readErr)
	}
	var parsedCheck interface{}
	if jsonErr := json.Unmarshal(rawJSON, &parsedCheck); jsonErr != nil {
		t.Fatalf("fixture JSON inválido: %v — saída: %s", jsonErr, rawJSON)
	}

	msgs, err := validateCredentialGuardHookResolvable()
	if err != nil {
		t.Fatalf("validateCredentialGuardHookResolvable() erro: %v", err)
	}
	if !hasViolation(msgs, "$PWD path") {
		t.Errorf("ML-4A: sh -c \"$PWD/…\" deve usar mensagem do $PWD, obteve: %v", msgs)
	}
	if hasViolation(msgs, "bare relative path") {
		t.Errorf("ML-4A: sh -c \"$PWD/…\" não deve dizer 'bare relative path', obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_NaoDisparaFormaRelativaEmCursor — AC3 (não-vácuo): Cursor com
// caminho relativo puro e script PRESENTE e executável não deve gerar violação. O caminho relativo
// é a forma CORRETA para Cursor (requiresVarOrShellPrefix=false).
func TestCredentialGuardHookResolvable_NaoDisparaFormaRelativaEmCursor(t *testing.T) {
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
	// Script presente e executável — prova que o silêncio não vem da ausência,
	// mas do tratamento correto de Cursor (requiresVarOrShellPrefix=false).
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
		t.Errorf("AC3: Cursor com relativo deve estar limpo (falso-positivo eliminado por construção), obteve: %v", msgs)
	}
}

// TestCredentialGuardHookResolvable_WindowsNaoDisparaBitDeExecucao — no Windows o bit de
// execução não é representável em NTFS: os.Stat().Mode()&0111 é 0 para todo arquivo
// comum, inclusive imediatamente depois de os.Chmod(path, 0o755). Uma checagem escrita
// como "o script não é executável" é, portanto, SEMPRE verdadeira lá, e nenhuma ação do
// usuário a torna falsa — `trackfw update`, o remédio que a mensagem prescreve, regenera
// o script com o mesmo modo irrepresentável.
//
// Mesmo precedente de generators.CurrentGOOS (scaffold_doctor.go, AC5), agora na camada
// do validator.
func TestCredentialGuardHookResolvable_WindowsNaoDisparaBitDeExecucao(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	t.Cleanup(config.Reset)

	CurrentGOOS = "windows"
	t.Cleanup(func() { CurrentGOOS = runtime.GOOS })

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
	if hasViolation(msgs, "not executable") {
		t.Errorf("no Windows o bit de execução não é representável — a regra não deve disparar. Obteve: %v", msgs)
	}
	// O script EXISTE: a checagem de existência continua valendo no Windows, e é
	// ela que ainda protege contra a fiação apontar para lugar nenhum.
	if hasViolation(msgs, "does not exist") {
		t.Errorf("script existe; violation de ausência não deveria aparecer: %v", msgs)
	}
}

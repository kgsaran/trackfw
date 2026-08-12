package validator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// credentialGuardScriptMarker é o nome do script que a regra credential_guard_hook_resolvable
// procura dentro dos comandos de hook de projeto.
const credentialGuardScriptMarker = "trackfw-credential-guard.sh"

// credentialGuardHookFile associa um arquivo de hook de projeto ao CLI que o consome, para
// compor mensagens de violação acionáveis.
type credentialGuardHookFile struct {
	path string // relativo à raiz do projeto (CWD)
	cli  string
}

// credentialGuardHookFiles é a lista fechada dos arquivos de hook de PROJETO que o trackfw
// gera hoje e que podem conter uma entrada de credential-guard (docs/roadmaps/wip/
// ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard-...md, ML-1A). Hooks de escopo
// GLOBAL (~/.trackfw/..., trackfw update harness) ficam fora — são um caso distinto, fora do
// repositório do usuário, e a checagem de dedup globalCredentialGuardInstalled*() já os pula de
// propósito nas entradas de projeto.
var credentialGuardHookFiles = []credentialGuardHookFile{
	{".claude/settings.json", "Claude Code"},
	{".codex/hooks.json", "Codex CLI"},
	{".gemini/settings.json", "Gemini CLI"},
	{".cursor/hooks.json", "Cursor"},
	{".github/hooks/trackfw-attention.json", "GitHub Copilot CLI"},
	{".kiro/hooks/trackfw-attention.json", "Kiro"},
}

// resolveCredentialGuardHookPath resolve o valor bruto de um comando de hook (string extraída do
// JSON) para um caminho de arquivo absoluto, usando exatamente as 3 formas de prefixo que o
// trackfw emite hoje (docs/cli-parity.md, "Mecanismo de resolução de caminho dos hooks de
// projeto, por CLI"):
//
//  1. "$CLAUDE_PROJECT_DIR/…" / "$GEMINI_PROJECT_DIR/…" — placeholder de env var expandido em
//     runtime pelo próprio CLI, substituído aqui pela raiz do projeto.
//  2. "\"$(git rev-parse --show-toplevel)/…\"" — substituição de shell entre aspas literais
//     (Codex). As aspas fazem parte do valor emitido (ver internal/generators/agentfiles.go,
//     const codexRoot) e são removidas antes de resolver contra a raiz do projeto.
//  3. Caminho relativo puro, sem prefixo nenhum (Cursor/Copilot/Kiro) — resolvido diretamente
//     contra a raiz do projeto.
//
// Qualquer valor que não bata em nenhuma das 3 formas retorna ok=false — o chamador NÃO deve
// tratar isso como violação. Não é função desta regra adivinhar wiring próprio de um usuário fora
// dos formatos que o trackfw gera.
func resolveCredentialGuardHookPath(raw, root string) (resolved string, ok bool) {
	const claudePrefix = "$CLAUDE_PROJECT_DIR/"
	const geminiPrefix = "$GEMINI_PROJECT_DIR/"
	const codexPrefix = `"$(git rev-parse --show-toplevel)/`

	switch {
	case strings.HasPrefix(raw, claudePrefix):
		return filepath.Join(root, strings.TrimPrefix(raw, claudePrefix)), true
	case strings.HasPrefix(raw, geminiPrefix):
		return filepath.Join(root, strings.TrimPrefix(raw, geminiPrefix)), true
	case strings.HasPrefix(raw, codexPrefix) && strings.HasSuffix(raw, `"`):
		inner := strings.TrimSuffix(strings.TrimPrefix(raw, codexPrefix), `"`)
		return filepath.Join(root, inner), true
	case !strings.HasPrefix(raw, "$") && !strings.HasPrefix(raw, `"`) && !filepath.IsAbs(raw):
		// Caminho relativo puro — Cursor (beforeShellExecution/preToolUse), GitHub Copilot CLI
		// (campo "bash"), Kiro (action.command).
		return filepath.Join(root, raw), true
	default:
		return "", false
	}
}

// collectCredentialGuardCommands percorre recursivamente um valor JSON já decodificado
// (map[string]interface{} / []interface{} / string / ...) e coleta todo valor-string que
// referencia trackfw-credential-guard.sh, independentemente do nome do campo que o contém.
//
// Os 6 formatos de hook usam campos diferentes para o comando: "command" (Claude/Codex/
// Gemini/Cursor), "bash" (GitHub Copilot CLI), "action.command" (Kiro). Varrer por VALOR em vez
// de por caminho de chave evita acoplar esta regra à forma exata de cada schema — decisão de
// design deste ML, não replicar o parsing schema-aware que agentfiles.go faz para escrever os
// hooks.
func collectCredentialGuardCommands(v interface{}, out *[]string) {
	switch t := v.(type) {
	case string:
		if strings.Contains(t, credentialGuardScriptMarker) {
			*out = append(*out, t)
		}
	case map[string]interface{}:
		for _, val := range t {
			collectCredentialGuardCommands(val, out)
		}
	case []interface{}:
		for _, val := range t {
			collectCredentialGuardCommands(val, out)
		}
	}
}

// validateCredentialGuardHookResolvable é a regra "credential_guard_hook_resolvable": para cada
// arquivo de hook de PROJETO que existir, extrai os comandos que referenciam
// trackfw-credential-guard.sh, resolve o caminho e verifica que o script existe e é executável.
//
// Riscos de regressão mapeados no roadmap (ver ML-1A):
//   - A regra só avalia entradas que EXISTEM. Ausência de entrada de guard é estado legítimo (guard
//     global instalado via `trackfw update harness`, dedup globalCredentialGuardInstalled*()) —
//     nunca é violação por si só.
//   - Arquivo de hook ausente é pulado em silêncio (não é responsabilidade desta regra garantir
//     que o arquivo exista).
//   - Arquivo de hook presente mas com JSON inválido é pulado em silêncio — validar a forma do
//     JSON não é escopo desta regra.
func validateCredentialGuardHookResolvable() ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	// os.Getwd() trusts $PWD when it names the same directory by inode, so it can return a
	// symlinked path (e.g. macOS's /tmp -> /private/tmp) instead of the physical one. Node's
	// process.cwd() and Python's os.getcwd() both call the getcwd(3) syscall directly and always
	// return the physical path — resolving symlinks here keeps the ABSOLUTE path embedded in this
	// rule's violation message byte-identical across the 3 stacks (this rule is the first one to
	// embed a resolved absolute path in its message; every prior rule only ever emits paths
	// relative to the project root, so this divergence was latent until now).
	if resolvedRoot, symErr := filepath.EvalSymlinks(root); symErr == nil {
		root = resolvedRoot
	}

	var msgs []string
	for _, hf := range credentialGuardHookFiles {
		fullPath := filepath.Join(root, hf.path)
		content, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return nil, fmt.Errorf("credential_guard_hook_resolvable: lendo %s: %w", hf.path, readErr)
		}

		var parsed interface{}
		if jsonErr := json.Unmarshal(content, &parsed); jsonErr != nil {
			continue
		}

		var commands []string
		collectCredentialGuardCommands(parsed, &commands)

		seen := make(map[string]bool, len(commands))
		for _, raw := range commands {
			if seen[raw] {
				continue
			}
			seen[raw] = true

			resolved, ok := resolveCredentialGuardHookPath(raw, root)
			if !ok {
				continue
			}

			info, statErr := os.Stat(resolved)
			switch {
			case statErr != nil:
				msgs = append(msgs, fmt.Sprintf(
					"%s (%s) references trackfw-credential-guard.sh resolved to %q, but the script does not exist — run `trackfw update` to regenerate it",
					hf.path, hf.cli, resolved,
				))
			case info.Mode()&0111 == 0:
				msgs = append(msgs, fmt.Sprintf(
					"%s (%s) references trackfw-credential-guard.sh resolved to %q, but the script is not executable — run `trackfw update` to regenerate it",
					hf.path, hf.cli, resolved,
				))
			}
		}
	}

	return msgs, nil
}

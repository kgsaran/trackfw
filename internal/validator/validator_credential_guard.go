package validator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// credentialGuardScriptMarker é o nome do script que a regra credential_guard_hook_resolvable
// procura dentro dos comandos de hook de projeto.
const credentialGuardScriptMarker = "trackfw-credential-guard.sh"

// gitBranchGuardScriptMarker é o nome do script que a regra git_branch_guard_hook_resolvable
// procura dentro dos comandos de hook de projeto (ROADMAP-2026-08-15-trackfw-validate-deve-
// detectar-scripts-de-hook-ausentes-ou-desatualizados, ML-1A). Mesmo padrão de
// credentialGuardScriptMarker — só o nome do arquivo muda.
const gitBranchGuardScriptMarker = "trackfw-git-branch-guard.sh"

// credentialGuardHookFile associa um arquivo de hook de projeto ao CLI que o consome, para
// compor mensagens de violação acionáveis.
//
// requiresCommandType (ROADMAP-2026-08-17 ML-4B): se true, um comando casado que não estiver
// dentro de um objeto JSON com "type":"command" como campo irmão é tratado como ENTRADA
// ESTRUTURALMENTE MALFORMADA (o CLI silenciosamente nunca a executa — hades-tf ML-4A barrier
// finding), não como ausência. true para todos os 5 CLIs cujo escritor sempre emite "type":
// "command" (Claude/Codex/Gemini via mergeClaudeHookArray, GitHub Copilot CLI via
// mergeCredentialGuardCopilotHooks, Kiro via action.type em InjectKiroHooks — internal/
// generators/agentfiles.go, update.go). false só para Cursor, cujo schema
// (mergeCredentialGuardCursorHooks, {"command":...}) nunca carrega um campo "type" — exigi-lo
// ali faria uma entrada Cursor válida e em execução ser acusada de malformada. Não uniformizar.
type credentialGuardHookFile struct {
	path                string // relativo à raiz do projeto (CWD)
	cli                 string
	requiresCommandType bool
}

// credentialGuardHookFiles é a lista fechada dos arquivos de hook de PROJETO que o trackfw
// gera hoje e que podem conter uma entrada de credential-guard (docs/roadmaps/wip/
// ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard-...md, ML-1A). Hooks de escopo
// GLOBAL (~/.trackfw/..., trackfw update harness) ficam fora — são um caso distinto, fora do
// repositório do usuário, e a checagem de dedup globalCredentialGuardInstalled*() já os pula de
// propósito nas entradas de projeto.
var credentialGuardHookFiles = []credentialGuardHookFile{
	{".claude/settings.json", "Claude Code", true},
	{".codex/hooks.json", "Codex CLI", true},
	{".gemini/settings.json", "Gemini CLI", true},
	{".cursor/hooks.json", "Cursor", false},
	{".github/hooks/trackfw-attention.json", "GitHub Copilot CLI", true},
	{".kiro/hooks/trackfw-attention.json", "Kiro", true},
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

// guardCommandMatch pairs a matched raw command string with whether the immediate JSON object it
// was found in also carries a "type" field equal to "command" — the structural discriminant
// ROADMAP-2026-08-17 ML-4B adds. Every hook schema this validator reads (Claude/Codex/Gemini's
// {"hooks":[{"type","command"}]}, Copilot's {"type","bash"}, Kiro's action:{"type","command"})
// places "type" as a SIBLING of the marker field within the SAME object — never nested deeper —
// so recording it from the object collectCommandsWithMarker is currently visiting is sufficient;
// no separate schema-aware walk is needed. Cursor's flat {"command":...} shape never carries a
// "type" field at all, so typeIsCommand is always false there — see credentialGuardHookFile.
// requiresCommandType for how callers decide whether that false is a violation or expected.
type guardCommandMatch struct {
	raw           string
	typeIsCommand bool
}

// collectCommandsWithMarker percorre recursivamente um valor JSON já decodificado
// (map[string]interface{} / []interface{} / string / ...) e coleta todo valor-string que contém
// marker, independentemente do nome do campo que o contém — junto com um sinal estrutural
// (guardCommandMatch.typeIsCommand) de que o objeto imediato que o contém também tem
// "type":"command" como campo irmão (ROADMAP-2026-08-17 ML-4B).
//
// Os 6 formatos de hook usam campos diferentes para o comando: "command" (Claude/Codex/
// Gemini/Cursor), "bash" (GitHub Copilot CLI), "action.command" (Kiro). Varrer por VALOR em vez
// de por caminho de chave evita acoplar esta regra à forma exata de cada schema — decisão de
// design deste ML, não replicar o parsing schema-aware que agentfiles.go faz para escrever os
// hooks. O sinal de "type" é a única exceção a essa regra: ele NÃO decide se algo é um "comando"
// (isso continua sendo puramente por valor/marker), só anota, para cada match já encontrado, se o
// objeto que o continha também parece estruturalmente válido — a decisão de exigir ou não esse
// sinal continua no chamador (via requiresCommandType), não aqui.
//
// Generalizado (ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
// desatualizados, ML-1A) para aceitar qualquer marker — originalmente hardcoded para
// credentialGuardScriptMarker; reusado agora também para gitBranchGuardScriptMarker, sem duplicar
// a travessia recursiva.
func collectCommandsWithMarker(v interface{}, marker string, out *[]guardCommandMatch) {
	switch t := v.(type) {
	case string:
		// Top-level/loose string match, outside any enclosing object (defensive fallback — every
		// hook file this validator reads is rooted at a JSON object in practice, so this branch is
		// not expected to fire, but preserves the original function's contract of matching ANY
		// string value anywhere in the tree). No enclosing object means no "type" sibling to read.
		if strings.Contains(t, marker) {
			*out = append(*out, guardCommandMatch{raw: t, typeIsCommand: false})
		}
	case map[string]interface{}:
		typeStr, _ := t["type"].(string)
		typeIsCommand := typeStr == "command"
		for _, val := range t {
			if s, ok := val.(string); ok {
				if strings.Contains(s, marker) {
					*out = append(*out, guardCommandMatch{raw: s, typeIsCommand: typeIsCommand})
				}
				continue
			}
			collectCommandsWithMarker(val, marker, out)
		}
	case []interface{}:
		for _, val := range t {
			collectCommandsWithMarker(val, marker, out)
		}
	}
}

// validateGuardHookResolvable é a implementação genérica compartilhada pelas regras
// "credential_guard_hook_resolvable" e "git_branch_guard_hook_resolvable": para cada arquivo de
// hook de PROJETO que existir, extrai os comandos que referenciam scriptMarker, resolve o caminho
// e verifica que o script existe e é executável.
//
// Generalizado a partir da antiga validateCredentialGuardHookResolvable (ROADMAP-2026-08-15-
// trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados, ML-1A) — a lógica de
// resolução de caminho por CLI é idêntica para os 2 scripts, só o marker e o texto da mensagem
// mudam.
//
// Riscos de regressão mapeados no roadmap (ver ML-1A):
//   - A regra só avalia entradas que EXISTEM. Ausência de entrada de guard é estado legítimo (guard
//     global instalado via `trackfw update harness`, dedup globalCredentialGuardInstalled*()) —
//     nunca é violação por si só.
//   - Arquivo de hook ausente é pulado em silêncio (não é responsabilidade desta regra garantir
//     que o arquivo exista).
//   - Arquivo de hook presente mas com JSON inválido é pulado em silêncio — validar a forma do
//     JSON não é escopo desta regra.
func validateGuardHookResolvable(ruleName, scriptMarker string) ([]string, error) {
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
			return nil, fmt.Errorf("%s: lendo %s: %w", ruleName, hf.path, readErr)
		}

		var parsed interface{}
		if jsonErr := json.Unmarshal(content, &parsed); jsonErr != nil {
			continue
		}

		var commands []guardCommandMatch
		collectCommandsWithMarker(parsed, scriptMarker, &commands)

		seen := make(map[string]bool, len(commands))
		for _, m := range commands {
			seenKey := m.raw + "\x00" + strconv.FormatBool(m.typeIsCommand)
			if seen[seenKey] {
				continue
			}
			seen[seenKey] = true

			resolved, ok := resolveCredentialGuardHookPath(m.raw, root)
			if !ok {
				continue
			}

			// ROADMAP-2026-08-17 ML-4B: a command that resolves to a real path but sits inside a
			// structurally malformed entry (missing/wrong "type" where this CLI's schema requires
			// it — hades-tf ML-4A barrier finding) will NEVER be executed by the CLI, regardless of
			// whether the script itself exists and is executable — checking existence/executability
			// first would silently pass a hook that never fires. Reported instead of the
			// exists/executable checks below, which assume a structurally valid entry.
			if hf.requiresCommandType && !m.typeIsCommand {
				msgs = append(msgs, fmt.Sprintf(
					`%s (%s) references %s resolved to %q, but the hook entry is missing "type":"command" (or has an invalid type) — %s will silently never execute it; run `+"`trackfw update`"+` to regenerate it`,
					hf.path, hf.cli, scriptMarker, resolved, hf.cli,
				))
				continue
			}

			info, statErr := os.Stat(resolved)
			switch {
			case statErr != nil:
				msgs = append(msgs, fmt.Sprintf(
					"%s (%s) references %s resolved to %q, but the script does not exist — run `trackfw update` to regenerate it",
					hf.path, hf.cli, scriptMarker, resolved,
				))
			case info.Mode()&0111 == 0:
				msgs = append(msgs, fmt.Sprintf(
					"%s (%s) references %s resolved to %q, but the script is not executable — run `trackfw update` to regenerate it",
					hf.path, hf.cli, scriptMarker, resolved,
				))
			}
		}
	}

	return msgs, nil
}

// validateCredentialGuardHookResolvable é a regra "credential_guard_hook_resolvable" — ver
// validateGuardHookResolvable para a implementação compartilhada.
func validateCredentialGuardHookResolvable() ([]string, error) {
	return validateGuardHookResolvable("credential_guard_hook_resolvable", credentialGuardScriptMarker)
}

// validateGitBranchGuardHookResolvable é a regra "git_branch_guard_hook_resolvable" — ver
// validateGuardHookResolvable para a implementação compartilhada.
func validateGitBranchGuardHookResolvable() ([]string, error) {
	return validateGuardHookResolvable("git_branch_guard_hook_resolvable", gitBranchGuardScriptMarker)
}

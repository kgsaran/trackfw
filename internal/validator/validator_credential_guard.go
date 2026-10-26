package validator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kgsaran/trackfw/internal/agentconfig"
	"github.com/kgsaran/trackfw/internal/pathanchor"
)

// credentialGuardScriptMarker é o nome do script que a regra credential_guard_hook_resolvable
// procura dentro dos comandos de hook de projeto.
const credentialGuardScriptMarker = "trackfw-credential-guard.sh"

// gitBranchGuardScriptMarker é o nome do script que a regra git_branch_guard_hook_resolvable
// procura dentro dos comandos de hook de projeto (ROADMAP-2026-08-15-trackfw-validate-deve-
// detectar-scripts-de-hook-ausentes-ou-desatualizados, ML-1A). Mesmo padrão de
// credentialGuardScriptMarker — só o nome do arquivo muda.
const gitBranchGuardScriptMarker = "trackfw-git-branch-guard.sh"

// credentialGuardSubcmdMarker é o prefixo do subcomando Go para o credential-guard
// (ADR-2026-10-04, ML-2B). Usado para detectar a NOVA forma de hook.
const credentialGuardSubcmdMarker = "trackfw guard credential"

// gitBranchGuardSubcmdMarker é o prefixo do subcomando Go para o git-branch-guard
// (ADR-2026-10-04, ML-2B).
const gitBranchGuardSubcmdMarker = "trackfw guard git-branch"

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
//
// requiresVarOrShellPrefix (ROADMAP-2026-08-21 ML-1B): se true, um comando casado na forma
// relativa pura (sem prefixo "$", sem aspas de substituição de shell, sem caminho absoluto) é
// tratado como FORMA RELATIVA ANTIGA — o ADR-2026-08-11 exige para estes CLIs que o comando
// use um mecanismo que ancora o caminho à raiz do projeto ($VAR/... ou "$(git ...)/..."), pois
// os hooks não são invocados necessariamente a partir da raiz (causa do bug REQ-2026-08-17).
// true para Claude/Codex/Gemini; false para Cursor/Copilot/Kiro, cujo caminho relativo puro
// é a forma CORRETA — acusá-los seria falso-positivo (o risco dominante desta REQ).
type credentialGuardHookFile struct {
	path                     string // relativo à raiz do projeto (CWD)
	cli                      string
	requiresCommandType      bool
	requiresVarOrShellPrefix bool
	// family classifica o ambiente de shell que executa este hook (ML-2B, ADR-2026-10-04 D2
	// revista). Determina a linha exata esperada quando a config usa o subcomando Go.
	// Tabela normativa em guardShellFamily (validator_guard_binary_probe.go).
	family guardShellFamily
}

// credentialGuardHookFiles é a lista fechada dos arquivos de hook de PROJETO que o trackfw
// gera hoje e que podem conter uma entrada de credential-guard (docs/roadmaps/wip/
// ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard-...md, ML-1A). Hooks de escopo
// GLOBAL (~/.trackfw/..., trackfw update harness) ficam fora — são um caso distinto, fora do
// repositório do usuário. D15 (ADR-2026-10-04 adendo 2026-10-24): o gerador não mais pula
// entradas de projeto quando o guard global está instalado; o validator não mais aceita ausência
// de wiring de projeto por conta do guard global.
var credentialGuardHookFiles = []credentialGuardHookFile{
	// ML-2B: `family` campo normativo — ADR-2026-10-04 D2 revista.
	// PS/POSIX: linha com "; exit $LASTEXITCODE". cmd.exe: linha sem sufixo.
	{".claude/settings.json", "Claude Code", true, true, guardShellFamilyPSPosix},
	{".codex/hooks.json", "Codex CLI", true, true, guardShellFamilyPSPosix},
	{".gemini/settings.json", "Gemini CLI", true, true, guardShellFamilyPSPosix},
	{".cursor/hooks.json", "Cursor", false, false, guardShellFamilyPSPosix},
	{".github/hooks/trackfw-attention.json", "GitHub Copilot CLI", true, false, guardShellFamilyPSPosix},
	{".kiro/hooks/trackfw-attention.json", "Kiro", true, false, guardShellFamilyCmdExe},
	// ML-2C / ML-1B (REQ-2026-10-06): Windsurf and Amazon Q — project-scope only.
	// ML-1B wires the credential guard for both CLIs (InjectWindsurfHooks, InjectAmazonQHooks).
	// validateCredentialGuardHookResolvable now enforces the credential guard is present
	// (validateCredentialGuardPresenceRequired). The stale premise "Both generators only inject
	// git-branch-guard (not credential guard)" was correct before ML-1B and is now obsolete.
	// Amazon Q has no documented global hook scope (Wave 0 residual R4), so the project file
	// is the sole credential guard install point for Amazon Q.
	// Windsurf has a global harness target (windsurf-credential-guard → ~/.codeium/windsurf/
	// hooks.json), but the presence check here is on the PROJECT file; a user with only
	// the global harness installed will still see a violation until the project file is
	// updated — this is intentional: project-scope install is the defence-in-depth layer.
	//
	// Windsurf: hooks.pre_run_command[{"command":"…","show_output":true}] — no "type" field;
	//   requiresVarOrShellPrefix=false: the legacy form is "bash scripts/…" (relative), and the
	//   anchorage check runs before path resolution; requiresVarOrShellPrefix=false keeps the
	//   legacy form in the warning path (not the cwd-dependent violation path) while
	//   resolveCredentialGuardHookPath's new "bash " case strips the prefix and resolves correctly.
	//   requiresCommandType=false (schema has no "type" discriminant);
	//   family=PS/POSIX: ADR-2026-10-04 D2 — hook executed via bash-compatible shell.
	{".windsurf/hooks.json", "Windsurf", false, false, guardShellFamilyPSPosix},
	// Amazon Q: hooks.preToolUse[{matcher:"execute_bash",hooks:[{command:"…"}]}] — no "type" field;
	//   File is always q_cli_default.json (mirrors amazonQDefaultAgentFile = "q_cli_default.json",
	//   internal/generators/agentfiles.go:1492 — only this one file, not a wildcard *.json).
	//   requiresVarOrShellPrefix=false: legacy form is "scripts/…" (plain relative path), same
	//   reasoning as Windsurf — must stay in warning path, not cwd-dependent violation path.
	//   requiresCommandType=false (schema has no "type" discriminant in inner command objects);
	//   family=cmd.exe: ADR-2026-10-04 D2 — Amazon Q invokes hooks without a shell suffix.
	{".amazonq/cli-agents/q_cli_default.json", "Amazon Q", false, false, guardShellFamilyCmdExe},
}

// hookAnchorageClass classifica a semântica de ancoragem de um valor de comando de hook.
// Avaliada apenas para CLIs com requiresVarOrShellPrefix=true (Claude Code, Codex CLI, Gemini
// CLI). A classificação opera sobre o valor com aspas externas já removidas (ver
// stripOuterQuotesForClassify). Decisão e motivo: ADR-2026-08-22. Três classes:
//
//   - hookAnchorageClassAnchored (1): ancora na raiz do projeto independentemente do cwd.
//   - hookAnchorageClassCwdDependent (2): expande a partir do diretório corrente; acusar.
//   - hookAnchorageClassUndecidable (3): não é possível decidir sem executar (residual declarado).
const (
	hookAnchorageClassAnchored     = 1
	hookAnchorageClassCwdDependent = 2
	hookAnchorageClassUndecidable  = 3
)

// stripOuterQuotesForClassify remove aspas duplas envolventes de raw, se presentes. Necessário
// porque o Codex emite "$(git rev-parse --show-toplevel)/…" com aspas como parte do valor — e
// um usuário que copia/edita à mão pode escrever "$PWD/scripts/…" com aspas (achado D.3 da
// barreira ML-3A de 2026-08-21). As aspas são sintaxe, não semântica de ancoragem; removê-las
// antes de classificar garante que a forma entre aspas receba o mesmo veredito que a forma sem
// aspas.
func stripOuterQuotesForClassify(raw string) string {
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return raw[1 : len(raw)-1]
	}
	return raw
}

// hookValueWasQuoted reporta se raw tinha aspas duplas externas que
// stripOuterQuotesForClassify removeria. Usado para distinguir ~/… (til
// expande para $HOME sem aspas — classe 1) de "~/…" (til NÃO expande dentro
// de aspas duplas — classe 2).
func hookValueWasQuoted(raw string) bool {
	return len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"'
}

// O predicado que classifica se um caminho de hook é ANCORADO (POSIX "/", letra de unidade
// Windows ou UNC — independente do SO host que roda o validator) mudou de nome e de pacote:
// pathIsAnchoredForHookConfig virou pathanchor.IsAnchored (internal/pathanchor). Decisão original:
// ADR-2026-09-04-caminho-posix-ancorado-num-config-lido-por-cli-de-agente-e-absoluto-independente-
// do-so-host, D1. Motivo da extração (ROADMAP-2026-09-03 Wave reaberta ML-R1, 2026-09-08): o MESMO
// defeito — filepath.IsAbs("/tmp/x") == false no Windows — vivia também em
// internal/integrations.Manager.resolve (manager.go:704), medido na VM Windows ARM64. A extração
// para um pacote-folha consumido pelos dois evita a segunda cópia do predicado — ver o doc comment
// de internal/pathanchor para a fronteira completa, corrigida, entre "classificação de uma string
// de entrada" (pathanchor) e "resolução real de caminho via path/filepath/os" (que continua em
// cada pacote, inalterada).

// classifyHookAnchorage retorna a classe de ancoragem de rawStripped (valor de comando de hook
// com aspas externas já removidas). wasQuoted indica se o valor original tinha aspas externas
// (obtido com hookValueWasQuoted antes de chamar stripOuterQuotesForClassify).
// Ver constantes hookAnchorageClass* e ADR-2026-08-22 para a decisão e o motivo de cada classe.
//
// Classe 2 é um predicado, não uma lista de literais: o critério é "expande a partir do
// diretório corrente". Formas novas que satisfaçam esse critério entram aqui automaticamente.
//
// Classe 3 permanece em silêncio por escolha, não por omissão: $FOO/… pode estar correto no
// ambiente do usuário; o validador não lê o ambiente em que o hook vai rodar.
func classifyHookAnchorage(rawStripped string, wasQuoted bool) int {
	// Classe 1 — ancora na raiz do projeto.
	if strings.HasPrefix(rawStripped, "$CLAUDE_PROJECT_DIR/") ||
		strings.HasPrefix(rawStripped, "$GEMINI_PROJECT_DIR/") ||
		strings.HasPrefix(rawStripped, "$(git rev-parse --show-toplevel)/") ||
		pathanchor.IsAnchored(rawStripped) {
		return hookAnchorageClassAnchored
	}
	// ~/… sem aspas: tilde expande para $HOME em qualquer shell POSIX — semânticamente ancorado.
	// "~/…" com aspas: tilde NÃO expande dentro de aspas duplas, logo a forma quebra — classe 2.
	if strings.HasPrefix(rawStripped, "~/") && !wasQuoted {
		return hookAnchorageClassAnchored
	}
	// Classe 2 — expande a partir do cwd.
	// ${PWD}/… tem a mesma semântica de $PWD/… (PWD é mandado pelo POSIX, sempre o cwd).
	if strings.HasPrefix(rawStripped, "$PWD/") ||
		strings.HasPrefix(rawStripped, "${PWD}/") ||
		strings.HasPrefix(rawStripped, "./") ||
		strings.HasPrefix(rawStripped, "../") ||
		(!strings.HasPrefix(rawStripped, "$") && !pathanchor.IsAnchored(rawStripped)) {
		return hookAnchorageClassCwdDependent
	}
	// Classe 3 — indecidível; silêncio declarado.
	return hookAnchorageClassUndecidable
}

// cwdDependentReason retorna o sufixo de mensagem específico por forma para violações de
// classe 2 (dependente do cwd), iniciando em "with a …". O resultado é concatenado após
// "references <scriptMarker> " pelo chamador. Dois formatos:
//
//   - $PWD/… ou qualquer forma contendo $PWD ou ${PWD} → "with a $PWD path — …"
//   - "~/…" com aspas (D4 da ADR-2026-09-04) → "with a quoted tilde path — …"
//   - "~usuario/…" (D4 da ADR-2026-09-04) → "with a named-user tilde path — …"
//   - demais (./…, ../…, relativo puro) → "with a bare relative path — this command only…"
//
// A detecção usa Contains em vez de HasPrefix para cobrir wrappers como
// `sh -c "$PWD/…"` e `env FOO=x $PWD/…` que contêm $PWD mas não o têm no prefixo.
// A frase "bare relative path" é preservada para não regredir os testes existentes e a UX
// introduzida pela ROADMAP-2026-08-21 ML-1B — os dois ramos de til abaixo NUNCA capturam nada
// fora das duas formas de til: qualquer outro caminho cai no fallback e mantém a frase intocada.
//
// 🔴 D4 da ADR-2026-09-04: os dois ramos de til abaixo distinguem a causa REAL de cada forma —
// achado do ML-2B. Antes desta função, "~usuario/" e "\"~/\"" caíam no catch-all e recebiam
// "bare relative path", que não é o motivo real: ~user/ EXPANDE em POSIX (o validator só não
// consegue resolver sem executar um shell — é indecidível, não relativo), e "~/" só falha por
// causa das ASPAS (til não expande dentro de aspas duplas). O invariante que permite reconhecer
// "~/" aqui SEM receber wasQuoted como parâmetro: classifyHookAnchorage já classifica "~/…"
// SEM aspas como classe 1 (ancorado) — logo todo "~/…" que chega até aqui (classe 2) já passou
// por aspas removidas de um valor originalmente citado; ver stripOuterQuotesForClassify.
func cwdDependentReason(rawStripped string) string {
	if strings.Contains(rawStripped, "$PWD") || strings.Contains(rawStripped, "${PWD}") {
		return "with a $PWD path — " +
			"$PWD expands to the current working directory, not the project root; " +
			"run `trackfw update` to fix it"
	}
	if strings.HasPrefix(rawStripped, "~/") {
		return "with a quoted tilde path — " +
			"double quotes prevent shell tilde expansion, so this never resolves to $HOME; " +
			"run `trackfw update` to fix it"
	}
	if strings.HasPrefix(rawStripped, "~") {
		return "with a named-user tilde path — " +
			"~user/ expands to that user's home directory, not the project root, and this " +
			"validator cannot resolve it without running a shell; " +
			"run `trackfw update` to fix it"
	}
	return "with a bare relative path — " +
		"this command only resolves from the project root and will silently " +
		"fail when the agent's cwd is a subdirectory; run `trackfw update` to fix it"
}

// isRelativePureForGuard foi substituída por classifyHookAnchorage (ADR-2026-08-22, ML-1A).
// Mantida apenas para referência durante a transição; não é usada em validateGuardHookResolvable.
// ATENÇÃO: remover após confirmação de que nenhum caller externo depende dela.
// func isRelativePureForGuard(raw string) bool {
// 	return !strings.HasPrefix(raw, "$") && !strings.HasPrefix(raw, `"`) && !filepath.IsAbs(raw)
// }

// resolveCredentialGuardHookPath resolve o valor bruto de um comando de hook (string extraída do
// JSON) para um caminho de arquivo absoluto, usando as formas de prefixo que o
// trackfw emite hoje (docs/cli-parity.md, "Mecanismo de resolução de caminho dos hooks de
// projeto, por CLI"):
//
//  1. "$CLAUDE_PROJECT_DIR/…" / "$GEMINI_PROJECT_DIR/…" — placeholder de env var expandido em
//     runtime pelo próprio CLI, substituído aqui pela raiz do projeto.
//  2. "\"$(git rev-parse --show-toplevel)/…\"" — substituição de shell entre aspas literais
//     (Codex). As aspas fazem parte do valor emitido (ver internal/generators/agentfiles.go,
//     const codexRoot) e são removidas antes de resolver contra a raiz do projeto.
//  3. "bash <relpath>" — forma legada do Windsurf (windsurfGitGuardCmd = "bash scripts/…"). O
//     prefixo "bash " é removido e o restante resolvido como caminho relativo (caso 4).
//  4. Caminho relativo puro, sem prefixo nenhum (Cursor/Copilot/Kiro/Amazon Q) — resolvido
//     diretamente contra a raiz do projeto.
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
	case strings.HasPrefix(raw, "bash "):
		// Windsurf legacy form: "bash scripts/trackfw-git-branch-guard.sh" — strip the "bash "
		// interpreter prefix and resolve the remaining path token as a relative path (Windsurf runs
		// hooks from the project root, so a bare relative path is safe here). Only simple single-token
		// paths are handled; if the remainder still looks like an env-var, absolute, or tilde path
		// we fall through to ok=false rather than guess.
		rest := raw[len("bash "):]
		if !strings.HasPrefix(rest, "$") && !strings.HasPrefix(rest, `"`) && !pathanchor.IsAnchored(rest) && !strings.HasPrefix(rest, "~/") {
			return filepath.Join(root, rest), true
		}
		return "", false
	case !strings.HasPrefix(raw, "$") && !strings.HasPrefix(raw, `"`) && !pathanchor.IsAnchored(raw) && !strings.HasPrefix(raw, "~/"):
		// Caminho relativo puro — Cursor (beforeShellExecution/preToolUse), GitHub Copilot CLI
		// (campo "bash"), Kiro (action.command), Amazon Q (command inside preToolUse hooks).
		// ~/… é excluído: é classe 1 (tilde expande para $HOME — ancorado) mas não é uma forma
		// que o validator consegue resolver sem expandir o til; ok=false silencia sem acusar.
		return filepath.Join(root, raw), true
	default:
		return "", false
	}
}

// guardCommandMatch pairs a matched raw command string with whether the immediate JSON object it
// was found in also carries a "type" field equal to "command" — the structural discriminant
// ROADMAP-2026-08-17 ML-4B adds. Every hook schema this validator reads (Claude/Codex/Gemini's
// {"hooks":[{"type","command"}]}, Copilot project {"type","command"}, Copilot global {"type","bash"},
// Kiro's action:{"type","command"}) places "type" as a SIBLING of the command field within the SAME
// object — never nested deeper — so recording it from the object collectCommandsWithMarkerBySchema
// is visiting is sufficient. Cursor's flat {"command":...} shape never carries a "type" field at
// all, so typeIsCommand is always false there — see credentialGuardHookFile.requiresCommandType
// for how callers decide whether that false is a violation or expected.
// D16 (ADR-2026-10-04): collectCommandsWithMarkerBySchema only records matches from the designated
// executable-command field of a hook entry; matches in any other field are silently ignored.
type guardCommandMatch struct {
	raw           string
	typeIsCommand bool
}

// collectCommandsFromCommandKeyArray iterates a flat array of hook-entry objects and appends every
// entry whose "command" string field contains marker. typeIsCommand is set when the entry also
// carries a sibling "type":"command" field (for requiresCommandType callers — ROADMAP-2026-08-17
// ML-4B). Used by extractCredentialGuardCommandsFromEvent (Windsurf flat array and Amazon Q inner
// hooks array) and by collectCommandsWithMarkerBySchema (hookCmdSchemaFlat inner iteration).
func collectCommandsFromCommandKeyArray(arr interface{}, marker string, out *[]guardCommandMatch) {
	entries, _ := arr.([]interface{})
	for _, entryRaw := range entries {
		obj, ok := entryRaw.(map[string]interface{})
		if !ok {
			continue
		}
		typeStr, _ := obj["type"].(string)
		cmd, _ := obj["command"].(string)
		if cmd != "" && strings.Contains(cmd, marker) {
			*out = append(*out, guardCommandMatch{raw: cmd, typeIsCommand: typeStr == "command"})
		}
	}
}

// collectCommandsWithMarkerBySchema collects command strings that contain marker from a parsed
// hook file using the structural path defined by schema — D16 (ADR-2026-10-04): only a string in
// the designated executable-command field of a hook entry counts as wiring; a marker found in any
// other field (e.g. "note", "autoMode.environment", "permissions.allow") is NOT wiring and must
// not be recognized as such.
//
// This replaces collectCommandsWithMarker (the old recursive key-blind scan) for all callers that
// receive a full parsed hook-file document. The schema for each file is registered in
// guardWiringHookLayout (project files, validator_guard_wiring.go) and guardGlobalHookLayout
// (global files, validator_git_branch_guard.go). Every file in credentialGuardHookFiles and
// globalGuardConfigFiles has a registered schema; TestGuardWiringHookLayout_CobreTodasAsEntradas
// and TestGuardGlobalHookLayout_CobreTodasAsEntradas enforce this invariant.
//
// typeIsCommand is preserved for requiresCommandType callers (ROADMAP-2026-08-17 ML-4B): an entry
// with the guard marker in the correct command key but with a missing/wrong "type":"command" sibling
// still produces a match so the caller can emit the "entry is structurally malformed" violation.
func collectCommandsWithMarkerBySchema(parsed interface{}, schema hookCmdSchemaKind, marker string, out *[]guardCommandMatch) {
	root, ok := parsed.(map[string]interface{})
	if !ok {
		return
	}
	hooksRaw, ok := root["hooks"]
	if !ok {
		return
	}

	if schema == hookCmdSchemaKiro {
		// Kiro: hooks is a root-level array of {trigger, matcher, action:{type, command}} objects.
		arr, _ := hooksRaw.([]interface{})
		for _, entryRaw := range arr {
			entry, ok := entryRaw.(map[string]interface{})
			if !ok {
				continue
			}
			action, _ := entry["action"].(map[string]interface{})
			if action == nil {
				continue
			}
			typeStr, _ := action["type"].(string)
			cmd, _ := action["command"].(string)
			if cmd != "" && strings.Contains(cmd, marker) {
				*out = append(*out, guardCommandMatch{raw: cmd, typeIsCommand: typeStr == "command"})
			}
		}
		return
	}

	// For all other schemas: hooks is an object whose values are event arrays.
	hooksObj, _ := hooksRaw.(map[string]interface{})
	if hooksObj == nil {
		return
	}

	for _, evRaw := range hooksObj {
		entries, ok := evRaw.([]interface{})
		if !ok {
			continue
		}
		switch schema {
		case hookCmdSchemaNested:
			// Each entries[i] is a group object with optional "matcher" and inner "hooks" array.
			// The executable command lives at group["hooks"][j]["command"].
			for _, entryRaw := range entries {
				entry, ok := entryRaw.(map[string]interface{})
				if !ok {
					continue
				}
				innerHooks, _ := entry["hooks"].([]interface{})
				for _, ih := range innerHooks {
					innerObj, ok := ih.(map[string]interface{})
					if !ok {
						continue
					}
					typeStr, _ := innerObj["type"].(string)
					cmd, _ := innerObj["command"].(string)
					if cmd != "" && strings.Contains(cmd, marker) {
						*out = append(*out, guardCommandMatch{raw: cmd, typeIsCommand: typeStr == "command"})
					}
				}
			}
		case hookCmdSchemaFlat:
			// Each entries[i] IS the command entry; executable command at entry["command"].
			collectCommandsFromCommandKeyArray(entries, marker, out)
		case hookCmdSchemaFlatBash:
			// GitHub Copilot CLI global (~/.copilot/settings.json): command at entry["bash"].
			// mergeCredentialGuardCopilotHooks (update.go ML-2E) always writes "bash" key.
			for _, entryRaw := range entries {
				entry, ok := entryRaw.(map[string]interface{})
				if !ok {
					continue
				}
				typeStr, _ := entry["type"].(string)
				cmd, _ := entry["bash"].(string)
				if cmd != "" && strings.Contains(cmd, marker) {
					*out = append(*out, guardCommandMatch{raw: cmd, typeIsCommand: typeStr == "command"})
				}
			}
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
//   - A regra só avalia entradas que EXISTEM. Ausência de entrada em arquivo inexistente é estado
//     legítimo (CLI não instalado no projeto) — nunca é violação por si só. D15 (ADR-2026-10-04
//     adendo 2026-10-24): ausência em arquivo que EXISTE é sempre violação, mesmo com guard global.
//   - Arquivo de hook ausente é pulado em silêncio (não é responsabilidade desta regra garantir
//     que o arquivo exista).
//   - ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio, ML-1A:
//     arquivo de hook presente mas com JSON inválido deixou de ser pulado em silêncio — ver o
//     comentário no branch de erro do json.Unmarshal, abaixo, para a decisão e a medição que a
//     sustentam.
// validateGuardHookResolvable é a implementação genérica compartilhada pelas regras
// "credential_guard_hook_resolvable" e "git_branch_guard_hook_resolvable": para cada arquivo de
// hook de PROJETO que existir, extrai os comandos que referenciam scriptMarker (forma .sh legada)
// ou subcmdMarker (nova forma de subcomando Go), e:
//
// Para entradas .sh (scriptMarker): mantém o comportamento existente — verifica existência e
// executabilidade do script referenciado. A mensagem de aviso sobre Windows (a forma .sh não
// executa no Windows fora do Git Bash) é retornada por validateGuardHookShLegacyWarnings,
// roteada via applyRuleWarnOnly em validator.go para não elevar aviso a violação.
//
// Para entradas de subcomando (subcmdMarker, ML-2B, ADR-2026-10-04 D6):
//   - Linha exata da família do CLI → OK (contabilizado para sonda do binário).
//   - Contém subcmdMarker mas não é a linha exata → violation citando a linha esperada.
//   - Sonda do binário: resolve `trackfw` no PATH, verifica que tem o subcomando guard.
//   - Windows + família PS/POSIX + shim .ps1 + política Restricted → violation.
//
// subcmdName é o nome do subcomando (ex.: "credential", "git-branch").
func validateGuardHookResolvable(ruleName, scriptMarker, subcmdMarker, subcmdName string) ([]string, error) {
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
	// anySubcmdFormFound e hasPSPosixSubcmd rastreiam se algum arquivo usa a nova forma de
	// subcomando Go, para acionar a sonda do binário uma única vez ao final do loop (ML-2B).
	var anySubcmdFormFound bool
	var hasPSPosixSubcmd bool
	for _, hf := range credentialGuardHookFiles {
		fullPath := filepath.Join(root, hf.path)
		content, readErr := readRegularFile(fullPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			// ROADMAP-2026-09-06-...-config-ilegivel-deixa-de-ser-silencio, ML-1C: readRegularFile
			// (regularfile.go) replaces the bare os.ReadFile this ML started with — a FIFO placed
			// at fullPath made os.ReadFile block INDEFINITELY (hades-tf's ML-1B barrier,
			// `mkfifo .claude/settings.json`), which this branch could never reach because the
			// call never returned. readRegularFile's errNotRegularFile sentinel (never
			// os.IsNotExist) falls into this SAME accusing branch — no new branch needed here.
			//
			// ROADMAP-2026-09-06-...-config-ilegivel-deixa-de-ser-silencio, ML-1B: was
			// `return nil, fmt.Errorf(...)` — a hard error that aborted the ENTIRE `trackfw
			// validate` run (exit 1, empty stdout, no other rule reported) on the very first
			// unreadable guard file, for a reason as mundane as a permission bit or a directory
			// sitting where the file should be. hades-tf's ML-1A barrier reproduced this live
			// (chmod 000, and a directory in place of the file): Go crashed the whole command
			// while Node/Python silently reported success — two different failures of the same
			// AC ("config ilegível deixa de ser silêncio"), not just the JSON-parse one ML-1A
			// closed. Aborting is not accusing: the user sees no diagnostic at all. Reported as a
			// violation of THIS rule instead, same remedy as the invalid-JSON branch below —
			// `trackfw update` regenerates the file regardless of why it couldn't be read.
			msgs = append(msgs, fmt.Sprintf(
				"%s (%s) could not be read — trackfw cannot tell whether %s is wired here, and "+
					"%s cannot load any hook from a file it cannot read; fix the file, or run "+
					"`trackfw update` to regenerate it",
				hf.path, hf.cli, scriptMarker, hf.cli,
			))
			continue
		}

		// ML-1B: decoding, not just parsing, diverges across the 3 runtimes on this exact byte
		// stream — Go's json.Unmarshal operates on raw bytes and never panics on invalid UTF-8
		// (either the parse already fails for an unrelated structural reason, or the bad byte is
		// silently accepted inside a string), but Node's fs.readFileSync(path,'utf8') is a LOSSY
		// decode (invalid bytes become U+FFFD, never throws) while Python's
		// open(path,encoding="utf-8") is STRICT (raises UnicodeDecodeError before json.loads even
		// runs) — hades-tf's ML-1A barrier reproduced this live (whole file saved as UTF-16; one
		// invalid UTF-8 byte inside an otherwise-valid JSON string) and found Python crashing with
		// an unstructured traceback in both fixtures, exit 1, no JSON output. utf8.Valid records
		// whether THIS content would decode losslessly, purely to pick which message to emit
		// below if parsing goes on to fail — it never gates whether parsing is attempted, so the
		// "1 invalid byte, otherwise-valid JSON" fixture keeps parsing (and staying silent) exactly
		// as before; only the message classification for an outright parse failure changes.
		wasValidUTF8 := utf8.Valid(content)

		var parsed interface{}
		if jsonErr := json.Unmarshal(content, &parsed); jsonErr != nil {
			// ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio,
			// ML-1A: was a silent `continue` (fail-open) — a corrupted hook file made this rule
			// report health about wiring it never actually inspected. Decision (measured, not
			// assumed): accuse as a violation of THIS rule, not a separate diagnostic. Measurement:
			// credentialGuardHookFiles is a closed, enumerable list of files trackfw
			// itself either writes or merges into — none of them is third-party or free-form
			// config trackfw doesn't own the schema of. Invalid JSON in any of them is never a
			// legitimate state: the owning CLI (hf.cli) requires strict JSON to load ITS OWN hooks
			// from that file, so a parse failure already means every hook wired there — ours
			// included, whether or not scriptMarker happens to be present — is dead, independent
			// of trackfw. This mirrors internal/generators/agentfiles.go's InjectClaudeHooks, which
			// already treats the same condition as a hard error on the WRITE path
			// ("parsing %s: %w") — the read path (this function) was the only place still fail-open
			// on it. A "diagnostic, not violation" response was rejected: it would still leave the
			// control silent by default (validate exits 0), which is the exact defect this ML
			// closes.
			//
			// ML-1B: a parse failure caused by content that isn't valid UTF-8 to begin with (e.g.
			// a file saved as UTF-16) gets its own message — "not valid JSON" would send a user
			// staring at JSON that looks fine in their editor investigating the wrong thing (D4 of
			// ADR-2026-09-04, same principle: classify right, explain right, or the reader
			// investigates the wrong thing).
			if !wasValidUTF8 {
				msgs = append(msgs, fmt.Sprintf(
					"%s (%s) is not valid UTF-8 — trackfw cannot tell whether %s is wired here, "+
						"and %s cannot load any hook from a file it cannot decode; fix the file's "+
						"encoding, or run `trackfw update` to regenerate it",
					hf.path, hf.cli, scriptMarker, hf.cli,
				))
				continue
			}
			msgs = append(msgs, fmt.Sprintf(
				"%s (%s) is not valid JSON — trackfw cannot tell whether %s is wired here, and "+
					"%s cannot load any hook from a file it cannot parse; fix the file, or run "+
					"`trackfw update` to regenerate it",
				hf.path, hf.cli, scriptMarker, hf.cli,
			))
			continue
		}

		// --- FORMA LEGADA (.sh) --- mantem comportamento existente ---
		// D16 (ADR-2026-10-04): only recognize the marker in the executable-command field for
		// this CLI's hook schema (guardWiringHookLayout). A marker in any other field is not wiring.
		var commands []guardCommandMatch
		collectCommandsWithMarkerBySchema(parsed, guardWiringHookLayout[hf.path], scriptMarker, &commands)

		seen := make(map[string]bool, len(commands))
		for _, m := range commands {
			seenKey := m.raw + "\x00" + strconv.FormatBool(m.typeIsCommand)
			if seen[seenKey] {
				continue
			}
			seen[seenKey] = true

			// ADR-2026-08-22: classificar por ancoragem ANTES de resolver. Aspas externas são
			// sintaxe, não semântica — removê-las antes de classificar garante que "$PWD/…"
			// (entre aspas) receba o mesmo veredito que $PWD/… sem aspas (achado D.3).
			// wasQuoted é necessário para distinguir ~/… (classe 1) de "~/…" (classe 2): o til
			// expande para $HOME sem aspas mas permanece literal dentro de aspas duplas.
			wasQuoted := hookValueWasQuoted(m.raw)
			rawStripped := stripOuterQuotesForClassify(m.raw)
			class := classifyHookAnchorage(rawStripped, wasQuoted)

			// Classe 2 (dependente do cwd) + CLI que exige ancoragem → acusar com mensagem
			// específica por forma. Cursor/Copilot/Kiro têm requiresVarOrShellPrefix=false e
			// nunca entram aqui — falso-positivo eliminado por construção (AC3 da REQ).
			if hf.requiresVarOrShellPrefix && class == hookAnchorageClassCwdDependent {
				msgs = append(msgs, fmt.Sprintf(
					"%s (%s) references %s %s",
					hf.path, hf.cli, scriptMarker,
					cwdDependentReason(rawStripped),
				))
				continue
			}

			// Classe 1 (ancorado) e classe 3 (indecidível) prosseguem para a resolução existente.
			// Classe 1 com forma absoluta resolve com ok=false (default branch) e é silenciada
			// por construção — caminho absoluto ancora, é wiring legítimo (ADR-2026-08-22).
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
			case CurrentGOOS != "windows" && info.Mode()&0111 == 0:
				msgs = append(msgs, fmt.Sprintf(
					"%s (%s) references %s resolved to %q, but the script is not executable — run `trackfw update` to regenerate it",
					hf.path, hf.cli, scriptMarker, resolved,
				))
			}
		}

		// --- NOVA FORMA (subcomando Go) --- ML-2B, ADR-2026-10-04 D6 ---
		// D16: same schema-aware restriction applies to the subcmd marker.
		if subcmdMarker != "" {
			var subcmdCommands []guardCommandMatch
			collectCommandsWithMarkerBySchema(parsed, guardWiringHookLayout[hf.path], subcmdMarker, &subcmdCommands)
			expectedLine := guardExpectedLine(subcmdName, hf.family)

			seenSubcmd := make(map[string]bool, len(subcmdCommands))
			for _, m := range subcmdCommands {
				seenKey := m.raw + "\x00" + strconv.FormatBool(m.typeIsCommand)
				if seenSubcmd[seenKey] {
					continue
				}
				seenSubcmd[seenKey] = true

				// Aplicar o mesmo filtro requiresCommandType da forma .sh: uma entrada sem
				// "type":"command" num CLI que o exige nunca executa — viola antes de checar a linha.
				if hf.requiresCommandType && !m.typeIsCommand {
					msgs = append(msgs, fmt.Sprintf(
						`%s (%s) references %q, but the hook entry is missing "type":"command" (or has an invalid type) — %s will silently never execute it; run `+"`trackfw update`"+` to regenerate it`,
						hf.path, hf.cli, m.raw, hf.cli,
					))
					continue
				}

				legacyD2Line := guardD2LegacyLine(subcmdName, hf.family)
				legacyD11Line := guardD11LegacyLine(subcmdName, hf.family)
				legacyD11RevLine := guardD11RevisedLegacyLine(subcmdName, hf.family)
				if m.raw == expectedLine {
					// Linha exata REQ-2026-10-20 fail-closed → OK; marcar para sonda do binário.
					anySubcmdFormFound = true
					if hf.family == guardShellFamilyPSPosix {
						hasPSPosixSubcmd = true
					}
				} else if m.raw == legacyD11RevLine && m.raw != expectedLine {
					// REQ-2026-10-20: D11 revised pré-REQ-2026-10-20 (sem prefixo) — aborta no zsh.
					// Tratada como warning (via validateGuardHookD11RevisedLegacyWarnings), não violation.
					anySubcmdFormFound = true
					if hf.family == guardShellFamilyPSPosix {
						hasPSPosixSubcmd = true
					}
				} else if m.raw == legacyD11Line && m.raw != expectedLine {
					// ML-6C: D11 pré-ML-6C (sem 4º trecho PS) — fail-open em PS para RC≠{0,2}.
					// Tratada como warning (via validateGuardHookD11LegacyWarnings), não violation.
					anySubcmdFormFound = true
					if hf.family == guardShellFamilyPSPosix {
						hasPSPosixSubcmd = true
					}
				} else if m.raw == legacyD2Line {
					// ML-6B: D2 revised form — fail-open quando trackfw ausente do PATH.
					// Tratada como warning (via validateGuardHookD2InlineWarnings), não violation.
					anySubcmdFormFound = true
					if hf.family == guardShellFamilyPSPosix {
						hasPSPosixSubcmd = true
					}
				} else {
					// Contém o marcador mas não é a linha exata → violation com linha esperada.
					msgs = append(msgs, fmt.Sprintf(
						"%s (%s) has %q but the expected form for this CLI is %q — "+
							"run `trackfw update` to regenerate",
						hf.path, hf.cli, m.raw, expectedLine,
					))
					anySubcmdFormFound = true
					if hf.family == guardShellFamilyPSPosix {
						hasPSPosixSubcmd = true
					}
				}
			}
		}
	}

	// Sonda do binário — executada uma vez se qualquer arquivo de hook usa a nova forma.
	if anySubcmdFormFound {
		probeMsgs := guardBinaryProbeOnce(hasPSPosixSubcmd)
		msgs = append(msgs, probeMsgs...)
	}

	return msgs, nil
}

// validateGuardHookShLegacyWarnings retorna avisos (always-warning, roteados via
// applyRuleWarnOnly) para arquivos de hook de PROJETO que ainda usam a forma legada .sh.
// A forma .sh não executa no Windows fora do Git Bash (ADR-2026-10-04, D5/D6).
// Os avisos são separados das violations para não elevar o nível da regra.
func validateGuardHookShLegacyWarnings(scriptMarker string) ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	var warnings []string
	for _, hf := range credentialGuardHookFiles {
		fullPath := filepath.Join(root, hf.path)
		content, readErr := readRegularFile(fullPath)
		if readErr != nil {
			continue // arquivos ilegíveis/ausentes já são tratados pela função principal
		}
		var parsed interface{}
		if json.Unmarshal(content, &parsed) != nil {
			continue // JSON inválido já é tratado pela função principal
		}

		var commands []guardCommandMatch
		// D16: schema-aware — only the executable-command field for this CLI counts.
		collectCommandsWithMarkerBySchema(parsed, guardWiringHookLayout[hf.path], scriptMarker, &commands)

		seen := make(map[string]bool, len(commands))
		for _, m := range commands {
			seenKey := m.raw
			if seen[seenKey] {
				continue
			}
			seen[seenKey] = true
			warnings = append(warnings, fmt.Sprintf(
				"%s (%s) references %s — this script does not execute on Windows outside Git Bash; "+
					"run `trackfw update` to migrate to the `trackfw guard` subcommand form",
				hf.path, hf.cli, scriptMarker,
			))
			break // um aviso por arquivo é suficiente
		}
	}
	return warnings, nil
}

// credentialGuardRequiredEvent describes a single event or matcher within a hook file where the
// credential guard MUST be present after ML-1B (REQ-2026-10-06). For Windsurf, events are top-level
// keys under "hooks"; for Amazon Q, events are "matcher" values inside hooks.preToolUse entries.
// D17 (ADR-2026-10-04 adendo 2026-10-25, REQ-2026-10-25 #619 ML-1A): extends to the 6 CLIs that
// carry a warning (not violation) via validateCredentialGuardPresenceWarnings.
type credentialGuardRequiredEvent struct {
	// hookEvent is the event key within the top-level "hooks" object (e.g. "pre_run_command").
	hookEvent string
	// amazonQMatcher, when non-empty, switches to Amazon Q's preToolUse[matcher] extraction mode:
	// instead of hooks[hookEvent], we look for preToolUse entries with this "matcher" value.
	// Legacy field — only used by the two warnOnly=false entries (Amazon Q). Prefer using the
	// matcher field for new warnOnly=true entries.
	amazonQMatcher string
	// matcher is the exact "matcher" string to look for in the hook entry (D17). Empty means no
	// matcher filtering (e.g. Cursor's beforeShellExecution, Windsurf pre_run_command).
	matcher string
}

// credentialGuardRequiredEntry pairs a hook file with the list of events that must carry the guard.
// D17 (ADR-2026-10-04 adendo 2026-10-25, REQ-2026-10-25 #619): warnOnly=false → violation (Windsurf,
// Amazon Q); warnOnly=true → warning for 6 CLIs (Claude, Codex, Gemini, Cursor, Copilot, Kiro).
// schema is the extraction schema for the presence check (used by warnOnly=true entries only).
type credentialGuardRequiredEntry struct {
	hookFile credentialGuardHookFile
	events   []credentialGuardRequiredEvent
	// warnOnly: false = violation (handled by validateCredentialGuardPresenceRequired),
	//           true = warning-only (handled by validateCredentialGuardPresenceWarnings).
	warnOnly bool
	// schema: hookCmdSchemaNested / hookCmdSchemaFlat / hookCmdSchemaKiro — D17 extraction.
	// Only consulted when warnOnly=true. Windsurf/Amazon Q use extractCredentialGuardCommandsFromEvent.
	schema hookCmdSchemaKind
}

// credentialGuardRequiredEntries is the closed list of (file, events) pairs where the credential
// guard is MANDATORY (violation, ML-1B, REQ-2026-10-06) or advisory (warning, D17, REQ-2026-10-25).
// Each event is checked independently. warnOnly=false → violation; warnOnly=true → warning only.
//
// This is the SINGLE SOURCE OF TRUTH for all CLIs (AC5, REQ-2026-10-25): every CLI in
// credentialGuardHookFiles (except Windsurf and Amazon Q which are warnOnly=false) has a
// warnOnly=true entry here. validateCredentialGuardPresenceRequired skips warnOnly=true;
// validateCredentialGuardPresenceWarnings processes only warnOnly=true.
var credentialGuardRequiredEntries = []credentialGuardRequiredEntry{
	// ── Violation-grade entries (warnOnly=false) ────────────────────────────────
	{
		hookFile: credentialGuardHookFile{".windsurf/hooks.json", "Windsurf", false, false, guardShellFamilyPSPosix},
		events: []credentialGuardRequiredEvent{
			{hookEvent: "pre_run_command"},
			{hookEvent: "pre_write_code"},
		},
		warnOnly: false,
		// D15 (ADR-2026-10-04 adendo 2026-10-24): project-scope wiring always required.
	},
	{
		hookFile: credentialGuardHookFile{".amazonq/cli-agents/q_cli_default.json", "Amazon Q", false, false, guardShellFamilyCmdExe},
		events: []credentialGuardRequiredEvent{
			{hookEvent: "preToolUse", amazonQMatcher: "execute_bash"},
			{hookEvent: "preToolUse", amazonQMatcher: "fs_write"},
		},
		warnOnly: false,
	},
	// ── Warning-grade entries (warnOnly=true, D17) ──────────────────────────────
	// Matchers and events are EXACT strings from the generator (confirmed in Wave 0
	// docs/seguranca/2026-10-25-wave0-aviso-presenca-credential-guard.md §1.2).
	{
		hookFile: credentialGuardHookFile{".claude/settings.json", "Claude Code", true, true, guardShellFamilyPSPosix},
		events: []credentialGuardRequiredEvent{
			{hookEvent: "PreToolUse", matcher: "Bash|PowerShell"},
			{hookEvent: "PreToolUse", matcher: "Read"},
			{hookEvent: "PreToolUse", matcher: "Write|Edit"},
		},
		warnOnly: true,
		schema:   hookCmdSchemaNested,
	},
	{
		hookFile: credentialGuardHookFile{".codex/hooks.json", "Codex CLI", true, true, guardShellFamilyPSPosix},
		events: []credentialGuardRequiredEvent{
			{hookEvent: "PreToolUse", matcher: "Bash"},
			{hookEvent: "PreToolUse", matcher: "apply_patch"},
		},
		warnOnly: true,
		schema:   hookCmdSchemaNested,
	},
	{
		hookFile: credentialGuardHookFile{".gemini/settings.json", "Gemini CLI", true, true, guardShellFamilyPSPosix},
		events: []credentialGuardRequiredEvent{
			{hookEvent: "BeforeTool", matcher: "run_shell_command"},
			{hookEvent: "BeforeTool", matcher: "read_file|read_many_files"},
			{hookEvent: "BeforeTool", matcher: "write_file|replace"},
		},
		warnOnly: true,
		schema:   hookCmdSchemaNested,
	},
	{
		hookFile: credentialGuardHookFile{".cursor/hooks.json", "Cursor", false, false, guardShellFamilyPSPosix},
		events: []credentialGuardRequiredEvent{
			{hookEvent: "beforeShellExecution"}, // no matcher — entire event is guarded
			{hookEvent: "preToolUse", matcher: "Read"},
			{hookEvent: "preToolUse", matcher: "Write"},
		},
		warnOnly: true,
		schema:   hookCmdSchemaFlat,
	},
	{
		hookFile: credentialGuardHookFile{".github/hooks/trackfw-attention.json", "GitHub Copilot CLI", true, false, guardShellFamilyPSPosix},
		events: []credentialGuardRequiredEvent{
			{hookEvent: "preToolUse", matcher: "bash"},
			{hookEvent: "preToolUse", matcher: "view"},
			{hookEvent: "preToolUse", matcher: "create|edit"},
		},
		warnOnly: true,
		schema:   hookCmdSchemaFlat,
	},
	{
		hookFile: credentialGuardHookFile{".kiro/hooks/trackfw-attention.json", "Kiro", true, false, guardShellFamilyCmdExe},
		events: []credentialGuardRequiredEvent{
			{hookEvent: "PreToolUse", matcher: "shell"},
			{hookEvent: "PreToolUse", matcher: "read"},
			{hookEvent: "PreToolUse", matcher: "write"},
		},
		warnOnly: true,
		schema:   hookCmdSchemaKiro,
	},
}

// extractCredentialGuardCommandsFromEvent extracts all command strings from a single event slot.
// For Windsurf-style: parsed["hooks"][event] is a []interface{} of {"command":...} objects.
// For Amazon Q-style: parsed["hooks"]["preToolUse"] is a []interface{} of {matcher:...,hooks:[...]}
// entries; we locate the entry whose "matcher" equals amazonQMatcher and recurse into its "hooks".
func extractCredentialGuardCommandsFromEvent(parsed interface{}, event credentialGuardRequiredEvent, marker string) []guardCommandMatch {
	root, ok := parsed.(map[string]interface{})
	if !ok {
		return nil
	}
	hooks, _ := root["hooks"].(map[string]interface{})
	if hooks == nil {
		return nil
	}

	if event.amazonQMatcher == "" {
		// Windsurf-style: hooks[event] is a flat array of {"command":"..."} objects
		// (hookCmdSchemaFlat). D16: only the "command" key counts as wiring.
		var out []guardCommandMatch
		collectCommandsFromCommandKeyArray(hooks[event.hookEvent], marker, &out)
		return out
	}

	// Amazon Q-style: hooks.preToolUse is [{matcher:..., hooks:[{type:"command",command:...}]}, ...]
	// (hookCmdSchemaNested). After locating the matcher entry, the inner hooks[] uses the same
	// {"type":"command","command":"..."} shape — "command" key only. D16: only that key is wiring.
	preToolUse, _ := hooks[event.hookEvent].([]interface{})
	for _, item := range preToolUse {
		obj, _ := item.(map[string]interface{})
		if obj == nil {
			continue
		}
		if obj["matcher"] != event.amazonQMatcher {
			continue
		}
		var out []guardCommandMatch
		collectCommandsFromCommandKeyArray(obj["hooks"], marker, &out)
		return out
	}
	return nil
}

// validateCredentialGuardPresenceRequired emite violations quando um arquivo de hook dos CLIs em
// credentialGuardRequiredEntries existe no disco mas falta o credential guard em algum dos eventos
// obrigatórios. Cada evento ausente produz uma mensagem separada (nomeando arquivo + evento).
// Isso captura arquivos gerados por versões anteriores ao ML-1B que só têm o git-branch guard, bem
// como arquivos com o guard em pre_run_command mas não em pre_write_code (ou vice-versa).
//
// D15 (ADR-2026-10-04 adendo 2026-10-24): project-scope wiring is always required — the validator
// no longer accepts absence due to global guard installation. Both generator and validator treat
// project scope unconditionally.
func validateCredentialGuardPresenceRequired() ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if resolvedRoot, symErr := filepath.EvalSymlinks(root); symErr == nil {
		root = resolvedRoot
	}

	var msgs []string
	for _, entry := range credentialGuardRequiredEntries {
		if entry.warnOnly {
			continue // D17: handled by validateCredentialGuardPresenceWarnings
		}
		hf := entry.hookFile
		fullPath := filepath.Join(root, hf.path)
		content, readErr := readRegularFile(fullPath)
		if readErr != nil {
			// Arquivo ausente ou ilegível: já tratado por validateGuardHookResolvable; não duplicar.
			continue
		}
		var parsed interface{}
		if json.Unmarshal(content, &parsed) != nil {
			// JSON inválido: já tratado por validateGuardHookResolvable.
			continue
		}

		for _, ev := range entry.events {
			shCmds := extractCredentialGuardCommandsFromEvent(parsed, ev, credentialGuardScriptMarker)
			subcmdCmds := extractCredentialGuardCommandsFromEvent(parsed, ev, credentialGuardSubcmdMarker)

			if len(shCmds) == 0 && len(subcmdCmds) == 0 {
				eventDesc := ev.hookEvent
				if ev.amazonQMatcher != "" {
					eventDesc = ev.hookEvent + "[" + ev.amazonQMatcher + "]"
				}
				msgs = append(msgs, fmt.Sprintf(
					"%s (%s) is present but has no credential guard entry in event %s — "+
						"run `trackfw update` to add the credential guard wiring",
					hf.path, hf.cli, eventDesc,
				))
			}
		}
	}
	return msgs, nil
}

// extractPresenceCommandsD17 extracts credential-guard command strings from a single (event, matcher)
// slot of a parsed hook file, using the extraction schema defined by schema. Both the .sh legacy form
// (credentialGuardScriptMarker) and the Go subcommand form (credentialGuardSubcmdMarker) are
// recognised. requiresCommandType=true means an entry without "type":"command" is NOT counted.
//
// Three schemas are supported (D17, REQ-2026-10-25 Wave 0 §1.3):
//   - hookCmdSchemaNested (Claude, Codex, Gemini): hooks[event] is [{matcher, hooks:[{type,command}]}];
//     filter by group["matcher"]==event.matcher, then read group["hooks"][i]["command"].
//   - hookCmdSchemaFlat (Cursor, Copilot project): hooks[event] is [{type,matcher,command}];
//     filter by entry["matcher"]==event.matcher (skip filter when event.matcher is empty).
//   - hookCmdSchemaKiro: root["hooks"] is [{trigger,matcher,action:{type,command}}];
//     filter by trigger==event.hookEvent && matcher==event.matcher.
func extractPresenceCommandsD17(parsed interface{}, event credentialGuardRequiredEvent, schema hookCmdSchemaKind, requiresCommandType bool) []guardCommandMatch {
	root, ok := parsed.(map[string]interface{})
	if !ok {
		return nil
	}

	containsMarker := func(cmd string) bool {
		return strings.Contains(cmd, credentialGuardSubcmdMarker) ||
			strings.Contains(cmd, credentialGuardScriptMarker)
	}

	switch schema {
	case hookCmdSchemaNested:
		// Claude, Codex, Gemini: hooks[event] is [{matcher, hooks:[{type, command}]}]
		hooksObj, _ := root["hooks"].(map[string]interface{})
		if hooksObj == nil {
			return nil
		}
		evArr, _ := hooksObj[event.hookEvent].([]interface{})
		var out []guardCommandMatch
		for _, groupRaw := range evArr {
			group, _ := groupRaw.(map[string]interface{})
			if group == nil {
				continue
			}
			if event.matcher != "" {
				m, _ := group["matcher"].(string)
				if m != event.matcher {
					continue
				}
			}
			innerHooks, _ := group["hooks"].([]interface{})
			for _, ih := range innerHooks {
				innerObj, _ := ih.(map[string]interface{})
				if innerObj == nil {
					continue
				}
				typeStr, _ := innerObj["type"].(string)
				cmd, _ := innerObj["command"].(string)
				if cmd == "" || !containsMarker(cmd) {
					continue
				}
				if requiresCommandType && typeStr != "command" {
					continue
				}
				out = append(out, guardCommandMatch{raw: cmd, typeIsCommand: typeStr == "command"})
			}
		}
		return out

	case hookCmdSchemaFlat:
		// Cursor, Copilot project: hooks[event] is [{type, matcher, command}]
		hooksObj, _ := root["hooks"].(map[string]interface{})
		if hooksObj == nil {
			return nil
		}
		evArr, _ := hooksObj[event.hookEvent].([]interface{})
		var out []guardCommandMatch
		for _, entryRaw := range evArr {
			entry, _ := entryRaw.(map[string]interface{})
			if entry == nil {
				continue
			}
			if event.matcher != "" {
				m, _ := entry["matcher"].(string)
				if m != event.matcher {
					continue
				}
			}
			typeStr, _ := entry["type"].(string)
			cmd, _ := entry["command"].(string)
			if cmd == "" || !containsMarker(cmd) {
				continue
			}
			if requiresCommandType && typeStr != "command" {
				continue
			}
			out = append(out, guardCommandMatch{raw: cmd, typeIsCommand: typeStr == "command"})
		}
		return out

	case hookCmdSchemaKiro:
		// Kiro: root["hooks"] is [{trigger, matcher, action:{type, command}}]
		arr, _ := root["hooks"].([]interface{})
		var out []guardCommandMatch
		for _, entryRaw := range arr {
			entry, _ := entryRaw.(map[string]interface{})
			if entry == nil {
				continue
			}
			trigger, _ := entry["trigger"].(string)
			if trigger != event.hookEvent {
				continue
			}
			if event.matcher != "" {
				m, _ := entry["matcher"].(string)
				if m != event.matcher {
					continue
				}
			}
			action, _ := entry["action"].(map[string]interface{})
			if action == nil {
				continue
			}
			typeStr, _ := action["type"].(string)
			cmd, _ := action["command"].(string)
			if cmd == "" || !containsMarker(cmd) {
				continue
			}
			if requiresCommandType && typeStr != "command" {
				continue
			}
			out = append(out, guardCommandMatch{raw: cmd, typeIsCommand: typeStr == "command"})
		}
		return out
	}
	return nil
}

// validateCredentialGuardPresenceWarnings emits WARNINGS (never violations) when a project hook
// file for one of the 6 CLIs without a violation-grade presence rule exists on disk but is missing
// the credential guard in one of the expected (event, matcher) pairs.
//
// D17 (ADR-2026-10-04 adendo 2026-10-25, REQ-2026-10-25 #619 ML-1A): warning-only, never blockable.
// Registered via applyRuleWarnOnly("credential_guard_presence_warning", ...) in validator.go.
// The violation-grade CLIs (Windsurf, Amazon Q) are handled by validateCredentialGuardPresenceRequired
// via validateCredentialGuardHookResolvable.
func validateCredentialGuardPresenceWarnings() ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if resolvedRoot, symErr := filepath.EvalSymlinks(root); symErr == nil {
		root = resolvedRoot
	}

	var msgs []string
	for _, entry := range credentialGuardRequiredEntries {
		if !entry.warnOnly {
			continue // D17: violations handled by validateCredentialGuardPresenceRequired
		}
		hf := entry.hookFile
		fullPath := filepath.Join(root, hf.path)
		content, readErr := readRegularFile(fullPath)
		if readErr != nil {
			// Absent or unreadable: no project hook installed, warning is inappropriate.
			continue
		}
		var parsed interface{}
		if json.Unmarshal(content, &parsed) != nil {
			// Invalid JSON: already reported by validateGuardHookResolvable.
			continue
		}

		for _, ev := range entry.events {
			matches := extractPresenceCommandsD17(parsed, ev, entry.schema, hf.requiresCommandType)
			if len(matches) == 0 {
				eventDesc := ev.hookEvent
				if ev.matcher != "" {
					eventDesc = ev.hookEvent + "[" + ev.matcher + "]"
				}
				msgs = append(msgs, fmt.Sprintf(
					"%s (%s) is present but has no credential guard entry in event %s — "+
						"run `trackfw update` to add the credential guard wiring",
					hf.path, hf.cli, eventDesc,
				))
			}
		}
	}
	return msgs, nil
}

// validateCredentialGuardHookResolvable é a regra "credential_guard_hook_resolvable" — ver
// validateGuardHookResolvable para a implementação compartilhada.
func validateCredentialGuardHookResolvable() ([]string, error) {
	formMsgs, err := validateGuardHookResolvable("credential_guard_hook_resolvable", credentialGuardScriptMarker, credentialGuardSubcmdMarker, "credential")
	if err != nil {
		return nil, err
	}
	presenceMsgs, err := validateCredentialGuardPresenceRequired()
	if err != nil {
		return nil, err
	}
	return append(formMsgs, presenceMsgs...), nil
}

// validateCredentialGuardHookResolvableLegacyWarnings retorna os avisos de forma legada
// (.sh não executa no Windows) para a regra credential_guard_hook_resolvable.
func validateCredentialGuardHookResolvableLegacyWarnings() ([]string, error) {
	return validateGuardHookShLegacyWarnings(credentialGuardScriptMarker)
}

// validateGitBranchGuardHookResolvable é a regra "git_branch_guard_hook_resolvable" — ver
// validateGuardHookResolvable para a implementação compartilhada.
func validateGitBranchGuardHookResolvable() ([]string, error) {
	return validateGuardHookResolvable("git_branch_guard_hook_resolvable", gitBranchGuardScriptMarker, gitBranchGuardSubcmdMarker, "git-branch")
}

// validateGitBranchGuardHookResolvableLegacyWarnings retorna os avisos de forma legada
// (.sh não executa no Windows) para a regra git_branch_guard_hook_resolvable.
func validateGitBranchGuardHookResolvableLegacyWarnings() ([]string, error) {
	return validateGuardHookShLegacyWarnings(gitBranchGuardScriptMarker)
}

// validateGuardHookD2InlineWarnings retorna avisos (always-warning, roteados via applyRuleWarnOnly)
// para arquivos de hook de PROJETO que ainda usam a forma D2-revised inline
// ("trackfw guard <nome>; exit $LASTEXITCODE" / "trackfw guard <nome>").
// A forma D2 falha aberta quando trackfw está ausente do PATH (ADR-2026-10-04 D11 / ML-6B).
func validateGuardHookD2InlineWarnings(subcmdMarker, subcmdName string) ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if resolvedRoot, symErr := filepath.EvalSymlinks(root); symErr == nil {
		root = resolvedRoot
	}
	var warnings []string
	for _, hf := range credentialGuardHookFiles {
		fullPath := filepath.Join(root, hf.path)
		content, readErr := readRegularFile(fullPath)
		if readErr != nil {
			continue
		}
		var parsed interface{}
		if json.Unmarshal(content, &parsed) != nil {
			continue
		}
		var subcmdCommands []guardCommandMatch
		collectCommandsWithMarkerBySchema(parsed, guardWiringHookLayout[hf.path], subcmdMarker, &subcmdCommands)
		legacyD2Line := guardD2LegacyLine(subcmdName, hf.family)
		expectedLine := guardExpectedLine(subcmdName, hf.family)
		for _, m := range subcmdCommands {
			if m.raw == legacyD2Line && m.raw != expectedLine {
				warnings = append(warnings, fmt.Sprintf(
					"%s (%s) has %q — linha antiga, falha aberta sem o trackfw no PATH; "+
						"rode `trackfw update` para migrar para a forma D11 fail-closed",
					hf.path, hf.cli, m.raw,
				))
				break // um aviso por arquivo é suficiente
			}
		}
	}
	return warnings, nil
}

// validateCredentialGuardHookD2InlineWarnings retorna os avisos de forma D2 legada
// para a regra credential_guard_hook_resolvable.
func validateCredentialGuardHookD2InlineWarnings() ([]string, error) {
	return validateGuardHookD2InlineWarnings(credentialGuardSubcmdMarker, "credential")
}

// validateGitBranchGuardHookD2InlineWarnings retorna os avisos de forma D2 legada
// para a regra git_branch_guard_hook_resolvable.
func validateGitBranchGuardHookD2InlineWarnings() ([]string, error) {
	return validateGuardHookD2InlineWarnings(gitBranchGuardSubcmdMarker, "git-branch")
}

// validateGuardHookD11LegacyWarnings retorna avisos (always-warning, roteados via applyRuleWarnOnly)
// para arquivos de hook de PROJETO que ainda usam a forma D11 pré-ML-6C
// (sem o 4º trecho de normalização PowerShell).
// A forma D11 pré-ML-6C passa saídas 1 e 3 do trackfw íntegras no PowerShell 5.1
// (ADR-2026-10-04 D11 revista / ML-6C).
func validateGuardHookD11LegacyWarnings(subcmdMarker, subcmdName string) ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if resolvedRoot, symErr := filepath.EvalSymlinks(root); symErr == nil {
		root = resolvedRoot
	}
	var warnings []string
	for _, hf := range credentialGuardHookFiles {
		fullPath := filepath.Join(root, hf.path)
		content, readErr := readRegularFile(fullPath)
		if readErr != nil {
			continue
		}
		var parsed interface{}
		if json.Unmarshal(content, &parsed) != nil {
			continue
		}
		var subcmdCommands []guardCommandMatch
		collectCommandsWithMarkerBySchema(parsed, guardWiringHookLayout[hf.path], subcmdMarker, &subcmdCommands)
		legacyD11Line := guardD11LegacyLine(subcmdName, hf.family)
		expectedLine := guardExpectedLine(subcmdName, hf.family)
		for _, m := range subcmdCommands {
			if m.raw == legacyD11Line && m.raw != expectedLine {
				warnings = append(warnings, fmt.Sprintf(
					"%s (%s) has %q — forma D11 pré-ML-6C, passa saídas ≠{0,2} íntegras no PowerShell; "+
						"rode `trackfw update` para migrar para a forma D11 revista",
					hf.path, hf.cli, m.raw,
				))
				break // um aviso por arquivo é suficiente
			}
		}
	}
	return warnings, nil
}

// validateCredentialGuardHookD11LegacyWarnings retorna os avisos de forma D11 pré-ML-6C
// para a regra credential_guard_hook_resolvable.
func validateCredentialGuardHookD11LegacyWarnings() ([]string, error) {
	return validateGuardHookD11LegacyWarnings(credentialGuardSubcmdMarker, "credential")
}

// validateGitBranchGuardHookD11LegacyWarnings retorna os avisos de forma D11 pré-ML-6C
// para a regra git_branch_guard_hook_resolvable.
func validateGitBranchGuardHookD11LegacyWarnings() ([]string, error) {
	return validateGuardHookD11LegacyWarnings(gitBranchGuardSubcmdMarker, "git-branch")
}

// validateGuardHookD11RevisedLegacyWarnings retorna avisos (always-warning, roteados via
// applyRuleWarnOnly) para arquivos de hook de PROJETO que ainda usam a forma D11 revised
// pré-REQ-2026-10-20 (com 4º trecho PS mas sem prefixo emulate -R sh).
// A forma pré-REQ-2026-10-20 aborta o zsh com exit 1 antes do guard rodar (fail-open no zsh).
func validateGuardHookD11RevisedLegacyWarnings(subcmdMarker, subcmdName string) ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if resolvedRoot, symErr := filepath.EvalSymlinks(root); symErr == nil {
		root = resolvedRoot
	}
	var warnings []string
	for _, hf := range credentialGuardHookFiles {
		fullPath := filepath.Join(root, hf.path)
		content, readErr := readRegularFile(fullPath)
		if readErr != nil {
			continue
		}
		var parsed interface{}
		if json.Unmarshal(content, &parsed) != nil {
			continue
		}
		var subcmdCommands []guardCommandMatch
		collectCommandsWithMarkerBySchema(parsed, guardWiringHookLayout[hf.path], subcmdMarker, &subcmdCommands)
		legacyD11RevLine := guardD11RevisedLegacyLine(subcmdName, hf.family)
		expectedLine := guardExpectedLine(subcmdName, hf.family)
		for _, m := range subcmdCommands {
			if m.raw == legacyD11RevLine && m.raw != expectedLine {
				warnings = append(warnings, fmt.Sprintf(
					"%s (%s) has %q — forma D11 revised pré-REQ-2026-10-20, aborta no zsh (EQUALS/NOMATCH); "+
						"rode `trackfw update` para migrar para a forma REQ-2026-10-20 com o prefixo `emulate -R sh`",
					hf.path, hf.cli, m.raw,
				))
				break // um aviso por arquivo é suficiente
			}
		}
	}
	return warnings, nil
}

// validateCredentialGuardHookD11RevisedLegacyWarnings retorna os avisos de forma D11 revised
// pré-REQ-2026-10-20 para a regra credential_guard_hook_resolvable.
func validateCredentialGuardHookD11RevisedLegacyWarnings() ([]string, error) {
	return validateGuardHookD11RevisedLegacyWarnings(credentialGuardSubcmdMarker, "credential")
}

// validateGitBranchGuardHookD11RevisedLegacyWarnings retorna os avisos de forma D11 revised
// pré-REQ-2026-10-20 para a regra git_branch_guard_hook_resolvable.
func validateGitBranchGuardHookD11RevisedLegacyWarnings() ([]string, error) {
	return validateGuardHookD11RevisedLegacyWarnings(gitBranchGuardSubcmdMarker, "git-branch")
}

// claudeGuardPowerShellSubcmdMarkers is the set of Claude Code guard subcmd prefixes
// that must be covered by the "Bash|PowerShell" matcher (ML-5A / REQ-2026-09-05).
// Using the same marker strings as credentialGuardSubcmdMarker / gitBranchGuardSubcmdMarker.
var claudeGuardPowerShellSubcmdMarkers = []string{
	credentialGuardSubcmdMarker, // "trackfw guard credential"
	gitBranchGuardSubcmdMarker,  // "trackfw guard git-branch"
}

// validateClaudeGuardHookMatcherWarningsInFile walks a single parsed Claude Code
// settings.json (decoded as interface{}) and returns a warning for every hook group
// that (a) contains a trackfw guard command AND (b) has a matcher that does not
// cover PowerShell. Returns nil when the file looks correct (all guard hooks use
// "Bash|PowerShell" or similar), or when no guard hooks are found.
//
// The check is intentionally narrow: it only looks at top-level
// root["hooks"]["PreToolUse"] and root["hooks"]["PostToolUse"] arrays (the only
// paths InjectClaudeHooks writes), and only for objects with a string "matcher"
// field and a "hooks" array — the canonical Claude Code hook-group shape.
func validateClaudeGuardHookMatcherWarningsInFile(parsed interface{}, displayPath string) []string {
	root, ok := parsed.(map[string]interface{})
	if !ok {
		return nil
	}
	hooksRoot, ok := root["hooks"].(map[string]interface{})
	if !ok {
		return nil
	}

	var warnings []string
	for _, phase := range []string{"PreToolUse", "PostToolUse"} {
		arr, _ := hooksRoot[phase].([]interface{})
		for _, item := range arr {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			matcher, _ := obj["matcher"].(string)
			if matcher == "" {
				continue
			}
			// Check if this group contains any Claude guard command.
			innerHooks, _ := obj["hooks"].([]interface{})
			hasGuard := false
			for _, h := range innerHooks {
				hObj, hOk := h.(map[string]interface{})
				if !hOk {
					continue
				}
				cmd, _ := hObj["command"].(string)
				for _, marker := range claudeGuardPowerShellSubcmdMarkers {
					if strings.Contains(cmd, marker) {
						hasGuard = true
						break
					}
				}
				if hasGuard {
					break
				}
			}
			if !hasGuard {
				continue
			}
			// Guard hook found — only shell-tool groups need PowerShell coverage.
			// Claude Code uses "Bash" (case-sensitive) as the discriminant for shell
			// matchers; groups like "Read" or "Write|Edit" cover non-shell tools and
			// are intentionally not "Bash|PowerShell". Flagging them would produce
			// unfixable false-positive warnings on every project initialised with
			// `trackfw init`, because `trackfw update` does not change those groups
			// (issue #530). Only warn when the matcher contains "Bash" and is missing
			// "PowerShell".
			if strings.Contains(matcher, "Bash") && !strings.Contains(matcher, "PowerShell") {
				warnings = append(warnings, fmt.Sprintf(
					"%s: Claude Code guard hook has matcher %q — "+
						"PowerShell is not covered; the hook will not fire on Windows "+
						"(run `trackfw update` to migrate to \"Bash|PowerShell\")",
					displayPath, matcher,
				))
				break // one warning per phase per file is enough
			}
		}
	}
	return warnings
}

// validateClaudeGuardHookMatcherWarnings checks the project-scope .claude/settings.json
// for Claude Code guard hooks whose matcher does not cover PowerShell (ML-5A /
// REQ-2026-09-05). Returns always-warnings (never violations) via applyRuleWarnOnly.
func validateClaudeGuardHookMatcherWarnings() ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(root, ".claude", "settings.json")
	content, readErr := readRegularFile(path)
	if readErr != nil {
		return nil, nil // absent / unreadable — not this rule's concern
	}
	var parsed interface{}
	if json.Unmarshal(content, &parsed) != nil {
		return nil, nil // invalid JSON — handled by the resolvable rule
	}
	return validateClaudeGuardHookMatcherWarningsInFile(parsed, ".claude/settings.json"), nil
}

// validateClaudeGuardHookMatcherGlobalWarnings checks the global ~/.claude/settings.json
// for Claude Code guard hooks whose matcher does not cover PowerShell (ML-5A /
// REQ-2026-09-05). Returns always-warnings (never violations) via applyRuleWarnOnly.
func validateClaudeGuardHookMatcherGlobalWarnings() ([]string, error) {
	// ML-1A (REQ-2026-09-01): use the shared Claude config dir resolver so writer and auditor
	// always agree — ADR-2026-10-14 D3. Replaces bare os.UserHomeDir() + ".claude" that was
	// already closer to correct than other callers (USERPROFILE on Windows) but did not honour
	// CLAUDE_CONFIG_DIR (A1).
	claudeDir, err := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
	if err != nil {
		return nil, nil
	}
	path := filepath.Join(claudeDir, "settings.json")
	content, readErr := readRegularFile(path)
	if readErr != nil {
		return nil, nil // absent / unreadable — not this rule's concern
	}
	var parsed interface{}
	if json.Unmarshal(content, &parsed) != nil {
		return nil, nil // invalid JSON — handled by the resolvable rule
	}
	return validateClaudeGuardHookMatcherWarningsInFile(parsed, "~/.claude/settings.json"), nil
}

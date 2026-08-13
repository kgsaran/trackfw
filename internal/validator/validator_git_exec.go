package validator

import (
	"os"
	"os/exec"
	"strings"
)

// ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1B.
// ADR: docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-
// estrita-entre-head-e-disco.md (Emenda 3).
//
// Achado do ML-3B, reproduzido por Zeus: headTrackfwYAML() e isGitWorktree() invocavam
// exec.Command("git", ...) sem limpar o ambiente herdado do processo — GIT_DIR/GIT_WORK_TREE
// redirecionados para outro repositório git (sem trackfw.yaml versionado) faziam a resolução do
// HEAD falhar EM SILÊNCIO, e credentialGuardRuleSeverity caía só no disco: derrota o mecanismo M4
// inteiro sem nenhum commit e sem sequer editar trackfw.yaml. Exposição NOVA para
// credential_guard_script_integrity e credential_guard_hook_resolvable — elas não dependiam de git
// antes do M4.
//
// TODA invocação de git deste pacote (internal/validator) passa a ir por gitCommand() abaixo —
// nunca chamar exec.Command("git", ...) diretamente neste pacote fora deste arquivo.

// gitEnvPrefix é o prefixo de TODA variável de ambiente que o git(1) reconhece como configuração
// (GIT_DIR, GIT_WORK_TREE, GIT_CONFIG_*, GIT_CEILING_DIRECTORIES, etc.).
//
// A tentativa inicial deste ML era uma lista fechada (GIT_DIR, GIT_WORK_TREE, GIT_COMMON_DIR,
// GIT_INDEX_FILE, GIT_OBJECT_DIRECTORY, GIT_ALTERNATE_OBJECT_DIRECTORIES, GIT_CEILING_DIRECTORIES,
// GIT_NAMESPACE) justificada por "variáveis que redirecionam ONDE o repositório é lido". Essa
// justificativa estava ERRADA: o vetor real não é redirecionamento, é fazer o `git` sair com
// status != 0 por QUALQUER motivo — toda chamada deste pacote trata falha do subprocesso como "sem
// âncora, silêncio" (headTrackfwYAML) ou "fallback para disco" (credentialGuardRuleSeverity), então
// qualquer variável capaz de tornar o git fatal já é um bypass, redirecionando ou não. Provado com
// GIT_CONFIG_COUNT=abc (fora da lista fechada — injeta config arbitrária via
// GIT_CONFIG_COUNT/GIT_CONFIG_KEY_n/GIT_CONFIG_VALUE_n, e um valor malformado faz
// `git rev-parse --is-inside-work-tree` sair com "fatal: unable to parse command-line config",
// exit 128): a lista fechada não a cobria, e `credential_guard_mode_downgrade` silenciava por
// inteiro (headTrackfwYAML retornava ok=false), não só a severidade das outras duas regras.
//
// Por isso a abordagem correta é NEGATIVA por prefixo, não uma enumeração positiva: nenhuma
// invocação deste pacote (`rev-parse`, `show`, `log`, `symbolic-ref`) depende de qualquer GIT_*
// herdada do ambiente para funcionar corretamente — `-C dir` já ancora explicitamente o
// repositório, então git redescobre a partir de dir como se tivesse sido iniciado lá. Contextos
// legítimos que setam GIT_* (hooks do próprio git, `git submodule foreach`, worktrees vinculadas
// via GIT_DIR/GIT_COMMON_DIR internos) continuam funcionando sem essas variáveis porque a
// descoberta normal a partir de `-C dir` já resolve o mesmo repositório.
const gitEnvPrefix = "GIT_"

// cleanGitEnv retorna os environ() do processo atual sem nenhuma variável cujo nome comece com
// gitEnvPrefix — usado como cmd.Env de toda invocação de git deste pacote, para que nenhuma
// variável GIT_* seja herdada pelo subprocesso, capaz de redirecionar a resolução do repositório
// ou de tornar o comando fatal por qualquer outro motivo.
func cleanGitEnv() []string {
	env := os.Environ()
	cleaned := make([]string, 0, len(env))
	for _, kv := range env {
		key, _, _ := strings.Cut(kv, "=")
		if !strings.HasPrefix(key, gitEnvPrefix) {
			cleaned = append(cleaned, kv)
		}
	}
	return cleaned
}

// gitCommand constrói um *exec.Cmd para git ancorado explicitamente em dir via `-C` — nunca
// dependendo só do cwd do processo ou da descoberta implícita de repositório — e com toda
// variável GIT_* removida do ambiente herdado (ver cleanGitEnv). dir vazio cai para "." (cwd do
// processo), o mesmo comportamento que todo call site deste pacote já assumia implicitamente
// antes deste ML.
func gitCommand(dir string, args ...string) *exec.Cmd {
	if dir == "" {
		dir = "."
	}
	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Env = cleanGitEnv()
	return cmd
}

package commands

// netexec.go — executor de subprocesso git com prazo de rede (ADR D2, D4, D7/A1, A9).
//
// Design:
//
//  1. classifyGitArgs encontra o primeiro subcomando positional, pulando flags globais
//     de dois tokens (-C, -c, --git-dir etc.). Retorna o nome do subcomando ou "" se
//     nenhum for encontrado.
//
//  2. defaultGitExec é o único ponto de execução de `git` na package commands.
//     Ele classifica os args, atribui o contexto e o WaitDelay adequados, e delega
//     a configuração do grupo de processos para as funções de build-tag
//     (applyGroupDeadline em netexec_unix.go / netexec_windows.go).
//
//  3. Os executores de `gh` (defaultGhExec, defaultCheckPROpen, defaultExecForgeAPI)
//     recebem sempre o prazo de metadados — `gh` só tem operações de rede.
//     defaultExecForgeCLI (interativo, stdin/stdout herdados) nunca recebe prazo (ADR D3/I).
//
// Nota sobre exec.ErrWaitDelay (ADR D1):
// Quando o processo filho sai normalmente mas um neto mantém o pipe de stderr aberto além
// de cmd.WaitDelay, o Go retorna exec.ErrWaitDelay — nunca um *exec.ExitError. Isso ocorre
// com ctx.Err() == nil (o prazo do contexto NÃO estourou; se tivesse, o caminho de timeout
// seria ativado antes). A condição correta é:
//   errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success()
// Nesse caso, o processo completou o trabalho; o neto órfão apenas segurou o pipe. Rejeitar
// um push bem-sucedido cujo hook deixou um daemon segurando o pipe seria mais danoso que
// ignorar o pipe pendente. A função resolveNetworkErr encapsula essa classificação.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/netdeadline"
)

// cmdWaitDelay is the duration cmd.Wait allows for I/O pipes to drain after the child
// exits. Matches the precedent in internal/validator/validator_guard_binary_probe.go.
// Overridable in tests that exercise WaitDelay-specific behaviour.
var cmdWaitDelay = 5 * time.Second

// resolveNetworkErr classifies the error from a network cmd.Run/Output call (ADR D1).
//
// Returns (isTimeout=true, _) when ctx.Err() != nil — the context deadline expired.
// Returns (_, isWaitDelaySuccess=true) when the process exited 0 but a grandchild held
// the pipe past cmd.WaitDelay: err is exec.ErrWaitDelay (not *exec.ExitError) and
// cmd.ProcessState.Success() is true.
// Returns (false, false) for any other error.
//
// Both defaultGitExec and runNetworkCmd delegate to this function so the classification
// logic is tested once and applied consistently.
func resolveNetworkErr(err error, ctx context.Context, cmd *exec.Cmd) (isTimeout, isWaitDelaySuccess bool) {
	if ctx.Err() != nil {
		return true, false
	}
	if errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState != nil && cmd.ProcessState.Success() {
		return false, true
	}
	return false, false
}

// twoTokenGlobalFlags is the set of git global flags that consume the next argument
// as a value; the value token is skipped to reach the subcommand.
var twoTokenGlobalFlags = map[string]bool{
	"-C": true, "-c": true,
	"--git-dir": true, "--work-tree": true,
	"--namespace": true, "--super-prefix": true,
}

// networkSubcmds maps git subcommands that open network connections to their deadline
// class. Values: "metadata" (read-only, small payload) or "transfer" (data push).
// Subcommands not listed here are classified as local — no deadline (RS-7).
var networkSubcmds = map[string]string{
	"fetch":     "metadata",
	"ls-remote": "metadata",
	"push":      "transfer",
}

// classifyGitArgs returns the first positional subcommand in the git argument list.
// Two-token global flags (-C <dir>, -c key=val, --git-dir=<path>, etc.) and their
// values are skipped before the subcommand search begins.
// Returns "" when no subcommand is found (only flags) or the list is empty.
func classifyGitArgs(args []string) string {
	skip := false
	for _, a := range args {
		if skip {
			skip = false
			continue
		}
		if twoTokenGlobalFlags[a] {
			skip = true
			continue
		}
		// Single-token equals-form: --git-dir=x, --work-tree=x
		if strings.HasPrefix(a, "--") && strings.Contains(a, "=") {
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		// First non-flag token is the subcommand.
		return a
	}
	return ""
}

// extractGlobalFlags returns only the global flag tokens (and their values) from args,
// stopping at the first positional token (the subcommand). Used to pass -C <dir> to
// auxiliary git invocations (remote get-url, config --get) so they target the right repo.
func extractGlobalFlags(args []string) []string {
	var flags []string
	skip := false
	for _, a := range args {
		if skip {
			flags = append(flags, a)
			skip = false
			continue
		}
		if twoTokenGlobalFlags[a] {
			flags = append(flags, a)
			skip = true
			continue
		}
		if strings.HasPrefix(a, "--") && strings.Contains(a, "=") {
			flags = append(flags, a)
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		break // subcommand reached
	}
	return flags
}

// defaultGitExec runs "git <args...>" and returns trimmed stdout.
//
// Deadline classes (ADR D2, D7/A9):
//   - local ops (commit, log, rev-parse, …): exec.Command, no deadline.
//   - metadata ops (fetch, ls-remote): CommandContext with TRACKFW_NETWORK_TIMEOUT.
//   - transfer ops (push): CommandContext with TRACKFW_PUSH_TIMEOUT; HTTPS push
//     additionally receives GIT_HTTP_LOW_SPEED_LIMIT/TIME when the user has not
//     configured http.lowSpeedLimit (ADR D7/A1).
//
// On failure, surfaces git's actual stderr text — same behaviour as the previous
// implementation, keeping all parity gates intact.
//
// This function replaces the exec.Command-based defaultGitExec that was in ship.go.
// It is the only exec entry-point for git in this package.
func defaultGitExec(args ...string) (string, error) {
	subcmd := classifyGitArgs(args)
	class := networkSubcmds[subcmd]

	if class == "" {
		// Local operation: no deadline.
		cmd := exec.Command("git", args...) //nolint:gosec
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return strings.TrimSpace(string(out)), buildGitError(args, stderr, err)
		}
		return strings.TrimSpace(string(out)), nil
	}

	// Network operation: apply deadline.
	var timeout time.Duration
	var envVar string
	if class == "transfer" {
		timeout = netdeadline.PushTimeout()
		envVar = netdeadline.PushTimeoutEnv
	} else {
		timeout = netdeadline.NetworkTimeout()
		envVar = netdeadline.NetworkTimeoutEnv
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec
	cmd.WaitDelay = cmdWaitDelay
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// HTTPS push idle-detection: inject low-speed limits only when the user has not
	// already configured http.lowSpeedLimit, to avoid overriding user SSH/HTTP config.
	if subcmd == "push" {
		applyHTTPSIdleDetection(cmd, args)
	}

	applyGroupDeadline(cmd)

	out, err := cmd.Output()
	if err != nil {
		// resolveNetworkErr checks context expiry first, then the WaitDelay-success case.
		// Do NOT inspect stderr text on deadline expiry: git's output may be partial.
		isTimeout, isSuccess := resolveNetworkErr(err, ctx, cmd)
		if isTimeout {
			return "", &netdeadline.TimeoutError{
				Op:    "git " + subcmd,
				Limit: timeout,
				Var:   envVar,
			}
		}
		if isSuccess {
			// Child exited 0 but grandchild held the stderr pipe past WaitDelay.
			// Treat as success: the work completed (ADR D1).
			return strings.TrimSpace(string(out)), nil
		}
		return strings.TrimSpace(string(out)), buildGitError(args, stderr, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// applyHTTPSIdleDetection injects GIT_HTTP_LOW_SPEED_LIMIT and GIT_HTTP_LOW_SPEED_TIME
// into cmd.Env for HTTPS push when the user has not configured http.lowSpeedLimit.
// This limits idle time without overriding the user's SSH config (ADR D7/A1, RS-1).
func applyHTTPSIdleDetection(cmd *exec.Cmd, args []string) {
	globalFlags := extractGlobalFlags(args)

	// Determine origin URL using the same repo context (-C <dir>, --git-dir=x, etc.)
	urlArgs := append(append([]string{}, globalFlags...), "remote", "get-url", "origin")
	urlOut, err := exec.Command("git", urlArgs...).Output() //nolint:gosec
	if err != nil {
		return // cannot determine URL; skip idle-detection
	}
	remoteURL := strings.TrimSpace(string(urlOut))
	if !strings.HasPrefix(remoteURL, "https://") {
		return // not HTTPS; skip (SSH has no safe idle injection, RS-1)
	}

	// Only inject when the user has not already configured http.lowSpeedLimit.
	cfgArgs := append(append([]string{}, globalFlags...), "config", "--get", "http.lowSpeedLimit")
	cfgOut, cfgErr := exec.Command("git", cfgArgs...).Output() //nolint:gosec
	if cfgErr == nil && strings.TrimSpace(string(cfgOut)) != "" {
		return // user has a custom limit; honour it
	}

	cmd.Env = append(os.Environ(),
		"GIT_HTTP_LOW_SPEED_LIMIT=1",
		"GIT_HTTP_LOW_SPEED_TIME=30",
	)
}

// buildGitError constructs an error from git's stderr, matching the existing behaviour
// (used in parity gates).
func buildGitError(args []string, stderr bytes.Buffer, err error) error {
	msg := strings.TrimSpace(stderr.String())
	if msg == "" {
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg = fmt.Sprintf("git %s exited with %d", strings.Join(args, " "), exitErr.ExitCode())
		} else {
			msg = err.Error()
		}
	}
	return errors.New(msg)
}

// runNetworkCmd is the shared helper for all non-git network executors
// (defaultGhExec, defaultCheckPROpen, defaultExecForgeAPI). All gh operations
// are metadata-class; the interactive executor (defaultExecForgeCLI) never calls
// this (ADR D3/I).
func runNetworkCmd(name string, args []string, stdinData string, captureStdout bool) ([]byte, error) {
	timeout := netdeadline.NetworkTimeout()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec
	cmd.WaitDelay = cmdWaitDelay
	if stdinData != "" {
		cmd.Stdin = strings.NewReader(stdinData)
	}
	var stdout, stderr bytes.Buffer
	if captureStdout {
		cmd.Stdout = &stdout
	}
	cmd.Stderr = &stderr

	applyGroupDeadline(cmd)

	err := cmd.Run()
	if err != nil {
		// resolveNetworkErr checks context expiry first, then the WaitDelay-success case.
		isTimeout, isSuccess := resolveNetworkErr(err, ctx, cmd)
		if isTimeout {
			return nil, &netdeadline.TimeoutError{
				Op:    name,
				Limit: timeout,
				Var:   netdeadline.NetworkTimeoutEnv,
			}
		}
		if isSuccess {
			// Child exited 0 but grandchild held the stderr pipe past WaitDelay (ADR D1).
			if captureStdout {
				return stdout.Bytes(), nil
			}
			return nil, nil
		}
		// Surface real stderr text (matching parity gate behaviour).
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			if exitErr, ok := err.(*exec.ExitError); ok {
				msg = fmt.Sprintf("%s %s exited with %d", name, strings.Join(args, " "), exitErr.ExitCode())
			} else {
				msg = err.Error()
			}
		}
		return nil, errors.New(msg)
	}
	if captureStdout {
		return stdout.Bytes(), nil
	}
	return nil, nil
}

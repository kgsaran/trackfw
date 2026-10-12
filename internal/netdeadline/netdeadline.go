// Package netdeadline provides timeout configuration for network operations
// performed by the trackfw CLI.
//
// Two environment variables govern deadlines (ADR D4, D7/A1, A3):
//
//   - TRACKFW_NETWORK_TIMEOUT (default 60 s): teto total para operações de leitura
//     de metadados (git fetch, git ls-remote, gh pr list, gh api de leitura).
//   - TRACKFW_PUSH_TIMEOUT (default 30 min): teto total para transferências de dados
//     (git push). Precisa de um teto próprio porque transferências legítimas podem
//     durar minutos, enquanto leituras de metadados têm payload limitado (ADR A1).
//
// Valores inválidos (0, negativos, acima de 24 h, texto não-parsável) emitem aviso
// em stderr e usam o padrão — o prazo nunca é desligado (ADR A3).
//
// ML-1C (internal/sync/linear.go) importa este pacote; ele não deve depender de
// internal/commands para evitar ciclo de importação.
package netdeadline

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

const (
	// NetworkTimeoutEnv is the environment variable for metadata operation deadlines.
	NetworkTimeoutEnv = "TRACKFW_NETWORK_TIMEOUT"

	// PushTimeoutEnv is the environment variable for data-transfer operation deadlines.
	PushTimeoutEnv = "TRACKFW_PUSH_TIMEOUT"

	// DefaultNetworkTimeout is the deadline applied to metadata operations when
	// TRACKFW_NETWORK_TIMEOUT is unset or invalid. 60 s provides ~20× margin over
	// the measured worst case (2.96 s for gh pr list with 463 PRs).
	DefaultNetworkTimeout = 60 * time.Second

	// DefaultPushTimeout is the deadline applied to git push when TRACKFW_PUSH_TIMEOUT
	// is unset or invalid.
	DefaultPushTimeout = 30 * time.Minute

	// maxTimeout is the upper bound accepted by the parser; above this, the value is
	// treated as absurd and the default is used (ADR A3).
	maxTimeout = 24 * time.Hour
)

// ErrTimeout is the sentinel returned by network executors when the deadline expires.
// Use errors.Is(err, ErrTimeout) to detect it.
var ErrTimeout = errors.New("network timeout")

// TimeoutError is the concrete error type carrying the operation, limit and env var.
// It implements errors.Is against ErrTimeout.
//
// Example message: "git fetch origin timed out after 1m0s (TRACKFW_NETWORK_TIMEOUT)"
type TimeoutError struct {
	// Op is the human-readable operation name, e.g. "git fetch".
	Op string
	// Limit is the deadline that expired.
	Limit time.Duration
	// Var is the controlling environment variable name.
	Var string
}

// Error returns the formatted timeout message (ADR D5).
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("%s timed out after %s (%s)", e.Op, e.Limit, e.Var)
}

// Is makes errors.Is(err, ErrTimeout) return true.
func (e *TimeoutError) Is(target error) bool {
	return target == ErrTimeout
}

// warnWriter is used for warning messages; overridable in tests without
// touching global os.Stderr.
var warnWriter io.Writer = os.Stderr

// Parse validates a raw duration string against the constraints of the ADR.
// Returns (parsed duration, "") on success.
// Returns (def, warning message) when the value is absent, invalid, zero,
// negative, or above maxTimeout.
// An empty raw string returns (def, "") with no warning — absent means "use default".
func Parse(raw string, def time.Duration, envVar string) (time.Duration, string) {
	if raw == "" {
		return def, ""
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return def, fmt.Sprintf("trackfw: %s=%q is invalid (%v); using default %s", envVar, raw, err, def)
	}
	if d <= 0 {
		return def, fmt.Sprintf("trackfw: %s=%q must be positive; using default %s", envVar, raw, def)
	}
	if d > maxTimeout {
		return def, fmt.Sprintf("trackfw: %s=%q exceeds maximum 24h; using default %s", envVar, raw, def)
	}
	return d, ""
}

// NetworkTimeout reads TRACKFW_NETWORK_TIMEOUT and returns the validated duration.
// Any warning is emitted to stderr. The deadline is never disabled.
func NetworkTimeout() time.Duration {
	return readEnv(NetworkTimeoutEnv, DefaultNetworkTimeout)
}

// PushTimeout reads TRACKFW_PUSH_TIMEOUT and returns the validated duration.
// Any warning is emitted to stderr. The deadline is never disabled.
func PushTimeout() time.Duration {
	return readEnv(PushTimeoutEnv, DefaultPushTimeout)
}

func readEnv(envVar string, def time.Duration) time.Duration {
	d, warn := Parse(os.Getenv(envVar), def, envVar)
	if warn != "" {
		fmt.Fprintln(warnWriter, warn)
	}
	return d
}

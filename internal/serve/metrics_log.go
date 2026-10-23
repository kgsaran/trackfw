package serve

import (
	"github.com/kgsaran/trackfw/internal/translog"
)

// Transition is an alias for translog.Transition — the two types are identical
// and share the same package representation. Using an alias removes the per-call
// conversion loop that previously existed between ParseLog and its callers, while
// keeping the serve.Transition name available for any future serve-only fields.
type Transition = translog.Transition

// ParseLog reads the roadmap transition log directory and returns a slice of
// Transitions, combining the legacy .trackfw-log file (if present) with entries
// in .trackfw-log-entries/. Returns a non-nil empty slice and a non-nil error
// when ReadAll fails (R5: the caller can surface the error in the API response).
func ParseLog(dir string) ([]Transition, error) {
	tls, err := translog.ReadAll(dir)
	if err != nil || len(tls) == 0 {
		return []Transition{}, err
	}
	return tls, nil
}

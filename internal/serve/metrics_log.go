package serve

import (
	"time"

	"github.com/kgsaran/trackfw/internal/translog"
)

// Transition represents a single state-change entry in the transition log.
type Transition struct {
	Timestamp time.Time
	Basename  string
	From      string
	To        string
}

// ParseLog reads the roadmap transition log directory and returns a slice of
// Transitions, combining the legacy .trackfw-log file (if present) with entries
// in .trackfw-log.d/. Returns an empty slice on any error or when no log exists.
func ParseLog(dir string) []Transition {
	tls, err := translog.ReadAll(dir)
	if err != nil || len(tls) == 0 {
		return []Transition{}
	}
	out := make([]Transition, len(tls))
	for i, t := range tls {
		out[i] = Transition{
			Timestamp: t.Timestamp,
			Basename:  t.Basename,
			From:      t.From,
			To:        t.To,
		}
	}
	return out
}

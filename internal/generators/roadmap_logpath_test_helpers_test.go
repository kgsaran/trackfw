package generators

import "github.com/kgsaran/trackfw/internal/config"

// logPath is used only by tests that need the legacy .trackfw-log path for test
// fixture setup (fail_closed_root_test.go and mute_guards_speak_test.go).
//
// It is defined here (in a _test.go file) rather than in the production roadmap.go
// because no production code calls it — the transition-log writer now only uses
// translog.WriteEntry to create per-entry files in .trackfw-log-entries/.
//
// M1: removed from roadmap.go where it had been dead code since ML-1A.
func logPath() string {
	return config.Load().RoadmapDir + "/.trackfw-log"
}

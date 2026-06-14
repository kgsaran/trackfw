package serve

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"time"
)

// Transition represents a single state-change entry in the .trackfw-log file.
type Transition struct {
	Timestamp time.Time
	Basename  string
	From      string
	To        string
}

// logLineRe matches lines in the format:
// "2026-06-12 14:30  ROADMAP-....md                  backlog → wip"
var logLineRe = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2})\s{2,}(\S+)\s{2,}(\S+)\s+→\s+(\S+)`,
)

// ParseLog reads the .trackfw-log file and returns a slice of Transitions.
// Returns an empty slice if the file does not exist or cannot be read.
func ParseLog(path string) []Transition {
	f, err := os.Open(path)
	if err != nil {
		return []Transition{}
	}
	defer f.Close()

	var out []Transition
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		m := logLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ts, err := time.Parse("2006-01-02 15:04", m[1])
		if err != nil {
			continue
		}
		out = append(out, Transition{
			Timestamp: ts,
			Basename:  m[2],
			From:      strings.TrimSpace(m[3]),
			To:        strings.TrimSpace(m[4]),
		})
	}
	return out
}

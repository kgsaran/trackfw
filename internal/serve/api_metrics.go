package serve

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// burndownPoint holds open/closed counts for one calendar week.
type burndownPoint struct {
	Date   string `json:"date"`
	Open   int    `json:"open"`
	Closed int    `json:"closed"`
}

// metricsResponse is the JSON shape returned by GET /api/metrics.
type metricsResponse struct {
	LeadTimeAvgDays  float64            `json:"lead_time_avg_days"`
	CycleTimeAvgDays float64            `json:"cycle_time_avg_days"`
	AbandonmentRate  float64            `json:"abandonment_rate"`
	StateDistrib     map[string]int     `json:"state_distribution"`
	Burndown         []burndownPoint    `json:"burndown"`
}

var openStates = map[string]bool{
	"wip": true, "backlog": true, "blocked": true,
}

// metricsHandler handles GET /api/metrics.
func metricsHandler(w http.ResponseWriter, _ *http.Request, cfg config.ProjectConfig) {
	setCORSHeaders(w)
	w.Header().Set("Content-Type", "application/json")

	logFile := filepath.Join(cfg.RoadmapDir, ".trackfw-log")
	transitions := ParseLog(logFile)

	leadTime := calcLeadTime(transitions)
	cycleTime := calcCycleTime(transitions)
	abandonment := calcAbandonmentRate(transitions)
	burndown := calcBurndown(transitions)

	// State distribution: count files currently in each state dir
	distrib := countStateDistribution(cfg)

	resp := metricsResponse{
		LeadTimeAvgDays:  leadTime,
		CycleTimeAvgDays: cycleTime,
		AbandonmentRate:  abandonment,
		StateDistrib:     distrib,
		Burndown:         burndown,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// calcLeadTime returns the average days between first appearance (backlog/wip) and done.
func calcLeadTime(transitions []Transition) float64 {
	// firstSeen[basename] = earliest timestamp in backlog or wip
	firstSeen := map[string]time.Time{}
	doneSeen := map[string]time.Time{}

	for _, t := range transitions {
		if t.To == "backlog" || t.To == "wip" {
			if existing, ok := firstSeen[t.Basename]; !ok || t.Timestamp.Before(existing) {
				firstSeen[t.Basename] = t.Timestamp
			}
		}
		if t.To == "done" {
			if existing, ok := doneSeen[t.Basename]; !ok || t.Timestamp.After(existing) {
				doneSeen[t.Basename] = t.Timestamp
			}
		}
	}

	var total float64
	count := 0
	for name, doneAt := range doneSeen {
		if start, ok := firstSeen[name]; ok {
			total += doneAt.Sub(start).Hours() / 24
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return round2(total / float64(count))
}

// calcCycleTime returns the average days between first entry in wip and done.
func calcCycleTime(transitions []Transition) float64 {
	firstWip := map[string]time.Time{}
	doneSeen := map[string]time.Time{}

	for _, t := range transitions {
		if t.To == "wip" {
			if existing, ok := firstWip[t.Basename]; !ok || t.Timestamp.Before(existing) {
				firstWip[t.Basename] = t.Timestamp
			}
		}
		if t.To == "done" {
			if existing, ok := doneSeen[t.Basename]; !ok || t.Timestamp.After(existing) {
				doneSeen[t.Basename] = t.Timestamp
			}
		}
	}

	var total float64
	count := 0
	for name, doneAt := range doneSeen {
		if start, ok := firstWip[name]; ok {
			total += doneAt.Sub(start).Hours() / 24
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return round2(total / float64(count))
}

// calcAbandonmentRate returns abandoned/(total unique roadmaps in log).
func calcAbandonmentRate(transitions []Transition) float64 {
	all := map[string]bool{}
	abandoned := map[string]bool{}
	for _, t := range transitions {
		all[t.Basename] = true
		if t.To == "abandoned" {
			abandoned[t.Basename] = true
		}
	}
	if len(all) == 0 {
		return 0
	}
	return round2(float64(len(abandoned)) / float64(len(all)))
}

// calcBurndown computes weekly open/closed counts from the transition log.
func calcBurndown(transitions []Transition) []burndownPoint {
	if len(transitions) == 0 {
		return []burndownPoint{}
	}

	// Find date range
	minT := transitions[0].Timestamp
	maxT := transitions[0].Timestamp
	for _, t := range transitions {
		if t.Timestamp.Before(minT) {
			minT = t.Timestamp
		}
		if t.Timestamp.After(maxT) {
			maxT = t.Timestamp
		}
	}

	// Align to Monday of the first week
	weekStart := startOfWeek(minT)
	weekEnd := startOfWeek(maxT).AddDate(0, 0, 7) // inclusive of last week

	// For each roadmap, track its state at each week boundary
	// stateAt[basename] = current state (default: unknown)
	type stateEvent struct {
		at    time.Time
		name  string
		state string
	}
	var events []stateEvent
	for _, t := range transitions {
		events = append(events, stateEvent{at: t.Timestamp, name: t.Basename, state: t.To})
	}

	var points []burndownPoint
	for w := weekStart; w.Before(weekEnd); w = w.AddDate(0, 0, 7) {
		// Compute each roadmap's state at end-of-week
		stateMap := map[string]string{}
		boundary := w.AddDate(0, 0, 7)
		for _, e := range events {
			if !e.at.After(boundary) {
				stateMap[e.name] = e.state
			}
		}
		open, closed := 0, 0
		for _, s := range stateMap {
			if openStates[s] {
				open++
			} else {
				closed++
			}
		}
		points = append(points, burndownPoint{
			Date:   w.Format("2006-01-02"),
			Open:   open,
			Closed: closed,
		})
	}
	return points
}

// countStateDistribution scans the filesystem for current roadmap counts per state.
func countStateDistribution(cfg config.ProjectConfig) map[string]int {
	distrib := map[string]int{
		"wip": 0, "backlog": 0, "blocked": 0, "done": 0, "abandoned": 0,
	}

	if cfg.RoadmapNamespacing == config.NamespacingByAgent {
		agents, _ := os.ReadDir(cfg.RoadmapDir)
		for _, a := range agents {
			if !a.IsDir() {
				continue
			}
			for state := range distrib {
				dir := filepath.Join(cfg.RoadmapDir, a.Name(), state)
				distrib[state] += countMDFiles(dir)
			}
		}
	} else {
		for state := range distrib {
			dir := filepath.Join(cfg.RoadmapDir, state)
			distrib[state] += countMDFiles(dir)
		}
	}
	return distrib
}

func countMDFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			n++
		}
	}
	return n
}

// startOfWeek returns the Monday of the week containing t (UTC).
func startOfWeek(t time.Time) time.Time {
	t = t.UTC().Truncate(24 * time.Hour)
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7 // Sunday = 7
	}
	return t.AddDate(0, 0, -(wd - 1))
}

// round2 rounds a float to 2 decimal places.
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

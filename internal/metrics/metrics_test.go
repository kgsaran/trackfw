package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLog_Empty(t *testing.T) {
	transitions, err := ParseLog("/tmp/trackfw-nonexistent-log-99999.log")
	require.NoError(t, err)
	assert.Nil(t, transitions)
}

func TestParseLog_WithLines(t *testing.T) {
	content := `2026-06-12 14:30  ROADMAP-2026-06-12-auth.md                  backlog → wip
2026-06-12 16:00  ROADMAP-2026-06-12-auth.md                  wip → done
`
	tmp := filepath.Join(t.TempDir(), ".trackfw-log")
	require.NoError(t, os.WriteFile(tmp, []byte(content), 0644))

	transitions, err := ParseLog(tmp)
	require.NoError(t, err)
	require.Len(t, transitions, 2)

	assert.Equal(t, "ROADMAP-2026-06-12-auth.md", transitions[0].Basename)
	assert.Equal(t, "backlog", transitions[0].From)
	assert.Equal(t, "wip", transitions[0].To)

	assert.Equal(t, "ROADMAP-2026-06-12-auth.md", transitions[1].Basename)
	assert.Equal(t, "wip", transitions[1].From)
	assert.Equal(t, "done", transitions[1].To)
}

func TestParseLog_IgnoresBlankLines(t *testing.T) {
	content := "\n2026-06-12 14:30  ROADMAP-2026-06-12-auth.md                  backlog → wip\n\n"
	tmp := filepath.Join(t.TempDir(), ".trackfw-log")
	require.NoError(t, os.WriteFile(tmp, []byte(content), 0644))

	transitions, err := ParseLog(tmp)
	require.NoError(t, err)
	assert.Len(t, transitions, 1)
}

func TestCalculate_CycleTime(t *testing.T) {
	baseTime := time.Date(2026, 6, 12, 10, 0, 0, 0, time.Local)
	transitions := []Transition{
		{Timestamp: baseTime, Basename: "roadmap-a.md", From: "created", To: "backlog"},
		{Timestamp: baseTime.Add(2 * time.Hour), Basename: "roadmap-a.md", From: "backlog", To: "wip"},
		{Timestamp: baseTime.Add(10 * time.Hour), Basename: "roadmap-a.md", From: "wip", To: "done"},
	}

	m := Calculate(transitions)

	// Cycle time: da entrada em backlog (t=0h) até done (t=10h) = 10h
	assert.Equal(t, 10*time.Hour, m.CycleTimeMean)
	assert.Empty(t, m.WIPEntries)
}

func TestCalculate_Throughput(t *testing.T) {
	// 2 roadmaps concluídos em 7 dias = throughput ~2/semana (dentro da semana → weeks=1)
	baseTime := time.Date(2026, 6, 1, 10, 0, 0, 0, time.Local)
	transitions := []Transition{
		{Timestamp: baseTime, Basename: "roadmap-a.md", From: "backlog", To: "wip"},
		{Timestamp: baseTime.Add(2 * 24 * time.Hour), Basename: "roadmap-a.md", From: "wip", To: "done"},
		{Timestamp: baseTime.Add(3 * 24 * time.Hour), Basename: "roadmap-b.md", From: "backlog", To: "wip"},
		{Timestamp: baseTime.Add(5 * 24 * time.Hour), Basename: "roadmap-b.md", From: "wip", To: "done"},
	}

	m := Calculate(transitions)
	// 2 done em período < 7 dias → weeks=1 → throughput=2.0
	assert.InDelta(t, 2.0, m.Throughput, 0.01)
}

func TestCalculate_WIPEntries(t *testing.T) {
	baseTime := time.Date(2026, 6, 1, 10, 0, 0, 0, time.Local)
	transitions := []Transition{
		{Timestamp: baseTime, Basename: "roadmap-wip.md", From: "backlog", To: "wip"},
		// sem done → deve aparecer em WIPEntries
		{Timestamp: baseTime, Basename: "roadmap-done.md", From: "backlog", To: "wip"},
		{Timestamp: baseTime.Add(1 * time.Hour), Basename: "roadmap-done.md", From: "wip", To: "done"},
	}

	m := Calculate(transitions)
	require.Len(t, m.WIPEntries, 1)
	assert.Equal(t, "roadmap-wip.md", m.WIPEntries[0].Basename)
}

func TestFilter_Since(t *testing.T) {
	now := time.Now()
	transitions := []Transition{
		{Timestamp: now.Add(-48 * time.Hour), Basename: "old.md", From: "backlog", To: "wip"},
		{Timestamp: now.Add(-1 * time.Hour), Basename: "recent.md", From: "backlog", To: "wip"},
	}

	since := now.Add(-24 * time.Hour)
	filtered := Filter(transitions, since)
	require.Len(t, filtered, 1)
	assert.Equal(t, "recent.md", filtered[0].Basename)
}

func TestExportCSV(t *testing.T) {
	baseTime := time.Date(2026, 6, 12, 10, 0, 0, 0, time.Local)
	transitions := []Transition{
		{Timestamp: baseTime, Basename: "roadmap-a.md", From: "backlog", To: "wip"},
	}
	m := Metrics{
		CycleTimeMean: 8 * time.Hour,
		Throughput:    1.5,
		WIPEntries:    []WIPEntry{{Basename: "roadmap-a.md", Age: time.Hour}},
	}

	tmp := filepath.Join(t.TempDir(), "metrics.csv")
	require.NoError(t, ExportCSV(m, transitions, tmp))

	content, err := os.ReadFile(tmp)
	require.NoError(t, err)
	assert.Contains(t, string(content), "basename,from,to,timestamp")
	assert.Contains(t, string(content), "roadmap-a.md")
	assert.Contains(t, string(content), "cycle_time_mean_hours")
	assert.Contains(t, string(content), "throughput_per_week")
	assert.Contains(t, string(content), "wip_count")
}

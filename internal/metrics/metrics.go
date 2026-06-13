package metrics

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Transition representa uma entrada do .trackfw-log.
type Transition struct {
	Timestamp time.Time
	Basename  string
	From, To  string
}

// WIPEntry representa um roadmap atualmente em wip com sua idade.
type WIPEntry struct {
	Basename string
	Age      time.Duration
}

// Metrics contém as métricas de delivery calculadas.
type Metrics struct {
	CycleTimeMean time.Duration
	Throughput    float64 // roadmaps concluídos por semana
	WIPEntries    []WIPEntry
	Period        string
}

// lineRe faz match de linhas do formato:
// 2026-06-12 14:30  ROADMAP-2026-06-12-auth.md                  backlog → wip
var lineRe = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2} \d{2}:\d{2})\s{2,}(\S+)\s{2,}(\S+)\s+→\s+(\S+)`)

// ParseLog lê o arquivo .trackfw-log e retorna todas as transições.
// Retorna nil, nil se o arquivo não existe.
func ParseLog(path string) ([]Transition, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("metrics: abrir log: %w", err)
	}
	defer f.Close()

	var transitions []Transition
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := lineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		ts, err := time.ParseInLocation("2006-01-02 15:04", m[1], time.Local)
		if err != nil {
			continue
		}
		transitions = append(transitions, Transition{
			Timestamp: ts,
			Basename:  strings.TrimSpace(m[2]),
			From:      strings.TrimSpace(m[3]),
			To:        strings.TrimSpace(m[4]),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("metrics: ler log: %w", err)
	}
	return transitions, nil
}

// Filter retorna apenas as transições com Timestamp >= since.
func Filter(transitions []Transition, since time.Time) []Transition {
	var out []Transition
	for _, t := range transitions {
		if !t.Timestamp.Before(since) {
			out = append(out, t)
		}
	}
	return out
}

// Calculate computa cycle time médio, throughput e WIP age a partir das transições.
func Calculate(transitions []Transition) Metrics {
	// Agrupar transições por basename para calcular ciclos.
	type stateEntry struct {
		ts    time.Time
		state string
	}
	// Mapa: basename → slice de entradas em ordem cronológica
	byName := make(map[string][]stateEntry)
	for _, t := range transitions {
		byName[t.Basename] = append(byName[t.Basename], stateEntry{ts: t.Timestamp, state: t.To})
	}

	// Calcular cycle time: tempo desde entrada em backlog (ou wip) até done.
	var cycleTimes []time.Duration
	for _, entries := range byName {
		var startTs *time.Time
		var doneTs *time.Time
		for _, e := range entries {
			if (e.state == "backlog" || e.state == "wip") && startTs == nil {
				ts := e.ts
				startTs = &ts
			}
			if e.state == "done" {
				ts := e.ts
				doneTs = &ts
			}
		}
		if startTs != nil && doneTs != nil {
			cycleTimes = append(cycleTimes, doneTs.Sub(*startTs))
		}
	}

	var cycleTimeMean time.Duration
	if len(cycleTimes) > 0 {
		var total time.Duration
		for _, ct := range cycleTimes {
			total += ct
		}
		cycleTimeMean = total / time.Duration(len(cycleTimes))
	}

	// Calcular throughput: roadmaps concluídos por semana.
	var doneCount int
	var minTs, maxTs time.Time
	for _, t := range transitions {
		if t.To == "done" {
			doneCount++
		}
		if minTs.IsZero() || t.Timestamp.Before(minTs) {
			minTs = t.Timestamp
		}
		if maxTs.IsZero() || t.Timestamp.After(maxTs) {
			maxTs = t.Timestamp
		}
	}
	var throughput float64
	if doneCount > 0 {
		weeks := maxTs.Sub(minTs).Hours() / (7 * 24)
		if weeks < 1 {
			weeks = 1
		}
		throughput = float64(doneCount) / weeks
	}

	// Calcular WIP age: basenames que entraram em wip mas não foram concluídos ou abandonados.
	var wipEntries []WIPEntry
	for name, entries := range byName {
		var wipTs *time.Time
		concluded := false
		for _, e := range entries {
			if e.state == "wip" {
				ts := e.ts
				wipTs = &ts
			}
			if e.state == "done" || e.state == "abandoned" {
				concluded = true
			}
		}
		if wipTs != nil && !concluded {
			wipEntries = append(wipEntries, WIPEntry{
				Basename: name,
				Age:      time.Since(*wipTs),
			})
		}
	}

	return Metrics{
		CycleTimeMean: cycleTimeMean,
		Throughput:    throughput,
		WIPEntries:    wipEntries,
	}
}

// ExportCSV grava as transições e métricas em um arquivo CSV.
func ExportCSV(m Metrics, transitions []Transition, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("metrics: criar CSV: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)

	// Seção de transições.
	if err := w.Write([]string{"basename", "from", "to", "timestamp"}); err != nil {
		return fmt.Errorf("metrics: escrever header CSV: %w", err)
	}
	for _, t := range transitions {
		if err := w.Write([]string{
			t.Basename,
			t.From,
			t.To,
			t.Timestamp.Format("2006-01-02 15:04"),
		}); err != nil {
			return fmt.Errorf("metrics: escrever linha CSV: %w", err)
		}
	}

	// Linha em branco.
	if err := w.Write([]string{""}); err != nil {
		return fmt.Errorf("metrics: escrever separador CSV: %w", err)
	}

	// Seção de métricas.
	if err := w.Write([]string{"metric", "value"}); err != nil {
		return fmt.Errorf("metrics: escrever header métricas CSV: %w", err)
	}
	cycleHours := m.CycleTimeMean.Hours()
	if err := w.Write([]string{"cycle_time_mean_hours", strconv.FormatFloat(cycleHours, 'f', 2, 64)}); err != nil {
		return fmt.Errorf("metrics: escrever cycle_time: %w", err)
	}
	if err := w.Write([]string{"throughput_per_week", strconv.FormatFloat(m.Throughput, 'f', 2, 64)}); err != nil {
		return fmt.Errorf("metrics: escrever throughput: %w", err)
	}
	if err := w.Write([]string{"wip_count", strconv.Itoa(len(m.WIPEntries))}); err != nil {
		return fmt.Errorf("metrics: escrever wip_count: %w", err)
	}

	w.Flush()
	return w.Error()
}

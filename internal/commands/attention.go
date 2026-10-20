package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kgsaran/trackfw/internal/pathguard"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const attentionMaxInput = 1 << 20
const attentionReadTimeout = 2 * time.Second

// Attention is informational: bad input and unavailable files never block tools.
// Usage errors still follow Cobra's normal contract. Cleanup never reads stdin.
func newAttentionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "attention", Short: "Signal or clear board attention"}
	for _, name := range []string{"signal", "cleanup"} {
		cmd.AddCommand(&cobra.Command{Use: name, Args: cobra.NoArgs, Run: func(c *cobra.Command, _ []string) {
			cwd, err := os.Getwd()
			if err == nil {
				runAttention(c.Name(), cwd, c.InOrStdin())
			}
		}})
	}
	return cmd
}

// The deadline is absolute, including a writer that keeps supplying tiny chunks.
// A buffered result lets the reader finish safely after a timeout. In the actual
// hook process a blocked stdin reader dies with the process; no payload is logged.
func attentionInput(r io.Reader, timeout time.Duration) ([]byte, bool) {
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(r, attentionMaxInput+1))
		done <- result{data, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case got := <-done:
		return got.data, got.err == nil && len(got.data) <= attentionMaxInput
	case <-timer.C:
		return nil, false
	}
}

// Read only the cwd config, without config.Load's process-exit side effects.
// Decode the complete YAML document so malformed/duplicate/multi-document YAML
// cannot silently select a destination. Other config keys remain allowed.
func attentionPath(cwd string) (string, string, bool) {
	root, err := pathguard.ResolveRoot(cwd)
	if err != nil {
		return "", "", false
	}
	data, err := os.ReadFile(filepath.Join(root, "trackfw.yaml"))
	if err != nil {
		return "", "", false
	}
	var cfg struct {
		RoadmapDir string `yaml:"roadmap_dir"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&cfg); err != nil && err != io.EOF {
		return "", "", false
	}
	var extra interface{}
	if dec.Decode(&extra) != io.EOF {
		return "", "", false
	}
	dir := cfg.RoadmapDir
	if dir == "" {
		dir = "docs/roadmaps"
	}
	// Treat Windows separators/volumes as unsafe on every host, too: the same
	// versioned config must not escape when used on Windows later.
	if strings.ContainsAny(dir, "\\:") || filepath.IsAbs(dir) {
		return "", "", false
	}
	for _, part := range strings.Split(filepath.ToSlash(dir), "/") {
		if part == ".." {
			return "", "", false
		}
	}
	filename := filepath.Join(root, dir, ".trackfw-attention.json")
	if pathguard.RejectAndReport(root, filename) != nil {
		return "", "", false
	}
	return root, filename, true
}

func attentionText(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, value)
}

func runAttention(action, cwd string, input io.Reader) {
	root, filename, ok := attentionPath(cwd)
	if !ok {
		return
	}
	if action == "cleanup" {
		// write-containment-allowed: leaf and ancestors checked by pathguard.RejectAndReport in attentionPath
		_ = os.Remove(filename)
		return
	}
	data, ok := attentionInput(input, attentionReadTimeout)
	if !ok {
		return
	}
	var payload struct {
		Tool  *string `json:"tool_name"`
		Input struct {
			Question *string `json:"question"`
			Command  *string `json:"command"`
		} `json:"tool_input"`
	}
	if json.Unmarshal(data, &payload) != nil || !bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		return
	}
	tool, fallback := "", "unknown"
	if payload.Tool != nil {
		tool, fallback = *payload.Tool, *payload.Tool
	}
	message := "Agent is executing: " + fallback
	if payload.Input.Question != nil {
		message = *payload.Input.Question
	} else if payload.Input.Command != nil {
		message = *payload.Input.Command
	}
	// The old jq/Python scripts slice before stripping ASCII controls.
	runes := []rune(message)
	if len(runes) > 300 {
		message = string(runes[:300])
	}
	signal := struct {
		Tool      string `json:"tool"`
		Message   string `json:"message"`
		Level     string `json:"level"`
		Timestamp string `json:"timestamp"`
	}{attentionText(tool), attentionText(message), "action_required", time.Now().UTC().Format(time.RFC3339)}
	encoded, err := json.Marshal(signal)
	if err != nil {
		return
	}
	// Recheck after reading stdin, before creating directories and writing the leaf.
	if pathguard.RejectAndReport(root, filename) != nil {
		return
	}
	// write-containment-allowed: guarded by pathguard.RejectAndReport above, including leaf and ancestors
	if os.MkdirAll(filepath.Dir(filename), 0755) != nil {
		return
	}
	if pathguard.RejectAndReport(root, filename) != nil {
		return
	}
	// write-containment-allowed: guarded by pathguard.RejectAndReport immediately above
	_ = os.WriteFile(filename, append(encoded, '\n'), 0600)
}

package commands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func attentionFixture(t *testing.T, config string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "trackfw.yaml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	return dir, filepath.Join(dir, "docs", "roadmaps", ".trackfw-attention.json")
}
func readAttention(t *testing.T, filename string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err = json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	stamp, err := time.Parse(time.RFC3339, got["timestamp"])
	if err != nil || stamp.Location() != time.UTC {
		t.Fatalf("bad UTC timestamp %q", got["timestamp"])
	}
	delete(got, "timestamp")
	return got
}
func TestAttentionLegacyCorpusParity(t *testing.T) {
	manifest, err := os.ReadFile("testdata/attention-legacy/MANIFEST.sha256")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(manifest)), "\n") {
		f := strings.Fields(line)
		data, err := os.ReadFile(filepath.Join("testdata/attention-legacy", f[1]))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != f[0] {
			t.Fatal("legacy evidence changed")
		}
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal("legacy parity requires bash", err)
	}
	script, err := filepath.Abs("testdata/attention-legacy/signal.sh")
	if err != nil {
		t.Fatal(err)
	}
	cleanup, _ := filepath.Abs("testdata/attention-legacy/cleanup.sh")
	cases := []string{`{}`, `{"tool_name":""}`, `{"tool_name":"Bash","tool_input":{"command":"echo olá 😀"}}`, `{"tool_name":"AskUserQuestion","tool_input":{"question":"approve \\\"á😀\n\t\r","command":"ignored"}}`, `{"tool_name":"Bash","tool_input":{"command":"$(touch NEVER_EXECUTED); ` + "`" + `id` + "`" + ` $HOME | cat"}}`}
	long, _ := json.Marshal(map[string]interface{}{"tool_name": "Unicode", "tool_input": map[string]string{"question": strings.Repeat("😀á", 200)}})
	cases = append(cases, string(long))
	for _, payload := range cases {
		t.Run(payload[:min(len(payload), 40)], func(t *testing.T) {
			legacy, _ := attentionFixture(t, "roadmap_dir: 'mapas ação'\r\n")
			native, _ := attentionFixture(t, "roadmap_dir: 'mapas ação'\r\n")
			cmd := exec.Command(bash, script)
			cmd.Dir = legacy
			cmd.Stdin = strings.NewReader(payload)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("legacy: %v %s", err, out)
			}
			runAttention("signal", native, strings.NewReader(payload))
			lf := filepath.Join(legacy, "mapas ação", ".trackfw-attention.json")
			nf := filepath.Join(native, "mapas ação", ".trackfw-attention.json")
			if a, b := readAttention(t, lf), readAttention(t, nf); !reflect.DeepEqual(a, b) {
				t.Fatalf("legacy %#v != native %#v", a, b)
			}
			cmd = exec.Command(bash, cleanup)
			cmd.Dir = legacy
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("cleanup legacy: %v %s", err, out)
			}
			runAttention("cleanup", native, nil)
			for _, p := range []string{lf, nf} {
				if _, err := os.Stat(p); !os.IsNotExist(err) {
					t.Fatalf("cleanup left %s", p)
				}
			}
		})
	}
}
func TestAttentionInvalidAndContainment(t *testing.T) {
	// Check the actual escaped destination, including both overwrite and remove.
	for _, absolute := range []bool{false, true} {
		base := t.TempDir()
		project := filepath.Join(base, "project")
		outside := filepath.Join(base, "outside")
		for _, dir := range []string{project, outside} {
			if err := os.Mkdir(dir, 0755); err != nil {
				t.Fatal(err)
			}
		}
		sentinel := filepath.Join(outside, ".trackfw-attention.json")
		if err := os.WriteFile(sentinel, []byte("untouched"), 0600); err != nil {
			t.Fatal(err)
		}
		destination := "../outside"
		if absolute {
			destination = filepath.ToSlash(outside)
		}
		if err := os.WriteFile(filepath.Join(project, "trackfw.yaml"), []byte("roadmap_dir: '"+destination+"'\n"), 0600); err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"signal", "cleanup"} {
			runAttention(action, project, strings.NewReader(`{}`))
		}
		if data, err := os.ReadFile(sentinel); err != nil || string(data) != "untouched" {
			t.Fatalf("unsafe destination modified: %q %v", data, err)
		}
		entries, err := os.ReadDir(outside)
		if err != nil || len(entries) != 1 {
			t.Fatalf("outside tree changed: %v %v", entries, err)
		}
		if err := os.Remove(sentinel); err != nil {
			t.Fatal(err)
		}
		runAttention("signal", project, strings.NewReader(`{}`))
		entries, err = os.ReadDir(outside)
		if err != nil || len(entries) != 0 {
			t.Fatalf("unsafe signal created outside: %v %v", entries, err)
		}
	}
	for _, cfg := range []string{"roadmap_dir: [", "roadmap_dir: docs/a\n---\nroadmap_dir: docs/b\n", "roadmap_dir: ../outside\n", "roadmap_dir: /tmp/outside\n", "roadmap_dir: 'C:\\outside'\n", "roadmap_dir: 'docs\\..\\outside'\n", "roadmap_dir: docs/a\nroadmap_dir: docs/b\n"} {
		t.Run(cfg, func(t *testing.T) {
			dir, file := attentionFixture(t, cfg)
			for _, action := range []string{"signal", "cleanup"} {
				runAttention(action, dir, strings.NewReader(`{}`))
			}
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				t.Fatal("unsafe config wrote signal")
			}
		})
	}
	for _, payload := range []string{"{", "null", "[]", "{} {}", `{"tool_input":{"question":4}}`, strings.Repeat("x", attentionMaxInput+1)} {
		dir, file := attentionFixture(t, "# defaults\n")
		runAttention("signal", dir, strings.NewReader(payload))
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatal("bad payload wrote signal")
		}
	}
	for _, leaf := range []bool{false, true} {
		t.Run(map[bool]string{false: "ancestor", true: "leaf"}[leaf], func(t *testing.T) {
			dir, file := attentionFixture(t, "roadmap_dir: docs/roadmaps\n")
			outside := t.TempDir()
			sentinelDir := outside
			if !leaf {
				sentinelDir = filepath.Join(outside, "roadmaps")
				if err := os.Mkdir(sentinelDir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			sentinel := filepath.Join(sentinelDir, ".trackfw-attention.json")
			if err := os.WriteFile(sentinel, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			link := filepath.Join(dir, "docs")
			target := outside
			if leaf {
				if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
					t.Fatal(err)
				}
				link = file
				target = sentinel
			}
			symlinkOrSkipCmd(t, target, link)
			runAttention("signal", dir, strings.NewReader(`{}`))
			runAttention("cleanup", dir, nil)
			got, err := os.ReadFile(sentinel)
			if err != nil || string(got) != "untouched" {
				t.Fatalf("outside sentinel changed: %q %v", got, err)
			}
			if _, err := os.Lstat(link); err != nil {
				t.Fatal("cleanup removed link", err)
			}
			entries, err := os.ReadDir(sentinelDir)
			if err != nil || len(entries) != 1 {
				t.Fatalf("outside tree changed: %v %v", entries, err)
			}
		})
	}
}
func TestAttentionRegisteredCommandRuntime(t *testing.T) {
	bin := rootBinary(t)
	for _, cfg := range []string{"roadmap_dir: [", "roadmap_dir: docs/a\n---\nx: 1\n"} {
		dir, file := attentionFixture(t, cfg)
		cmd := exec.Command(bin, "attention", "signal")
		cmd.Dir = dir
		cmd.Stdin = strings.NewReader(`{}`)
		if out, err := cmd.CombinedOutput(); err != nil || len(out) != 0 {
			t.Fatalf("runtime must no-op0: %v %s", err, out)
		}
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatal("invalid YAML wrote signal")
		}
	}
	dir, file := attentionFixture(t, "# defaults\n")
	cmd := exec.Command(bin, "attention", "signal")
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"$(touch NEVER_EXECUTED)"}}`)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("registered command: %v %s", err, out)
	}
	got := readAttention(t, file)
	if got["message"] != "$(touch NEVER_EXECUTED)" {
		t.Fatal(got)
	}
	if _, err := os.Stat(filepath.Join(dir, "NEVER_EXECUTED")); !os.IsNotExist(err) {
		t.Fatal("payload executed")
	}
	credential := filepath.Join(filepath.Dir(file), ".trackfw-credential-guard.json")
	if err := os.WriteFile(credential, []byte("credential stays"), 0600); err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, bin, "attention", "cleanup")
	cmd.Dir = dir
	cmd.Stdin = r
	start := time.Now()
	if out, err := cmd.CombinedOutput(); err != nil || time.Since(start) > 3*time.Second {
		t.Fatalf("cleanup drained open stdin: %v %s", err, out)
	}
	data, err := os.ReadFile(credential)
	if err != nil || string(data) != "credential stays" {
		t.Fatal("credential signal touched")
	}
	ctxSignal, cancelSignal := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancelSignal()
	cmd = exec.CommandContext(ctxSignal, bin, "attention", "signal")
	cmd.Dir = dir
	cmd.Stdin = r
	start = time.Now()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("open stdin: %v %s", err, out)
	}
	if elapsed := time.Since(start); elapsed < attentionReadTimeout || elapsed > attentionReadTimeout+3*time.Second {
		t.Fatalf("stdin deadline %s", elapsed)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("timed out signal wrote file")
	}
}
func TestAttentionQuestionsField(t *testing.T) {
	// real payload extracted from Claude Code 2.1.304 transcript
	const realPayload = `{"tool_name":"AskUserQuestion","tool_input":{"questions":[{"question":"What is your favorite color?","header":"Color","options":[{"label":"Red","description":"A bold, warm color"},{"label":"Blue","description":"A calm, cool color"}],"multiSelect":false}]}}`

	t.Run("real_AskUserQuestion_payload", func(t *testing.T) {
		// Asserts: questions[0].question is extracted as message when tool_input.question is absent.
		dir, file := attentionFixture(t, "# defaults\n")
		runAttention("signal", dir, strings.NewReader(realPayload))
		got := readAttention(t, file)
		if got["message"] != "What is your favorite color?" {
			t.Fatalf("got %q, want %q", got["message"], "What is your favorite color?")
		}
	})

	t.Run("questions_uses_first", func(t *testing.T) {
		// Asserts: when questions has multiple entries, only the first question is used.
		const payload = `{"tool_name":"AskUserQuestion","tool_input":{"questions":[{"question":"First?"},{"question":"Second?"}]}}`
		dir, file := attentionFixture(t, "# defaults\n")
		runAttention("signal", dir, strings.NewReader(payload))
		got := readAttention(t, file)
		if got["message"] != "First?" {
			t.Fatalf("got %q, want %q", got["message"], "First?")
		}
	})

	// Robustness: each of these falls through to the fallback without panicking.
	// Asserts: malformed/empty/missing questions values all produce the fallback message,
	// and a file is still written (payload is otherwise well-formed).
	robustCases := []struct {
		name    string
		payload string
	}{
		{"questions_not_array", `{"tool_name":"T","tool_input":{"questions":"not-array"}}`},
		{"questions_empty_array", `{"tool_name":"T","tool_input":{"questions":[]}}`},
		{"questions_item_not_object", `{"tool_name":"T","tool_input":{"questions":[42]}}`},
		{"questions_item_question_absent", `{"tool_name":"T","tool_input":{"questions":[{"header":"no-question"}]}}`},
		{"questions_item_question_empty_string", `{"tool_name":"T","tool_input":{"questions":[{"question":""}]}}`},
		{"questions_item_question_nonstring", `{"tool_name":"T","tool_input":{"questions":[{"question":4}]}}`},
	}
	for _, tc := range robustCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dir, file := attentionFixture(t, "# defaults\n")
			runAttention("signal", dir, strings.NewReader(tc.payload))
			got := readAttention(t, file)
			if got["message"] != "Agent is executing: T" {
				t.Fatalf("got %q, want fallback %q", got["message"], "Agent is executing: T")
			}
		})
	}

	t.Run("questions_large_array_only_first_read", func(t *testing.T) {
		// Asserts: a questions array whose first entry is valid and whose tail has
		// ~200 k additional objects (total within attentionMaxInput) returns the
		// first question. Only the first element is decoded — the tail is never
		// allocated — so memory usage stays constant regardless of array length.
		tail := strings.Repeat(",{}", 200_000)
		raw := json.RawMessage(`[{"question":"only this"}` + tail + `]`)
		if got := attentionFirstQuestion(raw); got != "only this" {
			t.Fatalf("got %q, want %q", got, "only this")
		}
	})

	t.Run("questions_first_valid_rest_malformed", func(t *testing.T) {
		// Asserts: attentionFirstQuestion returns the first question even when the
		// remainder of the array is malformed JSON. The decoder stops after the
		// first element and never reads the broken tail.
		//
		// NOTE: in the complete runAttention flow this case is a no-op: the outer
		// json.Unmarshal on the full payload rejects malformed JSON before
		// attentionFirstQuestion is ever called. This subtest exercises the
		// function directly to document and guarantee its early-exit semantics in
		// isolation.
		raw := json.RawMessage(`[{"question":"ok"}, }`)
		if got := attentionFirstQuestion(raw); got != "ok" {
			t.Fatalf("got %q, want %q", got, "ok")
		}
	})
}

func TestAttentionInputBoundsAndCwd(t *testing.T) {
	data := bytes.Repeat([]byte(" "), attentionMaxInput)
	copy(data, []byte("{}"))
	if got, ok := attentionInput(bytes.NewReader(data), time.Second); !ok || len(got) != attentionMaxInput {
		t.Fatal("exact limit rejected")
	}
	if _, ok := attentionInput(bytes.NewReader(append(data, ' ')), time.Second); ok {
		t.Fatal("oversize accepted")
	}
	r, w := io.Pipe()
	defer r.Close()
	defer w.Close()
	start := time.Now()
	if _, ok := attentionInput(r, 30*time.Millisecond); ok || time.Since(start) > time.Second {
		t.Fatal("deadline failed")
	}
	dir, file := attentionFixture(t, "# defaults\n")
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0755); err != nil {
		t.Fatal(err)
	}
	runAttention("signal", sub, strings.NewReader(`{}`))
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Fatal("attention searched ancestors")
	}
	runAttention("signal", dir, bytes.NewReader(data))
	if readAttention(t, file)["message"] != "Agent is executing: unknown" {
		t.Fatal("default failed")
	}
}

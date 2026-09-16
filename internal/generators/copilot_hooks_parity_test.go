package generators

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// ---------------------------------------------------------------------------
// ML-2D — cross-stack structural parity for .github/hooks/trackfw-attention.json
// (Go vs Node vs Python), required by this ML's acceptance criteria. Unlike
// credential_guard_test.go's byte-identical script comparison, JSON key/value
// structure is compared here (not byte-for-byte) since each stack's own JSON
// serializer is allowed its own formatting style -- the shapes just need to
// agree, mirroring the plan for the ML-3A parity gate.
// ---------------------------------------------------------------------------

func getGoCopilotHooks(t *testing.T) map[string]interface{} {
	t.Helper()
	dir := t.TempDir()
	if err := InjectCopilotHooks(dir); err != nil {
		t.Fatalf("InjectCopilotHooks erro: %v", err)
	}
	return readCopilotHooksJSON(t, filepath.Join(dir, ".github", "hooks", "trackfw-attention.json"))
}


func readCopilotHooksJSON(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("erro lendo %s: %v", path, err)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("erro parseando %s: %v", path, err)
	}
	return data
}

// copilotHookEntryKey builds a comparable key for a command-hook entry
// (ignores map ordering, which JSON round-tripping does not preserve anyway).
func copilotHookEntrySet(t *testing.T, entries interface{}) []map[string]interface{} {
	t.Helper()
	arr, ok := entries.([]interface{})
	if !ok {
		t.Fatalf("esperava array de hook entries, obteve %T: %v", entries, entries)
	}
	out := make([]map[string]interface{}, 0, len(arr))
	for _, e := range arr {
		obj, ok := e.(map[string]interface{})
		if !ok {
			t.Fatalf("esperava hook entry como objeto, obteve %T: %v", e, e)
		}
		out = append(out, obj)
	}
	return out
}

func assertCopilotHookEntry(t *testing.T, stack string, entries []map[string]interface{}, bash string, wantMatcher interface{}) {
	t.Helper()
	for _, e := range entries {
		if e["bash"] != bash {
			continue
		}
		if e["type"] != "command" {
			t.Errorf("%s: entrada bash=%s tem type=%v, esperado \"command\"", stack, bash, e["type"])
		}
		if got := e["matcher"]; !reflect.DeepEqual(got, wantMatcher) {
			t.Errorf("%s: entrada bash=%s tem matcher=%v, esperado %v", stack, bash, got, wantMatcher)
		}
		return
	}
	t.Errorf("%s: entrada bash=%s não encontrada", stack, bash)
}

func TestInjectCopilotHooks_StructuralParityAcrossStacks(t *testing.T) {
	// ML-3A (v8 — um binário, muitos canais): Node.js and Python reimplementations
	// removed. Cross-stack structural comparison replaced with Go behavioral pin.
	// Isolate global credential-guard dedup check from the real $HOME.
	t.Setenv("HOME", t.TempDir())
	goData := getGoCopilotHooks(t)

	for name, data := range map[string]map[string]interface{}{"Go": goData} {
		version, ok := data["version"].(float64)
		if !ok || version != 1 {
			t.Errorf("%s: version deveria ser 1, obteve %v", name, data["version"])
		}
		hooks, ok := data["hooks"].(map[string]interface{})
		if !ok {
			t.Fatalf("%s: hooks deveria ser um objeto keyed por evento, obteve %v", name, data["hooks"])
		}

		pre := copilotHookEntrySet(t, hooks["preToolUse"])
		post := copilotHookEntrySet(t, hooks["postToolUse"])
		// postToolUse: 4 entries in all 3 stacks -- signal/cleanup (no matcher) +
		// credential-guard scoped to "bash", "view", and "create|edit"
		// (ADR-2026-08-06 emenda 7, ROADMAP-2026-08-08 Wave 2 — Read/Write/Edit
		// coverage).
		if len(post) != 4 {
			t.Errorf("%s: postToolUse deveria ter 4 entradas, obteve %d", name, len(post))
		}
		// preToolUse: all three stacks carry the git-branch-guard "bash" entry
		// (ROADMAP-2026-08-14 ML-3A/ML-3B/ML-3C), so all three have 5 entries.
		wantPre := 5
		assertCopilotHookEntry(t, name, pre, "scripts/trackfw-git-branch-guard.sh", "bash")
		if len(pre) != wantPre {
			t.Errorf("%s: preToolUse deveria ter %d entradas, obteve %d", name, wantPre, len(pre))
		}

		assertCopilotHookEntry(t, name, pre, "scripts/trackfw-attention-signal.sh", nil)
		assertCopilotHookEntry(t, name, pre, "scripts/trackfw-credential-guard.sh", "bash")
		assertCopilotHookEntry(t, name, post, "scripts/trackfw-attention-cleanup.sh", nil)
		assertCopilotHookEntry(t, name, post, "scripts/trackfw-credential-guard.sh", "bash")
	}
}

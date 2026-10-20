package generators

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() erro: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("Não foi possível encontrar a raiz do repositório (go.mod)")
		}
		dir = parent
	}
}

func getGoScripts(t *testing.T) (signal, cleanup string) {
	t.Helper()
	dir := t.TempDir()
	orig, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(orig) }()

	if err := GenerateAttentionScripts(""); err != nil {
		t.Fatalf("generateAttentionScripts erro: %v", err)
	}

	sigBytes, err := os.ReadFile(filepath.Join("scripts", "trackfw-attention-signal.sh"))
	if err != nil {
		t.Fatalf("erro lendo signal em Go: %v", err)
	}

	cleanBytes, err := os.ReadFile(filepath.Join("scripts", "trackfw-attention-cleanup.sh"))
	if err != nil {
		t.Fatalf("erro lendo cleanup em Go: %v", err)
	}

	return string(sigBytes), string(cleanBytes)
}

func TestScriptsParity_GoldenCanonicalBlocks(t *testing.T) {
	signal, cleanup := getGoScripts(t)
	for name, script := range map[string]string{"signal": signal, "cleanup": cleanup} {
		if !strings.Contains(script, "trackfw attention "+name+` "$@" || true`) {
			t.Errorf("wrapper %s does not delegate safely: %s", name, script)
		}
		for _, obsolete := range []string{"python3", "jq ", "ROADMAP_DIR", "INPUT="} {
			if strings.Contains(script, obsolete) {
				t.Errorf("wrapper retains parser %q", obsolete)
			}
		}
	}
}

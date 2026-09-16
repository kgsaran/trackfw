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
	// ML-3A (v8 — um binário, muitos canais): Node.js and Python reimplementations
	// removed. Cross-stack comparison replaced with Go behavioral pin.
	goSig, goClean := getGoScripts(t)

	clis := map[string]struct{ signal, cleanup string }{
		"Go": {goSig, goClean},
	}

	// Canonical Block 1: Path Traversal Case Statement
	caseBlock := `case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac`

	for cliName, scripts := range clis {
		if !strings.Contains(scripts.signal, caseBlock) {
			t.Errorf("[%s Signal] Bloco case de path traversal diverge do canônico esperado:\n%s", cliName, scripts.signal)
		}
		if !strings.Contains(scripts.cleanup, caseBlock) {
			t.Errorf("[%s Cleanup] Bloco case de path traversal diverge do canônico esperado:\n%s", cliName, scripts.cleanup)
		}
	}

	// Canonical Block 2: tr -d '\000-\037' e escaping sed
	for cliName, scripts := range clis {
		if !strings.Contains(scripts.signal, `tr -d '\000-\037'`) {
			t.Errorf("[%s Signal] Sanitização não contém 'tr -d \\'\\000-\\037\\'' para remoção de caracteres de controle", cliName)
		}
		if !strings.Contains(scripts.signal, `sed`) || (!strings.Contains(scripts.signal, `s/\\/\\\\/g`) && !strings.Contains(scripts.signal, `s/\\\\/\\\\\\\\/g`)) {
			t.Errorf("[%s Signal] Sanitização não contém comando sed esperado para escaping de barras/aspas", cliName)
		}
	}

	// Canonical Block 3: roadmap_dir extraction com grep/sed
	roadmapDirGrep := `grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//'`
	for cliName, scripts := range clis {
		if !strings.Contains(scripts.signal, roadmapDirGrep) {
			t.Errorf("[%s Signal] Extração de roadmap_dir via grep/sed diverge do canônico", cliName)
		}
		if !strings.Contains(scripts.cleanup, roadmapDirGrep) {
			t.Errorf("[%s Cleanup] Extração de roadmap_dir via grep/sed diverge do canônico", cliName)
		}
	}

	// Canonical Block 4: Comentário de CWD no-op
	for cliName, scripts := range clis {
		if !strings.Contains(scripts.signal, `[ -f "trackfw.yaml" ] || exit 0`) {
			t.Errorf("[%s Signal] Verificação de cwd '[ -f \"trackfw.yaml\" ] || exit 0' ausente", cliName)
		}
		if !strings.Contains(scripts.cleanup, `[ -f "trackfw.yaml" ] || exit 0`) {
			t.Errorf("[%s Cleanup] Verificação de cwd '[ -f \"trackfw.yaml\" ] || exit 0' ausente", cliName)
		}

		// Verifica presença de comentário explicativo antes do check
		sigLines := strings.Split(scripts.signal, "\n")
		foundCommentSig := false
		for i, line := range sigLines {
			if strings.Contains(line, `[ -f "trackfw.yaml" ] || exit 0`) {
				if i > 0 && strings.HasPrefix(strings.TrimSpace(sigLines[i-1]), "#") {
					foundCommentSig = true
				}
			}
		}
		if !foundCommentSig {
			t.Errorf("[%s Signal] Comentário explicativo de cwd ausente antes de '[ -f \"trackfw.yaml\" ] || exit 0'", cliName)
		}

		cleanLines := strings.Split(scripts.cleanup, "\n")
		foundCommentClean := false
		for i, line := range cleanLines {
			if strings.Contains(line, `[ -f "trackfw.yaml" ] || exit 0`) {
				if i > 0 && strings.HasPrefix(strings.TrimSpace(cleanLines[i-1]), "#") {
					foundCommentClean = true
				}
			}
		}
		if !foundCommentClean {
			t.Errorf("[%s Cleanup] Comentário explicativo de cwd ausente antes de '[ -f \"trackfw.yaml\" ] || exit 0'", cliName)
		}
	}
}

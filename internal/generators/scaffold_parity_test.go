package generators

import (
	"os"
	"path/filepath"
	"regexp"
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

	if err := generateAttentionScripts(); err != nil {
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

func getNodeScripts(t *testing.T, repoRoot string) (signal, cleanup string) {
	t.Helper()
	hooksPath := filepath.Join(repoRoot, "npm", "src", "generators", "hooks.js")
	content, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("erro lendo %s: %v", hooksPath, err)
	}

	s := string(content)

	sigMatch := regexp.MustCompile(`const SIGNAL_SCRIPT = \x60([\s\S]*?)\x60`).FindStringSubmatch(s)
	if len(sigMatch) < 2 {
		t.Fatalf("SIGNAL_SCRIPT não encontrado em npm/src/generators/hooks.js")
	}

	cleanMatch := regexp.MustCompile(`const CLEANUP_SCRIPT = \x60([\s\S]*?)\x60`).FindStringSubmatch(s)
	if len(cleanMatch) < 2 {
		t.Fatalf("CLEANUP_SCRIPT não encontrado em npm/src/generators/hooks.js")
	}

	// Normaliza escapes de template JS (\${ROADMAP_DIR} -> ${ROADMAP_DIR}, \\ -> \)
	normNode := func(script string) string {
		res := strings.ReplaceAll(script, `\${ROADMAP_DIR:-docs/roadmaps}`, `${ROADMAP_DIR:-docs/roadmaps}`)
		res = strings.ReplaceAll(res, `\\000-\\037`, `\000-\037`)
		res = strings.ReplaceAll(res, `\\\\`, `\\`)
		res = strings.ReplaceAll(res, `\\\"`, `\"`)
		res = strings.ReplaceAll(res, `\\n`, `\n`)
		return res
	}

	return normNode(sigMatch[1]), normNode(cleanMatch[1])
}

func getPythonScripts(t *testing.T, repoRoot string) (signal, cleanup string) {
	t.Helper()
	initPath := filepath.Join(repoRoot, "pypi", "trackfw", "generators", "init_gen.py")
	content, err := os.ReadFile(initPath)
	if err != nil {
		t.Fatalf("erro lendo %s: %v", initPath, err)
	}

	s := string(content)

	sigMatch := regexp.MustCompile(`_ATTENTION_SIGNAL_SH = r?"""([\s\S]*?)"""`).FindStringSubmatch(s)
	if len(sigMatch) < 2 {
		t.Fatalf("_ATTENTION_SIGNAL_SH não encontrado em pypi/trackfw/generators/init_gen.py")
	}

	cleanMatch := regexp.MustCompile(`_ATTENTION_CLEANUP_SH = r?"""([\s\S]*?)"""`).FindStringSubmatch(s)
	if len(cleanMatch) < 2 {
		t.Fatalf("_ATTENTION_CLEANUP_SH não encontrado em pypi/trackfw/generators/init_gen.py")
	}

	return strings.TrimSpace(sigMatch[1]), strings.TrimSpace(cleanMatch[1])
}

func TestScriptsParity_GoldenCanonicalBlocks(t *testing.T) {
	repoRoot := findRepoRoot(t)

	goSig, goClean := getGoScripts(t)
	nodeSig, nodeClean := getNodeScripts(t, repoRoot)
	pySig, pyClean := getPythonScripts(t, repoRoot)

	clis := map[string]struct{ signal, cleanup string }{
		"Go":     {goSig, goClean},
		"Node":   {nodeSig, nodeClean},
		"Python": {pySig, pyClean},
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

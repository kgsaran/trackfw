package i18n

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// resetPackage resets the sync.Once and message cache so that the next call
// to load() re-reads DetectLocale(). Call this at the start of any unit test
// that sets locale env vars before calling T(). After this ML (ML-3E), load()
// always selects enUS regardless of locale — the reset only matters for the
// red run against the old code.
func resetPackage() {
	once = sync.Once{}
	messages = nil
}

// ─── Unit tests ──────────────────────────────────────────────────────────────

// TestDetectLocale_AlwaysEN — AFIRMA que DetectLocale() retorna "en-US"
// independentemente dos valores de LANG, LC_ALL e LANGUAGE.
func TestDetectLocale_AlwaysEN(t *testing.T) {
	cases := []struct{ env, val string }{
		{"LANG", "pt_BR.UTF-8"},
		{"LC_ALL", "pt_BR.UTF-8"},
		{"LANGUAGE", "pt_BR.UTF-8"},
		{"LANG", "es_ES.UTF-8"},
		{"LC_ALL", "es_ES.UTF-8"},
		{"LANGUAGE", "es_ES.UTF-8"},
		{"LANG", "C"},
		{"LANG", ""},
	}
	for _, c := range cases {
		t.Run(c.env+"="+c.val, func(t *testing.T) {
			t.Setenv(c.env, c.val)
			got := DetectLocale()
			if got != "en-US" {
				t.Errorf("DetectLocale() with %s=%q = %q, want \"en-US\"", c.env, c.val, got)
			}
		})
	}
}

// TestI18nT_AlwaysEN_UnderPTLocale — AFIRMA que i18n.T() retorna texto EN com
// LANG/LC_ALL/LANGUAGE apontando para pt_BR.UTF-8.
func TestI18nT_AlwaysEN_UnderPTLocale(t *testing.T) {
	t.Setenv("LANG", "pt_BR.UTF-8")
	t.Setenv("LC_ALL", "pt_BR.UTF-8")
	t.Setenv("LANGUAGE", "pt_BR.UTF-8")
	resetPackage()
	t.Cleanup(resetPackage)

	got := T("validate.ok")
	want := "✓ No violations found."
	if got != want {
		t.Errorf("T(\"validate.ok\") with LANG=pt_BR.UTF-8 = %q, want %q", got, want)
	}

	gotW := T("validate.warnings", "count", "1")
	if !strings.Contains(gotW, "warning") {
		t.Errorf("T(\"validate.warnings\") with LANG=pt_BR.UTF-8 = %q; want text containing \"warning\"", gotW)
	}

	gotD := T("validate.description")
	if !strings.Contains(gotD, "Validate") {
		t.Errorf("T(\"validate.description\") with LANG=pt_BR.UTF-8 = %q; want EN text containing \"Validate\"", gotD)
	}
}

// TestI18nT_AlwaysEN_UnderESLocale — AFIRMA que i18n.T() retorna texto EN com
// LANG/LC_ALL/LANGUAGE apontando para es_ES.UTF-8 (simetria com o braço PT).
func TestI18nT_AlwaysEN_UnderESLocale(t *testing.T) {
	t.Setenv("LANG", "es_ES.UTF-8")
	t.Setenv("LC_ALL", "es_ES.UTF-8")
	t.Setenv("LANGUAGE", "es_ES.UTF-8")
	resetPackage()
	t.Cleanup(resetPackage)

	got := T("validate.ok")
	want := "✓ No violations found."
	if got != want {
		t.Errorf("T(\"validate.ok\") with LANG=es_ES.UTF-8 = %q, want %q", got, want)
	}

	gotW := T("validate.warnings", "count", "2")
	if !strings.Contains(gotW, "warning") {
		t.Errorf("T(\"validate.warnings\") with LANG=es_ES.UTF-8 = %q; want text containing \"warning\"", gotW)
	}
}

// ─── End-to-end test (builds binary) ─────────────────────────────────────────

// TestI18n_BinaryOutput_AlwaysEN — AFIRMA que o binário real emite texto EN em
// "trackfw validate" e "trackfw validate --help" com LANG, LC_ALL e LANGUAGE
// configurados para pt_BR.UTF-8.
func TestI18n_BinaryOutput_AlwaysEN(t *testing.T) {
	bin := i18nBuildBinary(t)

	projDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projDir, "trackfw.yaml"), []byte("project: test\n"), 0o644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}

	ptEnv := append(os.Environ(),
		"LANG=pt_BR.UTF-8",
		"LC_ALL=pt_BR.UTF-8",
		"LANGUAGE=pt_BR.UTF-8",
	)

	t.Run("validate_contains_EN_warnings", func(t *testing.T) {
		cmd := exec.Command(bin, "validate")
		cmd.Dir = projDir
		cmd.Env = ptEnv
		out, _ := cmd.CombinedOutput()
		s := string(out)
		// Positive assertion: must contain EN marker
		if !strings.Contains(s, "warning(s)") && !strings.Contains(s, "No violations found") && !strings.Contains(s, "violation(s)") {
			t.Errorf("validate output with LANG=pt_BR.UTF-8 has no EN marker; output:\n%s", s)
		}
		// Negative assertion: must not contain PT markers
		for _, ptMark := range []string{"aviso(s)", "violação", "Nenhuma violação", "Validar"} {
			if strings.Contains(s, ptMark) {
				t.Errorf("validate output with LANG=pt_BR.UTF-8 contains PT marker %q; output:\n%s", ptMark, s)
			}
		}
	})

	t.Run("validate_help_contains_EN_description", func(t *testing.T) {
		cmd := exec.Command(bin, "validate", "--help")
		cmd.Dir = projDir
		cmd.Env = ptEnv
		out, _ := cmd.CombinedOutput()
		s := string(out)
		// Positive assertion: EN short description must appear
		if !strings.Contains(s, "Validate governance rules") {
			t.Errorf("validate --help with LANG=pt_BR.UTF-8 missing EN description; output:\n%s", s)
		}
		// Negative assertion: PT description must not appear
		if strings.Contains(s, "Validar") {
			t.Errorf("validate --help with LANG=pt_BR.UTF-8 contains PT marker \"Validar\"; output:\n%s", s)
		}
	})
}

// i18nBuildBinary compiles ./cmd/trackfw into a temp directory and returns
// the binary path. Each call to TestI18n_BinaryOutput_AlwaysEN uses a fresh
// build so the test always exercises the binary on-disk code.
func i18nBuildBinary(t *testing.T) string {
	t.Helper()
	root := i18nFindModuleRoot(t)
	dir := t.TempDir()
	bin := filepath.Join(dir, i18nBinaryName())
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/trackfw")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build ./cmd/trackfw failed: %v\n%s", err, out)
	}
	return bin
}

func i18nFindModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from working directory")
		}
		dir = parent
	}
}

func i18nBinaryName() string {
	if runtime.GOOS == "windows" {
		return "trackfw.exe"
	}
	return "trackfw"
}

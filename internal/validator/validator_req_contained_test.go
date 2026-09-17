package validator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── Falsificações ML-1A-quinquies: fail-closed em checkREQDirContained ───
//
// Estes testes exercitam checkREQDirContained diretamente (package validator),
// usando o seam interno getwdFn sem precisar exportar nenhum símbolo.
// Migrado de package sync (ML-1A-quater) após a auditoria recusar GetwdFn
// exportada: um botão exportado que desliga a contenção contradiz o próprio
// objetivo da REQ.

// TestCheckREQDirContained_GetWdFails_FailClosed afirma que checkREQDirContained
// retorna erro (não nil) quando getwdFn() falha — Direção A do fail-closed.
// Afirma: a correção reprova se checkREQDirContained devolver nil nessa condição.
// Técnica: injeção direta de getwdFn; portável em todas as plataformas.
func TestCheckREQDirContained_GetWdFails_FailClosed(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	origFn := getwdFn
	getwdFn = func() (string, error) {
		return "", errors.New("injected: getwd unavailable")
	}
	t.Cleanup(func() { getwdFn = origFn })

	gotErr := checkREQDirContained("docs/req")

	if gotErr == nil {
		t.Fatal("ML-1A-quinquies (Getwd falha): esperava erro, obteve nil")
	}
	if !strings.Contains(gotErr.Error(), "diretório de trabalho") {
		t.Errorf("ML-1A-quinquies (Getwd falha): esperava diagnóstico de CWD indisponível, obteve: %v", gotErr)
	}
}

// TestCheckREQDirContained_EvalSymlinksCWDFails_FailClosed afirma que
// checkREQDirContained retorna erro (não nil) quando getwdFn retorna um caminho
// sintaticamente válido mas fisicamente irresolvível (EvalSymlinks falha) —
// Direção B do fail-closed.
// Afirma: a correção reprova se checkREQDirContained devolver nil nessa condição.
// Técnica: injeção de getwdFn retornando caminho inexistente; portável em todas
// as plataformas. filepath.EvalSymlinks falha em todas as plataformas quando o
// caminho alvo não existe.
func TestCheckREQDirContained_EvalSymlinksCWDFails_FailClosed(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// Inject getwd to return a path that is syntactically valid but does not
	// exist on disk. filepath.EvalSymlinks fails on all platforms when the
	// target path is absent, exercising the second fail-closed branch.
	nonexistent := filepath.Join(t.TempDir(), "gone-dir-removed")
	origFn := getwdFn
	getwdFn = func() (string, error) {
		return nonexistent, nil
	}
	t.Cleanup(func() { getwdFn = origFn })

	gotErr := checkREQDirContained("docs/req")

	if gotErr == nil {
		t.Fatal("ML-1A-quinquies (EvalSymlinks falha): esperava erro, obteve nil")
	}
	if !strings.Contains(gotErr.Error(), "EvalSymlinks falhou") {
		t.Errorf("ML-1A-quinquies (EvalSymlinks falha): esperava diagnóstico de EvalSymlinks, obteve: %v", gotErr)
	}
}

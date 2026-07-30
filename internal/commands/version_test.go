package commands

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

// versionLineRE é o contrato congelado do cli-parity.md §Version output:
// exatamente "trackfw <major>.<minor>.<patch>", sem prefixo v, sem sufixo.
var versionLineRE = regexp.MustCompile(`^trackfw [0-9]+\.[0-9]+\.[0-9]+$`)

// captureVersionSubcmd executa o subcomando "version" e retorna a linha impressa,
// sem o \n final.
func captureVersionSubcmd(t *testing.T) string {
	t.Helper()

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("trackfw version: erro inesperado: %v", err)
	}
	return strings.TrimRight(buf.String(), "\n")
}

// captureVersionFlag executa a flag --version e retorna a linha impressa,
// sem o \n final.
func captureVersionFlag(t *testing.T) string {
	t.Helper()

	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"--version"})
	// cobra trata --version como ação especial e retorna nil após imprimir.
	_ = root.Execute()
	return strings.TrimRight(buf.String(), "\n")
}

// TestVersionSubcmdFormat garante que "trackfw version" imprime uma linha
// no formato exato do contrato, sem prefixo v.
func TestVersionSubcmdFormat(t *testing.T) {
	got := captureVersionSubcmd(t)
	if !versionLineRE.MatchString(got) {
		t.Errorf("trackfw version: saída %q não bate com %s", got, versionLineRE)
	}
}

// TestVersionFlagFormat garante que "trackfw --version" imprime uma linha
// no formato exato do contrato, sem prefixo v.
func TestVersionFlagFormat(t *testing.T) {
	got := captureVersionFlag(t)
	if !versionLineRE.MatchString(got) {
		t.Errorf("trackfw --version: saída %q não bate com %s", got, versionLineRE)
	}
}

// TestVersionSurfacesByteIdentical garante que "trackfw version" e
// "trackfw --version" são byte-idênticos (contrato cli-parity.md).
func TestVersionSurfacesByteIdentical(t *testing.T) {
	subcmd := captureVersionSubcmd(t)
	flag := captureVersionFlag(t)

	if subcmd != flag {
		t.Errorf("superfícies divergem:\n  version   = %q\n  --version = %q", subcmd, flag)
	}
}

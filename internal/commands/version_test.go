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

// TestShorthandVNotRegistered garante que o shorthand "v" nunca está registrado
// na flag set do root. Esta é a asserção estrutural que blinda contra regressão:
// se alguém reintroduzir o atalho (ex: removendo a pré-declaração em root.go),
// este teste falha imediatamente, antes mesmo de executar o binário.
// Motivação: -v/-−verbose é reservado para modo verboso (cli-parity.md).
func TestShorthandVNotRegistered(t *testing.T) {
	root := newRootCmd()
	if f := root.Flags().ShorthandLookup("v"); f != nil {
		t.Errorf("shorthand 'v' está registrado na flag %q — deve ser removido (cli-parity.md reserva -v para verbose)", f.Name)
	}
}

// TestShortVFlagRejected garante que "-v" retorna erro não-nulo e não imprime
// a linha de versão no stdout (contrato cli-parity.md §-v is reserved for verbose).
// O teste captura stdout separadamente do stderr para provar que a saída de
// versão não vazou — exit code sozinho não distingue "rejeitada" de "aceita com falha".
func TestShortVFlagRejected(t *testing.T) {
	root := newRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"-v"})

	err := root.Execute()

	if err == nil {
		t.Fatal("trackfw -v: esperava erro, obteve nil")
	}
	got := strings.TrimRight(stdout.String(), "\n")
	if versionLineRE.MatchString(got) {
		t.Errorf("trackfw -v: linha de versão não deve aparecer no stdout, obteve %q", got)
	}
}

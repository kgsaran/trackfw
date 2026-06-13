package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpNoArgs(t *testing.T) {
	cmd := newHelpCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("esperava sem erro, obteve: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "adr_dirs") {
		t.Errorf("esperava 'adr_dirs' na saída, obteve:\n%s", out)
	}
	if !strings.Contains(out, "wip_limit") {
		t.Errorf("esperava 'wip_limit' na saída, obteve:\n%s", out)
	}
}

func TestHelpKnownKey(t *testing.T) {
	cmd := newHelpCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"wip_limit"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("esperava sem erro, obteve: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Default: 1") {
		t.Errorf("esperava 'Default: 1' na saída, obteve:\n%s", out)
	}
	if !strings.Contains(out, "integer") {
		t.Errorf("esperava 'integer' na saída, obteve:\n%s", out)
	}
}

func TestHelpUnknownKey(t *testing.T) {
	cmd := newHelpCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&bytes.Buffer{}) // suprimir stderr no teste
	cmd.SetArgs([]string{"nao_existe"})

	err := cmd.RunE(cmd, []string{"nao_existe"})
	if err == nil {
		t.Fatal("esperava erro para chave desconhecida, obteve nil")
	}
	if !strings.Contains(err.Error(), "nao_existe") {
		t.Errorf("esperava 'nao_existe' na mensagem de erro, obteve: %v", err)
	}
}

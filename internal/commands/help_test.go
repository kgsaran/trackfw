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
	// Verificar entradas traceid adicionadas no ML-1A.
	if !strings.Contains(out, "trace_id_field") {
		t.Errorf("esperava 'trace_id_field' na saída, obteve:\n%s", out)
	}
	if !strings.Contains(out, "rules.traceid_orphan_roadmap") {
		t.Errorf("esperava 'rules.traceid_orphan_roadmap' na saída, obteve:\n%s", out)
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

func TestHelpUnknownTopicHasNoSuggestionWhenTooDistant(t *testing.T) {
	cmd := newHelpCmd()
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.RunE(cmd, []string{"chave-que-nao-existe"})
	if err == nil {
		t.Fatal("esperava erro para assunto desconhecido, obteve nil")
	}
	if strings.Contains(err.Error(), "Você quis dizer") {
		t.Errorf("não esperava sugestão para um assunto sem candidato próximo, obteve: %v", err)
	}
}

func TestHelpUnknownTopicSuggestsNearMatch(t *testing.T) {
	cmd := newHelpCmd()
	cmd.SetOut(&bytes.Buffer{})

	err := cmd.RunE(cmd, []string{"wip_limi"})
	if err == nil {
		t.Fatal("esperava erro para assunto desconhecido, obteve nil")
	}
	if !strings.Contains(err.Error(), "Você quis dizer: wip_limit?") {
		t.Errorf("esperava sugestão 'wip_limit', obteve: %v", err)
	}
}

func TestHelpKnownCommand(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"help", "init"})

	if err := root.Execute(); err != nil {
		t.Fatalf("esperava sem erro, obteve: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "trackfw init") {
		t.Errorf("esperava ajuda do comando init na saída, obteve:\n%s", out)
	}
}

func TestHelpDoesNotRegisterDuplicateEntry(t *testing.T) {
	root := newRootCmd()
	count := 0
	for _, sub := range root.Commands() {
		if sub.Name() == "help" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("esperava exatamente 1 comando 'help' registrado, obteve %d", count)
	}
}

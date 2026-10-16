package identity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_MissingFileReturnsZeroValue(t *testing.T) {
	homeDir := t.TempDir()

	cfg, err := Load(homeDir)
	if err != nil {
		t.Fatalf("Load com arquivo inexistente nao deveria retornar erro: %v", err)
	}
	if cfg.SchemaVersion != 0 || cfg.UserNickname != "" || cfg.Agents != nil {
		t.Fatalf("Load com arquivo inexistente deveria retornar Config zero-value, obteve %+v", cfg)
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	homeDir := t.TempDir()
	dir := filepath.Join(homeDir, ".trackfw")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "identity.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := Load(homeDir); err == nil {
		t.Fatal("Load deveria retornar erro para JSON invalido")
	}
}

func TestLoad_UnsupportedSchemaVersion(t *testing.T) {
	homeDir := t.TempDir()
	dir := filepath.Join(homeDir, ".trackfw")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	content := `{"schema_version": 99}`
	if err := os.WriteFile(filepath.Join(dir, "identity.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Load(homeDir)
	if err == nil {
		t.Fatal("Load deveria retornar erro para schema_version nao suportada")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Fatalf("mensagem de erro deveria citar a versao encontrada (99), obteve: %v", err)
	}
}

func TestLoad_NilAgentsBecomeEmptyMap(t *testing.T) {
	homeDir := t.TempDir()
	dir := filepath.Join(homeDir, ".trackfw")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	content := `{"schema_version": 1}`
	if err := os.WriteFile(filepath.Join(dir, "identity.json"), []byte(content), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cfg, err := Load(homeDir)
	if err != nil {
		t.Fatalf("Load nao deveria falhar: %v", err)
	}
	if cfg.Agents == nil {
		t.Fatal("Agents deveria ser inicializado como mapa vazio, nao nil")
	}
	if len(cfg.Agents) != 0 {
		t.Fatalf("Agents deveria estar vazio, tem %d entradas", len(cfg.Agents))
	}
}

func TestSaveThenLoad_RoundTrip(t *testing.T) {
	homeDir := t.TempDir()
	cfg := Config{
		UserNickname: "Kleber",
		Agents: map[string]AgentIdentity{
			"backend": {DisplayName: "Apolo", Slug: "apolo"},
		},
	}

	if err := Save(homeDir, cfg); err != nil {
		t.Fatalf("Save falhou: %v", err)
	}

	loaded, err := Load(homeDir)
	if err != nil {
		t.Fatalf("Load falhou: %v", err)
	}
	if loaded.SchemaVersion != schemaVersion {
		t.Fatalf("SchemaVersion = %d, esperava %d", loaded.SchemaVersion, schemaVersion)
	}
	if loaded.UserNickname != "Kleber" {
		t.Fatalf("UserNickname = %q, esperava %q", loaded.UserNickname, "Kleber")
	}
	agent, ok := loaded.Agents["backend"]
	if !ok {
		t.Fatal("agente 'backend' deveria estar presente apos round-trip")
	}
	if agent.DisplayName != "Apolo" || agent.Slug != "apolo" {
		t.Fatalf("agente 'backend' = %+v, esperava DisplayName=Apolo Slug=apolo", agent)
	}
}


func TestSave_ForcesSchemaVersion(t *testing.T) {
	homeDir := t.TempDir()
	cfg := Config{SchemaVersion: 999}

	if err := Save(homeDir, cfg); err != nil {
		t.Fatalf("Save falhou: %v", err)
	}

	loaded, err := Load(homeDir)
	if err != nil {
		t.Fatalf("Load falhou: %v", err)
	}
	if loaded.SchemaVersion != schemaVersion {
		t.Fatalf("Save deveria forcar SchemaVersion=%d, obteve %d", schemaVersion, loaded.SchemaVersion)
	}
}

func TestAgentName_AppliesSuffix(t *testing.T) {
	if got := AgentName("zeus"); got != "zeus-tf" {
		t.Fatalf("AgentName(zeus) = %q, esperava zeus-tf", got)
	}
}

func TestValidate_RejectsUnknownAgentID(t *testing.T) {
	cfg := Config{
		Agents: map[string]AgentIdentity{
			"unknown-id": {DisplayName: "Foo", Slug: "foo"},
		},
	}
	err := Validate(cfg, []string{"backend"})
	if err == nil {
		t.Fatal("Validate deveria rejeitar id desconhecido")
	}
}

func TestValidate_RejectsEmptyDisplayName(t *testing.T) {
	cfg := Config{
		Agents: map[string]AgentIdentity{
			"backend": {DisplayName: "", Slug: "apolo"},
		},
	}
	err := Validate(cfg, []string{"backend"})
	if err == nil {
		t.Fatal("Validate deveria rejeitar DisplayName vazio")
	}
}

func TestValidate_RejectsInvalidSlug(t *testing.T) {
	cases := []string{"Apolo", "apolo_deus", "-apolo", "apolo-", "apolo--deus", ""}
	for _, slug := range cases {
		cfg := Config{
			Agents: map[string]AgentIdentity{
				"backend": {DisplayName: "Apolo", Slug: slug},
			},
		}
		if err := Validate(cfg, []string{"backend"}); err == nil {
			t.Fatalf("Validate deveria rejeitar slug invalido %q", slug)
		}
	}
}

func TestValidate_RejectsDuplicateSlugs(t *testing.T) {
	cfg := Config{
		Agents: map[string]AgentIdentity{
			"backend":  {DisplayName: "Apolo", Slug: "apolo"},
			"frontend": {DisplayName: "Apolo Dois", Slug: "apolo"},
		},
	}
	err := Validate(cfg, []string{"backend", "frontend"})
	if err == nil {
		t.Fatal("Validate deveria rejeitar slugs duplicados")
	}
	if !strings.Contains(err.Error(), "backend") || !strings.Contains(err.Error(), "frontend") {
		t.Fatalf("mensagem de erro deveria citar ambos os ids, obteve: %v", err)
	}
}

func TestValidate_RejectsSlugWithTFSuffix(t *testing.T) {
	cfg := Config{
		Agents: map[string]AgentIdentity{
			"architect": {DisplayName: "Zeus", Slug: "zeus-tf"},
		},
	}
	err := Validate(cfg, []string{"architect"})
	if err == nil {
		t.Fatal("Validate deveria rejeitar slug terminado em -tf")
	}
	if !strings.Contains(err.Error(), "architect") || !strings.Contains(err.Error(), "zeus-tf") {
		t.Fatalf("mensagem de erro deveria citar o id e o slug, obteve: %v", err)
	}
}

func TestValidate_AcceptsSlugsThatDoNotEndWithTFSuffix(t *testing.T) {
	cases := []string{"zeus", "tf", "meu-tf-agente"}
	for _, slug := range cases {
		cfg := Config{
			Agents: map[string]AgentIdentity{
				"architect": {DisplayName: "Zeus", Slug: slug},
			},
		}
		if err := Validate(cfg, []string{"architect"}); err != nil {
			t.Fatalf("Validate nao deveria rejeitar slug %q: %v", slug, err)
		}
	}
}

func TestValidate_AcceptsValidConfig(t *testing.T) {
	cfg, err := Preset("greek")
	if err != nil {
		t.Fatalf("Preset(greek) nao deveria falhar: %v", err)
	}
	if err := Validate(cfg, KnownAgentIDs()); err != nil {
		t.Fatalf("Validate nao deveria rejeitar o preset grego: %v", err)
	}
}

func TestLookup(t *testing.T) {
	cfg, err := Preset("greek")
	if err != nil {
		t.Fatalf("Preset(greek) nao deveria falhar: %v", err)
	}

	agent, ok := Lookup(cfg, "backend")
	if !ok {
		t.Fatal("Lookup deveria encontrar o agente 'backend'")
	}
	if agent.Slug != "apolo" {
		t.Fatalf("Lookup(backend).Slug = %q, esperava apolo", agent.Slug)
	}

	_, ok = Lookup(cfg, "nao-existe")
	if ok {
		t.Fatal("Lookup nao deveria encontrar agente inexistente")
	}

	_, ok = Lookup(Config{}, "backend")
	if ok {
		t.Fatal("Lookup em Config zero-value nao deveria encontrar nada")
	}
}

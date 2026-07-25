package identity

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// schemaVersion is the current schema version for the identity config file.
const schemaVersion = 1

// slugPattern matches a valid slug: lowercase alphanumeric segments joined by
// single hyphens, e.g. "zeus", "meu-agente".
var slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// AgentIdentity holds the user-customizable identity of a single agent.
type AgentIdentity struct {
	DisplayName string `json:"display_name"`
	Slug        string `json:"slug"`
}

// Config is the persisted identity configuration, stored at
// <homeDir>/.trackfw/identity.json.
type Config struct {
	SchemaVersion int                      `json:"schema_version"`
	UserNickname  string                   `json:"user_nickname,omitempty"`
	Agents        map[string]AgentIdentity `json:"agents,omitempty"`
}

func identityPath(homeDir string) string {
	return filepath.Join(homeDir, ".trackfw", "identity.json")
}

// Load reads the identity config from <homeDir>/.trackfw/identity.json.
//
// If the file does not exist, Load returns a zero-value Config and a nil
// error — this is the non-regression path and must never surface as a
// failure to callers that have not yet customized their identity.
func Load(homeDir string) (Config, error) {
	filename := identityPath(homeDir)
	data, err := os.ReadFile(filename)
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("identity: falha ao ler %s: %w", filename, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("identity: falha ao decodificar %s: %w", filename, err)
	}
	if cfg.SchemaVersion != schemaVersion {
		return Config{}, fmt.Errorf("identity: versao de schema nao suportada em %s: %d (esperado %d)", filename, cfg.SchemaVersion, schemaVersion)
	}
	if cfg.Agents == nil {
		cfg.Agents = make(map[string]AgentIdentity)
	}
	return cfg, nil
}

// Save persists cfg to <homeDir>/.trackfw/identity.json atomically.
func Save(homeDir string, cfg Config) error {
	cfg.SchemaVersion = schemaVersion
	if cfg.Agents == nil {
		cfg.Agents = make(map[string]AgentIdentity)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("identity: falha ao codificar configuracao: %w", err)
	}
	data = append(data, '\n')

	filename := identityPath(homeDir)
	if err := atomicWrite(filename, data, 0o600); err != nil {
		return fmt.Errorf("identity: falha ao gravar %s: %w", filename, err)
	}
	return nil
}

// atomicWrite writes data to filename atomically: it creates a temporary
// file in the same directory, writes and syncs it, then renames it into
// place. Mirrors the pattern used by internal/integrations/manager.go.
func atomicWrite(filename string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(filename)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".trackfw-tmp-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, filename)
}

// AgentName returns the display name suffixed with "-tf". This is the only
// place in the codebase where the "-tf" suffix is applied to a slug.
func AgentName(slug string) string {
	return slug + "-tf"
}

// Validate checks cfg for structural and referential integrity:
//   - every agent id in cfg.Agents must be present in knownIDs
//   - DisplayName must not be empty
//   - Slug must match ^[a-z0-9]+(-[a-z0-9]+)*$
//   - slugs must be unique across agents
//   - Slug must not end with the "-tf" suffix (it is appended automatically
//     by AgentName)
func Validate(cfg Config, knownIDs []string) error {
	known := make(map[string]bool, len(knownIDs))
	for _, id := range knownIDs {
		known[id] = true
	}

	seenSlugs := make(map[string]string, len(cfg.Agents))

	for id, agent := range cfg.Agents {
		if !known[id] {
			return fmt.Errorf("identity: agente desconhecido %q nao esta na lista de agentes conhecidos", id)
		}
		if agent.DisplayName == "" {
			return fmt.Errorf("identity: display_name vazio para o agente %q", id)
		}
		if !slugPattern.MatchString(agent.Slug) {
			return fmt.Errorf("identity: slug invalido %q para o agente %q (esperado padrao %s)", agent.Slug, id, slugPattern.String())
		}
		if otherID, exists := seenSlugs[agent.Slug]; exists {
			return fmt.Errorf("identity: slug duplicado %q entre os agentes %q e %q", agent.Slug, otherID, id)
		}
		if strings.HasSuffix(agent.Slug, "-tf") {
			return fmt.Errorf("identity: slug %q do agente %q nao deve incluir o sufixo \"-tf\"; ele e acrescentado automaticamente (use %q em vez de %q)", agent.Slug, id, strings.TrimSuffix(agent.Slug, "-tf"), agent.Slug)
		}
		seenSlugs[agent.Slug] = id
	}

	return nil
}

// Lookup returns the identity configured for id, if any.
func Lookup(cfg Config, id string) (AgentIdentity, bool) {
	if cfg.Agents == nil {
		return AgentIdentity{}, false
	}
	agent, ok := cfg.Agents[id]
	return agent, ok
}

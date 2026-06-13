package validator

import (
	"os"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestByAgentNamespacingWIPLimit verifica que o WIP limit é aplicado POR AGENTE,
// não de forma global. Caso discriminante: cada agente tem 3 WIPs com limit=5 —
// total=6 violaria um check global, mas por agente (3 ≤ 5) não deve emitir warning.
func TestByAgentNamespacingWIPLimit(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	yaml := "roadmap_namespacing: by_agent\nwip_limit: 5\nagents:\n  - zeus\n  - apolo\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("escrever trackfw.yaml: %v", err)
	}

	// Zeus: 3 WIPs — abaixo do limite de 5 por agente
	if err := os.MkdirAll("docs/roadmaps/zeus/wip", 0755); err != nil {
		t.Fatalf("mkdir zeus/wip: %v", err)
	}
	for i := 1; i <= 3; i++ {
		_ = os.WriteFile(dir+"/docs/roadmaps/zeus/wip/ROADMAP-zeus-"+string(rune('0'+i))+".md", []byte("# Zeus"), 0644)
	}

	// Apolo: 3 WIPs — abaixo do limite de 5 por agente
	if err := os.MkdirAll("docs/roadmaps/apolo/wip", 0755); err != nil {
		t.Fatalf("mkdir apolo/wip: %v", err)
	}
	for i := 1; i <= 3; i++ {
		_ = os.WriteFile(dir+"/docs/roadmaps/apolo/wip/ROADMAP-apolo-"+string(rune('0'+i))+".md", []byte("# Apolo"), 0644)
	}

	// Total = 6 WIPs — violaria um check global (6 > 5), mas por agente (3 ≤ 5) deve passar sem warning
	_, warnings, err := validateWIPLimit()
	if err != nil {
		t.Fatalf("validateWIPLimit() erro: %v", err)
	}

	for _, w := range warnings {
		t.Errorf("warning inesperado com by_agent (limit=5, cada agente tem 3 WIPs): %q", w)
	}
}

// TestByAgentNamespacingWIPLimitExceeded verifica que warning é emitido quando UM agente
// ultrapassa o limite individualmente em modo by_agent.
func TestByAgentNamespacingWIPLimitExceeded(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	yaml := "roadmap_namespacing: by_agent\nwip_limit: 2\nagents:\n  - zeus\n  - apolo\n"
	if err := os.WriteFile("trackfw.yaml", []byte(yaml), 0644); err != nil {
		t.Fatalf("escrever trackfw.yaml: %v", err)
	}

	// Zeus: 3 WIPs — ultrapassa limite de 2 por agente → deve emitir warning
	if err := os.MkdirAll("docs/roadmaps/zeus/wip", 0755); err != nil {
		t.Fatalf("mkdir zeus/wip: %v", err)
	}
	for i := 1; i <= 3; i++ {
		_ = os.WriteFile(dir+"/docs/roadmaps/zeus/wip/ROADMAP-zeus-"+string(rune('0'+i))+".md", []byte("# Zeus"), 0644)
	}

	// Apolo: 1 WIP — dentro do limite
	if err := os.MkdirAll("docs/roadmaps/apolo/wip", 0755); err != nil {
		t.Fatalf("mkdir apolo/wip: %v", err)
	}
	_ = os.WriteFile(dir+"/docs/roadmaps/apolo/wip/ROADMAP-apolo-1.md", []byte("# Apolo"), 0644)

	_, warnings, err := validateWIPLimit()
	if err != nil {
		t.Fatalf("validateWIPLimit() erro: %v", err)
	}

	if !hasWarning(warnings, "zeus") {
		t.Errorf("esperado warning para agente 'zeus' com 3 WIPs (limit=2), obteve warnings: %v", warnings)
	}

	// Apolo não deve aparecer no warning
	if hasWarning(warnings, "apolo") {
		t.Errorf("warning inesperado para agente 'apolo' com 1 WIP (limit=2): %v", warnings)
	}
}

// TestByAgentNamespacingFlat verifica que sem roadmap_namespacing (ou valor != by_agent),
// o comportamento flat permanece inalterado — WIP limit é verificado globalmente.
func TestByAgentNamespacingFlat(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	// Sem trackfw.yaml → defaults: flat, wip_limit=1
	if err := os.MkdirAll("docs/roadmaps/wip", 0755); err != nil {
		t.Fatalf("mkdir wip: %v", err)
	}

	// 2 WIPs no flat com limit=1 → deve emitir warning
	if err := os.WriteFile(dir+"/docs/roadmaps/wip/ROADMAP-a.md", []byte("# A"), 0644); err != nil {
		t.Fatalf("escrever ROADMAP-a: %v", err)
	}
	if err := os.WriteFile(dir+"/docs/roadmaps/wip/ROADMAP-b.md", []byte("# B"), 0644); err != nil {
		t.Fatalf("escrever ROADMAP-b: %v", err)
	}

	_, warnings, err := validateWIPLimit()
	if err != nil {
		t.Fatalf("validateWIPLimit() erro: %v", err)
	}

	if !hasWarning(warnings, "wip/") && !hasWarning(warnings, "roadmaps in wip") {
		t.Errorf("esperado warning de WIP limit no modo flat (2 WIPs, limit=1), obteve: %v", warnings)
	}
}

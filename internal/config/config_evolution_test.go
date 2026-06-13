package config

import (
	"reflect"
	"testing"
)

func TestConfigDefaults_NewFields(t *testing.T) {
	Reset()
	cfg := defaults()

	if !reflect.DeepEqual(cfg.LinkFieldsReq, []string{"REQ:"}) {
		t.Errorf("LinkFieldsReq: got %v, want [REQ:]", cfg.LinkFieldsReq)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsADR, []string{"ADR:"}) {
		t.Errorf("LinkFieldsADR: got %v, want [ADR:]", cfg.LinkFieldsADR)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsRoadmap, []string{"Roadmap:"}) {
		t.Errorf("LinkFieldsRoadmap: got %v, want [Roadmap:]", cfg.LinkFieldsRoadmap)
	}
	wantMarkers := []string{"## Acceptance Criteria", "## Critérios de Aceite"}
	if !reflect.DeepEqual(cfg.AcceptanceMarkers, wantMarkers) {
		t.Errorf("AcceptanceMarkers: got %v, want %v", cfg.AcceptanceMarkers, wantMarkers)
	}
	if cfg.Rules["wip_has_req"] != "error" {
		t.Errorf("Rules[wip_has_req]: got %q, want error", cfg.Rules["wip_has_req"])
	}
	if cfg.Rules["stale_wip"] != "warning" {
		t.Errorf("Rules[stale_wip]: got %q, want warning", cfg.Rules["stale_wip"])
	}
}

func TestConfigLinkFields(t *testing.T) {
	Reset()
	yaml := `link_fields:
  req:
    - "REQ:"
    - "req_id"
  adr:
    - "ADR:"
  roadmap:
    - "Roadmap:"
`
	cfg := defaults()
	parse(yaml, &cfg)

	if !reflect.DeepEqual(cfg.LinkFieldsReq, []string{"REQ:", "req_id"}) {
		t.Errorf("LinkFieldsReq: got %v, want [REQ: req_id]", cfg.LinkFieldsReq)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsADR, []string{"ADR:"}) {
		t.Errorf("LinkFieldsADR: got %v, want [ADR:]", cfg.LinkFieldsADR)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsRoadmap, []string{"Roadmap:"}) {
		t.Errorf("LinkFieldsRoadmap: got %v, want [Roadmap:]", cfg.LinkFieldsRoadmap)
	}
}

func TestConfigAcceptanceMarkers(t *testing.T) {
	Reset()
	yaml := `acceptance_markers:
  - "## Done"
  - "## Concluído"
`
	cfg := defaults()
	parse(yaml, &cfg)

	want := []string{"## Done", "## Concluído"}
	if !reflect.DeepEqual(cfg.AcceptanceMarkers, want) {
		t.Errorf("AcceptanceMarkers: got %v, want %v", cfg.AcceptanceMarkers, want)
	}
}

func TestConfigRules(t *testing.T) {
	Reset()
	yaml := `rules:
  stale_wip: error
  adr_orphan: off
`
	cfg := defaults()
	parse(yaml, &cfg)

	if cfg.Rules["stale_wip"] != "error" {
		t.Errorf("Rules[stale_wip]: got %q, want error", cfg.Rules["stale_wip"])
	}
	if cfg.Rules["adr_orphan"] != "off" {
		t.Errorf("Rules[adr_orphan]: got %q, want off", cfg.Rules["adr_orphan"])
	}
	// default mantido para chave não sobrescrita
	if cfg.Rules["wip_has_req"] != "error" {
		t.Errorf("Rules[wip_has_req]: got %q, want error (default mantido)", cfg.Rules["wip_has_req"])
	}
}

func TestConfigSparse_NewFields(t *testing.T) {
	Reset()
	yaml := `wip_limit: 3
`
	cfg := defaults()
	parse(yaml, &cfg)

	if cfg.WipLimit != 3 {
		t.Errorf("WipLimit: got %d, want 3", cfg.WipLimit)
	}
	// todos os novos campos devem usar defaults
	if !reflect.DeepEqual(cfg.LinkFieldsReq, []string{"REQ:"}) {
		t.Errorf("LinkFieldsReq: got %v, want default [REQ:]", cfg.LinkFieldsReq)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsADR, []string{"ADR:"}) {
		t.Errorf("LinkFieldsADR: got %v, want default [ADR:]", cfg.LinkFieldsADR)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsRoadmap, []string{"Roadmap:"}) {
		t.Errorf("LinkFieldsRoadmap: got %v, want default [Roadmap:]", cfg.LinkFieldsRoadmap)
	}
	wantMarkers := []string{"## Acceptance Criteria", "## Critérios de Aceite"}
	if !reflect.DeepEqual(cfg.AcceptanceMarkers, wantMarkers) {
		t.Errorf("AcceptanceMarkers: got %v, want default %v", cfg.AcceptanceMarkers, wantMarkers)
	}
	if cfg.Rules["wip_has_req"] != "error" {
		t.Errorf("Rules[wip_has_req]: got %q, want error (default)", cfg.Rules["wip_has_req"])
	}
}

func TestConfigRetrocompat(t *testing.T) {
	Reset()
	// yaml com apenas campos v2.3
	yaml := `adr_dirs:
  - docs/adr
  - docs/adr/zeus
req_dir: docs/requisicoes
roadmap_dir: docs/roadmaps
wip_limit: 2
governance_mode: strict
`
	cfg := defaults()
	parse(yaml, &cfg)

	// campos v2.3 funcionam normalmente
	if !reflect.DeepEqual(cfg.ADRDirs, []string{"docs/adr", "docs/adr/zeus"}) {
		t.Errorf("ADRDirs: got %v", cfg.ADRDirs)
	}
	if cfg.REQDir != "docs/requisicoes" {
		t.Errorf("REQDir: got %q, want docs/requisicoes", cfg.REQDir)
	}
	if cfg.WipLimit != 2 {
		t.Errorf("WipLimit: got %d, want 2", cfg.WipLimit)
	}
	if cfg.GovernanceMode != "strict" {
		t.Errorf("GovernanceMode: got %q, want strict", cfg.GovernanceMode)
	}
	// novos campos devem usar defaults (retrocompatibilidade)
	if !reflect.DeepEqual(cfg.LinkFieldsReq, []string{"REQ:"}) {
		t.Errorf("LinkFieldsReq: got %v, want default [REQ:] (retrocompat)", cfg.LinkFieldsReq)
	}
}

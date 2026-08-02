package config

import (
	"reflect"
	"testing"
)

// Estes testes cobrem o defeito descoberto no ML-1B: uma sequência em bloco YAML pode estar
// no mesmo nível de indentação da chave que a abre — é YAML válido:
//
//	agents:
//	- zeus
//	- apolo
//
// Antes do fix, qualquer linha sem indentação (mesmo "- item") era tratada como top-level e
// fechava a lista aberta, descartando-a silenciosamente. Confirmado com yaml.safe_load real
// (ver roadmap ML-1B).
//
// `rules` é mapeamento (chave: valor), não sequência — o mesmo nível de indentação para
// sub-chaves de mapeamento NÃO é YAML válido (confirmado com yaml.safe_load: as sub-chaves
// viram siblings top-level, não aninhadas). Por isso `rules` fica de fora deste conjunto de
// "listas não indentadas": não há forma não indentada correta a suportar, e o comportamento
// atual (chave ignorada) já é o esperado e compatível com o Python.

func TestLoad_ADRDirs_Unindented(t *testing.T) {
	Reset()
	yaml := "adr_dirs:\n- docs/adr/zeus\n- docs/adr/apolo\n"
	cfg := defaults()
	parse(yaml, &cfg)

	want := []string{"docs/adr/zeus", "docs/adr/apolo"}
	if !reflect.DeepEqual(cfg.ADRDirs, want) {
		t.Errorf("ADRDirs (unindented): got %v, want %v", cfg.ADRDirs, want)
	}
}

func TestLoad_Agents_Unindented(t *testing.T) {
	Reset()
	yaml := "agents:\n- zeus\n- apolo\n"
	cfg := defaults()
	parse(yaml, &cfg)

	want := []string{"zeus", "apolo"}
	if !reflect.DeepEqual(cfg.Agents, want) {
		t.Errorf("Agents (unindented): got %v, want %v", cfg.Agents, want)
	}
}

func TestLoad_AcceptanceMarkers_Unindented(t *testing.T) {
	Reset()
	yaml := "acceptance_markers:\n- \"## Done\"\n- \"## Concluído\"\n"
	cfg := defaults()
	parse(yaml, &cfg)

	want := []string{"## Done", "## Concluído"}
	if !reflect.DeepEqual(cfg.AcceptanceMarkers, want) {
		t.Errorf("AcceptanceMarkers (unindented): got %v, want %v", cfg.AcceptanceMarkers, want)
	}
}

// link_fields: a chave em si (link_fields:) e as sub-chaves (req:/adr:/roadmap:) precisam
// permanecer indentadas — isso é exigido pela especificação YAML (mapeamento aninhado exige
// indentação maior, diferente de sequência). O que pode variar é a indentação dos ITENS da
// sequência em relação à sub-chave que os abre: podem estar mais indentados ou no mesmo nível.
func TestLoad_LinkFields_ItemsSameLevelAsSubKey(t *testing.T) {
	Reset()
	yaml := "link_fields:\n  req:\n  - \"REQ:\"\n  - \"req_id\"\n  adr:\n  - \"ADR:\"\n  roadmap:\n  - \"Roadmap:\"\n"
	cfg := defaults()
	parse(yaml, &cfg)

	if !reflect.DeepEqual(cfg.LinkFieldsReq, []string{"REQ:", "req_id"}) {
		t.Errorf("LinkFieldsReq: got %v", cfg.LinkFieldsReq)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsADR, []string{"ADR:"}) {
		t.Errorf("LinkFieldsADR: got %v", cfg.LinkFieldsADR)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsRoadmap, []string{"Roadmap:"}) {
		t.Errorf("LinkFieldsRoadmap: got %v", cfg.LinkFieldsRoadmap)
	}
}

func TestLoad_LinkFields_ItemsMoreIndentedThanSubKey(t *testing.T) {
	Reset()
	yaml := "link_fields:\n  req:\n    - \"REQ:\"\n  adr:\n    - \"ADR:\"\n  roadmap:\n    - \"Roadmap:\"\n"
	cfg := defaults()
	parse(yaml, &cfg)

	if !reflect.DeepEqual(cfg.LinkFieldsReq, []string{"REQ:"}) {
		t.Errorf("LinkFieldsReq: got %v", cfg.LinkFieldsReq)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsADR, []string{"ADR:"}) {
		t.Errorf("LinkFieldsADR: got %v", cfg.LinkFieldsADR)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsRoadmap, []string{"Roadmap:"}) {
		t.Errorf("LinkFieldsRoadmap: got %v", cfg.LinkFieldsRoadmap)
	}
}

// rules: mapeamento, não sequência. Sub-chaves não indentadas NÃO são YAML válido de forma
// aninhada (viram chaves top-level soltas, ignoradas pelo trackfw). Este teste documenta que
// o comportamento é intencional (defaults preservados) — não uma regressão do fix de listas.
func TestLoad_Rules_UnindentedIsNotNested(t *testing.T) {
	Reset()
	yaml := "rules:\nstale_wip: error\n"
	cfg := defaults()
	parse(yaml, &cfg)

	if cfg.Rules["stale_wip"] != "warning" {
		t.Errorf("Rules[stale_wip]: want default 'warning' (chave não-indentada não é aninhada), got %q", cfg.Rules["stale_wip"])
	}
}

// Forma indentada (regressão) continua funcionando em todas as cinco chaves.
func TestLoad_AllListKeys_IndentedStillWorks(t *testing.T) {
	Reset()
	yaml := `adr_dirs:
  - docs/adr/zeus
  - docs/adr/apolo
agents:
  - zeus
  - apolo
acceptance_markers:
  - "## Done"
link_fields:
  req:
    - "REQ:"
  adr:
    - "ADR:"
  roadmap:
    - "Roadmap:"
rules:
  stale_wip: error
`
	cfg := defaults()
	parse(yaml, &cfg)

	if !reflect.DeepEqual(cfg.ADRDirs, []string{"docs/adr/zeus", "docs/adr/apolo"}) {
		t.Errorf("ADRDirs (indented): got %v", cfg.ADRDirs)
	}
	if !reflect.DeepEqual(cfg.Agents, []string{"zeus", "apolo"}) {
		t.Errorf("Agents (indented): got %v", cfg.Agents)
	}
	if !reflect.DeepEqual(cfg.AcceptanceMarkers, []string{"## Done"}) {
		t.Errorf("AcceptanceMarkers (indented): got %v", cfg.AcceptanceMarkers)
	}
	if !reflect.DeepEqual(cfg.LinkFieldsReq, []string{"REQ:"}) {
		t.Errorf("LinkFieldsReq (indented): got %v", cfg.LinkFieldsReq)
	}
	if cfg.Rules["stale_wip"] != "error" {
		t.Errorf("Rules[stale_wip] (indented): got %q, want error", cfg.Rules["stale_wip"])
	}
}

package config

import (
	"reflect"
	"testing"
)

// Estes testes cobrem a forma flow-style (inline) de lista YAML, ignorada em silêncio antes
// deste fix: "agents: [zeus, apolo]" era tratada como escalar e a chave caía no default.
//
// Contrato (ADR-2026-08-02-suporte-a-lista-yaml-inline-nos-parsers-de-config-dos-tres-clis):
//
//	# | Entrada                                          | Resultado
//	1 | [a, b]                                           | [a, b]
//	2 | [a,b]                                             | [a, b]
//	3 | [ a , b ]                                         | [a, b]
//	4 | ["a", "b"]                                       | [a, b]
//	5 | ['a', 'b']                                       | [a, b]
//	6 | [a]                                                | [a]
//	7 | []                                                 | lista vazia, não default
//	8 | ["a, b", "c"]                                    | dois itens: "a, b" e "c"
//	9 | ["## Acceptance Criteria", "## Critérios de Aceite"] | os dois marcadores

func inlineListCases() []struct {
	name  string
	input string
	want  []string
} {
	return []struct {
		name  string
		input string
		want  []string
	}{
		{"1_espaco_simples", "[a, b]", []string{"a", "b"}},
		{"2_sem_espaco", "[a,b]", []string{"a", "b"}},
		{"3_espacos_extras", "[ a , b ]", []string{"a", "b"}},
		{"4_aspas_duplas", `["a", "b"]`, []string{"a", "b"}},
		{"5_aspas_simples", `['a', 'b']`, []string{"a", "b"}},
		{"6_item_unico", "[a]", []string{"a"}},
		{"7_lista_vazia", "[]", []string{}},
		{"8_virgula_dentro_de_aspas", `["a, b", "c"]`, []string{"a, b", "c"}},
		{"9_marcadores_reais", `["## Acceptance Criteria", "## Critérios de Aceite"]`, []string{"## Acceptance Criteria", "## Critérios de Aceite"}},
	}
}

func TestLoad_ADRDirs_Inline(t *testing.T) {
	for _, tc := range inlineListCases() {
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			yaml := "adr_dirs: " + tc.input + "\n"
			cfg := defaults()
			parse(yaml, &cfg)
			if !reflect.DeepEqual(cfg.ADRDirs, tc.want) {
				t.Errorf("ADRDirs: got %#v, want %#v", cfg.ADRDirs, tc.want)
			}
		})
	}
}

func TestLoad_Agents_Inline(t *testing.T) {
	for _, tc := range inlineListCases() {
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			yaml := "agents: " + tc.input + "\n"
			cfg := defaults()
			parse(yaml, &cfg)
			if !reflect.DeepEqual(cfg.Agents, tc.want) {
				t.Errorf("Agents: got %#v, want %#v", cfg.Agents, tc.want)
			}
		})
	}
}

func TestLoad_AcceptanceMarkers_Inline(t *testing.T) {
	for _, tc := range inlineListCases() {
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			yaml := "acceptance_markers: " + tc.input + "\n"
			cfg := defaults()
			parse(yaml, &cfg)
			if !reflect.DeepEqual(cfg.AcceptanceMarkers, tc.want) {
				t.Errorf("AcceptanceMarkers: got %#v, want %#v", cfg.AcceptanceMarkers, tc.want)
			}
		})
	}
}

func TestLoad_LinkFields_Inline(t *testing.T) {
	for _, tc := range inlineListCases() {
		t.Run("req_"+tc.name, func(t *testing.T) {
			Reset()
			yaml := "link_fields:\n  req: " + tc.input + "\n"
			cfg := defaults()
			parse(yaml, &cfg)
			if !reflect.DeepEqual(cfg.LinkFieldsReq, tc.want) {
				t.Errorf("LinkFieldsReq: got %#v, want %#v", cfg.LinkFieldsReq, tc.want)
			}
		})
		t.Run("adr_"+tc.name, func(t *testing.T) {
			Reset()
			yaml := "link_fields:\n  adr: " + tc.input + "\n"
			cfg := defaults()
			parse(yaml, &cfg)
			if !reflect.DeepEqual(cfg.LinkFieldsADR, tc.want) {
				t.Errorf("LinkFieldsADR: got %#v, want %#v", cfg.LinkFieldsADR, tc.want)
			}
		})
		t.Run("roadmap_"+tc.name, func(t *testing.T) {
			Reset()
			yaml := "link_fields:\n  roadmap: " + tc.input + "\n"
			cfg := defaults()
			parse(yaml, &cfg)
			if !reflect.DeepEqual(cfg.LinkFieldsRoadmap, tc.want) {
				t.Errorf("LinkFieldsRoadmap: got %#v, want %#v", cfg.LinkFieldsRoadmap, tc.want)
			}
		})
	}
}

// ADR-2026-08-02: adr_dirs aplica ExpandPath por item, tanto em bloco quanto inline.
func TestLoad_ADRDirs_Inline_ExpandsTilde(t *testing.T) {
	Reset()
	yaml := "adr_dirs: [~/adr, docs/adr]\n"
	cfg := defaults()
	parse(yaml, &cfg)
	if len(cfg.ADRDirs) != 2 || cfg.ADRDirs[1] != "docs/adr" {
		t.Fatalf("ADRDirs: got %#v", cfg.ADRDirs)
	}
	if cfg.ADRDirs[0] == "~/adr" {
		t.Errorf("ADRDirs[0]: esperava expansão de ~, got %q", cfg.ADRDirs[0])
	}
}

// Forma inline não deve abrir contexto de lista para as linhas seguintes.
func TestLoad_Inline_DoesNotOpenBlockContext(t *testing.T) {
	Reset()
	yaml := "agents: [zeus, apolo]\nreq_dir: docs/req\n"
	cfg := defaults()
	parse(yaml, &cfg)
	if !reflect.DeepEqual(cfg.Agents, []string{"zeus", "apolo"}) {
		t.Errorf("Agents: got %#v", cfg.Agents)
	}
	if cfg.REQDir != "docs/req" {
		t.Errorf("REQDir: got %q, want docs/req (não deve ter sido engolido pelo bloco de agents)", cfg.REQDir)
	}
}

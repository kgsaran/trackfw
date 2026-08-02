package config

import (
	"reflect"
	"testing"
)

// Este arquivo cobre o AC3 (fidelidade textual da normalização de escalares para string na
// fronteira, ADR-2026-08-02-parsing-de-config-por-biblioteca-yaml-com-normalizacao-para-string-
// na-fronteira.md), teste por chave das ~20 chaves suportadas, as formas antes não suportadas
// (mapa inline, lista aninhada inline, âncora) e o comportamento com config ausente/vazio.

// TestAC3_FidelidadeTextual reproduz a tabela do AC3: cada entrada deve chegar ao consumidor
// como o texto literal do YAML, não o valor tipado que a biblioteca produziria.
func TestAC3_FidelidadeTextual(t *testing.T) {
	cases := []struct {
		name  string
		yaml  string
		check func(t *testing.T, cfg *ProjectConfig)
	}{
		{
			name: "lenient_until data nua permanece string YYYY-MM-DD",
			yaml: "lenient_until: 2026-08-02\n",
			check: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.LenientUntil != "2026-08-02" {
					t.Errorf("LenientUntil: got %q, want \"2026-08-02\"", cfg.LenientUntil)
				}
			},
		},
		{
			name: "wip_limit octal 010 vira 10, nao 8",
			yaml: "wip_limit: 010\n",
			check: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.WipLimit != 10 {
					t.Errorf("WipLimit: got %d, want 10 (nao 8)", cfg.WipLimit)
				}
			},
		},
		{
			name: "wip_limit decimal simples",
			yaml: "wip_limit: 3\n",
			check: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.WipLimit != 3 {
					t.Errorf("WipLimit: got %d, want 3", cfg.WipLimit)
				}
			},
		},
		{
			name: "wip_by_squad true",
			yaml: "wip_by_squad: true\n",
			check: func(t *testing.T, cfg *ProjectConfig) {
				if !cfg.WipBySquad {
					t.Error("WipBySquad: want true")
				}
			},
		},
		{
			name: "governance_mode yes permanece string yes (nao bool)",
			yaml: "governance_mode: yes\n",
			check: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.GovernanceMode != "yes" {
					t.Errorf("GovernanceMode: got %q, want \"yes\"", cfg.GovernanceMode)
				}
			},
		},
		{
			name: "governance_mode 1.0 preserva ponto decimal",
			yaml: "governance_mode: 1.0\n",
			check: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.GovernanceMode != "1.0" {
					t.Errorf("GovernanceMode: got %q, want \"1.0\" (nao \"1\")", cfg.GovernanceMode)
				}
			},
		},
		{
			name: "governance_mode null",
			yaml: "governance_mode: null\n",
			check: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.GovernanceMode != "null" {
					t.Errorf("GovernanceMode: got %q, want \"null\"", cfg.GovernanceMode)
				}
			},
		},
		{
			name: "governance_mode ~ (til)",
			yaml: "governance_mode: ~\n",
			check: func(t *testing.T, cfg *ProjectConfig) {
				if cfg.GovernanceMode != "~" {
					t.Errorf("GovernanceMode: got %q, want \"~\"", cfg.GovernanceMode)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := defaults()
			parse(tc.yaml, &cfg)
			tc.check(t, &cfg)
		})
	}
}

// TestAC3_Ancora garante que "b: *x" chegue com o VALOR do anchor, não o nome do anchor —
// risco confirmado no ML-0A: Alias.Value devolveria "x" (o nome) se não resolvido.
func TestAC3_Ancora(t *testing.T) {
	yaml := "governance_mode: &gm strict\nforge: *gm\n"
	cfg := defaults()
	parse(yaml, &cfg)
	if cfg.GovernanceMode != "strict" {
		t.Fatalf("GovernanceMode: got %q, want strict", cfg.GovernanceMode)
	}
	if cfg.Forge != "strict" {
		t.Fatalf("Forge (via ancora *gm): got %q, want strict (o valor do anchor, nao o nome)", cfg.Forge)
	}
}

// TestAC3_AncoraEmLista garante que uma âncora usada dentro de uma lista também resolve.
func TestAC3_AncoraEmLista(t *testing.T) {
	yaml := "agents: [&a zeus, apolo, *a]\n"
	cfg := defaults()
	parse(yaml, &cfg)
	want := []string{"zeus", "apolo", "zeus"}
	if !reflect.DeepEqual(cfg.Agents, want) {
		t.Fatalf("Agents: got %v, want %v", cfg.Agents, want)
	}
}

// TestMapaInline_Rules garante suporte à forma antes não suportada: mapa inline
// ("rules: {stale_wip: error, adr_orphan: warning}").
func TestMapaInline_Rules(t *testing.T) {
	yaml := "rules:\n  stale_wip: error\n"
	cfg := defaults()
	parse(yaml, &cfg)
	if cfg.Rules["stale_wip"] != "error" {
		t.Fatalf("baseline bloco falhou: got %q", cfg.Rules["stale_wip"])
	}

	yamlInline := "rules: {stale_wip: error, adr_orphan: warning}\n"
	cfg2 := defaults()
	parse(yamlInline, &cfg2)
	if cfg2.Rules["stale_wip"] != "error" {
		t.Errorf("Rules[stale_wip] (mapa inline): got %q, want error", cfg2.Rules["stale_wip"])
	}
	if cfg2.Rules["adr_orphan"] != "warning" {
		t.Errorf("Rules[adr_orphan] (mapa inline): got %q, want warning", cfg2.Rules["adr_orphan"])
	}
	// chave não sobrescrita mantém default
	if cfg2.Rules["wip_has_req"] != "error" {
		t.Errorf("Rules[wip_has_req] (default preservado): got %q, want error", cfg2.Rules["wip_has_req"])
	}
}

// TestListaAninhadaInline garante suporte à forma antes não suportada: lista aninhada inline
// dentro de link_fields.req.
func TestListaAninhadaInline_LinkFields(t *testing.T) {
	yaml := "link_fields:\n  req: [\"REQ:\", \"req_id\"]\n"
	cfg := defaults()
	parse(yaml, &cfg)
	want := []string{"REQ:", "req_id"}
	if !reflect.DeepEqual(cfg.LinkFieldsReq, want) {
		t.Fatalf("LinkFieldsReq: got %v, want %v", cfg.LinkFieldsReq, want)
	}
}

// TestConfigAusenteOuVazio_CaiNosDefaults confirma que arquivo vazio ou ausente não gera erro
// e cai nos defaults.
func TestConfigAusenteOuVazio_CaiNosDefaults(t *testing.T) {
	want := defaults()

	cfg := defaults()
	parse("", &cfg)
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("config vazio: got %+v, want defaults %+v", cfg, want)
	}

	cfg2 := defaults()
	parse("# apenas um comentário\n\n", &cfg2)
	if !reflect.DeepEqual(cfg2, want) {
		t.Errorf("config só com comentário: got %+v, want defaults %+v", cfg2, want)
	}
}

// TestConfigMalformado_NaoQuebra confirma que parse() sozinho não entra em pânico com YAML
// malformado e preserva cfg como estava (não faz merge parcial de campos).
//
// ATENÇÃO — isto NÃO é mais o comportamento observável do CLI (ML-1B): Load() agora detecta o
// mesmo erro de sintaxe *antes* de chamar parse() e falha alto (stderr + exit 1) em vez de
// deixar parse() absorver o erro em silêncio — ver TestLoad_Malformed_FailsLoud. Este teste
// cobre só a função interna parse(), que continua sendo a peça reaproveitada por Load() para o
// caminho feliz; o "fallback silencioso" que seu nome descreve é o comportamento de parse(),
// não mais o do CLI ponta-a-ponta.
//
// A fixture abaixo (colchete de flow sequence sem fechar, atravessando linha) foi confirmada
// como erro real de parse nos 3 (não vacua sob nenhum): yaml.v3 devolve
// "did not find expected ',' or ']'"; PyYAML levanta YAMLError equivalente; a lib `yaml` do
// Node popula doc.errors (não lança, mas o array não fica vazio).
func TestConfigMalformado_NaoQuebra(t *testing.T) {
	cfg := defaults()
	parse("agents: [zeus, apolo\nwip_limit: 3\n", &cfg)
	// não deve ter panicado; parse() sozinho preserva o estado anterior (defaults) — quem
	// transforma isso em erro fatal é Load(), um nível acima.
	if cfg.WipLimit != 1 {
		t.Errorf("WipLimit apos YAML malformado: got %d, want default 1 (parse() nao muta cfg)", cfg.WipLimit)
	}
}

// TestPorChave_AsVinteChaves testa cada uma das ~20 chaves suportadas isoladamente.
func TestPorChave_AsVinteChaves(t *testing.T) {
	t.Run("adr_dirs", func(t *testing.T) {
		cfg := defaults()
		parse("adr_dirs:\n  - docs/adr/x\n", &cfg)
		if !reflect.DeepEqual(cfg.ADRDirs, []string{"docs/adr/x"}) {
			t.Errorf("got %v", cfg.ADRDirs)
		}
	})
	t.Run("agents", func(t *testing.T) {
		cfg := defaults()
		parse("agents:\n  - zeus\n", &cfg)
		if !reflect.DeepEqual(cfg.Agents, []string{"zeus"}) {
			t.Errorf("got %v", cfg.Agents)
		}
	})
	t.Run("req_dir", func(t *testing.T) {
		cfg := defaults()
		parse("req_dir: docs/req2\n", &cfg)
		if cfg.REQDir != "docs/req2" {
			t.Errorf("got %q", cfg.REQDir)
		}
	})
	t.Run("roadmap_dir", func(t *testing.T) {
		cfg := defaults()
		parse("roadmap_dir: docs/rm2\n", &cfg)
		if cfg.RoadmapDir != "docs/rm2" {
			t.Errorf("got %q", cfg.RoadmapDir)
		}
	})
	t.Run("roadmap_namespacing", func(t *testing.T) {
		cfg := defaults()
		parse("roadmap_namespacing: by_agent\n", &cfg)
		if cfg.RoadmapNamespacing != "by_agent" {
			t.Errorf("got %q", cfg.RoadmapNamespacing)
		}
	})
	t.Run("acceptance_markers", func(t *testing.T) {
		cfg := defaults()
		parse("acceptance_markers:\n  - \"## Done\"\n", &cfg)
		if !reflect.DeepEqual(cfg.AcceptanceMarkers, []string{"## Done"}) {
			t.Errorf("got %v", cfg.AcceptanceMarkers)
		}
	})
	t.Run("link_fields.req", func(t *testing.T) {
		cfg := defaults()
		parse("link_fields:\n  req:\n    - \"req_id\"\n", &cfg)
		if !reflect.DeepEqual(cfg.LinkFieldsReq, []string{"req_id"}) {
			t.Errorf("got %v", cfg.LinkFieldsReq)
		}
	})
	t.Run("link_fields.adr", func(t *testing.T) {
		cfg := defaults()
		parse("link_fields:\n  adr:\n    - \"adr_id\"\n", &cfg)
		if !reflect.DeepEqual(cfg.LinkFieldsADR, []string{"adr_id"}) {
			t.Errorf("got %v", cfg.LinkFieldsADR)
		}
	})
	t.Run("link_fields.roadmap", func(t *testing.T) {
		cfg := defaults()
		parse("link_fields:\n  roadmap:\n    - \"rm_id\"\n", &cfg)
		if !reflect.DeepEqual(cfg.LinkFieldsRoadmap, []string{"rm_id"}) {
			t.Errorf("got %v", cfg.LinkFieldsRoadmap)
		}
	})
	t.Run("rules", func(t *testing.T) {
		cfg := defaults()
		parse("rules:\n  stale_wip: error\n", &cfg)
		if cfg.Rules["stale_wip"] != "error" {
			t.Errorf("got %q", cfg.Rules["stale_wip"])
		}
	})
	t.Run("wip_limit", func(t *testing.T) {
		cfg := defaults()
		parse("wip_limit: 4\n", &cfg)
		if cfg.WipLimit != 4 {
			t.Errorf("got %d", cfg.WipLimit)
		}
	})
	t.Run("wip_by_squad", func(t *testing.T) {
		cfg := defaults()
		parse("wip_by_squad: true\n", &cfg)
		if !cfg.WipBySquad {
			t.Error("want true")
		}
	})
	t.Run("stale_wip_days", func(t *testing.T) {
		cfg := defaults()
		parse("stale_wip_days: 14\n", &cfg)
		if cfg.StaleWIPDays != 14 {
			t.Errorf("got %d", cfg.StaleWIPDays)
		}
	})
	t.Run("lenient_until", func(t *testing.T) {
		cfg := defaults()
		parse("lenient_until: 2026-09-01\n", &cfg)
		if cfg.LenientUntil != "2026-09-01" {
			t.Errorf("got %q", cfg.LenientUntil)
		}
	})
	t.Run("governance_mode", func(t *testing.T) {
		cfg := defaults()
		parse("governance_mode: strict\n", &cfg)
		if cfg.GovernanceMode != "strict" {
			t.Errorf("got %q", cfg.GovernanceMode)
		}
	})
	t.Run("require_req_in_commit", func(t *testing.T) {
		cfg := defaults()
		parse("require_req_in_commit: true\n", &cfg)
		if !cfg.RequireReqInCommit {
			t.Error("want true")
		}
	})
	t.Run("strict_ci_paths", func(t *testing.T) {
		cfg := defaults()
		parse("strict_ci_paths: true\n", &cfg)
		if !cfg.StrictCIPaths {
			t.Error("want true")
		}
	})
	t.Run("trace_id_field", func(t *testing.T) {
		cfg := defaults()
		parse("trace_id_field: req_id\n", &cfg)
		if cfg.TraceIdField != "req_id" {
			t.Errorf("got %q", cfg.TraceIdField)
		}
	})
	t.Run("forge", func(t *testing.T) {
		cfg := defaults()
		parse("forge: gitlab\n", &cfg)
		if cfg.Forge != "gitlab" {
			t.Errorf("got %q", cfg.Forge)
		}
	})
	t.Run("squad (chave nao consumida por ProjectConfig, nao deve quebrar)", func(t *testing.T) {
		cfg := defaults()
		parse("squad: platform\nreq_dir: docs/req\n", &cfg)
		if cfg.REQDir != "docs/req" {
			t.Errorf("got %q", cfg.REQDir)
		}
	})
}

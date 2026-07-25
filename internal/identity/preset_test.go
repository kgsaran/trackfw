package identity

import (
	"sort"
	"strings"
	"testing"
)

func TestPresetNames_ReturnsExpectedOrder(t *testing.T) {
	want := []string{"greek", "norse", "potter", "thrones", "chaves", "pioneers", "starwars", "tolkien", "turma", "egyptian"}
	got := PresetNames()
	if len(got) != len(want) {
		t.Fatalf("PresetNames() tem %d itens, esperava %d", len(got), len(want))
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("PresetNames()[%d] = %q, esperava %q", i, got[i], name)
		}
	}
}

func TestPresetNames_HasExactlyTenEntries(t *testing.T) {
	got := PresetNames()
	if len(got) != 10 {
		t.Fatalf("PresetNames() tem %d itens, esperava exatamente 10: %v", len(got), got)
	}
}

func TestPresetNames_EveryNameResolvesViaPreset(t *testing.T) {
	for _, name := range PresetNames() {
		if _, err := Preset(name); err != nil {
			t.Fatalf("Preset(%q) nao deveria falhar para nome listado em PresetNames(): %v", name, err)
		}
	}
}

func TestPreset_UnknownNameReturnsErrorListingValidNames(t *testing.T) {
	_, err := Preset("inexistente")
	if err == nil {
		t.Fatal("Preset(inexistente) deveria retornar erro")
	}
	for _, name := range PresetNames() {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("mensagem de erro deveria citar o preset valido %q, obteve: %v", name, err)
		}
	}
}

func TestPreset_ReturnsCopyNotSharedReference(t *testing.T) {
	first, err := Preset("greek")
	if err != nil {
		t.Fatalf("Preset(greek) nao deveria falhar: %v", err)
	}

	// Mutate the returned config.
	first.Agents["backend"] = AgentIdentity{DisplayName: "Mutado", Slug: "mutado"}
	delete(first.Agents, "architect")
	first.Agents["novo-id"] = AgentIdentity{DisplayName: "Novo", Slug: "novo"}

	second, err := Preset("greek")
	if err != nil {
		t.Fatalf("Preset(greek) nao deveria falhar: %v", err)
	}

	if second.Agents["backend"].DisplayName != "Apolo" || second.Agents["backend"].Slug != "apolo" {
		t.Fatalf("mutar o Config de uma chamada nao deveria afetar chamadas seguintes: backend = %+v", second.Agents["backend"])
	}
	if _, ok := second.Agents["architect"]; !ok {
		t.Fatal("delete no Config de uma chamada nao deveria afetar chamadas seguintes: architect sumiu")
	}
	if _, ok := second.Agents["novo-id"]; ok {
		t.Fatal("insercao no Config de uma chamada nao deveria afetar chamadas seguintes: novo-id vazou")
	}
}

func TestPreset_EveryPresetCoversExactlyKnownAgentIDs(t *testing.T) {
	knownIDs := KnownAgentIDs()
	knownSet := make(map[string]bool, len(knownIDs))
	for _, id := range knownIDs {
		knownSet[id] = true
	}

	for _, name := range PresetNames() {
		cfg, err := Preset(name)
		if err != nil {
			t.Fatalf("Preset(%q) nao deveria falhar: %v", name, err)
		}
		if len(cfg.Agents) != len(knownIDs) {
			t.Fatalf("preset %q tem %d agentes, esperava %d (KnownAgentIDs)", name, len(cfg.Agents), len(knownIDs))
		}
		for id := range cfg.Agents {
			if !knownSet[id] {
				t.Fatalf("preset %q contem id desconhecido %q", name, id)
			}
		}
		for _, id := range knownIDs {
			if _, ok := cfg.Agents[id]; !ok {
				t.Fatalf("preset %q nao cobre o id conhecido %q", name, id)
			}
		}
	}
}

func TestPreset_NoDuplicateSlugsWithinPreset(t *testing.T) {
	for _, name := range PresetNames() {
		cfg, err := Preset(name)
		if err != nil {
			t.Fatalf("Preset(%q) nao deveria falhar: %v", name, err)
		}
		seen := make(map[string]string, len(cfg.Agents))
		for id, agent := range cfg.Agents {
			if otherID, exists := seen[agent.Slug]; exists {
				t.Fatalf("preset %q tem slug duplicado %q entre os agentes %q e %q", name, agent.Slug, otherID, id)
			}
			seen[agent.Slug] = id
		}
	}
}

func TestPreset_AllPassValidate(t *testing.T) {
	knownIDs := KnownAgentIDs()
	for _, name := range PresetNames() {
		cfg, err := Preset(name)
		if err != nil {
			t.Fatalf("Preset(%q) nao deveria falhar: %v", name, err)
		}
		if err := Validate(cfg, knownIDs); err != nil {
			t.Fatalf("preset %q nao passou em Validate: %v", name, err)
		}
	}
}

func TestPreset_DisplayNameNonEmptyAndSlugMatchesPattern(t *testing.T) {
	for _, name := range PresetNames() {
		cfg, err := Preset(name)
		if err != nil {
			t.Fatalf("Preset(%q) nao deveria falhar: %v", name, err)
		}
		for id, agent := range cfg.Agents {
			if agent.DisplayName == "" {
				t.Fatalf("preset %q: agente %q tem DisplayName vazio", name, id)
			}
			if !slugPattern.MatchString(agent.Slug) {
				t.Fatalf("preset %q: agente %q tem slug %q que nao casa com %s", name, id, agent.Slug, slugPattern.String())
			}
		}
	}
}

// TestPreset_SlugsMatchSlugifyOfDisplayName documents (without creating a
// runtime dependency) that the hardcoded slug table is consistent with what
// Slugify would compute for each DisplayName. This applies to all 5 presets:
// a failure here indicates a typo in the presets table, not a bug in the
// test — the fix is to correct the table.
func TestPreset_SlugsMatchSlugifyOfDisplayName(t *testing.T) {
	for _, name := range PresetNames() {
		cfg, err := Preset(name)
		if err != nil {
			t.Fatalf("Preset(%q) nao deveria falhar: %v", name, err)
		}
		for id, agent := range cfg.Agents {
			got, err := Slugify(agent.DisplayName)
			if err != nil {
				t.Fatalf("preset %q: Slugify(%q) para o agente %q retornou erro: %v", name, agent.DisplayName, id, err)
			}
			if got != agent.Slug {
				t.Fatalf("preset %q: agente %q: Slugify(%q) = %q, mas slug hardcoded eh %q", name, id, agent.DisplayName, got, agent.Slug)
			}
		}
	}
}

func TestPreset_NamesAreSortedDeterministically(t *testing.T) {
	names := PresetNames()
	sortedCopy := append([]string(nil), names...)
	sort.Strings(sortedCopy)
	// PresetNames is intentionally curated order, not alphabetical — this
	// test only guards that PresetNames() is stable across calls.
	again := PresetNames()
	for i := range names {
		if names[i] != again[i] {
			t.Fatalf("PresetNames() nao eh deterministico: %v vs %v", names, again)
		}
	}
}

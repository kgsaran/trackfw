package generators

import "testing"

func TestDetectDomains_Authentication(t *testing.T) {
	probes := DetectDomains("tela de login para a aplicação")
	found := false
	for _, p := range probes {
		if p.Domain == "authentication" {
			found = true
		}
	}
	if !found {
		t.Error("esperava probe de authentication para 'login'")
	}
}

func TestDetectDomains_UI(t *testing.T) {
	probes := DetectDomains("criar tela de dashboard")
	found := false
	for _, p := range probes {
		if p.Domain == "ui" {
			found = true
		}
	}
	if !found {
		t.Error("esperava probe de ui para 'tela'")
	}
}

func TestDetectDomains_NoMatch(t *testing.T) {
	probes := DetectDomains("refatorar função de cálculo")
	if len(probes) != 0 {
		t.Errorf("esperava 0 probes, obteve %d", len(probes))
	}
}

func TestDetectDomains_MultiDomain(t *testing.T) {
	probes := DetectDomains("tela de login com banco de dados")
	domains := map[string]bool{}
	for _, p := range probes {
		domains[p.Domain] = true
	}
	if !domains["authentication"] {
		t.Error("esperava authentication")
	}
	if !domains["ui"] {
		t.Error("esperava ui")
	}
	if !domains["persistence"] {
		t.Error("esperava persistence")
	}
}

func TestDetectDomains_CaseInsensitive(t *testing.T) {
	probes := DetectDomains("Tela de Login")
	if len(probes) == 0 {
		t.Error("esperava probes para 'Tela de Login' (case-insensitive)")
	}
}

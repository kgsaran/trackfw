package commands

import "testing"

// TestRoadmapNewCmdExposesParityFlags afirma que roadmap new expõe os flags de paridade
// com os outros CLIs — title, req, from-req e agent (AC12: flag existe nos 3 CLIs).
func TestRoadmapNewCmdExposesParityFlags(t *testing.T) {
	cmd := newRoadmapNewCmd()

	for _, flag := range []string{"title", "req", "from-req", "agent"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Fatalf("roadmap new should expose --%s", flag)
		}
	}
}

// TestReqNewCmdExposesAgentFlag afirma que req new expõe --agent (AC12: flag existe nos 3 CLIs).
func TestReqNewCmdExposesAgentFlag(t *testing.T) {
	cmd := newReqNewCmd()

	if cmd.Flags().Lookup("agent") == nil {
		t.Fatal("req new should expose --agent")
	}
}

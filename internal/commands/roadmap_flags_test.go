package commands

import "testing"

func TestRoadmapNewCmdExposesParityFlags(t *testing.T) {
	cmd := newRoadmapNewCmd()

	for _, flag := range []string{"title", "req", "from-req"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Fatalf("roadmap new should expose --%s", flag)
		}
	}
}

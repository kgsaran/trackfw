package validator

import (
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
)

// TestIsMultiAgentByAgent_fourCases asserts the four discrimination cases for
// IsMultiAgentByAgent / ReqNewLine / RoadmapNewLine.
//
// Reconciliation:
//   case byAgent2plus: IsMultiAgentByAgent with by_agent + ["alpha","beta"] returns
//     (true, ["alpha","beta"]), affirming that 2 non-empty agents triggers the --agent form.
//   case byAgentSingle: IsMultiAgentByAgent with by_agent + ["alpha"] returns
//     (false, ...), affirming the contra-braço: a single-agent by_agent project
//     does NOT see the --agent variant.
//   case flat: IsMultiAgentByAgent with no namespacing returns (false, nil),
//     affirming the contra-braço: flat projects do NOT see the --agent variant.
//   case byAgent2plusWithEmpty: IsMultiAgentByAgent with by_agent + ["","alpha","beta"]
//     returns (true, ["alpha","beta"]), affirming that the empty-string filter works
//     and does not count empty entries toward the 2-agent threshold.
func TestIsMultiAgentByAgent_fourCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		cfg         config.ProjectConfig
		wantMulti   bool
		wantAgents  []string
		wantReqLine string
		wantRMLine  string
	}{
		{
			name: "byAgent2plus",
			cfg: config.ProjectConfig{
				RoadmapNamespacing: config.NamespacingByAgent,
				Agents:             []string{"alpha", "beta"},
			},
			wantMulti:   true,
			wantAgents:  []string{"alpha", "beta"},
			wantReqLine: `trackfw req new --agent <agent> "title"  # agents: alpha, beta`,
			wantRMLine:  `trackfw roadmap new --agent <agent> "title"`,
		},
		{
			name: "byAgentSingle",
			cfg: config.ProjectConfig{
				RoadmapNamespacing: config.NamespacingByAgent,
				Agents:             []string{"alpha"},
			},
			wantMulti:   false,
			wantReqLine: `trackfw req new "title"`,
			wantRMLine:  `trackfw roadmap new "title"`,
		},
		{
			name:        "flat",
			cfg:         config.ProjectConfig{},
			wantMulti:   false,
			wantReqLine: `trackfw req new "title"`,
			wantRMLine:  `trackfw roadmap new "title"`,
		},
		{
			name: "byAgent2plusWithEmpty",
			cfg: config.ProjectConfig{
				RoadmapNamespacing: config.NamespacingByAgent,
				Agents:             []string{"", "alpha", "beta"},
			},
			wantMulti:   true,
			wantAgents:  []string{"alpha", "beta"},
			wantReqLine: `trackfw req new --agent <agent> "title"  # agents: alpha, beta`,
			wantRMLine:  `trackfw roadmap new --agent <agent> "title"`,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotMulti, gotAgents := IsMultiAgentByAgent(tc.cfg)
			if gotMulti != tc.wantMulti {
				t.Errorf("IsMultiAgentByAgent(%q): got multi=%v, want %v",
					tc.name, gotMulti, tc.wantMulti)
			}
			if tc.wantMulti {
				for i, a := range tc.wantAgents {
					if i >= len(gotAgents) || gotAgents[i] != a {
						t.Errorf("IsMultiAgentByAgent(%q): agents[%d] got %q, want %q",
							tc.name, i, func() string {
								if i < len(gotAgents) {
									return gotAgents[i]
								}
								return "<missing>"
							}(), a)
					}
				}
			}

			gotReq := ReqNewLine(tc.cfg)
			if gotReq != tc.wantReqLine {
				t.Errorf("ReqNewLine(%q):\n  got  %q\n  want %q", tc.name, gotReq, tc.wantReqLine)
			}

			gotRM := RoadmapNewLine(tc.cfg)
			if gotRM != tc.wantRMLine {
				t.Errorf("RoadmapNewLine(%q):\n  got  %q\n  want %q", tc.name, gotRM, tc.wantRMLine)
			}
		})
	}
}

// TestBranchGovernanceOrientation_byAgent2plus asserts that BranchGovernanceOrientation
// embeds the --agent form when the project is by_agent with 2+ agents.
//
// Reconciliation: this test affirms that the governance orientation message surfaced
// to users of feat/fix/refactor branches correctly teaches `--agent` when applicable,
// i.e. the callsite in validateBranchHasWIPRoadmap passes the real cfg.
func TestBranchGovernanceOrientation_byAgent2plus(t *testing.T) {
	t.Parallel()
	cfg := config.ProjectConfig{
		RoadmapNamespacing: config.NamespacingByAgent,
		Agents:             []string{"zeus", "apolo"},
	}
	msg := BranchGovernanceOrientation("feat/my-feature", cfg)

	wantReq := `trackfw req new --agent <agent> "title"  # agents: zeus, apolo`
	wantRM := `trackfw roadmap new --agent <agent> "title"`

	if !strings.Contains(msg, wantReq) {
		t.Errorf("BranchGovernanceOrientation (by_agent 2+): missing req line\n  want substring: %q\n  got:\n%s",
			wantReq, msg)
	}
	if !strings.Contains(msg, wantRM) {
		t.Errorf("BranchGovernanceOrientation (by_agent 2+): missing roadmap line\n  want substring: %q\n  got:\n%s",
			wantRM, msg)
	}
}

// TestBranchGovernanceOrientation_flat asserts that BranchGovernanceOrientation
// uses the flat form for projects without by_agent namespacing.
//
// Reconciliation: contra-braço — flat projects must NOT see --agent in the orientation.
func TestBranchGovernanceOrientation_flat(t *testing.T) {
	t.Parallel()
	msg := BranchGovernanceOrientation("feat/my-feature", config.ProjectConfig{})

	wantReq := `trackfw req new "title"`
	wantRM := `trackfw roadmap new "title"`

	if !strings.Contains(msg, wantReq) {
		t.Errorf("BranchGovernanceOrientation (flat): missing req line\n  want substring: %q\n  got:\n%s",
			wantReq, msg)
	}
	if !strings.Contains(msg, wantRM) {
		t.Errorf("BranchGovernanceOrientation (flat): missing roadmap line\n  want substring: %q\n  got:\n%s",
			wantRM, msg)
	}
	if strings.Contains(msg, "--agent") {
		t.Errorf("BranchGovernanceOrientation (flat): must not contain --agent; got:\n%s", msg)
	}
}

package commands

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type configKeyDoc struct {
	Key         string
	Type        string
	Default     string
	Description string
	Example     string
	Impact      string
}

var configDocs = []configKeyDoc{
	{
		Key:         "adr_dirs",
		Type:        "list of strings",
		Default:     `["docs/adr"]`,
		Description: "Directories where ADRs are stored.",
		Example:     `adr_dirs:
  - docs/adr
  - docs/adr/zeus`,
		Impact: "Changing adds or removes directories tracked in ADR validations.",
	},
	{
		Key:         "req_dir",
		Type:        "string",
		Default:     `"docs/req"`,
		Description: "Directory where REQs are stored.",
		Example:     `req_dir: docs/requirements`,
		Impact:      "Changing affects where the gate looks for REQs for validation.",
	},
	{
		Key:         "roadmap_dir",
		Type:        "string",
		Default:     `"docs/roadmaps"`,
		Description: "Root directory for roadmaps.",
		Example:     `roadmap_dir: docs/roadmaps`,
		Impact:      "Changing affects where the gate looks for roadmaps in backlog/wip/blocked/done.",
	},
	{
		Key:         "roadmap_namespacing",
		Type:        "flat|by_agent",
		Default:     `"flat"`,
		Description: "Namespacing strategy for roadmaps.",
		Example:     `roadmap_namespacing: by_agent`,
		Impact:      "by_agent creates subdirectories per agent; flat uses a single directory per state.",
	},
	{
		Key:         "agents",
		Type:        "list of strings",
		Default:     `[]`,
		Description: "List of active agents in the project.",
		Example: `agents:
  - apolo
  - afrodite`,
		Impact: "Registered agents get their own subdirectories in by_agent mode.",
	},
	{
		Key:         "governance_mode",
		Type:        "string",
		Default:     `""`,
		Description: "Governance mode (strict, lenient).",
		Example:     `governance_mode: lenient`,
		Impact:      "lenient converts violations to warnings; strict (default) blocks with exit 1.",
	},
	{
		Key:         "lenient_until",
		Type:        "date (YYYY-MM-DD)",
		Default:     `""`,
		Description: "Mandatory deadline for lenient mode. Absence or a date more than 730 days in the future reverts to strict (horizon ceiling: no lenient window may exceed ~2 years without explicit renewal).",
		Example:     `lenient_until: 2027-03-31`,
		Impact:      "No deadline = strict. Past the date = strict. Carve-out rules (req_roadmap_lifecycle, ref_targets_exist) are never silenced by lenient mode.",
	},
	{
		Key:         "wip_limit",
		Type:        "integer",
		Default:     `1`,
		Description: "Limit on simultaneous WIP items.",
		Example:     `wip_limit: 3`,
		Impact:      "Increasing reduces blocking frequency; decreasing requires more discipline.",
	},
	{
		Key:         "wip_by_squad",
		Type:        "boolean",
		Default:     `false`,
		Description: "Apply WIP limit per squad individually.",
		Example:     `wip_by_squad: true`,
		Impact:      "true applies the limit per squad; false applies to the total project.",
	},
	{
		Key:         "require_req_in_commit",
		Type:        "boolean",
		Default:     `false`,
		Description: "Require REQ reference in commit messages.",
		Example:     `require_req_in_commit: true`,
		Impact:      "true installs a commit-msg hook that blocks commits without a REQ reference.",
	},
	{
		Key:         "link_fields.req",
		Type:        "list of strings",
		Default:     `["REQ:"]`,
		Description: "Markers identifying links to a REQ.",
		Example: `link_fields:
  req:
    - "REQ:"
    - "req_id:"`,
		Impact: "Changing affects which tokens the gate recognizes as REQ links.",
	},
	{
		Key:         "link_fields.adr",
		Type:        "list of strings",
		Default:     `["ADR:"]`,
		Description: "Markers identifying links to an ADR.",
		Example: `link_fields:
  adr:
    - "ADR:"
    - "adr_id:"`,
		Impact: "Changing affects which tokens the gate recognizes as ADR links.",
	},
	{
		Key:         "link_fields.roadmap",
		Type:        "list of strings",
		Default:     `["Roadmap:"]`,
		Description: "Markers identifying links to a Roadmap.",
		Example: `link_fields:
  roadmap:
    - "Roadmap:"
    - "roadmap_id:"`,
		Impact: "Changing affects which tokens the gate recognizes as Roadmap links.",
	},
	{
		Key:         "acceptance_markers",
		Type:        "list of strings",
		Default:     `["## Acceptance Criteria", "## Critérios de Aceite"]`,
		Description: "Acceptance criteria markers in WIP documents.",
		Example: `acceptance_markers:
  - "## Acceptance Criteria"
  - "## Critérios de Aceite"
  - "## AC"`,
		Impact: "Changing allows customizing the section heading the gate validates.",
	},
	{
		Key:         "rules.wip_has_req",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Severity: WIP without a linked REQ.",
		Example:     `rules:\n  wip_has_req: warning`,
		Impact:      "error blocks the gate; warning displays but does not block; off ignores.",
	},
	{
		Key:         "rules.wip_acceptance",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Severity: WIP without acceptance criteria.",
		Example:     `rules:\n  wip_acceptance: warning`,
		Impact:      "error blocks the gate; warning displays but does not block; off ignores.",
	},
	{
		Key:         "rules.wip_limit",
		Type:        "off|warning|error",
		Default:     `"warning"`,
		Description: "Severity: excess WIP items. Default warning: existing projects with more than one roadmap in wip/ are not blocked by default. Use error to block the gate.",
		Example:     `rules:\n  wip_limit: error`,
		Impact:      "error blocks the gate; warning displays but does not block; off ignores.",
	},
	{
		Key:         "rules.adr_req_has_content",
		Type:        "off|warning|error",
		Default:     `"warning"`,
		Description: "Severity: ADR or REQ whose required sections contain only template content (HTML comments, blank checkboxes, spaces). Checks ## Context/Decision/Consequences/Alternatives Considered (ADR) and ## Motivation/Acceptance Criteria (REQ). Optional sections never trigger.",
		Example:     `rules:\n  adr_req_has_content: off`,
		Impact:      "error blocks the gate; warning displays but does not block; off ignores.",
	},
	{
		Key:         "rules.stale_wip",
		Type:        "off|warning|error",
		Default:     `"warning"`,
		Description: "Severity: WIP without recent updates.",
		Example:     `rules:\n  stale_wip: error`,
		Impact:      "Increasing severity forces review of stalled roadmaps.",
	},
	{
		Key:         "rules.adr_orphan",
		Type:        "off|warning|error",
		Default:     `"warning"`,
		Description: "Severity: ADR without a linked REQ.",
		Example:     `rules:\n  adr_orphan: error`,
		Impact:      "error requires every ADR to be referenced by at least one REQ.",
	},
	{
		Key:         "rules.ref_targets_exist",
		Type:        "off|warning|error",
		Default:     `"warning"`,
		Description: "Severity: references with non-existent targets.",
		Example:     `rules:\n  ref_targets_exist: error`,
		Impact:      "error blocks when referenced REQs or ADRs do not exist in the repository.",
	},
	{
		Key:         "rules.folder_status",
		Type:        "off|warning|error",
		Default:     `"warning"`,
		Description: "Severity: consistency between folder and file status.",
		Example:     `rules:\n  folder_status: error`,
		Impact:      "error blocks when the frontmatter status does not match the folder (e.g. done/ with status Open).",
	},
	{
		Key:         "rules.filename_uniqueness",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Severity: duplicate file names.",
		Example:     `rules:\n  filename_uniqueness: warning`,
		Impact:      "error blocks when two artifacts share the same basename regardless of folder.",
	},
	{
		Key:         "rules.blocked_by_draft_adr",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Severity: REQ blocked by a draft ADR.",
		Example:     `rules:\n  blocked_by_draft_adr: warning`,
		Impact:      "error blocks when an Open REQ references an ADR with Status: Draft.",
	},
	{
		Key:         "trace_id_field",
		Type:        "string",
		Default:     `""`,
		Description: "Frontmatter field used as a stable traceability ID between REQ and Roadmap. Empty = disabled.",
		Example:     `trace_id_field: req_id`,
		Impact:      "Enables bidirectional REQ↔Roadmap verification (traceid_orphan_*, traceid_state_mismatch, traceid_duplicate_*).",
	},
	{
		Key:         "rules.traceid_orphan_roadmap",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Roadmap with req_id without a matching REQ.",
		Example:     "rules:\n  traceid_orphan_roadmap: warning",
		Impact:      "Detects Roadmaps without a paired REQ.",
	},
	{
		Key:         "rules.traceid_orphan_req",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "REQ with req_id without a matching Roadmap.",
		Example:     "rules:\n  traceid_orphan_req: warning",
		Impact:      "Detects REQs without a paired Roadmap.",
	},
	{
		Key:         "rules.traceid_state_mismatch",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "REQ and Roadmap with the same req_id in divergent states (e.g. done/wip).",
		Example:     "rules:\n  traceid_state_mismatch: warning",
		Impact:      "Ensures state consistency between REQ and Roadmap.",
	},
	{
		Key:         "rules.traceid_duplicate_req",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Same req_id in more than one REQ.",
		Example:     "rules:\n  traceid_duplicate_req: warning",
		Impact:      "Ensures logical uniqueness of REQs.",
	},
	{
		Key:         "rules.traceid_duplicate_roadmap",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Same req_id in more than one Roadmap.",
		Example:     "rules:\n  traceid_duplicate_roadmap: warning",
		Impact:      "Ensures logical uniqueness of Roadmaps.",
	},
}

// printCommandList escreve a lista de comandos disponíveis de root em out.
func printCommandList(out io.Writer, root *cobra.Command) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Available commands:")
	for _, sub := range root.Commands() {
		if !sub.IsAvailableCommand() {
			continue
		}
		fmt.Fprintf(w, "  %s\t%s\n", sub.Name(), sub.Short)
	}
	w.Flush()
}

// printConfigKeyList escreve a tabela KEY/DEFAULT/DESCRIÇÃO em out.
func printConfigKeyList(out io.Writer) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Configuration keys (trackfw.yaml):")
	fmt.Fprintln(w, "KEY\tDEFAULT\tDESCRIPTION")
	fmt.Fprintln(w, strings.Repeat("─", 80))
	for _, d := range configDocs {
		fmt.Fprintf(w, "%s\t%s\t%s\n", d.Key, d.Default, d.Description)
	}
	w.Flush()
}

// printConfigKeyDoc escreve a documentação completa de uma chave em out.
func printConfigKeyDoc(out io.Writer, d configKeyDoc) {
	fmt.Fprintf(out, "%s\n", d.Key)
	fmt.Fprintf(out, "  Type:    %s\n", d.Type)
	fmt.Fprintf(out, "  Default: %s\n", d.Default)
	fmt.Fprintf(out, "  Desc:    %s\n", d.Description)
	fmt.Fprintf(out, "  Example:\n")
	for _, line := range strings.Split(d.Example, "\n") {
		fmt.Fprintf(out, "    %s\n", line)
	}
	fmt.Fprintf(out, "  Impact:  %s\n", d.Impact)
}

// levenshtein calcula a distância de edição entre a e b.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			min := curr[j-1] + 1
			if prev[j]+1 < min {
				min = prev[j] + 1
			}
			if prev[j-1]+cost < min {
				min = prev[j-1] + cost
			}
			curr[j] = min
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

// suggestTopic retorna o comando ou chave de configuração conhecida mais
// próxima de topic (distância de edição <= 3), ou "" se nenhuma for próxima
// o suficiente. Usado para compor a mensagem de erro de assunto desconhecido.
func suggestTopic(topic string, root *cobra.Command) string {
	const threshold = 3
	best := ""
	bestDist := threshold + 1

	candidates := make([]string, 0)
	for _, sub := range root.Commands() {
		if sub.IsAvailableCommand() {
			candidates = append(candidates, sub.Name())
		}
	}
	for _, d := range configDocs {
		candidates = append(candidates, d.Key)
	}

	for _, c := range candidates {
		dist := levenshtein(topic, c)
		if dist < bestDist {
			bestDist = dist
			best = c
		}
	}
	if bestDist > threshold {
		return ""
	}
	return best
}

func newHelpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "help [command|key]",
		Short: "Display help for commands and documentation for trackfw.yaml configuration keys",
		Long: `Without argument: lists available commands and configurable keys in trackfw.yaml.
With argument: shows help for the given command, or full documentation for the
specified configuration key.`,
		Args:               cobra.MaximumNArgs(1),
		DisableFlagParsing: false,
		// Silenciados: a mensagem de erro é composta e impressa por nós mesmos
		// (ver caso 3 abaixo), para que a saída seja idêntica à de Node/Python
		// — sem o bloco "Error: ...\nUsage: ..." que o cobra imprimiria por
		// padrão, o que duplicaria a mensagem quando o processo top-level
		// (Execute em root.go) também reimprime o erro retornado.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			root := cmd.Root()

			if len(args) == 0 {
				printCommandList(out, root)
				fmt.Fprintln(out)
				printConfigKeyList(out)
				return nil
			}

			topic := args[0]

			// 1) comando conhecido → ajuda do comando.
			if sub, _, err := root.Find([]string{topic}); err == nil && sub != root {
				sub.SetOut(out)
				sub.InitDefaultHelpFlag()
				return sub.Help()
			}

			// 2) chave de configuração conhecida → documentação da chave.
			for _, d := range configDocs {
				if d.Key == topic {
					printConfigKeyDoc(out, d)
					return nil
				}
			}

			// 3) assunto desconhecido → erro com sugestão útil.
			msg := "unknown topic: " + topic
			if s := suggestTopic(topic, root); s != "" {
				msg += "\nDid you mean: " + s + "?"
			}
			return errors.New(msg)
		},
	}
}

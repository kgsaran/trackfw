package commands

import (
	"errors"
	"fmt"
	"os"
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
		Description: "Diretórios onde os ADRs são armazenados.",
		Example:     `adr_dirs:
  - docs/adr
  - docs/adr/zeus`,
		Impact: "Alterar adiciona ou remove diretórios rastreados nas validações de ADR.",
	},
	{
		Key:         "req_dir",
		Type:        "string",
		Default:     `"docs/req"`,
		Description: "Diretório onde as REQs são armazenadas.",
		Example:     `req_dir: docs/requirements`,
		Impact:      "Alterar muda onde o gate procura REQs para validação.",
	},
	{
		Key:         "roadmap_dir",
		Type:        "string",
		Default:     `"docs/roadmaps"`,
		Description: "Diretório raiz dos roadmaps.",
		Example:     `roadmap_dir: docs/roadmaps`,
		Impact:      "Alterar muda onde o gate procura roadmaps em backlog/wip/blocked/done.",
	},
	{
		Key:         "roadmap_namespacing",
		Type:        "flat|by_agent",
		Default:     `"flat"`,
		Description: "Estratégia de namespacing dos roadmaps.",
		Example:     `roadmap_namespacing: by_agent`,
		Impact:      "by_agent cria subdiretórios por agente; flat usa diretório único por estado.",
	},
	{
		Key:         "agents",
		Type:        "list of strings",
		Default:     `[]`,
		Description: "Lista de agentes ativos no projeto.",
		Example: `agents:
  - apolo
  - afrodite`,
		Impact: "Agentes registrados recebem subdiretórios próprios no modo by_agent.",
	},
	{
		Key:         "governance_mode",
		Type:        "string",
		Default:     `""`,
		Description: "Modo de governança (strict, lenient).",
		Example:     `governance_mode: lenient`,
		Impact:      "lenient converte violations em warnings; strict (padrão) bloqueia com exit 1.",
	},
	{
		Key:         "lenient_until",
		Type:        "date (YYYY-MM-DD)",
		Default:     `""`,
		Description: "Data até quando o modo lenient está ativo.",
		Example:     `lenient_until: 2026-12-31`,
		Impact:      "Após a data, o modo volta a strict automaticamente.",
	},
	{
		Key:         "wip_limit",
		Type:        "integer",
		Default:     `1`,
		Description: "Limite de itens WIP simultâneos.",
		Example:     `wip_limit: 3`,
		Impact:      "Aumentar reduz a frequência de bloqueio; diminuir exige mais disciplina.",
	},
	{
		Key:         "wip_by_squad",
		Type:        "boolean",
		Default:     `false`,
		Description: "Aplicar limite WIP por squad individualmente.",
		Example:     `wip_by_squad: true`,
		Impact:      "true aplica o limite por squad; false aplica ao total do projeto.",
	},
	{
		Key:         "require_req_in_commit",
		Type:        "boolean",
		Default:     `false`,
		Description: "Exigir referência de REQ em mensagens de commit.",
		Example:     `require_req_in_commit: true`,
		Impact:      "true instala hook commit-msg que bloqueia commits sem referência REQ.",
	},
	{
		Key:         "link_fields.req",
		Type:        "list of strings",
		Default:     `["REQ:"]`,
		Description: "Marcadores que identificam link a REQ.",
		Example: `link_fields:
  req:
    - "REQ:"
    - "req_id:"`,
		Impact: "Alterar muda quais tokens o gate reconhece como link de REQ.",
	},
	{
		Key:         "link_fields.adr",
		Type:        "list of strings",
		Default:     `["ADR:"]`,
		Description: "Marcadores que identificam link a ADR.",
		Example: `link_fields:
  adr:
    - "ADR:"
    - "adr_id:"`,
		Impact: "Alterar muda quais tokens o gate reconhece como link de ADR.",
	},
	{
		Key:         "link_fields.roadmap",
		Type:        "list of strings",
		Default:     `["Roadmap:"]`,
		Description: "Marcadores que identificam link a Roadmap.",
		Example: `link_fields:
  roadmap:
    - "Roadmap:"
    - "roadmap_id:"`,
		Impact: "Alterar muda quais tokens o gate reconhece como link de Roadmap.",
	},
	{
		Key:         "acceptance_markers",
		Type:        "list of strings",
		Default:     `["## Acceptance Criteria", "## Critérios de Aceite"]`,
		Description: "Marcadores de critério de aceite em documentos WIP.",
		Example: `acceptance_markers:
  - "## Acceptance Criteria"
  - "## Critérios de Aceite"
  - "## AC"`,
		Impact: "Alterar permite personalizar o cabeçalho de seção que o gate valida.",
	},
	{
		Key:         "rules.wip_has_req",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Severidade: WIP sem REQ linkada.",
		Example:     `rules:\n  wip_has_req: warning`,
		Impact:      "error bloqueia o gate; warning exibe mas não bloqueia; off ignora.",
	},
	{
		Key:         "rules.wip_acceptance",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Severidade: WIP sem critérios de aceite.",
		Example:     `rules:\n  wip_acceptance: warning`,
		Impact:      "error bloqueia o gate; warning exibe mas não bloqueia; off ignora.",
	},
	{
		Key:         "rules.wip_limit",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Severidade: excesso de itens WIP.",
		Example:     `rules:\n  wip_limit: warning`,
		Impact:      "error bloqueia o gate; warning exibe mas não bloqueia; off ignora.",
	},
	{
		Key:         "rules.stale_wip",
		Type:        "off|warning|error",
		Default:     `"warning"`,
		Description: "Severidade: WIP sem atualização recente.",
		Example:     `rules:\n  stale_wip: error`,
		Impact:      "Aumentar severidade força revisão de roadmaps parados.",
	},
	{
		Key:         "rules.adr_orphan",
		Type:        "off|warning|error",
		Default:     `"warning"`,
		Description: "Severidade: ADR sem REQ vinculada.",
		Example:     `rules:\n  adr_orphan: error`,
		Impact:      "error força que todo ADR seja referenciado por ao menos uma REQ.",
	},
	{
		Key:         "rules.ref_targets_exist",
		Type:        "off|warning|error",
		Default:     `"warning"`,
		Description: "Severidade: referências com destino inexistente.",
		Example:     `rules:\n  ref_targets_exist: error`,
		Impact:      "error bloqueia quando REQ ou ADR referenciados não existem no repositório.",
	},
	{
		Key:         "rules.folder_status",
		Type:        "off|warning|error",
		Default:     `"warning"`,
		Description: "Severidade: coerência entre pasta e status do arquivo.",
		Example:     `rules:\n  folder_status: error`,
		Impact:      "error bloqueia quando o status no frontmatter não bate com a pasta (ex: done/ com status Open).",
	},
	{
		Key:         "rules.filename_uniqueness",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Severidade: nomes de arquivo duplicados.",
		Example:     `rules:\n  filename_uniqueness: warning`,
		Impact:      "error bloqueia quando dois artefatos têm o mesmo basename independentemente de pasta.",
	},
	{
		Key:         "rules.blocked_by_draft_adr",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Severidade: REQ bloqueada por ADR em rascunho.",
		Example:     `rules:\n  blocked_by_draft_adr: warning`,
		Impact:      "error bloqueia quando REQ Open referencia ADR com Status: Draft.",
	},
	{
		Key:         "trace_id_field",
		Type:        "string",
		Default:     `""`,
		Description: "Campo de frontmatter usado como ID de rastreabilidade estável entre REQ e Roadmap. Vazio = desativado.",
		Example:     `trace_id_field: req_id`,
		Impact:      "Ativa verificação bidirecional REQ↔Roadmap (traceid_orphan_*, traceid_state_mismatch, traceid_duplicate_*).",
	},
	{
		Key:         "rules.traceid_orphan_roadmap",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Roadmap com req_id sem REQ correspondente.",
		Example:     "rules:\n  traceid_orphan_roadmap: warning",
		Impact:      "Detecta Roadmaps sem REQ pareada.",
	},
	{
		Key:         "rules.traceid_orphan_req",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "REQ com req_id sem Roadmap correspondente.",
		Example:     "rules:\n  traceid_orphan_req: warning",
		Impact:      "Detecta REQs sem Roadmap pareado.",
	},
	{
		Key:         "rules.traceid_state_mismatch",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "REQ e Roadmap com mesmo req_id em estados divergentes (ex: done/wip).",
		Example:     "rules:\n  traceid_state_mismatch: warning",
		Impact:      "Garante consistência de estado entre REQ e Roadmap.",
	},
	{
		Key:         "rules.traceid_duplicate_req",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Mesmo req_id em mais de uma REQ.",
		Example:     "rules:\n  traceid_duplicate_req: warning",
		Impact:      "Garante unicidade lógica de REQs.",
	},
	{
		Key:         "rules.traceid_duplicate_roadmap",
		Type:        "off|warning|error",
		Default:     `"error"`,
		Description: "Mesmo req_id em mais de um Roadmap.",
		Example:     "rules:\n  traceid_duplicate_roadmap: warning",
		Impact:      "Garante unicidade lógica de Roadmaps.",
	},
}

func newHelpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "help [key]",
		Short: "Exibe documentação das chaves de configuração do trackfw.yaml",
		Long: `Sem argumento: lista todas as chaves configuráveis com tipo, default e descrição.
Com argumento: exibe documentação completa da chave especificada.`,
		Args:               cobra.MaximumNArgs(1),
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()

			if len(args) == 0 {
				// Lista tabular: KEY | DEFAULT | DESCRIÇÃO
				w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
				fmt.Fprintln(w, "KEY\tDEFAULT\tDESCRIÇÃO")
				fmt.Fprintln(w, strings.Repeat("─", 80))
				for _, d := range configDocs {
					fmt.Fprintf(w, "%s\t%s\t%s\n", d.Key, d.Default, d.Description)
				}
				return w.Flush()
			}

			// Busca chave específica
			key := args[0]
			for _, d := range configDocs {
				if d.Key == key {
					fmt.Fprintf(out, "%s\n", d.Key)
					fmt.Fprintf(out, "  Type:    %s\n", d.Type)
					fmt.Fprintf(out, "  Default: %s\n", d.Default)
					fmt.Fprintf(out, "  Desc:    %s\n", d.Description)
					fmt.Fprintf(out, "  Example:\n")
					for _, line := range strings.Split(d.Example, "\n") {
						fmt.Fprintf(out, "    %s\n", line)
					}
					fmt.Fprintf(out, "  Impact:  %s\n", d.Impact)
					return nil
				}
			}

			fmt.Fprintf(os.Stderr, "chave desconhecida: %s\n", key)
			return errors.New("chave desconhecida: " + key)
		},
	}
}

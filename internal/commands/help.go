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

// printCommandList escreve a lista de comandos disponíveis de root em out.
func printCommandList(out io.Writer, root *cobra.Command) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Comandos disponíveis:")
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
	fmt.Fprintln(w, "Chaves de configuração (trackfw.yaml):")
	fmt.Fprintln(w, "KEY\tDEFAULT\tDESCRIÇÃO")
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
		Use:   "help [comando|chave]",
		Short: "Exibe ajuda de comandos e documentação das chaves de configuração do trackfw.yaml",
		Long: `Sem argumento: lista os comandos disponíveis e as chaves configuráveis do trackfw.yaml.
Com argumento: mostra a ajuda do comando informado, ou a documentação completa da
chave de configuração especificada.`,
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
			msg := "assunto desconhecido: " + topic
			if s := suggestTopic(topic, root); s != "" {
				msg += "\nVocê quis dizer: " + s + "?"
			}
			return errors.New(msg)
		},
	}
}

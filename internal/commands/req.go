package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

func newReqCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "req",
		Short: "Manage Requirements",
	}
	cmd.AddCommand(newReqNewCmd())
	cmd.AddCommand(newReqListCmd())
	cmd.AddCommand(newReqMoveCmd())
	return cmd
}

func newReqNewCmd() *cobra.Command {
	var agentFlag string
	var noRoadmap bool
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new REQ (and its roadmap, unless --no-roadmap)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReqNew(cmd, args, agentFlag, noRoadmap)
		},
	}
	cmd.Flags().StringVar(&agentFlag, "agent", "", "Agent namespace in by_agent projects")
	// ML-4A (AC10): o caminho DELIBERADO de criar REQ sem roadmap. É flag, e não ausência de flag,
	// porque o defeito medido é de ESQUECIMENTO: quem esquece produz o estado certo, quem decide
	// declara a decisão. Ver a comparação das três formas em runReqNew.
	cmd.Flags().BoolVar(&noRoadmap, "no-roadmap", false,
		"Create the REQ only, without a roadmap (deliberate: leaves the REQ unlinked)")
	return cmd
}

// runReqNew cria a REQ e, por padrão, o roadmap que a implementa — um comando, não dois.
//
// 🔴 ML-4A — por que o encadeamento é PADRÃO e não flag de adesão. O defeito que a REQ-2026-09-09
// nomeia é de esquecimento: o protocolo tem três passos e o segundo (`roadmap new`) cai. Três formas
// foram comparadas, pelo critério "o que acontece quando a pessoa esquece":
//
//  1. Flag de adesão (`--with-roadmap`): reproduz o defeito INTEIRO. Um flag que precisa ser lembrado
//     é a mesma coisa que um comando que precisa ser lembrado — só mais barata de digitar. Não fecha.
//  2. Prompt: morre no guard que este arquivo já tem duas linhas abaixo —
//     `cbterm.IsTerminal(uintptr(os.Stdin.Fd()))`. Sem TTY (CI, `stdin` fechado, agente) o prompt
//     não roda, então o defeito sobrevive exatamente onde este CLI mais roda. E prompt que roda em
//     pipeline TRAVA o pipeline, que é defeito novo.
//  3. Encadeamento padrão + saída deliberada (`--no-roadmap`): quem esquece obtém o estado
//     governado; quem decide o contrário escreve a decisão. É o que está implementado.
//
// Reversibilidade, que é o que autoriza inverter o padrão: o roadmap nasce em `backlog/`, onde ele
// não afeta nenhuma regra de `validate` (medido) e é removível com `roadmap move <nome> abandoned`.
// Já a REQ órfã é IRREVERSÍVEL no sentido que importa: ninguém volta para consertá-la, que é
// justamente o achado desta REQ.
func runReqNew(_ *cobra.Command, args []string, agentFlag string, noRoadmap bool) error {
	content := generators.REQContent{Title: args[0], Agent: agentFlag}

	// Detectar se stdin é TTY — wizard interativo somente em TTY
	if !cbterm.IsTerminal(uintptr(os.Stdin.Fd())) {
		res, err := generators.NewREQWithResult(content)
		if err != nil {
			return err
		}
		return chainRoadmapForREQ(res, noRoadmap)
	}

	// Form 1 — coleta título + motivação
	form1 := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project requirement").
				Description("Describe what you want to build or change").
				Value(&content.Title),
			huh.NewInput().
				Title("Motivation").
				Description("Why is this requirement needed?").
				Value(&content.Motivation),
		),
	)
	if err := form1.Run(); err != nil {
		return fmt.Errorf("wizard: %w", err)
	}

	// Detectar probes com base em título + motivação
	intention := content.Title + " " + content.Motivation
	detectedProbes := generators.DetectDomains(intention)

	// Construir grupos do Form 2 — sem campos manuais de ADR/roadmap:
	// os vínculos com ADRs são estabelecidos automaticamente pelo probe discovery abaixo.
	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().
				Title("Acceptance Criteria").
				Description("List acceptance criteria, one per line").
				Value(&content.Criteria),
		),
	}

	// Slice de respostas — indexada para evitar bug de closure
	answers := make([]string, 0)
	type questionRef struct {
		options []generators.ProbeOption
	}
	var questionRefs []questionRef

	for _, probe := range detectedProbes {
		for _, question := range probe.Questions {
			answers = append(answers, "")
			questionRefs = append(questionRefs, questionRef{options: question.Options})
			idx := len(answers) - 1
			opts := make([]huh.Option[string], len(question.Options))
			for i, opt := range question.Options {
				opts[i] = huh.NewOption(opt.Label, opt.ADRSlug)
			}
			groups = append(groups, huh.NewGroup(
				huh.NewSelect[string]().
					Title(question.Text).
					Options(opts...).
					Value(&answers[idx]),
			))
		}
	}

	// Escopo dos ADR drafts desta sessão de REQ — uma única pergunta para todas as probes,
	// não repetida por probe. Default "local", mesmo comportamento de antes desta escolha existir.
	adrScope := "local"
	scopeForm := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Escopo dos ADRs desta REQ").
				Options(
					huh.NewOption("Local (padrão)", "local"),
					huh.NewOption("Global (~/.trackfw/adr, cross-project)", "global"),
				).
				Value(&adrScope),
		),
	)
	if err := scopeForm.Run(); err != nil {
		return fmt.Errorf("wizard: %w", err)
	}

	// Form 2 — critérios, links e probes
	form2 := huh.NewForm(groups...)
	if err := form2.Run(); err != nil {
		return fmt.Errorf("wizard: %w", err)
	}

	// Resolve o diretório de ADR uma única vez para toda a sessão, com base na escolha acima.
	adrDir, err := resolveADRDir(map[string]string{"local": "project", "global": "global"}[adrScope])
	if err != nil {
		return err
	}

	// Processar respostas das probes → gerar ADR Drafts
	var generatedADRs []string
	for i, answer := range answers {
		_ = questionRefs[i] // referência mantida para rastreabilidade futura
		if answer != "" {   // ADRSlug não-vazio = decisão pendente
			basename, err := generators.NewADRDraft(answer, adrDir)
			if err != nil {
				fmt.Printf("warning: could not create ADR draft for %s: %v\n", answer, err)
				continue
			}
			generatedADRs = append(generatedADRs, basename)
		}
	}
	content.DependsOnADRs = uniqueStrings(generatedADRs)

	res, err := generators.NewREQWithResult(content)
	if err != nil {
		return err
	}

	if len(content.DependsOnADRs) > 0 {
		fmt.Println("\nADR drafts created:")
		for _, adr := range content.DependsOnADRs {
			fmt.Printf("  -> %s\n", adr)
		}
		// ML-4A: o conselho mudou porque o padrão mudou — o roadmap é criado no mesmo ato, logo
		// "antes de criar um roadmap" seria conselho impossível de seguir. E ele mudou para o que foi
		// MEDIDO: com o roadmap em backlog/, uma REQ bloqueada por ADR Draft dispara exatamente as
		// mesmas regras de validate que sem ele (blocked_by_draft_adr + req_has_adr) — nenhuma regra
		// nova. O que os ADRs Draft realmente barram é a PROMOÇÃO do roadmap, não a criação dele.
		fmt.Println("\nResolve these ADRs (set Status: Accepted) before moving the roadmap to wip.")
	}

	return chainRoadmapForREQ(res, noRoadmap)
}

// chainRoadmapForREQ é o caminho integrado do ML-4A: dada a REQ recém-criada, garante que exista um
// roadmap vinculado a ela.
//
// Três ramos, nesta ordem:
//
//  1. `--no-roadmap` — caminho deliberado. Não cria nada e DIZ o que ficou faltando, com o comando
//     exato. Silêncio aqui seria o defeito original com outro nome.
//  2. Roadmap já existente que aponta para esta REQ — não cria um segundo (idempotência), e regrava
//     o ponteiro na REQ, que `req new` acabou de reescrever do zero.
//  3. Caso comum — `NewRoadmapFromREQ`, o MESMO caminho que `roadmap new --from-req` usa: MLs
//     rascunho a partir dos ACs, `req:` no frontmatter do roadmap e backlink na REQ (ML-1B).
//
// O agente vem RESOLVIDO em res.Agent, não rederivado do caminho: é o que impede a REQ e o roadmap
// de discordarem de namespace em projetos by_agent.
//
// Falha na criação do roadmap é FATAL (exit não-zero), ao contrário da falha de backlink do ML-1B.
// A assimetria é deliberada: lá o artefato prometido já existia no disco e só o ponteiro faltava;
// aqui o artefato prometido pelo comando NÃO existe, e sair com 0 diria a um script de CI que a
// governança está completa quando ela não está.
func chainRoadmapForREQ(res generators.REQResult, noRoadmap bool) error {
	if noRoadmap {
		fmt.Printf("\n--no-roadmap: %s has no roadmap yet. Link one with:\n  trackfw roadmap new --from-req %s\n",
			filepath.Base(res.Path), res.Path)
		return nil
	}

	if existing := generators.FindRoadmapLinkingREQ(res.Path); existing != "" {
		fmt.Printf("roadmap already linked to this REQ: %s (not recreated)\n", existing)
		generators.RelinkREQToRoadmap(res.Path, existing)
		return nil
	}

	if err := generators.NewRoadmapFromREQ(res.Path, res.Agent, false); err != nil {
		return fmt.Errorf("REQ created at %s, but its roadmap was not: %w\n"+
			"finish with: trackfw roadmap new --from-req %s (or accept the gap with --no-roadmap)",
			res.Path, err, res.Path)
	}
	return nil
}

// uniqueStrings remove duplicatas mantendo a ordem de primeira ocorrência.
func uniqueStrings(ss []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

func newReqListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all REQs in docs/req/",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.ListREQs()
		},
	}
}

func newReqMoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "move <name> <status>",
		Short: "Update a REQ status in place",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.MoveREQ(args[0], args[1])
		},
	}
}

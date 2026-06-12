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
	return cmd
}

func newReqNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new REQ",
		Args:  cobra.ExactArgs(1),
		RunE:  runReqNew,
	}
}

func runReqNew(_ *cobra.Command, args []string) error {
	content := generators.REQContent{Title: args[0]}

	// Detectar se stdin é TTY — wizard interativo somente em TTY
	if !cbterm.IsTerminal(uintptr(os.Stdin.Fd())) {
		return generators.NewREQ(content)
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

	// Construir grupos do Form 2
	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewInput().
				Title("Acceptance Criteria").
				Description("List acceptance criteria, one per line").
				Value(&content.Criteria),
		),
	}

	// Linked ADR — select se existirem ADRs, input se não
	adrFiles, _ := filepath.Glob("docs/adr/*.md")
	if len(adrFiles) > 0 {
		adrOpts := []huh.Option[string]{huh.NewOption("None", "")}
		for _, f := range adrFiles {
			base := filepath.Base(f)
			adrOpts = append(adrOpts, huh.NewOption(base, base))
		}
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Linked ADR").
				Description("Select an existing ADR to link (or None)").
				Options(adrOpts...).
				Value(&content.LinkedADR),
		))
	} else {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title("Linked ADR").
				Description("ADR filename or slug (leave blank if none)").
				Value(&content.LinkedADR),
		))
	}

	// Linked Roadmap — select se existirem roadmaps, input se não
	var roadmapFiles []string
	for _, state := range []string{"backlog", "wip", "blocked", "done"} {
		matches, _ := filepath.Glob(filepath.Join("docs/roadmaps", state, "*.md"))
		roadmapFiles = append(roadmapFiles, matches...)
	}
	if len(roadmapFiles) > 0 {
		rmOpts := []huh.Option[string]{huh.NewOption("None", "")}
		for _, f := range roadmapFiles {
			base := filepath.Base(f)
			rmOpts = append(rmOpts, huh.NewOption(base, base))
		}
		groups = append(groups, huh.NewGroup(
			huh.NewSelect[string]().
				Title("Linked Roadmap").
				Description("Select an existing roadmap to link (or None)").
				Options(rmOpts...).
				Value(&content.LinkedRoadmap),
		))
	} else {
		groups = append(groups, huh.NewGroup(
			huh.NewInput().
				Title("Linked Roadmap").
				Description("Roadmap filename or slug (leave blank if none)").
				Value(&content.LinkedRoadmap),
		))
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

	// Form 2 — critérios, links e probes
	form2 := huh.NewForm(groups...)
	if err := form2.Run(); err != nil {
		return fmt.Errorf("wizard: %w", err)
	}

	// Processar respostas das probes → gerar ADR Drafts
	var generatedADRs []string
	for i, answer := range answers {
		_ = questionRefs[i] // referência mantida para rastreabilidade futura
		if answer != "" {   // ADRSlug não-vazio = decisão pendente
			basename, err := generators.NewADRDraft(answer)
			if err != nil {
				fmt.Printf("warning: could not create ADR draft for %s: %v\n", answer, err)
				continue
			}
			generatedADRs = append(generatedADRs, basename)
		}
	}
	content.DependsOnADRs = uniqueStrings(generatedADRs)

	if err := generators.NewREQ(content); err != nil {
		return err
	}

	if len(content.DependsOnADRs) > 0 {
		fmt.Println("\nADR drafts created:")
		for _, adr := range content.DependsOnADRs {
			fmt.Printf("  -> %s\n", adr)
		}
		fmt.Println("\nResolve these ADRs (set Status: Accepted) before creating a roadmap.")
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
			return generators.ListREQs("docs/req")
		},
	}
}

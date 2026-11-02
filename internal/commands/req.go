package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/validator"
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
	var adrFlag string
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new REQ (and its roadmap, unless --no-roadmap)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReqNewFull(cmd, args, agentFlag, noRoadmap, adrFlag)
		},
	}
	cmd.Flags().StringVar(&agentFlag, "agent", "", "Agent namespace in by_agent projects")
	// ML-4A (AC10): o caminho DELIBERADO de criar REQ sem roadmap. É flag, e não ausência de flag,
	// porque o defeito medido é de ESQUECIMENTO: quem esquece produz o estado certo, quem decide
	// declara a decisão. Ver a comparação das três formas em runReqNew.
	cmd.Flags().BoolVar(&noRoadmap, "no-roadmap", false,
		"Create the REQ only, without a roadmap (deliberate: leaves the REQ unlinked)")
	cmd.Flags().StringVar(&adrFlag, "adr", "",
		"Link an existing ADR by filename (with or without .md extension); resolves via adr_dirs")
	return cmd
}

// runReqNew is the backward-compatible entry point for tests that call it directly.
// It delegates to runReqNewFull with an empty adrFlag (no --adr).
//
// 🔴 Do NOT change this signature: req_chain_ml4a_test.go calls runReqNew directly
// and is outside the editable-file list for ML-1B.
func runReqNew(_ *cobra.Command, args []string, agentFlag string, noRoadmap bool) error {
	return runReqNewFull(nil, args, agentFlag, noRoadmap, "")
}

// runReqNewFull cria a REQ e, por padrão, o roadmap que a implementa — um comando, não dois.
// It is the full implementation; runReqNew and cobra RunE both delegate here.
//
// adrFlag, when non-empty, is the --adr value: validated (P5), resolved (P6), and
// written as the portably-relative ADR link in both the frontmatter adr: field and the
// body ADR: line so that req_has_adr passes without manual editing.
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
func runReqNewFull(_ *cobra.Command, args []string, agentFlag string, noRoadmap bool, adrFlag string) error {
	content := generators.REQContent{Title: args[0], Agent: agentFlag}

	// Resolve --adr BEFORE the TTY branch so both the wizard and non-TTY paths get the link.
	// Order: P5 validation → P6 resolution → P4 portable path → set on content.
	// Any error here is fatal: the ADR must be resolved before any file is created (P3).
	if adrFlag != "" {
		if err := validateADRFlagValue(adrFlag); err != nil {
			return err
		}
		cfg := config.Load()
		hit, err := resolveADRByArg(adrFlag, cfg)
		if err != nil {
			return err
		}
		portablePath, err := adrPortablePath(hit)
		if err != nil {
			return err
		}
		content.FrontmatterADR = portablePath
	}

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
//     o ponteiro na REQ quando o campo roadmap: ainda estiver vazio (a REQ é mantida intacta — AC3b).
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

// validateADRFlagValue validates the value of --adr per P5 (Wave 0 prescription).
// Rejected characters: /, \, .. (traversal), NUL, \n, \r, ", backtick.
// Space is accepted (P5 explicitly allows it).
func validateADRFlagValue(v string) error {
	if v == "" {
		return fmt.Errorf("--adr: ADR name must not be empty")
	}
	for i, r := range v {
		switch r {
		case '/', '\\', '\x00', '\n', '\r', '"', '`':
			return fmt.Errorf("--adr: invalid character %q at position %d in %q (path separators, control characters, quotes and backticks are not allowed)", r, i, v)
		}
	}
	// Reject ".." (traversal component — slashes already blocked above, so ".." alone is the remaining risk).
	if v == ".." {
		return fmt.Errorf("--adr: path traversal not allowed: %q", v)
	}
	return nil
}

// resolveADRByArg finds the single ADR file matching arg using exact basename comparison (P6).
// Match condition: base == arg OR base == arg+".md" (string-exact, not case-folded).
// Returns error when zero or more than one file matches.
func resolveADRByArg(arg string, cfg config.ProjectConfig) (string, error) {
	var matches []string
	for _, path := range validator.ResolveADRFiles(cfg) {
		base := filepath.Base(path)
		if base == arg || base == arg+".md" {
			matches = append(matches, path)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("--adr: ADR not found: %q (checked adr_dirs: %v)", arg, cfg.ADRDirs)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("--adr: ADR %q is ambiguous — found in multiple locations: %v", arg, matches)
	}
}

// adrPortablePath converts an ADR file path returned by ResolveADRFiles to a portable
// project-relative or HOME-relative path suitable for storage in REQ frontmatter (P4).
// Always uses forward slashes (filepath.ToSlash).
func adrPortablePath(hit string) (string, error) {
	// Canonicalize project root (handles /var → /private/var on macOS).
	absRoot, err := filepath.EvalSymlinks(".")
	if err != nil {
		absRoot = filepath.Clean(".")
	}

	// Make hit absolute relative to project root if it isn't already.
	absHit := hit
	if !filepath.IsAbs(absHit) {
		absHit = filepath.Join(absRoot, absHit)
	}

	// EvalSymlinks on the parent directory only (preserves the logical filename of a symlink-file).
	hitDir, hitBase := filepath.Split(absHit)
	canonDir, err := filepath.EvalSymlinks(hitDir)
	if err != nil {
		canonDir = filepath.Clean(hitDir)
	}
	absHitCanon := filepath.Join(canonDir, hitBase)

	canonRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		canonRoot = filepath.Clean(absRoot)
	}

	sep := string(os.PathSeparator)
	rel, relErr := filepath.Rel(canonRoot, absHitCanon)
	if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+sep) {
		return filepath.ToSlash(rel), nil
	}

	// Fallback: HOME-relative path (for global ADRs in ~/.trackfw/adr).
	home, _ := os.UserHomeDir()
	canonHome, homeErr := filepath.EvalSymlinks(home)
	if homeErr != nil || canonHome == "" {
		canonHome = home
	}
	relHome, homeRelErr := filepath.Rel(canonHome, absHitCanon)
	if homeRelErr == nil && relHome != ".." && !strings.HasPrefix(relHome, ".."+sep) {
		return "~/" + filepath.ToSlash(relHome), nil
	}

	return "", fmt.Errorf("--adr: ADR %q is outside both the project root and HOME; use a project-local ADR", hit)
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

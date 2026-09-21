package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/kgsaran/trackfw/internal/pathguard"
)

const configureYAMLHeader = "# trackfw.yaml — gerado por trackfw configure\n"

func newConfigureCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "configure",
		Short: "Wizard interativo para criar/recriar trackfw.yaml",
		Long: `Wizard interativo que guia a configuração do trackfw.yaml.
Gera arquivo esparso: apenas campos que diferem dos defaults são gravados.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verificar se trackfw.yaml já existe
			if _, err := os.Stat("trackfw.yaml"); err == nil {
				var action string
				confirm := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title("trackfw.yaml já existe. O que deseja fazer?").
							Options(
								huh.NewOption("Recriar do zero", "recreate"),
								huh.NewOption("Cancelar", "cancel"),
							).
							Value(&action),
					),
				)
				if err := confirm.Run(); err != nil {
					return fmt.Errorf("erro no wizard: %w", err)
				}
				if action == "cancel" {
					fmt.Println("Operação cancelada.")
					return nil
				}
			}

			// Campos principais
			var (
				adrDirsRaw  = "docs/adr"
				reqDir      = "docs/req"
				roadmapDir  = "docs/roadmaps"
				wipLimitStr = "1"
				reqMarker   = "REQ:"
			)

			mainForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Diretórios de ADR (separados por vírgula)").
						Placeholder("docs/adr").
						Value(&adrDirsRaw),
					huh.NewInput().
						Title("Diretório de REQs").
						Placeholder("docs/req").
						Value(&reqDir),
					huh.NewInput().
						Title("Diretório raiz de Roadmaps").
						Placeholder("docs/roadmaps").
						Value(&roadmapDir),
					huh.NewInput().
						Title("Limite de itens WIP simultâneos").
						Placeholder("1").
						Value(&wipLimitStr),
				),
				huh.NewGroup(
					huh.NewInput().
						Title("Marcador de REQ inline (ex: REQ: ou req_id:)").
						Placeholder("REQ:").
						Value(&reqMarker),
				),
			)

			if err := mainForm.Run(); err != nil {
				return fmt.Errorf("erro no wizard: %w", err)
			}

			// Processar valores
			adrDirs := parseCommaSeparated(adrDirsRaw)
			wipLimit, err := strconv.Atoi(strings.TrimSpace(wipLimitStr))
			if err != nil || wipLimit < 1 {
				wipLimit = 1
			}

			// Defaults de comparação
			defaultAdrDirs := []string{"docs/adr"}
			defaultReqDir := "docs/req"
			defaultRoadmapDir := "docs/roadmaps"
			defaultWipLimit := 1
			defaultReqMarker := "REQ:"

			// Construir config esparsa
			var lines []string
			customCount := 0

			if !stringSliceEqual(adrDirs, defaultAdrDirs) {
				lines = append(lines, "adr_dirs:")
				for _, d := range adrDirs {
					lines = append(lines, "  - "+d)
				}
				customCount++
			}

			if strings.TrimSpace(reqDir) != defaultReqDir {
				lines = append(lines, "req_dir: "+strings.TrimSpace(reqDir))
				customCount++
			}

			if strings.TrimSpace(roadmapDir) != defaultRoadmapDir {
				lines = append(lines, "roadmap_dir: "+strings.TrimSpace(roadmapDir))
				customCount++
			}

			if wipLimit != defaultWipLimit {
				lines = append(lines, fmt.Sprintf("wip_limit: %d", wipLimit))
				customCount++
			}

			if strings.TrimSpace(reqMarker) != defaultReqMarker {
				lines = append(lines, "link_fields:")
				lines = append(lines, "  req:")
				lines = append(lines, "    - \""+strings.TrimSpace(reqMarker)+"\"")
				customCount++
			}

			// Gravar arquivo
			var content string
			content = configureYAMLHeader
			if len(lines) > 0 {
				content += strings.Join(lines, "\n") + "\n"
			}

			// Guard before write: resolve project root from CWD, then reject any
			// symlink ancestor between root and trackfw.yaml. Guard precedes write
			// (and any MkdirAll) so a refused destination creates no stray file.
			// Root: EvalSymlinks(Getwd()) — macOS /tmp→/private/tmp invariant.
			configureRoot, cwdErr := os.Getwd()
			if cwdErr == nil {
				if resolved, resolveErr := filepath.EvalSymlinks(configureRoot); resolveErr == nil {
					configureRoot = resolved
				}
				absYAML := filepath.Join(configureRoot, "trackfw.yaml")
				if guardErr := pathguard.RejectSymlinks(configureRoot, absYAML); guardErr != nil {
					fmt.Fprintf(os.Stderr, "trackfw: refusing write to %s: %v\n", absYAML, guardErr)
					return fmt.Errorf("refusing write to trackfw.yaml: %w", guardErr)
				}
			}
			// write-containment-allowed: guarded by pathguard.RejectSymlinks at the enclosing write site
			if err := os.WriteFile("trackfw.yaml", []byte(content), 0644); err != nil {
				return fmt.Errorf("erro ao gravar trackfw.yaml: %w", err)
			}

			fmt.Printf("trackfw.yaml gravado com %d campos customizados\n", customCount)
			return nil
		},
	}
}

// parseCommaSeparated divide string por vírgula e remove espaços.
func parseCommaSeparated(s string) []string {
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return []string{"docs/adr"}
	}
	return result
}

// stringSliceEqual compara duas slices de string.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

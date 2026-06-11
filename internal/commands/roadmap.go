package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	cbterm "github.com/charmbracelet/x/term"
	"github.com/kgsaran/trackfw/internal/ai"
	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/spf13/cobra"
)

const roadmapPromptTemplate = `Você é um assistente de engenharia de software. Com base na REQ abaixo, gere um roadmap de implementação em Markdown seguindo ESTRITAMENTE este formato:

# Roadmap: <título derivado da REQ>

> Criado em: %s | Status: ⬜ Backlog

## Diagnóstico / Contexto
(resumo do problema a resolver com base na REQ)

## Wave 1 — <nome da wave> (N MLs em paralelo)
> Dependências: Independente

### ML-1A — <título do ML>
**Status:** ⬜ Pendente
**Arquivos afetados:** lista de arquivos
**Ações:** lista detalhada de ações
**Critérios de aceite:**
- [ ] build sem erros
- [ ] testes verdes
**Comandos de validação:** ` + "`go build ./...`" + `

(Adicione quantas Waves e MLs forem necessários. Maximize paralelismo entre MLs sem dependência de arquivos compartilhados. Cada ML deve ser executável por um agente independente.)

REQ:
---
%s
---`

func newRoadmapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "roadmap",
		Short: "Manage Roadmaps",
	}
	cmd.AddCommand(newRoadmapNewCmd(), newRoadmapMoveCmd())
	return cmd
}

func newRoadmapNewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new",
		Short: "Create a new roadmap from a REQ (AI-assisted when configured)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Listar REQs disponíveis
			reqFiles, _ := filepath.Glob("docs/req/*.md")

			var selectedREQ string
			var reqContent string

			isTTY := cbterm.IsTerminal(uintptr(os.Stdin.Fd()))

			if isTTY && len(reqFiles) > 0 {
				// Wizard: selecionar REQ
				options := make([]huh.Option[string], len(reqFiles))
				for i, f := range reqFiles {
					options[i] = huh.NewOption(filepath.Base(f), f)
				}
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title("Select a REQ to generate the roadmap from:").
							Options(options...).
							Value(&selectedREQ),
					),
				)
				if err := form.Run(); err != nil {
					return fmt.Errorf("wizard: %w", err)
				}
				data, err := os.ReadFile(selectedREQ)
				if err != nil {
					return fmt.Errorf("reading REQ: %w", err)
				}
				reqContent = string(data)
			} else if len(args) > 0 {
				// Nao-TTY ou sem REQs: usar titulo do argumento
				selectedREQ = args[0]
			} else if len(reqFiles) == 0 {
				fmt.Fprintln(os.Stderr, "Nenhuma REQ encontrada em docs/req/. Crie uma REQ primeiro com 'trackfw req new'.")
				return nil
			}

			// Extrair titulo da REQ (nome do arquivo sem extensão, removendo prefixo REQ-)
			title := strings.TrimSuffix(filepath.Base(selectedREQ), ".md")
			title = strings.TrimPrefix(title, "REQ-")

			// Tentar geração por IA
			var body string
			provider, model, apiKey, _ := ai.ReadConfig("trackfw.yaml")

			if provider != "" && provider != "none" && apiKey != "" {
				client, err := ai.NewClient(provider, model, apiKey)
				if err == nil {
					date := time.Now().Format("2006-01-02")
					prompt := fmt.Sprintf(roadmapPromptTemplate, date, reqContent)
					generated, err := client.Generate(context.Background(), prompt)
					if err != nil {
						fmt.Fprintf(os.Stderr, "AI indisponivel (%v), usando template vazio\n", err)
					} else {
						body = generated
					}
				} else {
					fmt.Fprintf(os.Stderr, "AI provider error: %v — usando template vazio\n", err)
				}
			}

			return generators.NewRoadmapFromContent(generators.RoadmapContent{
				Title:   title,
				REQPath: selectedREQ,
				Body:    body,
			})
		},
	}
}

func newRoadmapMoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "move <name> <state>",
		Short: "Move a roadmap between states (backlog|wip|blocked|done|abandoned)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return generators.MoveRoadmap(args[0], args[1])
		},
	}
}

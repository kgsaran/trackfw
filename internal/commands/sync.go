package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/kgsaran/trackfw/internal/sync"
	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	var to string

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Open REQs to a project management tool",
		Long: `Reads all Open REQs without a linked PM issue, creates issues in the target
tool, and updates the REQ frontmatter with the returned issue ID.

Idempotent: REQs that already have an issue linked are skipped.

Supported targets:
  --to=linear   Create issues in Linear (requires LINEAR_API_KEY, LINEAR_TEAM_ID)
  --to=jira     Create issues in Jira   (requires JIRA_BASE_URL, JIRA_EMAIL, JIRA_TOKEN, JIRA_PROJECT)

Credentials can be set via env vars or in trackfw.yaml:
  linear_api_key, linear_team_id
  jira_base_url, jira_email, jira_token, jira_project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				results []sync.SyncResult
				err     error
			)

			switch to {
			case "linear":
				results, err = sync.SyncToLinear()
			case "jira":
				results, err = sync.SyncToJira()
			default:
				return fmt.Errorf("unknown target %q — use --to=linear or --to=jira", to)
			}

			if err != nil {
				// AC6: zero REQs → mensagem com req_dir verbatim, exit 0 (não é um erro).
				var noREQ *sync.ErrNoREQsFound
				if errors.As(err, &noREQ) {
					fmt.Printf("No REQs found in req_dir %q\n", noREQ.REQDir)
					return nil
				}
				return err
			}

			if len(results) == 0 {
				// Todos os REQs foram skipped (nenhum estava Open ou todos já tinham issue).
				fmt.Println("No new REQs to sync (all skipped)")
				return nil
			}

			fmt.Printf("%-55s %s\n", "REQ", "ISSUE")
			fmt.Printf("%-55s %s\n", strings.Repeat("-", 54), strings.Repeat("-", 10))
			for _, r := range results {
				if r.Skipped {
					fmt.Printf("%-55s (skipped)\n", r.REQPath)
					continue
				}
				if r.Error != nil {
					fmt.Printf("%-55s ERROR: %v\n", r.REQPath, r.Error)
					continue
				}
				fmt.Printf("%-55s %s\n", r.REQPath, r.IssueID)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "Target PM tool: linear or jira")
	_ = cmd.MarkFlagRequired("to")

	return cmd
}

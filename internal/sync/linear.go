package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/kgsaran/trackfw/internal/config"
)

// LinearClient encapsula credenciais para a API do Linear.
type LinearClient struct {
	APIKey string
	TeamID string
}

// NewLinearClient cria um cliente Linear a partir de trackfw.yaml ou variáveis de ambiente.
// Ordem de busca: 1) trackfw.yaml (linear_api_key, linear_team_id)
//
//	2) env vars LINEAR_API_KEY, LINEAR_TEAM_ID
func NewLinearClient() (*LinearClient, error) {
	sc := config.Load().Sync
	apiKey := sc.LinearAPIKey
	if apiKey == "" {
		apiKey = os.Getenv("LINEAR_API_KEY")
	}
	teamID := sc.LinearTeamID
	if teamID == "" {
		teamID = os.Getenv("LINEAR_TEAM_ID")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("Linear API key not found. Set LINEAR_API_KEY env var or linear_api_key in trackfw.yaml")
	}
	if teamID == "" {
		return nil, fmt.Errorf("Linear Team ID not found. Set LINEAR_TEAM_ID env var or linear_team_id in trackfw.yaml")
	}
	return &LinearClient{APIKey: apiKey, TeamID: teamID}, nil
}

// CreateIssue cria uma issue no Linear e retorna o identifier (ex: "ENG-123").
func (c *LinearClient) CreateIssue(title, description string) (string, error) {
	query := `mutation IssueCreate($title: String!, $description: String!, $teamId: String!) {
		issueCreate(input: {title: $title, description: $description, teamId: $teamId}) {
			success
			issue {
				id
				identifier
			}
		}
	}`

	payload := map[string]interface{}{
		"query": query,
		"variables": map[string]string{
			"title":       title,
			"description": description,
			"teamId":      c.TeamID,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("linear: marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.linear.app/graphql", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("linear: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("linear: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("linear: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("linear: unexpected status %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Data struct {
			IssueCreate struct {
				Success bool `json:"success"`
				Issue   struct {
					ID         string `json:"id"`
					Identifier string `json:"identifier"`
				} `json:"issue"`
			} `json:"issueCreate"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("linear: parse response: %w", err)
	}

	if len(result.Errors) > 0 {
		return "", fmt.Errorf("linear: API error: %s", result.Errors[0].Message)
	}

	if !result.Data.IssueCreate.Success {
		return "", fmt.Errorf("linear: issueCreate returned success=false")
	}

	return result.Data.IssueCreate.Issue.Identifier, nil
}

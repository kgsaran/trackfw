package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/netdeadline"
)

const linearDefaultEndpoint = "https://api.linear.app/graphql"

// LinearClient encapsula credenciais para a API do Linear.
type LinearClient struct {
	APIKey string
	TeamID string
	// Endpoint is the GraphQL API URL; leave empty for production (linearDefaultEndpoint).
	// Override in tests to point at an httptest.Server.
	Endpoint string
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

	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = linearDefaultEndpoint
	}

	// ADR D7/A10 (ML-1C): apply TRACKFW_NETWORK_TIMEOUT to the Linear API call.
	// Using context.WithTimeout + NewRequestWithContext instead of http.Client.Timeout
	// so that only the user-configured deadline (ctx.Err() == context.DeadlineExceeded)
	// produces a TimeoutError — internal transport deadlines (TLS handshake, dial) do not.
	limit := netdeadline.NetworkTimeout()
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("linear: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			te := &netdeadline.TimeoutError{Op: "linear API", Limit: limit, Var: netdeadline.NetworkTimeoutEnv}
			// ADR D7/A8 (ML-1C): issueCreate is a write (POST mutation); a timeout leaves
			// remote state uncertain — the issue may have been created on Linear.
			return "", fmt.Errorf("linear: issueCreate: %w — remote state is uncertain: check if the issue was created on Linear before running again", te)
		}
		return "", fmt.Errorf("linear: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			te := &netdeadline.TimeoutError{Op: "linear API", Limit: limit, Var: netdeadline.NetworkTimeoutEnv}
			// ADR D7/A8 (ML-1C): timeout while reading response body also leaves state uncertain.
			return "", fmt.Errorf("linear: issueCreate: %w — remote state is uncertain: check if the issue was created on Linear before running again", te)
		}
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

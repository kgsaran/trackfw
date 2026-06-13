package sync

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	apiKey := readConfigField("linear_api_key")
	if apiKey == "" {
		apiKey = os.Getenv("LINEAR_API_KEY")
	}
	teamID := readConfigField("linear_team_id")
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

// readConfigField lê um campo de trackfw.yaml pelo nome (parse linha a linha).
func readConfigField(field string) string {
	data, err := os.ReadFile("trackfw.yaml")
	if err != nil {
		return ""
	}
	prefix := field + ":"
	for _, line := range splitLines(string(data)) {
		trimmed := trimLeft(line)
		if len(trimmed) > len(prefix) && trimmed[:len(prefix)] == prefix {
			value := trimmed[len(prefix):]
			value = trim(value)
			// remover aspas simples ou duplas ao redor do valor
			if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
				value = value[1 : len(value)-1]
			}
			return value
		}
	}
	return ""
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimLeft(s string) string {
	for i, c := range s {
		if c != ' ' && c != '\t' {
			return s[i:]
		}
	}
	return ""
}

func trim(s string) string {
	// trim spaces e tabs do início e fim
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

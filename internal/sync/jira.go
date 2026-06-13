package sync

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// JiraClient encapsula credenciais para a API do Jira Cloud.
type JiraClient struct {
	BaseURL string // ex: "https://mycompany.atlassian.net"
	Email   string
	Token   string
	Project string // ex: "ENG"
}

// NewJiraClient cria um cliente Jira a partir de trackfw.yaml ou variáveis de ambiente.
// Ordem de busca: 1) trackfw.yaml (jira_base_url, jira_email, jira_token, jira_project)
//
//	2) env vars JIRA_BASE_URL, JIRA_EMAIL, JIRA_TOKEN, JIRA_PROJECT
func NewJiraClient() (*JiraClient, error) {
	baseURL := readConfigField("jira_base_url")
	if baseURL == "" {
		baseURL = os.Getenv("JIRA_BASE_URL")
	}
	email := readConfigField("jira_email")
	if email == "" {
		email = os.Getenv("JIRA_EMAIL")
	}
	token := readConfigField("jira_token")
	if token == "" {
		token = os.Getenv("JIRA_TOKEN")
	}
	project := readConfigField("jira_project")
	if project == "" {
		project = os.Getenv("JIRA_PROJECT")
	}

	if baseURL == "" {
		return nil, fmt.Errorf("Jira base URL not found. Set JIRA_BASE_URL env var or jira_base_url in trackfw.yaml")
	}
	if email == "" {
		return nil, fmt.Errorf("Jira email not found. Set JIRA_EMAIL env var or jira_email in trackfw.yaml")
	}
	if token == "" {
		return nil, fmt.Errorf("Jira API token not found. Set JIRA_TOKEN env var or jira_token in trackfw.yaml")
	}
	if project == "" {
		return nil, fmt.Errorf("Jira project key not found. Set JIRA_PROJECT env var or jira_project in trackfw.yaml")
	}

	return &JiraClient{
		BaseURL: baseURL,
		Email:   email,
		Token:   token,
		Project: project,
	}, nil
}

// CreateIssue cria uma issue do tipo Story no Jira e retorna o issue key (ex: "ENG-456").
func (c *JiraClient) CreateIssue(title, description string) (string, error) {
	payload := map[string]interface{}{
		"fields": map[string]interface{}{
			"project": map[string]string{
				"key": c.Project,
			},
			"summary": title,
			"description": map[string]interface{}{
				"type":    "doc",
				"version": 1,
				"content": []interface{}{
					map[string]interface{}{
						"type": "paragraph",
						"content": []interface{}{
							map[string]interface{}{
								"type": "text",
								"text": description,
							},
						},
					},
				},
			},
			"issuetype": map[string]string{
				"name": "Story",
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("jira: marshal request: %w", err)
	}

	url := c.BaseURL + "/rest/api/3/issue"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("jira: build request: %w", err)
	}

	// Basic Auth: base64(email:token)
	creds := base64.StdEncoding.EncodeToString([]byte(c.Email + ":" + c.Token))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("jira: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("jira: read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("jira: unexpected status %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("jira: parse response: %w", err)
	}

	if result.Key == "" {
		return "", fmt.Errorf("jira: response missing issue key")
	}

	return result.Key, nil
}


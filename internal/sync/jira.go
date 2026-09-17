package sync

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// jiraAllowMixedOriginEnv is the environment variable that permits the
// (config base_url, env token) combination. Setting it is an operator decision
// made in the CI environment — not in trackfw.yaml — so a PR that edits only
// the repository cannot trigger it.
//
// Name rationale: TRACKFW_ prefix avoids collision with Jira-vendor variables;
// JIRA_ALLOW_MIXED_ORIGIN names the specific combination permitted (not a
// generic --force or skip flag, which the AC explicitly warns against accreting).
const jiraAllowMixedOriginEnv = "TRACKFW_JIRA_ALLOW_MIXED_ORIGIN"

// JiraClient encapsula credenciais para a API do Jira Cloud.
type JiraClient struct {
	BaseURL    string // ex: "https://mycompany.atlassian.net"
	Email      string
	Token      string
	Project    string // ex: "ENG"
	httpClient *http.Client
}

// NewJiraClient cria um cliente Jira a partir de trackfw.yaml ou variáveis de ambiente.
// Ordem de busca: 1) trackfw.yaml (jira_base_url, jira_email, jira_token, jira_project)
//
//	2) env vars JIRA_BASE_URL, JIRA_EMAIL, JIRA_TOKEN, JIRA_PROJECT
//
// AC2: the combination (base_url from trackfw.yaml, token from JIRA_TOKEN env var) is
// refused by default. See jiraAllowMixedOriginEnv.
func NewJiraClient() (*JiraClient, error) {
	sc := config.Load().Sync

	baseURL := sc.JiraBaseURL
	if baseURL == "" {
		baseURL = os.Getenv("JIRA_BASE_URL")
	}
	email := sc.JiraEmail
	if email == "" {
		email = os.Getenv("JIRA_EMAIL")
	}
	token := sc.JiraToken
	if token == "" {
		token = os.Getenv("JIRA_TOKEN")
	}
	project := sc.JiraProject
	if project == "" {
		project = os.Getenv("JIRA_PROJECT")
	}

	// Track effective origin of base_url and token for AC2.
	// baseURLFromConfig: true when trackfw.yaml provides the value (config wins precedence).
	// tokenFromEnv: true when trackfw.yaml has no token AND JIRA_TOKEN env var is set.
	baseURLFromConfig := sc.JiraBaseURL != ""
	tokenFromEnv := sc.JiraToken == "" && os.Getenv("JIRA_TOKEN") != ""

	return newJiraClientFromSources(baseURL, email, token, project, baseURLFromConfig, tokenFromEnv)
}

// newJiraClientFromSources is the testable core; NewJiraClient is the config-reading wrapper.
// Separating construction from config loading makes all AC2/AC3 invariants unit-testable
// without os.Chdir or filesystem setup.
func newJiraClientFromSources(baseURL, email, token, project string, baseURLFromConfig, tokenFromEnv bool) (*JiraClient, error) {
	// Empty-value checks — must run before all other checks to preserve error message
	// byte-identity (TestNewJiraClient_ErrorMessagesPreserved in config_loader_test.go).
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

	// AC2: Refuse the (config base_url, env token) combination by default.
	//
	// This combination means a PR that edits only trackfw.yaml can redirect an
	// authenticated request to a chosen host — the CI token travels with the
	// destination change. The combination is indistinguishable from a legitimate
	// self-hosted Jira configuration (URL in repo, token in CI secret), so no
	// signal can separate "trusted config" from "config altered by PR" — the
	// refusal is the only safe default.
	//
	// To allow: set TRACKFW_JIRA_ALLOW_MIXED_ORIGIN=1 in the CI environment
	// (not in trackfw.yaml). That act belongs to the operator who controls CI,
	// not to whoever opens the PR.
	if baseURLFromConfig && tokenFromEnv {
		if os.Getenv(jiraAllowMixedOriginEnv) == "" {
			return nil, fmt.Errorf(
				"jira: base URL from trackfw.yaml (repository) combined with token from "+
					"JIRA_TOKEN (environment) is refused — a PR that edits only trackfw.yaml "+
					"can redirect the authenticated request to a chosen host. "+
					"To allow this combination, set %s=1 in your CI environment (not in trackfw.yaml)",
				jiraAllowMixedOriginEnv,
			)
		}
	}

	// AC3: Validate the URL — must be absolute and https.
	// Reuses the same url.Parse + scheme-check pattern as internal/thirdparty/fetch.go.
	parsedURL, err := validateJiraURL(baseURL)
	if err != nil {
		return nil, err
	}

	return &JiraClient{
		BaseURL:    baseURL, // preserve raw validated string; parsedURL.String() can alter port notation
		Email:      email,
		Token:      token,
		Project:    project,
		httpClient: newJiraHTTPClient(parsedURL),
	}, nil
}

// validateJiraURL parses rawURL and requires an absolute https URL.
// Follows the same pattern as internal/thirdparty/fetch.go (url.Parse + scheme check).
func validateJiraURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("jira: invalid base URL %q: %w", rawURL, err)
	}
	if !parsed.IsAbs() || parsed.Host == "" {
		return nil, fmt.Errorf("jira: base URL must be an absolute URL with a host (got %q)", rawURL)
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("jira: base URL scheme must be https (got %q in %q)", parsed.Scheme, rawURL)
	}
	return parsed, nil
}

// jiraNormalizeHost returns "hostname:port", filling in the default port when
// the URL omits it, to prevent false host-change alarms in CheckRedirect when
// the redirect uses an explicit default port (e.g. https://h:443 vs https://h).
func jiraNormalizeHost(u *url.URL) string {
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}
	return host + ":" + port
}

// newJiraHTTPClient returns a dedicated http.Client for Jira with a CheckRedirect
// that rejects any redirect whose scheme is not https OR whose normalized host:port
// differs from the original URL.
//
// Two redirect vectors measured in Wave 0
// (docs/portabilidade/2026-09-17-threat-model-jira-base-url.md §5):
//
//   - Cross-hostname redirect: Go stdlib already strips Authorization
//     (shouldCopyHeaderOnRedirect). AC3 (https + url.Parse) is sufficient for
//     this vector; CheckRedirect adds defence-in-depth.
//
//   - Same hostname, different port (e.g. jira.co → jira.co:8443): Authorization
//     is preserved by the stdlib. CheckRedirect (normalizeHost comparison) closes
//     this gap.
//
// http→https upgrade on the same host: unreachable from this client because
// validateJiraURL already requires https for the initial URL. A downgrade
// (https→http) is blocked by the scheme clause before the Authorization header
// can travel over clear text.
//
// Pattern reused from internal/thirdparty/fetch.go:30-39.
func newJiraHTTPClient(original *url.URL) *http.Client {
	origNorm := jiraNormalizeHost(original)
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.Scheme != "https" {
				return fmt.Errorf("jira: redirect to non-https URL refused: %s", req.URL.String())
			}
			if jiraNormalizeHost(req.URL) != origNorm {
				return fmt.Errorf("jira: redirect to different host refused: %s (original: %s)", req.URL.Host, original.Host)
			}
			return nil
		},
	}
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

	// AC3: URL is built via url.JoinPath (not string concatenation) from the
	// already-validated BaseURL. This prevents the AC7 anti-reintroduction gate
	// from firing and ensures the path is properly encoded.
	endpoint, err := url.JoinPath(c.BaseURL, "rest/api/3/issue")
	if err != nil {
		return "", fmt.Errorf("jira: build endpoint URL: %w", err)
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("jira: build request: %w", err)
	}

	// Basic Auth: base64(email:token)
	creds := base64.StdEncoding.EncodeToString([]byte(c.Email + ":" + c.Token))
	req.Header.Set("Authorization", "Basic "+creds)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := c.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
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

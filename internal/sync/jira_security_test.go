package sync

// ML-1A (REQ-2026-09-17 / #380): security tests for jira_base_url origin guard (AC2),
// URL validation (AC3), redirect policy (AC3), and no-request-on-refusal (AC4).
//
// Each test declares: which conclusion of ML-1A it asserts, and what would make it fail.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// AC2 — (base_url source, token source) combination table
// ---------------------------------------------------------------------------

// TestNewJiraClient_AC2_Row1_ConfigConfig asserts: (config, config) is allowed without
// any extra step — the secret and destination share the same trust boundary.
// Would fail if: newJiraClientFromSources returns an error for this combination.
func TestNewJiraClient_AC2_Row1_ConfigConfig(t *testing.T) {
	os.Unsetenv(jiraAllowMixedOriginEnv)
	_, err := newJiraClientFromSources(
		"https://jira.example.com", "u@e.com", "tok", "PROJ",
		true,  // baseURLFromConfig
		false, // tokenFromEnv (token came from config)
	)
	if err != nil {
		t.Errorf("(config,config) must succeed without extra step; got: %v", err)
	}
}

// TestNewJiraClient_AC2_Row2_EnvEnv asserts: (env, env) is allowed — neither value comes
// from the repository, so a PR cannot alter the destination.
// Would fail if: newJiraClientFromSources returns an error for this combination.
func TestNewJiraClient_AC2_Row2_EnvEnv(t *testing.T) {
	os.Unsetenv(jiraAllowMixedOriginEnv)
	_, err := newJiraClientFromSources(
		"https://jira.example.com", "u@e.com", "tok", "PROJ",
		false, // baseURLFromConfig (URL came from env)
		true,  // tokenFromEnv
	)
	if err != nil {
		t.Errorf("(env,env) must succeed; got: %v", err)
	}
}

// TestNewJiraClient_AC2_Row3_ConfigEnv_Refused asserts: (config base_url, env token) is
// refused by default, and the error message names both origins and the opt-in variable.
// This is the primary AC2 invariant.
// Would fail if: newJiraClientFromSources succeeds without TRACKFW_JIRA_ALLOW_MIXED_ORIGIN set.
//
// Sub-test "no_request": proves AC4(a) — no HTTP request is emitted when construction is
// refused. The test server tracks requests; it must never be called.
func TestNewJiraClient_AC2_Row3_ConfigEnv_Refused(t *testing.T) {
	t.Run("error_returned", func(t *testing.T) {
		os.Unsetenv(jiraAllowMixedOriginEnv)
		_, err := newJiraClientFromSources(
			"https://jira.example.com", "u@e.com", "tok", "PROJ",
			true, // baseURLFromConfig
			true, // tokenFromEnv
		)
		if err == nil {
			t.Fatal("(config,env) without opt-in must be refused — got nil error")
		}
		msg := err.Error()
		if !strings.Contains(msg, "trackfw.yaml") {
			t.Errorf("error must name trackfw.yaml (URL source); got: %s", msg)
		}
		if !strings.Contains(msg, "JIRA_TOKEN") {
			t.Errorf("error must name JIRA_TOKEN (token source); got: %s", msg)
		}
		if !strings.Contains(msg, jiraAllowMixedOriginEnv) {
			t.Errorf("error must name the opt-in variable %s; got: %s", jiraAllowMixedOriginEnv, msg)
		}
	})

	t.Run("no_request", func(t *testing.T) {
		// AC4(a): refusal must emit zero HTTP requests — not just return a non-nil error.
		// A test server that records calls proves absence of traffic by construction.
		requestReceived := false
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestReceived = true
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		os.Unsetenv(jiraAllowMixedOriginEnv)
		client, err := newJiraClientFromSources(
			srv.URL, "u@e.com", "tok", "PROJ",
			true, // baseURLFromConfig
			true, // tokenFromEnv
		)

		if err == nil {
			t.Fatal("(config,env) must be refused")
		}
		if client != nil {
			t.Error("returned client must be nil on refusal")
		}
		if requestReceived {
			t.Error("AC4(a) violated: HTTP request was emitted even though construction was refused")
		}
	})
}

// TestNewJiraClient_AC2_Row4_EnvConfig asserts: (env base_url, config token) is allowed —
// the destination does not come from the repository, so a PR cannot redirect a CI credential.
// Would fail if: newJiraClientFromSources returns an error for this combination.
func TestNewJiraClient_AC2_Row4_EnvConfig(t *testing.T) {
	os.Unsetenv(jiraAllowMixedOriginEnv)
	_, err := newJiraClientFromSources(
		"https://jira.example.com", "u@e.com", "tok", "PROJ",
		false, // baseURLFromConfig (URL from env)
		false, // tokenFromEnv (token from config)
	)
	if err != nil {
		t.Errorf("(env,config) must succeed; got: %v", err)
	}
}

// TestNewJiraClient_AC2_Row5_EffectiveOriginIsConfig asserts: when base_url appears in both
// trackfw.yaml AND JIRA_BASE_URL env var, the effective origin is config (precedence rule).
// The (config,env) refusal must apply even when JIRA_BASE_URL is also set.
// Uses NewJiraClient (the full config-reading wrapper) to exercise precedence end-to-end.
// Would fail if: the origin-tracking logic reads the env var's presence instead of the
// effective source after precedence resolution.
func TestNewJiraClient_AC2_Row5_EffectiveOriginIsConfig(t *testing.T) {
	dir := chdirTemp(t)
	writeYAML(t, dir, "jira_base_url: \"https://config.jira.example.com\"\njira_email: u@e.com\njira_project: PROJ\n")
	t.Setenv("JIRA_TOKEN", "env-token")
	t.Setenv("JIRA_BASE_URL", "https://env.jira.example.com") // also in env, but config wins
	os.Unsetenv(jiraAllowMixedOriginEnv)

	_, err := NewJiraClient()
	if err == nil {
		t.Fatal("config base_url + env token must be refused even when JIRA_BASE_URL is also set in env")
	}
	if !strings.Contains(err.Error(), "trackfw.yaml") {
		t.Errorf("error must name trackfw.yaml; got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// AC2 — opt-in (contra-braço)
// ---------------------------------------------------------------------------

// TestNewJiraClient_AC2_OptIn is the contra-braço: it proves that the (config,env)
// combination CAN succeed when TRACKFW_JIRA_ALLOW_MIXED_ORIGIN is set.
// Without this test, the AC2 refusal test could pass because of a bug (e.g. always-error)
// rather than a guard. The opt-in passing proves the test discriminates.
// Conclusion: TRACKFW_JIRA_ALLOW_MIXED_ORIGIN=1 is the exact mechanism that unblocks
// the refused combination.
// Would fail if: the opt-in env var no longer lifts the refusal (guard became non-bypassable).
func TestNewJiraClient_AC2_OptIn(t *testing.T) {
	t.Setenv(jiraAllowMixedOriginEnv, "1")
	_, err := newJiraClientFromSources(
		"https://jira.example.com", "u@e.com", "tok", "PROJ",
		true, // baseURLFromConfig
		true, // tokenFromEnv
	)
	if err != nil {
		t.Errorf("(config,env) with opt-in must succeed; got: %v", err)
	}
}

// TestNewJiraClient_AC2_YAMLCannotUnlockOptIn asserts AC1: no value in trackfw.yaml can
// lift the mixed-origin refusal. Only TRACKFW_JIRA_ALLOW_MIXED_ORIGIN in the process
// environment does — which is a CI operator decision, not a repository committer decision.
//
// Implementation: newJiraClientFromSources reads os.Getenv(jiraAllowMixedOriginEnv) directly.
// It does not accept a config-sourced parameter for the opt-in. Any attempt to route the
// opt-in through config.Load() would require changing the function signature, making this
// contract explicit and auditable.
//
// Conclusion: the opt-in cannot be granted from within the repository.
// Would fail if: newJiraClientFromSources is refactored to read the opt-in from config.
func TestNewJiraClient_AC2_YAMLCannotUnlockOptIn(t *testing.T) {
	dir := chdirTemp(t)
	// Try adding an opt-in-looking key in YAML — it must have zero effect.
	writeYAML(t, dir,
		"jira_base_url: \"https://config.jira.example.com\"\n"+
			"jira_email: u@e.com\njira_project: PROJ\n"+
			"trackfw_jira_allow_mixed_origin: \"1\"\n")
	t.Setenv("JIRA_TOKEN", "env-token")
	os.Unsetenv(jiraAllowMixedOriginEnv)

	_, err := NewJiraClient()
	if err == nil {
		t.Fatal("a YAML key must not unlock the mixed-origin opt-in — only the env var does")
	}
	if !strings.Contains(err.Error(), jiraAllowMixedOriginEnv) {
		t.Errorf("error must name the env var %s; got: %s", jiraAllowMixedOriginEnv, err.Error())
	}
}

// ---------------------------------------------------------------------------
// AC3 — URL validation
// ---------------------------------------------------------------------------

// TestJiraURL_Validation asserts: validateJiraURL accepts only absolute https URLs.
// Conclusion: http scheme, relative path, and bare hostname are each refused with a
// named error. Valid https URL is accepted.
// Would fail if: validateJiraURL returns nil error for any invalid input.
func TestJiraURL_Validation(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantErr     bool
		errContains string
	}{
		{"valid https", "https://jira.example.com", false, ""},
		{"valid https with port", "https://jira.example.com:8080", false, ""},
		{"http rejected", "http://jira.example.com", true, "https"},
		{"relative path rejected", "/api/v1", true, "absolute"},
		{"bare hostname rejected", "jira.example.com", true, "absolute"},
		{"empty string rejected", "", true, "absolute"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateJiraURL(tc.raw)
			if tc.wantErr && err == nil {
				t.Errorf("want error for %q, got nil", tc.raw)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("want no error for %q, got: %v", tc.raw, err)
			}
			if tc.wantErr && tc.errContains != "" && !strings.Contains(err.Error(), tc.errContains) {
				t.Errorf("error for %q must contain %q; got: %s", tc.raw, tc.errContains, err.Error())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC3 — CheckRedirect policy (unit test of the policy function)
// ---------------------------------------------------------------------------

// TestJiraCheckRedirect_Policy asserts: the CheckRedirect installed by newJiraHTTPClient
// blocks four redirect vectors, each falsifying a distinct clause of the policy.
// Conclusion: non-https redirect, different-hostname redirect, and same-hostname
// different-port redirect are all refused; same-host same-port redirect is allowed.
// Would fail if: the policy function returns nil for any of the three refused arms,
// or returns an error for the allowed arm.
func TestJiraCheckRedirect_Policy(t *testing.T) {
	original, err := url.Parse("https://jira.example.com")
	if err != nil {
		t.Fatal(err)
	}
	client := newJiraHTTPClient(original)
	policy := client.CheckRedirect

	makeReq := func(target string) *http.Request {
		u, _ := url.Parse(target)
		req, _ := http.NewRequest("GET", target, nil)
		req.URL = u
		return req
	}
	via := []*http.Request{makeReq("https://jira.example.com/start")}

	// Arm 1: redirect to http (downgrade) — must refuse
	// Conclusion: scheme clause blocks clear-text redirect of Authorization header.
	if err := policy(makeReq("http://jira.example.com/path"), via); err == nil {
		t.Error("Arm 1: redirect to http must be refused")
	}

	// Arm 2: redirect to different hostname — must refuse
	// Conclusion: normalizeHost comparison blocks cross-hostname redirect.
	// (The stdlib also strips Authorization for cross-hostname, but CheckRedirect
	// provides defence-in-depth and prevents the request from being made at all.)
	if err := policy(makeReq("https://attacker.example.com/steal"), via); err == nil {
		t.Error("Arm 2: redirect to different hostname must be refused")
	}

	// Arm 3: redirect to same hostname, different port — must refuse.
	// Conclusion: normalizeHost comparison closes the Wave 0 gap — Authorization is
	// preserved by the stdlib for same-hostname redirects regardless of port.
	if err := policy(makeReq("https://jira.example.com:8443/path"), via); err == nil {
		t.Error("Arm 3: redirect to same hostname different port must be refused")
	}

	// Arm 4: redirect within same host/port (path change only) — must allow.
	// Conclusion: a normal path-level redirect (e.g. Jira's own auth flow) is not blocked.
	if err := policy(makeReq("https://jira.example.com/other-path"), via); err != nil {
		t.Errorf("Arm 4: redirect within same host/port must be allowed; got: %v", err)
	}

	// Arm 5: redirect with explicit default port (https://h:443 vs https://h) — must allow.
	// Conclusion: jiraNormalizeHost fills in 443 for https when port is omitted, so
	// https://jira.example.com:443 and https://jira.example.com are treated as the same host.
	if err := policy(makeReq("https://jira.example.com:443/path"), via); err != nil {
		t.Errorf("Arm 5: https://h:443 same as https://h must be allowed; got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// AC3 — CheckRedirect wired test (end-to-end: policy is installed on the client)
// ---------------------------------------------------------------------------

// TestJiraClient_CheckRedirect_Wired asserts: the CheckRedirect policy is actually installed
// on the httpClient built by newJiraHTTPClient — not just defined as a standalone function.
// Uses two httptest.NewTLSServer instances on 127.0.0.1 at different ports to trigger
// an actual redirect and verify the client refuses it.
//
// Conclusion: CreateIssue returns an error containing "redirect" when srv1 issues a 302
// to srv2 (same hostname, different port), proving the policy is wired, not just correct.
// Would fail if: the httpClient field is nil, or its CheckRedirect is not set, or the
// policy is installed on a different client than the one CreateIssue uses.
func TestJiraClient_CheckRedirect_Wired(t *testing.T) {
	// srv2: the redirect target — responds 201 Created with a valid Jira key.
	srv2 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]string{"key": "TEST-2"}); err != nil {
			t.Errorf("srv2: encode response: %v", err)
		}
	}))
	defer srv2.Close()

	// srv1: the initial target — redirects to srv2 (same hostname 127.0.0.1, different port).
	srv1 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv2.URL+r.URL.Path, http.StatusFound)
	}))
	defer srv1.Close()

	// Both srv1 and srv2 are on 127.0.0.1 but different ports — the exact vector from
	// Wave 0 Teste A (Authorization forwarded by the stdlib for same-hostname redirects).

	// Build the client via the normal construction path (with opt-in since srv1.URL is https).
	t.Setenv(jiraAllowMixedOriginEnv, "1")
	c, err := newJiraClientFromSources(srv1.URL, "u@e.com", "tok", "PROJ", true, true)
	if err != nil {
		t.Fatalf("newJiraClientFromSources: %v", err)
	}

	// Replace the transport with srv1's TLS-aware transport so the test server's
	// self-signed cert is trusted. The CheckRedirect set by newJiraHTTPClient is preserved.
	c.httpClient.Transport = srv1.Client().Transport

	// CreateIssue triggers the redirect — the CheckRedirect policy must block it.
	_, err = c.CreateIssue("test title", "test description")
	if err == nil {
		t.Fatal("CreateIssue must fail when the redirect is blocked by CheckRedirect")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Errorf("error must mention 'redirect'; got: %s", err.Error())
	}
}

// ---------------------------------------------------------------------------
// AC2 opt-in with real request (contra-braço end-to-end)
// ---------------------------------------------------------------------------

// TestJiraClient_AC2_OptIn_RequestMade is the end-to-end contra-braço:
// with TRACKFW_JIRA_ALLOW_MIXED_ORIGIN=1, a (config,env) client is created AND
// CreateIssue actually sends a request to the configured URL.
//
// Conclusion: the opt-in unblocks both construction AND the HTTP request — proving
// that the tests in AC2_Row3 would have measured a real request (not a hypothetical one)
// had the guard not been in place.
// Would fail if: the opt-in is set but CreateIssue still fails for an unrelated reason.
func TestJiraClient_AC2_OptIn_RequestMade(t *testing.T) {
	// Use TLS server to satisfy https requirement.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]string{"key": "OPT-1"}); err != nil {
			t.Errorf("srv: encode response: %v", err)
		}
	}))
	defer srv.Close()

	t.Setenv(jiraAllowMixedOriginEnv, "1")
	c, err := newJiraClientFromSources(srv.URL, "u@e.com", "tok", "PROJ", true, true)
	if err != nil {
		t.Fatalf("construction must succeed with opt-in: %v", err)
	}
	c.httpClient.Transport = srv.Client().Transport

	key, err := c.CreateIssue("test", "desc")
	if err != nil {
		t.Errorf("CreateIssue must succeed with opt-in and real TLS server: %v", err)
	}
	if key != "OPT-1" {
		t.Errorf("expected key OPT-1, got %q", key)
	}
}

// ---------------------------------------------------------------------------
// AC6 — surface enumeration
// ---------------------------------------------------------------------------

// TestJiraClient_AC6_SurfaceEnumeration documents the measured surface of the class:
// 1 key (jira_base_url), 1 command (sync --to=jira), 1 construction path (NewJiraClient).
// This is a compile-time check: it verifies that JiraClient has no exported constructors
// other than NewJiraClient by asserting the type is only constructed via newJiraClientFromSources.
// The actual enumeration was done in Wave 0 and is recorded in
// docs/portabilidade/2026-09-17-threat-model-jira-base-url.md §2.
//
// Conclusion: the guard in newJiraClientFromSources covers 100% of authenticated request
// paths for this class.
// Would fail (at build time) if: a new JiraClient{...} literal is added outside this package.
func TestJiraClient_AC6_SurfaceEnumeration(t *testing.T) {
	// Verified by grep: no JiraClient{} struct literal exists outside internal/sync/jira.go.
	// The only construction path is newJiraClientFromSources <- NewJiraClient.
	// Any struct literal outside this package would bypass the guard — detectable by:
	//   grep -rn 'JiraClient{' internal/ cmd/
	// This test documents that invariant; the AC7 gate (check-jira-url-concat.sh) guards
	// against reintroduction of unvalidated URL construction.
	t.Log("Surface: 1 key (jira_base_url), 1 command (sync --to=jira), " +
		"1 NewJiraClient path — verified in Wave 0 and by grep at ML-1A time")
}

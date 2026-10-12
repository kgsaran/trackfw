package sync

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/netdeadline"
)

// buildLinearSuccessResponse returns the JSON body a well-behaved Linear API would send
// for a successful issueCreate.
func buildLinearSuccessResponse(identifier string) string {
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"issueCreate": map[string]interface{}{
				"success": true,
				"issue": map[string]interface{}{
					"id":         "abc-123",
					"identifier": identifier,
				},
			},
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

// TestLinearCreateIssue_Timeout asserts: a CreateIssue call to a server that never
// responds is cancelled by TRACKFW_NETWORK_TIMEOUT and returns an error that satisfies
// errors.Is(err, netdeadline.ErrTimeout) with Op="linear API".
func TestLinearCreateIssue_Timeout(t *testing.T) {
	t.Setenv(netdeadline.NetworkTimeoutEnv, "1s")

	// Slow handler: blocks until the server closes the connection (r.Context().Done)
	// or 5 s pass — whichever comes first.  The 5 s fallback bounds goroutine lifetime
	// in case the HTTP server does not propagate client disconnect to r.Context().
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer func() {
		// Force-close active connections so the CreateIssue goroutine unblocks even
		// in the mordida scenario (http.DefaultClient, no context deadline).
		srv.CloseClientConnections()
		srv.Close()
	}()

	c := &LinearClient{APIKey: "key-test", TeamID: "team-test", Endpoint: srv.URL}

	type result struct {
		id  string
		err error
	}
	done := make(chan result, 1)
	go func() {
		id, err := c.CreateIssue("title", "description")
		done <- result{id, err}
	}()

	var r result
	select {
	case r = <-done:
		// received in time
	case <-time.After(10 * time.Second):
		t.Fatal("CreateIssue did not return within 10s — context deadline was not applied")
	}

	if r.id != "" {
		t.Errorf("expected empty identifier on timeout, got %q", r.id)
	}
	if !errors.Is(r.err, netdeadline.ErrTimeout) {
		t.Errorf("errors.Is(err, netdeadline.ErrTimeout) = false; err = %v", r.err)
	}

	var te *netdeadline.TimeoutError
	if !errors.As(r.err, &te) {
		t.Fatalf("expected *netdeadline.TimeoutError in chain; err = %v", r.err)
	}
	if te.Op != "linear API" {
		t.Errorf("TimeoutError.Op = %q; want %q", te.Op, "linear API")
	}
	if te.Limit != time.Second {
		t.Errorf("TimeoutError.Limit = %s; want 1s", te.Limit)
	}
	if te.Var != netdeadline.NetworkTimeoutEnv {
		t.Errorf("TimeoutError.Var = %q; want %q", te.Var, netdeadline.NetworkTimeoutEnv)
	}
	if !strings.Contains(r.err.Error(), netdeadline.NetworkTimeoutEnv) {
		t.Errorf("error message does not name %s: %v", netdeadline.NetworkTimeoutEnv, r.err)
	}
}

// TestLinearCreateIssue_Success asserts: a CreateIssue call to a well-behaved server
// returns the identifier without error, proving the happy path is unchanged.
func TestLinearCreateIssue_Success(t *testing.T) {
	wantIdentifier := "ENG-42"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method, content-type and authorization header.
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q; want application/json", ct)
		}
		if auth := r.Header.Get("Authorization"); auth != "key-success" {
			t.Errorf("Authorization = %q; want key-success", auth)
		}

		// Verify teamId reaches the server.
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		vars, _ := body["variables"].(map[string]interface{})
		if tid, _ := vars["teamId"].(string); tid != "team-success" {
			t.Errorf("teamId in body = %q; want team-success", tid)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(buildLinearSuccessResponse(wantIdentifier)))
	}))
	defer srv.Close()

	c := &LinearClient{APIKey: "key-success", TeamID: "team-success", Endpoint: srv.URL}

	id, err := c.CreateIssue("Test issue", "Test description")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != wantIdentifier {
		t.Errorf("identifier = %q; want %q", id, wantIdentifier)
	}
}

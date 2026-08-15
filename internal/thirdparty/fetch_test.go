package thirdparty

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetch_RefusesPlainHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Fetch must refuse http:// before making any request")
	}))
	defer srv.Close()

	// srv.URL is http://... — swap scheme is not needed, srv.URL already
	// starts with http://.
	_, err := Fetch(srv.URL)
	if err == nil {
		t.Fatal("expected error for http:// URL, got nil")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected error to mention https requirement, got: %v", err)
	}
}

func TestFetch_RefusesNonHTTPScheme(t *testing.T) {
	_, err := Fetch("ftp://example.com/skill.md")
	if err == nil {
		t.Fatal("expected error for ftp:// URL, got nil")
	}
	if !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected error to mention https requirement, got: %v", err)
	}
}

func TestFetch_RefusesRedirectDowngradeToHTTP(t *testing.T) {
	// A plain-http target that the https server will redirect to.
	insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downgraded http target must never be reached")
	}))
	defer insecure.Close()

	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, insecure.URL, http.StatusFound)
	}))
	defer secure.Close()

	client := secure.Client()
	client.Timeout = fetchClient.Timeout
	client.CheckRedirect = fetchClient.CheckRedirect
	old := fetchClient
	fetchClient = client
	defer func() { fetchClient = old }()

	_, err := Fetch(secure.URL)
	if err == nil {
		t.Fatal("expected error for redirect downgrading to http, got nil")
	}
}

func TestFetch_RefusesThirdRedirect(t *testing.T) {
	var mux *http.ServeMux
	var srv *httptest.Server

	hopCount := 0
	mux = http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		hopCount++
		next := fmt.Sprintf("%s/hop%d", srv.URL, hopCount)
		if hopCount > 5 {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("done"))
			return
		}
		http.Redirect(w, r, next, http.StatusFound)
	})
	srv = httptest.NewTLSServer(mux)
	defer srv.Close()

	client := srv.Client()
	client.Timeout = fetchClient.Timeout
	client.CheckRedirect = fetchClient.CheckRedirect
	old := fetchClient
	fetchClient = client
	defer func() { fetchClient = old }()

	_, err := Fetch(srv.URL)
	if err == nil {
		t.Fatal("expected error after exceeding max redirects, got nil")
	}
	if !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("expected error to mention redirects, got: %v", err)
	}
}

func TestFetch_RefusesContentOverSizeLimit(t *testing.T) {
	oversized := bytes.Repeat([]byte("a"), maxContentSize+1)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write(oversized)
	}))
	defer srv.Close()

	client := srv.Client()
	client.Timeout = fetchClient.Timeout
	client.CheckRedirect = fetchClient.CheckRedirect
	old := fetchClient
	fetchClient = client
	defer func() { fetchClient = old }()

	_, err := Fetch(srv.URL)
	if err == nil {
		t.Fatal("expected error for content exceeding size limit, got nil")
	}
	if !strings.Contains(err.Error(), "2097152") && !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected error to mention the size limit, got: %v", err)
	}
}

// TestFetch_RefusesNon200Status covers the resp.StatusCode != http.StatusOK
// branch in Fetch — present in the Go implementation since ML-2A, but never
// exercised by a test (hefesto-tf finding, ML-4B/ML-4C): a server that
// responds with a non-redirect, non-200 status (e.g. 404, 500) must be
// refused, not silently treated as success.
func TestFetch_RefusesNon200Status(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	client := srv.Client()
	client.Timeout = fetchClient.Timeout
	client.CheckRedirect = fetchClient.CheckRedirect
	old := fetchClient
	fetchClient = client
	defer func() { fetchClient = old }()

	_, err := Fetch(srv.URL)
	if err == nil {
		t.Fatal("expected error for HTTP 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected error to mention the HTTP status code, got: %v", err)
	}
}

func TestFetch_RefusesDisallowedContentType(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html></html>"))
	}))
	defer srv.Close()

	client := srv.Client()
	client.Timeout = fetchClient.Timeout
	client.CheckRedirect = fetchClient.CheckRedirect
	old := fetchClient
	fetchClient = client
	defer func() { fetchClient = old }()

	_, err := Fetch(srv.URL)
	if err == nil {
		t.Fatal("expected error for text/html Content-Type, got nil")
	}
	if !strings.Contains(err.Error(), "Content-Type") {
		t.Fatalf("expected error to mention Content-Type, got: %v", err)
	}
}

func TestFetch_AcceptsAllowedContentTypesWithCharset(t *testing.T) {
	for _, ct := range []string{"text/plain", "text/markdown", "text/x-markdown", "text/markdown; charset=utf-8"} {
		ct := ct
		t.Run(ct, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", ct)
				w.Write([]byte("# Hello\n\nbenign content\n"))
			}))
			defer srv.Close()

			client := srv.Client()
			client.Timeout = fetchClient.Timeout
			client.CheckRedirect = fetchClient.CheckRedirect
			old := fetchClient
			fetchClient = client
			defer func() { fetchClient = old }()

			body, err := Fetch(srv.URL)
			if err != nil {
				t.Fatalf("expected no error for Content-Type %q, got: %v", ct, err)
			}
			if !bytes.Contains(body, []byte("benign content")) {
				t.Fatalf("expected fetched body to contain benign content, got: %q", body)
			}
		})
	}
}

package kovaloopcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClaimLinkPostsCanonicalIdAndPrintsLinks(t *testing.T) {
	home := writeLocalKovaloopProfile(t, t.TempDir(), "kloop_agent_TEST", "OntologyAgent")

	var posted map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ledger/claims/link" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"agentId":   "kloop_agent_TEST",
			"claimCode": "clm_testclaim",
			"claimUrl":  "https://ledger.example.test/dashboard?claimCode=clm_testclaim&agentId=kloop_agent_TEST",
			"agentUrl":  "https://ledger.example.test/dashboard?agentId=kloop_agent_TEST",
		})
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"claim", "link"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_LEDGER_URL": server.URL,
		"KOVALOOP_HOME":       home,
	})

	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	// Only the canonical agentId + name are sent; no email field.
	want := map[string]any{"agentId": "kloop_agent_TEST", "agentName": "OntologyAgent"}
	if len(posted) != len(want) || posted["agentId"] != want["agentId"] || posted["agentName"] != want["agentName"] {
		t.Fatalf("posted = %#v", posted)
	}
	for _, w := range []string{
		"Agent ID:   kloop_agent_TEST",
		"Claim Code: clm_testclaim",
		"Claim Link: https://ledger.example.test/dashboard?claimCode=clm_testclaim&agentId=kloop_agent_TEST",
	} {
		if !strings.Contains(stdout.String(), w) {
			t.Fatalf("stdout missing %q: %s", w, stdout.String())
		}
	}
}

func TestClaimLinkWithoutLocalProfileReturnsExitCodeTwo(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"claim", "link"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_HOME": t.TempDir(), // no .kovaloop/profile.json
	})

	if exitCode != 2 {
		t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "kovaloop profile create") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestClaimLinkHTTPFailureReturnsNonZeroReadableError(t *testing.T) {
	home := writeLocalKovaloopProfile(t, t.TempDir(), "kloop_agent_TEST", "OntologyAgent")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not today", http.StatusTeapot)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"claim", "link"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_LEDGER_URL": server.URL,
		"KOVALOOP_HOME":       home,
	})

	if exitCode == 0 || exitCode == 2 {
		t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	for _, want := range []string{"ledger request failed", "HTTP 418", "not today"} {
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("stderr = %q, want substring %q", stderr.String(), want)
		}
	}
}

func TestPostJSONDoesNotMutateOutputWhenPrimaryReturnsInvalidJSON(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"agentId":"agent_primary",`)
	}))
	defer primary.Close()

	response := ClaimResponse{AgentID: "agent_existing"}
	err := postJSON(Config{
		LedgerURL: primary.URL,
	}, "/ledger/claims/link", ClaimRequest{AgentID: "agent_sender"}, &response)

	if err == nil {
		t.Fatal("postJSON returned nil error")
	}
	if !strings.Contains(err.Error(), "ledger response was not valid JSON") {
		t.Fatalf("err = %v", err)
	}
	if response.AgentID != "agent_existing" {
		t.Fatalf("response was mutated: %#v", response)
	}
}

func TestPostJSONDoesNotMutateOutputWhenPrimaryReturnsSchemaInvalidJSON(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"agentId":123}`)
	}))
	defer primary.Close()

	response := ClaimResponse{AgentID: "agent_existing"}
	err := postJSON(Config{LedgerURL: primary.URL}, "/ledger/claims/link", ClaimRequest{AgentID: "agent_sender"}, &response)

	if err == nil {
		t.Fatal("postJSON returned nil error")
	}
	if !strings.Contains(err.Error(), "ledger response was not valid JSON") {
		t.Fatalf("err = %v", err)
	}
	if response.AgentID != "agent_existing" {
		t.Fatalf("response was mutated: %#v", response)
	}
}

func TestGetJSONReturnsBodyReadErrors(t *testing.T) {
	originalClient := httpClient
	httpClient = &http.Client{Transport: readErrorTransport{}}
	defer func() {
		httpClient = originalClient
	}()

	var response map[string]bool
	err := getJSON(Config{LedgerURL: "http://ledger.example.test"}, "/health", &response)

	if err == nil {
		t.Fatal("getJSON returned nil error")
	}
	if !strings.Contains(err.Error(), "read failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestGetJSONSetsAcceptHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "application/json, text/event-stream" {
			t.Fatalf("Accept = %q", got)
		}
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer server.Close()

	var response map[string]bool
	if err := getJSON(Config{LedgerURL: server.URL}, "/health", &response); err != nil {
		t.Fatal(err)
	}
	if !response["ok"] {
		t.Fatalf("response = %#v", response)
	}
}

type readErrorTransport struct{}

func (readErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       errReader{},
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

type errReader struct{}

func (errReader) Read(p []byte) (int, error) {
	return 0, errors.New("read failed")
}

func (errReader) Close() error {
	return nil
}

var _ io.ReadCloser = errReader{}

package kovaloopcli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaimLinkPostsProfileAndPrintsLinks(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.json")
	err := os.WriteFile(profilePath, []byte(`{"email":"sender@example.com","agent_id":"agent_sender","agent_name":"Sender"}`), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	var posted ClaimRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ledger/claims/link" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/json, text/event-stream" {
			t.Fatalf("Accept = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"agentId":   "agent_sender",
			"claimCode": "clm_testclaim",
			"claimUrl":  "https://ledger.example.test/dashboard?claimCode=clm_testclaim&agentId=agent_sender",
			"agentUrl":  "https://ledger.example.test/dashboard?agentId=agent_sender",
		})
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"claim", "link"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_LEDGER_HTTP_URL":    server.URL,
		"KOVALOOP_AGENT_PROFILE_PATH": profilePath,
	})

	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if posted.AgentID != "agent_sender" || posted.Email != "sender@example.com" {
		t.Fatalf("posted = %#v", posted)
	}
	for _, want := range []string{
		"Agent ID:   agent_sender",
		"Claim Code: clm_testclaim",
		"Claim Link: https://ledger.example.test/dashboard?claimCode=clm_testclaim&agentId=agent_sender",
		"Agent Link: https://ledger.example.test/dashboard?agentId=agent_sender",
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("stdout missing %q: %s", want, stdout.String())
		}
	}
	if stderr.String() != "" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestClaimLinkProfileValidationReturnsExitCodeTwo(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(profilePath, []byte(`{"email":"owner@example.com"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"claim", "link"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_AGENT_PROFILE_PATH": profilePath,
	})

	if exitCode != 2 {
		t.Fatalf("exit code = %d, stderr=%q", exitCode, stderr.String())
	}
	if !strings.Contains(stderr.String(), "current OpenClaw profile is missing agent_id") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestClaimLinkHTTPFailureReturnsNonZeroReadableError(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(profilePath, []byte(`{"email":"sender@example.com","agent_id":"agent_sender"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not today", http.StatusTeapot)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"claim", "link"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_LEDGER_HTTP_URL":    server.URL,
		"KOVALOOP_AGENT_PROFILE_PATH": profilePath,
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

func TestPostJSONRetriesFallbackWhenPrimaryRequestFails(t *testing.T) {
	fallbackCalls := 0
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls++
		if r.URL.Path != "/ledger/claims/link" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/json, text/event-stream" {
			t.Fatalf("Accept = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q", got)
		}
		_ = json.NewEncoder(w).Encode(ClaimResponse{AgentID: "agent_sender"})
	}))
	defer fallback.Close()

	var response ClaimResponse
	err := postJSON(Config{
		LedgerURL:      "http://127.0.0.1:1",
		LedgerFallback: fallback.URL,
	}, "/ledger/claims/link", ClaimRequest{AgentID: "agent_sender"}, &response)

	if err != nil {
		t.Fatal(err)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls = %d", fallbackCalls)
	}
	if response.AgentID != "agent_sender" {
		t.Fatalf("response = %#v", response)
	}
}

func TestPostJSONRetriesFallbackWhenPrimaryReturnsHTTPFailure(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "primary unavailable", http.StatusBadGateway)
	}))
	defer primary.Close()

	fallbackCalls := 0
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls++
		_ = json.NewEncoder(w).Encode(ClaimResponse{AgentID: "agent_fallback"})
	}))
	defer fallback.Close()

	var response ClaimResponse
	err := postJSON(Config{
		LedgerURL:      primary.URL,
		LedgerFallback: fallback.URL,
	}, "/ledger/claims/link", ClaimRequest{AgentID: "agent_sender"}, &response)

	if err != nil {
		t.Fatal(err)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls = %d", fallbackCalls)
	}
	if response.AgentID != "agent_fallback" {
		t.Fatalf("response = %#v", response)
	}
}

func TestPostJSONDoesNotRetryFallbackOrMutateOutputWhenPrimaryReturnsInvalidJSON(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"agentId":"agent_primary",`)
	}))
	defer primary.Close()

	fallbackCalls := 0
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls++
		_ = json.NewEncoder(w).Encode(ClaimResponse{AgentID: "agent_fallback"})
	}))
	defer fallback.Close()

	response := ClaimResponse{AgentID: "agent_existing"}
	err := postJSON(Config{
		LedgerURL:      primary.URL,
		LedgerFallback: fallback.URL,
	}, "/ledger/claims/link", ClaimRequest{AgentID: "agent_sender"}, &response)

	if err == nil {
		t.Fatal("postJSON returned nil error")
	}
	if !strings.Contains(err.Error(), "ledger response was not valid JSON") {
		t.Fatalf("err = %v", err)
	}
	if fallbackCalls != 0 {
		t.Fatalf("fallback calls = %d", fallbackCalls)
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

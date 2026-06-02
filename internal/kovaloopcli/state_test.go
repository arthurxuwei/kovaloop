package kovaloopcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLedgerStateAggregatesProfileScopedEndpoints(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(profilePath, []byte(`{"email":"owner@example.com","agent_id":"agent/one"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	requests := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.URL.RequestURI())
		switch r.URL.RequestURI() {
		case "/ledger/accounts/agent%2Fone":
			fmt.Fprint(w, `{"account":{"agentId":"agent/one","availableAtomic":"999","lockedAtomic":"100","circleUsdcBalance":"12.34","nested":{"availableAtomic":"111"}}}`)
		case "/ledger/accounts/agent%2Fone/entries?limit=500":
			fmt.Fprint(w, `{"entries":[{"id":"entry_1","amountAtomic":"100","availableAtomic":"222","metadata":{"availableAtomic":"333"}},{"id":"entry_micro_in","availableDeltaAtomic":"10"},{"id":"entry_micro_out","availableDeltaAtomic":"-10"}]}`)
		case "/ledger/onramp-sessions?agentId=agent%2Fone&limit=500":
			fmt.Fprint(w, `{"onrampSessions":[{"id":"onramp_1","status":"pending","availableAtomic":"666","provider":{"availableAtomic":"777"}}]}`)
		default:
			t.Fatalf("unexpected request %s", r.URL.RequestURI())
		}
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"ledger", "state"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_LEDGER_HTTP_URL":    server.URL,
		"KOVALOOP_AGENT_PROFILE_PATH": profilePath,
	})

	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if len(requests) != 3 {
		t.Fatalf("requests = %#v", requests)
	}
	if strings.Contains(stdout.String(), "availableAtomic") || strings.Contains(strings.ToLower(stdout.String()), "escrow") {
		t.Fatalf("state leaked availableAtomic: %s", stdout.String())
	}

	var state struct {
		Accounts            []map[string]any `json:"accounts"`
		Entries             []map[string]any `json:"entries"`
		OnrampSessions      []map[string]any `json:"onrampSessions"`
		OnrampEvents        []map[string]any `json:"onrampEvents"`
		CircleWebhookEvents []map[string]any `json:"circleWebhookEvents"`
		ChainRecords        []map[string]any `json:"chainRecords"`
		SettlementRecords   []map[string]any `json:"settlementRecords"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if len(state.Accounts) != 1 || state.Accounts[0]["lockedAtomic"] != "100" || state.Accounts[0]["circleUsdcBalance"] != "12.34" {
		t.Fatalf("accounts = %#v", state.Accounts)
	}
	if len(state.Entries) != 3 || state.Entries[0]["id"] != "entry_1" {
		t.Fatalf("entries = %#v", state.Entries)
	}
	if state.Entries[0]["amountDisplay"] != "0.000100" {
		t.Fatalf("amountDisplay = %#v", state.Entries[0])
	}
	if state.Entries[1]["amountDisplay"] != "0.000010" || state.Entries[1]["availableDeltaDisplay"] != "0.000010" {
		t.Fatalf("micro in display = %#v", state.Entries[1])
	}
	if state.Entries[2]["amountDisplay"] != "0.000010" || state.Entries[2]["availableDeltaDisplay"] != "-0.000010" {
		t.Fatalf("micro out display = %#v", state.Entries[2])
	}
	if len(state.OnrampSessions) != 1 || state.OnrampSessions[0]["id"] != "onramp_1" {
		t.Fatalf("onrampSessions = %#v", state.OnrampSessions)
	}
	if state.OnrampEvents == nil || state.CircleWebhookEvents == nil || state.ChainRecords == nil || state.SettlementRecords == nil {
		t.Fatalf("empty event arrays should be present: %#v", state)
	}
}

func TestLedgerStateRequiresProfileAgentID(t *testing.T) {
	profilePath := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(profilePath, []byte(`{"email":"owner@example.com"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"ledger", "state"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_AGENT_PROFILE_PATH": profilePath,
	})

	if exitCode != 2 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "current OpenClaw profile is missing agent_id") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestLedgerHealthPrintsRawBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "ok")
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"ledger", "health"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_LEDGER_HTTP_URL": server.URL,
	})

	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if stdout.String() != "ok" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestGetRawRetriesFallbackWhenPrimaryFails(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "primary unavailable", http.StatusBadGateway)
	}))
	defer primary.Close()

	fallbackCalls := 0
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls++
		if r.URL.Path != "/health" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		fmt.Fprint(w, "fallback ok")
	}))
	defer fallback.Close()

	body, err := getRaw(Config{
		LedgerURL:      primary.URL,
		LedgerFallback: fallback.URL,
	}, "/health")

	if err != nil {
		t.Fatal(err)
	}
	if fallbackCalls != 1 {
		t.Fatalf("fallback calls = %d", fallbackCalls)
	}
	if string(body) != "fallback ok" {
		t.Fatalf("body = %q", string(body))
	}
}

package kovaloopcli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutePaymentIntentDecisions(t *testing.T) {
	tests := []struct {
		name               string
		intent             string
		method             string
		needsClarification bool
		allowedTools       []string
	}{
		{
			name:               "funding uses onramp",
			intent:             `{"deliveryMode":"funding"}`,
			method:             "onramp",
			needsClarification: false,
			allowedTools:       []string{"agent_wallet_create_onramp_session"},
		},
		{
			name:               "agent transfer uses gateway nanopayment",
			intent:             `{"deliveryMode":"agent_transfer"}`,
			method:             "gateway_nanopayment",
			needsClarification: false,
			allowedTools:       []string{"agent_wallet_transfer"},
		},
		{
			name:               "withdrawal needs clarification",
			intent:             `{"deliveryMode":"withdrawal"}`,
			method:             "needs_clarification",
			needsClarification: true,
			allowedTools:       []string{},
		},
		{
			name:               "immediate api needs clarification",
			intent:             `{"deliveryMode":"immediate_api"}`,
			method:             "needs_clarification",
			needsClarification: true,
			allowedTools:       []string{},
		},
		{
			name:               "async task uses escrow",
			intent:             `{"deliveryMode":"async_task"}`,
			method:             "ledger_escrow",
			needsClarification: false,
			allowedTools:       []string{"agent_wallet_create_escrow", "agent_wallet_release_escrow", "agent_wallet_refund_escrow"},
		},
		{
			name:               "requires acceptance uses escrow",
			intent:             `{"requiresAcceptance":true}`,
			method:             "ledger_escrow",
			needsClarification: false,
			allowedTools:       []string{"agent_wallet_create_escrow", "agent_wallet_release_escrow", "agent_wallet_refund_escrow"},
		},
		{
			name:               "ambiguous needs clarification",
			intent:             `{"asset":"USDC"}`,
			method:             "needs_clarification",
			needsClarification: true,
			allowedTools:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload struct {
				Method             string   `json:"method"`
				NeedsClarification bool     `json:"needsClarification"`
				AllowedTools       []string `json:"allowedTools"`
				Reason             string   `json:"reason"`
			}
			if err := json.Unmarshal([]byte(RoutePaymentIntent(tt.intent)), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Method != tt.method {
				t.Fatalf("method = %q", payload.Method)
			}
			if payload.NeedsClarification != tt.needsClarification {
				t.Fatalf("needsClarification = %v", payload.NeedsClarification)
			}
			if strings.TrimSpace(payload.Reason) == "" {
				t.Fatalf("reason is blank in %#v", payload)
			}
			if got, want := strings.Join(payload.AllowedTools, ","), strings.Join(tt.allowedTools, ","); got != want {
				t.Fatalf("allowedTools = %#v, want %#v", payload.AllowedTools, tt.allowedTools)
			}
		})
	}
}

func TestLedgerWalletGetOrCreateRequiresOwnerEmail(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"ledger", "wallet", "get-or-create", `{"agentId":"agent_1","email":"  "}`}, &stdout, &stderr, EnvMap{})

	if exitCode != 2 {
		t.Fatalf("exit code = %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "owner email is required") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestLedgerCreditPostsPayload(t *testing.T) {
	var posted map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.EscapedPath() != "/ledger/accounts/agent%2Fone/credit" {
			t.Fatalf("path = %s escaped=%s", r.URL.Path, r.URL.EscapedPath())
		}
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"ledger", "credit", "agent/one", "12345", "test credit"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_LEDGER_HTTP_URL": server.URL,
	})

	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if posted["amountAtomic"] != "12345" || posted["reason"] != "test credit" {
		t.Fatalf("posted = %#v", posted)
	}
	if strings.TrimSpace(stdout.String()) != `{"ok":"true"}` {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestLedgerEscrowReleasePostsEmptyObject(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/ledger/escrows/escrow%2F1/release" {
			t.Fatalf("path = %s escaped=%s", r.URL.Path, r.URL.EscapedPath())
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "released"})
	}))
	defer server.Close()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"ledger", "escrow", "release", "escrow/1"}, &stdout, &stderr, EnvMap{
		"KOVALOOP_LEDGER_HTTP_URL": server.URL,
	})

	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if len(body) != 0 {
		t.Fatalf("body = %#v", body)
	}
	if strings.TrimSpace(stdout.String()) != `{"status":"released"}` {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

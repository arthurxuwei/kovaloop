package kovaloopcli

import (
	"bytes"
	"encoding/json"
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
			name:               "async task needs clarification",
			intent:             `{"deliveryMode":"async_task"}`,
			method:             "needs_clarification",
			needsClarification: true,
			allowedTools:       []string{},
		},
		{
			name:               "requires acceptance needs clarification",
			intent:             `{"requiresAcceptance":true}`,
			method:             "needs_clarification",
			needsClarification: true,
			allowedTools:       []string{},
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
			if tt.method == "gateway_nanopayment" && !strings.Contains(payload.Reason, "risk controls") {
				t.Fatalf("agent transfer reason does not mention service risk controls: %q", payload.Reason)
			}
			if strings.Contains(strings.ToLower(payload.Method+payload.Reason+strings.Join(payload.AllowedTools, ",")), "escrow") {
				t.Fatalf("route exposes escrow in %#v", payload)
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

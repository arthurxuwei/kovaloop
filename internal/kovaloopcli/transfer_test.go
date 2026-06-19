package kovaloopcli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestLedgerTransferPostsValidatedPayload(t *testing.T) {
	home := writeTransferProfile(t, "agent_sender")

	var posted map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/ledger/transfers" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&posted); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "transfer_1"})
	}))
	defer server.Close()

	payload := `{"toAgentId":" agent_receiver ","amount":"1.5 USDC","reason":"spoofed reason","paymentContext":{"source":"local_user_request","userApproved":true,"reason":" thanks "}}`
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run([]string{"ledger", "transfer", payload}, &stdout, &stderr, EnvMap{
		"KOVALOOP_LEDGER_URL": server.URL,
		"KOVALOOP_HOME":       home,
	})

	if exitCode != 0 {
		t.Fatalf("exit=%d stderr=%s", exitCode, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != `{"id":"transfer_1"}` {
		t.Fatalf("stdout = %q", stdout.String())
	}
	want := map[string]any{
		"fromAgentId":  "agent_sender",
		"toAgentId":    "agent_receiver",
		"amountAtomic": "1500000",
		"reason":       "thanks",
	}
	if !reflect.DeepEqual(posted, want) {
		t.Fatalf("posted = %#v, want %#v", posted, want)
	}
	if len(posted) != 4 {
		t.Fatalf("posted body should contain only four fields: %#v", posted)
	}
}

func TestLedgerTransferValidationErrors(t *testing.T) {
	home := writeTransferProfile(t, "agent_sender")
	tests := []struct {
		name         string
		emptyProfile bool
		payload      string
		wantStderr   string
	}{
		{
			name:       "rejects explicit sender agent id",
			payload:    `{"fromAgentId":"agent_spoof","toAgentId":"agent_receiver","amount":"1000","paymentContext":{"source":"local_user_request","userApproved":true,"reason":"test"}}`,
			wantStderr: "fromAgentId is resolved from the current profile",
		},
		{
			name:       "rejects legacy sender email",
			payload:    `{"fromEmail":"sender@example.com","toAgentId":"agent_receiver","amount":"1000","paymentContext":{"source":"local_user_request","userApproved":true,"reason":"test"}}`,
			wantStderr: "fromEmail is no longer accepted",
		},
		{
			name:       "requires recipient agent id",
			payload:    `{"amount":"1000","paymentContext":{"source":"local_user_request","userApproved":true,"reason":"test"}}`,
			wantStderr: "recipient agent id is required via toAgentId",
		},
		{
			name:       "requires amount",
			payload:    `{"toAgentId":"agent_receiver","paymentContext":{"source":"local_user_request","userApproved":true,"reason":"test"}}`,
			wantStderr: "amount is required",
		},
		{
			name:       "rejects invalid amount",
			payload:    `{"toAgentId":"agent_receiver","amount":"1 potato","paymentContext":{"source":"local_user_request","userApproved":true,"reason":"test"}}`,
			wantStderr: "invalid amount",
		},
		{
			name:       "rejects zero amount",
			payload:    `{"toAgentId":"agent_receiver","amount":"0","paymentContext":{"source":"local_user_request","userApproved":true,"reason":"test"}}`,
			wantStderr: "amount must be greater than zero",
		},
		{
			name:       "rejects negative amount",
			payload:    `{"toAgentId":"agent_receiver","amount":"-1","paymentContext":{"source":"local_user_request","userApproved":true,"reason":"test"}}`,
			wantStderr: "amount must be greater than zero",
		},
		{
			name:       "rejects sub atomic decimal",
			payload:    `{"toAgentId":"agent_receiver","amount":"0.0000001 USDC","paymentContext":{"source":"local_user_request","userApproved":true,"reason":"test"}}`,
			wantStderr: "sub-atomic",
		},
		{
			name:       "requires payment context object",
			payload:    `{"toAgentId":"agent_receiver","amount":"1000"}`,
			wantStderr: "transfer requires paymentContext",
		},
		{
			name:       "rejects payment context non object",
			payload:    `{"toAgentId":"agent_receiver","amount":"1000","paymentContext":"approved"}`,
			wantStderr: "paymentContext must be an object",
		},
		{
			name:       "rejects unsupported payment source",
			payload:    `{"toAgentId":"agent_receiver","amount":"1000","paymentContext":{"source":"remote","userApproved":true,"reason":"test"}}`,
			wantStderr: "paymentContext.source must be local_user_request or local_user_test",
		},
		{
			name:       "requires boolean user approved",
			payload:    `{"toAgentId":"agent_receiver","amount":"1000","paymentContext":{"source":"local_user_test","userApproved":"true","reason":"test"}}`,
			wantStderr: "paymentContext.userApproved must be true",
		},
		{
			name:       "rejects false user approved",
			payload:    `{"toAgentId":"agent_receiver","amount":"1000","paymentContext":{"source":"local_user_test","userApproved":false,"reason":"test"}}`,
			wantStderr: "paymentContext.userApproved must be true",
		},
		{
			name:       "requires reason",
			payload:    `{"toAgentId":"agent_receiver","amount":"1000","paymentContext":{"source":"local_user_test","userApproved":true,"reason":" "}}`,
			wantStderr: "paymentContext.reason is required",
		},
		{
			name:         "requires sender agent id",
			emptyProfile: true,
			payload:      `{"toAgentId":"agent_receiver","amount":"1000","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`,
			wantStderr:   "no local KovaLoop profile",
		},
		{
			name:       "rejects same sender and receiver agent",
			payload:    `{"toAgentId":" agent_sender ","amount":"1000","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`,
			wantStderr: "sender and receiver agent ids must differ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caseHome := home
			if tt.emptyProfile {
				caseHome = writeTransferProfile(t, "")
			}
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := Run([]string{"ledger", "transfer", tt.payload}, &stdout, &stderr, EnvMap{
				"KOVALOOP_HOME": caseHome,
			})

			if exitCode != 2 {
				t.Fatalf("exit code = %d, stderr=%q stdout=%q", exitCode, stderr.String(), stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestLedgerTransferAmountParsing(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: `{"toAgentId":"agent_receiver","amount":"1000","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`, want: "1000"},
		{input: `{"toAgentId":"agent_receiver","amountAtomic":"2500","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`, want: "2500"},
		{input: `{"toAgentId":"agent_receiver","amount":"0.001 U","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`, want: "1000"},
		{input: `{"toAgentId":"agent_receiver","amount":"0.001U","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`, want: "1000"},
		{input: `{"toAgentId":"agent_receiver","amount":"1.5 USDC","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`, want: "1500000"},
		{input: `{"toAgentId":"agent_receiver","amount":"1.5USDC","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`, want: "1500000"},
		{input: `{"toAgentId":"agent_receiver","amountAtomic":"2500","amount":"1.5USDC","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`, want: "2500"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			req, err := buildTransferRequest([]byte(tt.input), "agent_sender")
			if err != nil {
				t.Fatal(err)
			}
			if req.AmountAtomic != tt.want {
				t.Fatalf("amountAtomic = %q, want %q", req.AmountAtomic, tt.want)
			}
		})
	}
}

func TestLedgerTransferRejectsDecimalAmountAtomic(t *testing.T) {
	req, err := buildTransferRequest([]byte(`{"toAgentId":"agent_receiver","amount":"1.5USDC","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`), "agent_sender")
	if err != nil {
		t.Fatal(err)
	}
	if req.AmountAtomic != "1500000" {
		t.Fatalf("amountAtomic = %q", req.AmountAtomic)
	}

	_, err = buildTransferRequest([]byte(`{"toAgentId":"agent_receiver","amountAtomic":"1.5USDC","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"test"}}`), "agent_sender")
	if err == nil {
		t.Fatal("decimal amountAtomic was accepted")
	}
	if !strings.Contains(err.Error(), "amountAtomic must be a positive integer") {
		t.Fatalf("err = %v", err)
	}
}

// writeTransferProfile writes a canonical .kovaloop profile with the given
// agentId and returns the KOVALOOP_HOME value to set.
func writeTransferProfile(t *testing.T, agentID string) string {
	t.Helper()
	return writeLocalKovaloopProfile(t, t.TempDir(), agentID, "Sender")
}

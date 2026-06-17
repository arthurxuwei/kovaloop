package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

type commandResult struct {
	stdout string
	stderr string
	code   int
}

type ledgerStub struct {
	mu              sync.Mutex
	postedClaims    []map[string]any
	postedTransfers []map[string]any
	postedWallets   []map[string]any
	statePaths      []string
	getCount        int
}

func (s *ledgerStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.handleGet(w, r)
		return
	}
	if r.Method == http.MethodPost {
		s.handlePost(w, r)
		return
	}
	http.NotFound(w, r)
}

func (s *ledgerStub) handleGet(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.getCount++
	s.mu.Unlock()

	switch r.URL.RequestURI() {
	case "/ledger/accounts?ownerEmail=receiver%40example.com":
		writeJSON(w, http.StatusOK, map[string]any{"accounts": []any{
			map[string]any{
				"agentId": "agent_receiver",
				"email":   "receiver@example.com",
			},
		}})
	case "/ledger/accounts/agent_sender":
		s.recordStatePath(r.URL.RequestURI())
		writeJSON(w, http.StatusOK, map[string]any{"account": map[string]any{
			"agentId":           "agent_sender",
			"email":             "sender@example.com",
			"availableAtomic":   "940000",
			"lockedAtomic":      "0",
			"circleUsdcBalance": "0.94",
		}})
	case "/ledger/accounts/agent_sender/entries?limit=500":
		s.recordStatePath(r.URL.RequestURI())
		writeJSON(w, http.StatusOK, map[string]any{"entries": []any{}})
	case "/ledger/onramp-sessions?agentId=agent_sender&limit=500":
		s.recordStatePath(r.URL.RequestURI())
		writeJSON(w, http.StatusOK, map[string]any{"onrampSessions": []any{}})
	default:
		if strings.HasPrefix(r.URL.Path, "/ledger/state") {
			s.recordStatePath(r.URL.RequestURI())
			writeJSON(w, http.StatusNotFound, map[string]string{"detail": "Not Found"})
			return
		}
		http.NotFound(w, r)
	}
}

func (s *ledgerStub) handlePost(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	switch r.URL.Path {
	case "/ledger/transfers":
		s.mu.Lock()
		s.postedTransfers = append(s.postedTransfers, body)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "transfer": body})
	case "/ledger/claims/link":
		s.mu.Lock()
		s.postedClaims = append(s.postedClaims, body)
		s.mu.Unlock()
		agentID, _ := body["agentId"].(string)
		writeJSON(w, http.StatusOK, map[string]any{
			"agentId":        agentID,
			"agentName":      body["agentName"],
			"ownerEmail":     strings.ToLower(asString(body["email"])),
			"claimCode":      "clm_testclaim",
			"claimUrl":       "https://ledger.example.test/dashboard?claimCode=clm_testclaim&agentId=" + agentID,
			"agentUrl":       "https://ledger.example.test/dashboard?agentId=" + agentID,
			"walletAddress":  "0x1111111111111111111111111111111111111111",
			"circleWalletId": "circle-wallet-1",
		})
	case "/ledger/wallets/get-or-create":
		s.mu.Lock()
		s.postedWallets = append(s.postedWallets, body)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "account": body})
	default:
		http.NotFound(w, r)
	}
}

func (s *ledgerStub) recordStatePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statePaths = append(s.statePaths, path)
}

func (s *ledgerStub) claims() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.postedClaims...)
}

func (s *ledgerStub) transfers() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.postedTransfers...)
}

func (s *ledgerStub) wallets() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.postedWallets...)
}

func (s *ledgerStub) paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.statePaths...)
}

func (s *ledgerStub) gets() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getCount
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func setupKovaloopWorkspace(t *testing.T) (*ledgerStub, *httptest.Server, string, string, []string) {
	t.Helper()
	stub := &ledgerStub{}
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)

	workspace := filepath.Join(t.TempDir(), "workspace")
	profileDir := filepath.Join(workspace, ".eigenflux", "servers", "eigenflux")
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(profileDir, "profile.json")
	writeFile(t, profilePath, `{"email":"sender@example.com","agent_id":"agent_sender","agent_name":"Sender"}`)

	env := append(os.Environ(),
		"KOVALOOP_LEDGER_HTTP_URL="+server.URL,
		"OPENCLAW_WORKSPACE_DIR="+workspace,
	)
	return stub, server, workspace, profilePath, env
}

func runKovaloop(t *testing.T, env []string, cwd string, args ...string) commandResult {
	t.Helper()
	cmd := exec.Command(testKovaloop, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), code: exitCode(err)}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

func restrictedEnv(t *testing.T, env []string) []string {
	t.Helper()
	binDir := filepath.Join(t.TempDir(), "restricted-path")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	next := make([]string, 0, len(env)+1)
	for _, item := range env {
		if !strings.HasPrefix(item, "PATH=") {
			next = append(next, item)
		}
	}
	return append(next, "PATH="+binDir)
}

func localUserTestContext() map[string]any {
	return map[string]any{
		"source":       "local_user_test",
		"userApproved": true,
		"reason":       "Local user asked this agent to run an online transfer test",
	}
}

func marshalPayload(t *testing.T, payload any) string {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestIntegrationLedgerStateIsProfileScopedAndSanitized(t *testing.T) {
	stub, _, _, _, env := setupKovaloopWorkspace(t)

	result := runKovaloop(t, restrictedEnv(t, env), "", "ledger", "state")
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	if strings.Contains(result.stdout, "availableAtomic") {
		t.Fatalf("state leaked availableAtomic: %s", result.stdout)
	}
	if !strings.Contains(result.stdout, "circleUsdcBalance") {
		t.Fatalf("state missing circleUsdcBalance: %s", result.stdout)
	}
	var state struct {
		Accounts []map[string]any `json:"accounts"`
	}
	if err := json.Unmarshal([]byte(result.stdout), &state); err != nil {
		t.Fatal(err)
	}
	if got := state.Accounts[0]["lockedAtomic"]; got != "0" {
		t.Fatalf("lockedAtomic = %#v", got)
	}
	for _, want := range []string{
		"/ledger/accounts/agent_sender",
		"/ledger/accounts/agent_sender/entries?limit=500",
		"/ledger/onramp-sessions?agentId=agent_sender&limit=500",
	} {
		if !containsString(stub.paths(), want) {
			t.Fatalf("missing state path %q in %#v", want, stub.paths())
		}
	}
	if strings.Contains(result.stdout, "other@example.com") {
		t.Fatalf("state included another account: %s", result.stdout)
	}
}

func TestIntegrationVersionCommandsPrintVersion(t *testing.T) {
	_, _, _, _, env := setupKovaloopWorkspace(t)
	for _, arg := range []string{"version", "--version"} {
		result := runKovaloop(t, env, "", arg)
		if result.code != 0 {
			t.Fatalf("%s exit=%d stderr=%s", arg, result.code, result.stderr)
		}
		if ok, _ := regexp.MatchString(`^kovaloop \d{4}\.\d{2}\.\d{2}\.\d+\n$`, result.stdout); !ok {
			t.Fatalf("stdout = %q", result.stdout)
		}
		if result.stderr != "" {
			t.Fatalf("stderr = %q", result.stderr)
		}
	}
}

func TestIntegrationTransferValidationAndPosting(t *testing.T) {
	stub, _, _, _, env := setupKovaloopWorkspace(t)
	valid := map[string]any{
		"toAgentId":      "agent_receiver",
		"amount":         "0.001 U",
		"paymentContext": localUserTestContext(),
	}

	result := runKovaloop(t, env, "", "ledger", "transfer", marshalPayload(t, valid))
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	want := map[string]any{
		"fromAgentId":  "agent_sender",
		"toAgentId":    "agent_receiver",
		"amountAtomic": "1000",
		"reason":       "Local user asked this agent to run an online transfer test",
	}
	assertJSONMapEqual(t, stub.transfers()[0], want)
	if stub.gets() != 0 {
		t.Fatalf("transfer should not fetch ledger state, got %d GETs", stub.gets())
	}

	stub, _, _, _, env = setupKovaloopWorkspace(t)
	result = runKovaloop(t, restrictedEnv(t, env), "", "ledger", "transfer", marshalPayload(t, valid))
	if result.code != 0 {
		t.Fatalf("restricted PATH transfer exit=%d stderr=%s", result.code, result.stderr)
	}
	assertJSONMapEqual(t, stub.transfers()[0], want)

	tests := []struct {
		name       string
		payload    map[string]any
		wantStderr string
	}{
		{
			name:       "rejects explicit sender agent id",
			payload:    map[string]any{"fromAgentId": "agent_sender", "toAgentId": "agent_receiver", "amountAtomic": "1000"},
			wantStderr: "fromAgentId is resolved from the current profile",
		},
		{
			name:       "requires payment context",
			payload:    map[string]any{"toAgentId": "agent_receiver", "amount": "0.001 U"},
			wantStderr: "transfer requires paymentContext",
		},
		{
			name: "rejects private dm source",
			payload: map[string]any{
				"toAgentId": "agent_receiver",
				"amount":    "0.001 U",
				"paymentContext": map[string]any{
					"source":       "private_dm_request",
					"userApproved": true,
					"reason":       "Counterparty asked for a test transfer in private DM",
				},
			},
			wantStderr: "paymentContext.source must be local_user_request or local_user_test",
		},
		{
			name: "rejects unapproved context",
			payload: map[string]any{
				"toAgentId":      "agent_receiver",
				"amount":         "0.001 U",
				"paymentContext": map[string]any{"source": "local_user_test", "userApproved": false, "reason": "Local user did not approve"},
			},
			wantStderr: "paymentContext.userApproved must be true",
		},
		{
			name: "rejects string approval",
			payload: map[string]any{
				"toAgentId":      "agent_receiver",
				"amount":         "0.001 U",
				"paymentContext": map[string]any{"source": "local_user_test", "userApproved": "true", "reason": "String approval must not count"},
			},
			wantStderr: "paymentContext.userApproved must be true",
		},
		{
			name: "rejects blank reason",
			payload: map[string]any{
				"toAgentId":      "agent_receiver",
				"amount":         "0.001 U",
				"paymentContext": map[string]any{"source": "local_user_test", "userApproved": true, "reason": "   "},
			},
			wantStderr: "paymentContext.reason is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub, _, _, _, env := setupKovaloopWorkspace(t)
			result := runKovaloop(t, env, "", "ledger", "transfer", marshalPayload(t, tt.payload))
			if result.code != 2 {
				t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
			}
			if !strings.Contains(result.stderr, tt.wantStderr) {
				t.Fatalf("stderr=%q want %q", result.stderr, tt.wantStderr)
			}
			if len(stub.transfers()) != 0 {
				t.Fatalf("posted transfers = %#v", stub.transfers())
			}
		})
	}
}

func TestIntegrationTransferResolvesRecipientEmailToAgentID(t *testing.T) {
	stub, _, _, _, env := setupKovaloopWorkspace(t)
	payload := map[string]any{
		"toEmail":        "receiver@example.com",
		"amount":         "0.000001 U",
		"paymentContext": localUserTestContext(),
	}

	result := runKovaloop(t, env, "", "ledger", "transfer", marshalPayload(t, payload))

	if result.code != 0 {
		t.Fatalf("transfer by email exit=%d stderr=%s", result.code, result.stderr)
	}
	if len(stub.transfers()) != 1 {
		t.Fatalf("posted transfers = %#v", stub.transfers())
	}
	assertJSONMapEqual(t, stub.transfers()[0], map[string]any{
		"fromAgentId":  "agent_sender",
		"toAgentId":    "agent_receiver",
		"amountAtomic": "1",
		"reason":       "Local user asked this agent to run an online transfer test",
	})
}

func TestIntegrationTransferFindsProfileFromWorkspaceCWD(t *testing.T) {
	stub, _, workspace, _, env := setupKovaloopWorkspace(t)
	env = removeEnv(env, "OPENCLAW_WORKSPACE_DIR")
	env = append(env, "PWD="+workspace)
	payload := map[string]any{
		"toAgentId":      "agent_receiver",
		"amount":         "0.001 U",
		"paymentContext": localUserTestContext(),
	}

	result := runKovaloop(t, env, workspace, "ledger", "transfer", marshalPayload(t, payload))
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	if got := stub.transfers()[0]["fromAgentId"]; got != "agent_sender" {
		t.Fatalf("fromAgentId = %#v", got)
	}
}

func TestIntegrationTransferAcceptsLocalUserRequestContext(t *testing.T) {
	stub, _, _, _, env := setupKovaloopWorkspace(t)
	payload := map[string]any{
		"toAgentId": "agent_receiver",
		"amount":    "0.001 U",
		"paymentContext": map[string]any{
			"source":       "local_user_request",
			"userApproved": true,
			"reason":       "Local user asked this agent to pay receiver for a real task",
		},
	}

	result := runKovaloop(t, env, "", "ledger", "transfer", marshalPayload(t, payload))
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	if got := stub.transfers()[0]["reason"]; got != "Local user asked this agent to pay receiver for a real task" {
		t.Fatalf("reason = %#v", got)
	}
}

func TestIntegrationClaimLinkProfileHandling(t *testing.T) {
	stub, _, _, profilePath, env := setupKovaloopWorkspace(t)
	result := runKovaloop(t, env, "", "claim", "link")
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	assertJSONMapEqual(t, stub.claims()[0], map[string]any{
		"agentId":          "agent_sender",
		"agentName":        "Sender",
		"email":            "sender@example.com",
		"agentDescription": "",
	})
	for _, want := range []string{
		"Agent ID:   agent_sender",
		"Claim Code: clm_testclaim",
		"Claim Link: https://ledger.example.test/dashboard?claimCode=clm_testclaim&agentId=agent_sender",
		"Agent Link: https://ledger.example.test/dashboard?agentId=agent_sender",
	} {
		if !strings.Contains(result.stdout, want) {
			t.Fatalf("stdout missing %q: %s", want, result.stdout)
		}
	}

	if err := os.Remove(profilePath); err != nil {
		t.Fatal(err)
	}
	result = runKovaloop(t, env, "", "claim", "link")
	if result.code == 0 {
		t.Fatalf("missing profile succeeded")
	}
	if !strings.Contains(result.stderr, "OpenClaw profile") || !strings.Contains(result.stderr, profilePath) {
		t.Fatalf("stderr = %q", result.stderr)
	}

	writeFile(t, profilePath, `{"agent_id": `)
	result = runKovaloop(t, env, "", "claim", "link")
	if result.code == 0 || !strings.Contains(result.stderr, "malformed JSON") || !strings.Contains(result.stderr, profilePath) {
		t.Fatalf("malformed profile exit=%d stderr=%q", result.code, result.stderr)
	}

	writeFile(t, profilePath, `[]`)
	result = runKovaloop(t, env, "", "claim", "link")
	if result.code == 0 || !strings.Contains(result.stderr, "malformed") || strings.Contains(result.stderr, "Traceback") {
		t.Fatalf("non-object profile exit=%d stderr=%q", result.code, result.stderr)
	}
}

func TestIntegrationClaimLinkDoesNotNeedExternalRuntimeAndEncodesProfile(t *testing.T) {
	stub, _, _, profilePath, env := setupKovaloopWorkspace(t)
	writeJSONFile(t, profilePath, map[string]any{
		"email":      "sender@example.com",
		"agent_id":   "agent_sender",
		"agent_name": `Sender "Slash" \ Agent`,
		"bio":        `Builds "quoted" paths like C:\agents\sender`,
	})

	result := runKovaloop(t, restrictedEnv(t, env), "", "claim", "link")
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	assertJSONMapEqual(t, stub.claims()[0], map[string]any{
		"agentId":          "agent_sender",
		"agentName":        `Sender "Slash" \ Agent`,
		"email":            "sender@example.com",
		"agentDescription": `Builds "quoted" paths like C:\agents\sender`,
	})

	writeJSONFile(t, profilePath, map[string]any{
		"email":      "sender@example.com",
		"agent_id":   "agent_sender",
		"agent_name": "   ",
	})
	result = runKovaloop(t, env, "", "claim", "link")
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	if got := stub.claims()[1]["agentName"]; got != "agent_sender" {
		t.Fatalf("agentName = %#v", got)
	}
}

func TestIntegrationWalletValidationDoesNotNeedExternalRuntime(t *testing.T) {
	stub, _, _, _, env := setupKovaloopWorkspace(t)
	payload := marshalPayload(t, map[string]any{"agentId": "x", "agentName": "X", "email": "   "})

	result := runKovaloop(t, restrictedEnv(t, env), "", "ledger", "wallet", "get-or-create", payload)
	if result.code != 2 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	if !strings.Contains(result.stderr, "owner email is required") {
		t.Fatalf("stderr = %q", result.stderr)
	}
	if len(stub.wallets()) != 0 {
		t.Fatalf("posted wallets = %#v", stub.wallets())
	}
}

func TestSkillsDescribeTransferAntiFraudPolicy(t *testing.T) {
	kovaloopLedger := readRepoFile(t, "skills", "kovaloop-ledger", "SKILL.md")

	for _, want := range []string{
		"Direct transfer is a high-risk",
		"must stop",
		"paymentContext",
		"Claim Link is a local owner wallet-binding link only",
		"Never tell the user to share a Claim Link",
		"Recipient email is not a final Kovaloop transfer identity",
		"call `kovaloop ledger transfer` with `toEmail`",
		"the CLI will look up the agent bound to that email",
		"If recipient email lookup fails or returns multiple agents",
		"Do not tell the recipient to install Kovaloop, download Kovaloop, or run `kovaloop claim link`",
	} {
		if !strings.Contains(kovaloopLedger, want) {
			t.Fatalf("kovaloop-ledger missing %q", want)
		}
	}
	directTransfer := readRepoFile(t, "skills", "kovaloop-ledger", "references", "direct-transfer.md")
	for _, want := range []string{
		"Recipient email is not a final Kovaloop transfer identity",
		"pass it as `toEmail`",
		"the CLI will look up the agent bound to that email",
		"If recipient email lookup fails or returns multiple agents",
		"Do not tell the recipient to install Kovaloop, download Kovaloop, or run `kovaloop claim link`",
	} {
		if !strings.Contains(directTransfer, want) {
			t.Fatalf("direct-transfer missing %q", want)
		}
	}
	if strings.Contains(kovaloopLedger, "If the user gives a recipient email plus a USDC amount and does not mention a service") {
		t.Fatalf("kovaloop-ledger contains removed direct-transfer shortcut")
	}
	if strings.Contains(kovaloopLedger, "ledger.settlementRecords") {
		t.Fatalf("kovaloop-ledger mentions permanently empty settlementRecords")
	}
}

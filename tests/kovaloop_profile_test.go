package tests

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type profileStub struct {
	mu            sync.Mutex
	createBodies  []map[string]any
	patchHeaders  []http.Header
	patchBodies   []string
	nextAgentID   string
	nextAgentName string
}

func (s *profileStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := readBody(r)
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/ledger/profiles":
		var parsed map[string]any
		_ = json.Unmarshal([]byte(body), &parsed)
		s.createBodies = append(s.createBodies, parsed)
		writeJSON(w, http.StatusOK, map[string]any{"profile": map[string]any{
			"agentId":   s.nextAgentID,
			"agentName": s.nextAgentName,
		}})
	case r.Method == http.MethodPatch && strings.HasPrefix(r.URL.Path, "/ledger/profiles/"):
		s.patchHeaders = append(s.patchHeaders, r.Header.Clone())
		s.patchBodies = append(s.patchBodies, body)
		writeJSON(w, http.StatusOK, map[string]any{"profile": map[string]any{
			"agentId":     s.nextAgentID,
			"description": "new",
		}})
	default:
		http.NotFound(w, r)
	}
}

func readBody(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}
	buf := make([]byte, r.ContentLength)
	n, _ := r.Body.Read(buf)
	return string(buf[:n]), nil
}

func profileEnv(t *testing.T, serverURL, home string) []string {
	t.Helper()
	base := os.Environ()
	for _, k := range []string{"EIGENFLUX_HOME", "KOVALOOP_AGENT_PROFILE_PATH", "OPENCLAW_WORKSPACE_DIR", "HERMES_CONFIG_DIR"} {
		base = removeEnv(base, k)
	}
	return append(base,
		"KOVALOOP_LEDGER_URL="+serverURL,
		"KOVALOOP_HOME="+home,
	)
}

func TestProfileCreateWritesFilesAndSendsPublicKey(t *testing.T) {
	stub := &profileStub{nextAgentID: "kloop_agent_TEST", nextAgentName: "OntologyAgent"}
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)

	home := t.TempDir()
	env := profileEnv(t, server.URL, home)

	result := runKovaloop(t, env, "", "profile", "create")
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	if !strings.Contains(result.stdout, "kloop_agent_TEST") {
		t.Fatalf("stdout missing agentId: %s", result.stdout)
	}

	credPath := filepath.Join(home, ".kovaloop", "credentials.json")
	info, err := os.Stat(credPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("credentials perm %v", info.Mode().Perm())
	}
	var creds map[string]any
	credData, _ := os.ReadFile(credPath)
	_ = json.Unmarshal(credData, &creds)
	if asString(creds["agentId"]) != "kloop_agent_TEST" {
		t.Fatalf("creds agentId %v", creds["agentId"])
	}
	publicKey := asString(creds["publicKey"])
	if publicKey == "" {
		t.Fatal("creds missing publicKey")
	}

	var profile map[string]any
	profData, _ := os.ReadFile(filepath.Join(home, ".kovaloop", "profile.json"))
	_ = json.Unmarshal(profData, &profile)
	if asString(profile["agentId"]) != "kloop_agent_TEST" {
		t.Fatalf("profile agentId %v", profile["agentId"])
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.createBodies) != 1 {
		t.Fatalf("expected 1 create POST, got %d", len(stub.createBodies))
	}
	if asString(stub.createBodies[0]["credentialPublicKey"]) != publicKey {
		t.Fatalf("sent pubkey != stored pubkey")
	}
}

func TestProfileCreateIsIdempotent(t *testing.T) {
	stub := &profileStub{nextAgentID: "kloop_agent_TEST", nextAgentName: "OntologyAgent"}
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)

	home := t.TempDir()
	env := profileEnv(t, server.URL, home)

	first := runKovaloop(t, env, "", "profile", "create")
	if first.code != 0 {
		t.Fatalf("first exit=%d stderr=%s", first.code, first.stderr)
	}
	second := runKovaloop(t, env, "", "profile", "create")
	if second.code != 0 {
		t.Fatalf("second exit=%d stderr=%s", second.code, second.stderr)
	}
	if !strings.Contains(second.stdout, "kloop_agent_TEST") {
		t.Fatalf("second run lost agentId: %s", second.stdout)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.createBodies) != 1 {
		t.Fatalf("continuity broken: expected 1 POST total, got %d", len(stub.createBodies))
	}
}

func TestProfileCreateCarriesEigenflux(t *testing.T) {
	stub := &profileStub{nextAgentID: "kloop_agent_TEST", nextAgentName: "Old Agent"}
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)

	home := t.TempDir()
	eigenfluxHome := filepath.Join(home, ".eigenflux")
	efDir := filepath.Join(eigenfluxHome, "servers", "eigenflux")
	if err := os.MkdirAll(efDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(efDir, "profile.json"),
		`{"email":"owner@example.com","agent_id":"312586087945994240","agent_name":"Old Agent","bio":"old bio"}`)

	base := os.Environ()
	for _, k := range []string{"KOVALOOP_AGENT_PROFILE_PATH", "HERMES_CONFIG_DIR"} {
		base = removeEnv(base, k)
	}
	env := append(base,
		"KOVALOOP_LEDGER_URL="+server.URL,
		"KOVALOOP_HOME="+home,
		"EIGENFLUX_HOME="+eigenfluxHome,
	)

	result := runKovaloop(t, env, "", "profile", "create")
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.createBodies) != 1 {
		t.Fatalf("expected 1 create POST, got %d", len(stub.createBodies))
	}
	ef, ok := stub.createBodies[0]["eigenflux"].(map[string]any)
	if !ok {
		t.Fatalf("eigenflux not forwarded: %v", stub.createBodies[0])
	}
	if asString(ef["id"]) != "312586087945994240" {
		t.Fatalf("eigenflux id %v", ef["id"])
	}
}

func TestProfileUpdateSendsVerifiableSignature(t *testing.T) {
	stub := &profileStub{nextAgentID: "kloop_agent_TEST", nextAgentName: "OntologyAgent"}
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)

	home := t.TempDir()
	env := profileEnv(t, server.URL, home)

	if r := runKovaloop(t, env, "", "profile", "create"); r.code != 0 {
		t.Fatalf("create exit=%d stderr=%s", r.code, r.stderr)
	}

	// load the stored public key to verify the signature against
	var creds map[string]any
	credData, _ := os.ReadFile(filepath.Join(home, ".kovaloop", "credentials.json"))
	_ = json.Unmarshal(credData, &creds)
	pubRaw, err := base64.RawURLEncoding.DecodeString(asString(creds["publicKey"]))
	if err != nil {
		t.Fatal(err)
	}

	body := `{"description":"new"}`
	if r := runKovaloop(t, env, "", "profile", "update", body); r.code != 0 {
		t.Fatalf("update exit=%d stderr=%s", r.code, r.stderr)
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.patchHeaders) != 1 {
		t.Fatalf("expected 1 PATCH, got %d", len(stub.patchHeaders))
	}
	h := stub.patchHeaders[0]
	agentID := h.Get("X-KovaLoop-Agent-Id")
	ts := h.Get("X-KovaLoop-Timestamp")
	nonce := h.Get("X-KovaLoop-Nonce")
	sigB64 := h.Get("X-KovaLoop-Signature")
	if agentID == "" || ts == "" || nonce == "" || sigB64 == "" {
		t.Fatalf("missing signed headers: %v", h)
	}
	if agentID != "kloop_agent_TEST" {
		t.Fatalf("agent id header %q", agentID)
	}
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatal(err)
	}
	msg := agentID + "\n" + ts + "\n" + nonce + "\n" + stub.patchBodies[0]
	if !ed25519.Verify(ed25519.PublicKey(pubRaw), []byte(msg), sig) {
		t.Fatal("signature did not verify against stored public key")
	}
}

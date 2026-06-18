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

type claimLedgerStub struct {
	mu     sync.Mutex
	claims []map[string]any
}

func (s *claimLedgerStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/ledger/claims/link" {
		http.NotFound(w, r)
		return
	}

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.claims = append(s.claims, body)
	s.mu.Unlock()

	agentID := asString(body["agentId"])
	writeJSON(w, http.StatusOK, map[string]any{
		"agentId":   agentID,
		"claimCode": "clm_" + agentID,
		"claimUrl":  "https://ledger.example.test/dashboard?claimCode=clm_" + agentID + "&agentId=" + agentID,
		"agentUrl":  "https://ledger.example.test/dashboard?agentId=" + agentID,
	})
}

func (s *claimLedgerStub) postedClaims() []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]map[string]any(nil), s.claims...)
}

type binaryAssetStub struct {
	mu             sync.Mutex
	assetName      string
	assetBytes     []byte
	requestedPaths []string
}

func (s *binaryAssetStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requestedPaths = append(s.requestedPaths, r.URL.Path)
	s.mu.Unlock()
	if r.Method != http.MethodGet || r.URL.Path != "/"+s.assetName {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(s.assetBytes)
}

func (s *binaryAssetStub) paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requestedPaths...)
}

func createOpenClawWorkspaces(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"runtime-openclaw-x", "runtime-openclaw-y"} {
		profile := filepath.Join(root, name, "workspace", ".eigenflux", "servers", "eigenflux", "profile.json")
		writeFile(t, profile, `{"email":"owner@example.com","agent_id":"`+name+`","agent_name":"`+name+`"}`)
	}
}

func createHermesConfigs(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"runtime-hermes-x", "runtime-hermes-y"} {
		profile := filepath.Join(root, name, "config", ".eigenflux", "servers", "eigenflux", "profile.json")
		writeFile(t, profile, `{"email":"owner@example.com","agent_id":"`+name+`","agent_name":"`+name+`"}`)
	}
}

func envValue(value string) *string {
	return &value
}

func runInstall(t *testing.T, root string, extraEnv map[string]*string) commandResult {
	t.Helper()
	// HOME is set to the test root so the binary installs into <root>/.local/bin
	// (the installer targets $HOME/.local/bin) instead of the real home dir.
	env := append(os.Environ(),
		"KOVALOOP_INSTALL_BIN_DIR="+testAssetDir,
		"KOVALOOP_LEDGER_URL=http://127.0.0.1:9",
	)
	env = removeEnv(env, "HOME")
	env = append(env, "HOME="+root)
	for key, value := range extraEnv {
		env = removeEnv(env, key)
		if value != nil {
			env = append(env, key+"="+*value)
		}
	}

	cmd := exec.Command(filepath.Join(repoRoot, "install.sh"))
	cmd.Dir = root
	cmd.Env = env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), code: exitCode(err)}
}

func fakeUname(t *testing.T, root string, system string, machine string) string {
	t.Helper()
	binDir := filepath.Join(root, "fake-bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	uname := filepath.Join(binDir, "uname")
	writeFile(t, uname, "#!/usr/bin/env sh\ncase \"$1\" in\n  -s) echo "+system+" ;;\n  -m) echo "+machine+" ;;\nesac\n")
	if err := os.Chmod(uname, 0o755); err != nil {
		t.Fatal(err)
	}
	return binDir + string(os.PathListSeparator) + os.Getenv("PATH")
}

func hideLocalDistAsset(t *testing.T, assetName string) {
	t.Helper()
	distAsset := filepath.Join(repoRoot, "dist", assetName)
	if _, err := os.Stat(distAsset); os.IsNotExist(err) {
		return
	} else if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), assetName)
	if err := os.Rename(distAsset, backup); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.MkdirAll(filepath.Dir(distAsset), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(backup, distAsset); err != nil {
			t.Fatal(err)
		}
	})
}

func writeDistAsset(t *testing.T, assetName string, data []byte) {
	t.Helper()
	distAsset := filepath.Join(repoRoot, "dist", assetName)
	var original []byte
	var originalMode os.FileMode
	hadOriginal := false
	if info, err := os.Stat(distAsset); err == nil {
		hadOriginal = true
		originalMode = info.Mode()
		original, err = os.ReadFile(distAsset)
		if err != nil {
			t.Fatal(err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(distAsset), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(distAsset, data, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadOriginal {
			if err := os.WriteFile(distAsset, original, originalMode); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(distAsset, originalMode); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := os.Remove(distAsset); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	})
}

func testAssetBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(testAssetPath)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestInstallIntoAllOpenClawWorkspacesAndKeepsLinkFailureNonfatal(t *testing.T) {
	root := t.TempDir()
	createOpenClawWorkspaces(t, root)
	// Old chief binary lives alongside the new one in $HOME/.local/bin (= root).
	writeFile(t, filepath.Join(root, ".local", "bin", "chief"), "old chief")
	for _, name := range []string{"runtime-openclaw-x", "runtime-openclaw-y"} {
		workspace := filepath.Join(root, name, "workspace")
		writeFile(t, filepath.Join(workspace, "skills", "chief-ledger", "SKILL.md"), "old chief skill")
	}

	result := runInstall(t, root, nil)
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}

	// The binary is installed once, to $HOME/.local/bin (on PATH, beside eigenflux).
	kovaloop := filepath.Join(root, ".local", "bin", "kovaloop")
	gotBytes, err := os.ReadFile(kovaloop)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBytes, testAssetBytes(t)) {
		t.Fatalf("%s binary bytes mismatch", kovaloop)
	}
	version := runCommand(t, nil, "", kovaloop, "version")
	if version.code != 0 {
		t.Fatalf("version exit=%d stderr=%s", version.code, version.stderr)
	}
	if ok, _ := regexp.MatchString(`^kovaloop \d{4}\.\d{2}\.\d{2}\.\d+\n$`, version.stdout); !ok {
		t.Fatalf("version stdout=%q", version.stdout)
	}
	if _, err := os.Stat(filepath.Join(root, ".local", "bin", "chief")); !os.IsNotExist(err) {
		t.Fatalf("old chief binary was not removed: %v", err)
	}

	// Skills are installed per-workspace, with references/ included.
	for _, name := range []string{"runtime-openclaw-x", "runtime-openclaw-y"} {
		workspace := filepath.Join(root, name, "workspace")
		if _, err := os.Stat(filepath.Join(workspace, "skills", "kovaloop-ledger", "SKILL.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(workspace, "skills", "kovaloop-ledger", "references", "onboarding.md")); err != nil {
			t.Fatalf("skill references not installed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(workspace, "skills", "kovaloop-a2a-service-trade")); !os.IsNotExist(err) {
			t.Fatalf("escrow service-trade skill was installed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(workspace, "skills", "chief-ledger")); !os.IsNotExist(err) {
			t.Fatalf("old chief skill was not removed: %v", err)
		}
		if !strings.Contains(result.stdout, "KOVALOOP_HOME="+filepath.Dir(workspace)) {
			t.Fatalf("stdout missing home retry: %s", result.stdout)
		}
	}
	if !strings.Contains(result.stdout, "Claim link unavailable") {
		t.Fatalf("stdout missing claim failure: %s", result.stdout)
	}
}

func TestInstallIntoAllHermesConfigsAndKeepsLinkFailureNonfatal(t *testing.T) {
	root := t.TempDir()
	createHermesConfigs(t, root)
	writeFile(t, filepath.Join(root, ".local", "bin", "chief"), "old chief")
	for _, name := range []string{"runtime-hermes-x", "runtime-hermes-y"} {
		config := filepath.Join(root, name, "config")
		writeFile(t, filepath.Join(config, "skills", "chief-ledger", "SKILL.md"), "old chief skill")
	}

	result := runInstall(t, root, nil)
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}

	kovaloop := filepath.Join(root, ".local", "bin", "kovaloop")
	gotBytes, err := os.ReadFile(kovaloop)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBytes, testAssetBytes(t)) {
		t.Fatalf("%s binary bytes mismatch", kovaloop)
	}
	if _, err := os.Stat(filepath.Join(root, ".local", "bin", "chief")); !os.IsNotExist(err) {
		t.Fatalf("old chief binary was not removed: %v", err)
	}

	for _, name := range []string{"runtime-hermes-x", "runtime-hermes-y"} {
		config := filepath.Join(root, name, "config")
		if _, err := os.Stat(filepath.Join(config, "skills", "kovaloop-ledger", "SKILL.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(config, "skills", "kovaloop-ledger", "references", "onboarding.md")); err != nil {
			t.Fatalf("skill references not installed: %v", err)
		}
		if _, err := os.Stat(filepath.Join(config, "skills", "chief-ledger")); !os.IsNotExist(err) {
			t.Fatalf("old chief skill was not removed: %v", err)
		}
		if !strings.Contains(result.stdout, "KOVALOOP_HOME="+config) {
			t.Fatalf("stdout missing home retry: %s", result.stdout)
		}
	}
	if !strings.Contains(result.stdout, "Hermes config:") {
		t.Fatalf("stdout missing Hermes label: %s", result.stdout)
	}
}

func runCommand(t *testing.T, env []string, cwd string, name string, args ...string) commandResult {
	t.Helper()
	cmd := exec.Command(name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if env != nil {
		cmd.Env = env
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return commandResult{stdout: stdout.String(), stderr: stderr.String(), code: exitCode(err)}
}

func TestInstallExplicitOpenClawWorkspaceInstallsOnlyThatWorkspace(t *testing.T) {
	root := t.TempDir()
	createOpenClawWorkspaces(t, root)
	target := filepath.Join(root, "runtime-openclaw-x", "workspace")

	result := runInstall(t, root, map[string]*string{"OPENCLAW_WORKSPACE_DIR": envValue(target)})
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	// Binary is global ($HOME/.local/bin); only the target workspace gets the skill.
	if _, err := os.Stat(filepath.Join(root, ".local", "bin", "kovaloop")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "kovaloop-ledger", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "runtime-openclaw-y", "workspace")
	if _, err := os.Stat(filepath.Join(other, "skills", "kovaloop-ledger")); !os.IsNotExist(err) {
		t.Fatalf("other workspace skill exists or stat failed: %v", err)
	}
}

func TestInstallExplicitHermesConfigInstallsOnlyThatConfig(t *testing.T) {
	root := t.TempDir()
	createHermesConfigs(t, root)
	target := filepath.Join(root, "runtime-hermes-x", "config")

	result := runInstall(t, root, map[string]*string{"HERMES_CONFIG_DIR": envValue(target)})
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".local", "bin", "kovaloop")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, "skills", "kovaloop-ledger", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, "runtime-hermes-y", "config")
	if _, err := os.Stat(filepath.Join(other, "skills", "kovaloop-ledger")); !os.IsNotExist(err) {
		t.Fatalf("other Hermes config skill exists or stat failed: %v", err)
	}
}

func TestInstallPrintsClaimCodeAndLinkWhenLedgerIsAvailable(t *testing.T) {
	root := t.TempDir()
	createOpenClawWorkspaces(t, root)
	stub := &claimLedgerStub{}
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	target := filepath.Join(root, "runtime-openclaw-x", "workspace")

	// Simulate the runtime providing EIGENFLUX_HOME so claim link can resolve the
	// EigenFlux profile (the installer never sets EIGENFLUX_* itself).
	result := runInstall(t, root, map[string]*string{
		"OPENCLAW_WORKSPACE_DIR": envValue(target),
		"KOVALOOP_LEDGER_URL":    envValue(server.URL),
		"EIGENFLUX_HOME":         envValue(filepath.Join(target, ".eigenflux")),
	})
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	assertJSONMapEqual(t, stub.postedClaims()[0], map[string]any{
		"agentId":          "runtime-openclaw-x",
		"agentName":        "runtime-openclaw-x",
		"email":            "owner@example.com",
		"agentDescription": "",
	})
	if !strings.Contains(result.stdout, "Claim Code: clm_runtime-openclaw-x") {
		t.Fatalf("stdout=%s", result.stdout)
	}
	if !strings.Contains(result.stdout, "Claim Link: https://ledger.example.test/dashboard?claimCode=clm_runtime-openclaw-x&agentId=runtime-openclaw-x") {
		t.Fatalf("stdout=%s", result.stdout)
	}
}

func TestInstallPrintsClaimCodeAndLinkForHermesWhenLedgerIsAvailable(t *testing.T) {
	root := t.TempDir()
	createHermesConfigs(t, root)
	stub := &claimLedgerStub{}
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	target := filepath.Join(root, "runtime-hermes-x", "config")

	result := runInstall(t, root, map[string]*string{
		"HERMES_CONFIG_DIR":   envValue(target),
		"KOVALOOP_LEDGER_URL": envValue(server.URL),
		"EIGENFLUX_HOME":      envValue(filepath.Join(target, ".eigenflux")),
	})
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	assertJSONMapEqual(t, stub.postedClaims()[0], map[string]any{
		"agentId":          "runtime-hermes-x",
		"agentName":        "runtime-hermes-x",
		"email":            "owner@example.com",
		"agentDescription": "",
	})
	if !strings.Contains(result.stdout, "Claim Code: clm_runtime-hermes-x") {
		t.Fatalf("stdout=%s", result.stdout)
	}
}

func TestInstallFailsWhenNoSupportedRuntimeExists(t *testing.T) {
	root := t.TempDir()
	result := runInstall(t, root, nil)

	if result.code != 2 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	if !strings.Contains(result.stderr, "No OpenClaw workspace or Hermes config found") {
		t.Fatalf("stderr=%q", result.stderr)
	}
}

func TestInstallFailsClearlyOnUnsupportedPlatform(t *testing.T) {
	root := t.TempDir()
	createOpenClawWorkspaces(t, root)
	path := fakeUname(t, root, "Plan9", "riscv64")

	result := runInstall(t, root, map[string]*string{"PATH": envValue(path)})
	if result.code != 2 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	if !strings.Contains(result.stderr, "Unsupported platform: Plan9/riscv64") {
		t.Fatalf("stderr=%q", result.stderr)
	}
}

func TestInstallDownloadsBinaryAssetFromBinaryBaseURL(t *testing.T) {
	root := t.TempDir()
	createOpenClawWorkspaces(t, root)
	assetName := "kovaloop_linux_amd64"
	hideLocalDistAsset(t, assetName)
	stub := &binaryAssetStub{assetName: assetName, assetBytes: testAssetBytes(t)}
	server := httptest.NewServer(stub)
	t.Cleanup(server.Close)
	path := fakeUname(t, root, "Linux", "x86_64")
	target := filepath.Join(root, "runtime-openclaw-x", "workspace")

	result := runInstall(t, root, map[string]*string{
		"OPENCLAW_WORKSPACE_DIR":        envValue(target),
		"KOVALOOP_INSTALL_BIN_DIR":      nil,
		"KOVALOOP_INSTALL_BASE_URL":     envValue("http://127.0.0.1:9/not-used"),
		"KOVALOOP_INSTALL_BIN_BASE_URL": envValue(server.URL),
		"PATH":                          envValue(path),
	})
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	if got := stub.paths(); !reflectStringSlices(got, []string{"/" + assetName}) {
		t.Fatalf("requested paths=%#v", got)
	}
	gotBytes, err := os.ReadFile(filepath.Join(root, ".local", "bin", "kovaloop"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBytes, testAssetBytes(t)) {
		t.Fatalf("installed binary bytes mismatch")
	}
}

func reflectStringSlices(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func TestInstallUsesLocalDistBeforeDownload(t *testing.T) {
	root := t.TempDir()
	createOpenClawWorkspaces(t, root)
	assetName := "kovaloop_linux_arm64"
	writeDistAsset(t, assetName, testAssetBytes(t))
	path := fakeUname(t, root, "Linux", "aarch64")
	target := filepath.Join(root, "runtime-openclaw-x", "workspace")

	result := runInstall(t, root, map[string]*string{
		"OPENCLAW_WORKSPACE_DIR":        envValue(target),
		"KOVALOOP_INSTALL_BIN_DIR":      nil,
		"KOVALOOP_INSTALL_BIN_BASE_URL": envValue("http://127.0.0.1:9/not-used"),
		"PATH":                          envValue(path),
	})
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	gotBytes, err := os.ReadFile(filepath.Join(root, ".local", "bin", "kovaloop"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotBytes, testAssetBytes(t)) {
		t.Fatalf("installed binary bytes mismatch")
	}
}

func TestInstallRetryCommandIsPasteableWhenWorkspacePathContainsSpaces(t *testing.T) {
	root, err := os.MkdirTemp("", "kovaloop install ")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	createOpenClawWorkspaces(t, root)

	result := runInstall(t, root, nil)
	if result.code != 0 {
		t.Fatalf("exit=%d stderr=%s", result.code, result.stderr)
	}
	lines := strings.Split(result.stdout, "\n")
	var retryCommands []string
	for i := 0; i+1 < len(lines); i++ {
		if lines[i] == "Retry:" {
			retryCommands = append(retryCommands, lines[i+1])
		}
	}
	if len(retryCommands) != 2 {
		t.Fatalf("retry commands=%#v stdout=%s", retryCommands, result.stdout)
	}
	firstRetry := retryCommands[0]
	if !strings.Contains(firstRetry, "KOVALOOP_HOME=") || !strings.Contains(firstRetry, `\ `) {
		t.Fatalf("retry command not escaped: %q", firstRetry)
	}
	env := append(os.Environ(),
		"KOVALOOP_INSTALL_BIN_DIR="+testAssetDir,
		"KOVALOOP_LEDGER_URL=http://127.0.0.1:9",
	)
	retry := runCommand(t, env, root, "sh", "-c", firstRetry)
	if retry.code == 0 || retry.code == 127 {
		t.Fatalf("retry exit=%d stderr=%s", retry.code, retry.stderr)
	}
}

func TestAgentWalletOnboardingDocsPointToClaimLink(t *testing.T) {
	skill := readRepoFile(t, "skills", "kovaloop-ledger", "SKILL.md")
	install := readRepoFile(t, "INSTALL.md")

	for _, source := range []struct {
		name string
		body string
	}{
		{"skill", skill},
		{"install", install},
	} {
		if !strings.Contains(source.body, "kovaloop claim link") {
			t.Fatalf("%s missing kovaloop claim link", source.name)
		}
		if !strings.Contains(source.body, "owner email") {
			t.Fatalf("%s missing owner email", source.name)
		}
		if strings.Contains(source.body, "kovaloop ledger wallet get-or-create") {
			t.Fatalf("%s still references direct wallet get-or-create", source.name)
		}
	}
}

func TestKovaloopLedgerSkillRunsClaimLinkAfterInstallOrClaimCodeRequests(t *testing.T) {
	skill := readRepoFile(t, "skills", "kovaloop-ledger", "SKILL.md")
	for _, want := range []string{"installation has just completed", "reinstall", "claimCode", "kovaloop claim link"} {
		if !strings.Contains(skill, want) {
			t.Fatalf("kovaloop-ledger missing %q", want)
		}
	}
}

func TestKovaloopSkillsDoNotExposeEscrow(t *testing.T) {
	ledgerSkill := readRepoFile(t, "skills", "kovaloop-ledger", "SKILL.md")
	install := readRepoFile(t, "install.sh")
	serviceTradeSkill := filepath.Join(repoRoot, "skills", "kovaloop-a2a-service-trade", "SKILL.md")
	if _, err := os.Stat(serviceTradeSkill); !os.IsNotExist(err) {
		t.Fatalf("service trade skill should not be present: %v", err)
	}

	for _, forbidden := range []string{
		"escrow",
		"Escrow",
		"kovaloop ledger escrow",
		"kovaloop-a2a-service-trade",
	} {
		if strings.Contains(ledgerSkill, forbidden) {
			t.Fatalf("kovaloop-ledger exposes %q", forbidden)
		}
		if strings.Contains(install, forbidden) {
			t.Fatalf("installer exposes %q", forbidden)
		}
	}
}

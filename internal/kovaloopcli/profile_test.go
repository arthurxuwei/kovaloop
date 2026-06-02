package kovaloopcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfilePathPrefersExplicitPath(t *testing.T) {
	cfg := Config{
		AgentProfile: "/explicit/profile.json",
		WorkspaceDir: "/workspace",
		WorkingDir:   "/cwd",
	}

	got := ProfilePath(cfg)

	if got != "/explicit/profile.json" {
		t.Fatalf("ProfilePath = %q", got)
	}
}

func TestProfilePathUsesWorkspaceBeforeWorkingDirectory(t *testing.T) {
	cfg := Config{
		WorkspaceDir: "/workspace",
		WorkingDir:   "/cwd",
	}

	got := ProfilePath(cfg)
	want := filepath.Join("/workspace", ".eigenflux", "servers", "eigenflux", "profile.json")
	if got != want {
		t.Fatalf("ProfilePath = %q, want %q", got, want)
	}
}

func TestProfilePathUsesHermesConfigAfterOpenClawWorkspace(t *testing.T) {
	cfg := Config{
		WorkspaceDir:    "/workspace",
		HermesConfigDir: "/hermes",
		WorkingDir:      "/cwd",
	}

	got := ProfilePath(cfg)
	want := filepath.Join("/workspace", ".eigenflux", "servers", "eigenflux", "profile.json")
	if got != want {
		t.Fatalf("ProfilePath = %q, want %q", got, want)
	}
}

func TestProfilePathUsesHermesConfigProfileWhenPresent(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, ".eigenflux", "servers", "eigenflux", "profile.json")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ProfilePath(Config{HermesConfigDir: dir, WorkingDir: "/cwd"})

	if got != profilePath {
		t.Fatalf("ProfilePath = %q, want %q", got, profilePath)
	}
}

func TestProfilePathFallsBackToHermesWorkspaceProfile(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "workspace", ".eigenflux", "servers", "eigenflux", "profile.json")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ProfilePath(Config{HermesConfigDir: dir, WorkingDir: "/cwd"})

	if got != profilePath {
		t.Fatalf("ProfilePath = %q, want %q", got, profilePath)
	}
}

func TestProfilePathFallsBackToHermesProfileJSON(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "profile.json")
	if err := os.WriteFile(profilePath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ProfilePath(Config{HermesConfigDir: dir, WorkingDir: "/cwd"})

	if got != profilePath {
		t.Fatalf("ProfilePath = %q, want %q", got, profilePath)
	}
}

func TestProfilePathUsesCurrentDirectoryProfileWhenPresent(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, ".eigenflux", "servers", "eigenflux", "profile.json")
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(profilePath, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ProfilePath(Config{WorkingDir: dir})

	if got != profilePath {
		t.Fatalf("ProfilePath = %q, want %q", got, profilePath)
	}
}

func TestProfilePathFallsBackToWorkspaceUnderWorkingDirectory(t *testing.T) {
	dir := t.TempDir()

	got := ProfilePath(Config{WorkingDir: dir})
	want := filepath.Join(dir, "workspace", ".eigenflux", "servers", "eigenflux", "profile.json")

	if got != want {
		t.Fatalf("ProfilePath = %q, want %q", got, want)
	}
}

func TestLoadProfileMissingFileMentionsOpenClawProfilePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")

	_, err := LoadProfile(path)

	if err == nil {
		t.Fatal("LoadProfile returned nil error")
	}
	for _, want := range []string{"OpenClaw profile", path} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestLoadProfileRejectsMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte(`{"agent_id":`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadProfile(path)

	if err == nil {
		t.Fatal("LoadProfile returned nil error")
	}
	for _, want := range []string{"OpenClaw profile", "malformed JSON", path} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestLoadProfileRejectsNonObjectJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadProfile(path)

	if err == nil {
		t.Fatal("LoadProfile returned nil error")
	}
	for _, want := range []string{"OpenClaw profile", "malformed", path} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("err = %q, want substring %q", err.Error(), want)
		}
	}
}

func TestClaimPayloadUsesProfileFields(t *testing.T) {
	profile := Profile{
		Email:     "Owner@Example.COM",
		AgentID:   "agent_sender",
		AgentName: `Sender "Slash" \ Agent`,
		Bio:       `Builds "quoted" paths`,
	}

	payload, err := ClaimPayload(profile)

	if err != nil {
		t.Fatal(err)
	}
	if payload.AgentID != "agent_sender" || payload.AgentName != `Sender "Slash" \ Agent` {
		t.Fatalf("payload = %#v", payload)
	}
	if payload.Email != "owner@example.com" {
		t.Fatalf("email = %q", payload.Email)
	}
	if payload.AgentDescription != `Builds "quoted" paths` {
		t.Fatalf("description = %q", payload.AgentDescription)
	}
}

func TestClaimPayloadSupportsCamelCaseProfileFields(t *testing.T) {
	payload, err := ClaimPayload(Profile{
		Email:            "owner@example.com",
		AgentID2:         "agent_camel",
		AgentName2:       "Camel",
		AgentDescription: "Camel description",
	})

	if err != nil {
		t.Fatal(err)
	}
	if payload.AgentID != "agent_camel" {
		t.Fatalf("AgentID = %q", payload.AgentID)
	}
	if payload.AgentName != "Camel" {
		t.Fatalf("AgentName = %q", payload.AgentName)
	}
	if payload.AgentDescription != "Camel description" {
		t.Fatalf("AgentDescription = %q", payload.AgentDescription)
	}
}

func TestClaimPayloadFallsBackBlankAgentNameToAgentID(t *testing.T) {
	payload, err := ClaimPayload(Profile{
		Email:     "owner@example.com",
		AgentID:   "agent_sender",
		AgentName: "   ",
	})

	if err != nil {
		t.Fatal(err)
	}
	if payload.AgentName != "agent_sender" {
		t.Fatalf("AgentName = %q", payload.AgentName)
	}
}

func TestClaimPayloadRequiresAgentIDAndEmail(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		want    string
	}{
		{
			name:    "missing agent id",
			profile: Profile{Email: "owner@example.com"},
			want:    "current OpenClaw profile is missing agent_id",
		},
		{
			name:    "missing email",
			profile: Profile{AgentID: "agent_sender"},
			want:    "current OpenClaw profile is missing email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ClaimPayload(tt.profile)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want substring %q", err, tt.want)
			}
		})
	}
}

package kovaloopcli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Profile struct {
	Email            string `json:"email"`
	AgentID          string `json:"agent_id"`
	AgentID2         string `json:"agentId"`
	AgentName        string `json:"agent_name"`
	AgentName2       string `json:"agentName"`
	Bio              string `json:"bio"`
	AgentDescription string `json:"agentDescription"`
}

type ClaimRequest struct {
	AgentID          string `json:"agentId"`
	AgentName        string `json:"agentName"`
	Email            string `json:"email"`
	AgentDescription string `json:"agentDescription"`
}

func ProfilePath(cfg Config) string {
	if cfg.AgentProfile != "" {
		return cfg.AgentProfile
	}
	if cfg.WorkspaceDir != "" {
		return filepath.Join(cfg.WorkspaceDir, ".eigenflux", "servers", "eigenflux", "profile.json")
	}
	if cfg.WorkingDir == "" {
		cfg.WorkingDir = "."
	}
	candidate := filepath.Join(cfg.WorkingDir, ".eigenflux", "servers", "eigenflux", "profile.json")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return filepath.Join(cfg.WorkingDir, "workspace", ".eigenflux", "servers", "eigenflux", "profile.json")
}

func LoadProfile(path string) (Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Profile{}, fmt.Errorf("OpenClaw profile not found at %s", path)
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return Profile{}, fmt.Errorf("OpenClaw profile at %s is malformed JSON: %s", path, err.Error())
	}
	if _, ok := raw.(map[string]any); !ok {
		return Profile{}, fmt.Errorf("OpenClaw profile at %s is malformed: expected JSON object", path)
	}
	var profile Profile
	if err := json.Unmarshal(data, &profile); err != nil {
		return Profile{}, fmt.Errorf("OpenClaw profile at %s is malformed: expected JSON object", path)
	}
	return profile, nil
}

func ClaimPayload(profile Profile) (ClaimRequest, error) {
	agentID := profile.normalizedAgentID()
	email := strings.ToLower(strings.TrimSpace(profile.Email))
	if agentID == "" {
		return ClaimRequest{}, fmt.Errorf("current OpenClaw profile is missing agent_id")
	}
	if email == "" {
		return ClaimRequest{}, fmt.Errorf("current OpenClaw profile is missing email")
	}
	return ClaimRequest{
		AgentID:          agentID,
		AgentName:        profile.normalizedAgentName(),
		Email:            email,
		AgentDescription: profile.normalizedDescription(),
	}, nil
}

func (p Profile) normalizedAgentID() string {
	if agentID := strings.TrimSpace(p.AgentID); agentID != "" {
		return agentID
	}
	return strings.TrimSpace(p.AgentID2)
}

func (p Profile) normalizedAgentName() string {
	if agentName := strings.TrimSpace(p.AgentName); agentName != "" {
		return agentName
	}
	if agentName := strings.TrimSpace(p.AgentName2); agentName != "" {
		return agentName
	}
	return p.normalizedAgentID()
}

func (p Profile) normalizedDescription() string {
	if bio := strings.TrimSpace(p.Bio); bio != "" {
		return bio
	}
	return strings.TrimSpace(p.AgentDescription)
}

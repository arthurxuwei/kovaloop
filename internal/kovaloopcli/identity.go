package kovaloopcli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LocalProfile is the minimal identity file written to the volume; the server is authoritative.
type LocalProfile struct {
	SchemaVersion int    `json:"schemaVersion"`
	AgentID       string `json:"agentId"`
	AgentName     string `json:"agentName,omitempty"`
}

// Credentials holds the device keypair. The private key (seed) never leaves the volume.
type Credentials struct {
	SchemaVersion  int    `json:"schemaVersion"`
	AgentID        string `json:"agentId"`
	PublicKey      string `json:"publicKey"`
	PrivateKeySeed string `json:"privateKeySeed"`
	CreatedAt      string `json:"createdAt"`
}

func b64urlEncode(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func b64urlDecode(s string) ([]byte, error) { return base64.RawURLEncoding.DecodeString(s) }

func newKeypair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}

// PrivateKey reconstructs the Ed25519 private key from the stored seed.
func (c Credentials) PrivateKey() (ed25519.PrivateKey, error) {
	seed, err := b64urlDecode(c.PrivateKeySeed)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid private key seed")
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func SaveCredentials(path string, c Credentials) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func LoadCredentials(path string) (Credentials, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return Credentials{}, err
	}
	return c, nil
}

func LoadLocalProfile(path string) (LocalProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LocalProfile{}, err
	}
	var p LocalProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return LocalProfile{}, err
	}
	return p, nil
}

func SaveLocalProfile(path string, p LocalProfile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// kovaloopHomeRoot returns the directory under which .kovaloop is created.
// It anchors to OpenClaw's own config dir ($HOME/.openclaw) — independent of
// where EigenFlux stores its data — and is overridable with KOVALOOP_HOME.
func kovaloopHomeRoot(cfg Config) string {
	if cfg.KovaloopHome != "" {
		return cfg.KovaloopHome
	}
	if cfg.Home != "" {
		return filepath.Join(cfg.Home, ".openclaw")
	}
	return "."
}

// KovaloopDir returns the .kovaloop directory anchored to the durable config volume.
func KovaloopDir(cfg Config) string { return filepath.Join(kovaloopHomeRoot(cfg), ".kovaloop") }

func ProfileJSONPath(cfg Config) string { return filepath.Join(KovaloopDir(cfg), "profile.json") }

func CredentialsJSONPath(cfg Config) string {
	return filepath.Join(KovaloopDir(cfg), "credentials.json")
}

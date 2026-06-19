package kovaloopcli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateAndPersistCredentials(t *testing.T) {
	dir := t.TempDir()
	pub, priv, err := newKeypair()
	if err != nil {
		t.Fatal(err)
	}
	creds := Credentials{
		SchemaVersion:  1,
		AgentID:        "kloop_agent_X",
		PublicKey:      b64urlEncode(pub),
		PrivateKeySeed: b64urlEncode(priv.Seed()),
		CreatedAt:      "t",
	}
	path := filepath.Join(dir, "nested", "credentials.json")
	if err := SaveCredentials(path, creds); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm %v", info.Mode().Perm())
	}
	loaded, err := LoadCredentials(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PublicKey != creds.PublicKey {
		t.Fatal("pubkey mismatch")
	}
	if _, err := loaded.PrivateKey(); err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
}

func TestKovaloopDirAnchorsToHomeOpenclaw(t *testing.T) {
	// Anchors to OpenClaw's own config dir, independent of EigenFlux location.
	cfg := Config{Home: "/root", EigenfluxHome: "/home/node/.openclaw/.eigenflux"}
	if got := KovaloopDir(cfg); got != "/root/.openclaw/.kovaloop" {
		t.Fatalf("got %q", got)
	}
}

func TestKovaloopDirHomeOverride(t *testing.T) {
	cfg := Config{KovaloopHome: "/custom", Home: "/root"}
	if got := KovaloopDir(cfg); got != "/custom/.kovaloop" {
		t.Fatalf("override: got %q", got)
	}
}

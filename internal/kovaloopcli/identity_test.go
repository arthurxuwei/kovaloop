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

func TestKovaloopDirPrefersConfigRoot(t *testing.T) {
	cfg := Config{WorkspaceDir: "/home/node/.openclaw/workspace"}
	if got := KovaloopDir(cfg); got != "/home/node/.openclaw/.kovaloop" {
		t.Fatalf("got %q", got)
	}
	cfg2 := Config{KovaloopHome: "/custom", WorkspaceDir: "/ws"}
	if got := KovaloopDir(cfg2); got != "/custom/.kovaloop" {
		t.Fatalf("override: got %q", got)
	}
}

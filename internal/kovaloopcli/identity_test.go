package kovaloopcli

import "testing"

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

package kovaloopcli

import (
	"crypto/ed25519"
	"testing"
)

func TestSignBodyVerifies(t *testing.T) {
	pub, priv, _ := newKeypair()
	msg := SigningMessage("kloop_agent_X", "2026-06-18T00:00:00Z", "n1", "{}")
	if msg != "kloop_agent_X\n2026-06-18T00:00:00Z\nn1\n{}" {
		t.Fatalf("msg %q", msg)
	}
	sig := SignBody(priv, "kloop_agent_X", "2026-06-18T00:00:00Z", "n1", "{}")
	raw, err := b64urlDecode(sig)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pub, []byte(msg), raw) {
		t.Fatal("verify failed")
	}
}

func TestNonceAndTimestamp(t *testing.T) {
	n1, err := NewNonce()
	if err != nil || n1 == "" {
		t.Fatalf("nonce: %v %q", err, n1)
	}
	n2, _ := NewNonce()
	if n1 == n2 {
		t.Fatal("nonces must differ")
	}
	if ts := NowTimestamp(); len(ts) < 20 || ts[len(ts)-1] != 'Z' {
		t.Fatalf("timestamp not RFC3339 UTC: %q", ts)
	}
}

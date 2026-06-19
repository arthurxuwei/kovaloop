package kovaloopcli

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// SigningMessage builds the canonical message the ledger server verifies. It MUST
// byte-match the server's agent_auth.signing_message: agentId\ntimestamp\nnonce\nbody.
func SigningMessage(agentID, timestamp, nonce, body string) string {
	return agentID + "\n" + timestamp + "\n" + nonce + "\n" + body
}

// SignBody signs the canonical message and returns a base64url (no padding) signature.
func SignBody(priv ed25519.PrivateKey, agentID, timestamp, nonce, body string) string {
	return b64urlEncode(ed25519.Sign(priv, []byte(SigningMessage(agentID, timestamp, nonce, body))))
}

func NewNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func NowTimestamp() string { return time.Now().UTC().Format(time.RFC3339) }

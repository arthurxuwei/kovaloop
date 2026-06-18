package kovaloopcli

import (
	"fmt"
	"io"
)

type createProfileRequest struct {
	AgentName        string         `json:"agentName"`
	OwnerEmail       string         `json:"ownerEmail,omitempty"`
	Description      string         `json:"description,omitempty"`
	Eigenflux        map[string]any `json:"eigenflux,omitempty"`
	CredentialPubKey string         `json:"credentialPublicKey"`
}

type profileEnvelope struct {
	Profile struct {
		AgentID   string `json:"agentId"`
		AgentName string `json:"agentName"`
	} `json:"profile"`
}

func runProfile(args []string, stdout io.Writer, stderr io.Writer, cfg Config) int {
	if len(args) == 0 || isHelpArg(args[0]) {
		fmt.Fprint(stdout, usageText)
		return 0
	}
	switch args[0] {
	case "create":
		return runProfileCreate(cfg, stdout, stderr)
	case "update":
		if len(args) < 2 {
			fmt.Fprintln(stderr, `usage: kovaloop profile update '{"description":"..."}'`)
			return 2
		}
		return runProfileUpdate(cfg, args[1], stdout, stderr)
	case "show":
		return runProfileShow(cfg, stdout, stderr)
	default:
		fmt.Fprint(stderr, usageText)
		return 2
	}
}

func runProfileCreate(cfg Config, stdout io.Writer, stderr io.Writer) int {
	credPath := CredentialsJSONPath(cfg)
	if existing, err := LoadCredentials(credPath); err == nil && existing.AgentID != "" {
		fmt.Fprintf(stdout, "Profile already exists: %s\n", existing.AgentID)
		return 0
	}

	pub, priv, err := newKeypair()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	req := createProfileRequest{AgentName: "OntologyAgent", CredentialPubKey: b64urlEncode(pub)}
	if prof, perr := LoadProfile(ProfilePath(cfg)); perr == nil {
		if name := prof.normalizedAgentName(); name != "" {
			req.AgentName = name
		}
		req.Description = prof.normalizedDescription()
		req.OwnerEmail = prof.Email
		ef := map[string]any{}
		if id := prof.normalizedAgentID(); id != "" {
			ef["id"] = id
		}
		if prof.Email != "" {
			ef["email"] = prof.Email
		}
		if name := prof.normalizedAgentName(); name != "" {
			ef["name"] = name
		}
		if bio := prof.normalizedDescription(); bio != "" {
			ef["bio"] = bio
		}
		if len(ef) > 0 {
			req.Eigenflux = ef
		}
	}

	var env profileEnvelope
	if err := postJSON(cfg, "/ledger/profiles", req, &env); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	if env.Profile.AgentID == "" {
		fmt.Fprintln(stderr, "ledger response missing agentId")
		return 1
	}

	if err := SaveLocalProfile(ProfileJSONPath(cfg), LocalProfile{
		SchemaVersion: 1,
		AgentID:       env.Profile.AgentID,
		AgentName:     env.Profile.AgentName,
	}); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	if err := SaveCredentials(credPath, Credentials{
		SchemaVersion:  1,
		AgentID:        env.Profile.AgentID,
		PublicKey:      b64urlEncode(pub),
		PrivateKeySeed: b64urlEncode(priv.Seed()),
		CreatedAt:      NowTimestamp(),
	}); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	fmt.Fprintf(stdout, "Profile created: %s\n", env.Profile.AgentID)
	return 0
}

func runProfileUpdate(cfg Config, body string, stdout io.Writer, stderr io.Writer) int {
	creds, err := LoadCredentials(CredentialsJSONPath(cfg))
	if err != nil {
		fmt.Fprintln(stderr, "no local credentials; run 'kovaloop profile create' first")
		return 1
	}
	priv, err := creds.PrivateKey()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	ts := NowTimestamp()
	nonce, err := NewNonce()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	headers := map[string]string{
		"X-KovaLoop-Agent-Id":  creds.AgentID,
		"X-KovaLoop-Timestamp": ts,
		"X-KovaLoop-Nonce":     nonce,
		"X-KovaLoop-Signature": SignBody(priv, creds.AgentID, ts, nonce, body),
	}
	resp, err := patchRaw(cfg, "/ledger/profiles/"+creds.AgentID, []byte(body), headers)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printRawResponse(stdout, resp)
	return 0
}

func runProfileShow(cfg Config, stdout io.Writer, stderr io.Writer) int {
	creds, err := LoadCredentials(CredentialsJSONPath(cfg))
	if err != nil {
		fmt.Fprintln(stderr, "no local credentials; run 'kovaloop profile create' first")
		return 1
	}
	resp, err := getRaw(cfg, "/ledger/profiles/"+creds.AgentID)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printRawResponse(stdout, resp)
	return 0
}

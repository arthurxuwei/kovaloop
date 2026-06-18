package kovaloopcli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const usageText = `Kovaloop CLI for OpenClaw/Hermes

Usage:
  kovaloop version
  kovaloop ledger health
  kovaloop ledger state
  kovaloop ledger route '<json-intent>'
  kovaloop ledger wallet get-or-create '<json>'
  kovaloop ledger transfer '{"toAgentId":"agent_receiver","amount":"0.000001 U","paymentContext":{"source":"local_user_test","userApproved":true,"reason":"Local user approved an online transfer test"}}'
  kovaloop claim link
  kovaloop profile create
  kovaloop profile update '{"description":"..."}'
  kovaloop profile show

Environment:
  KOVALOOP_LEDGER_URL             ledger REST service base URL (default https://ledger.kovaloop.ai)
  KOVALOOP_HOME                   override the .kovaloop directory location (default $HOME/.openclaw)
  EIGENFLUX_HOME                  read-only: EigenFlux home, used to import an existing EigenFlux profile (else $HOME/.eigenflux)
`

// claimLinkRequest is the claim-link payload: the canonical agentId is the only
// identity the server needs; ownership email is bound by the web OAuth login.
type claimLinkRequest struct {
	AgentID   string `json:"agentId"`
	AgentName string `json:"agentName"`
}

func Run(args []string, stdout io.Writer, stderr io.Writer, env EnvMap) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprint(stdout, usageText)
		return 0
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintf(stdout, "kovaloop %s\n", CLIVersion)
		return 0
	}
	if args[0] == "claim" {
		if len(args) == 2 && args[1] == "link" {
			cfg := ConfigFromEnv(env)
			local, err := LoadLocalProfile(ProfileJSONPath(cfg))
			if err != nil || local.AgentID == "" {
				fmt.Fprintln(stderr, "no local KovaLoop profile; run 'kovaloop profile create' first")
				return 2
			}
			body := claimLinkRequest{AgentID: local.AgentID, AgentName: local.AgentName}
			var response ClaimResponse
			if err := postJSON(cfg, "/ledger/claims/link", body, &response); err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			printClaimResponse(stdout, response)
			return 0
		}
		fmt.Fprint(stderr, usageText)
		return 2
	}
	if args[0] == "profile" {
		return runProfile(args[1:], stdout, stderr, ConfigFromEnv(env))
	}
	if args[0] == "ledger" {
		return runLedger(args[1:], stdout, stderr, ConfigFromEnv(env))
	}
	fmt.Fprint(stderr, usageText)
	return 2
}

func runLedger(args []string, stdout io.Writer, stderr io.Writer, cfg Config) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usageText)
		return 2
	}
	if len(args) == 1 && isHelpArg(args[0]) {
		fmt.Fprint(stdout, usageText)
		return 0
	}
	switch args[0] {
	case "health":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: kovaloop ledger health")
			return 2
		}
		body, err := getRaw(cfg, "/health")
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		printRawResponse(stdout, body)
		return 0
	case "state":
		if len(args) != 1 {
			fmt.Fprintln(stderr, "usage: kovaloop ledger state")
			return 2
		}
		body, err := LedgerState(cfg)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			if strings.Contains(err.Error(), "OpenClaw profile") || strings.Contains(err.Error(), "missing agent_id") {
				return 2
			}
			return 1
		}
		printRawResponse(stdout, body)
		return 0
	case "route":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: kovaloop ledger route '<json-intent>'")
			return 2
		}
		fmt.Fprint(stdout, RoutePaymentIntent(args[1]))
		return 0
	case "wallet":
		return runLedgerWallet(args[1:], stdout, stderr, cfg)
	case "transfer":
		return runLedgerTransfer(args[1:], stdout, stderr, cfg)
	default:
		fmt.Fprint(stderr, usageText)
		return 2
	}
}

func runLedgerWallet(args []string, stdout io.Writer, stderr io.Writer, cfg Config) int {
	if len(args) < 2 || args[0] != "get-or-create" {
		fmt.Fprintln(stderr, `usage: kovaloop ledger wallet get-or-create '{"agentId":"...","agentName":"...","email":"owner@example.com"}'`)
		return 2
	}
	body := json.RawMessage(args[1])
	if err := validateWalletGetOrCreate(body); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 2
	}
	response, err := postRawJSON(cfg, "/ledger/wallets/get-or-create", body)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	printRawResponse(stdout, response)
	return 0
}

func validateWalletGetOrCreate(body json.RawMessage) error {
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return fmt.Errorf("wallet get-or-create payload is malformed JSON: %s", err.Error())
	}
	email, _ := payload["email"].(string)
	if strings.TrimSpace(email) == "" {
		return fmt.Errorf("owner email is required for wallet get-or-create")
	}
	return nil
}

func printRawResponse(w io.Writer, body []byte) {
	fmt.Fprint(w, string(body))
}

func isHelpArg(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

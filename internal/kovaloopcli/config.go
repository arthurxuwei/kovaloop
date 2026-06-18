package kovaloopcli

import "os"

const CLIVersion = "2026.06.17.3"

type EnvMap map[string]string

func ProcessEnv() EnvMap {
	env := EnvMap{}
	for _, key := range []string{
		"KOVALOOP_LEDGER_URL",
		"KOVALOOP_LEDGER_HTTP_URL",
		"KOVALOOP_LEDGER_FALLBACK_URL",
		"KOVALOOP_AGENT_PROFILE_PATH",
		"KOVALOOP_HOME",
		"OPENCLAW_WORKSPACE_DIR",
		"HERMES_CONFIG_DIR",
		"PWD",
	} {
		if value, ok := os.LookupEnv(key); ok {
			env[key] = value
		}
	}
	if _, ok := env["PWD"]; !ok {
		if wd, err := os.Getwd(); err == nil {
			env["PWD"] = wd
		}
	}
	return env
}

type Config struct {
	LedgerURL       string
	LedgerFallback  string
	AgentProfile    string
	KovaloopHome    string
	WorkspaceDir    string
	HermesConfigDir string
	WorkingDir      string
}

func ConfigFromEnv(env EnvMap) Config {
	ledgerURL := env["KOVALOOP_LEDGER_HTTP_URL"]
	if ledgerURL == "" {
		ledgerURL = env["KOVALOOP_LEDGER_URL"]
	}
	if ledgerURL == "" {
		ledgerURL = "https://ledger.kovaloop.ai"
	}
	return Config{
		LedgerURL:       ledgerURL,
		LedgerFallback:  env["KOVALOOP_LEDGER_FALLBACK_URL"],
		AgentProfile:    env["KOVALOOP_AGENT_PROFILE_PATH"],
		KovaloopHome:    env["KOVALOOP_HOME"],
		WorkspaceDir:    env["OPENCLAW_WORKSPACE_DIR"],
		HermesConfigDir: env["HERMES_CONFIG_DIR"],
		WorkingDir:      env["PWD"],
	}
}

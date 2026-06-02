package kovaloopcli

import "testing"

func TestConfigFromEnvReadsHermesConfigDir(t *testing.T) {
	cfg := ConfigFromEnv(EnvMap{
		"HERMES_CONFIG_DIR":        "/hermes/config",
		"KOVALOOP_LEDGER_HTTP_URL": "https://ledger.example.test",
	})

	if cfg.HermesConfigDir != "/hermes/config" {
		t.Fatalf("HermesConfigDir = %q", cfg.HermesConfigDir)
	}
	if cfg.LedgerURL != "https://ledger.example.test" {
		t.Fatalf("LedgerURL = %q", cfg.LedgerURL)
	}
}

package kovaloopcli

import "testing"

func TestConfigFromEnvReadsIdentityAndLedger(t *testing.T) {
	cfg := ConfigFromEnv(EnvMap{
		"KOVALOOP_LEDGER_HTTP_URL": "https://ledger.example.test",
		"EIGENFLUX_HOME":           "/home/node/.openclaw/.eigenflux",
		"HOME":                     "/root",
	})

	if cfg.LedgerURL != "https://ledger.example.test" {
		t.Fatalf("LedgerURL = %q", cfg.LedgerURL)
	}
	if cfg.EigenfluxHome != "/home/node/.openclaw/.eigenflux" {
		t.Fatalf("EigenfluxHome = %q", cfg.EigenfluxHome)
	}
	if cfg.Home != "/root" {
		t.Fatalf("Home = %q", cfg.Home)
	}
}

func TestConfigFromEnvDefaultsLedgerURL(t *testing.T) {
	cfg := ConfigFromEnv(EnvMap{})
	if cfg.LedgerURL != "https://ledger.kovaloop.ai" {
		t.Fatalf("LedgerURL = %q", cfg.LedgerURL)
	}
}

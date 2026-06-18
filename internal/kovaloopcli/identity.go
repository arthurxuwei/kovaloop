package kovaloopcli

import "path/filepath"

func kovaloopHomeRoot(cfg Config) string {
	if cfg.KovaloopHome != "" {
		return cfg.KovaloopHome
	}
	if cfg.WorkspaceDir != "" {
		return filepath.Dir(cfg.WorkspaceDir) // config volume root (parent of workspace)
	}
	if cfg.HermesConfigDir != "" {
		return cfg.HermesConfigDir
	}
	if cfg.WorkingDir != "" {
		return cfg.WorkingDir
	}
	return "."
}

// KovaloopDir returns the .kovaloop directory anchored to the durable config volume.
func KovaloopDir(cfg Config) string { return filepath.Join(kovaloopHomeRoot(cfg), ".kovaloop") }

func ProfileJSONPath(cfg Config) string { return filepath.Join(KovaloopDir(cfg), "profile.json") }

func CredentialsJSONPath(cfg Config) string {
	return filepath.Join(KovaloopDir(cfg), "credentials.json")
}

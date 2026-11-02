package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds CLI configuration persisted to ~/.valtag/config.toml.
type Config struct {
	APIURL    string `toml:"api_url"`
	ProjectID string `toml:"project_id,omitempty"`
}

func defaultAPIURL() string { return "https://api.valt.dev" }

// Load reads config from ~/.valtag/config.toml. Returns defaults if file doesn't exist.
func Load() (*Config, error) {
	cfg := &Config{APIURL: defaultAPIURL()}
	path, err := configPath()
	if err != nil {
		return cfg, nil // best-effort: return defaults
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	// Env overrides
	if v := os.Getenv("VALTAG_API_URL"); v != "" {
		cfg.APIURL = v
	}
	if v := os.Getenv("VALT_PROJECT_ID"); v != "" {
		cfg.ProjectID = v
	}
	return cfg, nil
}

// Save writes config to ~/.valtag/config.toml, creating the directory if needed.
func Save(cfg *Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("opening config file: %w", err)
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("finding home dir: %w", err)
	}
	return filepath.Join(home, ".valtag", "config.toml"), nil
}

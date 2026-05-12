package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the global mkdev configuration loaded from ~/.mkdev/config.toml.
type Config struct {
	TLD          string `toml:"tld"`
	ProxyPort    int    `toml:"proxy_port"`
	Theme        string `toml:"theme"`
	LogRetention string `toml:"log_retention"` // e.g. "7d"
	LogMaxSize   string `toml:"log_max_size"`  // e.g. "100MB"
}

// Default returns a Config populated with built-in defaults.
func Default() Config {
	return Config{
		TLD:          ".local",
		ProxyPort:    443,
		Theme:        "auto",
		LogRetention: "7d",
		LogMaxSize:   "100MB",
	}
}

// Load reads the config file at path. If the file does not exist, Default is
// returned. Malformed TOML or invalid field values return an error.
func Load(path string) (Config, error) {
	c := Default()
	_, err := toml.DecodeFile(path, &c)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		c = Default()
	default:
		return Config{}, fmt.Errorf("config: decode %s: %w", path, err)
	}
	if c.ProxyPort < 1 || c.ProxyPort > 65535 {
		return Config{}, fmt.Errorf("config: proxy_port out of range: %d", c.ProxyPort)
	}
	return c, nil
}

// Save writes the config to path with 0600 perms.
func Save(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("config: open: %w", err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(c); err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}
	return nil
}

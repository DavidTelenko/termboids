package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// KeyBindings defines keyboard shortcuts for various actions
type KeyBindings struct {
	CycleColorMode string `toml:"cycle_color_mode"`
	Quit           string `toml:"quit"`
}

// Config holds all application configuration
type Config struct {
	KeyBindings KeyBindings `toml:"keybindings"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		KeyBindings: KeyBindings{
			CycleColorMode: "c",
			Quit:           "q",
		},
	}
}

// Load reads configuration from a TOML file
// If the file doesn't exist, it returns the default configuration
func Load(path string) (Config, error) {
	// If no path provided, check for config.toml in current directory
	if path == "" {
		path = "config.toml"
	}

	// If file doesn't exist, return default config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// LoadOrDefault attempts to load config from standard locations
// Falls back to default if no config file is found
func LoadOrDefault() (Config, error) {
	// Try current directory first
	cfg, err := Load("config.toml")
	if err != nil {
		return Config{}, err
	}

	// If no config file was found but no error, try home directory
	if cfg.KeyBindings.CycleColorMode == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			homeCfg := filepath.Join(homeDir, ".config", "termboids", "config.toml")
			cfg, err = Load(homeCfg)
			if err != nil {
				return Config{}, err
			}
		}
	}

	// If still no config, ensure we have defaults
	if cfg.KeyBindings.CycleColorMode == "" {
		cfg = DefaultConfig()
	}

	return cfg, nil
}

// Save writes the configuration to a TOML file
func (c *Config) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	return encoder.Encode(c)
}

package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// KeyBindings defines keyboard shortcuts for various actions
type KeyBindings struct {
	CycleColorMode string `toml:"cycle_color_mode"`
	DebugGrid      string `toml:"debug_grid"`
	ToggleHelp     string `toml:"toggle_help"`
	Quit           string `toml:"quit"`
}

type RenderingConfig struct {
	UseGPU bool `toml:"use_gpu"`
	FPS    int  `toml:"fps"`
}

// BoidsConfig holds boid simulation parameters
type BoidsConfig struct {
	NumBoids         int     `toml:"num_boids"`
	MaxSpeed         float64 `toml:"max_speed"`
	MaxForce         float64 `toml:"max_force"`
	SeparationRadius float64 `toml:"separation_radius"`
	AlignmentRadius  float64 `toml:"alignment_radius"`
	CohesionRadius   float64 `toml:"cohesion_radius"`
	SeparationWeight float64 `toml:"separation_weight"`
	AlignmentWeight  float64 `toml:"alignment_weight"`
	CohesionWeight   float64 `toml:"cohesion_weight"`
	RandomWeight     float64 `toml:"random_weight"`
	RenderRadius     int     `toml:"render_radius"`
}

// Config holds all application configuration
type Config struct {
	KeyBindings KeyBindings     `toml:"keybindings"`
	FPS         int             `toml:"fps"`
	Boids       BoidsConfig     `toml:"boids"`
	Rendering   RenderingConfig `toml:"rendering"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		KeyBindings: KeyBindings{
			CycleColorMode: "c",
			DebugGrid:      "d",
			ToggleHelp:     "h",
			Quit:           "q",
		},
		Boids: BoidsConfig{
			NumBoids:         1000,
			MaxSpeed:         50.0,
			MaxForce:         80.0,
			SeparationRadius: 5.0,
			AlignmentRadius:  35.0,
			CohesionRadius:   45.0,
			SeparationWeight: 1.8,
			AlignmentWeight:  1.2,
			CohesionWeight:   1.0,
			RandomWeight:     0.15,
			RenderRadius:     1,
		},
		Rendering: RenderingConfig{
			UseGPU: true, // Enable GPU by default
			FPS:    60,   // Enable GPU by default
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

	// Start with default config
	cfg := DefaultConfig()

	// If file doesn't exist, return default config
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}

	// Decode TOML file, merging with defaults
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

	// If config was loaded from file, return it
	// (Load now returns defaults if file doesn't exist)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		homeCfg := filepath.Join(homeDir, ".config", "termboids", "config.toml")
		if _, err := os.Stat(homeCfg); err == nil {
			// Home config exists, try to load it
			cfg, err = Load(homeCfg)
			if err != nil {
				return Config{}, err
			}
		}
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

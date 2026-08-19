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
	ShowConfig     string `toml:"show_config"`
	Pause          string `toml:"pause"`
}

// Preset defines a boid configuration preset that can be loaded via keybinding
type Preset struct {
	Key  string `toml:"key"`
	Name string `toml:"name"`
	Path string `toml:"path"`
}

type RenderingConfig struct {
	UseGPU bool `toml:"use_gpu"`
	FPS    int  `toml:"fps"`
}

// RepellantConfig holds mouse repellant interaction settings
type RepellantConfig struct {
	Radius   float64 `toml:"radius"`
	Strength float64 `toml:"strength"`
	Duration float64 `toml:"duration"`
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

// SystemConfig holds system-wide configuration (rendering, keybindings, presets)
type SystemConfig struct {
	KeyBindings KeyBindings     `toml:"keybindings"`
	Rendering   RenderingConfig `toml:"rendering"`
	Repellant   RepellantConfig `toml:"repellant"`
	Presets     []Preset        `toml:"presets"`
}

// Config holds all application configuration
type Config struct {
	System SystemConfig
	Boids  BoidsConfig
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		System: SystemConfig{
			KeyBindings: KeyBindings{
				CycleColorMode: "c",
				DebugGrid:      "d",
				ToggleHelp:     "h",
				Quit:           "q",
				ShowConfig:     "i",
				Pause:          "p",
			},
			Rendering: RenderingConfig{
				UseGPU: true,
				FPS:    60,
			},
			Repellant: RepellantConfig{
				Radius:   200.0,
				Strength: 10.0,
				Duration: 2.0,
			},
			Presets: []Preset{},
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
	}
}

// Load reads configuration from TOML files
// Loads system config from system.toml (or legacy config.toml)
// and boids config from boids.toml (or legacy config.toml)
func Load(systemPath, boidsPath string) (Config, error) {
	// Start with default config
	cfg := DefaultConfig()

	// Load system config
	if systemPath == "" {
		systemPath = "system.toml"
	}
	
	// Try loading system config (fall back to config.toml for backward compatibility)
	if _, err := os.Stat(systemPath); err == nil {
		if _, err := toml.DecodeFile(systemPath, &cfg.System); err != nil {
			return Config{}, err
		}
	} else if _, err := os.Stat("config.toml"); err == nil {
		// Backward compatibility: load from old config.toml format
		var legacyConfig struct {
			KeyBindings KeyBindings     `toml:"keybindings"`
			Rendering   RenderingConfig `toml:"rendering"`
		}
		if _, err := toml.DecodeFile("config.toml", &legacyConfig); err != nil {
			return Config{}, err
		}
		cfg.System.KeyBindings = legacyConfig.KeyBindings
		cfg.System.Rendering = legacyConfig.Rendering
	}

	// Load boids config
	if boidsPath == "" {
		boidsPath = "boids.toml"
	}
	
	// Try loading boids config (fall back to config.toml for backward compatibility)
	if _, err := os.Stat(boidsPath); err == nil {
		var boidsConfig struct {
			Boids BoidsConfig `toml:"boids"`
		}
		if _, err := toml.DecodeFile(boidsPath, &boidsConfig); err != nil {
			return Config{}, err
		}
		cfg.Boids = boidsConfig.Boids
	} else if _, err := os.Stat("config.toml"); err == nil {
		// Backward compatibility: load from old config.toml format
		var legacyConfig struct {
			Boids BoidsConfig `toml:"boids"`
		}
		if _, err := toml.DecodeFile("config.toml", &legacyConfig); err != nil {
			return Config{}, err
		}
		cfg.Boids = legacyConfig.Boids
	}

	return cfg, nil
}

// LoadBoidsConfig loads only the boids configuration from a file
func LoadBoidsConfig(path string) (BoidsConfig, error) {
	cfg := DefaultConfig().Boids
	
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, err
	}

	var boidsConfig struct {
		Boids BoidsConfig `toml:"boids"`
	}
	
	if _, err := toml.DecodeFile(path, &boidsConfig); err != nil {
		return BoidsConfig{}, err
	}

	return boidsConfig.Boids, nil
}

// LoadOrDefault attempts to load config from standard locations
// Falls back to default if no config file is found
func LoadOrDefault() (Config, error) {
	// Try current directory first
	cfg, err := Load("", "")
	if err != nil {
		return Config{}, err
	}

	// Try home directory config
	homeDir, err := os.UserHomeDir()
	if err == nil {
		homeSystemCfg := filepath.Join(homeDir, ".config", "termboids", "system.toml")
		homeBoidsCfg := filepath.Join(homeDir, ".config", "termboids", "boids.toml")
		
		if _, err := os.Stat(homeSystemCfg); err == nil {
			cfg, err = Load(homeSystemCfg, homeBoidsCfg)
			if err != nil {
				return Config{}, err
			}
		}
	}

	return cfg, nil
}


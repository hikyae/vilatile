package main

import (
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

type Position struct {
	X string `yaml:"x"`
	Y string `yaml:"y"`
}

type Config struct {
	GvimPath    string   `yaml:"gvim_path"`
	Columns     int      `yaml:"columns"`
	Lines       int      `yaml:"lines"`
	Position    Position `yaml:"position"`
	GlazeWMPort int      `yaml:"glazewm_port"`
}

func defaultConfig() *Config {
	return &Config{
		Columns: 80,
		Lines:   24,
		Position: Position{
			X: "left",
			Y: "top",
		},
		GlazeWMPort: 6123,
	}
}

func defaultConfigPath() string {
	if runtime.GOOS == "windows" {
		appdata := os.Getenv("APPDATA")
		if appdata == "" {
			appdata = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
		return filepath.Join(appdata, "vilatile", "config.yaml")
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(configHome, "vilatile", "config.yaml")
}

func loadConfig(configPath string) (*Config, error) {
	cfg := defaultConfig()

	if configPath == "" {
		configPath = defaultConfigPath()
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

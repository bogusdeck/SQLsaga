// Package utils owns the on-disk user config and the device id used to track
// progress without a login.
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config is the YAML persisted at ~/.sqlsaga/config.yaml.
type Config struct {
	Theme         string         `yaml:"theme"`
	Autocomplete  bool           `yaml:"autocomplete"`
	SyncEnabled   bool           `yaml:"sync_enabled"`
	DeviceID      string         `yaml:"device_id"`
	Editor        EditorSettings `yaml:"editor"`
	StoryPrefs    []string       `yaml:"story_preferences"`
	MySQLDSN      string         `yaml:"mysql_dsn"`
}

// EditorSettings controls the SQL editor pane.
type EditorSettings struct {
	TabWidth        int  `yaml:"tab_width"`
	ShowLineNumbers bool `yaml:"show_line_numbers"`
	AutoIndent      bool `yaml:"auto_indent"`
}

var (
	configOnce sync.Mutex
)

// Dir returns the SQL Saga config directory, creating it if needed.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	dir := filepath.Join(home, ".sqlsaga")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", dir, err)
	}
	return dir, nil
}

// DBPath returns the default local SQLite path.
func DBPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sqlsaga.db"), nil
}

// Load reads the config from disk, creating a default one if missing.
func Load() (*Config, error) {
	configOnce.Lock()
	defer configOnce.Unlock()

	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("read config: %w", err)
		}
		cfg := defaultConfig()
		if err := writeConfig(path, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.DeviceID == "" {
		id, err := newDeviceID()
		if err != nil {
			return nil, err
		}
		cfg.DeviceID = id
		if err := writeConfig(path, &cfg); err != nil {
			return nil, err
		}
	}
	if cfg.Editor.TabWidth == 0 {
		cfg.Editor.TabWidth = 2
	}
	return &cfg, nil
}

// Save writes the config back to disk.
func Save(c *Config) error {
	configOnce.Lock()
	defer configOnce.Unlock()
	dir, err := Dir()
	if err != nil {
		return err
	}
	return writeConfig(filepath.Join(dir, "config.yaml"), c)
}

func writeConfig(path string, c *Config) error {
	out, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func defaultConfig() *Config {
	id, err := newDeviceID()
	if err != nil {
		// Fall back to a deterministic placeholder; will be retried on next Load.
		id = "unknown-device"
	}
	return &Config{
		Theme:        "dark",
		Autocomplete: true,
		SyncEnabled:  false,
		DeviceID:     id,
		Editor: EditorSettings{
			TabWidth:        2,
			ShowLineNumbers: true,
			AutoIndent:      true,
		},
		StoryPrefs: []string{"mystery"},
	}
}

func newDeviceID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("device id: %w", err)
	}
	return "device-" + hex.EncodeToString(b[:]), nil
}

// StoriesDir returns the directory for locally installed stories, creating it if needed.
func StoriesDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	sdir := filepath.Join(dir, "stories")
	if err := os.MkdirAll(sdir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", sdir, err)
	}
	return sdir, nil
}

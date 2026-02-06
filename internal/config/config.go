package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Database settings
	Database DatabaseConfig `yaml:"database"`

	// Invoice settings
	Invoice InvoiceConfig `yaml:"invoice"`

	// User info for invoices
	User UserConfig `yaml:"user"`
}

type DatabaseConfig struct {
	Path string `yaml:"path"` // Path to SQLite database
}

type InvoiceConfig struct {
	DefaultDueDays int     `yaml:"default_due_days"` // Days until invoice due
	DefaultTaxRate float64 `yaml:"default_tax_rate"` // Tax rate as decimal (0.0825 = 8.25%)
	OutputDir      string  `yaml:"output_dir"`       // Directory for generated PDFs
	NumberPrefix   string  `yaml:"number_prefix"`    // Invoice number prefix (e.g., "INV")
}

type UserConfig struct {
	Name    string `yaml:"name"`
	Email   string `yaml:"email"`
	Address string `yaml:"address"`
	Phone   string `yaml:"phone"`
}

// DefaultConfigPath returns ~/.config/timesink/config.yaml
func DefaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home dir unavailable
		return filepath.Join(".", ".config", "timesink", "config.yaml")
	}
	return filepath.Join(homeDir, ".config", "timesink", "config.yaml")
}

// DefaultConfig returns sensible defaults
func DefaultConfig() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	return &Config{
		Database: DatabaseConfig{
			Path: filepath.Join(homeDir, ".config", "timesink", "timesink.db"),
		},
		Invoice: InvoiceConfig{
			DefaultDueDays: 30,
			DefaultTaxRate: 0.0,
			OutputDir:      filepath.Join(homeDir, ".config", "timesink", "invoices"),
			NumberPrefix:   "INV",
		},
		User: UserConfig{
			Name:    "",
			Email:   "",
			Address: "",
			Phone:   "",
		},
	}
}

// Load loads config from the given path, or returns defaults if file doesn't exist
func Load(path string) (*Config, error) {
	// If file doesn't exist, return defaults
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return DefaultConfig(), nil
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadDefault loads from the default config path
func LoadDefault() (*Config, error) {
	return Load(DefaultConfigPath())
}

// Save writes the config to the given path
func (c *Config) Save(path string) error {
	// Create parent directories if they don't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Marshal to YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(path, data, 0644)
}

// EnsureDirectories creates all necessary directories (for database, invoices, etc.)
func (c *Config) EnsureDirectories() error {
	// Create database directory
	dbDir := filepath.Dir(c.Database.Path)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return err
	}

	// Create invoice output directory
	if err := os.MkdirAll(c.Invoice.OutputDir, 0755); err != nil {
		return err
	}

	return nil
}

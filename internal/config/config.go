package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
)

// Config represents the complete configuration
type Config struct {
	// Log configuration
	Log Log `koanf:"log"`
	// Validator is the validator configuration
	Validator Validator `koanf:"validator"`
	// Cluster is the DoubleZero cluster configuration
	Cluster Cluster `koanf:"cluster"`
	// DoubleZero is the DoubleZero configuration
	DoubleZero DoubleZero `koanf:"doublezero"`
	// Sync is the version sync configuration
	Sync Sync `koanf:"sync"`
	// File is the file that the config was loaded from
	File string `koanf:"-"`

	logger *log.Logger
}

// New creates a new Config
func New() (config *Config, err error) {
	config = &Config{
		logger: log.WithPrefix("config"),
	}
	return config, nil
}

// NewFromConfigFile creates a new Config from a config file path
func NewFromConfigFile(configFile string) (*Config, error) {
	// Create new config
	cfg, err := New()
	if err != nil {
		return nil, err
	}

	// Load from file
	if err := cfg.LoadFromFile(configFile); err != nil {
		return nil, err
	}

	// Initialize
	if err := cfg.Initialize(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadFromFile loads configuration from file into the struct
func (c *Config) LoadFromFile(filePath string) error {
	k := koanf.New(".")
	
	// Resolve config file path to absolute
	resolvedConfigPath, err := ResolvePath(filePath, "")
	if err != nil {
		return fmt.Errorf("failed to resolve config file path: %w", err)
	}
	c.File = resolvedConfigPath

	// Set defaults in koanf first
	c.setKoanfDefaults(k)

	// Load YAML config file (this will merge with defaults)
	if err := k.Load(file.Provider(c.File), yaml.Parser()); err != nil {
		return fmt.Errorf("error loading config file: %w", err)
	}

	// Unmarshal into this config struct
	if err := k.Unmarshal("", c); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	return nil
}

// Initialize processes and validates the loaded configuration
func (c *Config) Initialize() error {
	// Load validator identities if RPC URL is configured (identity files are required if RPC URL is set)
	if c.Validator.RPCURL != "" {
		if c.Validator.Identities.ActiveKeyPairFile == "" || c.Validator.Identities.PassiveKeyPairFile == "" {
			return fmt.Errorf("validator.rpc_url is configured but validator.identities.active and validator.identities.passive must be provided")
		}
		if err := c.Validator.Identities.Load(); err != nil {
			return fmt.Errorf("failed to load validator identities: %w", err)
		}
	}

	// Resolve paths to absolute paths
	if err := c.resolvePaths(); err != nil {
		return err
	}

	// validate configuration
	if err := c.validate(); err != nil {
		return err
	}

	return nil
}

// resolvePaths resolves all file paths in the config to absolute paths
func (c *Config) resolvePaths() error {
	// Get the directory containing the config file
	configDir := filepath.Dir(c.File)
	if c.File == "" {
		// If no config file, use current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		configDir = cwd
	}

	// Resolve validator identity paths if configured
	if c.Validator.RPCURL != "" {
		if c.Validator.Identities.ActiveKeyPairFile != "" {
			resolvedActive, err := ResolvePath(c.Validator.Identities.ActiveKeyPairFile, configDir)
			if err != nil {
				return fmt.Errorf("failed to resolve validator.identities.active path: %w", err)
			}
			c.Validator.Identities.ActiveKeyPairFile = resolvedActive
		}
		if c.Validator.Identities.PassiveKeyPairFile != "" {
			resolvedPassive, err := ResolvePath(c.Validator.Identities.PassiveKeyPairFile, configDir)
			if err != nil {
				return fmt.Errorf("failed to resolve validator.identities.passive path: %w", err)
			}
			c.Validator.Identities.PassiveKeyPairFile = resolvedPassive
		}
	}

	// Resolve DoubleZero.Bin if it's a file path
	if IsFilePath(c.DoubleZero.Bin) {
		originalBin := c.DoubleZero.Bin
		resolvedBin, err := ResolvePath(c.DoubleZero.Bin, configDir)
		if err != nil {
			return fmt.Errorf("failed to resolve doublezero.bin path: %w", err)
		}
		c.DoubleZero.Bin = resolvedBin
		c.logger.Debug("resolved doublezero.bin to absolute path", "original", originalBin, "resolved", resolvedBin)
	}

	return nil
}

// validate validates the configuration
func (c *Config) validate() error {
	err := c.Log.Validate()
	if err != nil {
		return err
	}

	err = c.Validator.Validate()
	if err != nil {
		return err
	}

	err = c.Cluster.Validate()
	if err != nil {
		return err
	}

	err = c.DoubleZero.Validate()
	if err != nil {
		return err
	}

	err = c.Sync.Validate()
	if err != nil {
		return err
	}

	return nil
}

// setKoanfDefaults sets default values in koanf configuration
func (c *Config) setKoanfDefaults(k *koanf.Koanf) {
	// Set log defaults
	k.Set("log.level", "info")
	k.Set("log.format", "text")
	// Note: validator.rpc_url defaults to empty string (not set) so validator check is optional
}

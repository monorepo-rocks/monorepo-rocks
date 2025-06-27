package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	IndexRoot  string            `yaml:"index_root"`
	RepoGlobs  []string          `yaml:"repo_globs"`
	Languages  []string          `yaml:"languages"`
	Embedding  EmbeddingConfig   `yaml:"embedding"`
	Fusion     FusionConfig      `yaml:"fusion"`
	Watcher    WatcherConfig     `yaml:"watcher"`
	Security   SecurityConfig    `yaml:"security"`
}

// EmbeddingConfig holds embedding-related settings
type EmbeddingConfig struct {
	Model     string `yaml:"model"`
	Device    string `yaml:"device"`
	ChunkSize int    `yaml:"chunk_size"`
	BatchSize int    `yaml:"batch_size"`
}

// FusionConfig holds fusion ranking settings
type FusionConfig struct {
	BM25Weight float64 `yaml:"bm25_weight"`
}

// WatcherConfig holds file watcher settings
type WatcherConfig struct {
	DebounceMs int `yaml:"debounce_ms"`
}

// SecurityConfig holds security settings
type SecurityConfig struct {
	EncryptIndex bool   `yaml:"encrypt_index"`
	KeyPath      string `yaml:"key_path"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	
	return &Config{
		IndexRoot: filepath.Join(homeDir, ".cache", "mcpce"),
		RepoGlobs: []string{filepath.Join(homeDir, "code", "**")},
		Languages: []string{"js", "ts", "py", "go"},
		Embedding: EmbeddingConfig{
			Model:     "microsoft/codebert-base",
			Device:    "cpu",
			ChunkSize: 300,
			BatchSize: 32,
		},
		Fusion: FusionConfig{
			BM25Weight: 0.45,
		},
		Watcher: WatcherConfig{
			DebounceMs: 250,
		},
		Security: SecurityConfig{
			EncryptIndex: false,
			KeyPath:      filepath.Join(homeDir, ".config", "mcpce", "keyfile"),
		},
	}
}

// Load reads configuration from a file
func Load(path string) (*Config, error) {
	// If no path specified, try default locations
	if path == "" {
		path = findConfigFile()
	}

	// Start with defaults
	cfg := DefaultConfig()

	// If config file exists, load it
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Expand paths
	cfg.expandPaths()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// findConfigFile looks for config file in standard locations
func findConfigFile() string {
	homeDir, _ := os.UserHomeDir()
	
	locations := []string{
		"config.yaml",
		".mcpce.yaml",
		filepath.Join(homeDir, ".config", "mcpce", "config.yaml"),
		filepath.Join(homeDir, ".mcpce", "config.yaml"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

// expandPaths expands ~ and environment variables in paths
func (c *Config) expandPaths() {
	c.IndexRoot = expandPath(c.IndexRoot)
	c.Security.KeyPath = expandPath(c.Security.KeyPath)
	
	for i, glob := range c.RepoGlobs {
		c.RepoGlobs[i] = expandPath(glob)
	}
}

// expandPath expands ~ and environment variables
func expandPath(path string) string {
	if path == "" {
		return path
	}

	// Expand ~
	if path[0] == '~' {
		homeDir, _ := os.UserHomeDir()
		path = filepath.Join(homeDir, path[1:])
	}

	// Expand environment variables
	return os.ExpandEnv(path)
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.IndexRoot == "" {
		return fmt.Errorf("index_root cannot be empty")
	}

	if len(c.Languages) == 0 {
		return fmt.Errorf("at least one language must be specified")
	}

	if c.Fusion.BM25Weight < 0 || c.Fusion.BM25Weight > 1 {
		return fmt.Errorf("bm25_weight must be between 0 and 1")
	}

	if c.Watcher.DebounceMs < 0 {
		return fmt.Errorf("debounce_ms must be non-negative")
	}

	if c.Embedding.ChunkSize <= 0 {
		return fmt.Errorf("chunk_size must be positive")
	}

	if c.Embedding.BatchSize <= 0 {
		return fmt.Errorf("batch_size must be positive")
	}

	return nil
}

// Save writes the configuration to a file
func (c *Config) Save(path string) error {
	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	IndexRoot string          `yaml:"index_root"`
	RepoGlobs []string        `yaml:"repo_globs"`
	Languages []string        `yaml:"languages"`
	Embedding EmbeddingConfig `yaml:"embedding"`
	Fusion    FusionConfig    `yaml:"fusion"`
	Watcher   WatcherConfig   `yaml:"watcher"`
	Security  SecurityConfig  `yaml:"security"`
}

// EmbeddingConfig holds embedding-related settings
type EmbeddingConfig struct {
	Model     string `yaml:"model"`
	Device    string `yaml:"device"`
	ChunkSize int    `yaml:"chunk_size"`
	BatchSize int    `yaml:"batch_size"`
}

// FusionStrategy represents different fusion algorithms
type FusionStrategy string

const (
	FusionRRF            FusionStrategy = "rrf"             // Reciprocal Rank Fusion
	FusionWeightedLinear FusionStrategy = "weighted_linear" // Weighted linear combination
	FusionLearnedWeights FusionStrategy = "learned_weights" // ML-based learned weights
)

// ScoreNormalization represents different score normalization methods
type ScoreNormalization string

const (
	NormNone      ScoreNormalization = "none"       // No normalization
	NormMinMax    ScoreNormalization = "min_max"    // Min-max normalization
	NormZScore    ScoreNormalization = "z_score"    // Z-score normalization
	NormRankBased ScoreNormalization = "rank_based" // Rank-based normalization
)

// QueryType represents different query characteristics for adaptive weighting
type QueryType string

const (
	QueryNatural QueryType = "natural" // Natural language queries
	QueryCode    QueryType = "code"    // Code-specific queries
	QuerySymbol  QueryType = "symbol"  // Symbol/identifier queries
	QueryFile    QueryType = "file"    // File-specific queries
	QueryImport  QueryType = "import"  // Import/usage queries
	QueryConfig  QueryType = "config"  // Configuration file queries
)

// FusionConfig holds fusion ranking settings
type FusionConfig struct {
	// Basic settings (backward compatible)
	BM25Weight float64 `yaml:"bm25_weight"`

	// Enhanced fusion settings
	Strategy      FusionStrategy     `yaml:"strategy"`
	Normalization ScoreNormalization `yaml:"normalization"`
	RRFConstant   float64            `yaml:"rrf_constant"`

	// Adaptive weighting settings
	AdaptiveWeighting bool                  `yaml:"adaptive_weighting"`
	QueryTypeWeights  map[QueryType]float64 `yaml:"query_type_weights"`

	// Boost factors
	ExactMatchBoost  float64 `yaml:"exact_match_boost"`
	SymbolMatchBoost float64 `yaml:"symbol_match_boost"`
	FileTypeBoost    float64 `yaml:"file_type_boost"`
	RecencyBoost     float64 `yaml:"recency_boost"`

	// Score thresholds
	MinLexicalScore  float64 `yaml:"min_lexical_score"`
	MinSemanticScore float64 `yaml:"min_semantic_score"`

	// Analytics and debugging
	EnableAnalytics bool `yaml:"enable_analytics"`
	DebugScoring    bool `yaml:"debug_scoring"`
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
			BM25Weight:        0.45,
			Strategy:          FusionRRF,
			Normalization:     NormNone,
			RRFConstant:       60.0,
			AdaptiveWeighting: true,
			QueryTypeWeights: map[QueryType]float64{
				QueryNatural: 0.35, // Favor semantic for natural language
				QueryCode:    0.65, // Favor lexical for code queries
				QuerySymbol:  0.75, // Heavily favor lexical for symbols
				QueryFile:    0.55, // Slightly favor lexical for file queries
				QueryImport:  0.70, // Favor lexical for import queries
				QueryConfig:  0.80, // Heavily favor lexical for config queries
			},
			ExactMatchBoost:  1.5,
			SymbolMatchBoost: 1.3,
			FileTypeBoost:    1.2,
			RecencyBoost:     1.1,
			MinLexicalScore:  0.001,
			MinSemanticScore: 0.05,
			EnableAnalytics:  true,
			DebugScoring:     false,
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

	// Validate fusion strategy
	validStrategies := map[FusionStrategy]bool{
		FusionRRF:            true,
		FusionWeightedLinear: true,
		FusionLearnedWeights: true,
	}
	if !validStrategies[c.Fusion.Strategy] {
		return fmt.Errorf("invalid fusion strategy: %s", c.Fusion.Strategy)
	}

	// Validate normalization method
	validNormalizations := map[ScoreNormalization]bool{
		NormNone:      true,
		NormMinMax:    true,
		NormZScore:    true,
		NormRankBased: true,
	}
	if !validNormalizations[c.Fusion.Normalization] {
		return fmt.Errorf("invalid normalization method: %s", c.Fusion.Normalization)
	}

	// Validate RRF constant
	if c.Fusion.RRFConstant <= 0 {
		return fmt.Errorf("rrf_constant must be positive")
	}

	// Validate query type weights
	for queryType, weight := range c.Fusion.QueryTypeWeights {
		if weight < 0 || weight > 1 {
			return fmt.Errorf("query type weight for %s must be between 0 and 1", queryType)
		}
	}

	// Validate boost factors
	if c.Fusion.ExactMatchBoost < 1.0 {
		return fmt.Errorf("exact_match_boost must be >= 1.0")
	}
	if c.Fusion.SymbolMatchBoost < 1.0 {
		return fmt.Errorf("symbol_match_boost must be >= 1.0")
	}
	if c.Fusion.FileTypeBoost < 1.0 {
		return fmt.Errorf("file_type_boost must be >= 1.0")
	}
	if c.Fusion.RecencyBoost < 1.0 {
		return fmt.Errorf("recency_boost must be >= 1.0")
	}

	// Validate score thresholds
	if c.Fusion.MinLexicalScore < 0 {
		return fmt.Errorf("min_lexical_score must be non-negative")
	}
	if c.Fusion.MinSemanticScore < 0 {
		return fmt.Errorf("min_semantic_score must be non-negative")
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

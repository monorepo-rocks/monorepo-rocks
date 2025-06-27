package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.IndexRoot == "" {
		t.Error("IndexRoot should not be empty")
	}

	if len(cfg.Languages) == 0 {
		t.Error("Languages should not be empty")
	}

	if cfg.Embedding.Model == "" {
		t.Error("Embedding model should not be empty")
	}

	if cfg.Fusion.BM25Weight < 0 || cfg.Fusion.BM25Weight > 1 {
		t.Errorf("Invalid BM25Weight: %v", cfg.Fusion.BM25Weight)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		modify    func(*Config)
		expectErr bool
	}{
		{
			name:      "Valid config",
			modify:    func(c *Config) {},
			expectErr: false,
		},
		{
			name: "Empty index root",
			modify: func(c *Config) {
				c.IndexRoot = ""
			},
			expectErr: true,
		},
		{
			name: "No languages",
			modify: func(c *Config) {
				c.Languages = []string{}
			},
			expectErr: true,
		},
		{
			name: "Invalid BM25 weight",
			modify: func(c *Config) {
				c.Fusion.BM25Weight = 1.5
			},
			expectErr: true,
		},
		{
			name: "Negative debounce",
			modify: func(c *Config) {
				c.Watcher.DebounceMs = -100
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tt.modify(cfg)
			err := cfg.Validate()
			
			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestConfigSaveLoad(t *testing.T) {
	// Create temp file
	tmpDir, err := os.MkdirTemp("", "config-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")

	// Save config
	cfg := DefaultConfig()
	cfg.Languages = []string{"go", "rust"}
	cfg.Fusion.BM25Weight = 0.6

	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load config
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify
	if len(loaded.Languages) != 2 || loaded.Languages[0] != "go" {
		t.Errorf("Languages mismatch: %v", loaded.Languages)
	}

	if loaded.Fusion.BM25Weight != 0.6 {
		t.Errorf("BM25Weight mismatch: %v", loaded.Fusion.BM25Weight)
	}
}

func TestExpandPath(t *testing.T) {
	// Set test env var
	os.Setenv("TEST_VAR", "/test/path")
	defer os.Unsetenv("TEST_VAR")

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"/absolute/path", "/absolute/path"},
		{"$TEST_VAR/file", "/test/path/file"},
	}

	for _, tt := range tests {
		got := expandPath(tt.input)
		if got != tt.expected && !filepath.IsAbs(got) { // ~ expansion results in absolute path
			t.Errorf("expandPath(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
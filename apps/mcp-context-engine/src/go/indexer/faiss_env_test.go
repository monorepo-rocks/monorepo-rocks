package indexer

import (
	"os"
	"testing"
)

func TestFAISSIndexer_EnvironmentVariable(t *testing.T) {
	// Test factory function with environment variable
	originalValue := os.Getenv("FAISS_USE_REAL")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("FAISS_USE_REAL")
		} else {
			os.Setenv("FAISS_USE_REAL", originalValue)
		}
	}()

	// Test with FAISS_USE_REAL=false (default)
	os.Setenv("FAISS_USE_REAL", "false")
	
	indexer := NewFAISSIndexer("/tmp/test-env-faiss-false", 768)
	defer indexer.Close()

	if indexer == nil {
		t.Error("Expected non-nil indexer")
	}

	stats := indexer.VectorStats()
	if stats.Dimension != 768 {
		t.Errorf("Expected dimension 768, got %d", stats.Dimension)
	}

	// Test with FAISS_USE_REAL=true
	os.Setenv("FAISS_USE_REAL", "true")
	
	indexer2 := NewFAISSIndexer("/tmp/test-env-faiss-true", 768)
	defer indexer2.Close()

	if indexer2 == nil {
		t.Error("Expected non-nil indexer")
	}

	stats2 := indexer2.VectorStats()
	if stats2.Dimension != 768 {
		t.Errorf("Expected dimension 768, got %d", stats2.Dimension)
	}

	// Note: When CGO is disabled, the real FAISS indexer will fall back to stub
	// This is expected behavior and the factory function should handle it gracefully
}

func TestFAISSIndexer_DefaultBehavior(t *testing.T) {
	// Test default behavior (without FAISS_USE_REAL set)
	originalValue := os.Getenv("FAISS_USE_REAL")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("FAISS_USE_REAL")
		} else {
			os.Setenv("FAISS_USE_REAL", originalValue)
		}
	}()

	os.Unsetenv("FAISS_USE_REAL")
	
	indexer := NewFAISSIndexer("/tmp/test-default-faiss", 768)
	defer indexer.Close()

	if indexer == nil {
		t.Error("Expected non-nil indexer")
	}

	stats := indexer.VectorStats()
	if stats.Dimension != 768 {
		t.Errorf("Expected dimension 768, got %d", stats.Dimension)
	}
	if stats.TotalVectors != 0 {
		t.Errorf("Expected 0 vectors initially, got %d", stats.TotalVectors)
	}
}
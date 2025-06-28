//go:build cgo
// +build cgo

package indexer

import (
	"context"
	"os"
	"testing"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

func TestNewRealFAISSIndexer(t *testing.T) {
	// Skip if FAISS library is not available
	if !isFAISSAvailable() {
		t.Skip("FAISS library not available, skipping real FAISS tests")
	}

	indexer, err := NewRealFAISSIndexer("/tmp/test-real-faiss", 768)
	if err != nil {
		t.Fatalf("Failed to create real FAISS indexer: %v", err)
	}
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

func TestRealFAISSIndexer_Integration(t *testing.T) {
	// Skip if FAISS library is not available
	if !isFAISSAvailable() {
		t.Skip("FAISS library not available, skipping real FAISS tests")
	}

	indexer, err := NewRealFAISSIndexer("/tmp/test-real-faiss-integration", 768)
	if err != nil {
		t.Fatalf("Failed to create real FAISS indexer: %v", err)
	}
	defer indexer.Close()

	ctx := context.Background()

	// Test adding vectors
	embeddings := []types.Embedding{
		{ChunkID: "chunk1", Vector: GenerateRandomVector(768, 123)},
		{ChunkID: "chunk2", Vector: GenerateRandomVector(768, 456)},
		{ChunkID: "chunk3", Vector: GenerateRandomVector(768, 789)},
	}

	err = indexer.AddVectors(ctx, embeddings)
	if err != nil {
		t.Fatalf("Failed to add vectors: %v", err)
	}

	stats := indexer.VectorStats()
	if stats.TotalVectors != 3 {
		t.Errorf("Expected 3 vectors, got %d", stats.TotalVectors)
	}

	// Test search
	queryVector := embeddings[0].Vector
	options := VectorSearchOptions{MinScore: 0.0}
	
	results, err := indexer.Search(ctx, queryVector, 5, options)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Expected search results, got none")
	}

	// First result should be the exact match with high similarity
	if results[0].ChunkID != "chunk1" {
		t.Errorf("Expected first result to be chunk1, got %s", results[0].ChunkID)
	}
	if results[0].Score < 0.99 { // Should be very close to 1.0 for exact match
		t.Errorf("Expected high similarity score, got %f", results[0].Score)
	}

	// Test deletion
	err = indexer.Delete(ctx, []string{"chunk2"})
	if err != nil {
		t.Fatalf("Failed to delete vector: %v", err)
	}

	stats = indexer.VectorStats()
	if stats.TotalVectors != 2 {
		t.Errorf("Expected 2 vectors after deletion, got %d", stats.TotalVectors)
	}
}

func TestRealFAISSIndexer_SaveLoad(t *testing.T) {
	// Skip if FAISS library is not available
	if !isFAISSAvailable() {
		t.Skip("FAISS library not available, skipping real FAISS tests")
	}

	tempDir := t.TempDir()
	indexPath := tempDir + "/test-index.faiss"

	// Create indexer and add vectors
	indexer, err := NewRealFAISSIndexer(indexPath, 768)
	if err != nil {
		t.Fatalf("Failed to create real FAISS indexer: %v", err)
	}
	
	embeddings := []types.Embedding{
		{ChunkID: "chunk1", Vector: GenerateRandomVector(768, 123)},
		{ChunkID: "chunk2", Vector: GenerateRandomVector(768, 456)},
	}

	ctx := context.Background()
	err = indexer.AddVectors(ctx, embeddings)
	if err != nil {
		t.Fatalf("Failed to add vectors: %v", err)
	}

	// Save index
	err = indexer.Save(ctx, indexPath)
	if err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("Index file was not created")
	}

	indexer.Close()

	// Create new indexer and load
	indexer2, err := NewRealFAISSIndexer(indexPath, 768)
	if err != nil {
		t.Fatalf("Failed to create second real FAISS indexer: %v", err)
	}
	defer indexer2.Close()
	
	err = indexer2.Load(ctx, indexPath)
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	stats := indexer2.VectorStats()
	if stats.TotalVectors != 2 {
		t.Errorf("Expected 2 vectors after loading, got %d", stats.TotalVectors)
	}
}

func TestRealFAISSIndexer_EnvironmentVariable(t *testing.T) {
	// Test factory function with environment variable
	originalValue := os.Getenv("FAISS_USE_REAL")
	defer os.Setenv("FAISS_USE_REAL", originalValue)

	// Test with FAISS_USE_REAL=true
	os.Setenv("FAISS_USE_REAL", "true")
	
	indexer := NewFAISSIndexer("/tmp/test-env-faiss", 768)
	defer indexer.Close()

	if indexer == nil {
		t.Error("Expected non-nil indexer")
	}

	// Note: This test might use stub if FAISS library is not available
	// The factory function should handle this gracefully
}

// isFAISSAvailable checks if the FAISS library is available for testing
func isFAISSAvailable() bool {
	// Try to create a simple FAISS indexer to check availability
	_, err := NewRealFAISSIndexer("/tmp/faiss-test", 10)
	return err == nil
}
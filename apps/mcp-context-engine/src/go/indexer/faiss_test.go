package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"../types"
)

func TestFAISSIndexer_AddVectors(t *testing.T) {
	indexer := NewFAISSIndexer("/tmp/test-faiss", 768)
	defer indexer.Close()

	embeddings := []types.Embedding{
		{ChunkID: "chunk1", Vector: GenerateRandomVector(768, 123)},
		{ChunkID: "chunk2", Vector: GenerateRandomVector(768, 456)},
		{ChunkID: "chunk3", Vector: GenerateRandomVector(768, 789)},
	}

	ctx := context.Background()
	err := indexer.AddVectors(ctx, embeddings)
	if err != nil {
		t.Fatalf("Failed to add vectors: %v", err)
	}

	stats := indexer.VectorStats()
	if stats.TotalVectors != 3 {
		t.Errorf("Expected 3 vectors, got %d", stats.TotalVectors)
	}
	if stats.Dimension != 768 {
		t.Errorf("Expected dimension 768, got %d", stats.Dimension)
	}
}

func TestFAISSIndexer_AddVectorsDimensionMismatch(t *testing.T) {
	indexer := NewFAISSIndexer("/tmp/test-faiss", 768)
	defer indexer.Close()

	// Try to add vector with wrong dimension
	embeddings := []types.Embedding{
		{ChunkID: "chunk1", Vector: GenerateRandomVector(512, 123)}, // Wrong dimension
	}

	ctx := context.Background()
	err := indexer.AddVectors(ctx, embeddings)
	if err == nil {
		t.Error("Expected error for dimension mismatch, got nil")
	}
}

func TestFAISSIndexer_Search(t *testing.T) {
	indexer := NewFAISSIndexer("/tmp/test-faiss", 768)
	defer indexer.Close()

	// Add test vectors
	embeddings := []types.Embedding{
		{ChunkID: "chunk1", Vector: GenerateRandomVector(768, 123)},
		{ChunkID: "chunk2", Vector: GenerateRandomVector(768, 456)},
		{ChunkID: "chunk3", Vector: GenerateRandomVector(768, 789)},
	}

	ctx := context.Background()
	err := indexer.AddVectors(ctx, embeddings)
	if err != nil {
		t.Fatalf("Failed to add vectors: %v", err)
	}

	// Search with one of the added vectors (should get high similarity)
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

	// Verify results are sorted by score
	for i := 1; i < len(results); i++ {
		if results[i-1].Score < results[i].Score {
			t.Error("Results should be sorted by score in descending order")
		}
	}
}

func TestFAISSIndexer_SearchWithMinScore(t *testing.T) {
	indexer := NewFAISSIndexer("/tmp/test-faiss", 768)
	defer indexer.Close()

	embeddings := []types.Embedding{
		{ChunkID: "chunk1", Vector: GenerateRandomVector(768, 123)},
		{ChunkID: "chunk2", Vector: GenerateRandomVector(768, 456)},
	}

	ctx := context.Background()
	err := indexer.AddVectors(ctx, embeddings)
	if err != nil {
		t.Fatalf("Failed to add vectors: %v", err)
	}

	// Search with high minimum score threshold
	queryVector := GenerateRandomVector(768, 999) // Different from stored vectors
	options := VectorSearchOptions{MinScore: 0.9}  // Very high threshold
	
	results, err := indexer.Search(ctx, queryVector, 10, options)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	// Should get fewer or no results due to high threshold
	for _, result := range results {
		if result.Score < options.MinScore {
			t.Errorf("Result score %f is below minimum threshold %f", 
				result.Score, options.MinScore)
		}
	}
}

func TestFAISSIndexer_SearchTopK(t *testing.T) {
	indexer := NewFAISSIndexer("/tmp/test-faiss", 768)
	defer indexer.Close()

	// Add many vectors
	var embeddings []types.Embedding
	for i := 0; i < 10; i++ {
		embeddings = append(embeddings, types.Embedding{
			ChunkID: fmt.Sprintf("chunk%d", i),
			Vector:  GenerateRandomVector(768, int64(i*100)),
		})
	}

	ctx := context.Background()
	err := indexer.AddVectors(ctx, embeddings)
	if err != nil {
		t.Fatalf("Failed to add vectors: %v", err)
	}

	// Search with k=3
	queryVector := GenerateRandomVector(768, 999)
	options := VectorSearchOptions{MinScore: 0.0}
	
	results, err := indexer.Search(ctx, queryVector, 3, options)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if len(results) > 3 {
		t.Errorf("Expected at most 3 results, got %d", len(results))
	}
}

func TestFAISSIndexer_Delete(t *testing.T) {
	indexer := NewFAISSIndexer("/tmp/test-faiss", 768)
	defer indexer.Close()

	embeddings := []types.Embedding{
		{ChunkID: "chunk1", Vector: GenerateRandomVector(768, 123)},
		{ChunkID: "chunk2", Vector: GenerateRandomVector(768, 456)},
		{ChunkID: "chunk3", Vector: GenerateRandomVector(768, 789)},
	}

	ctx := context.Background()
	err := indexer.AddVectors(ctx, embeddings)
	if err != nil {
		t.Fatalf("Failed to add vectors: %v", err)
	}

	// Delete one vector
	err = indexer.Delete(ctx, []string{"chunk2"})
	if err != nil {
		t.Fatalf("Failed to delete vector: %v", err)
	}

	stats := indexer.VectorStats()
	if stats.TotalVectors != 2 {
		t.Errorf("Expected 2 vectors after deletion, got %d", stats.TotalVectors)
	}

	// Verify deleted vector is not found in search
	queryVector := embeddings[1].Vector // chunk2's vector
	options := VectorSearchOptions{MinScore: 0.0}
	
	results, err := indexer.Search(ctx, queryVector, 10, options)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	for _, result := range results {
		if result.ChunkID == "chunk2" {
			t.Error("Deleted vector should not appear in search results")
		}
	}
}

func TestFAISSIndexer_SaveLoad(t *testing.T) {
	tempDir := t.TempDir()
	indexPath := filepath.Join(tempDir, "test-index.faiss")

	// Create indexer and add vectors
	indexer := NewFAISSIndexer(indexPath, 768)
	
	embeddings := []types.Embedding{
		{ChunkID: "chunk1", Vector: GenerateRandomVector(768, 123)},
		{ChunkID: "chunk2", Vector: GenerateRandomVector(768, 456)},
	}

	ctx := context.Background()
	err := indexer.AddVectors(ctx, embeddings)
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

	// Create new indexer and load
	indexer2 := NewFAISSIndexer(indexPath, 768)
	defer indexer2.Close()
	
	err = indexer2.Load(ctx, indexPath)
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	indexer.Close()
}

func TestFAISSIndexer_LoadNonexistentFile(t *testing.T) {
	indexer := NewFAISSIndexer("/tmp/test-faiss", 768)
	defer indexer.Close()

	ctx := context.Background()
	err := indexer.Load(ctx, "/nonexistent/path/index.faiss")
	if err == nil {
		t.Error("Expected error when loading nonexistent file, got nil")
	}
}

func TestFAISSIndexer_SearchEmptyIndex(t *testing.T) {
	indexer := NewFAISSIndexer("/tmp/test-faiss", 768)
	defer indexer.Close()

	queryVector := GenerateRandomVector(768, 123)
	options := VectorSearchOptions{MinScore: 0.0}
	
	ctx := context.Background()
	results, err := indexer.Search(ctx, queryVector, 10, options)
	if err != nil {
		t.Fatalf("Search on empty index failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected no results from empty index, got %d", len(results))
	}
}

func TestFAISSIndexer_SearchWrongDimension(t *testing.T) {
	indexer := NewFAISSIndexer("/tmp/test-faiss", 768)
	defer indexer.Close()

	wrongDimVector := GenerateRandomVector(512, 123) // Wrong dimension
	options := VectorSearchOptions{MinScore: 0.0}
	
	ctx := context.Background()
	_, err := indexer.Search(ctx, wrongDimVector, 10, options)
	if err == nil {
		t.Error("Expected error for wrong dimension query vector, got nil")
	}
}

func TestFAISSIndexer_VectorStats(t *testing.T) {
	indexer := NewFAISSIndexer("/tmp/test-faiss", 768)
	defer indexer.Close()

	// Check initial stats
	stats := indexer.VectorStats()
	if stats.TotalVectors != 0 {
		t.Errorf("Expected 0 vectors initially, got %d", stats.TotalVectors)
	}
	if stats.Dimension != 768 {
		t.Errorf("Expected dimension 768, got %d", stats.Dimension)
	}

	// Add vectors and check updated stats
	embeddings := []types.Embedding{
		{ChunkID: "chunk1", Vector: GenerateRandomVector(768, 123)},
		{ChunkID: "chunk2", Vector: GenerateRandomVector(768, 456)},
	}

	ctx := context.Background()
	err := indexer.AddVectors(ctx, embeddings)
	if err != nil {
		t.Fatalf("Failed to add vectors: %v", err)
	}

	initialTime := stats.LastUpdated
	time.Sleep(10 * time.Millisecond) // Ensure time difference

	stats = indexer.VectorStats()
	if stats.TotalVectors != 2 {
		t.Errorf("Expected 2 vectors after adding, got %d", stats.TotalVectors)
	}
	if stats.IndexSize <= 0 {
		t.Errorf("Expected positive index size, got %d", stats.IndexSize)
	}
	if !stats.LastUpdated.After(initialTime) {
		t.Error("Expected LastUpdated to be updated")
	}
}

func TestGenerateRandomVector(t *testing.T) {
	dimension := 100
	seed := int64(42)

	vector1 := GenerateRandomVector(dimension, seed)
	vector2 := GenerateRandomVector(dimension, seed)

	// Same seed should produce same vector
	if len(vector1) != dimension {
		t.Errorf("Expected vector length %d, got %d", dimension, len(vector1))
	}

	for i := range vector1 {
		if vector1[i] != vector2[i] {
			t.Error("Same seed should produce identical vectors")
			break
		}
	}

	// Different seed should produce different vector
	vector3 := GenerateRandomVector(dimension, seed+1)
	identical := true
	for i := range vector1 {
		if vector1[i] != vector3[i] {
			identical = false
			break
		}
	}
	if identical {
		t.Error("Different seeds should produce different vectors")
	}
}

func TestVectorSimilarity(t *testing.T) {
	// Test identical vectors
	vec1 := []float32{1.0, 2.0, 3.0}
	vec2 := []float32{1.0, 2.0, 3.0}
	sim := VectorSimilarity(vec1, vec2)
	if sim < 0.99 {
		t.Errorf("Expected high similarity for identical vectors, got %f", sim)
	}

	// Test orthogonal vectors
	vec3 := []float32{1.0, 0.0, 0.0}
	vec4 := []float32{0.0, 1.0, 0.0}
	sim = VectorSimilarity(vec3, vec4)
	if sim != 0.0 {
		t.Errorf("Expected zero similarity for orthogonal vectors, got %f", sim)
	}

	// Test different dimension vectors
	vec5 := []float32{1.0, 2.0}
	vec6 := []float32{1.0, 2.0, 3.0}
	sim = VectorSimilarity(vec5, vec6)
	if sim != 0.0 {
		t.Errorf("Expected zero similarity for different dimensions, got %f", sim)
	}

	// Test zero vectors
	vec7 := []float32{0.0, 0.0, 0.0}
	vec8 := []float32{1.0, 2.0, 3.0}
	sim = VectorSimilarity(vec7, vec8)
	if sim != 0.0 {
		t.Errorf("Expected zero similarity for zero vector, got %f", sim)
	}
}

func TestCreateEmbeddingBatch(t *testing.T) {
	chunkIDs := []string{"chunk1", "chunk2", "chunk3"}
	dimension := 768

	embeddings := CreateEmbeddingBatch(chunkIDs, dimension)

	if len(embeddings) != len(chunkIDs) {
		t.Errorf("Expected %d embeddings, got %d", len(chunkIDs), len(embeddings))
	}

	for i, embedding := range embeddings {
		if embedding.ChunkID != chunkIDs[i] {
			t.Errorf("Expected chunk ID %s, got %s", chunkIDs[i], embedding.ChunkID)
		}
		if len(embedding.Vector) != dimension {
			t.Errorf("Expected vector dimension %d, got %d", dimension, len(embedding.Vector))
		}
	}

	// Test deterministic generation - same input should produce same output
	embeddings2 := CreateEmbeddingBatch(chunkIDs, dimension)
	for i := range embeddings {
		for j := range embeddings[i].Vector {
			if embeddings[i].Vector[j] != embeddings2[i].Vector[j] {
				t.Error("CreateEmbeddingBatch should be deterministic")
				break
			}
		}
	}
}
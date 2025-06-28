//go:build cgo
// +build cgo

package backends

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDimension = 768 // CodeBERT dimension
	defaultK      = 10
)

// generateTestEmbedding creates a test embedding vector with given characteristics
func generateTestEmbedding(chunkID string, seed int64, dimension int) types.Embedding {
	rand.Seed(seed)
	vector := make([]float32, dimension)
	
	// Create slightly different patterns for different chunk types
	var pattern float32
	switch {
	case len(chunkID) > 0 && chunkID[0] == 'f': // function chunks
		pattern = 0.8
	case len(chunkID) > 0 && chunkID[0] == 'c': // class chunks  
		pattern = 0.6
	case len(chunkID) > 0 && chunkID[0] == 'v': // variable chunks
		pattern = 0.4
	default:
		pattern = 0.5
	}
	
	for i := 0; i < dimension; i++ {
		// Create patterns that will result in measurable similarities
		base := pattern + rand.Float32()*0.2 - 0.1
		vector[i] = base + float32(i%10)*0.01
	}
	
	return types.Embedding{
		ChunkID: chunkID,
		Vector:  vector,
	}
}

// generateSimilarEmbedding creates an embedding similar to the base embedding
func generateSimilarEmbedding(baseEmbedding types.Embedding, chunkID string, similarity float32) types.Embedding {
	dimension := len(baseEmbedding.Vector)
	vector := make([]float32, dimension)
	
	// Create a vector that has the desired similarity with the base
	for i := 0; i < dimension; i++ {
		// Mix the original vector with some noise
		noise := rand.Float32()*0.2 - 0.1
		vector[i] = baseEmbedding.Vector[i]*similarity + noise*(1-similarity)
	}
	
	return types.Embedding{
		ChunkID: chunkID,
		Vector:  vector,
	}
}

// setupFAISSTestEnvironment creates a test environment for FAISS integration tests
func setupFAISSTestEnvironment(t *testing.T) (string, *indexer.RealFAISSIndexer) {
	tmpDir, err := os.MkdirTemp("", "faiss-real-integration-*")
	require.NoError(t, err, "Failed to create temp directory")

	indexPath := filepath.Join(tmpDir, "test.faiss")
	
	faissIdx, err := indexer.NewRealFAISSIndexer(indexPath, testDimension)
	require.NoError(t, err, "Failed to create FAISS indexer")

	return tmpDir, faissIdx
}

func TestRealFAISSIndexerBasicOperations(t *testing.T) {
	tmpDir, faissIdx := setupFAISSTestEnvironment(t)
	defer func() {
		faissIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	t.Run("Initial stats", func(t *testing.T) {
		stats := faissIdx.VectorStats()
		assert.Equal(t, 0, stats.TotalVectors, "Should start with 0 vectors")
		assert.Equal(t, testDimension, stats.Dimension, "Should have correct dimension")
	})

	t.Run("Add vectors", func(t *testing.T) {
		embeddings := []types.Embedding{
			generateTestEmbedding("func_authenticate", 12345, testDimension),
			generateTestEmbedding("func_validate", 12346, testDimension),
			generateTestEmbedding("class_User", 12347, testDimension),
			generateTestEmbedding("var_username", 12348, testDimension),
		}

		err := faissIdx.AddVectors(ctx, embeddings)
		assert.NoError(t, err, "Adding vectors should succeed")

		stats := faissIdx.VectorStats()
		assert.Equal(t, 4, stats.TotalVectors, "Should have 4 vectors after adding")
		assert.True(t, stats.LastUpdated.After(time.Now().Add(-time.Minute)), "LastUpdated should be recent")
	})

	t.Run("Search vectors", func(t *testing.T) {
		// Create a query vector similar to one of the indexed vectors
		queryEmbedding := generateTestEmbedding("func_authenticate", 12345, testDimension)
		
		options := indexer.VectorSearchOptions{
			MinScore: 0.0,
		}

		results, err := faissIdx.Search(ctx, queryEmbedding.Vector, defaultK, options)
		assert.NoError(t, err, "Vector search should succeed")
		assert.Greater(t, len(results), 0, "Should find at least one result")

		// The first result should be the most similar (potentially identical)
		if len(results) > 0 {
			topResult := results[0]
			assert.Equal(t, "func_authenticate", topResult.ChunkID, "Top result should be the exact match")
			assert.GreaterOrEqual(t, topResult.Score, 0.9, "Top result should have high similarity score")
		}

		// Verify results are sorted by score
		for i := 1; i < len(results); i++ {
			assert.GreaterOrEqual(t, results[i-1].Score, results[i].Score, 
				"Results should be sorted by score in descending order")
		}
	})

	t.Run("Search with minimum score threshold", func(t *testing.T) {
		queryEmbedding := generateTestEmbedding("func_authenticate", 12345, testDimension)
		
		highThreshold := indexer.VectorSearchOptions{
			MinScore: 0.8, // High threshold
		}

		results, err := faissIdx.Search(ctx, queryEmbedding.Vector, defaultK, highThreshold)
		assert.NoError(t, err, "Vector search with high threshold should succeed")

		// Verify all results meet the threshold
		for _, result := range results {
			assert.GreaterOrEqual(t, result.Score, 0.8, 
				"All results should meet the minimum score threshold")
		}
	})
}

func TestRealFAISSIndexerDimensionValidation(t *testing.T) {
	tmpDir, faissIdx := setupFAISSTestEnvironment(t)
	defer func() {
		faissIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	t.Run("Wrong dimension in AddVectors", func(t *testing.T) {
		wrongDimEmbedding := types.Embedding{
			ChunkID: "test",
			Vector:  make([]float32, 512), // Wrong dimension
		}

		err := faissIdx.AddVectors(ctx, []types.Embedding{wrongDimEmbedding})
		assert.Error(t, err, "Should return error for wrong dimension")
		assert.Contains(t, err.Error(), "dimension mismatch", "Error should mention dimension mismatch")
	})

	t.Run("Wrong dimension in Search", func(t *testing.T) {
		wrongQueryVector := make([]float32, 512) // Wrong dimension

		options := indexer.VectorSearchOptions{}
		_, err := faissIdx.Search(ctx, wrongQueryVector, defaultK, options)
		assert.Error(t, err, "Should return error for wrong query vector dimension")
		assert.Contains(t, err.Error(), "dimension mismatch", "Error should mention dimension mismatch")
	})

	t.Run("Empty embeddings", func(t *testing.T) {
		err := faissIdx.AddVectors(ctx, []types.Embedding{})
		assert.NoError(t, err, "Adding empty embeddings should not error")

		stats := faissIdx.VectorStats()
		assert.Equal(t, 0, stats.TotalVectors, "Should still have 0 vectors")
	})
}

func TestRealFAISSIndexerSimilarityCalculation(t *testing.T) {
	tmpDir, faissIdx := setupFAISSTestEnvironment(t)
	defer func() {
		faissIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// Create a base embedding and similar embeddings with known relationships
	baseEmbedding := generateTestEmbedding("base_func", 42, testDimension)
	
	embeddings := []types.Embedding{
		baseEmbedding,
		generateSimilarEmbedding(baseEmbedding, "similar_func_90", 0.9),
		generateSimilarEmbedding(baseEmbedding, "similar_func_70", 0.7),
		generateSimilarEmbedding(baseEmbedding, "similar_func_50", 0.5),
		generateTestEmbedding("different_func", 999, testDimension), // Very different
	}

	err := faissIdx.AddVectors(ctx, embeddings)
	require.NoError(t, err, "Adding test vectors should succeed")

	t.Run("Similarity ranking", func(t *testing.T) {
		options := indexer.VectorSearchOptions{MinScore: 0.0}
		results, err := faissIdx.Search(ctx, baseEmbedding.Vector, 5, options)
		require.NoError(t, err, "Search should succeed")
		require.GreaterOrEqual(t, len(results), 4, "Should find at least 4 results")

		// Verify the ranking order
		expectedOrder := []string{"base_func", "similar_func_90", "similar_func_70", "similar_func_50"}
		
		for i, expectedChunkID := range expectedOrder {
			if i < len(results) {
				assert.Equal(t, expectedChunkID, results[i].ChunkID, 
					"Result %d should be %s", i, expectedChunkID)
			}
		}

		// Verify scores are decreasing
		for i := 1; i < len(results); i++ {
			assert.GreaterOrEqual(t, results[i-1].Score, results[i].Score,
				"Similarity scores should be in descending order")
		}

		// The exact match should have very high similarity
		if len(results) > 0 {
			assert.GreaterOrEqual(t, results[0].Score, 0.95, 
				"Exact match should have very high similarity")
		}
	})

	t.Run("Cosine similarity properties", func(t *testing.T) {
		// Test with a unit vector (should normalize properly)
		unitVector := make([]float32, testDimension)
		unitVector[0] = 1.0 // Simple unit vector

		unitEmbedding := types.Embedding{
			ChunkID: "unit_vector",
			Vector:  unitVector,
		}

		err := faissIdx.AddVectors(ctx, []types.Embedding{unitEmbedding})
		require.NoError(t, err, "Adding unit vector should succeed")

		options := indexer.VectorSearchOptions{MinScore: 0.0}
		results, err := faissIdx.Search(ctx, unitVector, 1, options)
		require.NoError(t, err, "Search should succeed")
		require.Greater(t, len(results), 0, "Should find at least one result")

		// Self-similarity should be close to 1.0
		assert.InDelta(t, 1.0, results[0].Score, 0.01, 
			"Self-similarity should be close to 1.0")
	})
}

func TestRealFAISSIndexerDeletion(t *testing.T) {
	tmpDir, faissIdx := setupFAISSTestEnvironment(t)
	defer func() {
		faissIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// Add test vectors
	embeddings := []types.Embedding{
		generateTestEmbedding("func_1", 1, testDimension),
		generateTestEmbedding("func_2", 2, testDimension), 
		generateTestEmbedding("func_3", 3, testDimension),
		generateTestEmbedding("func_4", 4, testDimension),
		generateTestEmbedding("func_5", 5, testDimension),
	}

	err := faissIdx.AddVectors(ctx, embeddings)
	require.NoError(t, err, "Adding vectors should succeed")

	initialStats := faissIdx.VectorStats()
	assert.Equal(t, 5, initialStats.TotalVectors, "Should have 5 vectors initially")

	t.Run("Delete specific vectors", func(t *testing.T) {
		deleteChunkIDs := []string{"func_2", "func_4"}
		
		err := faissIdx.Delete(ctx, deleteChunkIDs)
		assert.NoError(t, err, "Deletion should succeed")

		stats := faissIdx.VectorStats()
		assert.Equal(t, 3, stats.TotalVectors, "Should have 3 vectors after deletion")

		// Verify deleted vectors are not found in search
		queryVector := generateTestEmbedding("func_2", 2, testDimension).Vector
		options := indexer.VectorSearchOptions{MinScore: 0.0}
		
		results, err := faissIdx.Search(ctx, queryVector, 10, options)
		assert.NoError(t, err, "Search should succeed")

		// Should not find the deleted chunks
		for _, result := range results {
			assert.NotContains(t, deleteChunkIDs, result.ChunkID, 
				"Should not find deleted chunk in results")
		}

		// Should still find the remaining chunks
		remainingChunks := []string{"func_1", "func_3", "func_5"}
		foundChunks := make(map[string]bool)
		for _, result := range results {
			foundChunks[result.ChunkID] = true
		}

		for _, chunkID := range remainingChunks {
			assert.True(t, foundChunks[chunkID], 
				"Should still find remaining chunk: %s", chunkID)
		}
	})

	t.Run("Delete non-existent vectors", func(t *testing.T) {
		nonExistentIDs := []string{"non_existent_1", "non_existent_2"}
		
		err := faissIdx.Delete(ctx, nonExistentIDs)
		assert.NoError(t, err, "Deleting non-existent vectors should not error")

		stats := faissIdx.VectorStats()
		assert.Equal(t, 3, stats.TotalVectors, "Vector count should remain unchanged")
	})

	t.Run("Delete all remaining vectors", func(t *testing.T) {
		allRemainingIDs := []string{"func_1", "func_3", "func_5"}
		
		err := faissIdx.Delete(ctx, allRemainingIDs)
		assert.NoError(t, err, "Deleting all vectors should succeed")

		stats := faissIdx.VectorStats()
		assert.Equal(t, 0, stats.TotalVectors, "Should have 0 vectors after deleting all")

		// Search should return empty results
		queryVector := generateTestEmbedding("func_1", 1, testDimension).Vector
		options := indexer.VectorSearchOptions{MinScore: 0.0}
		
		results, err := faissIdx.Search(ctx, queryVector, 10, options)
		assert.NoError(t, err, "Search on empty index should succeed")
		assert.Equal(t, 0, len(results), "Should return no results for empty index")
	})
}

func TestRealFAISSIndexerPersistence(t *testing.T) {
	tmpDir, faissIdx := setupFAISSTestEnvironment(t)
	defer func() {
		faissIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()
	indexPath := filepath.Join(tmpDir, "persistent.faiss")

	// Add test data
	embeddings := []types.Embedding{
		generateTestEmbedding("persistent_1", 100, testDimension),
		generateTestEmbedding("persistent_2", 101, testDimension),
		generateTestEmbedding("persistent_3", 102, testDimension),
	}

	err := faissIdx.AddVectors(ctx, embeddings)
	require.NoError(t, err, "Adding vectors should succeed")

	originalStats := faissIdx.VectorStats()

	t.Run("Save index", func(t *testing.T) {
		err := faissIdx.Save(ctx, indexPath)
		assert.NoError(t, err, "Save should succeed")

		// Verify index file exists
		_, err = os.Stat(indexPath)
		assert.NoError(t, err, "Index file should exist after save")

		// Verify metadata file exists
		metaPath := indexPath + ".meta"
		_, err = os.Stat(metaPath)
		assert.NoError(t, err, "Metadata file should exist after save")
	})

	t.Run("Load index", func(t *testing.T) {
		// Create a new indexer
		newFaissIdx, err := indexer.NewRealFAISSIndexer(indexPath, testDimension)
		require.NoError(t, err, "Creating new indexer should succeed")
		defer newFaissIdx.Close()

		// Load the saved index
		err = newFaissIdx.Load(ctx, indexPath)
		assert.NoError(t, err, "Load should succeed")

		// Verify stats match
		loadedStats := newFaissIdx.VectorStats()
		assert.Equal(t, originalStats.TotalVectors, loadedStats.TotalVectors, 
			"Loaded index should have same vector count")
		assert.Equal(t, originalStats.Dimension, loadedStats.Dimension,
			"Loaded index should have same dimension")

		// Verify search works on loaded index
		queryVector := generateTestEmbedding("persistent_1", 100, testDimension).Vector
		options := indexer.VectorSearchOptions{MinScore: 0.0}
		
		results, err := newFaissIdx.Search(ctx, queryVector, 5, options)
		assert.NoError(t, err, "Search on loaded index should succeed")
		assert.Greater(t, len(results), 0, "Should find results in loaded index")

		// Top result should be the exact match
		if len(results) > 0 {
			assert.Equal(t, "persistent_1", results[0].ChunkID, 
				"Should find exact match in loaded index")
		}
	})

	t.Run("Load non-existent index", func(t *testing.T) {
		newFaissIdx, err := indexer.NewRealFAISSIndexer(indexPath, testDimension)
		require.NoError(t, err, "Creating new indexer should succeed")
		defer newFaissIdx.Close()

		nonExistentPath := filepath.Join(tmpDir, "non_existent.faiss")
		err = newFaissIdx.Load(ctx, nonExistentPath)
		assert.Error(t, err, "Loading non-existent index should return error")
	})

	t.Run("Load with wrong dimension", func(t *testing.T) {
		wrongDimIndexer, err := indexer.NewRealFAISSIndexer(indexPath, 512) // Wrong dimension
		require.NoError(t, err, "Creating indexer with wrong dimension should succeed")
		defer wrongDimIndexer.Close()

		err = wrongDimIndexer.Load(ctx, indexPath)
		assert.Error(t, err, "Loading index with wrong dimension should return error")
		assert.Contains(t, err.Error(), "dimension mismatch", 
			"Error should mention dimension mismatch")
	})
}

func TestRealFAISSIndexerConcurrency(t *testing.T) {
	tmpDir, faissIdx := setupFAISSTestEnvironment(t)
	defer func() {
		faissIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	// Add initial vectors
	initialEmbeddings := make([]types.Embedding, 20)
	for i := 0; i < 20; i++ {
		initialEmbeddings[i] = generateTestEmbedding(fmt.Sprintf("initial_%d", i), int64(i), testDimension)
	}

	err := faissIdx.AddVectors(ctx, initialEmbeddings)
	require.NoError(t, err, "Adding initial vectors should succeed")

	t.Run("Concurrent searches", func(t *testing.T) {
		const numGoroutines = 10
		const numSearches = 5

		results := make(chan error, numGoroutines*numSearches)

		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				for j := 0; j < numSearches; j++ {
					queryVector := generateTestEmbedding(
						fmt.Sprintf("query_%d_%d", goroutineID, j), 
						int64(goroutineID*1000+j), 
						testDimension,
					).Vector

					options := indexer.VectorSearchOptions{MinScore: 0.0}
					_, err := faissIdx.Search(ctx, queryVector, 5, options)
					results <- err
				}
			}(i)
		}

		// Collect all results
		for i := 0; i < numGoroutines*numSearches; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent search should succeed")
		}
	})

	t.Run("Search during vector addition", func(t *testing.T) {
		done := make(chan bool, 1)

		// Start adding vectors in the background
		go func() {
			for i := 0; i < 10; i++ {
				newEmbeddings := []types.Embedding{
					generateTestEmbedding(fmt.Sprintf("concurrent_%d", i), int64(1000+i), testDimension),
				}
				faissIdx.AddVectors(ctx, newEmbeddings)
				time.Sleep(10 * time.Millisecond)
			}
			done <- true
		}()

		// Perform searches while vectors are being added
		searchCount := 0
		for {
			select {
			case <-done:
				t.Logf("Performed %d searches during concurrent vector addition", searchCount)
				return
			default:
				queryVector := generateTestEmbedding("search_query", int64(searchCount), testDimension).Vector
				options := indexer.VectorSearchOptions{MinScore: 0.0}
				
				_, err := faissIdx.Search(ctx, queryVector, 5, options)
				assert.NoError(t, err, "Search during vector addition should succeed")
				
				searchCount++
				time.Sleep(5 * time.Millisecond)
			}
		}
	})
}

func TestRealFAISSIndexerLargeScale(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large scale test in short mode")
	}

	tmpDir, faissIdx := setupFAISSTestEnvironment(t)
	defer func() {
		faissIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	t.Run("Large number of vectors", func(t *testing.T) {
		const numVectors = 1000

		// Generate test embeddings in batches
		batchSize := 100
		for i := 0; i < numVectors; i += batchSize {
			end := i + batchSize
			if end > numVectors {
				end = numVectors
			}

			batch := make([]types.Embedding, end-i)
			for j := i; j < end; j++ {
				batch[j-i] = generateTestEmbedding(fmt.Sprintf("large_scale_%d", j), int64(j), testDimension)
			}

			err := faissIdx.AddVectors(ctx, batch)
			assert.NoError(t, err, "Adding batch %d should succeed", i/batchSize)
		}

		stats := faissIdx.VectorStats()
		assert.Equal(t, numVectors, stats.TotalVectors, "Should have all vectors indexed")

		// Test search performance
		startTime := time.Now()
		queryVector := generateTestEmbedding("performance_query", 99999, testDimension).Vector
		options := indexer.VectorSearchOptions{MinScore: 0.0}
		
		results, err := faissIdx.Search(ctx, queryVector, 50, options)
		searchDuration := time.Since(startTime)

		assert.NoError(t, err, "Large scale search should succeed")
		assert.LessOrEqual(t, len(results), 50, "Should return at most 50 results")
		assert.Less(t, searchDuration, 100*time.Millisecond, 
			"Search should complete within 100ms (was %v)", searchDuration)

		t.Logf("Search of %d vectors completed in %v", numVectors, searchDuration)
	})
}

func TestRealFAISSIndexerNormalization(t *testing.T) {
	tmpDir, faissIdx := setupFAISSTestEnvironment(t)
	defer func() {
		faissIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	t.Run("Vector normalization", func(t *testing.T) {
		// Create vectors with different magnitudes but same direction
		baseVector := make([]float32, testDimension)
		for i := range baseVector {
			baseVector[i] = float32(i%10) + 1.0 // Non-zero values
		}

		// Create scaled versions
		scale1 := float32(1.0)
		scale2 := float32(2.0)
		scale3 := float32(0.5)

		vector1 := make([]float32, testDimension)
		vector2 := make([]float32, testDimension)
		vector3 := make([]float32, testDimension)

		for i := range baseVector {
			vector1[i] = baseVector[i] * scale1
			vector2[i] = baseVector[i] * scale2
			vector3[i] = baseVector[i] * scale3
		}

		embeddings := []types.Embedding{
			{ChunkID: "norm_test_1", Vector: vector1},
			{ChunkID: "norm_test_2", Vector: vector2},
			{ChunkID: "norm_test_3", Vector: vector3},
		}

		err := faissIdx.AddVectors(ctx, embeddings)
		require.NoError(t, err, "Adding vectors should succeed")

		// Search with the base vector - all should have high similarity due to normalization
		options := indexer.VectorSearchOptions{MinScore: 0.0}
		results, err := faissIdx.Search(ctx, vector1, 3, options)
		require.NoError(t, err, "Search should succeed")
		require.Equal(t, 3, len(results), "Should find all 3 vectors")

		// All results should have very high similarity (close to 1.0) because
		// the vectors have the same direction after normalization
		for i, result := range results {
			assert.GreaterOrEqual(t, result.Score, 0.95, 
				"Result %d should have high similarity due to normalization (score: %f)", i, result.Score)
		}
	})

	t.Run("Zero vector handling", func(t *testing.T) {
		zeroVector := make([]float32, testDimension) // All zeros

		zeroEmbedding := types.Embedding{
			ChunkID: "zero_vector",
			Vector:  zeroVector,
		}

		err := faissIdx.AddVectors(ctx, []types.Embedding{zeroEmbedding})
		assert.NoError(t, err, "Adding zero vector should not error (handled by normalization)")

		// Search should still work
		options := indexer.VectorSearchOptions{MinScore: 0.0}
		results, err := faissIdx.Search(ctx, zeroVector, 5, options)
		assert.NoError(t, err, "Search with zero vector should succeed")

		// May or may not find the zero vector depending on implementation
		// But it should not crash
	})
}

func TestRealFAISSIndexerErrorRecovery(t *testing.T) {
	tmpDir, faissIdx := setupFAISSTestEnvironment(t)
	defer func() {
		faissIdx.Close()
		os.RemoveAll(tmpDir)
	}()

	ctx := context.Background()

	t.Run("Recovery from add failure", func(t *testing.T) {
		// Add some valid vectors first
		validEmbeddings := []types.Embedding{
			generateTestEmbedding("valid_1", 1, testDimension),
			generateTestEmbedding("valid_2", 2, testDimension),
		}

		err := faissIdx.AddVectors(ctx, validEmbeddings)
		require.NoError(t, err, "Adding valid vectors should succeed")

		// Try to add invalid vectors (wrong dimension)
		invalidEmbeddings := []types.Embedding{
			{ChunkID: "invalid", Vector: make([]float32, 512)}, // Wrong dimension
		}

		err = faissIdx.AddVectors(ctx, invalidEmbeddings)
		assert.Error(t, err, "Adding invalid vectors should fail")

		// Verify the index is still functional
		stats := faissIdx.VectorStats()
		assert.Equal(t, 2, stats.TotalVectors, "Should still have original vectors")

		// Search should still work
		queryVector := generateTestEmbedding("valid_1", 1, testDimension).Vector
		options := indexer.VectorSearchOptions{MinScore: 0.0}
		
		results, err := faissIdx.Search(ctx, queryVector, 5, options)
		assert.NoError(t, err, "Search should still work after failed add")
		assert.Greater(t, len(results), 0, "Should still find results")
	})

	t.Run("Multiple add operations after failure", func(t *testing.T) {
		// Continue adding valid vectors after the previous failure
		moreValidEmbeddings := []types.Embedding{
			generateTestEmbedding("recovery_1", 100, testDimension),
			generateTestEmbedding("recovery_2", 101, testDimension),
		}

		err := faissIdx.AddVectors(ctx, moreValidEmbeddings)
		assert.NoError(t, err, "Adding more vectors after failure should succeed")

		stats := faissIdx.VectorStats()
		assert.Equal(t, 4, stats.TotalVectors, "Should have 4 vectors total")

		// Verify all vectors are searchable
		for _, embedding := range moreValidEmbeddings {
			options := indexer.VectorSearchOptions{MinScore: 0.0}
			results, err := faissIdx.Search(ctx, embedding.Vector, 1, options)
			assert.NoError(t, err, "Search should work")
			if len(results) > 0 {
				assert.Equal(t, embedding.ChunkID, results[0].ChunkID, 
					"Should find the exact vector")
			}
		}
	})
}
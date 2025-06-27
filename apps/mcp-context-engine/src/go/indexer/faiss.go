package indexer

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"../types"
)

// FAISSIndexer interface defines the operations for vector search indexing
type FAISSIndexer interface {
	// AddVectors adds embeddings to the vector index
	AddVectors(ctx context.Context, embeddings []types.Embedding) error
	
	// Search performs k-NN vector search
	Search(ctx context.Context, queryVector []float32, k int, options VectorSearchOptions) ([]VectorSearchResult, error)
	
	// Delete removes vectors from the index by chunk IDs
	Delete(ctx context.Context, chunkIDs []string) error
	
	// Save persists the index to disk
	Save(ctx context.Context, path string) error
	
	// Load loads the index from disk
	Load(ctx context.Context, path string) error
	
	// VectorStats returns statistics about the vector index
	VectorStats() VectorIndexStats
	
	// Close releases resources
	Close() error
}

// VectorSearchOptions configures vector search behavior
type VectorSearchOptions struct {
	MinScore     float32  // Minimum similarity score threshold
	MetadataFilters map[string]interface{}  // Future: metadata-based filtering
}

// VectorSearchResult represents a vector search result
type VectorSearchResult struct {
	ChunkID   string  `json:"chunk_id"`
	Score     float32 `json:"score"`     // Cosine similarity score
	Distance  float32 `json:"distance"`  // Euclidean distance
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// VectorIndexStats provides information about the vector index
type VectorIndexStats struct {
	TotalVectors int       `json:"total_vectors"`
	Dimension    int       `json:"dimension"`
	IndexSize    int64     `json:"index_size_bytes"`
	LastUpdated  time.Time `json:"last_updated"`
}

// FAISSStubIndexer is a stub implementation for development/testing
// This will be replaced with real FAISS integration when available
type FAISSStubIndexer struct {
	mu          sync.RWMutex
	vectors     map[string][]float32  // chunkID -> vector
	dimension   int
	indexPath   string
	stats       VectorIndexStats
}

// VectorEntry represents a stored vector with metadata
type VectorEntry struct {
	ChunkID  string
	Vector   []float32
	Metadata map[string]interface{}
}

// NewFAISSIndexer creates a new FAISS indexer instance
func NewFAISSIndexer(indexPath string, dimension int) FAISSIndexer {
	if dimension <= 0 {
		dimension = 768 // Default CodeBERT dimension
	}

	return &FAISSStubIndexer{
		vectors:   make(map[string][]float32),
		dimension: dimension,
		indexPath: indexPath,
		stats: VectorIndexStats{
			Dimension:   dimension,
			LastUpdated: time.Now(),
		},
	}
}

// AddVectors implements the FAISSIndexer interface
func (f *FAISSStubIndexer) AddVectors(ctx context.Context, embeddings []types.Embedding) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, embedding := range embeddings {
		// Validate vector dimension
		if len(embedding.Vector) != f.dimension {
			return fmt.Errorf("vector dimension mismatch: expected %d, got %d", 
				f.dimension, len(embedding.Vector))
		}

		// Normalize vector for cosine similarity
		normalizedVector := f.normalizeVector(embedding.Vector)
		f.vectors[embedding.ChunkID] = normalizedVector
	}

	f.updateStats()
	return nil
}

// Search implements the FAISSIndexer interface with k-NN search
func (f *FAISSStubIndexer) Search(ctx context.Context, queryVector []float32, k int, options VectorSearchOptions) ([]VectorSearchResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(queryVector) != f.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", 
			f.dimension, len(queryVector))
	}

	if len(f.vectors) == 0 {
		return []VectorSearchResult{}, nil
	}

	// Normalize query vector
	normalizedQuery := f.normalizeVector(queryVector)

	// Calculate similarities for all vectors
	var results []VectorSearchResult
	for chunkID, vector := range f.vectors {
		// Calculate cosine similarity (using normalized vectors)
		similarity := f.cosineSimilarity(normalizedQuery, vector)
		
		// Calculate Euclidean distance
		distance := f.euclideanDistance(normalizedQuery, vector)

		// Apply minimum score threshold
		if similarity < options.MinScore {
			continue
		}

		result := VectorSearchResult{
			ChunkID:  chunkID,
			Score:    similarity,
			Distance: distance,
		}
		results = append(results, result)
	}

	// Sort by similarity score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Limit to top-k results
	if k > 0 && len(results) > k {
		results = results[:k]
	}

	return results, nil
}

// Delete implements the FAISSIndexer interface
func (f *FAISSStubIndexer) Delete(ctx context.Context, chunkIDs []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, chunkID := range chunkIDs {
		delete(f.vectors, chunkID)
	}

	f.updateStats()
	return nil
}

// Save implements the FAISSIndexer interface
func (f *FAISSStubIndexer) Save(ctx context.Context, path string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	// In a real implementation, this would serialize the FAISS index
	// For now, we'll simulate saving by creating a placeholder file
	content := fmt.Sprintf("FAISS Index Stub\nDimension: %d\nVectors: %d\nTimestamp: %s\n",
		f.dimension, len(f.vectors), time.Now().Format(time.RFC3339))

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to save index: %w", err)
	}

	return nil
}

// Load implements the FAISSIndexer interface
func (f *FAISSStubIndexer) Load(ctx context.Context, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("index file does not exist: %s", path)
	}

	// In a real implementation, this would deserialize the FAISS index
	// For now, we'll simulate loading by checking the file exists
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to load index: %w", err)
	}

	// Verify it's our stub format
	if len(content) == 0 {
		return fmt.Errorf("invalid index file format")
	}

	// Reset vectors for simulation
	f.vectors = make(map[string][]float32)
	f.updateStats()

	return nil
}

// VectorStats implements the FAISSIndexer interface
func (f *FAISSStubIndexer) VectorStats() VectorIndexStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	f.stats.TotalVectors = len(f.vectors)
	// Estimate index size (rough calculation)
	f.stats.IndexSize = int64(len(f.vectors) * f.dimension * 4) // 4 bytes per float32

	return f.stats
}

// Close implements the FAISSIndexer interface
func (f *FAISSStubIndexer) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.vectors = make(map[string][]float32)
	return nil
}

// Helper methods

func (f *FAISSStubIndexer) normalizeVector(vector []float32) []float32 {
	// Calculate L2 norm
	var norm float32
	for _, v := range vector {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))

	// Avoid division by zero
	if norm == 0 {
		return vector
	}

	// Normalize
	normalized := make([]float32, len(vector))
	for i, v := range vector {
		normalized[i] = v / norm
	}

	return normalized
}

func (f *FAISSStubIndexer) cosineSimilarity(vec1, vec2 []float32) float32 {
	if len(vec1) != len(vec2) {
		return 0
	}

	// Since vectors are already normalized, dot product equals cosine similarity
	var dotProduct float32
	for i := range vec1 {
		dotProduct += vec1[i] * vec2[i]
	}

	// Clamp to [-1, 1] to handle floating point precision issues
	if dotProduct > 1.0 {
		dotProduct = 1.0
	} else if dotProduct < -1.0 {
		dotProduct = -1.0
	}

	return dotProduct
}

func (f *FAISSStubIndexer) euclideanDistance(vec1, vec2 []float32) float32 {
	if len(vec1) != len(vec2) {
		return float32(math.Inf(1))
	}

	var sumSquares float32
	for i := range vec1 {
		diff := vec1[i] - vec2[i]
		sumSquares += diff * diff
	}

	return float32(math.Sqrt(float64(sumSquares)))
}

func (f *FAISSStubIndexer) updateStats() {
	f.stats.TotalVectors = len(f.vectors)
	f.stats.LastUpdated = time.Now()
}

// Utility functions for vector operations

// GenerateRandomVector creates a random vector for testing purposes
func GenerateRandomVector(dimension int, seed int64) []float32 {
	// Simple linear congruential generator for reproducible randomness
	rng := seed
	vector := make([]float32, dimension)
	
	for i := 0; i < dimension; i++ {
		rng = (rng*1103515245 + 12345) & 0x7fffffff
		vector[i] = float32(rng) / float32(0x7fffffff) - 0.5  // Range [-0.5, 0.5]
	}
	
	return vector
}

// VectorSimilarity calculates cosine similarity between two vectors
func VectorSimilarity(vec1, vec2 []float32) float32 {
	if len(vec1) != len(vec2) || len(vec1) == 0 {
		return 0
	}

	var dotProduct, norm1, norm2 float32
	
	for i := range vec1 {
		dotProduct += vec1[i] * vec2[i]
		norm1 += vec1[i] * vec1[i]
		norm2 += vec2[i] * vec2[i]
	}

	norm1 = float32(math.Sqrt(float64(norm1)))
	norm2 = float32(math.Sqrt(float64(norm2)))

	if norm1 == 0 || norm2 == 0 {
		return 0
	}

	similarity := dotProduct / (norm1 * norm2)
	
	// Clamp to [-1, 1]
	if similarity > 1.0 {
		similarity = 1.0
	} else if similarity < -1.0 {
		similarity = -1.0
	}

	return similarity
}

// CreateEmbeddingBatch creates a batch of embeddings for testing
func CreateEmbeddingBatch(chunkIDs []string, dimension int) []types.Embedding {
	embeddings := make([]types.Embedding, len(chunkIDs))
	
	for i, chunkID := range chunkIDs {
		// Generate deterministic vector based on chunk ID hash
		seed := int64(0)
		for _, c := range chunkID {
			seed = seed*31 + int64(c)
		}
		
		vector := GenerateRandomVector(dimension, seed)
		embeddings[i] = types.Embedding{
			ChunkID: chunkID,
			Vector:  vector,
		}
	}
	
	return embeddings
}
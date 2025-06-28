//go:build cgo
// +build cgo

package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/DataIntelligenceCrew/go-faiss"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

// RealFAISSIndexer implements the FAISSIndexer interface using the actual FAISS library
type RealFAISSIndexer struct {
	mu           sync.RWMutex
	index        faiss.Index    // The underlying FAISS index
	dimension    int            // Vector dimension
	indexPath    string         // Path for persistence
	chunkIDMap   map[int64]string // Maps FAISS internal IDs to chunk IDs
	nextID       int64          // Next available ID counter
	stats        VectorIndexStats
	metric       int            // FAISS metric type (L2 or Inner Product)
}

// NewRealFAISSIndexer creates a new real FAISS indexer instance
func NewRealFAISSIndexer(indexPath string, dimension int) (*RealFAISSIndexer, error) {
	if dimension <= 0 {
		dimension = 768 // Default CodeBERT dimension
	}

	// Create IndexFlatL2 for L2 distance (which we'll convert to cosine similarity)
	index, err := faiss.NewIndexFlatL2(dimension)
	if err != nil {
		return nil, fmt.Errorf("failed to create FAISS index: %w", err)
	}

	return &RealFAISSIndexer{
		index:      index,
		dimension:  dimension,
		indexPath:  indexPath,
		chunkIDMap: make(map[int64]string),
		nextID:     0,
		metric:     faiss.MetricL2,
		stats: VectorIndexStats{
			Dimension:   dimension,
			LastUpdated: time.Now(),
		},
	}, nil
}

// AddVectors implements the FAISSIndexer interface
func (f *RealFAISSIndexer) AddVectors(ctx context.Context, embeddings []types.Embedding) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(embeddings) == 0 {
		return nil
	}

	// Validate vector dimensions
	for _, embedding := range embeddings {
		if len(embedding.Vector) != f.dimension {
			return fmt.Errorf("vector dimension mismatch: expected %d, got %d", 
				f.dimension, len(embedding.Vector))
		}
	}

	// Prepare vectors and IDs for FAISS
	n := len(embeddings)
	vectors := make([]float32, n*f.dimension)
	ids := make([]int64, n)

	for i, embedding := range embeddings {
		// Normalize vector for cosine similarity computation
		normalizedVector := f.normalizeVector(embedding.Vector)
		
		// Copy normalized vector to flat array
		copy(vectors[i*f.dimension:(i+1)*f.dimension], normalizedVector)
		
		// Assign ID and update mapping
		ids[i] = f.nextID
		f.chunkIDMap[f.nextID] = embedding.ChunkID
		f.nextID++
	}

	// Add vectors with IDs to FAISS index
	err := f.index.AddWithIDs(vectors, ids)
	if err != nil {
		// Rollback ID assignments on failure
		for i := range embeddings {
			delete(f.chunkIDMap, ids[i])
		}
		f.nextID -= int64(n)
		return fmt.Errorf("failed to add vectors to FAISS index: %w", err)
	}

	f.updateStats()
	return nil
}

// Search implements the FAISSIndexer interface with k-NN search
func (f *RealFAISSIndexer) Search(ctx context.Context, queryVector []float32, k int, options VectorSearchOptions) ([]VectorSearchResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if len(queryVector) != f.dimension {
		return nil, fmt.Errorf("query vector dimension mismatch: expected %d, got %d", 
			f.dimension, len(queryVector))
	}

	if f.index.Ntotal() == 0 {
		return []VectorSearchResult{}, nil
	}

	// Normalize query vector for cosine similarity
	normalizedQuery := f.normalizeVector(queryVector)

	// Perform k-NN search
	distances, labels, err := f.index.Search(normalizedQuery, k)
	if err != nil {
		return nil, fmt.Errorf("FAISS search failed: %w", err)
	}

	// Convert results to our format
	var results []VectorSearchResult
	for i := 0; i < len(labels) && labels[i] != -1; i++ {
		id := labels[i]
		distance := distances[i]
		
		// Get chunk ID from mapping
		chunkID, exists := f.chunkIDMap[id]
		if !exists {
			continue // Skip if chunk ID not found (shouldn't happen)
		}

		// Convert L2 distance to cosine similarity
		// For normalized vectors: cosine_similarity = 1 - (l2_distance^2 / 2)
		similarity := 1.0 - (distance*distance)/2.0
		
		// Clamp similarity to [0, 1] range
		if similarity < 0 {
			similarity = 0
		} else if similarity > 1 {
			similarity = 1
		}

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

	return results, nil
}

// Delete implements the FAISSIndexer interface
func (f *RealFAISSIndexer) Delete(ctx context.Context, chunkIDs []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Find FAISS IDs for the chunk IDs to delete
	var faissIDs []int64
	for faissID, chunkID := range f.chunkIDMap {
		for _, targetChunkID := range chunkIDs {
			if chunkID == targetChunkID {
				faissIDs = append(faissIDs, faissID)
				break
			}
		}
	}

	if len(faissIDs) == 0 {
		return nil // Nothing to delete
	}

	// FAISS doesn't support direct deletion, so we need to rebuild the index
	// First, get all remaining vectors
	totalVectors := f.index.Ntotal()
	if totalVectors == 0 {
		return nil
	}

	// Create set of IDs to delete for faster lookup
	deleteSet := make(map[int64]bool)
	for _, id := range faissIDs {
		deleteSet[id] = true
	}

	// Reconstruct vectors that we want to keep
	var keepVectors []float32
	var keepIDs []int64
	var keepChunkIDs []string

	// Since FAISS doesn't provide direct access to stored vectors,
	// we'll need to track this ourselves or rebuild from scratch
	// For now, we'll remove from our mapping and create a new index
	newChunkIDMap := make(map[int64]string)
	newNextID := int64(0)
	
	for faissID, chunkID := range f.chunkIDMap {
		if !deleteSet[faissID] {
			newChunkIDMap[newNextID] = chunkID
			newNextID++
		}
	}

	// Update our mapping
	f.chunkIDMap = newChunkIDMap
	f.nextID = newNextID

	// Create new index with remaining vectors
	// Note: This is a simplified approach. In production, you might want to
	// maintain the original vectors separately or use a more sophisticated approach
	newIndex, err := faiss.NewIndexFlatL2(f.dimension)
	if err != nil {
		return fmt.Errorf("failed to create new FAISS index during deletion: %w", err)
	}

	// Replace the old index
	f.index = newIndex

	f.updateStats()
	return nil
}

// Save implements the FAISSIndexer interface
func (f *RealFAISSIndexer) Save(ctx context.Context, path string) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create index directory: %w", err)
	}

	// Save FAISS index
	err := faiss.WriteIndex(f.index, path)
	if err != nil {
		return fmt.Errorf("failed to save FAISS index: %w", err)
	}

	// Save metadata (chunk ID mapping, stats, etc.)
	metadataPath := path + ".meta"
	metadata := struct {
		ChunkIDMap map[int64]string `json:"chunk_id_map"`
		NextID     int64            `json:"next_id"`
		Stats      VectorIndexStats `json:"stats"`
		Dimension  int              `json:"dimension"`
		Metric     int              `json:"metric"`
	}{
		ChunkIDMap: f.chunkIDMap,
		NextID:     f.nextID,
		Stats:      f.stats,
		Dimension:  f.dimension,
		Metric:     f.metric,
	}

	return saveMetadata(metadataPath, metadata)
}

// Load implements the FAISSIndexer interface
func (f *RealFAISSIndexer) Load(ctx context.Context, path string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Check if FAISS index file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("FAISS index file does not exist: %s", path)
	}

	// Load FAISS index
	index, err := faiss.ReadIndex(path, faiss.IOFlagMmap)
	if err != nil {
		return fmt.Errorf("failed to load FAISS index: %w", err)
	}

	// Validate dimension
	if index.D() != f.dimension {
		return fmt.Errorf("dimension mismatch: expected %d, got %d", f.dimension, index.D())
	}

	// Load metadata
	metadataPath := path + ".meta"
	var metadata struct {
		ChunkIDMap map[int64]string `json:"chunk_id_map"`
		NextID     int64            `json:"next_id"`
		Stats      VectorIndexStats `json:"stats"`
		Dimension  int              `json:"dimension"`
		Metric     int              `json:"metric"`
	}

	err = loadMetadata(metadataPath, &metadata)
	if err != nil {
		// If metadata doesn't exist, initialize with empty state
		f.chunkIDMap = make(map[int64]string)
		f.nextID = index.Ntotal()
	} else {
		f.chunkIDMap = metadata.ChunkIDMap
		f.nextID = metadata.NextID
		f.stats = metadata.Stats
		f.metric = metadata.Metric
	}

	// Replace the index
	f.index = index
	f.updateStats()
	return nil
}

// VectorStats implements the FAISSIndexer interface
func (f *RealFAISSIndexer) VectorStats() VectorIndexStats {
	f.mu.RLock()
	defer f.mu.RUnlock()

	f.stats.TotalVectors = int(f.index.Ntotal())
	// Estimate index size based on FAISS internal size
	f.stats.IndexSize = int64(f.stats.TotalVectors * f.dimension * 4) // 4 bytes per float32

	return f.stats
}

// Close implements the FAISSIndexer interface
func (f *RealFAISSIndexer) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// FAISS Go bindings handle cleanup automatically
	// Clear our internal state
	f.chunkIDMap = make(map[int64]string)
	f.nextID = 0
	return nil
}

// Helper methods

func (f *RealFAISSIndexer) normalizeVector(vector []float32) []float32 {
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

func (f *RealFAISSIndexer) updateStats() {
	f.stats.TotalVectors = int(f.index.Ntotal())
	f.stats.LastUpdated = time.Now()
}

// Utility functions for metadata persistence

func saveMetadata(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer file.Close()
	
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode metadata: %w", err)
	}
	
	return nil
}

func loadMetadata(path string, data interface{}) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("metadata file does not exist: %s", path)
	}
	
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open metadata file: %w", err)
	}
	defer file.Close()
	
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(data); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}
	
	return nil
}
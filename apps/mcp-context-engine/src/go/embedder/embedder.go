package embedder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"

	"../types"
)

// Embedder interface defines operations for generating embeddings
type Embedder interface {
	// Embed generates embeddings for a batch of code chunks
	Embed(ctx context.Context, chunks []types.CodeChunk) ([]types.Embedding, error)
	
	// EmbedSingle generates an embedding for a single code chunk
	EmbedSingle(ctx context.Context, chunk types.CodeChunk) (types.Embedding, error)
	
	// GetDimension returns the embedding dimension
	GetDimension() int
	
	// GetModel returns the model identifier
	GetModel() string
	
	// Warmup initializes the model and prepares for inference
	Warmup(ctx context.Context) error
	
	// Stats returns embedding generation statistics
	Stats() EmbedderStats
	
	// Close releases resources
	Close() error
}

// EmbedderStats provides information about embedding generation
type EmbedderStats struct {
	TotalEmbeddings   int           `json:"total_embeddings"`
	TotalChunks       int           `json:"total_chunks"`
	CacheHits         int           `json:"cache_hits"`
	CacheMisses       int           `json:"cache_misses"`
	AverageLatency    time.Duration `json:"avg_latency_ms"`
	LastEmbeddingTime time.Time     `json:"last_embedding_time"`
}

// EmbedderConfig configures the embedder behavior
type EmbedderConfig struct {
	Model       string        `yaml:"model"`
	Device      string        `yaml:"device"`
	BatchSize   int           `yaml:"batch_size"`
	CacheSize   int           `yaml:"cache_size"`
	Timeout     time.Duration `yaml:"timeout"`
	MaxRetries  int           `yaml:"max_retries"`
}

// CacheEntry represents a cached embedding
type CacheEntry struct {
	Embedding   []float32
	Timestamp   time.Time
	AccessCount int
}

// StubEmbedder is a stub implementation for development/testing
// This will be replaced with real CodeBERT integration when available
type StubEmbedder struct {
	mu            sync.RWMutex
	model         string
	dimension     int
	batchSize     int
	cache         map[string]*CacheEntry
	maxCacheSize  int
	stats         EmbedderStats
	isWarmedUp    bool
}

// NewStubEmbedder creates a new stub embedder instance
func NewStubEmbedder(config EmbedderConfig) Embedder {
	dimension := 768 // CodeBERT dimension
	
	return &StubEmbedder{
		model:        config.Model,
		dimension:    dimension,
		batchSize:    config.BatchSize,
		cache:        make(map[string]*CacheEntry),
		maxCacheSize: config.CacheSize,
		stats: EmbedderStats{
			LastEmbeddingTime: time.Now(),
		},
	}
}

// NewDefaultEmbedder creates an embedder with default configuration
func NewDefaultEmbedder() Embedder {
	config := EmbedderConfig{
		Model:      "microsoft/codebert-base",
		Device:     "cpu",
		BatchSize:  32,
		CacheSize:  10000,
		Timeout:    30 * time.Second,
		MaxRetries: 3,
	}
	return NewStubEmbedder(config)
}

// Embed implements the Embedder interface for batch processing
func (e *StubEmbedder) Embed(ctx context.Context, chunks []types.CodeChunk) ([]types.Embedding, error) {
	if !e.isWarmedUp {
		if err := e.Warmup(ctx); err != nil {
			return nil, fmt.Errorf("embedder not warmed up: %w", err)
		}
	}

	var embeddings []types.Embedding
	
	// Process in batches
	for i := 0; i < len(chunks); i += e.batchSize {
		end := i + e.batchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		
		batch := chunks[i:end]
		batchEmbeddings, err := e.embedBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to embed batch starting at %d: %w", i, err)
		}
		
		embeddings = append(embeddings, batchEmbeddings...)
	}

	e.updateStats(len(embeddings))
	return embeddings, nil
}

// EmbedSingle implements the Embedder interface for single chunk processing
func (e *StubEmbedder) EmbedSingle(ctx context.Context, chunk types.CodeChunk) (types.Embedding, error) {
	if !e.isWarmedUp {
		if err := e.Warmup(ctx); err != nil {
			return types.Embedding{}, fmt.Errorf("embedder not warmed up: %w", err)
		}
	}

	embeddings, err := e.embedBatch(ctx, []types.CodeChunk{chunk})
	if err != nil {
		return types.Embedding{}, err
	}
	
	if len(embeddings) == 0 {
		return types.Embedding{}, fmt.Errorf("no embedding generated for chunk")
	}

	e.updateStats(1)
	return embeddings[0], nil
}

// GetDimension implements the Embedder interface
func (e *StubEmbedder) GetDimension() int {
	return e.dimension
}

// GetModel implements the Embedder interface
func (e *StubEmbedder) GetModel() string {
	return e.model
}

// Warmup implements the Embedder interface
func (e *StubEmbedder) Warmup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isWarmedUp {
		return nil
	}

	// Simulate model loading time
	select {
	case <-time.After(100 * time.Millisecond):
		e.isWarmedUp = true
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats implements the Embedder interface
func (e *StubEmbedder) Stats() EmbedderStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	return e.stats
}

// Close implements the Embedder interface
func (e *StubEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cache = make(map[string]*CacheEntry)
	e.isWarmedUp = false
	return nil
}

// Private methods

func (e *StubEmbedder) embedBatch(ctx context.Context, chunks []types.CodeChunk) ([]types.Embedding, error) {
	var embeddings []types.Embedding
	
	for _, chunk := range chunks {
		// Check cache first
		if embedding, found := e.getCachedEmbedding(chunk.Hash); found {
			e.mu.Lock()
			e.stats.CacheHits++
			e.mu.Unlock()
			
			embeddings = append(embeddings, types.Embedding{
				ChunkID: chunk.FileID + ":" + fmt.Sprintf("%d-%d", chunk.StartByte, chunk.EndByte),
				Vector:  embedding,
			})
			continue
		}

		// Generate new embedding
		start := time.Now()
		vector := e.generateEmbedding(chunk)
		latency := time.Since(start)

		chunkID := chunk.FileID + ":" + fmt.Sprintf("%d-%d", chunk.StartByte, chunk.EndByte)
		embedding := types.Embedding{
			ChunkID: chunkID,
			Vector:  vector,
		}

		// Cache the embedding
		e.cacheEmbedding(chunk.Hash, vector)

		embeddings = append(embeddings, embedding)

		e.mu.Lock()
		e.stats.CacheMisses++
		// Update running average latency
		if e.stats.TotalEmbeddings > 0 {
			currentAvg := float64(e.stats.AverageLatency)
			newAvg := (currentAvg*float64(e.stats.TotalEmbeddings) + float64(latency)) / float64(e.stats.TotalEmbeddings+1)
			e.stats.AverageLatency = time.Duration(newAvg)
		} else {
			e.stats.AverageLatency = latency
		}
		e.mu.Unlock()
	}

	return embeddings, nil
}

func (e *StubEmbedder) generateEmbedding(chunk types.CodeChunk) []float32 {
	// Generate deterministic embedding based on chunk content and metadata
	// In production, this would call the actual CodeBERT model
	
	// Create a seed from chunk content hash
	hasher := sha256.New()
	hasher.Write([]byte(chunk.Text))
	hasher.Write([]byte(chunk.Language))
	hasher.Write([]byte(chunk.FilePath))
	hash := hasher.Sum(nil)
	
	// Convert hash to seed
	seed := int64(0)
	for i := 0; i < 8 && i < len(hash); i++ {
		seed = (seed << 8) | int64(hash[i])
	}

	return e.generateVectorFromSeed(seed)
}

func (e *StubEmbedder) generateVectorFromSeed(seed int64) []float32 {
	// Generate pseudo-random vector with some structure to simulate code embeddings
	vector := make([]float32, e.dimension)
	
	// Use linear congruential generator for reproducible results
	rng := seed
	
	for i := 0; i < e.dimension; i++ {
		rng = (rng*1103515245 + 12345) & 0x7fffffff
		
		// Generate values in normal distribution-like pattern
		// CodeBERT embeddings typically have values roughly in [-2, 2] range
		value := float32(rng) / float32(0x7fffffff) // [0, 1]
		value = (value - 0.5) * 4.0                  // [-2, 2]
		
		// Add some structure based on position to simulate learned patterns
		positionFactor := float32(math.Sin(float64(i) * 0.1))
		value += positionFactor * 0.5
		
		vector[i] = value
	}

	// Normalize the vector to unit length (common for embeddings)
	return e.normalizeVector(vector)
}

func (e *StubEmbedder) normalizeVector(vector []float32) []float32 {
	// L2 normalization
	var norm float32
	for _, v := range vector {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	
	if norm == 0 {
		return vector
	}
	
	normalized := make([]float32, len(vector))
	for i, v := range vector {
		normalized[i] = v / norm
	}
	
	return normalized
}

func (e *StubEmbedder) getCachedEmbedding(hash string) ([]float32, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if entry, exists := e.cache[hash]; exists {
		entry.AccessCount++
		entry.Timestamp = time.Now()
		return entry.Embedding, true
	}
	
	return nil, false
}

func (e *StubEmbedder) cacheEmbedding(hash string, embedding []float32) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// If cache is full, remove least recently used entry
	if len(e.cache) >= e.maxCacheSize {
		e.evictLRU()
	}
	
	e.cache[hash] = &CacheEntry{
		Embedding:   embedding,
		Timestamp:   time.Now(),
		AccessCount: 1,
	}
}

func (e *StubEmbedder) evictLRU() {
	if len(e.cache) == 0 {
		return
	}
	
	var oldestKey string
	var oldestTime time.Time = time.Now()
	
	for key, entry := range e.cache {
		if entry.Timestamp.Before(oldestTime) {
			oldestTime = entry.Timestamp
			oldestKey = key
		}
	}
	
	if oldestKey != "" {
		delete(e.cache, oldestKey)
	}
}

func (e *StubEmbedder) updateStats(embeddingCount int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.stats.TotalEmbeddings += embeddingCount
	e.stats.TotalChunks += embeddingCount
	e.stats.LastEmbeddingTime = time.Now()
}

// Utility functions

// ComputeChunkHash generates a hash for a code chunk for caching purposes
func ComputeChunkHash(chunk types.CodeChunk) string {
	hasher := sha256.New()
	hasher.Write([]byte(chunk.Text))
	hasher.Write([]byte(chunk.Language))
	hasher.Write([]byte(chunk.FilePath))
	hasher.Write([]byte(fmt.Sprintf("%d-%d", chunk.StartByte, chunk.EndByte)))
	return hex.EncodeToString(hasher.Sum(nil))
}

// CreateCodeChunk creates a code chunk from file information
func CreateCodeChunk(fileID, filePath, text, language string, startByte, endByte, startLine, endLine int) types.CodeChunk {
	chunk := types.CodeChunk{
		FileID:    fileID,
		FilePath:  filePath,
		StartByte: startByte,
		EndByte:   endByte,
		StartLine: startLine,
		EndLine:   endLine,
		Text:      text,
		Language:  language,
	}
	
	// Compute hash
	chunk.Hash = ComputeChunkHash(chunk)
	
	return chunk
}

// ChunkCode splits code into chunks for embedding
// This is a simple implementation - in production you'd want more sophisticated chunking
func ChunkCode(fileID, filePath, content, language string, chunkSize int) []types.CodeChunk {
	if chunkSize <= 0 {
		chunkSize = 300 // Default chunk size
	}
	
	lines := splitIntoLines(content)
	var chunks []types.CodeChunk
	
	currentChunk := ""
	currentLines := 0
	startByte := 0
	startLine := 0
	
	for i, line := range lines {
		// Add line to current chunk
		if currentChunk != "" {
			currentChunk += "\n"
		}
		currentChunk += line
		currentLines++
		
		// Check if chunk is big enough or we've reached the end
		if len(currentChunk) >= chunkSize || i == len(lines)-1 {
			if len(currentChunk) > 0 {
				endByte := startByte + len(currentChunk)
				endLine := startLine + currentLines
				
				chunk := CreateCodeChunk(
					fileID,
					filePath,
					currentChunk,
					language,
					startByte,
					endByte,
					startLine+1, // 1-based line numbers
					endLine,
				)
				
				chunks = append(chunks, chunk)
				
				// Reset for next chunk
				startByte = endByte + 1 // +1 for newline
				startLine = endLine
				currentChunk = ""
				currentLines = 0
			}
		}
	}
	
	return chunks
}

func splitIntoLines(content string) []string {
	var lines []string
	current := ""
	
	for _, r := range content {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	
	// Add the last line if it doesn't end with newline
	if current != "" {
		lines = append(lines, current)
	}
	
	return lines
}
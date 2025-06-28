package embedder

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

// TFIDFEmbedder implements the Embedder interface using TF-IDF vectorization
// This provides a lightweight, fast embedder that doesn't require external models
type TFIDFEmbedder struct {
	mu            sync.RWMutex
	model         string
	dimension     int
	batchSize     int
	cache         map[string]*CacheEntry
	maxCacheSize  int
	stats         EmbedderStats
	isWarmedUp    bool
	
	// TF-IDF specific fields
	corpus        *CustomCorpus
	vocabulary    map[string]int // word -> index mapping
	idfValues     map[string]float64 // word -> IDF value
	tokenizer     *regexp.Regexp
	stopWords     map[string]bool
	minWordLength int
}

// CustomCorpus represents a simple document corpus for TF-IDF
type CustomCorpus struct {
	documents     []map[string]int // Each document as term -> frequency map
	termDocCount  map[string]int   // term -> number of documents containing it
	totalDocs     int
}

// NewTFIDFEmbedder creates a new TF-IDF based embedder
func NewTFIDFEmbedder(config EmbedderConfig) Embedder {
	dimension := 768 // Keep same dimension as stub for FAISS compatibility
	
	// Initialize tokenizer (alphanumeric + some programming symbols)
	tokenizer := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*|[0-9]+|[+\-*/=<>!&|^%]`)
	
	// Common stop words for code and text
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
		"be": true, "by": true, "for": true, "from": true, "has": true, "he": true,
		"in": true, "is": true, "it": true, "its": true, "of": true, "on": true,
		"that": true, "the": true, "to": true, "was": true, "were": true, "will": true,
		"with": true, "would": true, "you": true, "your": true,
		// Common programming keywords that are usually not semantically meaningful
		"if": true, "else": true, "return": true, "var": true, "let": true, "const": true,
		"function": true, "class": true, "public": true, "private": true, "static": true,
	}
	
	return &TFIDFEmbedder{
		model:         config.Model,
		dimension:     dimension,
		batchSize:     config.BatchSize,
		cache:         make(map[string]*CacheEntry),
		maxCacheSize:  config.CacheSize,
		stats: EmbedderStats{
			LastEmbeddingTime: time.Now(),
		},
		corpus:        NewCustomCorpus(),
		vocabulary:    make(map[string]int),
		idfValues:     make(map[string]float64),
		tokenizer:     tokenizer,
		stopWords:     stopWords,
		minWordLength: 2,
	}
}

// Embed implements the Embedder interface for batch processing
func (e *TFIDFEmbedder) Embed(ctx context.Context, chunks []types.CodeChunk) ([]types.Embedding, error) {
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
func (e *TFIDFEmbedder) EmbedSingle(ctx context.Context, chunk types.CodeChunk) (types.Embedding, error) {
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
func (e *TFIDFEmbedder) GetDimension() int {
	return e.dimension
}

// GetModel implements the Embedder interface
func (e *TFIDFEmbedder) GetModel() string {
	return e.model + " (TF-IDF)"
}

// Warmup implements the Embedder interface
// For TF-IDF, this builds the initial corpus and vocabulary
func (e *TFIDFEmbedder) Warmup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isWarmedUp {
		return nil
	}

	// TF-IDF doesn't require external model loading, just mark as warmed up
	e.isWarmedUp = true
	return nil
}

// Stats implements the Embedder interface
func (e *TFIDFEmbedder) Stats() EmbedderStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	return e.stats
}

// Close implements the Embedder interface
func (e *TFIDFEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cache = make(map[string]*CacheEntry)
	e.corpus = NewCustomCorpus()
	e.vocabulary = make(map[string]int)
	e.idfValues = make(map[string]float64)
	e.isWarmedUp = false
	return nil
}

// Private methods

func (e *TFIDFEmbedder) embedBatch(ctx context.Context, chunks []types.CodeChunk) ([]types.Embedding, error) {
	var embeddings []types.Embedding
	
	// First pass: collect all documents for corpus building if needed
	documents := make([]string, len(chunks))
	for i, chunk := range chunks {
		documents[i] = e.preprocessText(chunk)
	}
	
	// Update corpus with new documents
	e.updateCorpus(documents)
	
	// Second pass: generate embeddings
	for i, chunk := range chunks {
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
		vector := e.generateTFIDFEmbedding(documents[i])
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

// preprocessText combines chunk content and metadata for better embeddings
func (e *TFIDFEmbedder) preprocessText(chunk types.CodeChunk) string {
	// Combine file name, language, and content for richer context
	var parts []string
	
	// Add file extension as context
	if chunk.Language != "" {
		parts = append(parts, chunk.Language)
	}
	
	// Add filename without path for context
	if chunk.FilePath != "" {
		fileName := chunk.FilePath
		if lastSlash := strings.LastIndex(fileName, "/"); lastSlash != -1 {
			fileName = fileName[lastSlash+1:]
		}
		parts = append(parts, fileName)
	}
	
	// Add the actual content
	parts = append(parts, chunk.Text)
	
	return strings.Join(parts, " ")
}

// tokenizeText extracts meaningful tokens from text
func (e *TFIDFEmbedder) tokenizeText(text string) []string {
	// Convert to lowercase for consistency
	text = strings.ToLower(text)
	
	// Extract tokens using regex
	rawTokens := e.tokenizer.FindAllString(text, -1)
	
	var tokens []string
	for _, token := range rawTokens {
		// Skip stop words and very short tokens
		if len(token) >= e.minWordLength && !e.stopWords[token] {
			tokens = append(tokens, token)
		}
	}
	
	return tokens
}

// updateCorpus adds new documents to the corpus and updates vocabulary
func (e *TFIDFEmbedder) updateCorpus(documents []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	// Add documents to corpus
	for _, doc := range documents {
		tokens := e.tokenizeText(doc)
		e.corpus.AddDocument(tokens)
		
		// Update vocabulary
		for _, token := range tokens {
			if _, exists := e.vocabulary[token]; !exists {
				e.vocabulary[token] = len(e.vocabulary)
			}
		}
	}
	
	// Rebuild IDF values
	e.computeIDF()
}

// computeIDF calculates IDF values for all terms in vocabulary
func (e *TFIDFEmbedder) computeIDF() {
	totalDocs := float64(e.corpus.TotalDocuments())
	if totalDocs == 0 {
		return
	}
	
	for term := range e.vocabulary {
		docFreq := float64(e.corpus.DocumentFrequency(term))
		if docFreq > 0 {
			e.idfValues[term] = math.Log(totalDocs / docFreq)
		}
	}
}

// generateTFIDFEmbedding creates a TF-IDF vector for the given text
func (e *TFIDFEmbedder) generateTFIDFEmbedding(text string) []float32 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	tokens := e.tokenizeText(text)
	
	// Calculate term frequencies
	termFreq := make(map[string]int)
	for _, token := range tokens {
		termFreq[token]++
	}
	
	// Create dense vector of fixed dimension
	vector := make([]float32, e.dimension)
	
	// If we have a small vocabulary, use all terms
	if len(e.vocabulary) <= e.dimension {
		// Direct mapping for small vocabularies
		for term, freq := range termFreq {
			if index, exists := e.vocabulary[term]; exists && index < e.dimension {
				tf := float64(freq) / float64(len(tokens))
				idf := e.idfValues[term]
				// If IDF is 0 (single document corpus), use a default value
				if idf == 0 {
					idf = 1.0 // Default IDF for single document scenarios
				}
				vector[index] = float32(tf * idf)
			}
		}
	} else {
		// For larger vocabularies, use feature hashing to map to fixed dimensions
		for term, freq := range termFreq {
			if _, exists := e.vocabulary[term]; exists {
				// Use simple hash function to map term to dimension
				hash := e.hashTerm(term) % e.dimension
				tf := float64(freq) / float64(len(tokens))
				idf := e.idfValues[term]
				// If IDF is 0 (single document corpus), use a default value
				if idf == 0 {
					idf = 1.0 // Default IDF for single document scenarios
				}
				vector[hash] += float32(tf * idf)
			}
		}
	}
	
	// If the vector is still all zeros (no vocabulary match), create a fallback vector
	allZero := true
	for _, v := range vector {
		if v != 0 {
			allZero = false
			break
		}
	}
	
	if allZero && len(tokens) > 0 {
		// Create a simple fallback vector based on the text content
		e.generateFallbackVector(text, vector)
	}
	
	// Normalize the vector to unit length
	return e.normalizeVector(vector)
}

// hashTerm provides a simple hash function for terms
func (e *TFIDFEmbedder) hashTerm(term string) int {
	hash := 0
	for _, char := range term {
		hash = hash*31 + int(char)
	}
	if hash < 0 {
		hash = -hash
	}
	return hash
}

// generateFallbackVector creates a simple fallback vector when TF-IDF produces all zeros
func (e *TFIDFEmbedder) generateFallbackVector(text string, vector []float32) {
	// Create a simple hash-based vector for the text content
	// This ensures we always have a non-zero vector that can be normalized
	textBytes := []byte(strings.ToLower(text))
	
	for i := 0; i < len(vector); i++ {
		// Use position-based hashing to create diverse values
		hash := 1
		for j, b := range textBytes {
			hash = (hash*31 + int(b) + i + j) % 1000000
		}
		
		// Convert to a value in range [-1, 1]
		value := float32(hash%2000-1000) / 1000.0
		
		// Add some structure based on position
		positionFactor := float32(math.Sin(float64(i) * 0.1))
		vector[i] = value + positionFactor*0.3
	}
}

// normalizeVector performs L2 normalization
func (e *TFIDFEmbedder) normalizeVector(vector []float32) []float32 {
	var norm float32
	for _, v := range vector {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	
	if norm == 0 {
		// If norm is still 0, create a minimal fallback vector
		// This should never happen with the fallback vector generation above,
		// but it's a safety net
		vector[0] = 1.0
		norm = 1.0
	}
	
	normalized := make([]float32, len(vector))
	for i, v := range vector {
		normalized[i] = v / norm
	}
	
	return normalized
}

// getCachedEmbedding retrieves a cached embedding
func (e *TFIDFEmbedder) getCachedEmbedding(hash string) ([]float32, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if entry, exists := e.cache[hash]; exists {
		entry.AccessCount++
		entry.Timestamp = time.Now()
		return entry.Embedding, true
	}
	
	return nil, false
}

// cacheEmbedding stores an embedding in cache
func (e *TFIDFEmbedder) cacheEmbedding(hash string, embedding []float32) {
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

// evictLRU removes the least recently used cache entry
func (e *TFIDFEmbedder) evictLRU() {
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

// updateStats updates the embedder statistics
func (e *TFIDFEmbedder) updateStats(embeddingCount int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.stats.TotalEmbeddings += embeddingCount
	e.stats.TotalChunks += embeddingCount
	e.stats.LastEmbeddingTime = time.Now()
}

// GetTopTerms returns the most important terms for a document (useful for debugging)
func (e *TFIDFEmbedder) GetTopTerms(text string, topK int) []struct {
	Term  string
	Score float64
} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	tokens := e.tokenizeText(text)
	termFreq := make(map[string]int)
	for _, token := range tokens {
		termFreq[token]++
	}
	
	type termScore struct {
		Term  string
		Score float64
	}
	
	var scores []termScore
	for term, freq := range termFreq {
		if idf, exists := e.idfValues[term]; exists {
			tf := float64(freq) / float64(len(tokens))
			score := tf * idf
			scores = append(scores, termScore{Term: term, Score: score})
		}
	}
	
	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Score > scores[j].Score
	})
	
	// Return top K terms
	if topK > len(scores) {
		topK = len(scores)
	}
	
	result := make([]struct {
		Term  string
		Score float64
	}, topK)
	
	for i := 0; i < topK; i++ {
		result[i] = struct {
			Term  string
			Score float64
		}{Term: scores[i].Term, Score: scores[i].Score}
	}
	
	return result
}

// Custom Corpus Implementation

// NewCustomCorpus creates a new empty corpus
func NewCustomCorpus() *CustomCorpus {
	return &CustomCorpus{
		documents:    make([]map[string]int, 0),
		termDocCount: make(map[string]int),
		totalDocs:    0,
	}
}

// AddDocument adds a document to the corpus
func (c *CustomCorpus) AddDocument(tokens []string) {
	// Create term frequency map for this document
	termFreq := make(map[string]int)
	for _, token := range tokens {
		termFreq[token]++
	}
	
	// Add to documents
	c.documents = append(c.documents, termFreq)
	c.totalDocs++
	
	// Update document frequency for each unique term
	for term := range termFreq {
		c.termDocCount[term]++
	}
}

// TotalDocuments returns the total number of documents in the corpus
func (c *CustomCorpus) TotalDocuments() int {
	return c.totalDocs
}

// DocumentFrequency returns the number of documents containing the given term
func (c *CustomCorpus) DocumentFrequency(term string) int {
	return c.termDocCount[term]
}
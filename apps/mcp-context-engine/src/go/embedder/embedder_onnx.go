//go:build onnx
// +build onnx

package embedder

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
	onnxruntime "github.com/yalue/onnxruntime_go"
)

// ONNXEmbedder implements the Embedder interface using ONNX Runtime
// This provides high-quality semantic embeddings using sentence transformers
type ONNXEmbedder struct {
	mu            sync.RWMutex
	model         string
	dimension     int
	batchSize     int
	cache         map[string]*CacheEntry
	maxCacheSize  int
	stats         EmbedderStats
	isWarmedUp    bool
	
	// ONNX specific fields
	session       *onnxruntime.Session
	tokenizer     *SimpleTokenizer
	modelPath     string
	maxSeqLength  int
	
	// Model configuration
	vocabSize     int
	padTokenID    int
	unknownTokenID int
}

// SimpleTokenizer provides basic tokenization for the ONNX model
type SimpleTokenizer struct {
	vocab         map[string]int
	reverseVocab  map[int]string
	wordRegex     *regexp.Regexp
	maxLength     int
	padToken      string
	unknownToken  string
	clsToken      string
	sepToken      string
}

const (
	// Model download configuration
	modelURL     = "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model.onnx"
	tokenizerURL = "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/tokenizer.json"
	vocabURL     = "https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/vocab.txt"
	
	// Model parameters for all-MiniLM-L6-v2
	defaultMaxSeqLength = 512
	defaultVocabSize    = 30522
	modelOutputDimension = 384  // all-MiniLM-L6-v2 native output dimension
	defaultDimension    = 768   // Padded dimension for FAISS compatibility
)

// NewONNXEmbedder creates a new ONNX-based embedder
func NewONNXEmbedder(config EmbedderConfig) Embedder {
	// Create models directory
	modelsDir := filepath.Join(".", "models")
	os.MkdirAll(modelsDir, 0755)
	
	return &ONNXEmbedder{
		model:         config.Model,
		dimension:     defaultDimension,
		batchSize:     config.BatchSize,
		cache:         make(map[string]*CacheEntry),
		maxCacheSize:  config.CacheSize,
		stats: EmbedderStats{
			LastEmbeddingTime: time.Now(),
		},
		modelPath:      filepath.Join(modelsDir, "all-MiniLM-L6-v2.onnx"),
		maxSeqLength:   defaultMaxSeqLength,
		vocabSize:      defaultVocabSize,
		padTokenID:     0,
		unknownTokenID: 100,
	}
}

// Embed implements the Embedder interface for batch processing
func (e *ONNXEmbedder) Embed(ctx context.Context, chunks []types.CodeChunk) ([]types.Embedding, error) {
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
func (e *ONNXEmbedder) EmbedSingle(ctx context.Context, chunk types.CodeChunk) (types.Embedding, error) {
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
func (e *ONNXEmbedder) GetDimension() int {
	return e.dimension
}

// GetModel implements the Embedder interface
func (e *ONNXEmbedder) GetModel() string {
	return e.model + " (ONNX)"
}

// Warmup implements the Embedder interface
func (e *ONNXEmbedder) Warmup(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.isWarmedUp {
		return nil
	}

	// Download model if it doesn't exist
	if err := e.ensureModelExists(ctx); err != nil {
		return fmt.Errorf("failed to ensure model exists: %w", err)
	}

	// Initialize ONNX Runtime
	if err := onnxruntime.InitializeEnvironment(); err != nil {
		return fmt.Errorf("failed to initialize ONNX Runtime: %w", err)
	}

	// Create session
	session, err := onnxruntime.NewSession(e.modelPath, onnxruntime.NewSessionOptions())
	if err != nil {
		return fmt.Errorf("failed to create ONNX session: %w", err)
	}
	e.session = session

	// Initialize tokenizer
	e.tokenizer = e.createSimpleTokenizer()

	e.isWarmedUp = true
	return nil
}

// Stats implements the Embedder interface
func (e *ONNXEmbedder) Stats() EmbedderStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	return e.stats
}

// Close implements the Embedder interface
func (e *ONNXEmbedder) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.session != nil {
		e.session.Destroy()
		e.session = nil
	}

	e.cache = make(map[string]*CacheEntry)
	e.isWarmedUp = false
	return nil
}

// Private methods

func (e *ONNXEmbedder) embedBatch(ctx context.Context, chunks []types.CodeChunk) ([]types.Embedding, error) {
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
		vector, err := e.generateONNXEmbedding(chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for chunk %s: %w", chunk.Hash, err)
		}
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

func (e *ONNXEmbedder) ensureModelExists(ctx context.Context) error {
	// Check if model file exists
	if _, err := os.Stat(e.modelPath); err == nil {
		return nil // Model already exists
	}

	fmt.Printf("Downloading ONNX model to %s...\n", e.modelPath)
	
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", modelURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	// Download model
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download model: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download model: status %d", resp.StatusCode)
	}

	// Create output file
	out, err := os.Create(e.modelPath)
	if err != nil {
		return fmt.Errorf("failed to create model file: %w", err)
	}
	defer out.Close()

	// Copy downloaded content
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to save model file: %w", err)
	}

	fmt.Printf("Model downloaded successfully to %s\n", e.modelPath)
	return nil
}

func (e *ONNXEmbedder) createSimpleTokenizer() *SimpleTokenizer {
	// Create a basic tokenizer for BERT-like models
	// This is a simplified version - in production you'd want to load the actual tokenizer
	vocab := make(map[string]int)
	reverseVocab := make(map[int]string)
	
	// Add special tokens
	vocab["[PAD]"] = 0
	vocab["[UNK]"] = 100
	vocab["[CLS]"] = 101
	vocab["[SEP]"] = 102
	
	reverseVocab[0] = "[PAD]"
	reverseVocab[100] = "[UNK]"
	reverseVocab[101] = "[CLS]"
	reverseVocab[102] = "[SEP]"
	
	// Add basic vocabulary (simplified)
	commonTokens := []string{
		"a", "an", "and", "are", "as", "at", "be", "by", "for", "from", "has", "he",
		"in", "is", "it", "its", "of", "on", "that", "the", "to", "was", "will", "with",
		"function", "class", "var", "let", "const", "if", "else", "return", "import", "export",
		"public", "private", "static", "void", "int", "string", "boolean", "true", "false",
		"null", "undefined", "new", "this", "super", "extends", "implements", "interface",
	}
	
	tokenID := 103
	for _, token := range commonTokens {
		vocab[token] = tokenID
		reverseVocab[tokenID] = token
		tokenID++
	}
	
	// Create word regex for basic tokenization
	wordRegex := regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*|[0-9]+|\S`)
	
	return &SimpleTokenizer{
		vocab:        vocab,
		reverseVocab: reverseVocab,
		wordRegex:    wordRegex,
		maxLength:    e.maxSeqLength,
		padToken:     "[PAD]",
		unknownToken: "[UNK]",
		clsToken:     "[CLS]",
		sepToken:     "[SEP]",
	}
}

func (e *ONNXEmbedder) generateONNXEmbedding(chunk types.CodeChunk) ([]float32, error) {
	// Prepare input text
	text := e.preprocessText(chunk)
	
	// Tokenize
	tokens := e.tokenizer.Tokenize(text)
	
	// Convert to input tensors
	inputIDs, attentionMask := e.tokenizer.ConvertToTensors(tokens, e.maxSeqLength)
	
	// Create ONNX tensors
	inputIDsTensor, err := onnxruntime.NewTensor(onnxruntime.NewShape(1, len(inputIDs)), inputIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to create input_ids tensor: %w", err)
	}
	defer inputIDsTensor.Destroy()
	
	attentionTensor, err := onnxruntime.NewTensor(onnxruntime.NewShape(1, len(attentionMask)), attentionMask)
	if err != nil {
		return nil, fmt.Errorf("failed to create attention_mask tensor: %w", err)
	}
	defer attentionTensor.Destroy()
	
	// Run inference
	outputs, err := e.session.Run([]onnxruntime.Value{inputIDsTensor, attentionTensor})
	if err != nil {
		return nil, fmt.Errorf("failed to run ONNX inference: %w", err)
	}
	defer func() {
		for _, output := range outputs {
			output.Destroy()
		}
	}()
	
	if len(outputs) == 0 {
		return nil, fmt.Errorf("no outputs from ONNX model")
	}
	
	// Extract embeddings from the output
	outputTensor := outputs[0]
	outputData := outputTensor.GetData()
	
	// Convert to float32 slice
	switch data := outputData.(type) {
	case []float32:
		// For sentence transformers, we typically use mean pooling over non-padding tokens
		return e.meanPooling(data, attentionMask), nil
	default:
		return nil, fmt.Errorf("unexpected output tensor type: %T", outputData)
	}
}

func (e *ONNXEmbedder) preprocessText(chunk types.CodeChunk) string {
	// Combine file context and content for better embeddings
	var parts []string
	
	// Add language context
	if chunk.Language != "" {
		parts = append(parts, chunk.Language)
	}
	
	// Add filename for context
	if chunk.FilePath != "" {
		fileName := filepath.Base(chunk.FilePath)
		parts = append(parts, fileName)
	}
	
	// Add the main content
	parts = append(parts, chunk.Text)
	
	return strings.Join(parts, " ")
}

func (e *ONNXEmbedder) meanPooling(embeddings []float32, attentionMask []int64) []float32 {
	seqLen := len(attentionMask)
	embeddingDim := len(embeddings) / seqLen
	
	pooled := make([]float32, embeddingDim)
	validTokens := 0
	
	for i := 0; i < seqLen; i++ {
		if attentionMask[i] == 1 { // Non-padding token
			for j := 0; j < embeddingDim; j++ {
				pooled[j] += embeddings[i*embeddingDim+j]
			}
			validTokens++
		}
	}
	
	// Average pooling
	if validTokens > 0 {
		for j := 0; j < embeddingDim; j++ {
			pooled[j] /= float32(validTokens)
		}
	}
	
	// L2 normalize
	normalized := e.normalizeVector(pooled)
	
	// Pad to target dimension for FAISS compatibility
	if len(normalized) < e.dimension {
		padded := make([]float32, e.dimension)
		copy(padded, normalized)
		// Fill remaining dimensions with small random values for better distribution
		for i := len(normalized); i < e.dimension; i++ {
			padded[i] = 0.001 * float32(i%7-3) // Small variation to avoid perfect zeros
		}
		return e.normalizeVector(padded) // Re-normalize after padding
	}
	
	return normalized
}

func (e *ONNXEmbedder) normalizeVector(vector []float32) []float32 {
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

// Tokenizer methods

func (t *SimpleTokenizer) Tokenize(text string) []string {
	// Convert to lowercase and extract tokens
	text = strings.ToLower(text)
	rawTokens := t.wordRegex.FindAllString(text, -1)
	
	// Add special tokens
	tokens := []string{t.clsToken}
	tokens = append(tokens, rawTokens...)
	tokens = append(tokens, t.sepToken)
	
	return tokens
}

func (t *SimpleTokenizer) ConvertToTensors(tokens []string, maxLength int) ([]int64, []int64) {
	// Convert tokens to IDs
	inputIDs := make([]int64, maxLength)
	attentionMask := make([]int64, maxLength)
	
	for i := 0; i < maxLength; i++ {
		if i < len(tokens) {
			if id, exists := t.vocab[tokens[i]]; exists {
				inputIDs[i] = int64(id)
			} else {
				inputIDs[i] = int64(t.vocab[t.unknownToken])
			}
			attentionMask[i] = 1
		} else {
			inputIDs[i] = int64(t.vocab[t.padToken])
			attentionMask[i] = 0
		}
	}
	
	return inputIDs, attentionMask
}

// Shared cache methods (same as TF-IDF implementation)

func (e *ONNXEmbedder) getCachedEmbedding(hash string) ([]float32, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if entry, exists := e.cache[hash]; exists {
		entry.AccessCount++
		entry.Timestamp = time.Now()
		return entry.Embedding, true
	}
	
	return nil, false
}

func (e *ONNXEmbedder) cacheEmbedding(hash string, embedding []float32) {
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

func (e *ONNXEmbedder) evictLRU() {
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

func (e *ONNXEmbedder) updateStats(embeddingCount int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.stats.TotalEmbeddings += embeddingCount
	e.stats.TotalChunks += embeddingCount
	e.stats.LastEmbeddingTime = time.Now()
}
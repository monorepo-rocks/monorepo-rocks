package embedder

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"../types"
)

func TestStubEmbedder_EmbedSingle(t *testing.T) {
	embedder := NewDefaultEmbedder()
	defer embedder.Close()

	chunk := types.CodeChunk{
		FileID:    "test-file",
		FilePath:  "/test/file.go",
		StartByte: 0,
		EndByte:   100,
		StartLine: 1,
		EndLine:   5,
		Text:      "func main() {\n    fmt.Println(\"Hello, World!\")\n}",
		Language:  "go",
		Hash:      "test-hash",
	}

	ctx := context.Background()
	embedding, err := embedder.EmbedSingle(ctx, chunk)
	if err != nil {
		t.Fatalf("Failed to embed single chunk: %v", err)
	}

	if embedding.ChunkID == "" {
		t.Error("Expected non-empty chunk ID")
	}
	if len(embedding.Vector) != 768 {
		t.Errorf("Expected vector dimension 768, got %d", len(embedding.Vector))
	}

	// Verify vector is normalized (approximately unit length)
	var norm float32
	for _, v := range embedding.Vector {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))
	if norm < 0.99 || norm > 1.01 {
		t.Errorf("Expected normalized vector (length ~1.0), got length %f", norm)
	}
}

func TestStubEmbedder_Embed(t *testing.T) {
	embedder := NewDefaultEmbedder()
	defer embedder.Close()

	chunks := []types.CodeChunk{
		{
			FileID:    "test-file-1",
			FilePath:  "/test/file1.go",
			Text:      "package main",
			Language:  "go",
			Hash:      "hash-1",
		},
		{
			FileID:    "test-file-2",
			FilePath:  "/test/file2.py",
			Text:      "def hello():\n    print('Hello')",
			Language:  "python",
			Hash:      "hash-2",
		},
		{
			FileID:    "test-file-3",
			FilePath:  "/test/file3.js",
			Text:      "function greet() { console.log('Hi'); }",
			Language:  "javascript",
			Hash:      "hash-3",
		},
	}

	ctx := context.Background()
	embeddings, err := embedder.Embed(ctx, chunks)
	if err != nil {
		t.Fatalf("Failed to embed chunks: %v", err)
	}

	if len(embeddings) != len(chunks) {
		t.Errorf("Expected %d embeddings, got %d", len(chunks), len(embeddings))
	}

	for i, embedding := range embeddings {
		if embedding.ChunkID == "" {
			t.Errorf("Embedding %d has empty chunk ID", i)
		}
		if len(embedding.Vector) != 768 {
			t.Errorf("Embedding %d has dimension %d, expected 768", i, len(embedding.Vector))
		}
	}
}

func TestStubEmbedder_Cache(t *testing.T) {
	embedder := NewDefaultEmbedder()
	defer embedder.Close()

	chunk := types.CodeChunk{
		FileID:   "test-file",
		FilePath: "/test/file.go",
		Text:     "func test() {}",
		Language: "go",
		Hash:     "test-hash-cache",
	}

	ctx := context.Background()

	// First embedding - should be cache miss
	embedding1, err := embedder.EmbedSingle(ctx, chunk)
	if err != nil {
		t.Fatalf("Failed to embed chunk (first time): %v", err)
	}

	stats1 := embedder.Stats()
	if stats1.CacheMisses == 0 {
		t.Error("Expected cache miss for first embedding")
	}

	// Second embedding - should be cache hit
	embedding2, err := embedder.EmbedSingle(ctx, chunk)
	if err != nil {
		t.Fatalf("Failed to embed chunk (second time): %v", err)
	}

	stats2 := embedder.Stats()
	if stats2.CacheHits <= stats1.CacheHits {
		t.Error("Expected cache hit for second embedding")
	}

	// Verify embeddings are identical
	if len(embedding1.Vector) != len(embedding2.Vector) {
		t.Error("Cached embeddings have different dimensions")
	}
	for i := range embedding1.Vector {
		if embedding1.Vector[i] != embedding2.Vector[i] {
			t.Error("Cached embeddings should be identical")
			break
		}
	}
}

func TestStubEmbedder_Warmup(t *testing.T) {
	config := EmbedderConfig{
		Model:     "test-model",
		BatchSize: 16,
		CacheSize: 1000,
	}
	embedder := NewStubEmbedder(config)
	defer embedder.Close()

	ctx := context.Background()

	// Test warmup
	err := embedder.Warmup(ctx)
	if err != nil {
		t.Fatalf("Failed to warmup embedder: %v", err)
	}

	// Test embedding after warmup
	chunk := types.CodeChunk{
		FileID:   "test",
		Text:     "test code",
		Language: "go",
		Hash:     "test-hash",
	}

	_, err = embedder.EmbedSingle(ctx, chunk)
	if err != nil {
		t.Fatalf("Failed to embed after warmup: %v", err)
	}
}

func TestStubEmbedder_Stats(t *testing.T) {
	embedder := NewDefaultEmbedder()
	defer embedder.Close()

	// Check initial stats
	stats := embedder.Stats()
	if stats.TotalEmbeddings != 0 {
		t.Errorf("Expected 0 total embeddings initially, got %d", stats.TotalEmbeddings)
	}
	if stats.TotalChunks != 0 {
		t.Errorf("Expected 0 total chunks initially, got %d", stats.TotalChunks)
	}

	// Generate some embeddings
	chunks := []types.CodeChunk{
		{FileID: "1", Text: "code1", Language: "go", Hash: "hash1"},
		{FileID: "2", Text: "code2", Language: "py", Hash: "hash2"},
	}

	ctx := context.Background()
	_, err := embedder.Embed(ctx, chunks)
	if err != nil {
		t.Fatalf("Failed to embed chunks: %v", err)
	}

	// Check updated stats
	stats = embedder.Stats()
	if stats.TotalEmbeddings != 2 {
		t.Errorf("Expected 2 total embeddings, got %d", stats.TotalEmbeddings)
	}
	if stats.TotalChunks != 2 {
		t.Errorf("Expected 2 total chunks, got %d", stats.TotalChunks)
	}
	if stats.AverageLatency <= 0 {
		t.Error("Expected positive average latency")
	}
}

func TestStubEmbedder_GetDimensionAndModel(t *testing.T) {
	config := EmbedderConfig{
		Model:     "test-model",
		BatchSize: 32,
	}
	embedder := NewStubEmbedder(config)
	defer embedder.Close()

	if embedder.GetDimension() != 768 {
		t.Errorf("Expected dimension 768, got %d", embedder.GetDimension())
	}

	if embedder.GetModel() != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", embedder.GetModel())
	}
}

func TestStubEmbedder_DeterministicEmbeddings(t *testing.T) {
	embedder := NewDefaultEmbedder()
	defer embedder.Close()

	chunk := types.CodeChunk{
		FileID:   "test-file",
		FilePath: "/test/file.go",
		Text:     "func deterministic() { return 42; }",
		Language: "go",
		Hash:     "deterministic-hash",
	}

	ctx := context.Background()

	// Generate embedding twice
	embedding1, err := embedder.EmbedSingle(ctx, chunk)
	if err != nil {
		t.Fatalf("Failed to generate first embedding: %v", err)
	}

	// Clear cache to force regeneration
	embedder.Close()
	embedder = NewDefaultEmbedder()
	defer embedder.Close()

	embedding2, err := embedder.EmbedSingle(ctx, chunk)
	if err != nil {
		t.Fatalf("Failed to generate second embedding: %v", err)
	}

	// Embeddings should be identical (deterministic generation)
	if len(embedding1.Vector) != len(embedding2.Vector) {
		t.Error("Deterministic embeddings have different dimensions")
		return
	}

	for i := range embedding1.Vector {
		if embedding1.Vector[i] != embedding2.Vector[i] {
			t.Error("Embeddings should be deterministic (identical for same input)")
			break
		}
	}
}

func TestComputeChunkHash(t *testing.T) {
	chunk1 := types.CodeChunk{
		FilePath: "/test/file.go",
		Text:     "func test() {}",
		Language: "go",
	}

	chunk2 := types.CodeChunk{
		FilePath: "/test/file.go",
		Text:     "func test() {}",
		Language: "go",
	}

	chunk3 := types.CodeChunk{
		FilePath: "/test/file.go",
		Text:     "func different() {}",
		Language: "go",
	}

	hash1 := ComputeChunkHash(chunk1)
	hash2 := ComputeChunkHash(chunk2)
	hash3 := ComputeChunkHash(chunk3)

	// Same chunks should have same hash
	if hash1 != hash2 {
		t.Error("Identical chunks should have identical hashes")
	}

	// Different chunks should have different hashes
	if hash1 == hash3 {
		t.Error("Different chunks should have different hashes")
	}

	// Hash should be non-empty
	if hash1 == "" {
		t.Error("Hash should not be empty")
	}
}

func TestCreateCodeChunk(t *testing.T) {
	chunk := CreateCodeChunk(
		"file-id",
		"/path/to/file.go",
		"func main() {}",
		"go",
		0, 14, 1, 1,
	)

	if chunk.FileID != "file-id" {
		t.Errorf("Expected FileID 'file-id', got '%s'", chunk.FileID)
	}
	if chunk.FilePath != "/path/to/file.go" {
		t.Errorf("Expected FilePath '/path/to/file.go', got '%s'", chunk.FilePath)
	}
	if chunk.Text != "func main() {}" {
		t.Errorf("Expected Text 'func main() {}', got '%s'", chunk.Text)
	}
	if chunk.Language != "go" {
		t.Errorf("Expected Language 'go', got '%s'", chunk.Language)
	}
	if chunk.Hash == "" {
		t.Error("Expected non-empty hash")
	}
}

func TestChunkCode(t *testing.T) {
	content := `package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}

func helper() {
    fmt.Println("Helper function")
}`

	chunks := ChunkCode("file-1", "/test/main.go", content, "go", 50)

	if len(chunks) == 0 {
		t.Error("Expected at least one chunk")
		return
	}

	// Verify all chunks have proper metadata
	for i, chunk := range chunks {
		if chunk.FileID != "file-1" {
			t.Errorf("Chunk %d has wrong FileID", i)
		}
		if chunk.FilePath != "/test/main.go" {
			t.Errorf("Chunk %d has wrong FilePath", i)
		}
		if chunk.Language != "go" {
			t.Errorf("Chunk %d has wrong Language", i)
		}
		if chunk.Text == "" {
			t.Errorf("Chunk %d has empty text", i)
		}
		if chunk.Hash == "" {
			t.Errorf("Chunk %d has empty hash", i)
		}
		if chunk.StartLine <= 0 {
			t.Errorf("Chunk %d has invalid StartLine: %d", i, chunk.StartLine)
		}
	}

	// Verify chunks don't overlap (in terms of bytes)
	for i := 1; i < len(chunks); i++ {
		if chunks[i].StartByte <= chunks[i-1].EndByte {
			t.Errorf("Chunks %d and %d overlap", i-1, i)
		}
	}
}

func TestChunkCodeEmptyContent(t *testing.T) {
	chunks := ChunkCode("file-1", "/test/empty.txt", "", "text", 100)
	if len(chunks) != 0 {
		t.Errorf("Expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestChunkCodeSmallContent(t *testing.T) {
	content := "small code"
	chunks := ChunkCode("file-1", "/test/small.txt", content, "text", 100)
	
	if len(chunks) != 1 {
		t.Errorf("Expected 1 chunk for small content, got %d", len(chunks))
		return
	}

	if chunks[0].Text != content {
		t.Errorf("Expected chunk text to be '%s', got '%s'", content, chunks[0].Text)
	}
}

func TestSplitIntoLines(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"single line", []string{"single line"}},
		{"line1\nline2", []string{"line1", "line2"}},
		{"line1\nline2\n", []string{"line1", "line2", ""}},
		{"a\nb\nc\n", []string{"a", "b", "c", ""}},
	}

	for _, test := range tests {
		result := splitIntoLines(test.input)
		if len(result) != len(test.expected) {
			t.Errorf("splitIntoLines(%q): expected %d lines, got %d",
				test.input, len(test.expected), len(result))
			continue
		}

		for i, line := range result {
			if line != test.expected[i] {
				t.Errorf("splitIntoLines(%q): line %d expected '%s', got '%s'",
					test.input, i, test.expected[i], line)
			}
		}
	}
}

func TestStubEmbedder_BatchProcessing(t *testing.T) {
	config := EmbedderConfig{
		BatchSize: 2, // Small batch size for testing
		CacheSize: 100,
	}
	embedder := NewStubEmbedder(config)
	defer embedder.Close()

	// Create more chunks than batch size
	var chunks []types.CodeChunk
	for i := 0; i < 5; i++ {
		chunks = append(chunks, types.CodeChunk{
			FileID:   fmt.Sprintf("file-%d", i),
			Text:     fmt.Sprintf("code content %d", i),
			Language: "go",
			Hash:     fmt.Sprintf("hash-%d", i),
		})
	}

	ctx := context.Background()
	embeddings, err := embedder.Embed(ctx, chunks)
	if err != nil {
		t.Fatalf("Failed to embed chunks in batches: %v", err)
	}

	if len(embeddings) != len(chunks) {
		t.Errorf("Expected %d embeddings, got %d", len(chunks), len(embeddings))
	}

	// Verify all embeddings are valid
	for i, embedding := range embeddings {
		if len(embedding.Vector) != 768 {
			t.Errorf("Embedding %d has wrong dimension", i)
		}
		if embedding.ChunkID == "" {
			t.Errorf("Embedding %d has empty chunk ID", i)
		}
	}
}
package query

import (
	"context"
	"testing"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/config"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

func createTestQueryService() *QueryService {
	// Create test configuration
	cfg := config.DefaultConfig()

	// Create test indexers
	zoektIndexer := indexer.NewZoektIndexer("/tmp/test-zoekt")
	faissIndexer := indexer.NewFAISSIndexer("/tmp/test-faiss", 768)
	testEmbedder := embedder.NewDefaultEmbedder()

	return NewQueryService(zoektIndexer, faissIndexer, testEmbedder, cfg)
}

func setupTestData(qs *QueryService) error {
	ctx := context.Background()

	// Add some test files to lexical index
	testFiles := []string{
		"/test/main.go",
		"/test/utils.py",
		"/test/handler.js",
	}
	if err := qs.zoektIndexer.Index(ctx, testFiles); err != nil {
		return err
	}

	// Add some test vectors to semantic index
	testEmbeddings := []types.Embedding{
		{
			ChunkID: "/test/main.go:0-100",
			Vector:  indexer.GenerateRandomVector(768, 123),
		},
		{
			ChunkID: "/test/utils.py:0-200",
			Vector:  indexer.GenerateRandomVector(768, 456),
		},
		{
			ChunkID: "/test/handler.js:0-150",
			Vector:  indexer.GenerateRandomVector(768, 789),
		},
	}
	if err := qs.faissIndexer.AddVectors(ctx, testEmbeddings); err != nil {
		return err
	}

	return nil
}

func TestQueryService_Search(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	// Setup test data
	if err := setupTestData(qs); err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}

	request := &types.SearchRequest{
		Query: "main function implementation",
		TopK:  10,
	}

	ctx := context.Background()
	response, err := qs.Search(ctx, request)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if response == nil {
		t.Fatal("Expected non-nil response")
	}

	if response.QueryTime <= 0 {
		t.Error("Expected positive query time")
	}

	if response.TotalHits < 0 {
		t.Error("Expected non-negative total hits")
	}

	// Verify response structure
	if len(response.Hits) != response.TotalHits {
		t.Errorf("Hits length (%d) doesn't match TotalHits (%d)",
			len(response.Hits), response.TotalHits)
	}
}

func TestQueryService_SearchEmptyQuery(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	request := &types.SearchRequest{
		Query: "",
		TopK:  10,
	}

	ctx := context.Background()
	_, err := qs.Search(ctx, request)
	if err == nil {
		t.Error("Expected error for empty query")
	}
}

func TestQueryService_SearchWithFilters(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	if err := setupTestData(qs); err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}

	request := &types.SearchRequest{
		Query:    "function",
		TopK:     10,
		Language: "go",
		Filters: struct {
			FilePatterns []string `json:"file_patterns,omitempty"`
			Repos        []string `json:"repos,omitempty"`
		}{
			FilePatterns: []string{"*.go"},
		},
	}

	ctx := context.Background()
	response, err := qs.Search(ctx, request)
	if err != nil {
		t.Fatalf("Search with filters failed: %v", err)
	}

	// Verify filters were applied (hits should only contain Go files)
	for _, hit := range response.Hits {
		if hit.Language != "" && hit.Language != "go" {
			t.Errorf("Expected Go language filter, got hit with language: %s", hit.Language)
		}
	}
}

func TestQueryService_GetIndexStatus(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	if err := setupTestData(qs); err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}

	ctx := context.Background()
	status, err := qs.GetIndexStatus(ctx)
	if err != nil {
		t.Fatalf("GetIndexStatus failed: %v", err)
	}

	if status == nil {
		t.Fatal("Expected non-nil status")
	}

	if status.ZoektProgress < 0 || status.ZoektProgress > 100 {
		t.Errorf("Invalid Zoekt progress: %d", status.ZoektProgress)
	}

	if status.FAISSProgress < 0 || status.FAISSProgress > 100 {
		t.Errorf("Invalid FAISS progress: %d", status.FAISSProgress)
	}

	if status.TotalFiles < 0 {
		t.Errorf("Invalid total files: %d", status.TotalFiles)
	}

	if status.IndexedFiles < 0 {
		t.Errorf("Invalid indexed files: %d", status.IndexedFiles)
	}
}

func TestQueryService_ExtractKeywords(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	tests := []struct {
		query    string
		expected []string
	}{
		{
			"find main function",
			[]string{"main", "function"},
		},
		{
			"how to implement authentication",
			[]string{"implement", "authentication"},
		},
		{
			"def calculate_sum(a, b)",
			[]string{"def", "calculate_sum"},
		},
		{
			"function getUserData()",
			[]string{"function", "getuserdata"},
		},
		{
			"the quick brown fox",
			[]string{"quick", "brown", "fox"},
		},
	}

	for _, test := range tests {
		keywords := qs.extractKeywords(test.query)

		// Check that all expected keywords are present
		for _, expected := range test.expected {
			found := false
			for _, keyword := range keywords {
				if keyword == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Query '%s': expected keyword '%s' not found in %v",
					test.query, expected, keywords)
			}
		}
	}
}

func TestQueryService_ExtractProgrammingTerms(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	tests := []struct {
		query    string
		expected []string
	}{
		{
			"def calculate_sum(a, b):",
			[]string{"calculate_sum"},
		},
		{
			"function getData() { return data; }",
			[]string{"getdata"},
		},
		{
			"class UserManager:",
			[]string{"usermanager"},
		},
		{
			"import numpy as np",
			[]string{"numpy"},
		},
	}

	for _, test := range tests {
		terms := qs.extractProgrammingTerms(test.query)

		if len(terms) == 0 && len(test.expected) > 0 {
			t.Errorf("Query '%s': expected programming terms, but none found", test.query)
			continue
		}

		for _, expected := range test.expected {
			if !terms[expected] {
				// Print available terms for debugging
				var availableTerms []string
				for term := range terms {
					availableTerms = append(availableTerms, term)
				}
				t.Errorf("Query '%s': expected programming term '%s' not found. Available terms: %v",
					test.query, expected, availableTerms)
			}
		}
	}
}

func TestQueryService_ContainsRegexPatterns(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	tests := []struct {
		query    string
		expected bool
	}{
		{"simple text", false},
		{"func\\s+\\w+", true},
		{"user[0-9]+", true},
		{"start.*end", true},
		{"normal query", false},
		{"\\bword\\b", true},
		{"(group|test)", true},
	}

	for _, test := range tests {
		result := qs.containsRegexPatterns(test.query)
		if result != test.expected {
			t.Errorf("Query '%s': expected regex detection %t, got %t",
				test.query, test.expected, result)
		}
	}
}

func TestQueryService_FusionRanking(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	lexicalHits := []types.SearchHit{
		{File: "/test/file1.go", Score: 0.9, Source: "lex"},
		{File: "/test/file2.py", Score: 0.7, Source: "lex"},
		{File: "/test/file3.js", Score: 0.5, Source: "lex"},
	}

	semanticHits := []types.SearchHit{
		{File: "/test/file1.go", Score: 0.8, Source: "vec"}, // Same file as lexical
		{File: "/test/file4.ts", Score: 0.6, Source: "vec"},
		{File: "/test/file5.rs", Score: 0.4, Source: "vec"},
	}

	fusedHits := qs.fusionRanking(lexicalHits, semanticHits, 10)

	if len(fusedHits) == 0 {
		t.Error("Expected fused results")
	}

	// Check that scores are properly sorted (descending)
	for i := 1; i < len(fusedHits); i++ {
		if fusedHits[i-1].Score < fusedHits[i].Score {
			t.Error("Fused results should be sorted by score (descending)")
		}
	}

	// Check that duplicate files are properly merged
	fileMap := make(map[string]int)
	for _, hit := range fusedHits {
		fileMap[hit.File]++
	}

	for file, count := range fileMap {
		if count > 1 {
			t.Errorf("File %s appears %d times in fused results", file, count)
		}
	}

	// Check that combined hit has correct source
	for _, hit := range fusedHits {
		if hit.File == "/test/file1.go" && hit.Source != "both" {
			t.Errorf("Expected source 'both' for merged hit, got '%s'", hit.Source)
		}
	}
}

func TestQueryService_SuggestQuery(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	ctx := context.Background()

	suggestions, err := qs.SuggestQuery(ctx, "func")
	if err != nil {
		t.Fatalf("SuggestQuery failed: %v", err)
	}

	if len(suggestions) == 0 {
		t.Error("Expected at least one suggestion")
	}

	// Check that suggestions contain relevant terms
	found := false
	for _, suggestion := range suggestions {
		if suggestion == "function definition" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'function definition' in suggestions for 'func'")
	}
}

func TestQueryService_SuggestQueryShortInput(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	ctx := context.Background()

	suggestions, err := qs.SuggestQuery(ctx, "a")
	if err != nil {
		t.Fatalf("SuggestQuery failed: %v", err)
	}

	if len(suggestions) != 0 {
		t.Errorf("Expected no suggestions for short input, got %d", len(suggestions))
	}
}

func TestQueryService_ExplainQuery(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	ctx := context.Background()

	explanation, err := qs.ExplainQuery(ctx, "find main function")
	if err != nil {
		t.Fatalf("ExplainQuery failed: %v", err)
	}

	if explanation == nil {
		t.Fatal("Expected non-nil explanation")
	}

	if explanation.OriginalQuery != "find main function" {
		t.Errorf("Expected original query 'find main function', got '%s'",
			explanation.OriginalQuery)
	}

	if len(explanation.ExtractedKeywords) == 0 {
		t.Error("Expected extracted keywords")
	}

	if explanation.SearchStrategy == "" {
		t.Error("Expected search strategy")
	}

	if explanation.BM25Weight < 0 || explanation.BM25Weight > 1 {
		t.Errorf("Invalid BM25 weight: %f", explanation.BM25Weight)
	}
}

func TestQueryService_ExplainRegexQuery(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	ctx := context.Background()

	explanation, err := qs.ExplainQuery(ctx, "func\\s+\\w+")
	if err != nil {
		t.Fatalf("ExplainQuery failed: %v", err)
	}

	if !explanation.IsRegexQuery {
		t.Error("Expected regex query to be detected")
	}
}

func TestQueryService_Close(t *testing.T) {
	qs := createTestQueryService()

	err := qs.Close()
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify that subsequent operations fail or handle closed state gracefully
	ctx := context.Background()
	request := &types.SearchRequest{Query: "test", TopK: 10}

	// This might succeed or fail depending on implementation details
	// The important thing is that Close() doesn't panic
	_, _ = qs.Search(ctx, request)
}

func TestQueryService_GetHitKey(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	tests := []struct {
		hit      types.SearchHit
		expected string
	}{
		{
			types.SearchHit{File: "/test/file.go", LineNumber: 42},
			"/test/file.go:42",
		},
		{
			types.SearchHit{File: "/test/file.py", LineNumber: 0},
			"/test/file.py",
		},
		{
			types.SearchHit{File: "/test/file.js"},
			"/test/file.js",
		},
	}

	for _, test := range tests {
		result := qs.getHitKey(test.hit)
		if result != test.expected {
			t.Errorf("getHitKey(%+v) = '%s', expected '%s'",
				test.hit, result, test.expected)
		}
	}
}

func TestQueryService_SearchPerformance(t *testing.T) {
	qs := createTestQueryService()
	defer qs.Close()

	if err := setupTestData(qs); err != nil {
		t.Fatalf("Failed to setup test data: %v", err)
	}

	request := &types.SearchRequest{
		Query: "performance test query",
		TopK:  20,
	}

	ctx := context.Background()
	start := time.Now()

	response, err := qs.Search(ctx, request)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	elapsed := time.Since(start)

	// Check that search completes in reasonable time (adjust threshold as needed)
	if elapsed > 5*time.Second {
		t.Errorf("Search took too long: %v", elapsed)
	}

	// Verify that the reported query time is reasonable
	if response.QueryTime > elapsed+100*time.Millisecond {
		t.Errorf("Reported query time (%v) is greater than actual elapsed time (%v)",
			response.QueryTime, elapsed)
	}
}

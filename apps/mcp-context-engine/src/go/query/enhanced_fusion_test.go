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

func createEnhancedTestQueryService() *QueryService {
	// Create enhanced test configuration
	cfg := config.DefaultConfig()
	
	// Configure enhanced fusion settings
	cfg.Fusion.Strategy = config.FusionRRF
	cfg.Fusion.Normalization = config.NormMinMax
	cfg.Fusion.AdaptiveWeighting = true
	cfg.Fusion.EnableAnalytics = true
	cfg.Fusion.DebugScoring = true

	// Create test indexers
	zoektIndexer := indexer.NewZoektIndexer("/tmp/test-zoekt-enhanced")
	faissIndexer := indexer.NewFAISSIndexer("/tmp/test-faiss-enhanced", 768)
	testEmbedder := embedder.NewDefaultEmbedder()

	return NewQueryService(zoektIndexer, faissIndexer, testEmbedder, cfg)
}

func TestEnhancedFusionRanking_RRF(t *testing.T) {
	qs := createEnhancedTestQueryService()
	defer qs.Close()

	lexicalHits := []types.SearchHit{
		{File: "/test/main.go", Score: 0.9, Source: "lex", LineNumber: 10, Text: "function main() {"},
		{File: "/test/utils.py", Score: 0.7, Source: "lex", LineNumber: 5, Text: "def calculate_sum(a, b):"},
		{File: "/test/config.json", Score: 0.5, Source: "lex", LineNumber: 1, Text: `{"main": "index.js"}`},
	}

	semanticHits := []types.SearchHit{
		{File: "/test/main.go", Score: 0.8, Source: "vec", LineNumber: 10, Text: "function main() {"},
		{File: "/test/handler.js", Score: 0.6, Source: "vec", LineNumber: 15, Text: "const handler = () => {}"},
		{File: "/test/utils.py", Score: 0.4, Source: "vec", LineNumber: 5, Text: "def calculate_sum(a, b):"},
	}

	request := &types.SearchRequest{
		Query: "main function",
		TopK:  10,
	}

	fileQuery := &FileQuery{
		OriginalQuery: "main function",
		FocusedQuery:  "main function",
	}

	fusedResults, analytics := qs.enhancedFusionRanking(lexicalHits, semanticHits, request, fileQuery)

	// Verify results
	if len(fusedResults) == 0 {
		t.Error("Expected fused results")
	}

	// Verify analytics
	if analytics == nil {
		t.Error("Expected analytics")
	}

	if analytics.Strategy != string(config.FusionRRF) {
		t.Errorf("Expected strategy RRF, got %s", analytics.Strategy)
	}

	if analytics.LexicalCandidates != len(lexicalHits) {
		t.Errorf("Expected %d lexical candidates, got %d", len(lexicalHits), analytics.LexicalCandidates)
	}

	if analytics.SemanticCandidates != len(semanticHits) {
		t.Errorf("Expected %d semantic candidates, got %d", len(semanticHits), analytics.SemanticCandidates)
	}

	// Verify that combined hit has correct source
	for _, hit := range fusedResults {
		if hit.File == "/test/main.go" && hit.Source != "both" {
			t.Errorf("Expected source 'both' for merged hit, got '%s'", hit.Source)
		}
	}

	// Verify scores are sorted
	for i := 1; i < len(fusedResults); i++ {
		if fusedResults[i-1].Score < fusedResults[i].Score {
			t.Error("Results should be sorted by score (descending)")
		}
	}

	t.Logf("Enhanced fusion test passed with %d results and analytics: %+v", len(fusedResults), analytics)
}

func TestEnhancedFusionRanking_WeightedLinear(t *testing.T) {
	qs := createEnhancedTestQueryService()
	defer qs.Close()

	// Configure for weighted linear fusion
	qs.config.Fusion.Strategy = config.FusionWeightedLinear

	lexicalHits := []types.SearchHit{
		{File: "/test/function.go", Score: 0.9, Source: "lex", LineNumber: 1, Text: "func main() {}"},
		{File: "/test/class.py", Score: 0.8, Source: "lex", LineNumber: 2, Text: "class Calculator:"},
	}

	semanticHits := []types.SearchHit{
		{File: "/test/function.go", Score: 0.7, Source: "vec", LineNumber: 1, Text: "func main() {}"},
		{File: "/test/handler.js", Score: 0.6, Source: "vec", LineNumber: 3, Text: "const handler = () => {}"},
	}

	request := &types.SearchRequest{
		Query: "function definition",
		TopK:  5,
	}

	fileQuery := &FileQuery{}

	fusedResults, analytics := qs.enhancedFusionRanking(lexicalHits, semanticHits, request, fileQuery)

	if analytics.Strategy != string(config.FusionWeightedLinear) {
		t.Errorf("Expected strategy weighted_linear, got %s", analytics.Strategy)
	}

	if len(fusedResults) == 0 {
		t.Error("Expected fused results")
	}

	t.Logf("Weighted linear fusion test passed with %d results", len(fusedResults))
}

func TestEnhancedFusionRanking_LearnedWeights(t *testing.T) {
	qs := createEnhancedTestQueryService()
	defer qs.Close()

	// Configure for learned weights fusion
	qs.config.Fusion.Strategy = config.FusionLearnedWeights

	lexicalHits := []types.SearchHit{
		{File: "/test/short.go", Score: 0.8, Source: "lex"},
	}

	semanticHits := []types.SearchHit{
		{File: "/test/semantic.py", Score: 0.7, Source: "vec"},
	}

	// Test short query (should favor lexical)
	shortRequest := &types.SearchRequest{
		Query: "main",
		TopK:  5,
	}

	fileQuery := &FileQuery{}

	fusedResults, analytics := qs.enhancedFusionRanking(lexicalHits, semanticHits, shortRequest, fileQuery)

	if analytics.Strategy != string(config.FusionLearnedWeights) {
		t.Errorf("Expected strategy learned_weights, got %s", analytics.Strategy)
	}

	// Test long query (should favor semantic)
	longRequest := &types.SearchRequest{
		Query: "this is a very long query that should favor semantic search over lexical search",
		TopK:  5,
	}

	fusedResults2, analytics2 := qs.enhancedFusionRanking(lexicalHits, semanticHits, longRequest, fileQuery)

	if len(fusedResults) == 0 || len(fusedResults2) == 0 {
		t.Error("Expected fused results for both queries")
	}

	t.Logf("Learned weights fusion test passed with short query: %d results, long query: %d results", 
		len(fusedResults), len(fusedResults2))
}

func TestQueryTypeDetection(t *testing.T) {
	qs := createEnhancedTestQueryService()
	defer qs.Close()

	tests := []struct {
		query    string
		expected config.QueryType
	}{
		{"import React from 'react'", config.QueryImport},
		{"package.json dependencies", config.QueryConfig},
		{"find main function", config.QueryCode},
		{"getUserData method", config.QuerySymbol},
		{"*.go files", config.QueryFile},
		{"how to implement authentication", config.QueryNatural},
	}

	for _, test := range tests {
		fileQuery := qs.parseFileQuery(test.query)
		queryType := qs.detectQueryType(test.query, fileQuery)
		
		if queryType != test.expected {
			t.Errorf("Query '%s': expected type %s, got %s", test.query, test.expected, queryType)
		}
	}
}

func TestScoreNormalization(t *testing.T) {
	qs := createEnhancedTestQueryService()
	defer qs.Close()

	hits := []types.SearchHit{
		{File: "/test/file1.go", Score: 1.0},
		{File: "/test/file2.go", Score: 0.5},
		{File: "/test/file3.go", Score: 0.1},
	}

	// Test min-max normalization
	normalizedMinMax := qs.normalizeScores(hits, config.NormMinMax, "test")
	if normalizedMinMax[0].Score != 1.0 || normalizedMinMax[2].Score != 0.0 {
		t.Error("Min-max normalization failed")
	}

	// Test z-score normalization
	normalizedZScore := qs.normalizeScores(hits, config.NormZScore, "test")
	if len(normalizedZScore) != len(hits) {
		t.Error("Z-score normalization changed result count")
	}

	// Test rank-based normalization
	normalizedRank := qs.normalizeScores(hits, config.NormRankBased, "test")
	if normalizedRank[0].Score != 1.0 || normalizedRank[1].Score != 0.5 {
		t.Error("Rank-based normalization failed")
	}

	t.Log("Score normalization tests passed")
}

func TestBoostFactors(t *testing.T) {
	qs := createEnhancedTestQueryService()
	defer qs.Close()

	hits := []types.SearchHit{
		{File: "/test/main.go", Score: 0.5, Text: "function main() { return 42; }"},
		{File: "/test/utils.py", Score: 0.4, Text: "def calculate_sum(a, b): return a + b"},
		{File: "/test/other.js", Score: 0.3, Text: "const handler = () => {}"},
	}

	request := &types.SearchRequest{
		Query: "main function",
	}

	fileQuery := &FileQuery{
		FilePatterns: []string{"*.go"},
	}

	cfg := &qs.config.Fusion
	boostStats := &types.BoostFactors{}

	boostedHits := qs.applyBoostFactors(hits, request, fileQuery, cfg, boostStats)

	// Verify boosts were applied
	if boostedHits[0].Score <= hits[0].Score {
		t.Error("Expected boost to be applied to exact match")
	}

	if boostStats.ExactMatches == 0 {
		t.Error("Expected at least one exact match boost")
	}

	if boostStats.FileTypeBoosts == 0 {
		t.Error("Expected at least one file type boost")
	}

	t.Logf("Boost factors test passed: exact matches=%d, file type boosts=%d, avg boost=%.2f",
		boostStats.ExactMatches, boostStats.FileTypeBoosts, boostStats.AvgBoostFactor)
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that the legacy fusionRanking function still works
	qs := createTestQueryService() // Use original test service
	defer qs.Close()

	lexicalHits := []types.SearchHit{
		{File: "/test/file1.go", Score: 0.9, Source: "lex"},
		{File: "/test/file2.py", Score: 0.7, Source: "lex"},
	}

	semanticHits := []types.SearchHit{
		{File: "/test/file1.go", Score: 0.8, Source: "vec"},
		{File: "/test/file3.js", Score: 0.6, Source: "vec"},
	}

	// This should use the original implementation since strategy is empty
	results := qs.fusionRanking(lexicalHits, semanticHits, 10)

	if len(results) == 0 {
		t.Error("Expected results from legacy fusion ranking")
	}

	// Verify that the combined hit has correct source
	for _, hit := range results {
		if hit.File == "/test/file1.go" && hit.Source != "both" {
			t.Errorf("Expected source 'both' for merged hit, got '%s'", hit.Source)
		}
	}

	t.Log("Backward compatibility test passed")
}

func TestAnalyticsOutput(t *testing.T) {
	qs := createEnhancedTestQueryService()
	defer qs.Close()

	lexicalHits := []types.SearchHit{
		{File: "/test/file1.go", Score: 0.9, Source: "lex"},
		{File: "/test/file2.py", Score: 0.8, Source: "lex"},
	}

	semanticHits := []types.SearchHit{
		{File: "/test/file1.go", Score: 0.7, Source: "vec"},
		{File: "/test/file3.js", Score: 0.6, Source: "vec"},
	}

	request := &types.SearchRequest{
		Query: "function implementation",
		TopK:  5,
	}

	fileQuery := &FileQuery{}

	_, analytics := qs.enhancedFusionRanking(lexicalHits, semanticHits, request, fileQuery)

	// Verify analytics structure
	if analytics.ProcessingTime <= 0 {
		t.Error("Expected positive processing time")
	}

	if analytics.TotalCandidates < 0 {
		t.Error("Expected non-negative total candidates")
	}

	if analytics.EffectiveWeight < 0 || analytics.EffectiveWeight > 1 {
		t.Errorf("Expected effective weight between 0 and 1, got %f", analytics.EffectiveWeight)
	}

	if analytics.QueryType == "" {
		t.Error("Expected query type to be detected")
	}

	t.Logf("Analytics test passed: %+v", analytics)
}
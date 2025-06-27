package query

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/config"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

// QueryService orchestrates search across lexical and vector indexes
type QueryService struct {
	zoektIndexer  indexer.ZoektIndexer
	faissIndexer  indexer.FAISSIndexer
	embedder      embedder.Embedder
	config        *config.Config
}

// NewQueryService creates a new query service instance
func NewQueryService(
	zoektIndexer indexer.ZoektIndexer,
	faissIndexer indexer.FAISSIndexer,
	embedder embedder.Embedder,
	config *config.Config,
) *QueryService {
	return &QueryService{
		zoektIndexer: zoektIndexer,
		faissIndexer: faissIndexer,
		embedder:     embedder,
		config:       config,
	}
}

// Search performs hybrid search combining lexical and semantic results
func (qs *QueryService) Search(ctx context.Context, request *types.SearchRequest) (*types.SearchResponse, error) {
	start := time.Now()

	// Validate request
	if request.Query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Set default top-k if not specified
	if request.TopK <= 0 {
		request.TopK = 20
	}

	// Extract keywords from natural language query
	keywords := qs.extractKeywords(request.Query)

	// Perform lexical search
	lexicalResults, err := qs.performLexicalSearch(ctx, request, keywords)
	if err != nil {
		return nil, fmt.Errorf("lexical search failed: %w", err)
	}

	// Perform semantic search
	semanticResults, err := qs.performSemanticSearch(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("semantic search failed: %w", err)
	}

	// Fusion ranking: combine lexical and semantic results
	fusedResults := qs.fusionRanking(lexicalResults, semanticResults, request.TopK)

	// Build response
	response := &types.SearchResponse{
		Hits:         fusedResults,
		TotalHits:    len(fusedResults),
		QueryTime:    time.Since(start),
		LexicalHits:  len(lexicalResults),
		SemanticHits: len(semanticResults),
	}

	return response, nil
}

// GetIndexStatus returns the current indexing status
func (qs *QueryService) GetIndexStatus(ctx context.Context) (*types.IndexStatus, error) {
	// Get stats from both indexes
	zoektStats := qs.zoektIndexer.Stats()
	faissStats := qs.faissIndexer.VectorStats()

	// Calculate progress percentages (simplified)
	zoektProgress := 100
	faissProgress := 100
	if zoektStats.TotalFiles > 0 {
		zoektProgress = (zoektStats.IndexedFiles * 100) / zoektStats.TotalFiles
	}
	if faissStats.TotalVectors > 0 {
		faissProgress = 100 // Assume complete if vectors exist
	}

	status := &types.IndexStatus{
		Repository:     "default", // TODO: support multiple repos
		ZoektProgress:  zoektProgress,
		FAISSProgress:  faissProgress,
		TotalFiles:     zoektStats.TotalFiles,
		IndexedFiles:   zoektStats.IndexedFiles,
		LastUpdated:    zoektStats.LastIndexTime,
	}

	return status, nil
}

// Private methods for search orchestration

func (qs *QueryService) extractKeywords(query string) []string {
	// Enhanced keyword extraction with better handling of search intent and technical terms
	
	// Detect file-type queries first (before tokenization)
	fileTypeTerms := qs.detectFileTypeQueries(query)
	
	// Remove common stop words but preserve action verbs that indicate search intent
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true,
		"could": true, "should": true, "may": true, "might": true, "must": true,
		"can": true, "this": true, "that": true, "these": true, "those": true,
		"i": true, "you": true, "he": true, "she": true, "it": true, "we": true, "they": true,
		"my": true, "your": true, "his": true, "her": true, "its": true, "our": true, "their": true,
		"me": true, "him": true, "us": true, "them": true,
		"what": true, "where": true, "when": true, "why": true, "how": true,
		// Removed action verbs that indicate search intent:
		// "find": true, "search": true, "look": true, "show": true, "get": true, "list": true, "display": true,
	}

	// Extract programming-specific terms and preserve them
	programmingTerms := qs.extractProgrammingTerms(query)

	// Tokenize using enhanced tokenizer that preserves compound terms
	words := qs.tokenizeWithCompounds(strings.ToLower(query))
	var keywords []string

	for _, word := range words {
		// Keep file-type terms regardless of other filters
		if _, isFileType := fileTypeTerms[word]; isFileType {
			keywords = append(keywords, word)
			continue
		}

		// Keep programming terms regardless of stop word status
		if _, isProgrammingTerm := programmingTerms[word]; isProgrammingTerm {
			keywords = append(keywords, word)
			continue
		}

		// Filter out stop words and short words
		if len(word) > 2 && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	// Add back file-type terms that might have been tokenized differently
	for term := range fileTypeTerms {
		found := false
		for _, keyword := range keywords {
			if keyword == term {
				found = true
				break
			}
		}
		if !found {
			keywords = append(keywords, term)
		}
	}

	// Add back programming terms that might have been tokenized differently
	for term := range programmingTerms {
		found := false
		for _, keyword := range keywords {
			if keyword == term {
				found = true
				break
			}
		}
		if !found {
			keywords = append(keywords, term)
		}
	}

	return keywords
}

// detectFileTypeQueries identifies file names and extensions in the query
func (qs *QueryService) detectFileTypeQueries(query string) map[string]bool {
	terms := make(map[string]bool)
	
	// Common config files and their patterns
	configFiles := []string{
		"package.json", "tsconfig.json", "go.mod", "go.sum", "Cargo.toml", "Cargo.lock",
		"composer.json", "pom.xml", "build.gradle", "requirements.txt", "Pipfile",
		"Dockerfile", "docker-compose.yml", "docker-compose.yaml",
		".gitignore", ".eslintrc", ".prettierrc", ".babelrc", "webpack.config.js",
		"jest.config.js", "vite.config.js", "rollup.config.js",
		"makefile", "Makefile", "justfile", "Justfile",
		"README.md", "CHANGELOG.md", "LICENSE", "CONTRIBUTING.md",
	}
	
	// Check for exact matches (case-insensitive)
	lowerQuery := strings.ToLower(query)
	for _, file := range configFiles {
		if strings.Contains(lowerQuery, strings.ToLower(file)) {
			terms[strings.ToLower(file)] = true
		}
	}
	
	// File extension patterns
	extensionPatterns := []string{
		`\.[a-zA-Z0-9]+\b`, // .js, .py, .go, etc.
	}
	
	for _, pattern := range extensionPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(query, -1)
		for _, match := range matches {
			terms[strings.ToLower(match)] = true
		}
	}
	
	// JSON field names and common config keys
	jsonFields := []string{
		"main", "scripts", "dependencies", "devDependencies", "peerDependencies",
		"name", "version", "description", "author", "license", "homepage",
		"repository", "bugs", "keywords", "engines", "files", "bin",
		"workspaces", "private", "publishConfig", "config",
		"compilerOptions", "include", "exclude", "extends", "references",
		"target", "module", "lib", "outDir", "rootDir", "strict",
		"esModuleInterop", "skipLibCheck", "forceConsistentCasingInFileNames",
	}
	
	// Check for JSON field names in context (e.g., "main field", "scripts section")
	for _, field := range jsonFields {
		if strings.Contains(lowerQuery, field) {
			terms[field] = true
		}
	}
	
	return terms
}

// tokenizeWithCompounds preserves compound terms like package.json while tokenizing
func (qs *QueryService) tokenizeWithCompounds(text string) []string {
	// First, identify and preserve compound terms
	compoundPatterns := []string{
		`\w+\.\w+`, // file.ext patterns
		`\w+-\w+`,  // hyphenated terms
		`\w+_\w+`,  // underscore terms
	}
	
	var compounds []string
	var replacements []string
	text = strings.ToLower(text)
	
	for i, pattern := range compoundPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(text, -1)
		for j, match := range matches {
			placeholder := fmt.Sprintf("__COMPOUND_%d_%d__", i, j)
			compounds = append(compounds, match)
			replacements = append(replacements, placeholder)
			text = strings.Replace(text, match, placeholder, 1)
		}
	}
	
	// Regular tokenization
	re := regexp.MustCompile(`[^\w]+`)
	tokens := re.Split(text, -1)
	
	// Replace placeholders with original compound terms
	for i, replacement := range replacements {
		for j, token := range tokens {
			if token == replacement {
				tokens[j] = compounds[i]
				break
			}
		}
	}
	
	// Filter out empty tokens
	var result []string
	for _, token := range tokens {
		if len(token) > 0 {
			result = append(result, token)
		}
	}
	
	return result
}

func (qs *QueryService) extractProgrammingTerms(query string) map[string]bool {
	terms := make(map[string]bool)
	
	// Enhanced programming patterns with better file and config recognition
	patterns := []string{
		// Function patterns
		`\b\w+\(\)`,  // function calls
		`\bdef\s+\w+`, `\bfunction\s+\w+`, `\bfunc\s+\w+`, // function definitions
		
		// Variable patterns
		`\b[a-zA-Z_][a-zA-Z0-9_]*\s*=`, // variable assignments
		
		// Class/struct patterns
		`\bclass\s+\w+`, `\bstruct\s+\w+`, `\binterface\s+\w+`,
		
		// Import/include patterns
		`\bimport\s+\w+`, `\bfrom\s+\w+`, `\b#include\s*<\w+>`,
		
		// File patterns with extensions
		`\b\w+\.\w{1,5}\b`, // files with extensions (e.g., main.go, index.js)
		
		// Configuration patterns
		`\b[a-zA-Z_][a-zA-Z0-9_]*\.json\b`, // JSON config files
		`\b[a-zA-Z_][a-zA-Z0-9_]*\.ya?ml\b`, // YAML config files
		`\b[a-zA-Z_][a-zA-Z0-9_]*\.toml\b`, // TOML config files
		`\b[a-zA-Z_][a-zA-Z0-9_]*\.lock\b`, // Lock files
		
		// API and HTTP patterns
		`\b(GET|POST|PUT|DELETE|PATCH)\b`, // HTTP methods
		`\b/\w+(/\w+)*\b`, // API endpoints
		
		// Common keywords
		`\b(if|else|for|while|switch|case|try|catch|finally|return|break|continue)\b`,
		`\b(public|private|protected|static|const|let|var|final)\b`,
		`\b(int|string|bool|float|double|char|void|null|undefined)\b`,
		
		// Database patterns
		`\b(SELECT|INSERT|UPDATE|DELETE|FROM|WHERE|JOIN)\b`,
		
		// Framework/library specific
		`\b(React|Vue|Angular|Express|Django|Flask|Rails)\b`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllString(query, -1)
		for _, match := range matches {
			// Extract the meaningful part (remove keywords like 'def', 'function', etc.)
			cleanMatch := qs.cleanProgrammingTerm(match)
			if cleanMatch != "" {
				terms[strings.ToLower(cleanMatch)] = true
			}
		}
	}

	return terms
}

func (qs *QueryService) cleanProgrammingTerm(term string) string {
	// Remove common prefixes and suffixes while preserving important file names
	term = strings.TrimSpace(term)
	
	// Preserve file extensions and compound names
	if strings.Contains(term, ".") && !strings.HasPrefix(term, ".") {
		// This looks like a filename, preserve it
		return term
	}
	
	// Remove function definition keywords
	prefixesToRemove := []string{"def ", "function ", "func ", "class ", "struct ", "interface ", "import ", "from "}
	for _, prefix := range prefixesToRemove {
		if after, found := strings.CutPrefix(term, prefix); found {
			term = after
			break
		}
	}

	// Remove parentheses for function calls
	term = strings.TrimSuffix(term, "()")

	// Remove assignment operators
	if idx := strings.Index(term, "="); idx > 0 {
		term = strings.TrimSpace(term[:idx])
	}
	
	// Remove angle brackets from includes
	if strings.HasPrefix(term, "<") && strings.HasSuffix(term, ">") {
		term = strings.TrimPrefix(strings.TrimSuffix(term, ">"), "<")
	}

	return term
}

func (qs *QueryService) performLexicalSearch(ctx context.Context, request *types.SearchRequest, keywords []string) ([]types.SearchHit, error) {
	// Construct search query from keywords
	searchQuery := strings.Join(keywords, " ")
	if searchQuery == "" {
		searchQuery = request.Query // fallback to original query
	}

	// Set up search options
	options := indexer.SearchOptions{
		MaxResults:    request.TopK * 2, // Get more results for fusion ranking
		UseRegex:      qs.containsRegexPatterns(request.Query),
		CaseSensitive: false,
		FilePatterns:  request.Filters.FilePatterns,
		Languages:     []string{request.Language},
	}

	// Filter out empty language
	if request.Language == "" {
		options.Languages = []string{}
	}

	return qs.zoektIndexer.Search(ctx, searchQuery, options)
}

func (qs *QueryService) performSemanticSearch(ctx context.Context, request *types.SearchRequest) ([]types.SearchHit, error) {
	// Create a dummy code chunk for the query to generate its embedding
	queryChunk := types.CodeChunk{
		FileID:   "query",
		FilePath: "query",
		Text:     request.Query,
		Language: request.Language,
		Hash:     fmt.Sprintf("query-%d", time.Now().UnixNano()),
	}

	// Generate embedding for the query
	queryEmbedding, err := qs.embedder.EmbedSingle(ctx, queryChunk)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Search in vector index
	vectorOptions := indexer.VectorSearchOptions{
		MinScore: 0.1, // Minimum similarity threshold
	}

	vectorResults, err := qs.faissIndexer.Search(ctx, queryEmbedding.Vector, request.TopK*2, vectorOptions)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Convert vector results to search hits
	var hits []types.SearchHit
	for _, result := range vectorResults {
		// Parse chunk ID to extract file info (simplified)
		parts := strings.Split(result.ChunkID, ":")
		filePath := result.ChunkID
		if len(parts) > 1 {
			filePath = parts[0]
		}

		hit := types.SearchHit{
			File:       filePath,
			LineNumber: 1, // TODO: extract actual line number from chunk ID
			Text:       qs.getChunkText(result.ChunkID), // TODO: implement chunk text retrieval
			Score:      float64(result.Score),
			Source:     "vec",
			Language:   request.Language,
		}
		hits = append(hits, hit)
	}

	return hits, nil
}

func (qs *QueryService) fusionRanking(lexicalHits, semanticHits []types.SearchHit, topK int) []types.SearchHit {
	// Reciprocal Rank Fusion (RRF) with BM25 weighting
	lambda := qs.config.Fusion.BM25Weight
	k := 60.0 // RRF constant

	// Create maps for efficient lookup
	lexicalScores := make(map[string]float64)
	semanticScores := make(map[string]float64)
	allHits := make(map[string]types.SearchHit)

	// Process lexical hits
	for rank, hit := range lexicalHits {
		key := qs.getHitKey(hit)
		rrf := 1.0 / (k + float64(rank+1))
		lexicalScores[key] = hit.Score * lambda * rrf
		allHits[key] = hit
	}

	// Process semantic hits
	for rank, hit := range semanticHits {
		key := qs.getHitKey(hit)
		rrf := 1.0 / (k + float64(rank+1))
		semanticScore := hit.Score * (1.0 - lambda) * rrf
		
		if existing, exists := allHits[key]; exists {
			// Combine scores for hits that appear in both results
			existing.Score = lexicalScores[key] + semanticScore
			existing.Source = "both"
			allHits[key] = existing
		} else {
			// New hit from semantic search only
			hit.Score = semanticScore
			semanticScores[key] = semanticScore
			allHits[key] = hit
		}
	}

	// Convert back to slice and sort by combined score
	var fusedHits []types.SearchHit
	for _, hit := range allHits {
		fusedHits = append(fusedHits, hit)
	}

	sort.Slice(fusedHits, func(i, j int) bool {
		return fusedHits[i].Score > fusedHits[j].Score
	})

	// Limit to top-k results
	if len(fusedHits) > topK {
		fusedHits = fusedHits[:topK]
	}

	return fusedHits
}

func (qs *QueryService) getHitKey(hit types.SearchHit) string {
	// Create a unique key for deduplication
	// Use file path and line number, or just file path if line number is not reliable
	if hit.LineNumber > 0 {
		return fmt.Sprintf("%s:%d", hit.File, hit.LineNumber)
	}
	return hit.File
}

func (qs *QueryService) containsRegexPatterns(query string) bool {
	// Simple heuristic to detect regex patterns
	regexIndicators := []string{
		"\\b", "\\w", "\\d", "\\s", // word boundaries and character classes
		"[", "]", "(", ")", // character sets and groups
		"*", "+", "?", "{", "}", // quantifiers
		"^", "$", // anchors
		"|", // alternation
	}

	for _, indicator := range regexIndicators {
		if strings.Contains(query, indicator) {
			return true
		}
	}

	return false
}

func (qs *QueryService) getChunkText(chunkID string) string {
	// Parse chunkID format: filePath:startByte-endByte
	parts := strings.Split(chunkID, ":")
	if len(parts) != 2 {
		return fmt.Sprintf("Invalid chunk ID format: %s", chunkID)
	}
	
	filePath := parts[0]
	byteRange := parts[1]
	
	// Parse byte range
	rangeParts := strings.Split(byteRange, "-")
	if len(rangeParts) != 2 {
		return fmt.Sprintf("Invalid byte range format: %s", byteRange)
	}
	
	startByte, err := strconv.Atoi(rangeParts[0])
	if err != nil {
		return fmt.Sprintf("Invalid start byte: %s", rangeParts[0])
	}
	
	endByte, err := strconv.Atoi(rangeParts[1])
	if err != nil {
		return fmt.Sprintf("Invalid end byte: %s", rangeParts[1])
	}
	
	// Validate byte range
	if startByte < 0 || endByte < startByte {
		return fmt.Sprintf("Invalid byte range: %d-%d", startByte, endByte)
	}
	
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Sprintf("Failed to read file %s: %v", filePath, err)
	}
	
	// Validate byte positions against file size
	if startByte >= len(content) {
		return fmt.Sprintf("Start byte %d exceeds file size %d", startByte, len(content))
	}
	
	// Adjust end byte if it exceeds file size
	if endByte > len(content) {
		endByte = len(content)
	}
	
	// Extract the chunk
	chunkContent := content[startByte:endByte]
	return string(chunkContent)
}

// Additional utility methods

// SuggestQuery provides query suggestions based on common patterns
func (qs *QueryService) SuggestQuery(ctx context.Context, partial string) ([]string, error) {
	if len(partial) < 2 {
		return []string{}, nil
	}

	// Common programming query patterns
	suggestions := []string{
		"function definition",
		"class implementation",
		"import statements",
		"error handling",
		"unit tests",
		"configuration setup",
		"API endpoints",
		"database queries",
		"authentication logic",
		"utility functions",
	}

	// Filter suggestions based on partial match
	var matches []string
	lowerPartial := strings.ToLower(partial)
	
	for _, suggestion := range suggestions {
		if strings.Contains(strings.ToLower(suggestion), lowerPartial) {
			matches = append(matches, suggestion)
		}
	}

	// Add some dynamic suggestions based on the partial query
	if strings.Contains(lowerPartial, "func") {
		matches = append(matches, "function "+partial)
	}
	if strings.Contains(lowerPartial, "class") {
		matches = append(matches, "class "+partial)
	}
	if strings.Contains(lowerPartial, "test") {
		matches = append(matches, "test cases for "+partial)
	}

	// Limit suggestions
	if len(matches) > 10 {
		matches = matches[:10]
	}

	return matches, nil
}

// ExplainQuery provides explanation of how a query will be processed
func (qs *QueryService) ExplainQuery(ctx context.Context, query string) (*QueryExplanation, error) {
	keywords := qs.extractKeywords(query)
	isRegex := qs.containsRegexPatterns(query)
	
	explanation := &QueryExplanation{
		OriginalQuery:    query,
		ExtractedKeywords: keywords,
		IsRegexQuery:     isRegex,
		SearchStrategy:   "hybrid",
		BM25Weight:       qs.config.Fusion.BM25Weight,
	}

	if len(keywords) == 0 {
		explanation.SearchStrategy = "semantic-only"
	}

	return explanation, nil
}

// QueryExplanation describes how a query will be processed
type QueryExplanation struct {
	OriginalQuery     string   `json:"original_query"`
	ExtractedKeywords []string `json:"extracted_keywords"`
	IsRegexQuery      bool     `json:"is_regex_query"`
	SearchStrategy    string   `json:"search_strategy"`
	BM25Weight        float64  `json:"bm25_weight"`
}

// Close releases resources
func (qs *QueryService) Close() error {
	var errs []error

	if err := qs.zoektIndexer.Close(); err != nil {
		errs = append(errs, err)
	}

	if err := qs.faissIndexer.Close(); err != nil {
		errs = append(errs, err)
	}

	if err := qs.embedder.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing query service: %v", errs)
	}

	return nil
}

// GetStatus returns the current indexing status
func (qs *QueryService) GetStatus(ctx context.Context) (*types.IndexStatus, error) {
	zoektStats := qs.zoektIndexer.Stats()
	faissStats := qs.faissIndexer.VectorStats()
	
	// Calculate progress percentages
	zoektProgress := 0
	if zoektStats.TotalFiles > 0 {
		zoektProgress = int((float64(zoektStats.IndexedFiles) / float64(zoektStats.TotalFiles)) * 100)
	}
	
	faissProgress := 0
	if faissStats.TotalVectors > 0 {
		faissProgress = 100 // Assume complete if we have vectors
	}
	
	return &types.IndexStatus{
		Repository:    "current", // TODO: Get actual repo path
		ZoektProgress: zoektProgress,
		FAISSProgress: faissProgress,
		TotalFiles:    zoektStats.TotalFiles,
		IndexedFiles:  zoektStats.IndexedFiles,
		LastUpdated:   zoektStats.LastIndexTime,
	}, nil
}
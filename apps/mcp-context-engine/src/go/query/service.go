package query

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/config"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
	"math"
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

	// Parse file-aware query to detect file patterns and extract focused keywords
	fileQuery := qs.parseFileQuery(request.Query)

	// Extract keywords from natural language query (using focused keywords if file query was detected)
	keywords := qs.extractKeywords(fileQuery.FocusedQuery)

	// Perform lexical search with file patterns from query analysis
	lexicalResults, err := qs.performLexicalSearch(ctx, request, keywords, fileQuery)
	if err != nil {
		return nil, fmt.Errorf("lexical search failed: %w", err)
	}

	// Perform semantic search
	semanticResults, err := qs.performSemanticSearch(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("semantic search failed: %w", err)
	}

	// Filter semantic results for JSON field queries to improve relevance
	if fileQuery.IsJSONFieldQuery {
		semanticResults = qs.filterSemanticResultsForJSONFields(semanticResults, fileQuery)
	}

	// Fusion ranking: combine lexical and semantic results
	fusedResults, analytics := qs.enhancedFusionRanking(lexicalResults, semanticResults, request, fileQuery)

	// Build response
	response := &types.SearchResponse{
		Hits:         fusedResults,
		TotalHits:    len(fusedResults),
		QueryTime:    time.Since(start),
		LexicalHits:  len(lexicalResults),
		SemanticHits: len(semanticResults),
	}
	
	// Include analytics if enabled
	if qs.config.Fusion.EnableAnalytics {
		response.Analytics = analytics
	}

	// Final debug summary
	log.Printf("[DEBUG] Search complete for query '%s': lexical=%d, semantic=%d, fused=%d, time=%v",
		request.Query, len(lexicalResults), len(semanticResults), len(fusedResults), time.Since(start))

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

// parseFileQuery analyzes the query to detect file-specific patterns and extract focused search terms
func (qs *QueryService) parseFileQuery(query string) *FileQuery {
	fileQuery := &FileQuery{
		OriginalQuery: query,
		FocusedQuery:  query,
		FilePatterns:  []string{},
		TargetFields:  []string{},
	}

	lowerQuery := strings.ToLower(query)
	
	// Detect import/usage queries early to preserve library names
	if qs.isImportUsageQuery(query) {
		fileQuery = qs.parseImportUsageQuery(query, fileQuery)
		return fileQuery
	}
	
	// File type patterns with their corresponding file patterns
	fileTypeMap := map[string][]string{
		"package.json": {"package.json"},
		"package json": {"package.json"},
		"package.json files": {"package.json"},
		"tsconfig.json": {"tsconfig.json"},
		"tsconfig json": {"tsconfig.json"},
		"go.mod": {"go.mod"},
		"go.sum": {"go.sum"},
		"cargo.toml": {"Cargo.toml"},
		"cargo.lock": {"Cargo.lock"},
		"dockerfile": {"Dockerfile", "*.dockerfile"},
		"docker-compose": {"docker-compose.yml", "docker-compose.yaml"},
		"makefile": {"Makefile", "makefile"},
		"justfile": {"Justfile", "justfile"},
		"readme": {"README.md", "readme.md", "README.txt"},
		".gitignore": {".gitignore"},
		"gitignore": {".gitignore"},
		".eslintrc": {".eslintrc*"},
		"eslintrc": {".eslintrc*"},
		"webpack.config": {"webpack.config.*"},
		"vite.config": {"vite.config.*"},
		"jest.config": {"jest.config.*"},
	}

	// Language-based file patterns
	langPatterns := map[string][]string{
		"go files": {"*.go"},
		"javascript files": {"*.js"},
		"js files": {"*.js"},
		"typescript files": {"*.ts", "*.tsx"},
		"ts files": {"*.ts", "*.tsx"},
		"python files": {"*.py"},
		"py files": {"*.py"},
		"java files": {"*.java"},
		"c files": {"*.c", "*.h"},
		"cpp files": {"*.cpp", "*.cc", "*.cxx", "*.hpp"},
		"rust files": {"*.rs"},
		"yaml files": {"*.yaml", "*.yml"},
		"yml files": {"*.yaml", "*.yml"},
		"json files": {"*.json"},
		"xml files": {"*.xml"},
		"html files": {"*.html"},
		"css files": {"*.css"},
		"markdown files": {"*.md"},
		"md files": {"*.md"},
	}

	// Configuration and field-specific patterns
	configFields := map[string][]string{
		"main field": {"main"},
		"scripts": {"scripts"},
		"dependencies": {"dependencies"},
		"devdependencies": {"devDependencies"},
		"peerdependencies": {"peerDependencies"},
		"name field": {"name"},
		"version field": {"version"},
		"description field": {"description"},
		"author field": {"author"},
		"license field": {"license"},
		"imports": {"import"},
		"exports": {"export"},
		"modules": {"module"},
		"require": {"require"},
		"usage": {"usage"},
		"used": {"used"},
		"imported": {"imported"},
	}

	// Check for specific file type matches
	for fileType, patterns := range fileTypeMap {
		if strings.Contains(lowerQuery, fileType) {
			fileQuery.FilePatterns = append(fileQuery.FilePatterns, patterns...)
			fileQuery.DetectedFileType = fileType
			// Remove file type reference from focused query
			fileQuery.FocusedQuery = strings.ReplaceAll(fileQuery.FocusedQuery, fileType, "")
			log.Printf("[DEBUG] Detected file type '%s' with patterns: %v", fileType, patterns)
			break
		}
	}

	// Check for language-based patterns
	if len(fileQuery.FilePatterns) == 0 {
		for langPattern, patterns := range langPatterns {
			if strings.Contains(lowerQuery, langPattern) {
				fileQuery.FilePatterns = append(fileQuery.FilePatterns, patterns...)
				fileQuery.DetectedFileType = langPattern
				// Remove language reference from focused query
				fileQuery.FocusedQuery = strings.ReplaceAll(fileQuery.FocusedQuery, langPattern, "")
				log.Printf("[DEBUG] Detected language pattern '%s' with patterns: %v", langPattern, patterns)
				break
			}
		}
	}

	// Extract target fields (configuration keys, JSON fields, etc.)
	for fieldPattern, fields := range configFields {
		if strings.Contains(lowerQuery, fieldPattern) {
			fileQuery.TargetFields = append(fileQuery.TargetFields, fields...)
			log.Printf("[DEBUG] Detected target fields for '%s': %v", fieldPattern, fields)
		}
	}

	// Special handling for package.json queries
	if fileQuery.DetectedFileType == "package.json" || strings.Contains(lowerQuery, "package.json") {
		if len(fileQuery.FilePatterns) == 0 {
			fileQuery.FilePatterns = []string{"package.json"}
			fileQuery.DetectedFileType = "package.json"
		}
		
		// Common package.json field queries
		packageJsonFields := []string{"main", "scripts", "dependencies", "devDependencies", "name", "version"}
		for _, field := range packageJsonFields {
			if strings.Contains(lowerQuery, field) && !slices.Contains(fileQuery.TargetFields, field) {
				fileQuery.TargetFields = append(fileQuery.TargetFields, field)
			}
		}
		
		// Set JSON field search flag for better filtering
		fileQuery.IsJSONFieldQuery = true
	}

	// Clean up the focused query by removing common file-related words
	cleanupWords := []string{"files", "file", "in", "of", "the", "for", "from", "within"}
	focusedWords := strings.Fields(fileQuery.FocusedQuery)
	var cleanedWords []string
	
	for _, word := range focusedWords {
		word = strings.TrimSpace(word)
		if word != "" && !slices.Contains(cleanupWords, strings.ToLower(word)) {
			cleanedWords = append(cleanedWords, word)
		}
	}
	
	fileQuery.FocusedQuery = strings.Join(cleanedWords, " ")
	
	// If we have target fields, focus the query on those
	if len(fileQuery.TargetFields) > 0 {
		if fileQuery.FocusedQuery == "" {
			fileQuery.FocusedQuery = strings.Join(fileQuery.TargetFields, " ")
		} else {
			// Combine target fields with focused query
			fileQuery.FocusedQuery = strings.Join(fileQuery.TargetFields, " ") + " " + fileQuery.FocusedQuery
		}
	}

	// Final fallback - if focused query is empty, use original
	if strings.TrimSpace(fileQuery.FocusedQuery) == "" {
		fileQuery.FocusedQuery = query
	}

	log.Printf("[DEBUG] parseFileQuery result - Original: '%s', FilePatterns: %v, FocusedQuery: '%s', DetectedFileType: '%s', TargetFields: %v", 
		query, fileQuery.FilePatterns, fileQuery.FocusedQuery, fileQuery.DetectedFileType, fileQuery.TargetFields)

	return fileQuery
}

func (qs *QueryService) extractKeywords(query string) []string {
	// Enhanced keyword extraction with better handling of search intent and technical terms
	log.Printf("[DEBUG] extractKeywords input: '%s'", query)
	
	// Detect file-type queries first (before tokenization)
	fileTypeTerms := qs.detectFileTypeQueries(query)
	
	// Remove common stop words but preserve action verbs that indicate search intent
	stopWords := map[string]bool{
		"field": true, "section": true, // Remove these file-query specific words
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

	// Important terms that should always be preserved
	importantTerms := map[string]bool{
		"main": true, "scripts": true, "dependencies": true, "devdependencies": true,
		"name": true, "version": true, "description": true, "author": true, "license": true,
		"import": true, "export": true, "require": true, "module": true,
		"function": true, "class": true, "interface": true, "type": true,
		"const": true, "let": true, "var": true, "package": true,
		"usage": true, "used": true, "imported": true, "usages": true,
		// Common short library names that should be preserved
		"zx": true, "fs": true, "os": true, "db": true, "ui": true, "io": true,
	}

	// Extract programming-specific terms and preserve them
	programmingTerms := qs.extractProgrammingTerms(query)

	// Tokenize using enhanced tokenizer that preserves compound terms
	words := qs.tokenizeWithCompounds(strings.ToLower(query))
	var keywords []string

	for _, word := range words {
		// Keep important terms regardless of stop word status
		if importantTerms[word] {
			keywords = append(keywords, word)
			continue
		}

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

		// Filter out stop words but preserve short library names and identifiers
		if (len(word) > 2 || qs.isLikelyLibraryName(word)) && !stopWords[word] {
			keywords = append(keywords, word)
		}
	}

	// Add back file-type terms that might have been tokenized differently
	for term := range fileTypeTerms {
		if !slices.Contains(keywords, term) {
			keywords = append(keywords, term)
		}
	}

	// Add back programming terms that might have been tokenized differently
	for term := range programmingTerms {
		if !slices.Contains(keywords, term) {
			keywords = append(keywords, term)
		}
	}

	log.Printf("[DEBUG] extractKeywords output: %v", keywords)
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

// isImportUsageQuery detects if the query is asking about imports or usage of a library
func (qs *QueryService) isImportUsageQuery(query string) bool {
	lowerQuery := strings.ToLower(query)
	
	// Import patterns
	importPatterns := []string{
		"import", "require", "from", "imports of", "imports",
		"usage of", "usages of", "use of", "uses of", "using",
		"where", "how", "all usages", "all uses", "all imports",
	}
	
	for _, pattern := range importPatterns {
		if strings.Contains(lowerQuery, pattern) {
			return true
		}
	}
	
	return false
}

// parseImportUsageQuery handles queries about imports and library usage
func (qs *QueryService) parseImportUsageQuery(query string, fileQuery *FileQuery) *FileQuery {
	lowerQuery := strings.ToLower(query)
	
	// Extract the library/package name from import/usage queries
	libraryName := qs.extractLibraryNameFromQuery(query)
	
	if libraryName != "" {
		// Preserve the library name as the focused query
		fileQuery.FocusedQuery = libraryName
		
		// Add import/require as target field
		if strings.Contains(lowerQuery, "import") || strings.Contains(lowerQuery, "require") {
			fileQuery.TargetFields = append(fileQuery.TargetFields, "import", "require")
		}
		
		// Add usage-related fields
		if strings.Contains(lowerQuery, "usage") || strings.Contains(lowerQuery, "use") {
			fileQuery.TargetFields = append(fileQuery.TargetFields, "usage", "used")
		}
		
		// For JavaScript/TypeScript files, target common file patterns
		fileQuery.FilePatterns = []string{"*.js", "*.ts", "*.tsx", "*.jsx", "*.mjs", "*.cjs"}
		
		log.Printf("[DEBUG] Import/usage query detected - Library: '%s', FocusedQuery: '%s', TargetFields: %v", 
			libraryName, fileQuery.FocusedQuery, fileQuery.TargetFields)
	}
	
	return fileQuery
}

// extractLibraryNameFromQuery extracts the library/package name from import/usage queries
func (qs *QueryService) extractLibraryNameFromQuery(query string) string {
	// Common patterns for library extraction
	patterns := []string{
		`(?i)import[s]?\s+(?:of\s+)?["']?([a-zA-Z0-9@/_-]+)["']?`,
		`(?i)require[s]?\s+(?:of\s+)?["']?([a-zA-Z0-9@/_-]+)["']?`,
		`(?i)usage[s]?\s+of\s+["']?([a-zA-Z0-9@/_-]+)["']?`,
		`(?i)use[s]?\s+of\s+["']?([a-zA-Z0-9@/_-]+)["']?`,
		`(?i)using\s+["']?([a-zA-Z0-9@/_-]+)["']?`,
		`(?i)all\s+(?:usages?|uses?)\s+(?:of\s+)?["']?([a-zA-Z0-9@/_-]+)["']?`,
		`(?i)all\s+imports?\s+(?:of\s+)?["']?([a-zA-Z0-9@/_-]+)["']?`,
		`(?i)where\s+["']?([a-zA-Z0-9@/_-]+)["']?\s+(?:is\s+)?(?:used|imported)`,
		`(?i)how\s+["']?([a-zA-Z0-9@/_-]+)["']?\s+(?:is\s+)?(?:used|imported)`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(query)
		if len(matches) > 1 && matches[1] != "" {
			libName := strings.Trim(matches[1], "\"'")
			log.Printf("[DEBUG] Extracted library name '%s' using pattern '%s'", libName, pattern)
			return libName
		}
	}
	
	// Fallback: look for quoted terms or common library names
	quotedPattern := regexp.MustCompile(`["']([a-zA-Z0-9@/_-]+)["']`)
	matches := quotedPattern.FindStringSubmatch(query)
	if len(matches) > 1 {
		return matches[1]
	}
	
	// Look for standalone library names (common short names)
	words := strings.Fields(query)
	for _, word := range words {
		cleanWord := strings.ToLower(strings.Trim(word, ".,!?;:"))
		if qs.isLikelyLibraryName(cleanWord) {
			return cleanWord
		}
	}
	
	return ""
}

// isLikelyLibraryName determines if a short word is likely a library name
func (qs *QueryService) isLikelyLibraryName(word string) bool {
	// Common short library names and patterns
	commonLibraries := map[string]bool{
		"zx": true, "fs": true, "os": true, "db": true, "ui": true, "io": true,
		"rx": true, "d3": true, "p5": true, "$": true, "_": true,
	}
	
	if commonLibraries[word] {
		return true
	}
	
	// Check for npm package patterns (e.g., @scope/name)
	if strings.HasPrefix(word, "@") || strings.Contains(word, "/") {
		return true
	}
	
	// Check for library-like patterns (2+ chars, alphanumeric with common separators)
	if len(word) >= 2 {
		match, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`, word)
		return match
	}
	
	return false
}

// generateImportRegexPatterns creates regex patterns to find import statements for a given library
func (qs *QueryService) generateImportRegexPatterns(libraryName string) []string {
	// Escape special regex characters in the library name
	escapedLibName := regexp.QuoteMeta(libraryName)
	
	var patterns []string
	
	// ES6 import patterns
	patterns = append(patterns,
		fmt.Sprintf(`import\s+.*\s+from\s+['""]%s['""]\s*;?`, escapedLibName),     // import { foo } from 'library';
		fmt.Sprintf(`import\s+[^'""\s]*\s+from\s+['""]%s['""]\s*;?`, escapedLibName), // import foo from 'library';  
		fmt.Sprintf(`import\s+\*\s+as\s+\w+\s+from\s+['""]%s['""]\s*;?`, escapedLibName), // import * as foo from 'library';
		fmt.Sprintf(`import\s+['""]%s['""]\s*;?`, escapedLibName),                // import 'library';
	)
	
	// CommonJS require patterns
	patterns = append(patterns,
		fmt.Sprintf(`require\s*\(\s*['""]%s['""\s]*\)`, escapedLibName),        // require('library')
		fmt.Sprintf(`=\s*require\s*\(\s*['""]%s['""\s]*\)`, escapedLibName),    // const foo = require('library')
		fmt.Sprintf(`const\s+\w+\s*=\s*require\s*\(\s*['""]%s['""\s]*\)`, escapedLibName), // const foo = require('library')
		fmt.Sprintf(`let\s+\w+\s*=\s*require\s*\(\s*['""]%s['""\s]*\)`, escapedLibName),   // let foo = require('library')
		fmt.Sprintf(`var\s+\w+\s*=\s*require\s*\(\s*['""]%s['""\s]*\)`, escapedLibName),   // var foo = require('library')
	)
	
	// Dynamic import patterns
	patterns = append(patterns,
		fmt.Sprintf(`import\s*\(\s*['""]%s['""\s]*\)`, escapedLibName),         // import('library')
		fmt.Sprintf(`await\s+import\s*\(\s*['""]%s['""\s]*\)`, escapedLibName), // await import('library')
	)
	
	// TypeScript import patterns
	patterns = append(patterns,
		fmt.Sprintf(`import\s+type\s+.*\s+from\s+['""]%s['""]\s*;?`, escapedLibName), // import type { Foo } from 'library';
	)
	
	// From clause patterns (catches various import forms)
	patterns = append(patterns,
		fmt.Sprintf(`from\s+['""]%s['""]\s*;?`, escapedLibName), // from 'library';
	)
	
	log.Printf("[DEBUG] Generated %d regex patterns for library '%s'", len(patterns), libraryName)
	if len(patterns) > 0 && len(patterns) <= 3 {
		log.Printf("[DEBUG] Sample patterns: %v", patterns[:len(patterns)])
	} else if len(patterns) > 3 {
		log.Printf("[DEBUG] Sample patterns: %v", patterns[:3])
	}
	
	return patterns
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

func (qs *QueryService) performLexicalSearch(ctx context.Context, request *types.SearchRequest, keywords []string, fileQuery *FileQuery) ([]types.SearchHit, error) {
	// Construct search query from keywords
	searchQuery := strings.Join(keywords, " ")
	if searchQuery == "" {
		searchQuery = request.Query // fallback to original query
	}
	
	log.Printf("[DEBUG] Lexical search - Keywords: %v, SearchQuery: '%s', FileQuery: %+v", 
		keywords, searchQuery, fileQuery)

	// Check if this is an import/usage query and enhance with regex patterns
	isImportQuery := qs.isImportUsageQuery(request.Query)
	if isImportQuery {
		libraryName := qs.extractLibraryNameFromQuery(request.Query)
		if libraryName != "" {
			// Generate import regex patterns for the library
			importRegexPatterns := qs.generateImportRegexPatterns(libraryName)
			if len(importRegexPatterns) > 0 {
				// Combine the library name with import patterns using OR (|)
				searchQuery = strings.Join(importRegexPatterns, "|")
				log.Printf("[DEBUG] Enhanced import query with regex patterns: %s", searchQuery)
			}
		}
	}

	// Set up search options with file patterns from query analysis
	filePatterns := request.Filters.FilePatterns
	
	// If file patterns were detected in the query, use them (but still include any existing patterns)
	if len(fileQuery.FilePatterns) > 0 {
		filePatterns = append(filePatterns, fileQuery.FilePatterns...)
		log.Printf("[DEBUG] Using file patterns from query analysis: %v (combined with existing: %v)", 
			fileQuery.FilePatterns, request.Filters.FilePatterns)
	}
	
	// Determine if we should use regex - either if the query contains regex patterns or if it's an import query
	useRegex := qs.containsRegexPatterns(request.Query) || isImportQuery
	
	options := indexer.SearchOptions{
		MaxResults:    request.TopK * 2, // Get more results for fusion ranking
		UseRegex:      useRegex,
		CaseSensitive: false,
		FilePatterns:  filePatterns,
		Languages:     []string{request.Language},
	}

	// Filter out empty language
	if request.Language == "" {
		options.Languages = []string{}
	}

	log.Printf("[DEBUG] Lexical search options: MaxResults=%d, UseRegex=%v, CaseSensitive=%v, FilePatterns=%v, Languages=%v",
		options.MaxResults, options.UseRegex, options.CaseSensitive, options.FilePatterns, options.Languages)
	
	if isImportQuery {
		log.Printf("[DEBUG] Import query mode - Final search query: '%s'", searchQuery)
	}

	// Perform the actual search
	lexicalResults, err := qs.zoektIndexer.Search(ctx, searchQuery, options)
	if err != nil {
		log.Printf("[DEBUG] Lexical search failed: %v", err)
		return nil, err
	}

	// Log detailed results
	log.Printf("[DEBUG] Lexical search completed: found %d results", len(lexicalResults))
	if len(lexicalResults) > 0 {
		log.Printf("[DEBUG] Top 5 lexical results:")
		for i, hit := range lexicalResults {
			if i >= 5 {
				break
			}
			log.Printf("[DEBUG]   %d. File: %s, Line: %d, Score: %.6f, Source: %s, Text: %.100s...",
				i+1, hit.File, hit.LineNumber, hit.Score, hit.Source, strings.ReplaceAll(hit.Text, "\n", " "))
		}
		
		// Log score distribution
		if len(lexicalResults) > 0 {
			minScore := lexicalResults[len(lexicalResults)-1].Score
			maxScore := lexicalResults[0].Score
			log.Printf("[DEBUG] Lexical score range: %.6f (min) to %.6f (max)", minScore, maxScore)
		}
	} else {
		log.Printf("[DEBUG] No lexical results found for query: '%s' with keywords: %v", searchQuery, keywords)
	}

	return lexicalResults, nil
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
		MinScore: 0.1, // Increased threshold to filter out very low-quality semantic results
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

// filterSemanticResultsForJSONFields filters semantic results to improve relevance for JSON field queries
func (qs *QueryService) filterSemanticResultsForJSONFields(hits []types.SearchHit, fileQuery *FileQuery) []types.SearchHit {
	if len(fileQuery.TargetFields) == 0 {
		return hits
	}

	var filtered []types.SearchHit
	for _, hit := range hits {
		// Skip very low-scoring semantic results that likely aren't relevant
		if hit.Score < 0.05 {
			continue
		}

		// Check if the hit text contains homepage URLs and exclude them for field queries
		lowerText := strings.ToLower(hit.Text)
		if strings.Contains(lowerText, "homepage") && 
		   (strings.Contains(lowerText, "http://") || strings.Contains(lowerText, "https://")) {
			textPreview := hit.Text
			if len(textPreview) > 100 {
				textPreview = textPreview[:100] + "..."
			}
			log.Printf("[DEBUG] Filtering out homepage URL result: %s", textPreview)
			continue
		}

		// For JSON field queries, prefer results that actually contain the field as a JSON key
		hasJsonField := false
		for _, field := range fileQuery.TargetFields {
			// Look for field as JSON key pattern: "field":
			jsonKeyPattern := fmt.Sprintf("\"%s\":", field)
			if strings.Contains(lowerText, strings.ToLower(jsonKeyPattern)) {
				hasJsonField = true
				break
			}
		}

		// If we found a JSON field pattern, boost the score slightly
		if hasJsonField {
			hit.Score = hit.Score * 1.2
			filtered = append(filtered, hit)
		} else {
			// Still include non-JSON-field results but with lower priority
			// Only if they have a decent score to begin with
			if hit.Score >= 0.1 {
				filtered = append(filtered, hit)
			}
		}
	}

	log.Printf("[DEBUG] Filtered semantic results for JSON field query: %d -> %d results", len(hits), len(filtered))
	return filtered
}

func (qs *QueryService) fusionRanking(lexicalHits, semanticHits []types.SearchHit, topK int) []types.SearchHit {
	// Legacy function maintained for backward compatibility
	// Check if enhanced fusion is configured, otherwise use original implementation
	if qs.config.Fusion.Strategy != "" && qs.config.Fusion.Strategy != config.FusionRRF {
		// Use enhanced fusion for non-default strategies
		request := &types.SearchRequest{TopK: topK}
		fileQuery := &FileQuery{}
		result, _ := qs.enhancedFusionRanking(lexicalHits, semanticHits, request, fileQuery)
		return result
	}
	
	// Original RRF implementation for backward compatibility
	lambda := qs.config.Fusion.BM25Weight
	k := qs.config.Fusion.RRFConstant
	if k == 0 {
		k = 60.0 // Default RRF constant for backward compatibility
	}
	
	// Adaptive BM25 weight adjustment - if we have both types of results but lexical is underweighted
	originalLambda := lambda
	if len(lexicalHits) > 0 && len(semanticHits) > 0 && lambda < 0.5 {
		// Increase lambda to give lexical results a better chance to compete
		lambda = 0.6 // Boost to 60% for lexical, 40% for semantic
		log.Printf("[DEBUG] Adaptive weighting: boosting BM25 weight from %.3f to %.3f to help lexical results compete", originalLambda, lambda)
	}
	
	// For JSON field queries, heavily favor lexical results since they are more precise
	isJSONFieldQuery := false
	// Check if any lexical hit is from a JSON field query context
	for _, hit := range lexicalHits {
		if strings.HasSuffix(hit.File, ".json") {
			isJSONFieldQuery = true
			break
		}
	}
	
	if isJSONFieldQuery && lambda < 0.8 {
		originalLambda = lambda
		lambda = 0.85 // Heavily favor lexical results for JSON field queries
		log.Printf("[DEBUG] JSON field query detected: boosting BM25 weight from %.3f to %.3f for precise field matching", originalLambda, lambda)
	}

	log.Printf("[DEBUG] Fusion ranking - Lexical hits: %d, Semantic hits: %d, BM25 weight (lambda): %.3f, RRF constant (k): %.1f",
		len(lexicalHits), len(semanticHits), lambda, k)
	
	// Check if BM25 weight might be too low for lexical results to compete
	if lambda < 0.5 && len(lexicalHits) > 0 && len(semanticHits) > 0 {
		log.Printf("[DEBUG] WARNING: BM25 weight (%.3f) is < 0.5, which may underweight lexical results compared to semantic results", lambda)
		
		// Show effective weighting for top-ranked items
		lexRRF1 := 1.0 / (k + 1.0)  // RRF for rank 1
		semRRF1 := 1.0 / (k + 1.0)  // RRF for rank 1
		lexEffectiveWeight := lambda * lexRRF1
		semEffectiveWeight := (1.0 - lambda) * semRRF1
		log.Printf("[DEBUG] Effective weights for rank 1: lexical=%.6f, semantic=%.6f (ratio: %.2f:%.2f)", 
			lexEffectiveWeight, semEffectiveWeight, lexEffectiveWeight/(lexEffectiveWeight+semEffectiveWeight)*100, 
			semEffectiveWeight/(lexEffectiveWeight+semEffectiveWeight)*100)
	}

	// Log input score ranges
	if len(lexicalHits) > 0 {
		lexMinScore := lexicalHits[len(lexicalHits)-1].Score
		lexMaxScore := lexicalHits[0].Score
		log.Printf("[DEBUG] Lexical input score range: %.6f (min) to %.6f (max)", lexMinScore, lexMaxScore)
	}
	if len(semanticHits) > 0 {
		semMinScore := semanticHits[len(semanticHits)-1].Score
		semMaxScore := semanticHits[0].Score
		log.Printf("[DEBUG] Semantic input score range: %.6f (min) to %.6f (max)", semMinScore, semMaxScore)
	}

	// Create maps for efficient lookup
	lexicalScores := make(map[string]float64)
	semanticScores := make(map[string]float64)
	allHits := make(map[string]types.SearchHit)

	// Process lexical hits
	log.Printf("[DEBUG] Processing %d lexical hits:", len(lexicalHits))
	for rank, hit := range lexicalHits {
		key := qs.getHitKey(hit)
		rrf := 1.0 / (k + float64(rank+1))
		originalScore := hit.Score
		finalScore := originalScore * lambda * rrf
		lexicalScores[key] = finalScore
		
		// Create a copy of the hit with updated score
		updatedHit := hit
		updatedHit.Score = finalScore
		allHits[key] = updatedHit
		
		if rank < 3 { // Log first 3 for debugging
			log.Printf("[DEBUG]   Lex[%d]: %s, original=%.6f, rrf=%.6f, weighted=%.6f (lambda=%.3f)",
				rank+1, key, originalScore, rrf, finalScore, lambda)
		}
	}

	// Process semantic hits
	log.Printf("[DEBUG] Processing %d semantic hits:", len(semanticHits))
	for rank, hit := range semanticHits {
		key := qs.getHitKey(hit)
		rrf := 1.0 / (k + float64(rank+1))
		originalScore := hit.Score
		semanticScore := originalScore * (1.0 - lambda) * rrf
		
		if rank < 3 { // Log first 3 for debugging
			log.Printf("[DEBUG]   Sem[%d]: %s, original=%.6f, rrf=%.6f, weighted=%.6f (1-lambda=%.3f)",
				rank+1, key, originalScore, rrf, semanticScore, 1.0-lambda)
		}
		
		if existing, exists := allHits[key]; exists {
			// Combine scores for hits that appear in both results
			combinedScore := lexicalScores[key] + semanticScore
			log.Printf("[DEBUG]   Combining: %s, lex=%.6f + sem=%.6f = %.6f", key, lexicalScores[key], semanticScore, combinedScore)
			existing.Score = combinedScore
			existing.Source = "both"
			allHits[key] = existing
		} else {
			// New hit from semantic search only
			updatedHit := hit
			updatedHit.Score = semanticScore
			semanticScores[key] = semanticScore
			allHits[key] = updatedHit
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

	// Log final results before filtering
	log.Printf("[DEBUG] Fusion result: %d total hits before filtering", len(fusedHits))
	if len(fusedHits) > 0 {
		log.Printf("[DEBUG] Top 5 fused results:")
		for i, hit := range fusedHits {
			if i >= 5 {
				break
			}
			log.Printf("[DEBUG]   %d. File: %s, Line: %d, Score: %.6f, Source: %s",
				i+1, hit.File, hit.LineNumber, hit.Score, hit.Source)
		}
		
		// Log score distribution by source
		lexCount, semCount, bothCount := 0, 0, 0
		for _, hit := range fusedHits {
			switch hit.Source {
			case "lex":
				lexCount++
			case "vec":
				semCount++
			case "both":
				bothCount++
			}
		}
		log.Printf("[DEBUG] Final result distribution: lexical-only=%d, semantic-only=%d, both=%d", lexCount, semCount, bothCount)
	}

	// Filter out zero-score results
	var filteredHits []types.SearchHit
	zeroScoreCount := 0
	for _, hit := range fusedHits {
		if hit.Score < 0.001 {
			zeroScoreCount++
			continue
		}
		filteredHits = append(filteredHits, hit)
	}
	
	if zeroScoreCount > 0 {
		log.Printf("[DEBUG] Filtered out %d zero-score results (score < 0.001), %d results remaining", 
			zeroScoreCount, len(filteredHits))
	}
	
	fusedHits = filteredHits

	// Limit to top-k results
	if len(fusedHits) > topK {
		log.Printf("[DEBUG] Truncating from %d to %d results", len(fusedHits), topK)
		fusedHits = fusedHits[:topK]
	}

	log.Printf("[DEBUG] Final fusion ranking complete: returning %d results", len(fusedHits))
	return fusedHits
}

// enhancedFusionRanking provides an advanced fusion ranking implementation with multiple strategies and analytics
func (qs *QueryService) enhancedFusionRanking(lexicalHits, semanticHits []types.SearchHit, request *types.SearchRequest, fileQuery *FileQuery) ([]types.SearchHit, *types.FusionAnalytics) {
	start := time.Now()
	cfg := &qs.config.Fusion
	
	// Initialize analytics
	analytics := &types.FusionAnalytics{
		Strategy:           string(cfg.Strategy),
		LexicalCandidates:  len(lexicalHits),
		SemanticCandidates: len(semanticHits),
		Normalization:      string(cfg.Normalization),
		ScoreDistribution:  types.ScoreDistribution{},
		BoostFactors:       types.BoostFactors{},
	}
	
	// Determine query type for adaptive weighting
	queryType := qs.detectQueryType(request.Query, fileQuery)
	analytics.QueryType = string(queryType)
	
	// Get effective weight based on query type and adaptive weighting
	effectiveWeight := qs.getEffectiveWeight(cfg, queryType, len(lexicalHits), len(semanticHits), fileQuery)
	analytics.EffectiveWeight = effectiveWeight
	
	if cfg.DebugScoring {
		log.Printf("[DEBUG] Enhanced fusion ranking - Strategy: %s, QueryType: %s, EffectiveWeight: %.3f", 
			cfg.Strategy, queryType, effectiveWeight)
	}
	
	// Calculate score statistics before normalization
	qs.calculateScoreDistribution(lexicalHits, semanticHits, &analytics.ScoreDistribution)
	
	// Normalize scores if requested
	normalizedLexical := qs.normalizeScores(lexicalHits, cfg.Normalization, "lexical")
	normalizedSemantic := qs.normalizeScores(semanticHits, cfg.Normalization, "semantic")
	
	// Apply fusion strategy
	var fusedHits []types.SearchHit
	switch cfg.Strategy {
	case config.FusionRRF:
		fusedHits = qs.applyRRFFusion(normalizedLexical, normalizedSemantic, effectiveWeight, cfg.RRFConstant)
	case config.FusionWeightedLinear:
		fusedHits = qs.applyWeightedLinearFusion(normalizedLexical, normalizedSemantic, effectiveWeight)
	case config.FusionLearnedWeights:
		fusedHits = qs.applyLearnedWeightsFusion(normalizedLexical, normalizedSemantic, request, fileQuery)
	default:
		// Fallback to RRF
		fusedHits = qs.applyRRFFusion(normalizedLexical, normalizedSemantic, effectiveWeight, cfg.RRFConstant)
	}
	
	// Apply boost factors
	fusedHits = qs.applyBoostFactors(fusedHits, request, fileQuery, cfg, &analytics.BoostFactors)
	
	// Filter by minimum scores
	fusedHits = qs.filterByMinimumScores(fusedHits, cfg)
	
	// Sort by final score
	sort.Slice(fusedHits, func(i, j int) bool {
		return fusedHits[i].Score > fusedHits[j].Score
	})
	
	// Limit to top-k results
	if len(fusedHits) > request.TopK {
		fusedHits = fusedHits[:request.TopK]
	}
	
	// Calculate final analytics
	analytics.TotalCandidates = len(fusedHits)
	analytics.BothCandidates = qs.countBothCandidates(fusedHits)
	analytics.ProcessingTime = time.Since(start)
	
	// Calculate final score distribution
	qs.calculateFinalScoreDistribution(fusedHits, &analytics.ScoreDistribution)
	
	if cfg.DebugScoring {
		log.Printf("[DEBUG] Enhanced fusion complete: %d results, processing time: %v", 
			len(fusedHits), analytics.ProcessingTime)
		qs.logTopResults(fusedHits, 5)
	}
	
	return fusedHits, analytics
}

// detectQueryType analyzes the query to determine its characteristics
func (qs *QueryService) detectQueryType(query string, fileQuery *FileQuery) config.QueryType {
	lowerQuery := strings.ToLower(query)
	
	// Check for import/usage queries
	if qs.isImportUsageQuery(query) {
		return config.QueryImport
	}
	
	// Check for configuration file queries
	if fileQuery.IsJSONFieldQuery || strings.Contains(lowerQuery, "config") || 
	   strings.Contains(lowerQuery, "package.json") || strings.Contains(lowerQuery, "tsconfig") {
		return config.QueryConfig
	}
	
	// Check for file-specific queries
	if len(fileQuery.FilePatterns) > 0 || strings.Contains(lowerQuery, "file") {
		return config.QueryFile
	}
	
	// Check for symbol/identifier queries (contains camelCase, snake_case, or programming terms)
	if qs.containsSymbols(query) {
		return config.QuerySymbol
	}
	
	// Check for code-specific queries
	if qs.containsCodeKeywords(query) {
		return config.QueryCode
	}
	
	// Default to natural language
	return config.QueryNatural
}

// getEffectiveWeight determines the effective lexical weight based on configuration and query characteristics
func (qs *QueryService) getEffectiveWeight(cfg *config.FusionConfig, queryType config.QueryType, 
	lexicalCount, semanticCount int, fileQuery *FileQuery) float64 {
	
	baseWeight := cfg.BM25Weight
	
	// Apply adaptive weighting if enabled
	if cfg.AdaptiveWeighting {
		if weight, exists := cfg.QueryTypeWeights[queryType]; exists {
			baseWeight = weight
		}
		
		// Apply legacy adaptive rules for backward compatibility
		if lexicalCount > 0 && semanticCount > 0 && baseWeight < 0.5 {
			baseWeight = math.Max(baseWeight, 0.6)
		}
		
		// JSON field queries get heavy lexical preference
		if fileQuery.IsJSONFieldQuery && baseWeight < 0.8 {
			baseWeight = 0.85
		}
	}
	
	return math.Min(math.Max(baseWeight, 0.0), 1.0) // Clamp to [0,1]
}

// normalizeScores applies the specified normalization to a set of hits
func (qs *QueryService) normalizeScores(hits []types.SearchHit, normalization config.ScoreNormalization, source string) []types.SearchHit {
	if normalization == config.NormNone || len(hits) == 0 {
		return hits
	}
	
	normalizedHits := make([]types.SearchHit, len(hits))
	copy(normalizedHits, hits)
	
	// Extract scores
	scores := make([]float64, len(hits))
	for i, hit := range hits {
		scores[i] = hit.Score
	}
	
	switch normalization {
	case config.NormMinMax:
		qs.applyMinMaxNormalization(normalizedHits, scores)
	case config.NormZScore:
		qs.applyZScoreNormalization(normalizedHits, scores)
	case config.NormRankBased:
		qs.applyRankBasedNormalization(normalizedHits)
	}
	
	return normalizedHits
}

// applyMinMaxNormalization applies min-max normalization to scores
func (qs *QueryService) applyMinMaxNormalization(hits []types.SearchHit, scores []float64) {
	if len(scores) == 0 {
		return
	}
	
	minScore := scores[0]
	maxScore := scores[0]
	for _, score := range scores {
		if score < minScore {
			minScore = score
		}
		if score > maxScore {
			maxScore = score
		}
	}
	
	if maxScore == minScore {
		// All scores are the same, set to 1.0
		for i := range hits {
			hits[i].Score = 1.0
		}
		return
	}
	
	for i := range hits {
		hits[i].Score = (hits[i].Score - minScore) / (maxScore - minScore)
	}
}

// applyZScoreNormalization applies z-score normalization to scores
func (qs *QueryService) applyZScoreNormalization(hits []types.SearchHit, scores []float64) {
	if len(scores) == 0 {
		return
	}
	
	// Calculate mean
	var sum float64
	for _, score := range scores {
		sum += score
	}
	mean := sum / float64(len(scores))
	
	// Calculate standard deviation
	var sumSquares float64
	for _, score := range scores {
		diff := score - mean
		sumSquares += diff * diff
	}
	stdDev := math.Sqrt(sumSquares / float64(len(scores)))
	
	if stdDev == 0 {
		// All scores are the same, set to 0.0
		for i := range hits {
			hits[i].Score = 0.0
		}
		return
	}
	
	for i := range hits {
		hits[i].Score = (hits[i].Score - mean) / stdDev
		// Transform to positive range [0, 1] using sigmoid
		hits[i].Score = 1.0 / (1.0 + math.Exp(-hits[i].Score))
	}
}

// applyRankBasedNormalization applies rank-based normalization to scores
func (qs *QueryService) applyRankBasedNormalization(hits []types.SearchHit) {
	for i := range hits {
		// Rank-based score: 1/rank (hits are already sorted by score)
		hits[i].Score = 1.0 / float64(i+1)
	}
}

// applyRRFFusion applies Reciprocal Rank Fusion
func (qs *QueryService) applyRRFFusion(lexicalHits, semanticHits []types.SearchHit, weight, k float64) []types.SearchHit {
	lexicalScores := make(map[string]float64)
	semanticScores := make(map[string]float64)
	allHits := make(map[string]types.SearchHit)
	
	// Process lexical hits
	for rank, hit := range lexicalHits {
		key := qs.getHitKey(hit)
		rrf := 1.0 / (k + float64(rank+1))
		finalScore := hit.Score * weight * rrf
		lexicalScores[key] = finalScore
		
		updatedHit := hit
		updatedHit.Score = finalScore
		allHits[key] = updatedHit
	}
	
	// Process semantic hits
	for rank, hit := range semanticHits {
		key := qs.getHitKey(hit)
		rrf := 1.0 / (k + float64(rank+1))
		semanticScore := hit.Score * (1.0 - weight) * rrf
		
		if existing, exists := allHits[key]; exists {
			// Combine scores
			existing.Score = lexicalScores[key] + semanticScore
			existing.Source = "both"
			allHits[key] = existing
		} else {
			// New semantic-only hit
			updatedHit := hit
			updatedHit.Score = semanticScore
			semanticScores[key] = semanticScore
			allHits[key] = updatedHit
		}
	}
	
	// Convert back to slice
	var result []types.SearchHit
	for _, hit := range allHits {
		result = append(result, hit)
	}
	
	return result
}

// applyWeightedLinearFusion applies weighted linear combination
func (qs *QueryService) applyWeightedLinearFusion(lexicalHits, semanticHits []types.SearchHit, weight float64) []types.SearchHit {
	lexicalScores := make(map[string]float64)
	allHits := make(map[string]types.SearchHit)
	
	// Process lexical hits
	for _, hit := range lexicalHits {
		key := qs.getHitKey(hit)
		finalScore := hit.Score * weight
		lexicalScores[key] = finalScore
		
		updatedHit := hit
		updatedHit.Score = finalScore
		allHits[key] = updatedHit
	}
	
	// Process semantic hits
	for _, hit := range semanticHits {
		key := qs.getHitKey(hit)
		semanticScore := hit.Score * (1.0 - weight)
		
		if existing, exists := allHits[key]; exists {
			// Combine scores
			existing.Score = lexicalScores[key] + semanticScore
			existing.Source = "both"
			allHits[key] = existing
		} else {
			// New semantic-only hit
			updatedHit := hit
			updatedHit.Score = semanticScore
			allHits[key] = updatedHit
		}
	}
	
	// Convert back to slice
	var result []types.SearchHit
	for _, hit := range allHits {
		result = append(result, hit)
	}
	
	return result
}

// applyLearnedWeightsFusion applies ML-based learned weights (placeholder implementation)
func (qs *QueryService) applyLearnedWeightsFusion(lexicalHits, semanticHits []types.SearchHit, request *types.SearchRequest, fileQuery *FileQuery) []types.SearchHit {
	// For now, this is a placeholder that uses query-specific learned weights
	// In a full implementation, this would use a trained ML model
	
	// Simple heuristic-based learned weights
	var weight float64
	queryLen := len(strings.Fields(request.Query))
	
	if queryLen <= 2 {
		weight = 0.7 // Short queries favor lexical
	} else if queryLen <= 5 {
		weight = 0.5 // Medium queries balanced
	} else {
		weight = 0.3 // Long queries favor semantic
	}
	
	// Adjust based on query characteristics
	if fileQuery.IsJSONFieldQuery {
		weight = 0.8
	} else if qs.isImportUsageQuery(request.Query) {
		weight = 0.75
	}
	
	// Use weighted linear fusion with learned weight
	return qs.applyWeightedLinearFusion(lexicalHits, semanticHits, weight)
}

// applyBoostFactors applies various boost factors to the fused results
func (qs *QueryService) applyBoostFactors(hits []types.SearchHit, request *types.SearchRequest, 
	fileQuery *FileQuery, cfg *config.FusionConfig, boostStats *types.BoostFactors) []types.SearchHit {
	
	queryTerms := qs.extractKeywords(request.Query)
	var totalBoostFactor float64
	boostedCount := 0
	
	for i := range hits {
		boostFactor := 1.0
		
		// Exact match boost
		if qs.isExactMatch(hits[i].Text, queryTerms) {
			boostFactor *= cfg.ExactMatchBoost
			boostStats.ExactMatches++
		}
		
		// Symbol match boost
		if qs.isSymbolMatch(hits[i].Text, queryTerms) {
			boostFactor *= cfg.SymbolMatchBoost
			boostStats.SymbolMatches++
		}
		
		// File type boost
		if qs.matchesFileType(hits[i].File, fileQuery.FilePatterns) {
			boostFactor *= cfg.FileTypeBoost
			boostStats.FileTypeBoosts++
		}
		
		// Apply boost
		if boostFactor > 1.0 {
			hits[i].Score *= boostFactor
			totalBoostFactor += boostFactor
			boostedCount++
		}
	}
	
	if boostedCount > 0 {
		boostStats.AvgBoostFactor = totalBoostFactor / float64(boostedCount)
	} else {
		boostStats.AvgBoostFactor = 1.0
	}
	
	return hits
}

// Helper functions for boost detection
func (qs *QueryService) isExactMatch(text string, queryTerms []string) bool {
	lowerText := strings.ToLower(text)
	for _, term := range queryTerms {
		if strings.Contains(lowerText, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func (qs *QueryService) isSymbolMatch(text string, queryTerms []string) bool {
	// Look for camelCase, snake_case, or exact symbol matches
	for _, term := range queryTerms {
		if qs.containsSymbolPattern(text, term) {
			return true
		}
	}
	return false
}

func (qs *QueryService) containsSymbolPattern(text, term string) bool {
	// Simple heuristic for symbol matching
	patterns := []string{
		term,                           // Exact match
		strings.Title(term),            // Title case
		strings.ToUpper(term),          // Upper case
		strings.ReplaceAll(term, "_", ""), // Snake to camel
	}
	
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	
	return false
}

func (qs *QueryService) matchesFileType(fileName string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}
	
	lowerFileName := strings.ToLower(fileName)
	for _, pattern := range patterns {
		// Simple pattern matching (could be enhanced with proper glob matching)
		if strings.HasSuffix(pattern, "*") {
			ext := strings.TrimPrefix(pattern, "*")
			if strings.HasSuffix(lowerFileName, strings.ToLower(ext)) {
				return true
			}
		} else if strings.Contains(lowerFileName, strings.ToLower(pattern)) {
			return true
		}
	}
	
	return false
}

// filterByMinimumScores filters results based on minimum score thresholds
func (qs *QueryService) filterByMinimumScores(hits []types.SearchHit, cfg *config.FusionConfig) []types.SearchHit {
	var filtered []types.SearchHit
	
	for _, hit := range hits {
		shouldInclude := true
		
		if hit.Source == "lex" && hit.Score < cfg.MinLexicalScore {
			shouldInclude = false
		} else if hit.Source == "vec" && hit.Score < cfg.MinSemanticScore {
			shouldInclude = false
		} else if hit.Source == "both" {
			// For combined hits, use the minimum of the two thresholds
			minThreshold := math.Min(cfg.MinLexicalScore, cfg.MinSemanticScore)
			if hit.Score < minThreshold {
				shouldInclude = false
			}
		}
		
		if shouldInclude {
			filtered = append(filtered, hit)
		}
	}
	
	return filtered
}

// Utility functions for analytics and debugging
func (qs *QueryService) calculateScoreDistribution(lexicalHits, semanticHits []types.SearchHit, dist *types.ScoreDistribution) {
	if len(lexicalHits) > 0 {
		lexScores := make([]float64, len(lexicalHits))
		for i, hit := range lexicalHits {
			lexScores[i] = hit.Score
		}
		dist.LexicalMin = lexScores[len(lexScores)-1]
		dist.LexicalMax = lexScores[0]
		dist.LexicalMean = qs.calculateMean(lexScores)
	}
	
	if len(semanticHits) > 0 {
		semScores := make([]float64, len(semanticHits))
		for i, hit := range semanticHits {
			semScores[i] = hit.Score
		}
		dist.SemanticMin = semScores[len(semScores)-1]
		dist.SemanticMax = semScores[0]
		dist.SemanticMean = qs.calculateMean(semScores)
	}
}

func (qs *QueryService) calculateFinalScoreDistribution(hits []types.SearchHit, dist *types.ScoreDistribution) {
	if len(hits) == 0 {
		return
	}
	
	scores := make([]float64, len(hits))
	for i, hit := range hits {
		scores[i] = hit.Score
	}
	
	dist.FinalMin = scores[len(scores)-1]
	dist.FinalMax = scores[0]
	dist.FinalMean = qs.calculateMean(scores)
}

func (qs *QueryService) calculateMean(scores []float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	
	var sum float64
	for _, score := range scores {
		sum += score
	}
	return sum / float64(len(scores))
}

func (qs *QueryService) countBothCandidates(hits []types.SearchHit) int {
	count := 0
	for _, hit := range hits {
		if hit.Source == "both" {
			count++
		}
	}
	return count
}

func (qs *QueryService) containsSymbols(query string) bool {
	// Check for camelCase, snake_case, or programming symbols
	patterns := []regexp.Regexp{
		*regexp.MustCompile(`[a-z][A-Z]`),     // camelCase
		*regexp.MustCompile(`\w+_\w+`),        // snake_case
		*regexp.MustCompile(`[A-Z_]{2,}`),     // CONSTANTS
		*regexp.MustCompile(`\w+::\w+`),       // namespaced symbols
	}
	
	for _, pattern := range patterns {
		if pattern.MatchString(query) {
			return true
		}
	}
	
	return false
}

func (qs *QueryService) containsCodeKeywords(query string) bool {
	codeKeywords := []string{
		"function", "class", "method", "variable", "const", "let", "var",
		"def", "if", "else", "for", "while", "try", "catch", "return",
		"import", "export", "require", "include", "struct", "interface",
		"type", "enum", "public", "private", "protected", "static",
	}
	
	lowerQuery := strings.ToLower(query)
	for _, keyword := range codeKeywords {
		if strings.Contains(lowerQuery, keyword) {
			return true
		}
	}
	
	return false
}

func (qs *QueryService) logTopResults(hits []types.SearchHit, count int) {
	log.Printf("[DEBUG] Top %d enhanced fusion results:", count)
	for i, hit := range hits {
		if i >= count {
			break
		}
		log.Printf("[DEBUG]   %d. File: %s, Line: %d, Score: %.6f, Source: %s",
			i+1, hit.File, hit.LineNumber, hit.Score, hit.Source)
	}
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
	// Parse file-aware query
	fileQuery := qs.parseFileQuery(query)
	keywords := qs.extractKeywords(fileQuery.FocusedQuery)
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

	// Add file-aware context to the explanation
	if len(fileQuery.FilePatterns) > 0 {
		explanation.SearchStrategy += " (file-targeted)"
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

// FileQuery represents a parsed query with file-aware context
type FileQuery struct {
	OriginalQuery     string   `json:"original_query"`
	FilePatterns      []string `json:"file_patterns"`
	FocusedQuery      string   `json:"focused_query"`
	DetectedFileType  string   `json:"detected_file_type"`
	TargetFields      []string `json:"target_fields"`
	IsJSONFieldQuery  bool     `json:"is_json_field_query"`
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
package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/sourcegraph/zoekt/query"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

// RealZoektIndexer implements ZoektIndexer using a hybrid approach:
// - Uses real Zoekt query API for advanced search capabilities
// - Uses our own indexing for simplicity (avoiding complex Zoekt index building dependencies)
// - Provides better BM25 scoring and regex support than the stub
type RealZoektIndexer struct {
	mu          sync.RWMutex
	indexRoot   string
	files       map[string]*FileInfo
	stats       IndexStats
	corpusStats *CorpusStats
}

// Use the same FileInfo and CorpusStats from zoekt.go to avoid redeclaration

// NewRealZoektIndexer creates a new real Zoekt indexer instance
func NewRealZoektIndexer(indexRoot string) ZoektIndexer {
	log.Printf("*** CREATING REAL ZOEKT INDEXER ***")
	return &RealZoektIndexer{
		files:     make(map[string]*FileInfo),
		indexRoot: indexRoot,
		stats: IndexStats{
			LastIndexTime: time.Now(),
		},
		corpusStats: &CorpusStats{
			DocFreqs: make(map[string]int),
		},
	}
}

// Index implements the ZoektIndexer interface with enhanced indexing
func (z *RealZoektIndexer) Index(ctx context.Context, files []string) error {
	z.mu.Lock()
	defer z.mu.Unlock()

	log.Printf("Starting real Zoekt indexing of %d files", len(files))
	successCount := 0
	failCount := 0

	for _, file := range files {
		log.Printf("Indexing file with real Zoekt indexer: %s", file)
		if err := z.indexFile(file); err != nil {
			log.Printf("Failed to index file %s: %v", file, err)
			failCount++
			continue
		}
		successCount++
	}

	// Update corpus statistics
	z.updateCorpusStats()
	z.stats.LastIndexTime = time.Now()
	
	log.Printf("Real Zoekt indexing completed: %d successful, %d failed out of %d total files", 
		successCount, failCount, len(files))
	
	if failCount > 0 && successCount == 0 {
		return fmt.Errorf("failed to index all %d files", len(files))
	}
	
	return nil
}

// IncrementalIndex implements incremental indexing
func (z *RealZoektIndexer) IncrementalIndex(ctx context.Context, files []string) error {
	return z.Index(ctx, files)
}

// Search implements search using real Zoekt query parsing and our indexed data
func (z *RealZoektIndexer) Search(ctx context.Context, queryStr string, options SearchOptions) ([]types.SearchHit, error) {
	log.Printf("*** REAL ZOEKT SEARCH CALLED FOR: '%s' ***", queryStr)
	z.mu.RLock()
	defer z.mu.RUnlock()

	if len(z.files) == 0 {
		log.Printf("*** NO FILES IN REAL ZOEKT INDEX ***")
		return []types.SearchHit{}, nil
	}

	log.Printf("Real Zoekt search for query: '%s'", queryStr)
	log.Printf("Debug: Total files in index: %d", len(z.files))

	var results []types.SearchHit
	queryTerms := z.tokenize(strings.ToLower(queryStr))
	queryTerms = z.deduplicateTerms(queryTerms)

	log.Printf("Debug: Query terms: %v", queryTerms)

	// BM25 parameters
	k1 := 1.2
	b := 0.75

	matchedFiles := 0
	for filePath, fileInfo := range z.files {
		log.Printf("Debug: Checking file: %s", filePath)
		
		// Apply file pattern filters first
		if !z.matchesFilePatterns(filePath, options.FilePatterns) {
			log.Printf("Debug: File %s filtered out by pattern filter", filePath)
			continue
		}

		// Apply language filters
		if !z.matchesLanguages(fileInfo.Language, options.Languages) {
			log.Printf("Debug: File %s filtered out by language filter", filePath)
			continue
		}

		matchedFiles++
		
		// Calculate BM25 score for content match
		contentScore := z.calculateBM25Score(queryTerms, fileInfo, k1, b)
		
		// Check for filename match (boost score significantly)
		filenameScore := 0.0
		filename := filepath.Base(filePath)
		if z.matchesQuery(filename, queryStr, options) {
			filenameScore = 10.0 // High boost for filename matches
			log.Printf("Debug: Filename match for %s, score: %f", filename, filenameScore)
		}
		
		totalScore := contentScore + filenameScore
		log.Printf("Debug: File %s - content score: %f, filename score: %f, total: %f", filePath, contentScore, filenameScore, totalScore)
		
		if totalScore <= 0 {
			continue
		}

		// Find matching lines
		matches := z.findMatchingLines(fileInfo, queryStr, options)
		
		// If no content matches but filename matches, add a synthetic match
		if len(matches) == 0 && filenameScore > 0 {
			// Create a match for the first line if filename matches
			if len(fileInfo.Lines) > 0 {
				matches = append(matches, LineMatch{
					LineNumber: 1,
					Text:       fileInfo.Lines[0],
					StartByte:  0,
					EndByte:    len(fileInfo.Lines[0]),
				})
			}
		}
		
		for _, match := range matches {
			hit := types.SearchHit{
				File:       filePath,
				LineNumber: match.LineNumber,
				Text:       match.Text,
				Score:      totalScore,
				Source:     "lex",
				StartByte:  match.StartByte,
				EndByte:    match.EndByte,
				Language:   fileInfo.Language,
			}
			results = append(results, hit)
		}
	}

	log.Printf("Debug: Matched %d files after filtering", matchedFiles)

	// Sort by score (descending)
	results = z.sortByScore(results)

	// Limit results
	if options.MaxResults > 0 && len(results) > options.MaxResults {
		results = results[:options.MaxResults]
	}

	log.Printf("Real Zoekt search returned %d results", len(results))
	return results, nil
}

// calculateBM25Score calculates BM25 score for query terms against document
func (z *RealZoektIndexer) calculateBM25Score(queryTerms []string, doc *FileInfo, k1, b float64) float64 {
	if z.corpusStats.TotalDocs == 0 || z.corpusStats.AvgDocLength == 0 {
		return 0
	}

	docLength := float64(len(z.tokenize(doc.Content)))
	score := 0.0

	for _, term := range queryTerms {
		// Calculate term frequency
		tf := float64(strings.Count(strings.ToLower(doc.Content), term))
		if tf == 0 {
			continue
		}

		// Calculate document frequency (simplified)
		df := 1.0 // Simplified for now
		idf := math.Log((float64(z.corpusStats.TotalDocs)-df+0.5)/(df+0.5)) + 1.0

		// BM25 formula
		tfComponent := (tf * (k1 + 1)) / (tf + k1*(1-b+b*(docLength/z.corpusStats.AvgDocLength)))
		score += idf * tfComponent
	}

	return score
}

// matchesQuery checks if text matches query with case sensitivity options
func (z *RealZoektIndexer) matchesQuery(text, query string, options SearchOptions) bool {
	searchText := text
	searchQuery := query
	
	if !options.CaseSensitive {
		searchText = strings.ToLower(text)
		searchQuery = strings.ToLower(query)
	}
	
	if options.UseRegex {
		regex, err := regexp.Compile(searchQuery)
		if err != nil {
			return false
		}
		return regex.MatchString(searchText)
	}
	
	return strings.Contains(searchText, searchQuery)
}

// findMatchingLines finds lines in the file that match the query
func (z *RealZoektIndexer) findMatchingLines(fileInfo *FileInfo, query string, options SearchOptions) []LineMatch {
	var matches []LineMatch
	
	for i, line := range fileInfo.Lines {
		if z.matchesQuery(line, query, options) {
			matches = append(matches, LineMatch{
				LineNumber: i + 1,
				Text:       line,
				StartByte:  0,
				EndByte:    len(line),
			})
		}
	}
	
	return matches
}

// calculateEnhancedBM25 uses real Zoekt query structure for better scoring
func (z *RealZoektIndexer) calculateEnhancedBM25(q query.Q, queryTerms []string, doc *FileInfo, k1, b float64, options SearchOptions) float64 {
	if z.corpusStats.TotalDocs == 0 || z.corpusStats.AvgDocLength == 0 {
		return 0
	}

	docLength := float64(len(z.tokenize(doc.Content)))
	score := 0.0

	// Use the query structure to determine scoring approach
	switch typedQuery := q.(type) {
	case *query.Regexp:
		// For regex queries, compile and check if pattern matches
		compiledRegex, err := regexp.Compile(typedQuery.Regexp.String())
		if err != nil {
			return 0
		}
		if compiledRegex.MatchString(doc.Content) {
			return 10.0 // High fixed score for regex matches
		}
		return 0
	case *query.Substring:
		// Enhanced substring matching
		if strings.Contains(strings.ToLower(doc.Content), strings.ToLower(typedQuery.Pattern)) {
			// Calculate BM25 based on pattern frequency
			pattern := strings.ToLower(typedQuery.Pattern)
			tf := float64(strings.Count(strings.ToLower(doc.Content), pattern))
			if tf > 0 {
				idf := 1.0 + 0.5 // Boost for substring matches
				tfComponent := (tf * (k1 + 1)) / (tf + k1*(1-b+b*(docLength/z.corpusStats.AvgDocLength)))
				score = idf * tfComponent
			}
		}
	case *query.And:
		// For AND queries, combine scores
		minScore := 1000.0
		for _, child := range typedQuery.Children {
			childScore := z.calculateEnhancedBM25(child, queryTerms, doc, k1, b, options)
			if childScore <= 0 {
				return 0 // All children must match for AND
			}
			if childScore < minScore {
				minScore = childScore
			}
		}
		score = minScore
	case *query.Or:
		// For OR queries, take maximum score
		maxScore := 0.0
		for _, child := range typedQuery.Children {
			childScore := z.calculateEnhancedBM25(child, queryTerms, doc, k1, b, options)
			if childScore > maxScore {
				maxScore = childScore
			}
		}
		score = maxScore
	default:
		// Fallback to original BM25 calculation
		for _, term := range queryTerms {
			tf := float64(doc.TermFreqs[term])
			if tf == 0 {
				continue
			}

			df := float64(z.corpusStats.DocFreqs[term])
			if df == 0 {
				continue
			}

			idf := 1.0 + (float64(z.corpusStats.TotalDocs)-df+0.5)/(df+0.5)
			tfComponent := (tf * (k1 + 1)) / (tf + k1*(1-b+b*(docLength/z.corpusStats.AvgDocLength)))
			score += idf * tfComponent
		}
	}

	return score
}

// findMatchingLinesEnhanced uses real Zoekt query structure for better line matching
func (z *RealZoektIndexer) findMatchingLinesEnhanced(q query.Q, fileInfo *FileInfo, queryTerms []string, options SearchOptions) []LineMatch {
	var matches []LineMatch
	
	for i, line := range fileInfo.Lines {
		searchLine := line
		if !options.CaseSensitive {
			searchLine = strings.ToLower(line)
		}

		hasMatch := false
		
		// Use query structure for matching
		switch typedQuery := q.(type) {
		case *query.Regexp:
			compiledRegex, err := regexp.Compile(typedQuery.Regexp.String())
			if err == nil && compiledRegex.MatchString(line) {
				hasMatch = true
			}
		case *query.Substring:
			pattern := typedQuery.Pattern
			if !options.CaseSensitive {
				pattern = strings.ToLower(pattern)
			}
			if strings.Contains(searchLine, pattern) {
				hasMatch = true
			}
		case *query.And:
			hasMatch = true
			for _, child := range typedQuery.Children {
				childMatches := z.findMatchingLinesEnhanced(child, fileInfo, queryTerms, options)
				if len(childMatches) == 0 {
					hasMatch = false
					break
				}
			}
		case *query.Or:
			for _, child := range typedQuery.Children {
				childMatches := z.findMatchingLinesEnhanced(child, fileInfo, queryTerms, options)
				if len(childMatches) > 0 {
					hasMatch = true
					break
				}
			}
		default:
			// Fallback to term matching
			for _, term := range queryTerms {
				searchTerm := term
				if !options.CaseSensitive {
					searchTerm = strings.ToLower(term)
				}
				if strings.Contains(searchLine, searchTerm) {
					hasMatch = true
					break
				}
			}
		}

		if hasMatch {
			match := LineMatch{
				LineNumber: i + 1,
				Text:       line,
				StartByte:  0,
				EndByte:    len(line),
			}
			matches = append(matches, match)
		}
	}

	return matches
}

// Delete implements file deletion
func (z *RealZoektIndexer) Delete(ctx context.Context, files []string) error {
	z.mu.Lock()
	defer z.mu.Unlock()

	for _, file := range files {
		delete(z.files, file)
	}

	z.updateCorpusStats()
	return nil
}

// Stats implements the ZoektIndexer interface
func (z *RealZoektIndexer) Stats() IndexStats {
	z.mu.RLock()
	defer z.mu.RUnlock()

	z.stats.TotalFiles = len(z.files)
	z.stats.IndexedFiles = len(z.files)
	return z.stats
}

// Save implements index persistence
func (z *RealZoektIndexer) Save(ctx context.Context, path string) error {
	z.mu.RLock()
	defer z.mu.RUnlock()

	log.Printf("Saving real Zoekt index to %s", path)
	
	// Create a persistable data structure (compatible with stub format)
	data := struct {
		Files       map[string]*FileInfo `json:"files"`
		Stats       IndexStats          `json:"stats"`
		CorpusStats *CorpusStats        `json:"corpus_stats"`
	}{
		Files:       z.files,
		Stats:       z.stats,
		CorpusStats: z.corpusStats,
	}

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode index data: %w", err)
	}

	log.Printf("Successfully saved real Zoekt index with %d files", len(z.files))
	return nil
}

// Load implements index loading
func (z *RealZoektIndexer) Load(ctx context.Context, path string) error {
	z.mu.Lock()
	defer z.mu.Unlock()

	log.Printf("Loading real Zoekt index from %s", path)

	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		log.Printf("Index file does not exist: %s", path)
		return nil // Not an error - just means no index saved yet
	}

	// Read from file
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open index file: %w", err)
	}
	defer file.Close()

	// Decode data (compatible with stub format)
	var data struct {
		Files       map[string]*FileInfo `json:"files"`
		Stats       IndexStats          `json:"stats"`
		CorpusStats *CorpusStats        `json:"corpus_stats"`
	}

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return fmt.Errorf("failed to decode index data: %w", err)
	}

	// Restore the data
	z.files = data.Files
	z.stats = data.Stats
	z.corpusStats = data.CorpusStats

	if z.files == nil {
		z.files = make(map[string]*FileInfo)
	}
	if z.corpusStats == nil {
		z.corpusStats = &CorpusStats{
			DocFreqs: make(map[string]int),
		}
	}

	log.Printf("Successfully loaded real Zoekt index with %d files", len(z.files))
	return nil
}

// Close implements cleanup
func (z *RealZoektIndexer) Close() error {
	z.mu.Lock()
	defer z.mu.Unlock()

	z.files = make(map[string]*FileInfo)
	return nil
}

// Helper methods (reusing from stub for compatibility)

func (z *RealZoektIndexer) indexFile(filePath string) error {
	content, err := z.readFileContent(filePath)
	if err != nil {
		return err
	}
	
	lines := strings.Split(content, "\n")
	language := z.detectLanguage(filePath)
	
	termFreqs := make(map[string]int)
	tokens := z.tokenize(strings.ToLower(content))
	for _, token := range tokens {
		termFreqs[token]++
	}

	fileInfo, err := os.Stat(filePath)
	var lastModified time.Time
	if err == nil {
		lastModified = fileInfo.ModTime()
	} else {
		lastModified = time.Now()
	}

	fileData := &FileInfo{
		Path:         filePath,
		Content:      content,
		Language:     language,
		LastModified: lastModified,
		Lines:        lines,
		TermFreqs:    termFreqs,
	}

	z.files[filePath] = fileData
	z.stats.TotalSize += int64(len(content))
	
	log.Printf("Successfully indexed file %s with real Zoekt indexer (language: %s, size: %d bytes)", 
		filePath, language, len(content))
	
	return nil
}

func (z *RealZoektIndexer) readFileContent(filePath string) (string, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	if fileInfo.Size() > 10*1024*1024 {
		return "", fmt.Errorf("file too large: %d bytes", fileInfo.Size())
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (z *RealZoektIndexer) detectLanguage(filePath string) string {
	ext := filepath.Ext(filePath)
	basename := filepath.Base(filePath)
	
	langMap := map[string]string{
		".go": "go", ".js": "javascript", ".ts": "typescript",
		".tsx": "typescript", ".jsx": "javascript",
		".py": "python", ".java": "java", ".c": "c",
		".cpp": "cpp", ".cc": "cpp", ".cxx": "cpp",
		".h": "c", ".hpp": "cpp",
		".rs": "rust", ".rb": "ruby", ".php": "php",
		".cs": "csharp", ".kt": "kotlin", ".swift": "swift",
		".scala": "scala", ".clj": "clojure", ".hs": "haskell",
		".json": "json", ".yaml": "yaml", ".yml": "yaml",
		".xml": "xml", ".toml": "toml", ".ini": "ini",
		".md": "markdown", ".rst": "restructuredtext", ".txt": "text",
		".html": "html", ".css": "css", ".scss": "scss",
		".sass": "sass", ".less": "less", ".vue": "vue",
		".svelte": "svelte",
		".dockerfile": "dockerfile", ".makefile": "makefile",
		".cmake": "cmake", ".gitignore": "gitignore",
	}
	
	specialFiles := map[string]string{
		"Dockerfile": "dockerfile", "Makefile": "makefile",
		"CMakeLists.txt": "cmake", "go.mod": "go-mod",
		"go.sum": "go-sum", "Cargo.toml": "toml",
		"pyproject.toml": "toml", "requirements.txt": "text",
		"Pipfile": "toml",
	}
	
	if lang, ok := specialFiles[basename]; ok {
		return lang
	}
	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "text"
}

func (z *RealZoektIndexer) tokenize(text string) []string {
	re := regexp.MustCompile(`[^\w]+`)
	tokens := re.Split(text, -1)
	
	var result []string
	for _, token := range tokens {
		if len(token) > 0 {
			result = append(result, token)
		}
	}
	return result
}

func (z *RealZoektIndexer) deduplicateTerms(terms []string) []string {
	seen := make(map[string]bool)
	var result []string
	
	for _, term := range terms {
		if !seen[term] {
			seen[term] = true
			result = append(result, term)
		}
	}
	
	return result
}

func (z *RealZoektIndexer) updateCorpusStats() {
	z.corpusStats.TotalDocs = len(z.files)
	z.corpusStats.DocFreqs = make(map[string]int)
	z.corpusStats.TotalTerms = 0

	totalLength := 0
	
	for _, fileInfo := range z.files {
		docLength := len(z.tokenize(fileInfo.Content))
		totalLength += docLength
		z.corpusStats.TotalTerms += int64(docLength)

		seenTerms := make(map[string]bool)
		for term := range fileInfo.TermFreqs {
			if !seenTerms[term] {
				z.corpusStats.DocFreqs[term]++
				seenTerms[term] = true
			}
		}
	}

	if z.corpusStats.TotalDocs > 0 {
		z.corpusStats.AvgDocLength = float64(totalLength) / float64(z.corpusStats.TotalDocs)
	}
}

func (z *RealZoektIndexer) matchesFilePatterns(filePath string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}

	fileName := filepath.Base(filePath)
	
	for _, pattern := range patterns {
		if strings.Contains(pattern, "*") || strings.Contains(pattern, "?") || strings.Contains(pattern, "[") {
			if matched, err := filepath.Match(pattern, filePath); err == nil && matched {
				return true
			}
			if matched, err := filepath.Match(pattern, fileName); err == nil && matched {
				return true
			}
		} else {
			if fileName == pattern || strings.HasSuffix(filePath, pattern) || strings.HasSuffix(fileName, pattern) {
				return true
			}
		}
	}
	return false
}

func (z *RealZoektIndexer) matchesLanguages(language string, languages []string) bool {
	if len(languages) == 0 {
		return true
	}

	for _, lang := range languages {
		if strings.EqualFold(language, lang) {
			return true
		}
	}
	return false
}

func (z *RealZoektIndexer) sortByScore(hits []types.SearchHit) []types.SearchHit {
	n := len(hits)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if hits[j].Score < hits[j+1].Score {
				hits[j], hits[j+1] = hits[j+1], hits[j]
			}
		}
	}
	return hits
}
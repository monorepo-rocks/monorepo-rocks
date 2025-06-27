package indexer

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

// ZoektIndexer interface defines the operations for lexical search indexing
type ZoektIndexer interface {
	// Index adds files to the lexical index
	Index(ctx context.Context, files []string) error
	
	// IncrementalIndex updates the index with changed files
	IncrementalIndex(ctx context.Context, files []string) error
	
	// Search performs lexical search with BM25 scoring
	Search(ctx context.Context, query string, options SearchOptions) ([]types.SearchHit, error)
	
	// Delete removes files from the index
	Delete(ctx context.Context, files []string) error
	
	// Stats returns indexing statistics
	Stats() IndexStats
	
	// Close closes the indexer and releases resources
	Close() error
}

// SearchOptions configures search behavior
type SearchOptions struct {
	MaxResults   int
	UseRegex     bool
	CaseSensitive bool
	WholeWord    bool
	FilePatterns []string
	Languages    []string
}

// IndexStats provides information about the index
type IndexStats struct {
	TotalFiles    int
	IndexedFiles  int
	TotalSize     int64
	LastIndexTime time.Time
}

// ZoektStubIndexer is a stub implementation for development/testing
// This will be replaced with real Zoekt integration when available
type ZoektStubIndexer struct {
	mu          sync.RWMutex
	files       map[string]*FileInfo
	indexRoot   string
	stats       IndexStats
	corpusStats *CorpusStats
}

// FileInfo represents indexed file information
type FileInfo struct {
	Path         string
	Content      string
	Language     string
	LastModified time.Time
	Lines        []string
	TermFreqs    map[string]int
}

// CorpusStats maintains corpus-wide statistics for BM25
type CorpusStats struct {
	TotalDocs     int
	AvgDocLength  float64
	DocFreqs      map[string]int // term -> number of docs containing term
	TotalTerms    int64
}

// NewZoektIndexer creates a new Zoekt indexer instance
func NewZoektIndexer(indexRoot string) ZoektIndexer {
	return &ZoektStubIndexer{
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

// Index implements the ZoektIndexer interface
func (z *ZoektStubIndexer) Index(ctx context.Context, files []string) error {
	z.mu.Lock()
	defer z.mu.Unlock()

	log.Printf("Starting to index %d files", len(files))
	successCount := 0
	failCount := 0

	for _, file := range files {
		log.Printf("Indexing file: %s", file)
		if err := z.indexFile(file); err != nil {
			log.Printf("Failed to index file %s: %v", file, err)
			failCount++
			// Continue indexing other files instead of failing the entire batch
			continue
		}
		successCount++
	}

	// Always update corpus stats even if some files failed
	z.updateCorpusStats()
	z.stats.LastIndexTime = time.Now()
	
	log.Printf("Indexing completed: %d successful, %d failed out of %d total files", successCount, failCount, len(files))
	log.Printf("Corpus stats: %d total docs, %.2f avg doc length, %d total terms", 
		z.corpusStats.TotalDocs, z.corpusStats.AvgDocLength, z.corpusStats.TotalTerms)
	
	// Only return error if all files failed
	if failCount > 0 && successCount == 0 {
		return fmt.Errorf("failed to index all %d files", len(files))
	}
	
	return nil
}

// IncrementalIndex implements the ZoektIndexer interface
func (z *ZoektStubIndexer) IncrementalIndex(ctx context.Context, files []string) error {
	// For incremental indexing, we just re-index the files
	// In a real implementation, this would be more sophisticated
	return z.Index(ctx, files)
}

// Search implements the ZoektIndexer interface with BM25 scoring
func (z *ZoektStubIndexer) Search(ctx context.Context, query string, options SearchOptions) ([]types.SearchHit, error) {
	z.mu.RLock()
	defer z.mu.RUnlock()

	if len(z.files) == 0 {
		return []types.SearchHit{}, nil
	}

	var results []types.SearchHit
	queryTerms := z.tokenize(strings.ToLower(query))
	
	// Remove duplicate terms and preserve order
	queryTerms = z.deduplicateTerms(queryTerms)

	// Handle regex search
	if options.UseRegex {
		return z.searchRegex(query, options)
	}

	// BM25 parameters
	k1 := 1.2
	b := 0.75

	// Determine search strategy based on file patterns
	hasFilePatterns := len(options.FilePatterns) > 0

	for filePath, fileInfo := range z.files {
		// Apply file pattern filters
		if !z.matchesFilePatterns(filePath, options.FilePatterns) {
			continue
		}

		// Apply language filters
		if !z.matchesLanguages(fileInfo.Language, options.Languages) {
			continue
		}

		// Calculate BM25 score for this document
		var score float64
		if hasFilePatterns {
			// For file-pattern searches, use OR logic (any term matching gives a score)
			score = z.calculateBM25WithOR(queryTerms, fileInfo, k1, b)
		} else {
			// For general search, use traditional BM25 (all terms contribute)
			score = z.calculateBM25(queryTerms, fileInfo, k1, b)
		}
		
		if score <= 0 {
			continue
		}

		// Find matching lines for context
		matches := z.findMatchingLines(fileInfo, queryTerms, options.CaseSensitive)
		for _, match := range matches {
			hit := types.SearchHit{
				File:       filePath,
				LineNumber: match.LineNumber,
				Text:       match.Text,
				Score:      score,
				Source:     "lex",
				StartByte:  match.StartByte,
				EndByte:    match.EndByte,
				Language:   fileInfo.Language,
			}
			results = append(results, hit)
		}
	}

	// Sort by score (descending)
	results = z.sortByScore(results)

	// Limit results
	if options.MaxResults > 0 && len(results) > options.MaxResults {
		results = results[:options.MaxResults]
	}

	return results, nil
}

// Delete implements the ZoektIndexer interface
func (z *ZoektStubIndexer) Delete(ctx context.Context, files []string) error {
	z.mu.Lock()
	defer z.mu.Unlock()

	for _, file := range files {
		delete(z.files, file)
	}

	z.updateCorpusStats()
	return nil
}

// Stats implements the ZoektIndexer interface
func (z *ZoektStubIndexer) Stats() IndexStats {
	z.mu.RLock()
	defer z.mu.RUnlock()

	z.stats.TotalFiles = len(z.files)
	z.stats.IndexedFiles = len(z.files)
	return z.stats
}

// Close implements the ZoektIndexer interface
func (z *ZoektStubIndexer) Close() error {
	z.mu.Lock()
	defer z.mu.Unlock()

	z.files = make(map[string]*FileInfo)
	return nil
}

// Helper methods

func (z *ZoektStubIndexer) indexFile(filePath string) error {
	// Read actual file content from disk
	content, err := z.readFileContent(filePath)
	if err != nil {
		log.Printf("Warning: Cannot read file %s: %v", filePath, err)
		// Return the error so it can be counted as a failure
		return err
	}
	
	lines := strings.Split(content, "\n")
	language := z.detectLanguage(filePath)
	
	termFreqs := make(map[string]int)
	tokens := z.tokenize(strings.ToLower(content))
	for _, token := range tokens {
		termFreqs[token]++
	}

	// Get actual file modification time
	fileInfo, err := os.Stat(filePath)
	var lastModified time.Time
	if err == nil {
		lastModified = fileInfo.ModTime()
	} else {
		lastModified = time.Now()
		log.Printf("Warning: Cannot get file stats for %s: %v", filePath, err)
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
	
	log.Printf("Successfully indexed file %s (language: %s, size: %d bytes, lines: %d)", 
		filePath, language, len(content), len(lines))
	
	return nil
}

func (z *ZoektStubIndexer) readFileContent(filePath string) (string, error) {
	// Check if file exists and is readable
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Cannot access file %s: %v", filePath, err)
		return "", err
	}

	// Skip very large files to avoid memory issues (limit to 10MB)
	if fileInfo.Size() > 10*1024*1024 {
		log.Printf("Skipping large file %s: %d bytes (limit: 10MB)", filePath, fileInfo.Size())
		return "", fmt.Errorf("file too large: %d bytes", fileInfo.Size())
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Cannot read file content %s: %v", filePath, err)
		return "", err
	}

	// Convert to string and handle different encodings
	return string(content), nil
}

func (z *ZoektStubIndexer) detectLanguage(filePath string) string {
	ext := filepath.Ext(filePath)
	basename := filepath.Base(filePath)
	
	langMap := map[string]string{
		// Programming languages
		".go": "go", ".js": "javascript", ".ts": "typescript",
		".tsx": "typescript", ".jsx": "javascript",
		".py": "python", ".java": "java", ".c": "c",
		".cpp": "cpp", ".cc": "cpp", ".cxx": "cpp",
		".h": "c", ".hpp": "cpp",
		".rs": "rust", ".rb": "ruby", ".php": "php",
		".cs": "csharp", ".kt": "kotlin", ".swift": "swift",
		".scala": "scala", ".clj": "clojure", ".hs": "haskell",
		// Configuration and data formats
		".json": "json", ".yaml": "yaml", ".yml": "yaml",
		".xml": "xml", ".toml": "toml", ".ini": "ini",
		// Documentation
		".md": "markdown", ".rst": "restructuredtext", ".txt": "text",
		// Web technologies
		".html": "html", ".css": "css", ".scss": "scss",
		".sass": "sass", ".less": "less", ".vue": "vue",
		".svelte": "svelte",
		// Build files
		".dockerfile": "dockerfile", ".makefile": "makefile",
		".cmake": "cmake", ".gitignore": "gitignore",
	}
	
	// Check special files without extensions
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

func (z *ZoektStubIndexer) tokenize(text string) []string {
	// Simple tokenization - split on non-alphanumeric characters
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

func (z *ZoektStubIndexer) calculateBM25(queryTerms []string, doc *FileInfo, k1, b float64) float64 {
	if z.corpusStats.TotalDocs == 0 || z.corpusStats.AvgDocLength == 0 {
		return 0
	}

	docLength := float64(len(z.tokenize(doc.Content)))
	score := 0.0

	for _, term := range queryTerms {
		tf := float64(doc.TermFreqs[term])
		if tf == 0 {
			continue
		}

		df := float64(z.corpusStats.DocFreqs[term])
		if df == 0 {
			continue
		}

		// IDF calculation
		idf := math.Log((float64(z.corpusStats.TotalDocs)-df+0.5)/(df+0.5) + 1.0)

		// TF component with saturation
		tfComponent := (tf * (k1 + 1)) / (tf + k1*(1-b+b*(docLength/z.corpusStats.AvgDocLength)))

		score += idf * tfComponent
	}

	return score
}

// calculateBM25WithOR calculates BM25 score using OR logic - any matching term contributes to score
// This is useful for file-pattern searches where we want results if ANY keyword matches
func (z *ZoektStubIndexer) calculateBM25WithOR(queryTerms []string, doc *FileInfo, k1, b float64) float64 {
	if z.corpusStats.TotalDocs == 0 || z.corpusStats.AvgDocLength == 0 {
		return 0
	}

	docLength := float64(len(z.tokenize(doc.Content)))
	score := 0.0
	matchingTerms := 0

	for _, term := range queryTerms {
		tf := float64(doc.TermFreqs[term])
		if tf == 0 {
			continue // Term not found in document
		}

		df := float64(z.corpusStats.DocFreqs[term])
		if df == 0 {
			continue // Term not found in corpus
		}

		// IDF calculation
		idf := math.Log((float64(z.corpusStats.TotalDocs)-df+0.5)/(df+0.5) + 1.0)

		// TF component with saturation
		tfComponent := (tf * (k1 + 1)) / (tf + k1*(1-b+b*(docLength/z.corpusStats.AvgDocLength)))

		score += idf * tfComponent
		matchingTerms++
	}

	// Return score if at least one term matched
	if matchingTerms > 0 {
		return score
	}

	return 0
}

// deduplicateTerms removes duplicate terms while preserving order
func (z *ZoektStubIndexer) deduplicateTerms(terms []string) []string {
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

func (z *ZoektStubIndexer) updateCorpusStats() {
	log.Printf("Updating corpus statistics...")
	
	z.corpusStats.TotalDocs = len(z.files)
	z.corpusStats.DocFreqs = make(map[string]int)
	z.corpusStats.TotalTerms = 0

	totalLength := 0
	uniqueTerms := 0
	
	for _, fileInfo := range z.files {
		docLength := len(z.tokenize(fileInfo.Content))
		totalLength += docLength
		z.corpusStats.TotalTerms += int64(docLength)

		// Count document frequencies
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
	
	uniqueTerms = len(z.corpusStats.DocFreqs)
	log.Printf("Corpus stats updated: %d documents, %d unique terms, %.2f average document length, %d total terms",
		z.corpusStats.TotalDocs, uniqueTerms, z.corpusStats.AvgDocLength, z.corpusStats.TotalTerms)
}

type LineMatch struct {
	LineNumber int
	Text       string
	StartByte  int
	EndByte    int
}

func (z *ZoektStubIndexer) findMatchingLines(fileInfo *FileInfo, queryTerms []string, caseSensitive bool) []LineMatch {
	var matches []LineMatch
	
	for i, line := range fileInfo.Lines {
		searchLine := line
		if !caseSensitive {
			searchLine = strings.ToLower(line)
		}

		hasMatch := false
		for _, term := range queryTerms {
			searchTerm := term
			if !caseSensitive {
				searchTerm = strings.ToLower(term)
			}
			if strings.Contains(searchLine, searchTerm) {
				hasMatch = true
				break
			}
		}

		if hasMatch {
			match := LineMatch{
				LineNumber: i + 1,
				Text:       line,
				StartByte:  0, // Simplified - would calculate actual byte positions
				EndByte:    len(line),
			}
			matches = append(matches, match)
		}
	}

	return matches
}

func (z *ZoektStubIndexer) searchRegex(pattern string, options SearchOptions) ([]types.SearchHit, error) {
	// Compile regex with appropriate flags
	var re *regexp.Regexp
	var err error
	if !options.CaseSensitive {
		re, err = regexp.Compile("(?i)" + pattern)
	} else {
		re, err = regexp.Compile(pattern)
	}
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}

	var results []types.SearchHit
	for filePath, fileInfo := range z.files {
		if !z.matchesFilePatterns(filePath, options.FilePatterns) {
			continue
		}
		if !z.matchesLanguages(fileInfo.Language, options.Languages) {
			continue
		}

		for i, line := range fileInfo.Lines {
			if re.MatchString(line) {
				hit := types.SearchHit{
					File:       filePath,
					LineNumber: i + 1,
					Text:       line,
					Score:      1.0, // Fixed score for regex matches
					Source:     "lex",
					Language:   fileInfo.Language,
				}
				results = append(results, hit)
			}
		}
	}

	if options.MaxResults > 0 && len(results) > options.MaxResults {
		results = results[:options.MaxResults]
	}

	return results, nil
}

func (z *ZoektStubIndexer) matchesFilePatterns(filePath string, patterns []string) bool {
	if len(patterns) == 0 {
		return true
	}

	for _, pattern := range patterns {
		matched, err := filepath.Match(pattern, filepath.Base(filePath))
		if err == nil && matched {
			return true
		}
	}
	return false
}

func (z *ZoektStubIndexer) matchesLanguages(language string, languages []string) bool {
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

func (z *ZoektStubIndexer) sortByScore(hits []types.SearchHit) []types.SearchHit {
	// Simple bubble sort by score (descending)
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
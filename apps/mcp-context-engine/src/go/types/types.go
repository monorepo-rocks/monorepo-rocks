package types

import (
	"time"
)

// SearchHit represents a single search result
type SearchHit struct {
	File       string  `json:"file"`
	LineNumber int     `json:"lno"`
	Text       string  `json:"text"`
	Score      float64 `json:"score"`
	Source     string  `json:"source"` // "lex" or "vec"
	StartByte  int     `json:"start_byte,omitempty"`
	EndByte    int     `json:"end_byte,omitempty"`
	Language   string  `json:"language,omitempty"`
}

// SearchRequest represents a search query
type SearchRequest struct {
	Query    string `json:"query"`
	TopK     int    `json:"k,omitempty"`
	Language string `json:"lang,omitempty"`
	Filters  struct {
		FilePatterns []string `json:"file_patterns,omitempty"`
		Repos        []string `json:"repos,omitempty"`
	} `json:"filters,omitempty"`
}

// SearchResponse contains search results
type SearchResponse struct {
	Hits         []SearchHit   `json:"hits"`
	TotalHits    int           `json:"total_hits"`
	QueryTime    time.Duration `json:"query_time_ms"`
	LexicalHits  int           `json:"lexical_hits"`
	SemanticHits int           `json:"semantic_hits"`
}

// IndexStatus represents the indexing status
type IndexStatus struct {
	Repository     string    `json:"repo"`
	ZoektProgress  int       `json:"zoekt_pct"`  // 0-100
	FAISSProgress  int       `json:"faiss_pct"`  // 0-100
	TotalFiles     int       `json:"total_files"`
	IndexedFiles   int       `json:"indexed_files"`
	LastUpdated    time.Time `json:"last_updated"`
}

// CodeChunk represents a chunk of code for embedding
type CodeChunk struct {
	FileID    string `json:"file_id"`
	FilePath  string `json:"file_path"`
	StartByte int    `json:"start_byte"`
	EndByte   int    `json:"end_byte"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Text      string `json:"text"`
	Language  string `json:"language"`
	Hash      string `json:"hash"` // SHA-256 of text
}

// Embedding represents a vector embedding
type Embedding struct {
	ChunkID string    `json:"chunk_id"`
	Vector  []float32 `json:"vector"`
}

// RepoMetadata contains repository information
type RepoMetadata struct {
	Path         string    `json:"path"`
	LastIndexed  time.Time `json:"last_indexed"`
	FileCount    int       `json:"file_count"`
	ChunkCount   int       `json:"chunk_count"`
	Languages    []string  `json:"languages"`
}

// Error types
type ErrorCode string

const (
	ErrNotFound      ErrorCode = "NOT_FOUND"
	ErrInvalidQuery  ErrorCode = "INVALID_QUERY"
	ErrIndexing      ErrorCode = "INDEXING_ERROR"
	ErrInternal      ErrorCode = "INTERNAL_ERROR"
)

// APIError represents an API error response
type APIError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
}
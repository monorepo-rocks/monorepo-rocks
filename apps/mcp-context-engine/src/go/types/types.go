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
	Hits         []SearchHit      `json:"hits"`
	TotalHits    int              `json:"total_hits"`
	QueryTime    time.Duration    `json:"query_time_ms"`
	LexicalHits  int              `json:"lexical_hits"`
	SemanticHits int              `json:"semantic_hits"`
	Analytics    *FusionAnalytics `json:"analytics,omitempty"`
}

// IndexStatus represents the indexing status
type IndexStatus struct {
	Repository    string    `json:"repo"`
	ZoektProgress int       `json:"zoekt_pct"` // 0-100
	FAISSProgress int       `json:"faiss_pct"` // 0-100
	TotalFiles    int       `json:"total_files"`
	IndexedFiles  int       `json:"indexed_files"`
	LastUpdated   time.Time `json:"last_updated"`
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
	Path        string    `json:"path"`
	LastIndexed time.Time `json:"last_indexed"`
	FileCount   int       `json:"file_count"`
	ChunkCount  int       `json:"chunk_count"`
	Languages   []string  `json:"languages"`
}

// Error types
type ErrorCode string

const (
	ErrNotFound     ErrorCode = "NOT_FOUND"
	ErrInvalidQuery ErrorCode = "INVALID_QUERY"
	ErrIndexing     ErrorCode = "INDEXING_ERROR"
	ErrInternal     ErrorCode = "INTERNAL_ERROR"
)

// APIError represents an API error response
type APIError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Details string    `json:"details,omitempty"`
}

// FusionAnalytics contains detailed fusion ranking analytics
type FusionAnalytics struct {
	Strategy           string            `json:"strategy"`
	TotalCandidates    int               `json:"total_candidates"`
	LexicalCandidates  int               `json:"lexical_candidates"`
	SemanticCandidates int               `json:"semantic_candidates"`
	BothCandidates     int               `json:"both_candidates"`
	EffectiveWeight    float64           `json:"effective_weight"`
	QueryType          string            `json:"query_type"`
	Normalization      string            `json:"normalization"`
	ProcessingTime     time.Duration     `json:"processing_time_ms"`
	ScoreDistribution  ScoreDistribution `json:"score_distribution"`
	BoostFactors       BoostFactors      `json:"boost_factors"`
}

// ScoreDistribution contains statistics about score distributions
type ScoreDistribution struct {
	LexicalMin   float64 `json:"lexical_min"`
	LexicalMax   float64 `json:"lexical_max"`
	LexicalMean  float64 `json:"lexical_mean"`
	SemanticMin  float64 `json:"semantic_min"`
	SemanticMax  float64 `json:"semantic_max"`
	SemanticMean float64 `json:"semantic_mean"`
	FinalMin     float64 `json:"final_min"`
	FinalMax     float64 `json:"final_max"`
	FinalMean    float64 `json:"final_mean"`
}

// BoostFactors contains information about applied boost factors
type BoostFactors struct {
	ExactMatches   int     `json:"exact_matches"`
	SymbolMatches  int     `json:"symbol_matches"`
	FileTypeBoosts int     `json:"file_type_boosts"`
	RecencyBoosts  int     `json:"recency_boosts"`
	AvgBoostFactor float64 `json:"avg_boost_factor"`
}

// EnhancedSearchHit extends SearchHit with additional metadata
type EnhancedSearchHit struct {
	SearchHit
	RawLexicalScore  float64 `json:"raw_lexical_score,omitempty"`
	RawSemanticScore float64 `json:"raw_semantic_score,omitempty"`
	NormalizedScore  float64 `json:"normalized_score,omitempty"`
	BoostFactor      float64 `json:"boost_factor,omitempty"`
	Rank             int     `json:"rank"`
	IsExactMatch     bool    `json:"is_exact_match,omitempty"`
	IsSymbolMatch    bool    `json:"is_symbol_match,omitempty"`
}

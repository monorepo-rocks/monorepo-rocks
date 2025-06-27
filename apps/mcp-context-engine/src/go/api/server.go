package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"../query"
	"../types"
)

// Server provides HTTP API endpoints for the search engine
type Server struct {
	queryService *query.QueryService
	port         int
	server       *http.Server
}

// NewServer creates a new API server instance
func NewServer(queryService *query.QueryService, port int) *Server {
	return &Server{
		queryService: queryService,
		port:         port,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := http.NewServeMux()
	
	// Register routes
	mux.HandleFunc("/v1/search", s.handleSearch)
	mux.HandleFunc("/v1/indexStatus", s.handleIndexStatus)
	mux.HandleFunc("/v1/suggest", s.handleSuggest)
	mux.HandleFunc("/v1/explain", s.handleExplain)
	mux.HandleFunc("/health", s.handleHealth)
	
	// Add middleware
	handler := s.corsMiddleware(s.loggingMiddleware(mux))
	
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting API server on port %d", s.port)
	return s.server.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	
	log.Println("Shutting down API server...")
	return s.server.Shutdown(ctx)
}

// HTTP handlers

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed")
		return
	}

	var request types.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, string(types.ErrInvalidQuery), 
			"Invalid JSON request body: "+err.Error())
		return
	}

	// Validate request
	if strings.TrimSpace(request.Query) == "" {
		s.writeError(w, http.StatusBadRequest, string(types.ErrInvalidQuery), 
			"Query cannot be empty")
		return
	}

	// Set default values
	if request.TopK <= 0 {
		request.TopK = 20
	}
	if request.TopK > 100 {
		request.TopK = 100 // Limit maximum results
	}

	// Perform search
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	response, err := s.queryService.Search(ctx, request)
	if err != nil {
		log.Printf("Search error: %v", err)
		s.writeError(w, http.StatusInternalServerError, string(types.ErrInternal), 
			"Search failed: "+err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleIndexStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	status, err := s.queryService.GetIndexStatus(ctx)
	if err != nil {
		log.Printf("Index status error: %v", err)
		s.writeError(w, http.StatusInternalServerError, string(types.ErrInternal), 
			"Failed to get index status: "+err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		s.writeError(w, http.StatusBadRequest, string(types.ErrInvalidQuery), 
			"Query parameter 'q' is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	suggestions, err := s.queryService.SuggestQuery(ctx, query)
	if err != nil {
		log.Printf("Suggest error: %v", err)
		s.writeError(w, http.StatusInternalServerError, string(types.ErrInternal), 
			"Failed to get suggestions: "+err.Error())
		return
	}

	response := map[string]interface{}{
		"query":       query,
		"suggestions": suggestions,
	}

	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleExplain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		s.writeError(w, http.StatusBadRequest, string(types.ErrInvalidQuery), 
			"Query parameter 'q' is required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	explanation, err := s.queryService.ExplainQuery(ctx, query)
	if err != nil {
		log.Printf("Explain error: %v", err)
		s.writeError(w, http.StatusInternalServerError, string(types.ErrInternal), 
			"Failed to explain query: "+err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, explanation)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed")
		return
	}

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	s.writeJSON(w, http.StatusOK, health)
}

// Middleware

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Wrap ResponseWriter to capture status code
		wrapper := &responseWrapper{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapper, r)
		
		duration := time.Since(start)
		log.Printf("%s %s %d %v %s", 
			r.Method, r.URL.Path, wrapper.statusCode, duration, r.RemoteAddr)
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Helper methods

func (s *Server) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

func (s *Server) writeError(w http.ResponseWriter, statusCode int, code, message string) {
	errorResponse := types.APIError{
		Code:    types.ErrorCode(code),
		Message: message,
	}
	
	s.writeJSON(w, statusCode, errorResponse)
}

// responseWrapper wraps http.ResponseWriter to capture status code
type responseWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Additional API endpoints for advanced features

// BatchSearchRequest represents a batch search request
type BatchSearchRequest struct {
	Queries []types.SearchRequest `json:"queries"`
}

// BatchSearchResponse represents a batch search response
type BatchSearchResponse struct {
	Results []types.SearchResponse `json:"results"`
	Errors  []string               `json:"errors,omitempty"`
}

func (s *Server) handleBatchSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only POST method is allowed")
		return
	}

	var request BatchSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, string(types.ErrInvalidQuery), 
			"Invalid JSON request body: "+err.Error())
		return
	}

	if len(request.Queries) == 0 {
		s.writeError(w, http.StatusBadRequest, string(types.ErrInvalidQuery), 
			"At least one query is required")
		return
	}

	if len(request.Queries) > 10 {
		s.writeError(w, http.StatusBadRequest, string(types.ErrInvalidQuery), 
			"Maximum 10 queries allowed per batch")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	var results []types.SearchResponse
	var errors []string

	for i, query := range request.Queries {
		response, err := s.queryService.Search(ctx, query)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Query %d failed: %v", i, err))
			// Add empty response to maintain index alignment
			results = append(results, types.SearchResponse{})
		} else {
			results = append(results, *response)
		}
	}

	batchResponse := BatchSearchResponse{
		Results: results,
		Errors:  errors,
	}

	s.writeJSON(w, http.StatusOK, batchResponse)
}

// SearchStatsResponse provides search statistics
type SearchStatsResponse struct {
	TotalQueries      int64         `json:"total_queries"`
	AverageQueryTime  time.Duration `json:"avg_query_time_ms"`
	CacheHitRate      float64       `json:"cache_hit_rate"`
	IndexStatus       string        `json:"index_status"`
	LastIndexUpdate   time.Time     `json:"last_index_update"`
}

func (s *Server) handleSearchStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is allowed")
		return
	}

	// TODO: Implement actual statistics collection
	// For now, return mock data
	stats := SearchStatsResponse{
		TotalQueries:     1000,
		AverageQueryTime: 150 * time.Millisecond,
		CacheHitRate:     0.75,
		IndexStatus:      "healthy",
		LastIndexUpdate:  time.Now().Add(-1 * time.Hour),
	}

	s.writeJSON(w, http.StatusOK, stats)
}

// RegisterRoutes registers additional routes (can be called before Start)
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/batch-search", s.handleBatchSearch)
	mux.HandleFunc("/v1/stats", s.handleSearchStats)
}

// SetupRoutes sets up all routes and returns the configured handler
func (s *Server) SetupRoutes() http.Handler {
	mux := http.NewServeMux()
	
	// Core routes
	mux.HandleFunc("/v1/search", s.handleSearch)
	mux.HandleFunc("/v1/indexStatus", s.handleIndexStatus)
	mux.HandleFunc("/v1/suggest", s.handleSuggest)
	mux.HandleFunc("/v1/explain", s.handleExplain)
	mux.HandleFunc("/health", s.handleHealth)
	
	// Additional routes
	s.RegisterRoutes(mux)
	
	// Apply middleware
	return s.corsMiddleware(s.loggingMiddleware(mux))
}

// StartWithCustomHandler starts the server with a custom handler
func (s *Server) StartWithCustomHandler(handler http.Handler) error {
	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("Starting API server on port %d", s.port)
	return s.server.ListenAndServe()
}

// GetPort returns the configured port
func (s *Server) GetPort() int {
	return s.port
}

// IsRunning checks if the server is currently running
func (s *Server) IsRunning() bool {
	return s.server != nil
}

// Utility functions for testing

// ParseTopK extracts and validates the topK parameter from URL query
func ParseTopK(r *http.Request, defaultValue int) (int, error) {
	topKStr := r.URL.Query().Get("k")
	if topKStr == "" {
		return defaultValue, nil
	}

	topK, err := strconv.Atoi(topKStr)
	if err != nil {
		return 0, fmt.Errorf("invalid k parameter: %s", topKStr)
	}

	if topK <= 0 {
		return 0, fmt.Errorf("k must be positive, got: %d", topK)
	}

	if topK > 100 {
		return 0, fmt.Errorf("k cannot exceed 100, got: %d", topK)
	}

	return topK, nil
}

// ParseLanguage extracts and validates the language parameter
func ParseLanguage(r *http.Request) string {
	lang := strings.TrimSpace(r.URL.Query().Get("lang"))
	if lang == "" {
		return ""
	}

	// Validate against common languages
	validLanguages := map[string]bool{
		"go": true, "python": true, "javascript": true, "typescript": true,
		"java": true, "cpp": true, "c": true, "rust": true, "php": true,
		"ruby": true, "kotlin": true, "swift": true, "scala": true,
	}

	if validLanguages[strings.ToLower(lang)] {
		return strings.ToLower(lang)
	}

	return lang // Return as-is if not in predefined list
}
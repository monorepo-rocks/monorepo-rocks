package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"../config"
	"../embedder"
	"../indexer"
	"../query"
	"../types"
)

func createTestServer() *Server {
	// Create test configuration
	cfg := config.DefaultConfig()

	// Create test components
	zoektIndexer := indexer.NewZoektIndexer("/tmp/test-zoekt")
	faissIndexer := indexer.NewFAISSIndexer("/tmp/test-faiss", 768)
	testEmbedder := embedder.NewDefaultEmbedder()
	queryService := query.NewQueryService(zoektIndexer, faissIndexer, testEmbedder, cfg)

	return NewServer(queryService, 8080)
}

func setupTestData(server *Server) error {
	// For now, skip the actual data setup since we can't access private fields
	// The stub implementations will work without explicit setup
	return nil
}

func TestServer_HandleSearch(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	request := types.SearchRequest{
		Query: "main function",
		TopK:  10,
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/v1/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.handleSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response types.SearchResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.QueryTime <= 0 {
		t.Error("Expected positive query time")
	}
}

func TestServer_HandleSearchInvalidMethod(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/search", nil)
	rr := httptest.NewRecorder()
	
	server.handleSearch(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

func TestServer_HandleSearchEmptyQuery(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	request := types.SearchRequest{
		Query: "",
		TopK:  10,
	}

	body, _ := json.Marshal(request)
	req := httptest.NewRequest(http.MethodPost, "/v1/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.handleSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var errorResponse types.APIError
	if err := json.NewDecoder(rr.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if errorResponse.Code != types.ErrInvalidQuery {
		t.Errorf("Expected error code %s, got %s", types.ErrInvalidQuery, errorResponse.Code)
	}
}

func TestServer_HandleSearchInvalidJSON(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	invalidJSON := `{"query": "test", "topK": "invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/search", strings.NewReader(invalidJSON))
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.handleSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestServer_HandleIndexStatus(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/indexStatus", nil)
	rr := httptest.NewRecorder()
	
	server.handleIndexStatus(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var status types.IndexStatus
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode status response: %v", err)
	}

	if status.ZoektProgress < 0 || status.ZoektProgress > 100 {
		t.Errorf("Invalid Zoekt progress: %d", status.ZoektProgress)
	}
}

func TestServer_HandleIndexStatusInvalidMethod(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/indexStatus", nil)
	rr := httptest.NewRecorder()
	
	server.handleIndexStatus(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

func TestServer_HandleSuggest(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/suggest?q=func", nil)
	rr := httptest.NewRecorder()
	
	server.handleSuggest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode suggest response: %v", err)
	}

	if response["query"] != "func" {
		t.Errorf("Expected query 'func', got %v", response["query"])
	}

	if suggestions, ok := response["suggestions"].([]interface{}); ok {
		if len(suggestions) < 0 {
			t.Error("Expected non-negative number of suggestions")
		}
	} else {
		t.Error("Expected suggestions array in response")
	}
}

func TestServer_HandleSuggestMissingQuery(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/suggest", nil)
	rr := httptest.NewRecorder()
	
	server.handleSuggest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestServer_HandleExplain(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/explain?q=find+main+function", nil)
	rr := httptest.NewRecorder()
	
	server.handleExplain(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var explanation query.QueryExplanation
	if err := json.NewDecoder(rr.Body).Decode(&explanation); err != nil {
		t.Fatalf("Failed to decode explain response: %v", err)
	}

	if explanation.OriginalQuery != "find main function" {
		t.Errorf("Expected original query 'find main function', got '%s'", explanation.OriginalQuery)
	}
}

func TestServer_HandleHealth(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	
	server.handleHealth(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var health map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode health response: %v", err)
	}

	if health["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", health["status"])
	}

	if health["version"] == nil {
		t.Error("Expected version in health response")
	}
}

func TestServer_HandleBatchSearch(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	batchRequest := BatchSearchRequest{
		Queries: []types.SearchRequest{
			{Query: "main function", TopK: 5},
			{Query: "error handling", TopK: 10},
		},
	}

	body, _ := json.Marshal(batchRequest)
	req := httptest.NewRequest(http.MethodPost, "/v1/batch-search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.handleBatchSearch(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response BatchSearchResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode batch response: %v", err)
	}

	if len(response.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(response.Results))
	}
}

func TestServer_HandleBatchSearchTooManyQueries(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	// Create more than 10 queries
	var queries []types.SearchRequest
	for i := 0; i < 11; i++ {
		queries = append(queries, types.SearchRequest{Query: "test", TopK: 5})
	}

	batchRequest := BatchSearchRequest{Queries: queries}
	body, _ := json.Marshal(batchRequest)
	req := httptest.NewRequest(http.MethodPost, "/v1/batch-search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	
	rr := httptest.NewRecorder()
	server.handleBatchSearch(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestServer_HandleSearchStats(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/stats", nil)
	rr := httptest.NewRecorder()
	
	server.handleSearchStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var stats SearchStatsResponse
	if err := json.NewDecoder(rr.Body).Decode(&stats); err != nil {
		t.Fatalf("Failed to decode stats response: %v", err)
	}

	if stats.TotalQueries < 0 {
		t.Error("Expected non-negative total queries")
	}
}

func TestServer_CORSMiddleware(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	req := httptest.NewRequest(http.MethodOptions, "/v1/search", nil)
	rr := httptest.NewRecorder()
	
	handler := server.corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for OPTIONS, got %d", rr.Code)
	}

	expectedHeaders := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, POST, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, Authorization",
	}

	for header, expected := range expectedHeaders {
		if got := rr.Header().Get(header); got != expected {
			t.Errorf("Expected header %s: %s, got %s", header, expected, got)
		}
	}
}

func TestServer_LoggingMiddleware(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	
	handler := server.loggingMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestServer_WriteJSON(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	data := map[string]interface{}{
		"message": "test",
		"number":  42,
	}

	rr := httptest.NewRecorder()
	server.writeJSON(rr, http.StatusOK, data)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode JSON: %v", err)
	}

	if result["message"] != "test" {
		t.Errorf("Expected message 'test', got %v", result["message"])
	}
}

func TestServer_WriteError(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	rr := httptest.NewRecorder()
	server.writeError(rr, http.StatusBadRequest, "TEST_ERROR", "Test error message")

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var errorResponse types.APIError
	if err := json.NewDecoder(rr.Body).Decode(&errorResponse); err != nil {
		t.Fatalf("Failed to decode error response: %v", err)
	}

	if errorResponse.Code != "TEST_ERROR" {
		t.Errorf("Expected error code 'TEST_ERROR', got %s", errorResponse.Code)
	}

	if errorResponse.Message != "Test error message" {
		t.Errorf("Expected message 'Test error message', got %s", errorResponse.Message)
	}
}

func TestParseTopK(t *testing.T) {
	tests := []struct {
		queryParam   string
		defaultValue int
		expected     int
		expectError  bool
	}{
		{"", 20, 20, false},
		{"10", 20, 10, false},
		{"0", 20, 0, true},
		{"-5", 20, 0, true},
		{"101", 20, 0, true},
		{"invalid", 20, 0, true},
	}

	for _, test := range tests {
		req := httptest.NewRequest(http.MethodGet, "/?k="+test.queryParam, nil)
		
		result, err := ParseTopK(req, test.defaultValue)
		
		if test.expectError {
			if err == nil {
				t.Errorf("Expected error for query param '%s', got nil", test.queryParam)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for query param '%s': %v", test.queryParam, err)
			}
			if result != test.expected {
				t.Errorf("Expected %d for query param '%s', got %d", test.expected, test.queryParam, result)
			}
		}
	}
}

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		queryParam string
		expected   string
	}{
		{"", ""},
		{"go", "go"},
		{"Python", "python"},
		{"JavaScript", "javascript"},
		{"custom-lang", "custom-lang"},
		{"  rust  ", "rust"},
	}

	for _, test := range tests {
		req := httptest.NewRequest(http.MethodGet, "/?lang="+test.queryParam, nil)
		
		result := ParseLanguage(req)
		
		if result != test.expected {
			t.Errorf("Expected '%s' for query param '%s', got '%s'", 
				test.expected, test.queryParam, result)
		}
	}
}

func TestServer_GettersAndState(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	if server.GetPort() != 8080 {
		t.Errorf("Expected port 8080, got %d", server.GetPort())
	}

	if server.IsRunning() {
		t.Error("Expected server to not be running initially")
	}
}

func TestServer_SetupRoutes(t *testing.T) {
	server := createTestServer()
	defer server.queryService.Close()

	handler := server.SetupRoutes()
	if handler == nil {
		t.Error("Expected non-nil handler from SetupRoutes")
	}

	// Test that routes are properly set up by making test requests
	testRoutes := []string{
		"/v1/search",
		"/v1/indexStatus", 
		"/v1/suggest",
		"/v1/explain",
		"/health",
		"/v1/batch-search",
		"/v1/stats",
	}

	for _, route := range testRoutes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		rr := httptest.NewRecorder()
		
		handler.ServeHTTP(rr, req)
		
		// We expect either a valid response or method not allowed (405)
		// but not 404 (route not found)
		if rr.Code == http.StatusNotFound {
			t.Errorf("Route %s not found (404)", route)
		}
	}
}

func TestResponseWrapper(t *testing.T) {
	rr := httptest.NewRecorder()
	wrapper := &responseWrapper{ResponseWriter: rr, statusCode: http.StatusOK}

	// Test default status code
	if wrapper.statusCode != http.StatusOK {
		t.Errorf("Expected default status 200, got %d", wrapper.statusCode)
	}

	// Test WriteHeader
	wrapper.WriteHeader(http.StatusBadRequest)
	if wrapper.statusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 after WriteHeader, got %d", wrapper.statusCode)
	}
}
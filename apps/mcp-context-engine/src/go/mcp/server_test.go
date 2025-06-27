package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

// mockSearchHandler implements SearchHandler for testing
type mockSearchHandler struct {
	searchFunc func(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error)
	statusFunc func(ctx context.Context) (*types.IndexStatus, error)
}

func (m *mockSearchHandler) Search(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, req)
	}
	return &types.SearchResponse{
		Hits: []types.SearchHit{
			{
				File:       "test.go",
				LineNumber: 10,
				Text:       "func TestExample() {",
				Score:      0.95,
				Source:     "lex",
			},
		},
		TotalHits: 1,
	}, nil
}

func (m *mockSearchHandler) GetStatus(ctx context.Context) (*types.IndexStatus, error) {
	if m.statusFunc != nil {
		return m.statusFunc(ctx)
	}
	return &types.IndexStatus{
		Repository:    "/test/repo",
		ZoektProgress: 100,
		FAISSProgress: 100,
	}, nil
}

func TestServer_HandleToolsList(t *testing.T) {
	server := NewServer(&mockSearchHandler{})
	
	req := &Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	
	resp := server.handleRequest(context.Background(), req)
	
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
	
	// Check that code_context tool is listed
	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatal("Expected result to be a map")
	}
	
	tools, ok := result["tools"].([]ToolDefinition)
	if !ok || len(tools) == 0 {
		t.Fatal("Expected tools list")
	}
	
	if tools[0].Name != "code_context" {
		t.Errorf("Expected tool name 'code_context', got %s", tools[0].Name)
	}
}

func TestServer_HandleToolCall(t *testing.T) {
	server := NewServer(&mockSearchHandler{})
	
	// Create tool call request
	searchArgs, _ := json.Marshal(map[string]interface{}{
		"query": "test function",
		"top_k": 10,
	})
	
	params, _ := json.Marshal(map[string]interface{}{
		"name":      "code_context",
		"arguments": searchArgs,
	})
	
	req := &Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  params,
	}
	
	resp := server.handleRequest(context.Background(), req)
	
	if resp.Error != nil {
		t.Fatalf("Unexpected error: %v", resp.Error)
	}
	
	// Verify search response
	searchResp, ok := resp.Result.(*types.SearchResponse)
	if !ok {
		t.Fatal("Expected SearchResponse result")
	}
	
	if len(searchResp.Hits) != 1 {
		t.Errorf("Expected 1 hit, got %d", len(searchResp.Hits))
	}
}

func TestServer_Run(t *testing.T) {
	// Create a simple request/response test
	input := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	output := &bytes.Buffer{}
	
	server := &Server{
		input:  input,
		output: output,
		search: &mockSearchHandler{},
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	
	// Run server (will exit when input is exhausted or context times out)
	err := server.Run(ctx)
	if err != nil && err != context.DeadlineExceeded {
		t.Fatalf("Unexpected error: %v", err)
	}
	
	// Check output contains initialization and response
	outputStr := output.String()
	if !strings.Contains(outputStr, "initialized") {
		t.Error("Expected initialization notification")
	}
	if !strings.Contains(outputStr, "tools/list") {
		t.Error("Expected tools/list in output")
	}
}
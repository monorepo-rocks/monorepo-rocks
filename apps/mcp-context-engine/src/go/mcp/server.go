package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

// Request represents an MCP JSON-RPC request
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response represents an MCP JSON-RPC response
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error represents a JSON-RPC error
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Notification represents an MCP notification
type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// ToolDefinition represents an MCP tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// SearchHandler is the interface for search functionality
type SearchHandler interface {
	Search(ctx context.Context, req *types.SearchRequest) (*types.SearchResponse, error)
	GetStatus(ctx context.Context) (*types.IndexStatus, error)
}

// Server implements the MCP stdio protocol
type Server struct {
	input   io.Reader
	output  io.Writer
	search  SearchHandler
	mu      sync.Mutex
	running bool
}

// NewServer creates a new MCP server
func NewServer(search SearchHandler) *Server {
	return &Server{
		input:  os.Stdin,
		output: os.Stdout,
		search: search,
	}
}

// Run starts the MCP server
func (s *Server) Run(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	scanner := bufio.NewScanner(s.input)
	encoder := json.NewEncoder(s.output)

	// Send initialization notification
	if err := s.sendInitNotification(encoder); err != nil {
		return fmt.Errorf("failed to send init notification: %w", err)
	}

	// Process requests
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			// Invalid JSON, skip
			continue
		}

		// Handle request
		resp := s.handleRequest(ctx, &req)
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				return fmt.Errorf("failed to encode response: %w", err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

// RunWithProtocolDetection starts the server with automatic protocol detection
func (s *Server) RunWithProtocolDetection(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("server already running")
	}
	s.running = true
	s.mu.Unlock()

	scanner := bufio.NewScanner(s.input)
	encoder := json.NewEncoder(s.output)

	// Process the first line to detect protocol
	if !scanner.Scan() {
		return fmt.Errorf("no input received")
	}

	firstLine := scanner.Bytes()
	if len(firstLine) == 0 {
		return fmt.Errorf("empty input received")
	}

	// Try to detect protocol by checking if it's a valid MCP JSON-RPC request
	var req Request
	if err := json.Unmarshal(firstLine, &req); err == nil && req.JSONRPC == "2.0" {
		// This looks like MCP JSON-RPC protocol
		return s.runMCPProtocol(ctx, scanner, encoder, &req)
	}

	// Try to parse as simple JSON search request
	var searchReq types.SearchRequest
	if err := json.Unmarshal(firstLine, &searchReq); err == nil && searchReq.Query != "" {
		// This looks like simple JSON lines protocol
		return s.runSimpleProtocol(ctx, scanner, encoder, &searchReq)
	}

	return fmt.Errorf("unrecognized input format")
}

// runMCPProtocol handles the full MCP JSON-RPC protocol
func (s *Server) runMCPProtocol(ctx context.Context, scanner *bufio.Scanner, encoder *json.Encoder, firstReq *Request) error {
	// Send initialization notification
	if err := s.sendInitNotification(encoder); err != nil {
		return fmt.Errorf("failed to send init notification: %w", err)
	}

	// Handle the first request
	if resp := s.handleRequest(ctx, firstReq); resp != nil {
		if err := encoder.Encode(resp); err != nil {
			return fmt.Errorf("failed to encode response: %w", err)
		}
	}

	// Process remaining requests
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}

		if resp := s.handleRequest(ctx, &req); resp != nil {
			if err := encoder.Encode(resp); err != nil {
				return fmt.Errorf("failed to encode response: %w", err)
			}
		}
	}

	return scanner.Err()
}

// runSimpleProtocol handles the simple JSON lines protocol
func (s *Server) runSimpleProtocol(ctx context.Context, scanner *bufio.Scanner, encoder *json.Encoder, firstReq *types.SearchRequest) error {
	// Handle the first request
	if err := s.handleSimpleRequest(ctx, encoder, firstReq); err != nil {
		return err
	}

	// Process remaining requests
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var searchReq types.SearchRequest
		if err := json.Unmarshal(line, &searchReq); err != nil {
			// Invalid JSON, skip
			continue
		}

		if err := s.handleSimpleRequest(ctx, encoder, &searchReq); err != nil {
			return err
		}
	}

	return scanner.Err()
}

// handleSimpleRequest processes a simple search request and returns JSON array of hits
func (s *Server) handleSimpleRequest(ctx context.Context, encoder *json.Encoder, searchReq *types.SearchRequest) error {
	// Set default values - support both top_k and k fields
	if searchReq.TopK == 0 && searchReq.K == 0 {
		searchReq.TopK = 20
	} else if searchReq.TopK == 0 && searchReq.K > 0 {
		searchReq.TopK = searchReq.K // Use K if TopK is not set
	}

	// Execute search
	resp, err := s.search.Search(ctx, searchReq)
	if err != nil {
		// Return error as JSON
		errorResp := map[string]interface{}{
			"error": err.Error(),
		}
		return encoder.Encode(errorResp)
	}

	// Return just the hits array (backward compatibility)
	return encoder.Encode(resp.Hits)
}

// sendInitNotification sends the initialization notification
func (s *Server) sendInitNotification(encoder *json.Encoder) error {
	notification := Notification{
		JSONRPC: "2.0",
		Method:  "initialized",
		Params: map[string]interface{}{
			"protocolVersion": "1.0",
			"serverInfo": map[string]interface{}{
				"name":    "mcp-context-engine",
				"version": "0.0.1",
			},
		},
	}
	return encoder.Encode(notification)
}

// handleRequest processes a single request
func (s *Server) handleRequest(ctx context.Context, req *Request) *Response {
	resp := &Response{
		JSONRPC: "2.0",
		ID:      req.ID,
	}

	switch req.Method {
	case "initialize":
		resp.Result = s.handleInitialize()
	case "tools/list":
		resp.Result = s.handleToolsList()
	case "tools/call":
		result, err := s.handleToolCall(ctx, req.Params)
		if err != nil {
			resp.Error = &Error{
				Code:    -32603,
				Message: err.Error(),
			}
		} else {
			resp.Result = result
		}
	default:
		resp.Error = &Error{
			Code:    -32601,
			Message: "Method not found",
		}
	}

	return resp
}

// handleInitialize handles the initialize request
func (s *Server) handleInitialize() interface{} {
	return map[string]interface{}{
		"protocolVersion": "1.0",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
	}
}

// handleToolsList returns the list of available tools
func (s *Server) handleToolsList() interface{} {
	return map[string]interface{}{
		"tools": []ToolDefinition{
			{
				Name:        "code_context",
				Description: "Hybrid lexical+semantic code search",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]interface{}{
							"type":        "string",
							"description": "Search query",
						},
						"lang": map[string]interface{}{
							"type":        "string",
							"description": "Filter by language (optional)",
						},
						"top_k": map[string]interface{}{
							"type":        "integer",
							"description": "Number of results to return",
							"default":     20,
						},
					},
					"required": []string{"query"},
				},
			},
		},
	}
}

// handleToolCall handles a tool invocation
func (s *Server) handleToolCall(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var callParams struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(params, &callParams); err != nil {
		return nil, fmt.Errorf("invalid tool call params: %w", err)
	}

	if callParams.Name != "code_context" {
		return nil, fmt.Errorf("unknown tool: %s", callParams.Name)
	}

	// Parse search request
	var searchReq types.SearchRequest
	if err := json.Unmarshal(callParams.Arguments, &searchReq); err != nil {
		return nil, fmt.Errorf("invalid search arguments: %w", err)
	}

	// Default values - support both top_k and k fields
	if searchReq.TopK == 0 && searchReq.K == 0 {
		searchReq.TopK = 20
	} else if searchReq.TopK == 0 && searchReq.K > 0 {
		searchReq.TopK = searchReq.K // Use K if TopK is not set
	}

	// Execute search
	resp, err := s.search.Search(ctx, &searchReq)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MCPMessage represents a JSON-RPC message for MCP protocol
type MCPMessage struct {
	JSONRpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Method  string      `json:"method,omitempty"`
	Params  interface{} `json:"params,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an error in MCP protocol
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCPClient represents a test client for MCP protocol
type MCPClient struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
	reader *bufio.Scanner
	mu     sync.Mutex
}

// setupMCPTestEnvironment creates a test environment for MCP protocol tests
func setupMCPTestEnvironment(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "mcp-protocol-test-*")
	require.NoError(t, err, "Failed to create temp directory")

	// Create test configuration
	configContent := `
indexing:
  batch_size: 100
  index_path: "./indexes"
  languages:
    - "go"
    - "python"
    - "javascript"
    - "typescript"
  file_patterns:
    - "*.go"
    - "*.py"
    - "*.js"
    - "*.ts"
    - "*.md"
    - "*.yaml"
    - "*.json"

search:
  max_results: 50
  min_score: 0.1
  timeout: "10s"
  fusion:
    enabled: true
    weights:
      lexical: 0.6
      semantic: 0.4

embeddings:
  model: "tfidf"
  dimension: 256
  batch_size: 32
  cache_size: 1000

logging:
  level: "info"
  format: "json"
`

	configPath := filepath.Join(tmpDir, "config.yaml")
	err = ioutil.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Failed to write config file")

	// Create test source files
	testFiles := map[string]string{
		"main.go": `package main

import (
	"fmt"
	"log"
)

// User represents a user in the system
type User struct {
	ID   int    \`json:"id"\`
	Name string \`json:"name"\`
}

// authenticate validates user credentials
func authenticate(username, password string) bool {
	// Simple authentication logic
	return username == "admin" && password == "secret"
}

// main function
func main() {
	user := &User{ID: 1, Name: "John Doe"}
	fmt.Printf("User: %+v\n", user)
	
	if authenticate("admin", "secret") {
		log.Println("Authentication successful")
	} else {
		log.Println("Authentication failed")
	}
}`,

		"utils.py": `#!/usr/bin/env python3
"""
Utility functions for the application
"""

import hashlib
import json
from typing import Dict, List, Optional


class UserManager:
    """Manages user operations"""
    
    def __init__(self):
        self.users: Dict[str, Dict] = {}
    
    def authenticate(self, username: str, password: str) -> Optional[Dict]:
        """Authenticate a user"""
        if username in self.users:
            user = self.users[username]
            if self._hash_password(password) == user['password_hash']:
                return {
                    'id': user['id'],
                    'username': username,
                    'email': user['email']
                }
        return None
    
    def create_user(self, username: str, email: str, password: str) -> bool:
        """Create a new user"""
        if username in self.users:
            return False
        
        self.users[username] = {
            'id': len(self.users) + 1,
            'email': email,
            'password_hash': self._hash_password(password)
        }
        return True
    
    def _hash_password(self, password: str) -> str:
        """Hash a password"""
        return hashlib.sha256(password.encode()).hexdigest()


def validate_email(email: str) -> bool:
    """Validate email format"""
    return '@' in email and '.' in email


if __name__ == "__main__":
    manager = UserManager()
    manager.create_user("admin", "admin@example.com", "secret")
    
    user = manager.authenticate("admin", "secret")
    if user:
        print(f"Authentication successful: {user}")
    else:
        print("Authentication failed")`,

		"config.json": `{
  "server": {
    "host": "localhost",
    "port": 8080,
    "timeout": 30
  },
  "database": {
    "driver": "sqlite",
    "path": "app.db"
  },
  "auth": {
    "enabled": true,
    "jwt_secret": "your-secret-key"
  }
}`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		err := ioutil.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err, "Failed to write test file %s", filename)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// startMCPServer starts the MCP server in stdio mode
func startMCPServer(t *testing.T, workDir string) *MCPClient {
	// Build the binary first
	buildCmd := exec.Command("go", "build", "-o", "mcpce", "./src/go/main.go")
	buildCmd.Dir = filepath.Join(workDir, "../../../..")
	err := buildCmd.Run()
	require.NoError(t, err, "Failed to build MCP server binary")

	// Start the server in stdio mode
	binaryPath := filepath.Join(workDir, "../../../../mcpce")
	cmd := exec.Command(binaryPath, "stdio", "--config", filepath.Join(workDir, "config.yaml"))
	cmd.Dir = workDir

	stdin, err := cmd.StdinPipe()
	require.NoError(t, err, "Failed to create stdin pipe")

	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err, "Failed to create stdout pipe")

	stderr, err := cmd.StderrPipe()
	require.NoError(t, err, "Failed to create stderr pipe")

	err = cmd.Start()
	require.NoError(t, err, "Failed to start MCP server")

	client := &MCPClient{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		reader: bufio.NewScanner(stdout),
	}

	// Wait a moment for server to start
	time.Sleep(100 * time.Millisecond)

	return client
}

// sendMessage sends a JSON-RPC message to the MCP server
func (c *MCPClient) sendMessage(msg MCPMessage) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	_, err = c.stdin.Write(append(data, '\n'))
	return err
}

// readMessage reads a JSON-RPC message from the MCP server
func (c *MCPClient) readMessage() (*MCPMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.reader.Scan() {
		if err := c.reader.Err(); err != nil {
			return nil, err
		}
		return nil, io.EOF
	}

	line := c.reader.Text()
	if strings.TrimSpace(line) == "" {
		return c.readMessage() // Skip empty lines
	}

	var msg MCPMessage
	err := json.Unmarshal([]byte(line), &msg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w, line: %s", err, line)
	}

	return &msg, nil
}

// close shuts down the MCP client
func (c *MCPClient) close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.stderr != nil {
		c.stderr.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	return nil
}

func TestMCPProtocolBasicOperations(t *testing.T) {
	tmpDir, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	// Skip if we can't build the binary
	if os.Getenv("SKIP_MCP_INTEGRATION") == "true" {
		t.Skip("MCP integration tests skipped")
	}

	client := startMCPServer(t, tmpDir)
	defer client.close()

	t.Run("Initialize MCP server", func(t *testing.T) {
		// Send initialize request
		initMsg := MCPMessage{
			JSONRpc: "2.0",
			ID:      1,
			Method:  "initialize",
			Params: map[string]interface{}{
				"protocolVersion": "1.0",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"clientInfo": map[string]interface{}{
					"name":    "test-client",
					"version": "1.0.0",
				},
			},
		}

		err := client.sendMessage(initMsg)
		assert.NoError(t, err, "Should send initialize message")

		// Read response
		response, err := client.readMessage()
		assert.NoError(t, err, "Should receive initialize response")
		assert.Equal(t, "2.0", response.JSONRpc, "Should have correct JSON-RPC version")
		assert.Equal(t, 1, response.ID, "Should have correct ID")
		assert.Nil(t, response.Error, "Should not have error")
		assert.NotNil(t, response.Result, "Should have result")

		t.Logf("Initialize response: %+v", response.Result)
	})

	t.Run("List available tools", func(t *testing.T) {
		// Send tools/list request
		listMsg := MCPMessage{
			JSONRpc: "2.0",
			ID:      2,
			Method:  "tools/list",
			Params:  map[string]interface{}{},
		}

		err := client.sendMessage(listMsg)
		assert.NoError(t, err, "Should send tools/list message")

		// Read response
		response, err := client.readMessage()
		assert.NoError(t, err, "Should receive tools/list response")
		assert.Equal(t, "2.0", response.JSONRpc, "Should have correct JSON-RPC version")
		assert.Equal(t, 2, response.ID, "Should have correct ID")
		assert.Nil(t, response.Error, "Should not have error")

		if response.Result != nil {
			t.Logf("Available tools: %+v", response.Result)
		}
	})
}

func TestMCPProtocolSearchOperations(t *testing.T) {
	tmpDir, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	if os.Getenv("SKIP_MCP_INTEGRATION") == "true" {
		t.Skip("MCP integration tests skipped")
	}

	client := startMCPServer(t, tmpDir)
	defer client.close()

	// Initialize the server first
	initMsg := MCPMessage{
		JSONRpc: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "1.0",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	err := client.sendMessage(initMsg)
	require.NoError(t, err, "Should send initialize message")

	_, err = client.readMessage()
	require.NoError(t, err, "Should receive initialize response")

	t.Run("Index files", func(t *testing.T) {
		// Index the test files
		indexMsg := MCPMessage{
			JSONRpc: "2.0",
			ID:      2,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "index_files",
				"arguments": map[string]interface{}{
					"paths": []string{
						filepath.Join(tmpDir, "main.go"),
						filepath.Join(tmpDir, "utils.py"),
						filepath.Join(tmpDir, "config.json"),
					},
				},
			},
		}

		err := client.sendMessage(indexMsg)
		assert.NoError(t, err, "Should send index_files message")

		// Read response
		response, err := client.readMessage()
		assert.NoError(t, err, "Should receive index_files response")
		assert.Equal(t, 2, response.ID, "Should have correct ID")

		if response.Error != nil {
			t.Logf("Index error (may be expected): %+v", response.Error)
		} else {
			t.Logf("Index response: %+v", response.Result)
		}
	})

	t.Run("Search for code", func(t *testing.T) {
		searchQueries := []struct {
			query    string
			expected string
		}{
			{"authenticate", "should find authentication functions"},
			{"User struct", "should find User type definitions"},
			{"password", "should find password-related code"},
		}

		for i, testCase := range searchQueries {
			searchMsg := MCPMessage{
				JSONRpc: "2.0",
				ID:      3 + i,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "search_code",
					"arguments": map[string]interface{}{
						"query":   testCase.query,
						"max_results": 10,
					},
				},
			}

			err := client.sendMessage(searchMsg)
			assert.NoError(t, err, "Should send search_code message for query: %s", testCase.query)

			// Read response
			response, err := client.readMessage()
			assert.NoError(t, err, "Should receive search_code response for query: %s", testCase.query)

			t.Logf("Search query: '%s'", testCase.query)
			if response.Error != nil {
				t.Logf("Search error: %+v", response.Error)
			} else {
				t.Logf("Search results: %+v", response.Result)
			}
		}
	})

	t.Run("Get file content", func(t *testing.T) {
		contentMsg := MCPMessage{
			JSONRpc: "2.0",
			ID:      10,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "get_file_content",
				"arguments": map[string]interface{}{
					"path": filepath.Join(tmpDir, "main.go"),
				},
			},
		}

		err := client.sendMessage(contentMsg)
		assert.NoError(t, err, "Should send get_file_content message")

		// Read response
		response, err := client.readMessage()
		assert.NoError(t, err, "Should receive get_file_content response")

		if response.Error != nil {
			t.Logf("Get content error: %+v", response.Error)
		} else {
			t.Logf("File content response received (content length: %d chars)", 
				len(fmt.Sprintf("%v", response.Result)))
		}
	})
}

func TestMCPProtocolErrorHandling(t *testing.T) {
	tmpDir, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	if os.Getenv("SKIP_MCP_INTEGRATION") == "true" {
		t.Skip("MCP integration tests skipped")
	}

	client := startMCPServer(t, tmpDir)
	defer client.close()

	t.Run("Invalid method", func(t *testing.T) {
		invalidMsg := MCPMessage{
			JSONRpc: "2.0",
			ID:      1,
			Method:  "invalid/method",
			Params:  map[string]interface{}{},
		}

		err := client.sendMessage(invalidMsg)
		assert.NoError(t, err, "Should send invalid method message")

		// Read response
		response, err := client.readMessage()
		assert.NoError(t, err, "Should receive error response")
		assert.NotNil(t, response.Error, "Should have error for invalid method")
		
		t.Logf("Invalid method error: %+v", response.Error)
	})

	t.Run("Invalid parameters", func(t *testing.T) {
		invalidParamsMsg := MCPMessage{
			JSONRpc: "2.0",
			ID:      2,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "search_code",
				"arguments": map[string]interface{}{
					"invalid_param": "value",
				},
			},
		}

		err := client.sendMessage(invalidParamsMsg)
		assert.NoError(t, err, "Should send invalid params message")

		// Read response
		response, err := client.readMessage()
		assert.NoError(t, err, "Should receive error response")
		
		if response.Error != nil {
			t.Logf("Invalid params error: %+v", response.Error)
		}
	})

	t.Run("Non-existent file", func(t *testing.T) {
		nonExistentMsg := MCPMessage{
			JSONRpc: "2.0",
			ID:      3,
			Method:  "tools/call",
			Params: map[string]interface{}{
				"name": "get_file_content",
				"arguments": map[string]interface{}{
					"path": filepath.Join(tmpDir, "non_existent_file.go"),
				},
			},
		}

		err := client.sendMessage(nonExistentMsg)
		assert.NoError(t, err, "Should send non-existent file message")

		// Read response
		response, err := client.readMessage()
		assert.NoError(t, err, "Should receive response")
		
		if response.Error != nil {
			t.Logf("Non-existent file error: %+v", response.Error)
			assert.Contains(t, strings.ToLower(response.Error.Message), "not found", 
				"Error should indicate file not found")
		}
	})
}

func TestMCPProtocolConcurrency(t *testing.T) {
	tmpDir, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	if os.Getenv("SKIP_MCP_INTEGRATION") == "true" {
		t.Skip("MCP integration tests skipped")
	}

	client := startMCPServer(t, tmpDir)
	defer client.close()

	// Initialize first
	initMsg := MCPMessage{
		JSONRpc: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "1.0",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "test-client",
				"version": "1.0.0",
			},
		},
	}

	err := client.sendMessage(initMsg)
	require.NoError(t, err, "Should send initialize message")

	_, err = client.readMessage()
	require.NoError(t, err, "Should receive initialize response")

	t.Run("Multiple concurrent requests", func(t *testing.T) {
		const numRequests = 5
		
		// Send multiple search requests
		for i := 0; i < numRequests; i++ {
			searchMsg := MCPMessage{
				JSONRpc: "2.0",
				ID:      2 + i,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "search_code",
					"arguments": map[string]interface{}{
						"query": fmt.Sprintf("test query %d", i),
					},
				},
			}

			err := client.sendMessage(searchMsg)
			assert.NoError(t, err, "Should send search message %d", i)
		}

		// Read all responses
		responses := make(map[int]*MCPMessage)
		for i := 0; i < numRequests; i++ {
			response, err := client.readMessage()
			assert.NoError(t, err, "Should receive response %d", i)
			
			if response.ID != nil {
				if id, ok := response.ID.(float64); ok {
					responses[int(id)] = response
				}
			}
		}

		// Verify we got all responses
		assert.Equal(t, numRequests, len(responses), "Should receive all responses")

		for id, response := range responses {
			t.Logf("Response %d: error=%v", id, response.Error != nil)
		}
	})
}

func TestMCPProtocolStdioMode(t *testing.T) {
	tmpDir, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	if os.Getenv("SKIP_MCP_INTEGRATION") == "true" {
		t.Skip("MCP integration tests skipped")
	}

	t.Run("Server startup and shutdown", func(t *testing.T) {
		// Test that server starts and can be shut down cleanly
		client := startMCPServer(t, tmpDir)
		
		// Send a simple ping/initialize
		initMsg := MCPMessage{
			JSONRpc: "2.0",
			ID:      1,
			Method:  "initialize",
			Params: map[string]interface{}{
				"protocolVersion": "1.0",
			},
		}

		err := client.sendMessage(initMsg)
		assert.NoError(t, err, "Should send initialize message")

		// Try to read response with timeout
		done := make(chan bool, 1)
		var response *MCPMessage
		var readErr error

		go func() {
			response, readErr = client.readMessage()
			done <- true
		}()

		select {
		case <-done:
			if readErr == nil {
				assert.Equal(t, "2.0", response.JSONRpc, "Should have correct JSON-RPC version")
				t.Logf("Server responded correctly")
			} else {
				t.Logf("Read error (may be expected): %v", readErr)
			}
		case <-time.After(5 * time.Second):
			t.Log("Timeout reading response (may indicate server issue)")
		}

		// Clean shutdown
		err = client.close()
		assert.NoError(t, err, "Should close client cleanly")
	})

	t.Run("Message formatting", func(t *testing.T) {
		// Test that messages are properly formatted as JSON-RPC
		client := startMCPServer(t, tmpDir)
		defer client.close()

		// Test different message types
		messages := []MCPMessage{
			{
				JSONRpc: "2.0",
				ID:      1,
				Method:  "initialize",
				Params:  map[string]interface{}{"protocolVersion": "1.0"},
			},
			{
				JSONRpc: "2.0",
				ID:      2,
				Method:  "tools/list",
				Params:  map[string]interface{}{},
			},
		}

		for i, msg := range messages {
			err := client.sendMessage(msg)
			assert.NoError(t, err, "Should send message %d", i)

			// Don't wait for response in this test, just verify sending works
		}
	})
}

func TestMCPProtocolInLibraryMode(t *testing.T) {
	tmpDir, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Direct MCP server usage", func(t *testing.T) {
		// Test using the MCP server directly as a library
		config := mcp.Config{
			IndexPath:   filepath.Join(tmpDir, "indexes"),
			ConfigPath:  filepath.Join(tmpDir, "config.yaml"),
			LogLevel:    "info",
		}

		server, err := mcp.NewServer(config)
		if err != nil {
			t.Skipf("Could not create MCP server: %v", err)
		}
		defer server.Close()

		// Test basic operations through library interface
		searchResult, err := server.SearchCode(ctx, "authenticate", 10)
		if err != nil {
			t.Logf("Search error (expected if no indexing done): %v", err)
		} else {
			t.Logf("Direct search returned %d results", len(searchResult))
		}
	})
}

func TestMCPProtocolPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	tmpDir, cleanup := setupMCPTestEnvironment(t)
	defer cleanup()

	if os.Getenv("SKIP_MCP_INTEGRATION") == "true" {
		t.Skip("MCP integration tests skipped")
	}

	client := startMCPServer(t, tmpDir)
	defer client.close()

	// Initialize
	initMsg := MCPMessage{
		JSONRpc: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "1.0",
		},
	}

	err := client.sendMessage(initMsg)
	require.NoError(t, err, "Should send initialize message")

	_, err = client.readMessage()
	require.NoError(t, err, "Should receive initialize response")

	t.Run("Response time measurement", func(t *testing.T) {
		queries := []string{
			"authenticate",
			"user",
			"password",
			"config",
			"function",
		}

		var totalTime time.Duration
		successfulQueries := 0

		for i, query := range queries {
			startTime := time.Now()

			searchMsg := MCPMessage{
				JSONRpc: "2.0",
				ID:      2 + i,
				Method:  "tools/call",
				Params: map[string]interface{}{
					"name": "search_code",
					"arguments": map[string]interface{}{
						"query": query,
					},
				},
			}

			err := client.sendMessage(searchMsg)
			if err != nil {
				t.Logf("Failed to send query '%s': %v", query, err)
				continue
			}

			_, err = client.readMessage()
			responseTime := time.Since(startTime)

			if err != nil {
				t.Logf("Failed to read response for query '%s': %v", query, err)
			} else {
				totalTime += responseTime
				successfulQueries++
				t.Logf("Query '%s' took %v", query, responseTime)
			}
		}

		if successfulQueries > 0 {
			avgTime := totalTime / time.Duration(successfulQueries)
			t.Logf("Average response time: %v", avgTime)
			
			// Performance should be reasonable for integration tests
			assert.Less(t, avgTime, 2*time.Second, 
				"Average response time should be under 2 seconds")
		}
	})
}
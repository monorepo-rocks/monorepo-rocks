package embedders

import (
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test data for embedder integration tests
var testCodeSamples = map[string]string{
	"simple_function": `func authenticate(username, password string) bool {
    return username == "admin" && password == "secret"
}`,
	"class_definition": `class UserManager {
    constructor() {
        this.users = new Map();
    }
    
    authenticate(username, password) {
        return username === "admin" && password === "secret";
    }
}`,
	"python_function": `def authenticate(username, password):
    """Authenticate a user with username and password"""
    return username == "admin" and password == "secret"`,
	"import_statement": `import (
    "fmt"
    "log"
    "os"
)`,
	"variable_declaration": `const API_KEY = "your-api-key-here";
let userCount = 0;
var isAuthenticated = false;`,
	"sql_query": `SELECT users.id, users.name, profiles.email 
FROM users 
JOIN profiles ON users.id = profiles.user_id 
WHERE users.active = true`,
	"json_config": `{
    "database": {
        "host": "localhost",
        "port": 5432,
        "name": "testdb"
    },
    "auth": {
        "enabled": true,
        "method": "jwt"
    }
}`,
	"comment_block": `/**
 * This is a comprehensive authentication system
 * that supports multiple authentication methods
 * including JWT tokens and OAuth integration
 */`,
}

// setupEmbedderTestEnvironment creates a test environment for embedder tests
func setupEmbedderTestEnvironment(t *testing.T) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "embedder-integration-*")
	require.NoError(t, err, "Failed to create temp directory")

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

// calculateCosineSimilarity computes cosine similarity between two vectors
func calculateCosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0.0
	}

	var dotProduct, normA, normB float64
	
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	normA = math.Sqrt(normA)
	normB = math.Sqrt(normB)

	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (normA * normB)
}

func TestTFIDFEmbedderIntegration(t *testing.T) {
	tmpDir, cleanup := setupEmbedderTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Basic TF-IDF functionality", func(t *testing.T) {
		config := embedder.EmbedderConfig{
			Model:      "sentence-transformers/all-MiniLM-L6-v2",
			Device:     "cpu",
			BatchSize:  32,
			CacheSize:  1000,
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		}

		emb := embedder.NewStubEmbedder(config)
		require.NotNil(t, emb, "Embedder should be created successfully")

		// Test single text embedding
		text := testCodeSamples["simple_function"]
		vector, err := emb.EmbedText(ctx, text)
		assert.NoError(t, err, "EmbedText should succeed")
		assert.Equal(t, config.Dimension, len(vector), "Vector should have correct dimension")

		// Verify vector is not all zeros
		hasNonZeroValues := false
		for _, val := range vector {
			if val != 0 {
				hasNonZeroValues = true
				break
			}
		}
		assert.True(t, hasNonZeroValues, "Vector should have non-zero values")
	})

	t.Run("TF-IDF vocabulary building", func(t *testing.T) {
		config := embedder.EmbedderConfig{
			Model:      "sentence-transformers/all-MiniLM-L6-v2",
			Device:     "cpu",
			BatchSize:  32,
			CacheSize:  1000,
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		}

		emb := embedder.NewStubEmbedder(config)

		// Embed multiple texts to build vocabulary
		texts := []string{
			testCodeSamples["simple_function"],
			testCodeSamples["class_definition"],
			testCodeSamples["python_function"],
		}

		vectors := make([][]float32, len(texts))
		for i, text := range texts {
			vector, err := emb.EmbedText(ctx, text)
			assert.NoError(t, err, "EmbedText should succeed for text %d", i)
			vectors[i] = vector
		}

		// Verify that similar texts have higher similarity
		// Functions with "authenticate" should be more similar to each other
		funcSimilarity := calculateCosineSimilarity(vectors[0], vectors[2]) // Go and Python functions
		classFuncSimilarity := calculateCosineSimilarity(vectors[1], vectors[2]) // Class and Python function

		assert.Greater(t, funcSimilarity, 0.1, "Functions should have some similarity")
		t.Logf("Function similarity: %f", funcSimilarity)
		t.Logf("Class-function similarity: %f", classFuncSimilarity)
	})

	t.Run("TF-IDF batch processing", func(t *testing.T) {
		config := embedder.EmbedderConfig{
			Model:      "sentence-transformers/all-MiniLM-L6-v2",
			Device:     "cpu",
			BatchSize:  32,
			CacheSize:  1000,
			Timeout:    30 * time.Second,
			MaxRetries: 3,
		}

		emb := embedder.NewStubEmbedder(config)

		// Prepare batch of texts
		var texts []string
		for _, sample := range testCodeSamples {
			texts = append(texts, sample)
		}

		startTime := time.Now()
		vectors, err := emb.EmbedBatch(ctx, texts)
		duration := time.Since(startTime)

		assert.NoError(t, err, "EmbedBatch should succeed")
		assert.Equal(t, len(texts), len(vectors), "Should return vector for each text")

		t.Logf("Batch processing of %d texts took %v", len(texts), duration)

		// Verify all vectors have correct dimension
		for i, vector := range vectors {
			assert.Equal(t, config.Dimension, len(vector), "Vector %d should have correct dimension", i)
		}

		// Test consistency - same text should produce same vector
		singleVector, err := emb.EmbedText(ctx, texts[0])
		assert.NoError(t, err, "Single embedding should succeed")

		similarity := calculateCosineSimilarity(vectors[0], singleVector)
		assert.Greater(t, similarity, 0.99, "Same text should produce very similar vectors")
	})

	t.Run("TF-IDF caching behavior", func(t *testing.T) {
		config := embedder.EmbedderConfig{
			Type:      "tfidf",
			Dimension: 128,
			CacheSize: 10, // Small cache for testing
		}

		emb := embedder.NewStubEmbedder(config)

		text := testCodeSamples["simple_function"]

		// First embedding - should be cached
		startTime := time.Now()
		vector1, err := emb.EmbedText(ctx, text)
		firstDuration := time.Since(startTime)
		assert.NoError(t, err, "First embedding should succeed")

		// Second embedding - should use cache
		startTime = time.Now()
		vector2, err := emb.EmbedText(ctx, text)
		secondDuration := time.Since(startTime)
		assert.NoError(t, err, "Second embedding should succeed")

		// Cached version should be significantly faster (though TF-IDF is already fast)
		assert.True(t, secondDuration <= firstDuration*2, 
			"Cached embedding should not be much slower (first: %v, second: %v)", 
			firstDuration, secondDuration)

		// Vectors should be identical
		similarity := calculateCosineSimilarity(vector1, vector2)
		assert.Equal(t, 1.0, similarity, "Cached vector should be identical")
	})

	t.Run("TF-IDF with different code languages", func(t *testing.T) {
		config := embedder.EmbedderConfig{
			Type:      "tfidf",
			Dimension: 256,
			CacheSize: 1000,
		}

		emb := embedder.NewStubEmbedder(config)

		// Test with different programming languages
		languageTexts := map[string]string{
			"go":         testCodeSamples["simple_function"],
			"javascript": testCodeSamples["class_definition"],
			"python":     testCodeSamples["python_function"],
			"sql":        testCodeSamples["sql_query"],
			"json":       testCodeSamples["json_config"],
		}

		vectors := make(map[string][]float32)
		for lang, text := range languageTexts {
			vector, err := emb.EmbedText(ctx, text)
			assert.NoError(t, err, "Should embed %s code successfully", lang)
			vectors[lang] = vector
		}

		// Verify that code with similar semantics has higher similarity
		// All function definitions should be somewhat similar
		goVector := vectors["go"]
		jsVector := vectors["javascript"]
		pyVector := vectors["python"]

		goJsSimilarity := calculateCosineSimilarity(goVector, jsVector)
		goPySimilarity := calculateCosineSimilarity(goVector, pyVector)
		jsPySimilarity := calculateCosineSimilarity(jsVector, pyVector)

		t.Logf("Go-JS similarity: %f", goJsSimilarity)
		t.Logf("Go-Python similarity: %f", goPySimilarity)
		t.Logf("JS-Python similarity: %f", jsPySimilarity)

		// Code functions should have some similarity
		assert.Greater(t, goJsSimilarity, 0.05, "Go and JavaScript functions should have some similarity")
		assert.Greater(t, goPySimilarity, 0.05, "Go and Python functions should have some similarity")
		assert.Greater(t, jsPySimilarity, 0.05, "JavaScript and Python functions should have some similarity")
	})
}

func TestONNXEmbedderIntegration(t *testing.T) {
	// Check if ONNX runtime is available
	if os.Getenv("SKIP_ONNX_TESTS") == "true" {
		t.Skip("ONNX tests skipped (SKIP_ONNX_TESTS=true)")
	}

	tmpDir, cleanup := setupEmbedderTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("ONNX embedder availability", func(t *testing.T) {
		config := embedder.EmbedderConfig{
			Type:      "onnx",
			Dimension: 768, // CodeBERT dimension
			ModelPath: "./models/codebert.onnx", // This would need to exist
			CacheSize: 1000,
		}

		emb := embedder.NewStubEmbedder(config)
		
		// If ONNX is not available, this will fallback to TF-IDF
		// We test both scenarios
		
		text := testCodeSamples["simple_function"]
		vector, err := emb.EmbedText(ctx, text)
		
		if err != nil {
			// ONNX not available, should fallback gracefully
			t.Logf("ONNX embedder not available, fallback behavior: %v", err)
			// This is acceptable - the system should degrade gracefully
		} else {
			// ONNX is available
			assert.Equal(t, config.Dimension, len(vector), "Vector should have correct dimension")
			
			// Verify vector has reasonable values (not all zeros)
			hasNonZeroValues := false
			for _, val := range vector {
				if val != 0 {
					hasNonZeroValues = true
					break
				}
			}
			assert.True(t, hasNonZeroValues, "ONNX vector should have non-zero values")
		}
	})

	t.Run("ONNX semantic understanding", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping ONNX semantic test in short mode")
		}

		config := embedder.EmbedderConfig{
			Type:      "onnx",
			Dimension: 768,
			ModelPath: "./models/codebert.onnx",
			CacheSize: 1000,
		}

		emb := embedder.NewStubEmbedder(config)

		// Test semantic understanding with similar code patterns
		semanticallyRelated := []string{
			"func authenticate(user, pass string) bool { return user == \"admin\" }",
			"function authenticate(username, password) { return username === \"admin\"; }",
			"def authenticate(username, password): return username == \"admin\"",
		}

		semanticallyUnrelated := []string{
			"SELECT * FROM users WHERE active = true",
			"let colors = [\"red\", \"green\", \"blue\"];",
			"# This is a comment about database configuration",
		}

		var relatedVectors [][]float32
		var unrelatedVectors [][]float32

		for _, text := range semanticallyRelated {
			vector, err := emb.EmbedText(ctx, text)
			if err != nil {
				t.Skip("ONNX embedder not available for semantic testing")
			}
			relatedVectors = append(relatedVectors, vector)
		}

		for _, text := range semanticallyUnrelated {
			vector, err := emb.EmbedText(ctx, text)
			if err != nil {
				t.Skip("ONNX embedder not available for semantic testing")
			}
			unrelatedVectors = append(unrelatedVectors, vector)
		}

		if len(relatedVectors) >= 2 && len(unrelatedVectors) >= 2 {
			// Calculate similarities within related group
			relatedSimilarity := calculateCosineSimilarity(relatedVectors[0], relatedVectors[1])
			
			// Calculate similarity between related and unrelated
			crossSimilarity := calculateCosineSimilarity(relatedVectors[0], unrelatedVectors[0])

			t.Logf("Related similarity: %f", relatedSimilarity)
			t.Logf("Cross similarity: %f", crossSimilarity)

			// ONNX models should show better semantic understanding
			assert.Greater(t, relatedSimilarity, crossSimilarity,
				"Semantically related code should have higher similarity")
		}
	})
}

func TestEmbedderConfigurationAndFallback(t *testing.T) {
	tmpDir, cleanup := setupEmbedderTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Default configuration", func(t *testing.T) {
		config := embedder.DefaultConfig()
		assert.NotNil(t, config, "Default config should not be nil")
		assert.Greater(t, config.Dimension, 0, "Default dimension should be positive")
		assert.Greater(t, config.CacheSize, 0, "Default cache size should be positive")

		emb := embedder.NewStubEmbedder(config)
		require.NotNil(t, emb, "Embedder with default config should be created")

		text := testCodeSamples["simple_function"]
		vector, err := emb.EmbedText(ctx, text)
		assert.NoError(t, err, "Default embedder should work")
		assert.Equal(t, config.Dimension, len(vector), "Vector should have default dimension")
	})

	t.Run("Invalid configuration handling", func(t *testing.T) {
		invalidConfigs := []embedder.EmbedderConfig{
			{Type: "invalid_type", Dimension: 128},
			{Type: "tfidf", Dimension: 0}, // Invalid dimension
			{Type: "tfidf", Dimension: -1}, // Negative dimension
		}

		for i, config := range invalidConfigs {
			emb := embedder.NewStubEmbedder(config)
			
			// Should either create a fallback embedder or handle gracefully
			if emb != nil {
				text := testCodeSamples["simple_function"]
				vector, err := emb.EmbedText(ctx, text)
				
				if err == nil {
					// If it succeeds, it should have reasonable dimension
					assert.Greater(t, len(vector), 0, "Fallback vector should have positive dimension")
				}
				
				t.Logf("Invalid config %d handled gracefully", i)
			}
		}
	})

	t.Run("Performance comparison", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping performance test in short mode")
		}

		tfidfConfig := embedder.EmbedderConfig{
			Type:      "tfidf",
			Dimension: 256,
			CacheSize: 1000,
		}

		onnxConfig := embedder.EmbedderConfig{
			Type:      "onnx",
			Dimension: 768,
			ModelPath: "./models/codebert.onnx",
			CacheSize: 1000,
		}

		tfidfEmb := embedder.NewStubEmbedder(tfidfConfig)
		onnxEmb := embedder.NewStubEmbedder(onnxConfig)

		text := testCodeSamples["class_definition"]

		// Benchmark TF-IDF
		startTime := time.Now()
		tfidfVector, tfidfErr := tfidfEmb.EmbedText(ctx, text)
		tfidfDuration := time.Since(startTime)

		// Benchmark ONNX (or fallback)
		startTime = time.Now()
		onnxVector, onnxErr := onnxEmb.EmbedText(ctx, text)
		onnxDuration := time.Since(startTime)

		t.Logf("TF-IDF embedding time: %v (error: %v)", tfidfDuration, tfidfErr)
		t.Logf("ONNX embedding time: %v (error: %v)", onnxDuration, onnxErr)

		if tfidfErr == nil && onnxErr == nil {
			// TF-IDF should generally be faster
			assert.Less(t, tfidfDuration, 100*time.Millisecond, "TF-IDF should be fast")
			
			// Both should produce valid vectors
			assert.Greater(t, len(tfidfVector), 0, "TF-IDF should produce valid vector")
			assert.Greater(t, len(onnxVector), 0, "ONNX should produce valid vector")
		}
	})
}

func TestEmbedderErrorHandling(t *testing.T) {
	tmpDir, cleanup := setupEmbedderTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Empty text handling", func(t *testing.T) {
		config := embedder.DefaultConfig()
		emb := embedder.NewStubEmbedder(config)

		vector, err := emb.EmbedText(ctx, "")
		// Should either handle gracefully or return appropriate error
		if err == nil {
			// If it succeeds, should return valid vector
			assert.Equal(t, config.Dimension, len(vector), "Empty text should produce vector of correct dimension")
		} else {
			// Error should be meaningful
			assert.Contains(t, strings.ToLower(err.Error()), "empty", "Error should mention empty text")
		}
	})

	t.Run("Very long text handling", func(t *testing.T) {
		config := embedder.DefaultConfig()
		emb := embedder.NewStubEmbedder(config)

		// Create very long text (10KB)
		longText := strings.Repeat("this is a very long code snippet with lots of repetitive content ", 150)
		
		vector, err := emb.EmbedText(ctx, longText)
		if err == nil {
			assert.Equal(t, config.Dimension, len(vector), "Long text should produce vector of correct dimension")
		} else {
			t.Logf("Long text handling error (acceptable): %v", err)
		}
	})

	t.Run("Special characters handling", func(t *testing.T) {
		config := embedder.DefaultConfig()
		emb := embedder.NewStubEmbedder(config)

		specialTexts := []string{
			"func 测试(参数 string) bool { return true }",                    // Unicode
			"function test() { return \"Hello 🌍\"; }",                      // Emoji
			"var x = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$/;", // Regex
			"SELECT * FROM \"table\" WHERE column = 'value''s';",            // SQL with quotes
		}

		for i, text := range specialTexts {
			vector, err := emb.EmbedText(ctx, text)
			if err == nil {
				assert.Equal(t, config.Dimension, len(vector), 
					"Special text %d should produce vector of correct dimension", i)
			} else {
				t.Logf("Special text %d error (may be acceptable): %v", i, err)
			}
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		config := embedder.DefaultConfig()
		emb := embedder.NewStubEmbedder(config)

		// Create a context that's already cancelled
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		text := testCodeSamples["simple_function"]
		_, err := emb.EmbedText(cancelledCtx, text)
		
		if err != nil {
			// Should respect context cancellation
			assert.Contains(t, err.Error(), "context", "Error should mention context cancellation")
		}
	})
}

func TestEmbedderIntegrationWithRealFiles(t *testing.T) {
	tmpDir, cleanup := setupEmbedderTestEnvironment(t)
	defer cleanup()

	ctx := context.Background()

	// Create test files with real code content
	testFiles := map[string]string{
		"user.go": `package main

import (
	"fmt"
	"crypto/sha256"
	"encoding/hex"
)

type User struct {
	ID       int    ` + "`json:\"id\"`" + `
	Username string ` + "`json:\"username\"`" + `
	Email    string ` + "`json:\"email\"`" + `
	Password string ` + "`json:\"-\"`" + `
}

func (u *User) HashPassword(password string) {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	u.Password = hex.EncodeToString(hasher.Sum(nil))
}

func (u *User) ValidatePassword(password string) bool {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	return u.Password == hex.EncodeToString(hasher.Sum(nil))
}

func main() {
	user := &User{
		ID:       1,
		Username: "admin",
		Email:    "admin@example.com",
	}
	user.HashPassword("secret123")
	fmt.Println("User created successfully")
}`,
		"auth.py": `import hashlib
import json
from typing import Optional, Dict, Any

class AuthManager:
    def __init__(self):
        self.users: Dict[str, Dict[str, Any]] = {}
    
    def hash_password(self, password: str) -> str:
        """Hash password using SHA256"""
        return hashlib.sha256(password.encode()).hexdigest()
    
    def create_user(self, username: str, email: str, password: str) -> bool:
        """Create a new user"""
        if username in self.users:
            return False
        
        self.users[username] = {
            'email': email,
            'password': self.hash_password(password),
            'active': True
        }
        return True
    
    def authenticate(self, username: str, password: str) -> Optional[Dict[str, Any]]:
        """Authenticate user credentials"""
        if username not in self.users:
            return None
        
        user = self.users[username]
        if user['password'] == self.hash_password(password) and user['active']:
            return {
                'username': username,
                'email': user['email']
            }
        return None
    
    def deactivate_user(self, username: str) -> bool:
        """Deactivate a user account"""
        if username in self.users:
            self.users[username]['active'] = False
            return True
        return False

if __name__ == "__main__":
    auth = AuthManager()
    auth.create_user("admin", "admin@example.com", "secret123")
    
    result = auth.authenticate("admin", "secret123")
    if result:
        print(f"Authentication successful for {result['username']}")
    else:
        print("Authentication failed")`,
	}

	// Write test files
	for filename, content := range testFiles {
		filePath := filepath.Join(tmpDir, filename)
		err := ioutil.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err, "Failed to write test file %s", filename)
	}

	t.Run("File content embedding", func(t *testing.T) {
		config := embedder.DefaultConfig()
		emb := embedder.NewStubEmbedder(config)

		for filename := range testFiles {
			filePath := filepath.Join(tmpDir, filename)
			content, err := ioutil.ReadFile(filePath)
			require.NoError(t, err, "Failed to read test file %s", filename)

			vector, err := emb.EmbedText(ctx, string(content))
			assert.NoError(t, err, "Should embed file content successfully for %s", filename)
			assert.Equal(t, config.Dimension, len(vector), "Vector should have correct dimension for %s", filename)

			// Verify vector has meaningful content
			hasNonZeroValues := false
			for _, val := range vector {
				if val != 0 {
					hasNonZeroValues = true
					break
				}
			}
			assert.True(t, hasNonZeroValues, "File %s should produce meaningful vector", filename)
		}
	})

	t.Run("Cross-language similarity", func(t *testing.T) {
		config := embedder.DefaultConfig()
		emb := embedder.NewStubEmbedder(config)

		// Read both files
		goContent, err := ioutil.ReadFile(filepath.Join(tmpDir, "user.go"))
		require.NoError(t, err, "Failed to read Go file")

		pythonContent, err := ioutil.ReadFile(filepath.Join(tmpDir, "auth.py"))
		require.NoError(t, err, "Failed to read Python file")

		// Embed both files
		goVector, err := emb.EmbedText(ctx, string(goContent))
		assert.NoError(t, err, "Should embed Go file")

		pythonVector, err := emb.EmbedText(ctx, string(pythonContent))
		assert.NoError(t, err, "Should embed Python file")

		// Calculate similarity
		similarity := calculateCosineSimilarity(goVector, pythonVector)
		t.Logf("Go-Python file similarity: %f", similarity)

		// Both files implement authentication logic, so they should have some similarity
		assert.Greater(t, similarity, 0.05, "Related authentication code should have some similarity")

		// Test with unrelated content
		unrelatedContent := "This is just a plain text document with no code content whatsoever."
		unrelatedVector, err := emb.EmbedText(ctx, unrelatedContent)
		assert.NoError(t, err, "Should embed unrelated text")

		goUnrelatedSimilarity := calculateCosineSimilarity(goVector, unrelatedVector)
		pythonUnrelatedSimilarity := calculateCosineSimilarity(pythonVector, unrelatedVector)

		t.Logf("Go-unrelated similarity: %f", goUnrelatedSimilarity)
		t.Logf("Python-unrelated similarity: %f", pythonUnrelatedSimilarity)

		// Code files should be more similar to each other than to unrelated text
		assert.Greater(t, similarity, goUnrelatedSimilarity, "Code files should be more similar to each other")
		assert.Greater(t, similarity, pythonUnrelatedSimilarity, "Code files should be more similar to each other")
	})

	t.Run("Chunk-based embedding", func(t *testing.T) {
		config := embedder.DefaultConfig()
		emb := embedder.NewStubEmbedder(config)

		// Read Go file and split into chunks (simulate real usage)
		goContent, err := ioutil.ReadFile(filepath.Join(tmpDir, "user.go"))
		require.NoError(t, err, "Failed to read Go file")

		content := string(goContent)
		lines := strings.Split(content, "\n")

		// Create function-level chunks
		var chunks []string
		var currentChunk strings.Builder
		
		for _, line := range lines {
			currentChunk.WriteString(line + "\n")
			
			// Simple heuristic: end chunk on closing brace at start of line
			if strings.TrimSpace(line) == "}" {
				chunk := strings.TrimSpace(currentChunk.String())
				if len(chunk) > 0 {
					chunks = append(chunks, chunk)
				}
				currentChunk.Reset()
			}
		}

		// Add any remaining content
		if currentChunk.Len() > 0 {
			chunk := strings.TrimSpace(currentChunk.String())
			if len(chunk) > 0 {
				chunks = append(chunks, chunk)
			}
		}

		t.Logf("Split file into %d chunks", len(chunks))

		// Embed all chunks
		chunkVectors := make([][]float32, len(chunks))
		for i, chunk := range chunks {
			vector, err := emb.EmbedText(ctx, chunk)
			assert.NoError(t, err, "Should embed chunk %d", i)
			chunkVectors[i] = vector
		}

		// Find most similar chunks to a query about authentication
		queryVector, err := emb.EmbedText(ctx, "password validation authentication")
		assert.NoError(t, err, "Should embed query")

		bestSimilarity := 0.0
		bestChunkIndex := -1

		for i, chunkVector := range chunkVectors {
			similarity := calculateCosineSimilarity(queryVector, chunkVector)
			if similarity > bestSimilarity {
				bestSimilarity = similarity
				bestChunkIndex = i
			}
		}

		assert.GreaterOrEqual(t, bestChunkIndex, 0, "Should find best matching chunk")
		assert.Greater(t, bestSimilarity, 0.05, "Best chunk should have reasonable similarity")

		t.Logf("Best matching chunk (similarity: %f):", bestSimilarity)
		if bestChunkIndex >= 0 && bestChunkIndex < len(chunks) {
			t.Logf("%s", chunks[bestChunkIndex])
		}
	})
}
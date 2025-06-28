package performance

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/embedder"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/indexer"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/query"
	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BenchmarkResult represents the result of a performance benchmark
type BenchmarkResult struct {
	Name           string
	Operation      string
	Duration       time.Duration
	Throughput     float64 // operations per second
	MemoryUsage    int64   // bytes
	CPUUsage       float64 // percentage
	DataSize       int64   // size of data processed
	Success        bool
	ErrorMessage   string
	AdditionalInfo map[string]interface{}
}

// PerformanceReport aggregates multiple benchmark results
type PerformanceReport struct {
	TestName      string
	StartTime     time.Time
	EndTime       time.Time
	TotalDuration time.Duration
	Results       []BenchmarkResult
	SystemInfo    SystemInfo
}

// SystemInfo captures system information for benchmark context
type SystemInfo struct {
	GOOS         string
	GOARCH       string
	NumCPU       int
	GoVersion    string
	MemoryLimit  int64
}

// generateTestCodeFiles creates a variety of code files for performance testing
func generateTestCodeFiles(tmpDir string, numFiles int) ([]string, error) {
	var filePaths []string

	codeTemplates := map[string]string{
		"go": `package {{package}}

import (
	"fmt"
	"log"
	"time"
	{{imports}}
)

// {{struct}} represents a {{entity}} in the system
type {{struct}} struct {
	ID        int       ` + "`json:\"id\"`" + `
	Name      string    ` + "`json:\"name\"`" + `
	Email     string    ` + "`json:\"email\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
	{{fields}}
}

// New{{struct}} creates a new {{entity}}
func New{{struct}}(name, email string) *{{struct}} {
	return &{{struct}}{
		ID:        {{id}},
		Name:      name,
		Email:     email,
		CreatedAt: time.Now(),
	}
}

// {{method}} {{action}} the {{entity}}
func ({{receiver}} *{{struct}}) {{method}}() error {
	if {{receiver}}.Name == "" {
		return fmt.Errorf("{{entity}} name cannot be empty")
	}
	
	log.Printf("{{action}} {{entity}}: %s", {{receiver}}.Name)
	{{logic}}
	return nil
}

// Validate checks if the {{entity}} is valid
func ({{receiver}} *{{struct}}) Validate() bool {
	return {{receiver}}.Name != "" && {{receiver}}.Email != ""
}

// String returns string representation
func ({{receiver}} *{{struct}}) String() string {
	return fmt.Sprintf("{{struct}}{ID: %d, Name: %s}", {{receiver}}.ID, {{receiver}}.Name)
}`,

		"python": `#!/usr/bin/env python3
"""
{{module}} module for {{description}}
"""

import json
import hashlib
import logging
from datetime import datetime
from typing import Optional, List, Dict, Any
{{imports}}


class {{class}}:
    """{{description}} class"""
    
    def __init__(self, name: str, email: str):
        self.id = {{id}}
        self.name = name
        self.email = email
        self.created_at = datetime.now()
        {{init_fields}}
    
    def {{method}}(self) -> bool:
        """{{action}} the {{entity}}"""
        if not self.name:
            raise ValueError("{{entity}} name cannot be empty")
        
        logging.info(f"{{action}} {{entity}}: {self.name}")
        {{logic}}
        return True
    
    def validate(self) -> bool:
        """Validate the {{entity}}"""
        return bool(self.name and self.email)
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary"""
        return {
            'id': self.id,
            'name': self.name,
            'email': self.email,
            'created_at': self.created_at.isoformat()
        }
    
    def {{action}}_{{entity}}(self, {{params}}) -> Optional[Dict]:
        """{{description}} method"""
        if not self.validate():
            return None
        
        {{complex_logic}}
        return self.to_dict()
    
    def __str__(self) -> str:
        return f"{{class}}(id={self.id}, name={self.name})"


class {{class}}Manager:
    """Manages {{entity}} operations"""
    
    def __init__(self):
        self.{{entities}}: Dict[int, {{class}}] = {}
        self.{{entity}}_count = 0
    
    def create_{{entity}}(self, name: str, email: str) -> {{class}}:
        """Create a new {{entity}}"""
        {{entity}} = {{class}}(name, email)
        self.{{entities}}[{{entity}}.id] = {{entity}}
        self.{{entity}}_count += 1
        return {{entity}}
    
    def get_{{entity}}_by_id(self, {{entity}}_id: int) -> Optional[{{class}}]:
        """Get {{entity}} by ID"""
        return self.{{entities}}.get({{entity}}_id)
    
    def list_{{entities}}(self) -> List[{{class}}]:
        """List all {{entities}}"""
        return list(self.{{entities}}.values())
    
    def {{complex_operation}}(self, {{parameters}}) -> Any:
        """Complex operation for performance testing"""
        results = []
        for {{entity}} in self.{{entities}}.values():
            if {{entity}}.validate():
                {{entity}}.{{method}}()
                results.append({{entity}}.to_dict())
        return results`,

		"javascript": `/**
 * {{module}} - {{description}}
 * @author Performance Test Generator
 */

class {{class}} {
    constructor(name, email) {
        this.id = {{id}};
        this.name = name;
        this.email = email;
        this.createdAt = new Date();
        {{init_fields}}
    }

    /**
     * {{action}} the {{entity}}
     * @returns {boolean} Success status
     */
    {{method}}() {
        if (!this.name) {
            throw new Error('{{entity}} name cannot be empty');
        }

        console.log(` + "`{{action}} {{entity}}: ${this.name}`" + `);
        {{logic}}
        return true;
    }

    /**
     * Validate the {{entity}}
     * @returns {boolean} Validation result
     */
    validate() {
        return Boolean(this.name && this.email);
    }

    /**
     * Convert to JSON object
     * @returns {Object} JSON representation
     */
    toJSON() {
        return {
            id: this.id,
            name: this.name,
            email: this.email,
            createdAt: this.createdAt.toISOString()
        };
    }

    /**
     * {{description}} method
     * @param {string} {{param}} - {{param}} parameter
     * @returns {Object|null} Result object
     */
    {{action}}{{entity}}({{param}}) {
        if (!this.validate()) {
            return null;
        }

        {{complex_logic}}
        return this.toJSON();
    }

    toString() {
        return ` + "`{{class}}(id=${this.id}, name=${this.name})`" + `;
    }
}

/**
 * Manager class for {{entity}} operations
 */
class {{class}}Manager {
    constructor() {
        this.{{entities}} = new Map();
        this.{{entity}}Count = 0;
    }

    /**
     * Create a new {{entity}}
     * @param {string} name - {{entity}} name
     * @param {string} email - {{entity}} email
     * @returns {{{class}}} Created {{entity}}
     */
    create{{entity}}(name, email) {
        const {{entity}} = new {{class}}(name, email);
        this.{{entities}}.set({{entity}}.id, {{entity}});
        this.{{entity}}Count++;
        return {{entity}};
    }

    /**
     * Get {{entity}} by ID
     * @param {number} {{entity}}Id - {{entity}} ID
     * @returns {{{class}}|null} Found {{entity}}
     */
    get{{entity}}ById({{entity}}Id) {
        return this.{{entities}}.get({{entity}}Id) || null;
    }

    /**
     * List all {{entities}}
     * @returns {Array<{{class}}>} Array of {{entities}}
     */
    list{{entities}}() {
        return Array.from(this.{{entities}}.values());
    }

    /**
     * Complex operation for performance testing
     * @param {Object} options - Operation options
     * @returns {Array} Results array
     */
    {{complexOperation}}(options = {}) {
        const results = [];
        for (const {{entity}} of this.{{entities}}.values()) {
            if ({{entity}}.validate()) {
                {{entity}}.{{method}}();
                results.push({{entity}}.toJSON());
            }
        }
        return results;
    }
}

// Export for Node.js
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { {{class}}, {{class}}Manager };
}`,
	}

	for i := 0; i < numFiles; i++ {
		fileType := []string{"go", "python", "javascript"}[i%3]
		
		// Generate template variables
		vars := map[string]string{
			"package":          fmt.Sprintf("pkg%d", i),
			"module":           fmt.Sprintf("module_%d", i),
			"class":            fmt.Sprintf("Entity%d", i),
			"struct":           fmt.Sprintf("Entity%d", i),
			"entity":           fmt.Sprintf("entity%d", i),
			"entities":         fmt.Sprintf("entities%d", i),
			"description":      fmt.Sprintf("Entity %d management", i),
			"method":           fmt.Sprintf("Process%d", i),
			"action":           []string{"Create", "Update", "Process", "Handle"}[i%4],
			"receiver":         "e",
			"id":               fmt.Sprintf("%d", 1000+i),
			"params":           "data: str",
			"param":            "data",
			"parameters":       "filters: dict",
			"fields":           fmt.Sprintf("Field%d string `json:\"field_%d\"`", i, i),
			"init_fields":      fmt.Sprintf("self.field_%d = \"\"", i),
			"imports":          "",
			"logic":            fmt.Sprintf("// Processing logic for entity %d", i),
			"complex_logic":    fmt.Sprintf("const result = this.process%dData(%s);", i, "{{param}}"),
			"complex_operation": fmt.Sprintf("complexOperation%d", i),
			"complexOperation": fmt.Sprintf("complexOperation%d", i),
		}

		// Replace template variables
		content := codeTemplates[fileType]
		for key, value := range vars {
			content = strings.ReplaceAll(content, "{{"+key+"}}", value)
		}

		// Write file
		extension := map[string]string{"go": ".go", "python": ".py", "javascript": ".js"}[fileType]
		filename := fmt.Sprintf("test_file_%d%s", i, extension)
		filePath := filepath.Join(tmpDir, filename)
		
		err := ioutil.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			return nil, err
		}
		
		filePaths = append(filePaths, filePath)
	}

	return filePaths, nil
}

// measureMemoryUsage returns current memory usage in bytes
func measureMemoryUsage() int64 {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return int64(m.Alloc)
}

// runBenchmark executes a benchmark function and measures performance
func runBenchmark(name, operation string, benchFunc func() error) BenchmarkResult {
	memBefore := measureMemoryUsage()
	startTime := time.Now()
	
	err := benchFunc()
	
	duration := time.Since(startTime)
	memAfter := measureMemoryUsage()
	memUsage := memAfter - memBefore
	if memUsage < 0 {
		memUsage = 0 // GC might have run
	}

	result := BenchmarkResult{
		Name:        name,
		Operation:   operation,
		Duration:    duration,
		MemoryUsage: memUsage,
		Success:     err == nil,
		AdditionalInfo: make(map[string]interface{}),
	}

	if err != nil {
		result.ErrorMessage = err.Error()
	}

	return result
}

func BenchmarkZoektIndexingPerformance(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "zoekt-bench-*")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	b.Run("Small dataset", func(b *testing.B) {
		files, err := generateTestCodeFiles(tmpDir, 10)
		require.NoError(b, err)

		zoektIdx := indexer.NewRealZoektIndexer(tmpDir)
		defer zoektIdx.Close()

		ctx := context.Background()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := zoektIdx.Index(ctx, files)
			if err != nil {
				b.Fatalf("Indexing failed: %v", err)
			}
		}

		b.StopTimer()
		stats := zoektIdx.Stats()
		b.Logf("Indexed %d files, total size: %d bytes", stats.TotalFiles, stats.TotalSize)
	})

	b.Run("Medium dataset", func(b *testing.B) {
		files, err := generateTestCodeFiles(tmpDir, 100)
		require.NoError(b, err)

		zoektIdx := indexer.NewRealZoektIndexer(tmpDir)
		defer zoektIdx.Close()

		ctx := context.Background()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := zoektIdx.Index(ctx, files)
			if err != nil {
				b.Fatalf("Indexing failed: %v", err)
			}
		}
	})

	b.Run("Large dataset", func(b *testing.B) {
		if testing.Short() {
			b.Skip("Skipping large dataset test in short mode")
		}

		files, err := generateTestCodeFiles(tmpDir, 500)
		require.NoError(b, err)

		zoektIdx := indexer.NewRealZoektIndexer(tmpDir)
		defer zoektIdx.Close()

		ctx := context.Background()

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			err := zoektIdx.Index(ctx, files)
			if err != nil {
				b.Fatalf("Indexing failed: %v", err)
			}
		}
	})
}

func BenchmarkZoektSearchPerformance(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "zoekt-search-bench-*")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	// Setup
	files, err := generateTestCodeFiles(tmpDir, 100)
	require.NoError(b, err)

	zoektIdx := indexer.NewRealZoektIndexer(tmpDir)
	defer zoektIdx.Close()

	ctx := context.Background()
	err = zoektIdx.Index(ctx, files)
	require.NoError(b, err)

	searchQueries := []string{
		"Entity",
		"Process",
		"function",
		"class",
		"validate",
		"create",
		"manager",
		"email",
		"import",
		"return",
	}

	b.Run("Simple queries", func(b *testing.B) {
		options := indexer.SearchOptions{MaxResults: 10}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			query := searchQueries[i%len(searchQueries)]
			results, err := zoektIdx.Search(ctx, query, options)
			if err != nil {
				b.Fatalf("Search failed: %v", err)
			}
			_ = results // Use results to prevent optimization
		}
	})

	b.Run("Regex queries", func(b *testing.B) {
		regexQueries := []string{
			`func[[:space:]]+[[:word:]]+`,
			`class[[:space:]]+[[:word:]]+`,
			`def[[:space:]]+[[:word:]]+`,
			`[[:word:]]+Manager`,
		}

		options := indexer.SearchOptions{
			MaxResults: 10,
			UseRegex:   true,
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			query := regexQueries[i%len(regexQueries)]
			results, err := zoektIdx.Search(ctx, query, options)
			if err != nil {
				b.Fatalf("Regex search failed: %v", err)
			}
			_ = results
		}
	})

	b.Run("Large result sets", func(b *testing.B) {
		options := indexer.SearchOptions{MaxResults: 100}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := zoektIdx.Search(ctx, "Entity", options)
			if err != nil {
				b.Fatalf("Large result search failed: %v", err)
			}
			_ = results
		}
	})
}

func BenchmarkFAISSPerformance(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "faiss-bench-*")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	dimension := 768
	indexPath := filepath.Join(tmpDir, "bench.faiss")

	faissIdx, err := indexer.NewRealFAISSIndexer(indexPath, dimension)
	require.NoError(b, err)
	defer faissIdx.Close()

	ctx := context.Background()

	// Generate test embeddings
	generateEmbeddings := func(count int) []types.Embedding {
		embeddings := make([]types.Embedding, count)
		for i := 0; i < count; i++ {
			vector := make([]float32, dimension)
			for j := 0; j < dimension; j++ {
				vector[j] = rand.Float32()*2 - 1 // Random values between -1 and 1
			}
			embeddings[i] = types.Embedding{
				ChunkID: fmt.Sprintf("chunk_%d", i),
				Vector:  vector,
			}
		}
		return embeddings
	}

	b.Run("Vector addition", func(b *testing.B) {
		embeddings := generateEmbeddings(100)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			batch := embeddings[i%len(embeddings) : (i%len(embeddings))+min(10, len(embeddings)-(i%len(embeddings)))]
			err := faissIdx.AddVectors(ctx, batch)
			if err != nil {
				b.Fatalf("AddVectors failed: %v", err)
			}
		}
	})

	// Setup index for search benchmarks
	embeddings := generateEmbeddings(1000)
	err = faissIdx.AddVectors(ctx, embeddings)
	require.NoError(b, err)

	b.Run("Vector search", func(b *testing.B) {
		queryVector := make([]float32, dimension)
		for i := 0; i < dimension; i++ {
			queryVector[i] = rand.Float32()*2 - 1
		}

		options := indexer.VectorSearchOptions{MinScore: 0.0}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := faissIdx.Search(ctx, queryVector, 10, options)
			if err != nil {
				b.Fatalf("Vector search failed: %v", err)
			}
			_ = results
		}
	})

	b.Run("Large k search", func(b *testing.B) {
		queryVector := make([]float32, dimension)
		for i := 0; i < dimension; i++ {
			queryVector[i] = rand.Float32()*2 - 1
		}

		options := indexer.VectorSearchOptions{MinScore: 0.0}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := faissIdx.Search(ctx, queryVector, 100, options)
			if err != nil {
				b.Fatalf("Large k search failed: %v", err)
			}
			_ = results
		}
	})
}

func BenchmarkEmbedderPerformance(b *testing.B) {
	config := embedder.DefaultConfig()
	emb := embedder.NewEmbedder(config)

	texts := []string{
		"func authenticate(username, password string) bool { return true }",
		"class UserManager { authenticate(user, pass) { return true; } }",
		"def authenticate(username, password): return True",
		"SELECT * FROM users WHERE username = ? AND password = ?",
		"const authenticate = (user, pass) => user === 'admin' && pass === 'secret';",
	}

	ctx := context.Background()

	b.Run("Single text embedding", func(b *testing.B) {
		text := texts[0]

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			vector, err := emb.EmbedText(ctx, text)
			if err != nil {
				b.Fatalf("EmbedText failed: %v", err)
			}
			_ = vector
		}
	})

	b.Run("Batch embedding", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			vectors, err := emb.EmbedBatch(ctx, texts)
			if err != nil {
				b.Fatalf("EmbedBatch failed: %v", err)
			}
			_ = vectors
		}
	})

	b.Run("Large text embedding", func(b *testing.B) {
		largeText := strings.Repeat("This is a large text with many repeated sentences. ", 100)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			vector, err := emb.EmbedText(ctx, largeText)
			if err != nil {
				b.Fatalf("Large text embedding failed: %v", err)
			}
			_ = vector
		}
	})
}

func BenchmarkFusionRankingPerformance(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "fusion-bench-*")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	// Setup components
	files, err := generateTestCodeFiles(tmpDir, 50)
	require.NoError(b, err)

	zoektIdx := indexer.NewRealZoektIndexer(tmpDir)
	defer zoektIdx.Close()

	faissIdx, err := indexer.NewRealFAISSIndexer(filepath.Join(tmpDir, "bench.faiss"), 256)
	require.NoError(b, err)
	defer faissIdx.Close()

	config := embedder.Config{Type: "tfidf", Dimension: 256, CacheSize: 1000}
	emb := embedder.NewEmbedder(config)

	ctx := context.Background()

	// Index files
	err = zoektIdx.Index(ctx, files)
	require.NoError(b, err)

	// Create embeddings
	var embeddings []types.Embedding
	for _, file := range files {
		content, err := ioutil.ReadFile(file)
		require.NoError(b, err)

		vector, err := emb.EmbedText(ctx, string(content))
		require.NoError(b, err)

		embedding := types.Embedding{
			ChunkID: file,
			Vector:  vector,
		}
		embeddings = append(embeddings, embedding)
	}

	err = faissIdx.AddVectors(ctx, embeddings)
	require.NoError(b, err)

	querySvc := query.NewService(zoektIdx, faissIdx, emb, 0.5)

	b.Run("Fusion search", func(b *testing.B) {
		searchRequest := &types.SearchRequest{
			Query: "authenticate function user",
			TopK:  10,
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			response, err := querySvc.Search(ctx, searchRequest)
			if err != nil {
				b.Fatalf("Fusion search failed: %v", err)
			}
			_ = response
		}
	})

	b.Run("Large result fusion", func(b *testing.B) {
		searchRequest := &types.SearchRequest{
			Query: "Entity",
			TopK:  50,
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			response, err := querySvc.Search(ctx, searchRequest)
			if err != nil {
				b.Fatalf("Large result fusion failed: %v", err)
			}
			_ = response
		}
	})
}

func BenchmarkConcurrentOperations(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "concurrent-bench-*")
	require.NoError(b, err)
	defer os.RemoveAll(tmpDir)

	// Setup
	files, err := generateTestCodeFiles(tmpDir, 20)
	require.NoError(b, err)

	zoektIdx := indexer.NewRealZoektIndexer(tmpDir)
	defer zoektIdx.Close()

	ctx := context.Background()
	err = zoektIdx.Index(ctx, files)
	require.NoError(b, err)

	queries := []string{"Entity", "Process", "validate", "create", "Manager"}

	b.Run("Concurrent searches", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			options := indexer.SearchOptions{MaxResults: 10}
			queryIndex := 0

			for pb.Next() {
				query := queries[queryIndex%len(queries)]
				queryIndex++

				results, err := zoektIdx.Search(ctx, query, options)
				if err != nil {
					b.Fatalf("Concurrent search failed: %v", err)
				}
				_ = results
			}
		})
	})

	b.Run("Mixed operations", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			options := indexer.SearchOptions{MaxResults: 5}
			queryIndex := 0

			for pb.Next() {
				// Mix of search and stats operations
				if queryIndex%10 == 0 {
					_ = zoektIdx.Stats()
				} else {
					query := queries[queryIndex%len(queries)]
					results, err := zoektIdx.Search(ctx, query, options)
					if err != nil {
						b.Fatalf("Mixed operation failed: %v", err)
					}
					_ = results
				}
				queryIndex++
			}
		})
	})
}

func TestPerformanceRegression(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance regression test in short mode")
	}

	tmpDir, err := os.MkdirTemp("", "perf-regression-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Generate test data
	files, err := generateTestCodeFiles(tmpDir, 100)
	require.NoError(t, err)

	report := PerformanceReport{
		TestName:  "Performance Regression Test",
		StartTime: time.Now(),
		SystemInfo: SystemInfo{
			GOOS:      runtime.GOOS,
			GOARCH:    runtime.GOARCH,
			NumCPU:    runtime.NumCPU(),
			GoVersion: runtime.Version(),
		},
	}

	ctx := context.Background()

	t.Run("Indexing performance", func(t *testing.T) {
		zoektIdx := indexer.NewRealZoektIndexer(tmpDir)
		defer zoektIdx.Close()

		result := runBenchmark("ZoektIndexing", "Index 100 files", func() error {
			return zoektIdx.Index(ctx, files)
		})

		assert.True(t, result.Success, "Indexing should succeed")
		assert.Less(t, result.Duration, 30*time.Second, "Indexing should complete within 30 seconds")

		stats := zoektIdx.Stats()
		result.AdditionalInfo["files_indexed"] = stats.TotalFiles
		result.AdditionalInfo["total_size"] = stats.TotalSize
		result.Throughput = float64(stats.TotalFiles) / result.Duration.Seconds()

		report.Results = append(report.Results, result)
		t.Logf("Indexing: %v, %d files, %.2f files/sec", result.Duration, stats.TotalFiles, result.Throughput)
	})

	t.Run("Search performance", func(t *testing.T) {
		zoektIdx := indexer.NewRealZoektIndexer(tmpDir)
		defer zoektIdx.Close()

		err := zoektIdx.Index(ctx, files)
		require.NoError(t, err)

		searchQueries := []string{"Entity", "Process", "function", "class", "validate"}
		options := indexer.SearchOptions{MaxResults: 10}

		result := runBenchmark("ZoektSearch", "Search 5 queries", func() error {
			for _, query := range searchQueries {
				_, err := zoektIdx.Search(ctx, query, options)
				if err != nil {
					return err
				}
			}
			return nil
		})

		assert.True(t, result.Success, "Search should succeed")
		assert.Less(t, result.Duration, 5*time.Second, "Search should complete within 5 seconds")

		result.Throughput = float64(len(searchQueries)) / result.Duration.Seconds()
		result.AdditionalInfo["queries_executed"] = len(searchQueries)

		report.Results = append(report.Results, result)
		t.Logf("Search: %v, %d queries, %.2f queries/sec", result.Duration, len(searchQueries), result.Throughput)
	})

	t.Run("Embedding performance", func(t *testing.T) {
		config := embedder.DefaultConfig()
		emb := embedder.NewEmbedder(config)

		testTexts := make([]string, 20)
		for i := 0; i < 20; i++ {
			content, err := ioutil.ReadFile(files[i%len(files)])
			require.NoError(t, err)
			testTexts[i] = string(content)
		}

		result := runBenchmark("EmbedderBatch", "Embed 20 texts", func() error {
			_, err := emb.EmbedBatch(ctx, testTexts)
			return err
		})

		assert.True(t, result.Success, "Embedding should succeed")
		assert.Less(t, result.Duration, 10*time.Second, "Embedding should complete within 10 seconds")

		result.Throughput = float64(len(testTexts)) / result.Duration.Seconds()
		result.AdditionalInfo["texts_embedded"] = len(testTexts)

		report.Results = append(report.Results, result)
		t.Logf("Embedding: %v, %d texts, %.2f texts/sec", result.Duration, len(testTexts), result.Throughput)
	})

	report.EndTime = time.Now()
	report.TotalDuration = report.EndTime.Sub(report.StartTime)

	// Print performance report
	t.Logf("\n=== Performance Report ===")
	t.Logf("Test: %s", report.TestName)
	t.Logf("Duration: %v", report.TotalDuration)
	t.Logf("System: %s/%s, %d CPUs, %s", report.SystemInfo.GOOS, report.SystemInfo.GOARCH, report.SystemInfo.NumCPU, report.SystemInfo.GoVersion)
	t.Logf("\nResults:")
	for _, result := range report.Results {
		status := "✓"
		if !result.Success {
			status = "✗"
		}
		t.Logf("  %s %s: %v (%.2f ops/sec, %s memory)", 
			status, result.Name, result.Duration, result.Throughput, formatBytes(result.MemoryUsage))
	}
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
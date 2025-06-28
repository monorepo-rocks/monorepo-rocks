//go:build !cgo
// +build !cgo

package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/monorepo-rocks/monorepo-rocks/apps/mcp-context-engine/src/go/types"
)

// RealFAISSIndexer is a stub that returns an error when CGO is not available
type RealFAISSIndexer struct{}

// NewRealFAISSIndexer returns an error when CGO is not available
func NewRealFAISSIndexer(indexPath string, dimension int) (*RealFAISSIndexer, error) {
	return nil, fmt.Errorf("real FAISS indexer requires CGO support; build with CGO_ENABLED=1 and ensure FAISS library is installed")
}

// All methods return errors indicating CGO is required

func (f *RealFAISSIndexer) AddVectors(ctx context.Context, embeddings []types.Embedding) error {
	return fmt.Errorf("real FAISS indexer requires CGO support")
}

func (f *RealFAISSIndexer) Search(ctx context.Context, queryVector []float32, k int, options VectorSearchOptions) ([]VectorSearchResult, error) {
	return nil, fmt.Errorf("real FAISS indexer requires CGO support")
}

func (f *RealFAISSIndexer) Delete(ctx context.Context, chunkIDs []string) error {
	return fmt.Errorf("real FAISS indexer requires CGO support")
}

func (f *RealFAISSIndexer) Save(ctx context.Context, path string) error {
	return fmt.Errorf("real FAISS indexer requires CGO support")
}

func (f *RealFAISSIndexer) Load(ctx context.Context, path string) error {
	return fmt.Errorf("real FAISS indexer requires CGO support")
}

func (f *RealFAISSIndexer) VectorStats() VectorIndexStats {
	return VectorIndexStats{
		TotalVectors: 0,
		Dimension:    0,
		IndexSize:    0,
		LastUpdated:  time.Now(),
	}
}

func (f *RealFAISSIndexer) Close() error {
	return fmt.Errorf("real FAISS indexer requires CGO support")
}
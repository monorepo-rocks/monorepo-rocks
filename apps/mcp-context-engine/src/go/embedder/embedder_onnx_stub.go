//go:build !onnx
// +build !onnx

package embedder

import (
	"fmt"
)

// NewONNXEmbedder creates a stub when ONNX is not available
func NewONNXEmbedder(config EmbedderConfig) Embedder {
	fmt.Println("ONNX embedder not available (build without onnx tag)")
	fmt.Println("Falling back to TF-IDF embedder")
	config.Model = "tfidf-vectorizer-fallback"
	return NewTFIDFEmbedder(config)
}
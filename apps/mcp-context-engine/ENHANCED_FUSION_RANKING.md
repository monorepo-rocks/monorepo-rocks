# Enhanced Fusion Ranking Implementation

This document describes the enhanced fusion ranking algorithm that optimizes hybrid search by combining lexical and semantic results with advanced strategies, query-aware weighting, and comprehensive analytics.

## Overview

The enhanced fusion ranking system provides:

- **Multiple fusion strategies**: RRF, weighted linear, and learned weights
- **Query-aware adaptive weighting**: Different weights based on query characteristics  
- **Score normalization**: Options for better cross-modal score comparison
- **Custom boosting**: Exact matches, symbol matches, file type matches, etc.
- **Comprehensive analytics**: Detailed debugging and performance metrics
- **Backward compatibility**: Seamless integration with existing implementations

## Configuration

### Basic Configuration

```yaml
fusion:
  # Basic settings (backward compatible)
  bm25_weight: 0.45
  
  # Enhanced fusion settings
  strategy: "rrf"  # Options: "rrf", "weighted_linear", "learned_weights"
  normalization: "none"  # Options: "none", "min_max", "z_score", "rank_based"
  rrf_constant: 60.0
```

### Advanced Configuration

```yaml
fusion:
  # Adaptive weighting settings
  adaptive_weighting: true
  query_type_weights:
    natural: 0.35    # Favor semantic for natural language
    code: 0.65       # Favor lexical for code queries
    symbol: 0.75     # Heavily favor lexical for symbols
    file: 0.55       # Slightly favor lexical for file queries
    import: 0.70     # Favor lexical for import queries
    config: 0.80     # Heavily favor lexical for config queries
  
  # Boost factors
  exact_match_boost: 1.5
  symbol_match_boost: 1.3
  file_type_boost: 1.2
  recency_boost: 1.1
  
  # Score thresholds
  min_lexical_score: 0.001
  min_semantic_score: 0.05
  
  # Analytics and debugging
  enable_analytics: true
  debug_scoring: false
```

## Fusion Strategies

### 1. Reciprocal Rank Fusion (RRF)

The default strategy that combines rankings using reciprocal rank weighting:

```
score = lexical_score * λ * (1/(k + rank)) + semantic_score * (1-λ) * (1/(k + rank))
```

- **Best for**: General-purpose ranking with good balance
- **Parameters**: `rrf_constant` (default: 60.0)

### 2. Weighted Linear Fusion

Direct weighted combination of scores:

```
score = lexical_score * λ + semantic_score * (1-λ)
```

- **Best for**: When raw scores are well-calibrated
- **Parameters**: Uses effective weight from adaptive weighting

### 3. Learned Weights Fusion

ML-inspired heuristic-based weighting:

- Short queries (≤2 words): 70% lexical
- Medium queries (3-5 words): 50% lexical  
- Long queries (>5 words): 30% lexical
- Adjusted for query characteristics

- **Best for**: Experimental query-specific optimization

## Query Type Detection

The system automatically detects query characteristics:

| Query Type | Description | Example | Default Weight |
|------------|-------------|---------|----------------|
| `natural` | Natural language queries | "how to implement auth" | 0.35 (favor semantic) |
| `code` | Code-specific queries | "function main implementation" | 0.65 (favor lexical) |
| `symbol` | Symbol/identifier queries | "getUserData method" | 0.75 (heavily lexical) |
| `file` | File-specific queries | "*.go files" | 0.55 (slightly lexical) |
| `import` | Import/usage queries | "import React usage" | 0.70 (favor lexical) |
| `config` | Configuration queries | "package.json dependencies" | 0.80 (heavily lexical) |

## Score Normalization

### None (`none`)
No normalization applied - uses raw scores.

### Min-Max (`min_max`)
Normalizes scores to [0, 1] range:
```
normalized = (score - min) / (max - min)
```

### Z-Score (`z_score`)
Standardizes scores using mean and standard deviation:
```
normalized = sigmoid((score - mean) / stddev)
```

### Rank-Based (`rank_based`)
Uses rank position instead of raw scores:
```
normalized = 1 / rank
```

## Boost Factors

### Exact Match Boost (1.5x default)
Applied when query terms appear exactly in the result text.

### Symbol Match Boost (1.3x default)
Applied for programming symbol matches (camelCase, snake_case, etc.).

### File Type Boost (1.2x default)
Applied when results match specified file patterns.

### Recency Boost (1.1x default)
Applied for recently modified files (when file metadata available).

## Analytics Output

When `enable_analytics: true`, the system provides detailed fusion metrics:

```json
{
  "analytics": {
    "strategy": "rrf",
    "total_candidates": 15,
    "lexical_candidates": 8,
    "semantic_candidates": 12,
    "both_candidates": 5,
    "effective_weight": 0.65,
    "query_type": "code",
    "normalization": "min_max",
    "processing_time_ms": 12.5,
    "score_distribution": {
      "lexical_min": 0.001,
      "lexical_max": 0.95,
      "lexical_mean": 0.35,
      "semantic_min": 0.05,
      "semantic_max": 0.87,
      "semantic_mean": 0.42,
      "final_min": 0.01,
      "final_max": 0.92,
      "final_mean": 0.38
    },
    "boost_factors": {
      "exact_matches": 3,
      "symbol_matches": 2,
      "file_type_boosts": 1,
      "recency_boosts": 0,
      "avg_boost_factor": 1.35
    }
  }
}
```

## Performance Considerations

### Strategy Performance

1. **RRF**: Moderate computational cost, excellent ranking quality
2. **Weighted Linear**: Low computational cost, good for calibrated scores
3. **Learned Weights**: Low computational cost, experimental quality

### Normalization Performance

1. **None**: Fastest, no overhead
2. **Min-Max**: Fast, O(n) single pass
3. **Z-Score**: Moderate, O(n) with statistics calculation
4. **Rank-Based**: Fast, O(1) per item

### Memory Usage

- Analytics add ~200 bytes per search
- Score normalization creates temporary copies
- Boost factor calculation is in-place

## Migration Guide

### From Legacy Fusion

The enhanced system is backward compatible. Existing configurations continue to work:

```yaml
# Legacy config (still works)
fusion:
  bm25_weight: 0.45

# Enhanced config (new features)
fusion:
  bm25_weight: 0.45
  strategy: "rrf"
  adaptive_weighting: true
  enable_analytics: true
```

### Recommended Migration Steps

1. **Enable analytics** to understand current performance
2. **Try different strategies** with your query patterns
3. **Tune query type weights** based on your use cases
4. **Enable score normalization** if you see score scale issues
5. **Adjust boost factors** for your domain-specific needs

## Usage Examples

### Code Search Optimization

```yaml
fusion:
  strategy: "rrf"
  adaptive_weighting: true
  query_type_weights:
    symbol: 0.85    # Very high lexical preference for symbols
    code: 0.75      # High lexical preference for code
    import: 0.80    # High lexical preference for imports
  exact_match_boost: 2.0    # Strong boost for exact matches
  symbol_match_boost: 1.8   # Strong boost for symbol matches
```

### Natural Language Search Optimization

```yaml
fusion:
  strategy: "weighted_linear"
  adaptive_weighting: true
  query_type_weights:
    natural: 0.25   # Favor semantic for natural language
    code: 0.60      # Still prefer lexical for code terms
  normalization: "z_score"  # Better cross-modal comparison
```

### Development and Debugging

```yaml
fusion:
  strategy: "rrf"
  adaptive_weighting: true
  enable_analytics: true
  debug_scoring: true     # Detailed debug logs
  normalization: "min_max"
```

## Troubleshooting

### Common Issues

1. **Poor ranking quality**: Try different fusion strategies or adjust query type weights
2. **Score scale problems**: Enable score normalization (min_max recommended)
3. **Slow performance**: Disable analytics in production, use simpler normalization
4. **Lexical dominance**: Lower query type weights or reduce boost factors
5. **Semantic dominance**: Increase query type weights or enable boosting

### Debug Settings

Enable detailed logging:

```yaml
fusion:
  debug_scoring: true
  enable_analytics: true
```

This provides:
- Query type detection details
- Effective weight calculations
- Score distribution statistics
- Boost factor applications
- Processing time metrics

### Performance Monitoring

Monitor these analytics fields for performance insights:

- `processing_time_ms`: Fusion algorithm overhead
- `total_candidates`: Result set size
- `effective_weight`: Actual lexical/semantic balance
- `avg_boost_factor`: Boost application rate

## Future Enhancements

Planned improvements:

1. **True ML learned weights**: Train models on query performance data
2. **Dynamic RRF constants**: Adaptive k parameter based on result quality
3. **File importance scoring**: Boost based on file significance metrics
4. **Temporal relevance**: Time-based scoring for recency
5. **User preference learning**: Personalized fusion weights
6. **A/B testing framework**: Systematic fusion strategy evaluation

## API Integration

The enhanced fusion ranking integrates seamlessly with the existing search API. Analytics are included in search responses when enabled:

```go
response, err := queryService.Search(ctx, &types.SearchRequest{
    Query: "function implementation",
    TopK:  20,
})

if response.Analytics != nil {
    log.Printf("Fusion strategy: %s, effective weight: %.3f", 
        response.Analytics.Strategy, response.Analytics.EffectiveWeight)
}
```

The enhanced system provides powerful tools for optimizing hybrid search quality while maintaining full backward compatibility with existing implementations.
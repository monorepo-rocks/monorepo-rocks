# MCP Context Engine

Fast, offline code search engine combining lexical and semantic search for LLM agents.

## Features

- **Hybrid Search**: Combines Zoekt (lexical/trigram) and FAISS (semantic/embedding) search
- **Sub-100ms Retrieval**: Optimized for p95 < 100ms query latency
- **Zero Network Dependencies**: All indexing and search runs entirely offline
- **Multi-Language Support**: JavaScript/TypeScript, Python, and Go
- **Real-time Updates**: File changes reflected in index within 10 seconds
- **MCP Protocol**: Native support for Model Context Protocol (stdio mode)

## Installation

```bash
npm install -g mcp-context-engine
```

## Usage

### Index a repository

```bash
mcpce index /path/to/repo
```

### Search indexed code

```bash
# Ad-hoc search
mcpce search -q "import chalk" -k 15

# Filter by language
mcpce search -q "function authenticate" -l js
```

### MCP stdio mode

```bash
# Run as MCP tool for LLM agents
mcpce stdio
```

### MCP Configuration

Add to your MCP settings:

```json
{
  "tools": [
    {
      "name": "mcp-context-engine",
      "command": "mcpce",
      "args": ["stdio"]
    }
  ]
}
```

## Architecture

The engine consists of:

- **File Watcher**: Monitors file changes using fsnotify
- **Lexical Indexer**: Zoekt for exact/regex/symbol search
- **Embedding Worker**: ONNX Runtime with CodeBERT for semantic search
- **Vector Store**: FAISS for efficient similarity search
- **Query Service**: Fusion ranking combining BM25 and cosine similarity

## Configuration

Create `~/.config/mcpce/config.yaml`:

```yaml
index_root: ~/.cache/mcpce
repo_globs: ['~/code/**']
languages: ['js', 'ts', 'py', 'go']
embedding:
  model: 'microsoft/codebert-base'
  device: 'cpu'
fusion:
  bm25_weight: 0.45
watcher:
  debounce_ms: 250
```

## Development

```bash
# Install dependencies
pnpm install
make install-deps

# Build for current platform
pnpm build

# Build for all platforms
make build-all

# Run tests
pnpm test
make test
```

## Performance

| Repo Size | Cold Index | RAM (Search) | Disk | p95 Query |
|-----------|------------|--------------|------|-----------|
| 5k files  | 30s        | ~200MB       | ~150MB | 55ms   |
| 50k files | 4min       | ~1GB         | ~1.2GB | 95ms   |

## License

MIT
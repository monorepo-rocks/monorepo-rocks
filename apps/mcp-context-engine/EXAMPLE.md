# MCP Context Engine - Usage Examples

This document provides practical examples of using mcp-context-engine.

## Installation

```bash
# Install globally
npm install -g mcp-context-engine

# Or use npx
npx mcp-context-engine --help
```

## Basic Usage

### 1. Index a Repository

```bash
# Index a single repository
mcpce index /path/to/your/repo

# Index with file watching (auto-update on changes)
mcpce index /path/to/your/repo --watch

# Force full re-indexing
mcpce index /path/to/your/repo --force
```

### 2. Search Indexed Code

```bash
# Simple search
mcpce search -q "authenticate function"

# Search with more results
mcpce search -q "import chalk" -k 50

# Filter by language
mcpce search -q "async function" -l javascript

# Output as JSON
mcpce search -q "TODO" --json
```

### 3. Run as HTTP API Server

```bash
# Start API server on default port 8080
mcpce serve

# Custom port
mcpce serve -p 3000

# Bind to specific host
mcpce serve --host localhost -p 8080
```

#### API Usage Examples

```bash
# Search via HTTP API
curl -X POST http://localhost:8080/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "authenticate user",
    "k": 20,
    "lang": "python"
  }'

# Check indexing status
curl http://localhost:8080/v1/indexStatus

# Get query suggestions
curl "http://localhost:8080/v1/suggest?q=auth"

# Explain query processing
curl -X POST http://localhost:8080/v1/explain \
  -H "Content-Type: application/json" \
  -d '{"query": "import { useState } from react"}'
```

### 4. MCP Integration for LLM Agents

```bash
# Run in MCP stdio mode
mcpce stdio
```

#### MCP Configuration

Add to your LLM agent's MCP configuration:

```json
{
  "tools": [
    {
      "name": "mcp-context-engine",
      "command": "mcpce",
      "args": ["stdio"],
      "description": "Fast code search across repositories"
    }
  ]
}
```

#### Using from an LLM Agent

```javascript
// Example tool call from an LLM agent
const searchResult = await callTool({
  name: "code_context",
  arguments: {
    query: "function that handles user authentication",
    top_k: 10,
    lang: "javascript"
  }
});
```

## Advanced Usage

### Configuration File

Create `~/.config/mcpce/config.yaml`:

```yaml
# Index storage location
index_root: ~/.cache/mcpce

# Repositories to index
repo_globs:
  - ~/projects/**
  - ~/work/**

# Supported languages
languages:
  - javascript
  - typescript
  - python
  - go

# Embedding configuration
embedding:
  model: microsoft/codebert-base
  device: cpu
  chunk_size: 300
  batch_size: 32

# Search fusion settings
fusion:
  bm25_weight: 0.45  # Balance between lexical (0.45) and semantic (0.55)

# File watcher settings
watcher:
  debounce_ms: 250

# Security settings
security:
  encrypt_index: false
  key_path: ~/.config/mcpce/keyfile
```

### Custom Configuration

```bash
# Use custom config file
mcpce search -q "test" --config ~/my-config.yaml

# Index with custom config
mcpce index /repo --config ~/my-config.yaml
```

## Common Search Patterns

### 1. Find Function Definitions

```bash
# Find specific function
mcpce search -q "function authenticate"

# Find async functions
mcpce search -q "async function.*await"

# Find class methods
mcpce search -q "class.*authenticate.*method"
```

### 2. Find Imports/Dependencies

```bash
# Find specific imports
mcpce search -q "import chalk"

# Find React hooks usage
mcpce search -q "import.*useState.*from react"

# Find Python imports
mcpce search -q "from django import" -l python
```

### 3. Find TODOs and FIXMEs

```bash
# Find all TODOs
mcpce search -q "TODO:" -k 100

# Find FIXMEs with context
mcpce search -q "FIXME.*authentication"
```

### 4. Natural Language Queries

```bash
# The semantic search understands intent
mcpce search -q "where is user authentication handled"

# Find error handling
mcpce search -q "error handling for database connections"

# Find test files
mcpce search -q "unit tests for authentication"
```

## Performance Tips

1. **Initial Indexing**: First-time indexing may take a few minutes for large repositories
2. **Watch Mode**: Use `--watch` flag during development for real-time updates
3. **Language Filters**: Use `-l` flag to narrow search scope and improve performance
4. **Cache Management**: Indexes are cached in `~/.cache/mcpce` by default

## Troubleshooting

### Index Not Found

```bash
# Check if repository is indexed
mcpce serve
# Then visit http://localhost:8080/v1/indexStatus
```

### Slow Searches

```bash
# Check index size
du -sh ~/.cache/mcpce

# Re-index if corrupted
mcpce index /repo --force
```

### Memory Usage

For large repositories (>50k files), ensure adequate RAM:
- 5k files: ~200MB RAM
- 50k files: ~1GB RAM

## Integration Examples

### VS Code Extension

```javascript
// Example VS Code extension integration
const { exec } = require('child_process');

function searchCode(query) {
  return new Promise((resolve, reject) => {
    exec(`mcpce search -q "${query}" --json`, (error, stdout) => {
      if (error) reject(error);
      else resolve(JSON.parse(stdout));
    });
  });
}
```

### GitHub Action

```yaml
name: Code Search Index
on:
  push:
    branches: [main]

jobs:
  index:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
      - run: npm install -g mcp-context-engine
      - run: mcpce index .
```

## Security Considerations

1. **Local-Only**: All processing happens on your machine
2. **No Network Calls**: Zero external API dependencies
3. **Optional Encryption**: Enable index encryption for sensitive code
4. **Respects .gitignore**: Automatically excludes ignored files
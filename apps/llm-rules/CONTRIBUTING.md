# Contributing to llm-rules

## Development

```bash
# Install dependencies
pnpm install

# Run tests
pnpm test

# Build
pnpm build

# Type check
pnpm check
```

## Architecture

- `src/rule-parser.ts`: Parses MDC files and extracts frontmatter/content
- `src/mcp-server.ts`: Creates and manages the MCP server
- `src/cli.ts`: Command-line interface for starting the server

## Testing

The project includes comprehensive tests for both rule parsing and MCP server functionality:

- `src/rule-parser.test.ts`: Tests for MDC file parsing and frontmatter extraction
- `src/mcp-server.test.ts`: Tests for MCP server creation and tool generation
- Test fixtures in `src/test/fixtures/.cursor/rules/`: Sample rule files for testing

## Rule Format

The server expects Cursor rule files in MDC format with frontmatter:

```markdown
---
description: 'A brief description of what this rule covers'
globs: ['**/*.ts', '**/*.tsx']
alwaysApply: false
---

# Rule Content

Your rule content goes here...
```

## Making Changes

1. Make your changes
2. Run tests: `pnpm test`
3. Build: `pnpm build`
4. Test the CLI: `node ./dist/llm-rules.cjs --dir /path/to/test/repo`

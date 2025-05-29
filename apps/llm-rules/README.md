# llm-rules

A Model Context Protocol (MCP) server that provides tools for accessing Cursor rules found in `.cursor/rules/*.mdc` files within a repository. This allows AI tools like Claude and other LLM assistants to access and use Cursor rules through the MCP protocol.

## Overview

This package creates an MCP server that dynamically discovers Cursor rule files and exposes them as callable tools. Each rule file becomes a tool that can be invoked to retrieve the rule content, with descriptions automatically extracted from the frontmatter.

## Usage

Start the MCP server:

```bash
# Using npx
npx llm-rules@latest --dir /path/to/your/repository

# Using bunx
bunx llm-rules@latest --dir /path/to/your/repository
```

The server will:

- Scan the specified directory for `.cursor/rules/*.mdc` files
- Create MCP tools named `cursor_rule_<filename>` for each rule
- Extract descriptions from frontmatter to help LLMs understand when to use each tool
- Serve the rules through the MCP protocol on stdio

### MCP Configuration

To use with MCP clients, add to your `mcp.json` or similar configuration:

```json
{
	"mcpServers": {
		"llm-rules": {
			"command": "npx",
			"args": ["llm-rules@latest", "--dir", "/path/to/your/repository"]
		}
	}
}
```

For Claude Desktop, add to `claude_desktop_config.json`:

```json
{
	"mcpServers": {
		"llm-rules": {
			"command": "npx",
			"args": ["llm-rules@latest", "--dir", "/path/to/your/repository"]
		}
	}
}
```

### Tool Parameters

Each generated tool accepts:

- `include_frontmatter` (boolean, optional): Whether to include YAML frontmatter in the response

## Features

- Dynamic rule discovery from `.cursor/rules/` directories
- MCP protocol compliance for integration with AI tools
- Automatic tool generation with descriptive names
- Frontmatter parsing for rule metadata
- Optional frontmatter inclusion in responses
- Comprehensive error handling

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup and guidelines.

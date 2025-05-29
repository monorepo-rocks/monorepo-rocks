import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js'
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js'
import { z } from 'zod/v4'
import { join } from 'node:path'
import { parseRulesFromDir } from './rule-parser.js'

/**
 * Create and start an MCP server that provides tools for each .cursor/rules/*.mdc file
 */
export async function createMCPServer(workingDir: string = process.cwd()) {
	const rulesDir = join(workingDir, '.cursor', 'rules')
	const rules = parseRulesFromDir(rulesDir)
	
	console.error(`Found ${rules.length} rules in ${rulesDir}`)
	rules.forEach(rule => {
		console.error(`  - ${rule.name}: ${rule.frontmatter.description}`)
	})

	const server = new McpServer({
		name: 'llm-rules',
		version: '1.0.0',
	})

	// Generate tools dynamically from rules
	rules.forEach((rule) => {
		server.tool(
			`cursor_rule_${rule.name}`,
			`Read Cursor rule: ${rule.frontmatter.description}`,
			{
				include_frontmatter: z.boolean().optional().default(false).describe('Whether to include YAML frontmatter in the response'),
			},
			async ({ include_frontmatter }: { include_frontmatter?: boolean }) => {
				const content = include_frontmatter ? rule.fullContent : rule.content
				return {
					content: [
						{
							type: 'text',
							text: content,
						},
					],
				}
			}
		)
	})

	return server
}

/**
 * Start the MCP server with stdio transport
 */
export async function startMCPServer(workingDir?: string) {
	const server = await createMCPServer(workingDir)
	const transport = new StdioServerTransport()
	await server.connect(transport)
	console.error('MCP server started and listening on stdio')
}

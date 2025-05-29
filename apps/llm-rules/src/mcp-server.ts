import { join } from 'node:path'
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js'
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js'
import { z as z3 } from 'zod/v3'

import { version } from '../package.json'
import { fmt } from './format'
import { parseRulesFromDir } from './rule-parser.js'

/**
 * Create and start an MCP server that provides tools for each .cursor/rules/*.mdc file
 */
export async function createMCPServer(workingDir: string = process.cwd()) {
	const rulesDir = join(workingDir, '.cursor', 'rules')
	const rules = await parseRulesFromDir(rulesDir)

	console.error(`Found ${rules.length} rules in ${rulesDir}`)
	rules.forEach((rule) => {
		console.error(`  - ${rule.name}: ${rule.frontmatter.description}`)
	})

	const server = new McpServer({
		name: 'llm-rules',
		version,
	})

	// Generate tools dynamically from rules
	for (const rule of rules) {
		server.tool(
			`cursor_rule_${rule.name}`,
			{
				include_frontmatter: z3
					.boolean()
					.optional()
					.default(false)
					.describe(
						fmt.trim(`
							Whether to include YAML frontmatter (metadata like file patterns, always-apply status).
							Set to true if you need to know when/where this rule applies, false for just the rule content.
						`)
					),
			},
			{ title: `Read Cursor rule: ${rule.frontmatter.description}` },
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
	}

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

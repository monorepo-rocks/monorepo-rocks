import { join } from 'node:path'
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js'
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js'

import { version } from '../package.json'
import { parseContextFromDir } from './context-parser.js'
import { parseRulesFromDir } from './rule-parser.js'

/**
 * Create and start an MCP server that provides tools for both .cursor/rules and .context files
 */
export async function createMCPServer(workingDir: string = process.cwd()) {
	const rulesDir = join(workingDir, '.cursor', 'rules')
	const rules = await parseRulesFromDir(rulesDir)
	const { contexts } = await parseContextFromDir(workingDir)

	console.error(`Found ${rules.length} Cursor rules in ${rulesDir}`)
	rules.forEach((rule) => {
		console.error(`  - ${rule.name}: ${rule.frontmatter.description}`)
	})

	console.error(`Found ${contexts.length} context files`)
	contexts.forEach((context) => {
		console.error(`  - ${context.name} (${context.source}): ${context.frontmatter.description}`)
	})

	const server = new McpServer({
		name: 'llm-rules',
		version,
	})

	// Generate tools dynamically from Cursor rules
	for (const rule of rules) {
		// Build description with metadata to help LLMs decide when to use this rule
		let description = `Read Cursor rule: ${rule.frontmatter.description}`

		const metadata: string[] = []
		if (rule.frontmatter.globs) {
			metadata.push(`applies to ${rule.frontmatter.globs}`)
		}
		if (rule.frontmatter.alwaysApply) {
			metadata.push('always-apply')
		}

		if (metadata.length > 0) {
			description += ` (${metadata.join(', ')})`
		}

		server.tool(`cursor_rule_${rule.name}`, description, {}, async () => {
			return {
				content: [
					{
						type: 'text',
						text: rule.content,
					},
				],
			}
		})
	}

	// Generate tools dynamically from client-hosted context files
	for (const context of contexts) {
		// Build description with metadata to help LLMs decide when to use this context
		let description = `Read context: ${context.frontmatter.description}`

		const metadata: string[] = []

		// Add appliesTo patterns
		if (context.frontmatter.appliesTo) {
			const patterns = Array.isArray(context.frontmatter.appliesTo)
				? context.frontmatter.appliesTo.join(',')
				: context.frontmatter.appliesTo
			metadata.push(`applies to ${patterns}`)
		}

		// Add trigger type
		if (context.frontmatter.trigger && context.frontmatter.trigger !== 'manual') {
			metadata.push(`trigger: ${context.frontmatter.trigger}`)
		}

		// Add source
		metadata.push(`source: ${context.source}`)

		if (metadata.length > 0) {
			description += ` (${metadata.join(', ')})`
		}

		server.tool(`context_${context.name}`, description, {}, async () => {
			return {
				content: [
					{
						type: 'text',
						text: context.content,
					},
				],
			}
		})
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

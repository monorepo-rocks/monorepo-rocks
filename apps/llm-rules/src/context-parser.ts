import { readFile } from 'node:fs/promises'
import { homedir } from 'node:os'
import { join, resolve } from 'node:path'
import matter from 'gray-matter'
import { z as z3 } from 'zod/v3'
import { glob } from 'zx'

export type ContextFrontmatter = z3.infer<typeof ContextFrontmatter>
export const ContextFrontmatter = z3
	.object({
		description: z3.string().nullable().default(''),
		appliesTo: z3.union([z3.string(), z3.array(z3.string())]).optional(),
		globs: z3.union([z3.string(), z3.array(z3.string())]).optional(), // backwards compatibility
		disabled: z3.boolean().optional().default(false),
		trigger: z3.enum(['always', 'pattern', 'agent', 'manual']).optional().default('manual'),
	})
	.transform((data) => ({
		...data,
		description: data.description || '',
		// Use appliesTo if set, otherwise fall back to globs
		appliesTo: data.appliesTo || data.globs,
	}))

export type ContextConfig = z3.infer<typeof ContextConfig>
export const ContextConfig = z3.object({
	clientContext: z3
		.object({
			includeFiles: z3.array(z3.string()).optional().default(['*']),
			excludeFiles: z3.array(z3.string()).optional().default(['context-config.json']),
			ignoreGlobalContext: z3.boolean().optional().default(false),
			ignoreAncestorContext: z3.boolean().optional().default(false),
		})
		.optional()
		.default({
			includeFiles: ['*'],
			excludeFiles: ['context-config.json'],
			ignoreGlobalContext: false,
			ignoreAncestorContext: false,
		}),
})

export type ParsedContext = z3.infer<typeof ParsedContext>
export const ParsedContext = z3.object({
	filename: z3.string(),
	name: z3.string(),
	frontmatter: ContextFrontmatter,
	content: z3.string(),
	fullContent: z3.string(),
	source: z3.enum(['global', 'static', 'dynamic']),
})

/**
 * Get the client context path from environment or default
 */
function getClientContextPath(): string {
	return process.env.CLIENT_CONTEXT_PATH || '.context'
}

/**
 * Get the global context path from environment or default
 */
function getGlobalContextPath(): string {
	const envPath = process.env.GLOBAL_CONTEXT_PATH
	if (envPath) {
		// Handle case where user might have included the client context path
		const clientContextPath = getClientContextPath()
		if (envPath.endsWith(clientContextPath)) {
			return envPath
		}
		return join(envPath, clientContextPath)
	}
	return join(homedir(), getClientContextPath())
}

/**
 * Parse context configuration from JSON file
 */
async function parseContextConfig(configPath: string): Promise<ContextConfig> {
	try {
		const content = await readFile(configPath, 'utf-8')
		const rawConfig = JSON.parse(content)
		const result = ContextConfig.safeParse(rawConfig)

		if (!result.success) {
			console.warn(`Invalid context config at ${configPath}:`, result.error)
			return ContextConfig.parse({}) // Return default config
		}

		return result.data
	} catch (error) {
		// Config file doesn't exist or is invalid - return default
		return ContextConfig.parse({})
	}
}

/**
 * Find and parse context files from a directory
 */
async function parseContextFilesFromDir(
	contextDir: string,
	source: 'global' | 'static' | 'dynamic'
): Promise<ParsedContext[]> {
	try {
		// Find markdown and text files (excluding config)
		const patterns = ['*.md', '*.mdc', '*.txt']
		const allFiles: string[] = []

		for (const pattern of patterns) {
			const files = await glob(pattern, { cwd: contextDir })
			allFiles.push(...files)
		}

		// Filter out config files
		const contextFiles = allFiles.filter((file) => !file.includes('context-config'))

		const results = await Promise.all(
			contextFiles.map((file) => parseContextFile(join(contextDir, file), source))
		)

		return results.filter((context): context is ParsedContext => context !== null)
	} catch (error) {
		console.warn(`Could not read context directory: ${contextDir}`, error)
		return []
	}
}

/**
 * Parse a single context file (markdown or text)
 */
async function parseContextFile(
	filePath: string,
	source: 'global' | 'static' | 'dynamic'
): Promise<ParsedContext | null> {
	try {
		const fullContent = await readFile(filePath, 'utf-8')
		const filename = filePath.split('/').pop() || filePath
		const name = filename.replace(/\.(md|mdc|txt)$/, '')

		let frontmatter: ContextFrontmatter
		let content: string

		if (filename.endsWith('.txt')) {
			// Plain text files have no frontmatter
			frontmatter = ContextFrontmatter.parse({})
			content = fullContent.trim()
		} else {
			// Markdown files may have frontmatter
			const parsed = matter(fullContent)
			const frontmatterResult = ContextFrontmatter.safeParse(parsed.data)

			if (!frontmatterResult.success) {
				console.warn(`Invalid frontmatter in ${filePath}:`, frontmatterResult.error)
				return null
			}

			frontmatter = frontmatterResult.data
			content = parsed.content.trim()
		}

		// Skip disabled context files
		if (frontmatter.disabled) {
			return null
		}

		// Require description for agent-triggered context
		if (frontmatter.trigger === 'agent' && !frontmatter.description) {
			console.warn(`Context file ${filePath} has trigger 'agent' but no description. Skipping.`)
			return null
		}

		// Create a better fallback description if none provided
		if (!frontmatter.description) {
			const humanReadableName = name.replace(/[-_]/g, ' ').toLowerCase()
			frontmatter.description = `Context guidelines for ${humanReadableName}`
		}

		return {
			filename,
			name,
			frontmatter,
			content,
			fullContent,
			source,
		}
	} catch (error) {
		console.warn(`Error parsing context file ${filePath}:`, error)
		return null
	}
}

/**
 * Find and parse all context files following the client-hosted context spec
 */
export async function parseContextFromDir(workingDir: string): Promise<{
	contexts: ParsedContext[]
	config: ContextConfig
}> {
	const clientContextPath = getClientContextPath()
	const staticContextDir = join(workingDir, clientContextPath)
	const globalContextDir = getGlobalContextPath()

	const allContexts: ParsedContext[] = []
	let mergedConfig = ContextConfig.parse({})

	try {
		// 1. Parse global context (if not ignored)
		if (!mergedConfig.clientContext.ignoreGlobalContext) {
			const globalContexts = await parseContextFilesFromDir(globalContextDir, 'global')
			allContexts.push(...globalContexts)

			// Parse global config
			const globalConfigPath = join(globalContextDir, 'context-config.json')
			const globalConfig = await parseContextConfig(globalConfigPath)
			mergedConfig = mergeConfigs(mergedConfig, globalConfig)
		}

		// 2. Parse static directory context
		const staticContexts = await parseContextFilesFromDir(staticContextDir, 'static')
		allContexts.push(...staticContexts)

		// Parse static config
		const staticConfigPath = join(staticContextDir, 'context-config.json')
		const staticConfig = await parseContextConfig(staticConfigPath)
		mergedConfig = mergeConfigs(mergedConfig, staticConfig)

		// 3. TODO: Dynamic subdirectory context would need the AI working directory
		// For now, we'll just support static directory context

		// 4. Apply configuration filters
		const filteredContexts = applyConfigFilters(allContexts, mergedConfig)

		return {
			contexts: filteredContexts,
			config: mergedConfig,
		}
	} catch (error) {
		console.warn(`Error parsing context from directory ${workingDir}:`, error)
		return {
			contexts: [],
			config: mergedConfig,
		}
	}
}

/**
 * Merge two context configurations according to the spec
 */
function mergeConfigs(base: ContextConfig, override: ContextConfig): ContextConfig {
	return {
		clientContext: {
			includeFiles: Array.from(
				new Set([
					...(base.clientContext.includeFiles || []),
					...(override.clientContext?.includeFiles || []),
				])
			),
			excludeFiles: Array.from(
				new Set([
					...(base.clientContext.excludeFiles || []),
					...(override.clientContext?.excludeFiles || []),
				])
			),
			ignoreGlobalContext:
				override.clientContext?.ignoreGlobalContext ?? base.clientContext.ignoreGlobalContext,
			ignoreAncestorContext:
				override.clientContext?.ignoreAncestorContext ?? base.clientContext.ignoreAncestorContext,
		},
	}
}

/**
 * Apply configuration filters to context files
 */
function applyConfigFilters(contexts: ParsedContext[], config: ContextConfig): ParsedContext[] {
	let filtered = [...contexts]

	// Apply ignoreGlobalContext
	if (config.clientContext.ignoreGlobalContext) {
		filtered = filtered.filter((ctx) => ctx.source !== 'global')
	}

	// Apply ignoreAncestorContext
	if (config.clientContext.ignoreAncestorContext) {
		filtered = filtered.filter((ctx) => ctx.source !== 'dynamic')
	}

	// TODO: Apply includeFiles and excludeFiles patterns
	// This would require implementing glob matching against file paths

	return filtered
}

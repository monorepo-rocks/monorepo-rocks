import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import matter from 'gray-matter'
import { z } from 'zod/v4'

export type RuleFrontmatter = z.infer<typeof RuleFrontmatter>
export const RuleFrontmatter = z.object({
	description: z.string().nullable().default(''),
	globs: z.string().nullable().optional(),
	alwaysApply: z.boolean().optional(),
}).transform(data => ({
	...data,
	description: data.description || 'Cursor rule',
}))

export type ParsedRule = z.infer<typeof ParsedRule>
export const ParsedRule = z.object({
	filename: z.string(),
	name: z.string(),
	frontmatter: RuleFrontmatter,
	content: z.string(),
	fullContent: z.string(),
})

/**
 * Find and parse all .mdc files in .cursor/rules directory
 */
export function parseRulesFromDir(rulesDir: string): ParsedRule[] {
	try {
		const files = readdirSync(rulesDir)
			.filter((file) => file.endsWith('.mdc'))
			.map((file) => join(rulesDir, file))

		return files.map(parseRuleFile).filter((rule): rule is ParsedRule => rule !== null)
	} catch (error) {
		console.warn(`Could not read rules directory: ${rulesDir}`, error)
		return []
	}
}

/**
 * Parse a single .mdc rule file
 */
export function parseRuleFile(filePath: string): ParsedRule | null {
	try {
		const fullContent = readFileSync(filePath, 'utf-8')
		const parsed = matter(fullContent)
		
		const frontmatterResult = RuleFrontmatter.safeParse(parsed.data)
		if (!frontmatterResult.success) {
			console.warn(`Invalid frontmatter in ${filePath}:`, frontmatterResult.error)
			return null
		}

		const filename = filePath.split('/').pop() || filePath
		const name = filename.replace('.mdc', '')

		return {
			filename,
			name,
			frontmatter: frontmatterResult.data,
			content: parsed.content.trim(),
			fullContent,
		}
	} catch (error) {
		console.warn(`Error parsing rule file ${filePath}:`, error)
		return null
	}
}

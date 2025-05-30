import { join } from 'node:path'
import { assert, describe, expect, it } from 'vitest'

import { parseContextFromDir } from './context-parser.js'

describe('context-parser', () => {
	const fixturesDir = join(__dirname, 'test/fixtures/context')

	it('should parse context files from .context directory', async () => {
		const { contexts } = await parseContextFromDir(fixturesDir)

		expect(contexts).toHaveLength(3)

		// Check typescript-conventions.md
		const tsContext = contexts.find((c) => c.name === 'typescript-conventions')
		expect(tsContext).toBeDefined()
		assert(tsContext)
		expect(tsContext.frontmatter.description).toBe(
			'TypeScript coding conventions and best practices'
		)
		expect(tsContext.frontmatter.appliesTo).toEqual(['**/*.ts', '**/*.tsx'])
		expect(tsContext.frontmatter.trigger).toBe('pattern')
		expect(tsContext.source).toBe('static')

		// Check general-guidelines.md
		const generalContext = contexts.find((c) => c.name === 'general-guidelines')
		assert(generalContext)
		expect(generalContext.frontmatter.trigger).toBe('always')

		// Check security-notes.txt (plain text file)
		const securityContext = contexts.find((c) => c.name === 'security-notes')
		assert(securityContext)
		expect(securityContext.frontmatter.trigger).toBe('manual') // default
		expect(securityContext.content).toContain('Never log sensitive data')
	})

	it('should parse context configuration', async () => {
		const { config } = await parseContextFromDir(fixturesDir)

		expect(config.clientContext.includeFiles).toEqual(['*'])
		expect(config.clientContext.excludeFiles).toEqual(['context-config.json', '**/*.private.md'])
		expect(config.clientContext.ignoreGlobalContext).toBe(false)
		expect(config.clientContext.ignoreAncestorContext).toBe(false)
	})

	it('should handle missing .context directory gracefully', async () => {
		const { contexts } = await parseContextFromDir('/nonexistent/directory')
		expect(contexts).toEqual([])
	})
})

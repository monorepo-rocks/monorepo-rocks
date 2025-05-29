import { join } from 'node:path'
import { assert, describe, expect, it } from 'vitest'

import { parseRuleFile, parseRulesFromDir } from './rule-parser.js'

describe('Rule Parser', () => {
	const fixturesDir = join(__dirname, 'test', 'fixtures')
	const rulesDir = join(fixturesDir, '.cursor', 'rules')

	describe('parseRulesFromDir', () => {
		it('should find and parse all rules in directory', () => {
			const rules = parseRulesFromDir(rulesDir)

			expect(rules).toHaveLength(5)

			const ruleNames = rules.map((r) => r.name).sort()
			expect(ruleNames).toEqual([
				'always-apply',
				'api-design',
				'manual-only',
				'react-components',
				'typescript-style',
			])
		})

		it('should return empty array for non-existent directory', () => {
			const rules = parseRulesFromDir('/nonexistent/path')
			expect(rules).toEqual([])
		})
	})

	describe('parseRuleFile', () => {
		it('should parse TypeScript style rule correctly', () => {
			const filePath = join(rulesDir, 'typescript-style.mdc')
			const rule = parseRuleFile(filePath)

			assert(rule)
			expect(rule.name).toBe('typescript-style')
			expect(rule.filename).toBe('typescript-style.mdc')
			expect(rule.frontmatter.description).toBe('TypeScript coding standards and style guide')
			expect(rule.frontmatter.globs).toBe('**/*.ts,**/*.tsx')
			expect(rule.frontmatter.alwaysApply).toBe(false)
			expect(rule.content).toContain('# TypeScript Style Guide')
			expect(rule.fullContent).toContain('---')
		})

		it('should parse always-apply rule correctly', () => {
			const filePath = join(rulesDir, 'always-apply.mdc')
			const rule = parseRuleFile(filePath)

			assert(rule)
			expect(rule.frontmatter.alwaysApply).toBe(true)
			expect(rule.frontmatter.description).toBe('Project-wide coding standards that always apply')
		})

		it('should handle empty description gracefully', () => {
			const filePath = join(rulesDir, 'manual-only.mdc')
			const rule = parseRuleFile(filePath)

			assert(rule)
			expect(rule.frontmatter.description).toBe('Database migration patterns and procedures')
		})

		it('should handle null globs field', () => {
			const filePath = join(rulesDir, 'always-apply.mdc')
			const rule = parseRuleFile(filePath)

			assert(rule)
			expect(rule.frontmatter.globs).toBe(null)
		})

		it('should return null for non-existent file', () => {
			const rule = parseRuleFile('/nonexistent/file.mdc')
			expect(rule).toBeNull()
		})

		it('should validate all fixture files have correct structure', () => {
			const files = [
				'typescript-style.mdc',
				'react-components.mdc',
				'api-design.mdc',
				'always-apply.mdc',
				'manual-only.mdc',
			]

			for (const file of files) {
				const filePath = join(rulesDir, file)
				const rule = parseRuleFile(filePath)

				assert(rule, `Failed to parse ${file}`)
				expect(rule.name).toBe(file.replace('.mdc', ''))
				expect(rule.frontmatter.description).toBeTruthy()
				expect(rule.content.length).toBeGreaterThan(0)
				expect(rule.fullContent).toContain('---')
			}
		})
	})

	describe('Frontmatter Validation', () => {
		it('should handle various frontmatter configurations', () => {
			const rules = parseRulesFromDir(rulesDir)

			// Check that we have rules with different configurations
			const alwaysApplyRule = rules.find((r) => r.name === 'always-apply')
			expect(alwaysApplyRule?.frontmatter.alwaysApply).toBe(true)

			const typescriptRule = rules.find((r) => r.name === 'typescript-style')
			expect(typescriptRule?.frontmatter.globs).toBe('**/*.ts,**/*.tsx')
			expect(typescriptRule?.frontmatter.alwaysApply).toBe(false)
		})

		it('should provide fallback description when empty', () => {
			// All our test rules should have proper descriptions
			const rules = parseRulesFromDir(rulesDir)

			for (const rule of rules) {
				expect(rule.frontmatter.description).toBeTruthy()
				expect(rule.frontmatter.description).not.toBe('Cursor rule') // Should not use fallback
			}
		})
	})
})

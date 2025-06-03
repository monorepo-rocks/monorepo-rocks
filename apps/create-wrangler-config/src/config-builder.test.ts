import { describe, expect, it } from 'vitest'
import { buildWranglerConfig, formatWranglerConfig, type WorkerConfigOptions } from './config-builder.js'

describe('config-builder', () => {
	describe('buildWranglerConfig', () => {
		it('should build config with entry point only', () => {
			const options: WorkerConfigOptions = {
				name: 'test-worker',
				features: ['entryPoint'],
				entryPoint: 'src/index.ts',
			}

			const config = buildWranglerConfig(options)

			expect(config.name).toBe('test-worker')
			expect(config.main).toBe('src/index.ts')
			expect(config.observability).toEqual({ enabled: true })
			expect(config.assets).toBeUndefined()
			expect(config.compatibility_date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
		})

		it('should build config with static assets only', () => {
			const options: WorkerConfigOptions = {
				name: 'test-worker',
				features: ['staticAssets'],
				assetsDirectory: './public',
			}

			const config = buildWranglerConfig(options)

			expect(config.name).toBe('test-worker')
			expect(config.main).toBeUndefined()
			expect(config.observability).toBeUndefined()
			expect(config.assets).toEqual({ directory: './public' })
			expect(config.compatibility_date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
		})

		it('should build config with both entry point and static assets', () => {
			const options: WorkerConfigOptions = {
				name: 'test-worker',
				features: ['entryPoint', 'staticAssets'],
				entryPoint: 'src/index.ts',
				assetsDirectory: './public',
			}

			const config = buildWranglerConfig(options)

			expect(config.name).toBe('test-worker')
			expect(config.main).toBe('src/index.ts')
			expect(config.observability).toEqual({ enabled: true })
			expect(config.assets).toEqual({
				directory: './public',
				binding: 'ASSETS',
			})
			expect(config.compatibility_date).toMatch(/^\d{4}-\d{2}-\d{2}$/)
		})

		it('should validate worker name', () => {
			const options: WorkerConfigOptions = {
				name: 'invalid name with spaces',
				features: ['entryPoint'],
				entryPoint: 'src/index.ts',
			}

			expect(() => buildWranglerConfig(options)).toThrow()
		})

		it('should require at least one feature', () => {
			const options: WorkerConfigOptions = {
				name: 'test-worker',
				features: [],
			}

			expect(() => buildWranglerConfig(options)).toThrow()
		})
	})

	describe('formatWranglerConfig', () => {
		it('should format config as valid JSONC', () => {
			const config = {
				name: 'test-worker',
				main: 'src/index.ts',
				compatibility_date: '2024-01-15',
				observability: { enabled: true },
			}

			const formatted = formatWranglerConfig(config)

			expect(formatted).toContain('"name": "test-worker",')
			expect(formatted).toContain('"main": "src/index.ts",')
			expect(formatted).toContain('"compatibility_date": "2024-01-15",')
			expect(formatted).toContain('"observability": {')
			expect(formatted).toContain('"enabled": true')
			expect(formatted).not.toContain(',}') // No trailing comma before closing brace
		})
	})
})

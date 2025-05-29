import { $ } from 'zx'

export async function setup() {
	console.log('Building CLI before running tests...')

	try {
		// Build the CLI
		await $`pnpm build`
		console.log('✅ CLI built successfully')
	} catch (error) {
		console.error('❌ Failed to build CLI:', error)
		throw error
	}
}

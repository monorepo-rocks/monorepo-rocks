#!/usr/bin/env node
import { spawn } from 'child_process'
import { fileURLToPath } from 'url'
import { dirname, join } from 'path'
import { platform, arch } from 'os'
import { existsSync } from 'fs'

const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

function getBinaryName(): string {
	const os = platform()
	const architecture = arch()
	
	let binaryName = 'mcpce'
	
	if (os === 'win32') {
		binaryName += '.exe'
	}
	
	// Map platform/arch to binary path
	const platformMap: Record<string, string> = {
		'darwin-arm64': 'mcpce-darwin-arm64',
		'darwin-x64': 'mcpce-darwin-amd64',
		'linux-arm64': 'mcpce-linux-arm64',
		'linux-x64': 'mcpce-linux-amd64',
		'win32-x64': 'mcpce-windows-amd64.exe',
	}
	
	const platformKey = `${os}-${architecture}`
	return platformMap[platformKey] || binaryName
}

function findBinary(): string {
	const binaryName = getBinaryName()
	
	// Look for binary in several locations
	const possiblePaths = [
		// In production (npm package)
		join(__dirname, '..', '..', 'bin', binaryName),
		// In development
		join(__dirname, '..', '..', '..', '..', 'bin', binaryName),
		// Fallback to cwd
		join(process.cwd(), 'bin', binaryName),
	]
	
	for (const path of possiblePaths) {
		if (existsSync(path)) {
			return path
		}
	}
	
	throw new Error(
		`Could not find mcpce binary. Expected at one of:\n${possiblePaths.join('\n')}`
	)
}

function main() {
	try {
		const binaryPath = findBinary()
		const args = process.argv.slice(2)
		
		// Spawn the Go binary with the same arguments
		const child = spawn(binaryPath, args, {
			stdio: 'inherit',
			env: process.env,
		})
		
		child.on('error', (err) => {
			console.error(`Failed to start mcpce: ${err.message}`)
			process.exit(1)
		})
		
		child.on('exit', (code) => {
			process.exit(code || 0)
		})
		
		// Handle signals
		process.on('SIGINT', () => {
			child.kill('SIGINT')
		})
		
		process.on('SIGTERM', () => {
			child.kill('SIGTERM')
		})
	} catch (err) {
		console.error(err instanceof Error ? err.message : String(err))
		process.exit(1)
	}
}

main()
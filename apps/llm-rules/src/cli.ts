import 'zx/globals'

import { program } from '@commander-js/extra-typings'
import { catchProcessError } from '@jahands/cli-tools/proc'

import { version } from '../package.json'
import { startMCPServer } from './mcp-server.js'

export const runCLI = () =>
	program
		.name('llm-rules')
		.description('A local MCP server for LLM rules')
		.version(version)
		.option(
			'-d, --dir <directory>',
			'Working directory to scan for .cursor/rules (defaults to current directory)'
		)
		.action(async (options) => {
			await startMCPServer(options.dir)
		})

		// Don't hang for unresolved promises
		.hook('postAction', () => process.exit(0))
		.parseAsync()
		.catch(catchProcessError())

import 'zx/globals'

import { program } from '@commander-js/extra-typings'
import { validateArg } from '@jahands/cli-tools/args'
import { catchProcessError } from '@jahands/cli-tools/proc'
import { z } from 'zod/v4'

import { version } from '../package.json'

export const runCLI = () =>
	program
		.name('llm-rules')
		.description('A local MCP server for LLM rules')
		.version(version)
		.option('-p, --port <port>', 'The port to run the server on', validateArg(z.coerce.number()))
		.action(async () => {
			console.log('todo')
		})

		// Don't hang for unresolved promises
		.hook('postAction', () => process.exit(0))
		.parseAsync()
		.catch(catchProcessError())

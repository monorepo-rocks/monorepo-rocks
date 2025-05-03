import 'zx/globals'

import { program } from '@commander-js/extra-typings'
import { catchProcessError } from '@jahands/cli-tools/proc'

void program
	.name('create-workers-monorepo')
	.description('A CLI for creating a workers monorepo')

	.action(async () => {
		console.log('Hello, world!')
	})

	// Don't hang for unresolved promises
	.hook('postAction', () => process.exit(0))
	.parseAsync()
	.catch(catchProcessError())

import { program } from '@commander-js/extra-typings'
import { catchProcessError } from '@jahands/cli-tools/proc'
import { $ } from 'zx/core'

void program
	.name('create-workers-monorepo')
	.description('A CLI for creating a workers monorepo')

	.action(async () => {
		await $`ls -lh`
		console.log('Hello, world!')
	})

	// Don't hang for unresolved promises
	.hook('postAction', () => process.exit(0))
	.parseAsync()
	.catch(catchProcessError())

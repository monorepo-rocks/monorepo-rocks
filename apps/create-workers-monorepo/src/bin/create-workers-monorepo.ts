import 'zx/globals'

import { program } from '@commander-js/extra-typings'
import { validateArg } from '@jahands/cli-tools/args'
import { catchProcessError } from '@jahands/cli-tools/proc'

import { createMonorepo, RepoName } from '../monorepo'

void program
	.name('create-workers-monorepo')
	.description('A CLI for creating a Workers monorepo')
	.option('-n, --name <name>', 'The name of the monorepo', validateArg(RepoName))
	.action(async (opts) => {
		await createMonorepo(opts)
	})

	// Don't hang for unresolved promises
	.hook('postAction', () => process.exit(0))
	.parseAsync()
	.catch(catchProcessError())

import 'zx/globals'

import { program } from '@commander-js/extra-typings'
import { input } from '@inquirer/prompts'
import { catchProcessError } from '@jahands/cli-tools/proc'
import { z } from 'zod'

const RepoName = z.string().regex(/^[a-z0-9-_.]+$/i)

void program
	.name('create-workers-monorepo')
	.description('A CLI for creating a workers monorepo')

	.action(async () => {
		const name = await input({
			message: 'What would you like to name your monorepo?',
			validate: (value) => {
				const trimmedValue = value.trim()
				if (trimmedValue === '') {
					return 'Repository name cannot be empty.'
				}
				if (RepoName.safeParse(trimmedValue).success) {
					return true
				}
				return 'The repository name can only contain ASCII letters, digits, and the characters ., -, and _.'
			},
		}).then((answer) => answer.trim())

		echo(chalk.green(`Creating monorepo with name: ${chalk.white(name)}`))
	})

	// Don't hang for unresolved promises
	.hook('postAction', () => process.exit(0))
	.parseAsync()
	.catch(catchProcessError())

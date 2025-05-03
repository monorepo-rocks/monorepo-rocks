import { input } from '@inquirer/prompts'
import { z } from 'zod'
import { fs } from 'zx'

export type RepoName = z.infer<typeof RepoName>
export const RepoName = z.string().regex(/^[a-z0-9-_.]+$/i)

export interface CreateMonorepoOptions {
	name?: string
}

async function promptRepoName(): Promise<string> {
	return input({
		message: 'What do you want to name your monorepo?',
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
}

export async function createMonorepo(opts: CreateMonorepoOptions = {}) {
	const name = opts.name ?? (await promptRepoName())
	echo(chalk.green(`Creating monorepo with name: ${chalk.white(name)}`))

	const targetDir = path.resolve(process.cwd(), name)
	if (fs.existsSync(targetDir)) {
		const files = fs.readdirSync(targetDir)
		if (files.length > 0) {
			throw new Error(
				`Directory "${name}" already exists and is not empty. Please choose a different name or remove the existing directory.`
			)
		}
		echo(chalk.yellow(`Directory "${name}" already exists but is empty. Proceeding...`))
	}
}

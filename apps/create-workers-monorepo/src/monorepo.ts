import os from 'node:os'
import path from 'node:path'
import { input } from '@inquirer/prompts'
import { cliError } from '@jahands/cli-tools/errors'
import { z } from 'zod'
import { $, chalk, fs } from 'zx'

export type RepoName = z.infer<typeof RepoName>
export const RepoName = z.string().regex(/^(?!\.+$)(?!_+$)[a-z0-9-_.]+$/i)

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

async function cloneTemplateRepo(templateUrl: string, targetDir: string) {
	await $`git clone --depth 1 ${templateUrl} ${targetDir}`
	fs.rmSync(path.join(targetDir, '.git'), { recursive: true, force: true })
}

export async function createMonorepo(opts: CreateMonorepoOptions) {
	const name = opts.name ?? (await promptRepoName())
	echo(chalk.green(`Creating monorepo with name: ${chalk.white(name)}`))

	const targetDir = path.resolve(process.cwd(), name)
	let dirExisted = false
	if (fs.existsSync(targetDir)) {
		dirExisted = true
		const files = fs.readdirSync(targetDir)
		if (files.length > 0) {
			throw cliError(
				`Directory "${name}" already exists and is not empty. Please choose a different name or remove the existing directory.`
			)
		}
		echo(chalk.yellow(`Directory "${name}" already exists but is empty. Proceeding...`))
	} else {
		fs.mkdirSync(targetDir)
	}

	const templateUrl = 'https://github.com/jahands/workers-monorepo-template.git'

	try {
		echo(chalk.blue(`Cloning template from ${templateUrl}...`))
		await cloneTemplateRepo(templateUrl, targetDir)

		echo(chalk.green('Template files copied successfully!'))
	} catch (e) {
		// Clean up the target directory if it was created by this script
		if (!fs.existsSync(path.resolve(process.cwd(), name))) {
			// only remove the directory if it was created by this script
			if (!dirExisted) {
				fs.rmSync(targetDir, { recursive: true, force: true })
			}
		}
		throw cliError(`Failed to create monorepo: ${e instanceof Error ? e.message : String(e)}`)
	}
}

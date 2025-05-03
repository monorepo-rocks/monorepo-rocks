import os from 'node:os'
import path from 'node:path'
import { input } from '@inquirer/prompts'
import { cliError } from '@jahands/cli-tools/errors'
import { z } from 'zod'
import { $, chalk, fs } from 'zx'

export async function ensurePrerequisites() {
	if (!(await which('git', { nothrow: true }))) {
		throw cliError('git is required to create a monorepo. Please install it and try again.')
	}
}

export const RepoName = z.string().regex(/^(?!\.+$)(?!_+$)[a-z0-9-_.]+$/i)

export interface CreateMonorepoOptions {
	name?: string
}

async function promptRepoName(): Promise<string> {
	return input({
		message: 'What do you want to name your monorepo?',
		validate: async (value) => {
			const trimmedValue = value.trim()
			if (trimmedValue === '') {
				return 'Repository name cannot be empty.'
			}

			const targetDir = path.resolve(process.cwd(), trimmedValue)
			if (fs.existsSync(targetDir)) {
				try {
					const files = fs.readdirSync(targetDir)
					if (files.length > 0) {
						return `Directory "${trimmedValue}" already exists and is not empty. Please choose a different name or remove the existing directory.`
					}
				} catch (e) {
					// Handle potential errors reading the directory (e.g., permissions)
					return `Could not check directory "${trimmedValue}": ${e instanceof Error ? e.message : String(e)}`
				}
			}

			if (RepoName.safeParse(trimmedValue).success) {
				return true
			} else {
				return 'The repository name can only contain ASCII letters, digits, and the characters ., -, and _.'
			}
		},
	}).then((answer) => answer.trim())
}

export async function createMonorepo(opts: CreateMonorepoOptions) {
	await ensurePrerequisites()

	const name = opts.name ?? (await promptRepoName())
	echo(chalk.blue(`Creating monorepo with name: ${chalk.white(name)}`))

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
	}

	const templateUrl = 'https://github.com/jahands/workers-monorepo-template.git'

	try {
		await $`git clone --depth 1 ${templateUrl} ${targetDir}`.quiet()
		fs.rmSync(path.join(targetDir, '.git'), { recursive: true, force: true })
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

	echo(chalk.dim(`Initializing git repository...`))
	cd(targetDir)
	await $`git init`.quiet()
	await $`git add .`.quiet()
	await $`git commit -m "Initial commit"`.quiet()

	echo(chalk.green(`Monorepo created successfully!`))
	echo(chalk.dim(`  ${targetDir}`))
}

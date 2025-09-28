#!/usr/bin/env node

import 'zx/globals'

import { promises as fs } from 'node:fs'
import { Command } from '@commander-js/extra-typings'
import { catchProcessError, cliError } from '@jahands/cli-tools'

import { z } from '@repo/zod'

import { getTurboConfig } from '../config'
import { getTurboJsonPath } from '../path'

export const program = new Command('turbo-config').description(
	'Commands for working with turbo.config.ts and turbo.json'
)

program
	.command('generate')
	.description('Generate turbo.json')
	.action(async () => {
		const config = await getTurboConfig()
		const turboJsonPath = getTurboJsonPath()

		let currentConfig: string | null = null
		if (await fileExists(turboJsonPath)) {
			currentConfig = JSON.stringify(
				z.looseObject({}).parse(await readJsonFile(turboJsonPath)),
				null,
				'\t'
			)
		}

		const newConfig = JSON.stringify(config, null, '\t')

		// only update if the config has changed to avoid unnecessary formatting
		if (newConfig !== currentConfig) {
			await fs.writeFile(turboJsonPath, newConfig, 'utf8')
			echo(chalk.green('turbo.json updated'))
		} else {
			echo(chalk.green('turbo.json is up to date'))
		}
	})

program
	.command('check')
	.description('Ensure turbo.config.ts is valid and matches turbo.json')
	.action(async () => {
		const config = await getTurboConfig()
		const turboJsonPath = getTurboJsonPath()

		// note: looseObject is used to ensure keys are sorted consistently
		const jsoncConfig = z.looseObject({}).parse(await readJsonFile(turboJsonPath))
		const matches = JSON.stringify(config) === JSON.stringify(jsoncConfig)
		if (!matches) {
			throw cliError(
				'turbo.config.ts does not match turbo.json - run `just generate-turbo-config` to update turbo.json'
			)
		}
		echo(chalk.green('turbo.json is up to date'))
	})

program
	// don't hang for unresolved promises
	.hook('postAction', () => process.exit(0))
	.parseAsync()
	.catch(catchProcessError())

async function fileExists(filePath: string): Promise<boolean> {
	try {
		await fs.access(filePath)
		return true
	} catch (error) {
		if ((error as NodeJS.ErrnoException).code === 'ENOENT') {
			return false
		}
		throw error
	}
}

async function readJsonFile(filePath: string): Promise<unknown> {
	const contents = await fs.readFile(filePath, 'utf8')
	return JSON.parse(contents)
}

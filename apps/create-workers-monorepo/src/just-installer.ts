import { confirm } from '@inquirer/prompts'
import Table from 'cli-table3'

export async function checkAndInstallJust(): Promise<void> {
	// Check if just is already installed
	if (await which('just', { nothrow: true })) {
		return
	}

	echo(chalk.yellow('\n⚠️  just is not installed on your system.'))
	echo(chalk.dim('just is required to run tasks in the monorepo.'))
	echo(chalk.dim('You can install just manually from: https://just.systems/man/en/packages.html\n'))

	const shouldInstall = await confirm({
		message: 'Would you like to install just now?',
		default: true,
	})

	if (!shouldInstall) {
		echo(
			chalk.blue(
				'\nYou can install just later by visiting: https://just.systems/man/en/packages.html'
			)
		)
		echo(
			chalk.dim("The monorepo will still be created, but you won't be able to use just commands.\n")
		)
		return
	}

	const platform = process.platform
	const installCommand = await getInstallCommand(platform)

	if (!installCommand) {
		echo(chalk.yellow('\n⚠️  No automatic installation method found for your system.'))
		echo(chalk.blue('Please install just manually from: https://just.systems/man/en/packages.html'))
		echo(
			chalk.dim("The monorepo will still be created, but you won't be able to use just commands.\n")
		)
		return
	}

	const table = new Table({
		head: [chalk.blueBright('Install Command')],
	})
	table.push([installCommand.cmd])
	echo(table.toString())

	const confirmInstall = await confirm({
		message: `Run this command to install just?`,
		default: true,
	})

	if (!confirmInstall) {
		echo(
			chalk.blue(
				'\nYou can install just later by visiting: https://just.systems/man/en/packages.html'
			)
		)
		echo(
			chalk.dim("The monorepo will still be created, but you won't be able to use just commands.\n")
		)
		return
	}

	try {
		echo(chalk.dim('Installing just...'))

		await $({
			stdio: 'inherit',
		})`sh -c ${installCommand.cmd}`.verbose()

		// Verify installation
		if (await which('just', { nothrow: true })) {
			echo(chalk.green('✅ just installed successfully!\n'))
		} else {
			echo(chalk.yellow('⚠️  just was installed but not found in PATH.'))
			echo(chalk.dim('You may need to restart your terminal or add it to your PATH.\n'))
		}
	} catch (error) {
		echo(chalk.red('❌ Failed to install just automatically.'))
		echo(chalk.blue('Please install just manually from: https://just.systems/man/en/packages.html'))
		echo(chalk.dim(`Error: ${error instanceof Error ? error.message : String(error)}\n`))
	}
}

interface InstallCommand {
	cmd: string
}

async function getInstallCommand(platform: NodeJS.Platform): Promise<InstallCommand | null> {
	switch (platform) {
		case 'win32':
			// Windows - use winget
			if (await which('winget', { nothrow: true })) {
				return { cmd: 'winget install --id Casey.Just --exact' }
			}
			break

		case 'darwin':
			// macOS - prefer mise, then brew
			if (await which('mise', { nothrow: true })) {
				return { cmd: 'mise use -g just' }
			}
			if (await which('brew', { nothrow: true })) {
				return { cmd: 'brew install just' }
			}
			break

		case 'linux':
			// Linux - check for different package managers
			if (await which('apt', { nothrow: true })) {
				// Debian/Ubuntu
				return { cmd: 'sudo apt update && sudo apt install -y just' }
			}
			if (await which('dnf', { nothrow: true })) {
				// Fedora
				return { cmd: 'sudo dnf install -y just' }
			}
			if (await which('pacman', { nothrow: true })) {
				// Arch Linux
				return { cmd: 'sudo pacman -S --noconfirm just' }
			}
			break
	}

	return null
}

interface Editor {
	name: string
	command: string
}

const editors = [
	{ name: 'Cursor', command: 'cursor' },
	{ name: 'Visual Studio Code', command: 'code' },
] as const satisfies Editor[]

export async function getAvailableEditors(): Promise<Editor[]> {
	const availableEditors: Editor[] = []
	await Promise.all(
		editors.map(async (editor) => {
			if (await which(editor.command, { nothrow: true })) {
				availableEditors.push(editor)
			}
		})
	)
	availableEditors.sort((a, b) => a.name.localeCompare(b.name))
	return availableEditors
}

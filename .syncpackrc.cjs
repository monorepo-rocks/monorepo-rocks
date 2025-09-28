// @ts-check
/** @type {import("syncpack").RcFile} */
const config = {
	indent: '\t',
	lintFormatting: false, // handled by prettier
	versionGroups: [
		{
			label: 'use published turbo-config',
			packages: ['monorepo-rocks-monorepo'],
			dependencies: ['turbo-config'],
		},
		{
			label: 'local packages',
			packages: ['**'],
			dependencies: ['$LOCAL'],
			dependencyTypes: ['!local'], // Exclude the local package itself
			pinVersion: 'workspace:*',
		},
		{
			label: 'ignore zod peer deps for 3.25.76 compatibility',
			packages: ['dagger-env'],
			dependencies: ['zod'],
			dependencyTypes: ['peer'],
			specifierTypes: ['range'],
			isIgnored: true,
		},
	],
	semverGroups: [
		{
			label: 'pin all deps',
			range: '',
			dependencies: ['**'],
			packages: ['**'],
			// url is not supported so we need to exclude it
			// to allow using deps from pkg.pr.new
			specifierTypes: ['!url'],
		},
	],
}

module.exports = config

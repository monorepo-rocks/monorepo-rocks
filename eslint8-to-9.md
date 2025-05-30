# ESLint 8 to 9 Migration Guide for Workers Monorepo Template

> **Instructions for AI Assistants**: This file contains a complete migration guide. When a user shares this file with you, follow ALL steps in this guide to migrate their repository from ESLint 8 to ESLint 9. Work through each step systematically, creating/updating files as specified. Use the migration checklist at the end to verify completion. Do not ask for additional instructions - this guide is complete and self-contained.

This guide provides step-by-step instructions for migrating a repository based on the [workers-monorepo-template](https://github.com/jahands/workers-monorepo-template) from ESLint 8 (legacy config) to ESLint 9 (flat config). This guide is designed to be used by LLMs for automated migration.

## Overview

**Before (ESLint 8):**

- Uses `.eslintrc.cjs` files throughout the monorepo
- Single shared config: `packages/eslint-config/default.cjs`
- Script command: `run-eslint-default`
- Legacy configuration format

**After (ESLint 9):**

- Uses `eslint.config.ts` files throughout the monorepo
- Multiple shared configs with TypeScript support
- Script command: `run-eslint`
- Flat configuration format with better TypeScript integration

## Migration Steps

### Step 1: Update Shared ESLint Config Package

#### 1.1 Update packages/eslint-config/package.json

Replace the entire file with:

```json
{
	"name": "@repo/eslint-config",
	"version": "0.2.3",
	"private": true,
	"sideEffects": false,
	"exports": {
		".": "./src/default.config.ts",
		"./react": "./src/react.config.ts"
	},
	"devDependencies": {
		"@eslint/compat": "1.2.9",
		"@eslint/js": "9.27.0",
		"@types/eslint": "9.6.1",
		"@types/node": "22.15.27",
		"@typescript-eslint/eslint-plugin": "8.32.1",
		"@typescript-eslint/parser": "8.32.1",
		"eslint": "9.27.0",
		"eslint-config-prettier": "10.1.5",
		"eslint-config-turbo": "2.5.3",
		"eslint-import-resolver-typescript": "4.4.1",
		"eslint-plugin-astro": "1.3.1",
		"eslint-plugin-import": "2.31.0",
		"eslint-plugin-jsx-a11y": "6.10.2",
		"eslint-plugin-only-warn": "1.1.0",
		"eslint-plugin-react": "7.37.5",
		"eslint-plugin-react-hooks": "5.2.0",
		"eslint-plugin-unused-imports": "4.1.4",
		"jiti": "2.4.2",
		"typescript": "5.5.4",
		"typescript-eslint": "8.33.0",
		"vitest": "3.1.4"
	}
}
```

#### 1.2 Create packages/eslint-config/src/ directory structure

Create `packages/eslint-config/src/` directory and add the following files:

#### 1.3 Create packages/eslint-config/src/helpers.ts

```typescript
import { existsSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { includeIgnoreFile } from '@eslint/compat'

import type { FlatConfig } from '@eslint/compat'

export function getDirname(importMetaUrl: string) {
	const __filename = fileURLToPath(importMetaUrl)
	return path.dirname(__filename)
}

export function getGitIgnoreFiles(importMetaUrl: string) {
	// always include the root gitignore file
	const rootGitignorePath = fileURLToPath(new URL('../../../.gitignore', import.meta.url))

	const ignoreFiles: FlatConfig[] = [includeIgnoreFile(rootGitignorePath)]

	const packageDir = getDirname(importMetaUrl)
	const packageGitignorePath = path.join(packageDir, '.gitignore')
	if (existsSync(packageGitignorePath)) {
		ignoreFiles.push(includeIgnoreFile(packageGitignorePath))
	}

	return ignoreFiles
}

export function getTsconfigRootDir(importMetaUrl: string) {
	const tsconfigRootDir = getDirname(importMetaUrl)
	return existsSync(path.join(tsconfigRootDir, 'tsconfig.json')) ? tsconfigRootDir : undefined
}
```

#### 1.4 Create packages/eslint-config/src/default.config.ts

```typescript
import { FlatCompat } from '@eslint/eslintrc'
import eslint from '@eslint/js'
import tsEslintPlugin from '@typescript-eslint/eslint-plugin'
import tsEslintParser from '@typescript-eslint/parser'
import eslintConfigPrettier from 'eslint-config-prettier'
import turboConfig from 'eslint-config-turbo/flat'
// @ts-ignore eslint-plugin-import has no types
import * as importPlugin from 'eslint-plugin-import'
import unusedImportsPlugin from 'eslint-plugin-unused-imports'
import { defineConfig } from 'eslint/config'
import tseslint from 'typescript-eslint'

import { getDirname, getGitIgnoreFiles, getTsconfigRootDir } from './helpers'

export { defineConfig }

const compat = new FlatCompat({
	// This helps FlatCompat resolve plugins relative to this config file
	baseDirectory: getDirname(import.meta.url),
})

export function getConfig(importMetaUrl: string) {
	return defineConfig([
		// Global ignores
		{
			ignores: [
				'.*.{js,cjs}',
				'**/*.{js,cjs}',
				'**/node_modules/**',
				'**/dist/**',
				'eslint.config.ts',
				'**/eslint.config.ts',
				'**/worker-configuration.d.ts',
			],
		},

		...getGitIgnoreFiles(importMetaUrl),

		eslint.configs.recommended,
		tseslint.configs.recommended,
		importPlugin.flatConfigs?.recommended,
		...turboConfig,

		// TypeScript Configuration
		{
			files: ['**/*.{ts,tsx,mts}'],
			languageOptions: {
				parser: tsEslintParser,
				parserOptions: {
					ecmaFeatures: {
						jsx: true,
					},
					sourceType: 'module',
					project: true,
					tsconfigRootDir: getTsconfigRootDir(importMetaUrl),
				},
			},
			plugins: {
				'unused-imports': unusedImportsPlugin,
			},
			settings: {
				'import/resolver': {
					typescript: {
						project: './tsconfig.json',
					},
				},
				'import/parsers': {
					'@typescript-eslint/parser': ['.ts', '.tsx', '*.mts'],
				},
			},
			rules: {
				...tsEslintPlugin.configs.recommended.rules,
				...importPlugin.configs?.typescript.rules,

				'@typescript-eslint/consistent-type-imports': ['warn', { prefer: 'type-imports' }],
				'@typescript-eslint/explicit-function-return-type': 'off',
				'@typescript-eslint/ban-ts-comment': 'off',
				'@typescript-eslint/no-floating-promises': 'warn',
				'unused-imports/no-unused-imports': 'warn',
				'@typescript-eslint/array-type': ['warn', { default: 'array-simple' }],
				'@typescript-eslint/no-unused-vars': [
					'warn',
					{
						argsIgnorePattern: '^_',
						varsIgnorePattern: '^_',
					},
				],
				'@typescript-eslint/no-empty-object-type': 'off',
				'@typescript-eslint/no-explicit-any': 'off',
				'import/no-named-as-default': 'off',
				'import/no-named-as-default-member': 'off',
				'prefer-const': 'warn',
				'no-mixed-spaces-and-tabs': ['error', 'smart-tabs'],
				'no-empty': 'warn',

				// Add Prettier last to override other formatting rules
				...eslintConfigPrettier.rules,
			},
		},

		// Import plugin's TypeScript specific rules using FlatCompat
		...compat.extends('plugin:import/typescript').map((config) => ({
			...config,
			files: ['**/*.{ts,tsx,mjs}'],
		})),

		// Configuration for Node files
		{
			files: ['**/*.spec.ts', '**/*.test.ts', '**/test/**/*.ts', '**/mocks.ts'],
			rules: {
				// this is having issues with @cloudflare/vitest-pool-workers types
				'import/no-unresolved': 'off',
			},
		},
		{
			files: ['**/*.ts'],
			rules: {
				// ignoring fully for now due to issues
				'import/no-unresolved': 'off',
			},
		},
		{
			files: ['tailwind.config.ts', 'postcss.config.mjs'],
			rules: {
				'@typescript-eslint/no-require-imports': 'off',
			},
		},

		// Prettier (should be last to override other formatting rules)
		{ rules: eslintConfigPrettier.rules },
	])
}
```

#### 1.5 Create packages/eslint-config/src/react.config.ts

```typescript
import tsEslintParser from '@typescript-eslint/parser'
import eslintConfigPrettier from 'eslint-config-prettier'
import react from 'eslint-plugin-react'
import * as reactHooks from 'eslint-plugin-react-hooks'
import unusedImportsPlugin from 'eslint-plugin-unused-imports'

import { defineConfig, getConfig } from './default.config'
import { getTsconfigRootDir } from './helpers'

export function getReactConfig(importMetaUrl: string) {
	return defineConfig([
		...getConfig(importMetaUrl),
		{
			files: ['**/*.{js,jsx,mjs,cjs,ts,tsx}'],
			plugins: {
				react,
				'unused-imports': unusedImportsPlugin,
			},
			languageOptions: {
				parser: tsEslintParser,
				parserOptions: {
					ecmaFeatures: {
						jsx: true,
					},
					sourceType: 'module',
					project: true,
					tsconfigRootDir: getTsconfigRootDir(importMetaUrl),
				},
			},
		},
		reactHooks.configs['recommended-latest'],
		{
			rules: {
				// this commonly causes false positives with Hono middleware
				// that have a similar naming scheme (e.g. useSentry())
				'react-hooks/rules-of-hooks': 'off',
			},
		},

		// Prettier (should be last to override other formatting rules)
		{ rules: eslintConfigPrettier.rules },
	])
}
```

#### 1.6 Create packages/eslint-config/eslint.config.ts

```typescript
import { defineConfig, getConfig } from './src/default.config'

const config = getConfig(import.meta.url)

export default defineConfig([...config])
```

#### 1.7 Create packages/eslint-config/tsconfig.json

**Important:** Use this exact content - do NOT add any compilerOptions or other fields:

```json
{
	"extends": "@repo/typescript-config/lib.json",
	"include": ["*.ts", "src/**/*.ts"],
	"exclude": ["node_modules/"]
}
```

#### 1.8 Delete old files

Delete `packages/eslint-config/default.cjs`.

### Step 2: Update TypeScript Configuration

Add `eslint.config.ts` exclusion to shared TypeScript configs:

#### 2.1 Update packages/typescript-config/base.json

Add `"${configDir}/eslint.config.ts"` to the exclude array:

```json
{
	"$schema": "https://json.schemastore.org/tsconfig",
	"display": "Default",
	"include": ["${configDir}/**/*.ts", "${configDir}/**/*.tsx"],
	"exclude": ["${configDir}/node_modules/", "${configDir}/dist/", "${configDir}/eslint.config.ts"],
	"compilerOptions": {
		"composite": false,
		"declaration": true,
		"declarationMap": true,
		"esModuleInterop": true,
		"forceConsistentCasingInFileNames": true,
		"inlineSources": false,
		"isolatedModules": true,
		"moduleResolution": "bundler",
		"noUnusedLocals": false,
		"noUnusedParameters": false,
		"preserveWatchOutput": true,
		"skipLibCheck": true,
		"noImplicitOverride": true,
		"strict": true,
		"noEmit": true,
		"resolveJsonModule": true
	}
}
```

#### 2.2 Update packages/typescript-config/workers.json

Add `"${configDir}/eslint.config.ts"` to the exclude array:

```json
{
	"$schema": "https://json.schemastore.org/tsconfig",
	"include": [
		"${configDir}/worker-configuration.d.ts",
		"${configDir}/env.d.ts",
		"${configDir}/**/*.ts",
		"${configDir}/**/*.tsx"
	],
	"exclude": ["${configDir}/node_modules/", "${configDir}/dist/", "${configDir}/eslint.config.ts"],
	"compilerOptions": {
		"target": "es2022",
		"lib": ["es2022"],
		"jsx": "react",
		"module": "es2022",
		"moduleResolution": "bundler",
		"types": ["./worker-configuration.d.ts", "@cloudflare/vitest-pool-workers"],
		"resolveJsonModule": true,
		"allowJs": true,
		"checkJs": false,
		"noEmit": true,
		"isolatedModules": true,
		"allowSyntheticDefaultImports": true,
		"forceConsistentCasingInFileNames": true,
		"strict": true,
		"skipLibCheck": true,
		"esModuleInterop": true,
		"moduleDetection": "force"
	}
}
```

#### 2.3 Update packages/typescript-config/workers-lib.json

Add `"${configDir}/eslint.config.ts"` to the exclude array:

```json
{
	"$schema": "https://json.schemastore.org/tsconfig",
	"extends": "@repo/typescript-config/workers.json",
	"include": ["${configDir}/**/*.ts", "${configDir}/**/*.tsx"],
	"exclude": ["${configDir}/node_modules/", "${configDir}/dist/", "${configDir}/eslint.config.ts"],
	"compilerOptions": {
		"types": ["@cloudflare/workers-types", "@cloudflare/vitest-pool-workers"]
	}
}
```

### Step 3: Update Root Configuration

#### 3.1 Replace .eslintrc.cjs with eslint.config.ts

Delete `.eslintrc.cjs` and create `eslint.config.ts`:

```typescript
import { defineConfig, getConfig } from '@repo/eslint-config'

const config = getConfig(import.meta.url)

export default defineConfig([...config])
```

#### 3.2 Update root package.json scripts

Update lint scripts:

```json
{
	"scripts": {
		"check:lint:all": "run-eslint",
		"fix:lint": "FIX_ESLINT=1 run-eslint"
	}
}
```

### Step 4: Update Tools Package

#### 4.1 Rename and update the existing ESLint script

**For LLMs:** Look for either `packages/tools/bin/run-eslint-default` OR `packages/tools/bin/run-eslint-workers` (older templates used different names). Rename whichever exists to `packages/tools/bin/run-eslint` and replace its contents with this EXACT script (do not modify or create your own version):

```bash
#!/bin/bash
set -euo pipefail

# store cache under node_modules when available to reduce clutter
cache_location=".eslintcache" # default
if [[ -d "node_modules" ]]; then
	cache_location="node_modules/.cache/run-eslint/.eslintcache"
fi

args=(
	--cache
	--cache-strategy content
	--cache-location "$cache_location"
	--max-warnings 1000
	--flag unstable_config_lookup_from_file
	.
)

if [[ -n "${FIX_ESLINT:-}" ]]; then
	args+=("--fix")
fi

if [[ -n "${GITHUB_ACTIONS:-}" ]] || [[ -n "${CI:-}" ]]; then
	args+=("--max-warnings=0")
fi

# get additional args
while [[ $# -gt 0 ]]; do
	args+=("$1")
	shift
done

if command -v bun >/dev/null 2>&1; then
	# bun is much faster, so use it when available
	bun --bun eslint "${args[@]}"
else
	eslint "${args[@]}"
fi
```

**Note:** The file will keep its executable permissions from the rename.

#### 4.2 Update packages/tools/src/cmd/check.ts

Update the lint check to use the new command:

```typescript
const checks = {
	deps: ['pnpm', 'check:deps'],
	// eslint can be run from anywhere and it'll automatically only lint the current dir and children
	lint: ['run-eslint'],
	types: ['turbo', turboFlags, 'check:types'].flat(),
	format: ['pnpm', 'check:format'],
} as const satisfies { [key: string]: string[] }
```

#### 4.3 Create packages/tools/eslint.config.ts

```typescript
import { defineConfig, getConfig } from '@repo/eslint-config'

const config = getConfig(import.meta.url)

export default defineConfig([...config])
```

Delete `packages/tools/.eslintrc.cjs`.

### Step 5: Update Turbo Configuration

#### 5.1 Rename turbo.json to turbo.jsonc (if not already renamed)

#### 5.2 Update specific lint tasks in turbo.jsonc

Make these exact changes to the `tasks` section:

**Update the `check:lint` task** - add the comment and ensure it has the FIX_ESLINT env:

```jsonc
"check:lint": {
	// does not depend on ^check:lint because it's better to run it
	// from the root when needing to lint multiple packages
	"dependsOn": ["build", "topo"],
	"outputLogs": "new-only",
	"env": ["FIX_ESLINT"]
},
```

**Add the new `//#check:lint:all` task** (if not already present):

```jsonc
"//#check:lint:all": {
	"outputLogs": "new-only",
	"outputs": ["node_modules/.cache/run-eslint/.eslintcache"],
	"env": ["FIX_ESLINT"]
},
```

**Update the `check:ci` task** to include `//#check:lint:all` in dependsOn (if not already present):

```jsonc
"check:ci": {
	"dependsOn": [
		"//#check:format",
		"//#check:deps",
		"check:types",
		"//#check:lint:all",
		"//#test:ci",
		"test:ci",
		"topo"
	],
	"outputLogs": "new-only"
},
```

**Important:** Keep both the `check` and `check:ci` tasks. The `check` task is for local development, while `check:ci` is for CI environments and includes additional checks like formatting and dependency validation.

### Step 6: Update All Package Configurations

#### 6.1 For each package/app, replace .eslintrc.cjs with eslint.config.ts

**For packages using default config:**

```typescript
import { defineConfig, getConfig } from '@repo/eslint-config'

const config = getConfig(import.meta.url)

export default defineConfig([...config])
```

**For packages using React config:**

```typescript
import { defineConfig } from '@repo/eslint-config'
import { getReactConfig } from '@repo/eslint-config/react'

const config = getReactConfig(import.meta.url)

export default defineConfig([...config])
```

#### 6.2 Update package.json scripts

In all package.json files, replace either:

```json
"check:lint": "run-eslint-default"
```
OR:
```json
"check:lint": "run-eslint-workers"
```

With:

```json
"check:lint": "run-eslint"
```

### Step 7: Update Turbo Generator Templates

Update template files in `turbo/generators/templates/`:

- `fetch-worker/package.json.hbs`
- `fetch-worker-vite/package.json.hbs`  
- `package/package.json.hbs`

Replace `"check:lint": "run-eslint-default"` or `"check:lint": "run-eslint-workers"` with `"check:lint": "run-eslint"`

### Step 8: Update VS Code Settings

Update `.vscode/settings.json` to include `eslint.config.ts` in file associations:

```json
{
	"explorer.fileNesting.patterns": {
		"package.json": "eslint.config.ts, CLAUDE.md, AGENT.md, ...(other patterns)"
	}
}
```

### Step 9: Update Documentation

#### 9.1 Update CLAUDE.md

Replace references to `run-eslint-default` or `run-eslint-workers` with `run-eslint`.

#### 9.2 Update .cursor/rules/package-management.mdc

Replace `"check:lint": "run-eslint-default"` or `"check:lint": "run-eslint-workers"` with `"check:lint": "run-eslint"`.

### Step 10: Testing the Migration

1. **Install dependencies**: `pnpm install`
2. **Verify ESLint version**: `pnpm eslint --version` (should show 9.x.x)
3. **Test linting**: `run-eslint` or `just check`
4. **Test auto-fix**: `FIX_ESLINT=1 run-eslint` or `just fix`
5. **Run full checks**: `just check`

**Note**: Use `pnpm eslint --version` to check the ESLint version, not `run-eslint --version`. The `run-eslint` command is a wrapper script for use within the monorepo's script system.

## Migration Checklist

- [ ] Updated `packages/eslint-config/package.json` with new exports and dependencies
- [ ] Created `packages/eslint-config/src/` directory structure
- [ ] Created `packages/eslint-config/src/helpers.ts`
- [ ] Created `packages/eslint-config/src/default.config.ts`
- [ ] Created `packages/eslint-config/src/react.config.ts`
- [ ] Created `packages/eslint-config/eslint.config.ts`
- [ ] Created `packages/eslint-config/tsconfig.json`
- [ ] Deleted old `packages/eslint-config/default.cjs`
- [ ] Updated TypeScript configs to exclude `eslint.config.ts`
- [ ] Replaced root `.eslintrc.cjs` with `eslint.config.ts`
- [ ] Updated root package.json scripts
- [ ] Renamed existing ESLint script (`run-eslint-default` or `run-eslint-workers`) to `run-eslint` and updated contents
- [ ] Updated `packages/tools/src/cmd/check.ts`
- [ ] Created `packages/tools/eslint.config.ts`
- [ ] Deleted `packages/tools/.eslintrc.cjs`
- [ ] Renamed `turbo.json` to `turbo.jsonc` and updated lint tasks
- [ ] Replaced all package `.eslintrc.cjs` files with `eslint.config.ts`
- [ ] Updated all package.json scripts from `run-eslint-default` or `run-eslint-workers` to `run-eslint`
- [ ] Updated turbo generator templates
- [ ] Updated VS Code settings
- [ ] Updated documentation files
- [ ] Tested linting works: `run-eslint`
- [ ] Verified auto-fix works: `FIX_ESLINT=1 run-eslint`

## Common Issues and Solutions

#### Issue: "run-eslint: command not found"

**Solution:** Ensure the script was properly renamed and refresh bin links:

```bash
pnpm install  # Refresh bin links
```

#### Issue: TypeScript parser errors

**Solution:** Verify that TypeScript configs exclude `eslint.config.ts` and that `tsconfig.json` exists in the package root.

#### Issue: Plugin resolution errors

**Solution:** Ensure all required ESLint plugins are installed in the `packages/eslint-config` package.

## Notes for LLMs

When automating this migration:

1. **Always backup** existing ESLint configurations before starting
2. **Follow the exact file structure** - this template has specific conventions
3. **Update all script references** from `run-eslint-default` to `run-eslint`
4. **Don't forget generator templates** in `turbo/generators/templates/`
5. **Test thoroughly** after migration by running `just check`
6. **Handle React packages** separately using the React config
7. **Update documentation references** that mention the old command

The migration preserves all existing functionality while providing modern ESLint 9 features optimized for the workers-monorepo-template structure.

# Product Requirements Document: create-wrangler-config

## Overview

`create-wrangler-config` is a CLI tool for quickly setting up a `wrangler.jsonc` configuration file for Cloudflare Workers projects. This tool helps developers bootstrap their Workers configuration with interactive prompts and intelligent defaults.

## Goals

- **Simplify Workers setup**: Reduce friction in creating new Cloudflare Workers projects
- **Interactive configuration**: Guide users through the configuration process with prompts
- **Smart defaults**: Provide sensible defaults based on project structure and common patterns
- **Package manager integration**: Automatically install wrangler as a dependency if needed
- **Safety first**: Prevent overwriting existing configuration files

## Target Users

- Developers new to Cloudflare Workers
- Experienced developers who want to quickly bootstrap new Workers projects
- Teams standardizing on Workers configuration patterns

## Usage

```bash
npm create wrangler-config@latest [assets-directory]
```

### Arguments

- `assets-directory` (optional): Path to directory containing static assets to be served by the Worker

### Examples

```bash
# Basic setup in current directory
npm create wrangler-config@latest

# Setup with assets directory
npm create wrangler-config@latest ./public

# Setup in specific directory
cd my-worker-project && npm create wrangler-config@latest
```

## Core Features

### 1. Configuration File Detection

**Requirement**: Before creating any files, check for existing Wrangler configuration files.

**Behavior**:
- Check for existence of: `wrangler.jsonc`, `wrangler.json`, `wrangler.toml`
- If any exist, exit with warning message: "Wrangler configuration already exists. This tool only creates new configuration files."
- Exit code: 1

### 2. Package Manager Detection & Wrangler Installation

**Requirement**: Detect package manager and ensure wrangler is available as a dependency.

**Detection Logic**:
1. Check for lock files in order of preference:
   - `bun.lockb` or `bun.lock` → Bun
   - `pnpm-lock.yaml` → pnpm
   - `yarn.lock` → Yarn
   - `package-lock.json` → npm
   - Default to npm if no lock file found

**Wrangler Dependency Check**:
1. If `package.json` exists:
   - Parse dependencies and devDependencies
   - If `wrangler` not found, prompt to install as devDependency
   - Use detected package manager for installation
2. If no `package.json`, skip dependency management

### 3. Interactive Configuration Prompts

**Required Prompts**:

1. **Worker Name**
   - Prompt: "What is your Worker name?"
   - Validation: Must be valid identifier (alphanumeric, hyphens, underscores)
   - Default: Directory name (sanitized)

2. **Entry Point**
   - Prompt: "What is your main entry file?"
   - Default: Auto-detect from common patterns:
     - `src/index.ts` (if exists)
     - `src/index.js` (if exists)
     - `index.ts` (if exists)
     - `index.js` (if exists)
     - Default to `src/index.ts`

3. **Compatibility Date**
   - Prompt: "Compatibility date (YYYY-MM-DD)?"
   - Default: Current date
   - Validation: Valid date format, not in future

4. **Assets Directory** (if provided as argument or detected)
   - If assets directory argument provided, use it
   - Otherwise, check for common static directories: `public`, `static`, `assets`, `dist`
   - If found, prompt: "Serve static assets from [directory]?"

**Optional Prompts**:

5. **Account ID**
   - Prompt: "Cloudflare Account ID (optional)?"
   - Help text: "Find this in your Cloudflare dashboard"
   - Can be skipped

6. **Environment Configuration**
   - Prompt: "Set up staging environment?"
   - If yes, prompt for staging-specific settings

### 4. Configuration Generation

**Output**: Generate `wrangler.jsonc` with collected configuration.

**Base Configuration Structure**:
```jsonc
{
  "name": "worker-name",
  "main": "src/index.ts",
  "compatibility_date": "2024-01-15",
  // Optional fields based on prompts
  "account_id": "...",
  "assets": {
    "directory": "./public"
  }
}
```

**Environment Support**:
If staging environment requested:
```jsonc
{
  "name": "worker-name",
  "main": "src/index.ts", 
  "compatibility_date": "2024-01-15",
  "env": {
    "staging": {
      "name": "worker-name-staging"
    }
  }
}
```

### 5. Success Output

**Completion Message**:
```
✅ Created wrangler.jsonc successfully!

Next steps:
1. Implement your Worker in src/index.ts
2. Test locally: npx wrangler dev
3. Deploy: npx wrangler deploy

Documentation: https://developers.cloudflare.com/workers/
```

## Technical Requirements

### Architecture

**Scaffolding Pattern**: Follow `apps/llm-rules` structure:
- `src/bin/create-wrangler-config.ts` - CLI entry point
- `src/cli.ts` - CLI setup and argument parsing
- `src/create-config.ts` - Main configuration logic
- `package.json` with proper bin configuration

**Dependencies**:
- `zx` for shell commands (package manager detection/installation)
- `zod` v4 for data validation/parsing
- `@inquirer/prompts` for interactive prompts (following `create-workers-monorepo` pattern)
- `@commander-js/extra-typings` for CLI argument parsing

**Key Modules**:

1. **Package Manager Detection** (`src/package-manager.ts`)
   - Detect package manager from lock files
   - Check for wrangler dependency
   - Install wrangler if needed

2. **Configuration Builder** (`src/config-builder.ts`)
   - Zod schemas for configuration validation
   - Generate wrangler.jsonc content
   - Handle environment configurations

3. **File System Utils** (`src/fs-utils.ts`)
   - Check for existing config files
   - Detect common file patterns
   - Write configuration file

4. **Prompts** (`src/prompts.ts`)
   - Interactive configuration prompts
   - Input validation
   - Default value logic

### Error Handling

- Graceful handling of file system errors
- Clear error messages for validation failures
- Proper exit codes for different error conditions
- Rollback on partial failures

### Testing Strategy

- Unit tests for configuration generation
- Integration tests for CLI flow
- Mock file system operations
- Test package manager detection logic

## Future Enhancements

### Phase 2 Features

1. **Advanced Configuration Options**
   - KV namespace setup
   - D1 database configuration
   - R2 bucket configuration
   - Custom domains/routes

2. **Template Support**
   - Pre-built configuration templates
   - Framework-specific configurations (Hono, Itty Router, etc.)

3. **Migration Support**
   - Convert from wrangler.toml to wrangler.jsonc
   - Upgrade existing configurations

4. **Integration Features**
   - Git repository initialization
   - TypeScript configuration setup
   - ESLint/Prettier configuration

### Success Metrics

- Adoption rate among new Workers projects
- Reduction in configuration-related support requests
- User feedback on setup experience
- Time to first successful deployment

## Dependencies

- Node.js 18+
- npm/yarn/pnpm/bun for package management
- Cloudflare account (for deployment, not required for config generation)

## Risks & Mitigations

**Risk**: Overwriting existing configurations
**Mitigation**: Strict pre-flight checks for existing files

**Risk**: Package manager detection failures
**Mitigation**: Fallback to npm, clear error messages

**Risk**: Invalid configuration generation
**Mitigation**: Comprehensive Zod validation, testing

**Risk**: Breaking changes in Wrangler
**Mitigation**: Pin to stable Wrangler versions, regular updates

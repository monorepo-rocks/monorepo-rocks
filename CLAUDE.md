# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a monorepo template for building and managing multiple Cloudflare Workers applications. It uses pnpm workspaces, Turborepo for orchestration, and provides CLI tools for scaffolding new Workers.

## Essential Commands

```bash
# Installation and setup
just install              # Install all dependencies

# Development
just dev                  # Start dev servers for all workers
just new-worker          # Generate new worker from template (interactive)

# Building and testing
just build               # Build all workers
just test                # Run all tests across the monorepo
just check               # Run linting, type checking, and formatting checks
just fix                 # Auto-fix linting and formatting issues

# Deployment
just deploy              # Deploy all workers to Cloudflare

# Dependency management
just update-deps         # Update and sync dependencies using syncpack
just cs                  # Create a changeset for versioning
```

## Architecture

### Directory Structure

- `/apps/` - Individual Cloudflare Worker applications
- `/packages/` - Shared code and configurations:
  - `hono-helpers` - Shared Hono middleware and utilities
  - `tools` - Build scripts and CLI binaries
  - `eslint-config` - Shared ESLint configuration
  - `typescript-config` - Shared TypeScript configurations
  - `workspace-dependencies` - Common dependencies (zod, zx, yaml)

### Key Patterns

1. **Shared utilities**: Workers import from `@repo/*` packages
2. **Centralized scripts**: All scripts reference binaries in `packages/tools/bin/`
3. **Hono framework**: All workers use Hono with shared middleware
4. **Testing**: Vitest with Cloudflare Workers pool configuration

### Creating New Workers

Use `just new-worker` to scaffold a new worker. The generator will:

- Create the worker in `/apps/` directory
- Set up Hono with standard middleware
- Configure Wrangler for local development and deployment
- Add testing setup with Vitest

### Testing

- Run all tests: `just test`
- Run specific worker tests: `cd apps/worker-name && pnpm test`
- Tests use Cloudflare Workers runtime via `@cloudflare/vitest-pool-workers`

### Deployment

Workers are deployed using Wrangler. Ensure you have:

- `CLOUDFLARE_ACCOUNT_ID` environment variable set
- `CLOUDFLARE_API_TOKEN` with appropriate permissions
- Worker names configured in each `wrangler.jsonc`

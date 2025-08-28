# AGENTS.md

This is a Cloudflare Workers monorepo using pnpm workspaces, Turborepo, and Hono.

## Commands

```bash
# Main commands (via Just or pnpm)
just test                    # Run all tests
just test <worker-name>      # Run specific worker tests
pnpm --filter <worker-name> test  # Alternative single worker test
just build                   # Build all
just check                   # Lint, types, format check
just fix                     # Auto-fix lint/format
just new-worker              # Generate new worker

# For individual workers:
cd apps/<worker-name> && pnpm test
```

## Code Style

- **Imports**: Workspace packages via `@repo/*`, sorted by prettier with type imports separated
- **Formatting**: Prettier with tabs, no semicolons, single quotes, 100 char width
- **Types**: TypeScript strict mode, prefer type imports, no explicit any allowed
- **Zod**: Inferred types above schemas with same name: `export type Thing = z.infer<typeof Thing>`
- **Naming**: Use underscore prefix for unused vars (`_unused`)
- **Framework**: All workers use Hono with shared middleware from `@repo/hono-helpers`

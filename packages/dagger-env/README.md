# dagger-env

A type-safe, reusable environment configuration abstraction for Dagger modules with full Zod v4 validation.

## Features

- 🔒 **Type-safe**: Full TypeScript support with Zod v4 validation
- 🔄 **Reusable**: Create multiple environment configurations for different projects
- 🎯 **Consistent**: Standardized API across all Dagger modules
- 🛡️ **Validated**: Runtime validation of arguments, environment variables, and secrets
- 📦 **Modular**: Secret presets and derived environment variables
- 🚀 **Easy to use**: Simple configuration-based setup

## Installation

```bash
npm install dagger-env zod
```

## Quick Start

```typescript
import { z } from 'zod/v4'
import { createDaggerEnv } from 'dagger-env'

// Define your environment configuration
const myDaggerEnv = createDaggerEnv({
  args: z.object({
    push: z.string().optional(),
    environment: z.enum(['dev', 'staging', 'prod']).optional(),
  }),
  env: z.object({
    CI: z.string().optional(),
    NODE_ENV: z.string().optional(),
  }),
  secrets: z.object({
    API_TOKEN: z.string(),
    DATABASE_URL: z.string(),
    REDIS_URL: z.string(),
  }),
  secretPresets: {
    api: ['API_TOKEN', 'DATABASE_URL'],
    cache: ['REDIS_URL'],
  } as const,
  derivedEnvVars: {
    API_TOKEN: {
      API_BASE_URL: 'https://api.example.com',
      API_VERSION: 'v1',
    },
    DATABASE_URL: {
      DB_POOL_SIZE: '10',
    },
  } as const,
})

// Use in your Dagger module
import { dag, Container, object, func, Secret } from '@dagger.io/dagger'

@object()
export class MyModule {
  @func()
  async build(options: Secret): Promise<Container> {
    const opts = await myDaggerEnv.parseDaggerOptions(options)
    const withEnv = await myDaggerEnv.getWithEnv(options, ['api'], ['REDIS_URL'])

    return withEnv(dag.container().from('node:18')).withExec(['npm', 'run', 'build']).sync()
  }
}
```

## API Reference

### Environment Configuration

#### `createDaggerEnv(config)`

Creates a new DaggerEnv instance with the provided configuration.

**Parameters:**

- `config.args`: Zod schema for command-line arguments
- `config.env`: Zod schema for environment variables
- `config.secrets`: Zod schema for secrets
- `config.secretPresets`: Object mapping preset names to arrays of secret names
- `config.derivedEnvVars`: Object mapping secret names to derived environment variables

**Returns:** `DaggerEnv<T>` instance

### `daggerEnv.parseDaggerOptions(options: Secret)`

Parses and validates dagger options from a Secret containing JSON.

**Parameters:**

- `options`: Dagger Secret containing JSON options

**Returns:** `Promise<DaggerOptionsFromConfig<T>>` - Parsed and typed options object

### `daggerEnv.getWithEnv(options, secretPresets, secretNames?)`

Creates a function that applies environment variables and secrets to a container.

**Parameters:**

- `options`: Secret or parsed options object
- `secretPresets`: Array of preset names to include (e.g., `['api', 'cache']`)
- `secretNames`: Optional array of additional individual secret names

**Returns:** `Promise<(con: Container) => Container>` - Function that applies env vars and secrets

### `daggerEnv.getOptionsSchema()`

Returns the Zod schema for the complete options object.

**Returns:** `ZodObject` - The combined schema for args, env, and secrets

### `daggerEnv.getSecretPresets()`

Returns array of available secret preset names.

**Returns:** `Array<string>` - Available preset names

### `daggerEnv.getPresetSecrets(preset)`

Returns array of secret names for a specific preset.

**Parameters:**

- `preset`: Name of the preset

**Returns:** `readonly string[]` - Secret names in the preset

## Configuration Examples

### Simple API Service

```typescript
const apiServiceEnv = createDaggerEnv({
  args: z.object({
    push: z.string().optional(),
  }),
  env: z.object({
    CI: z.string().optional(),
  }),
  secrets: z.object({
    API_TOKEN: z.string(),
    DATABASE_URL: z.string(),
  }),
  secretPresets: {
    api: ['API_TOKEN', 'DATABASE_URL'],
  } as const,
  derivedEnvVars: {
    API_TOKEN: {
      API_BASE_URL: 'https://api.example.com',
    },
  } as const,
})
```

### Multi-Environment Setup

```typescript
const multiEnvDaggerEnv = createDaggerEnv({
  args: z.object({
    environment: z.enum(['dev', 'staging', 'prod']),
    push: z.string().optional(),
  }),
  env: z.object({
    CI: z.string().optional(),
  }),
  secrets: z.object({
    DEV_API_KEY: z.string(),
    STAGING_API_KEY: z.string(),
    PROD_API_KEY: z.string(),
  }),
  secretPresets: {
    dev: ['DEV_API_KEY'],
    staging: ['STAGING_API_KEY'],
    prod: ['PROD_API_KEY'],
  } as const,
  derivedEnvVars: {
    DEV_API_KEY: { API_URL: 'https://dev-api.example.com' },
    STAGING_API_KEY: { API_URL: 'https://staging-api.example.com' },
    PROD_API_KEY: { API_URL: 'https://api.example.com' },
  } as const,
})
```

## Type Extraction

Extract TypeScript types from your configuration:

```typescript
import { z } from 'zod'

// Extract output type (after parsing/validation)
type MyOptions = z.output<ReturnType<typeof myDaggerEnv.getOptionsSchema>>

// Extract input type (before parsing/validation)
type MyInput = z.input<ReturnType<typeof myDaggerEnv.getOptionsSchema>>
```

## Best Practices

1. **Use `as const`** for `secretPresets` and `derivedEnvVars` to ensure proper typing
2. **Group related secrets** into logical presets (e.g., `api`, `database`, `cache`)
3. **Validate early** by calling `parseDaggerOptions()` at the start of functions
4. **Reuse configurations** across multiple Dagger modules in the same project
5. **Document your schemas** with JSDoc comments for better developer experience

## Requirements

- Node.js 18+
- Dagger SDK
- Zod v4+

## License

MIT

## Contributing

Contributions are welcome! Please read our contributing guidelines and submit pull requests to our GitHub repository.

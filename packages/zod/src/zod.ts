// Re-exported with additional functionality based on:
// https://x.com/colinhacks/status/1852428728103776631
/* eslint-disable import/export */

// Export everything from zod/v4
export * from 'zod/v4'

// Export our custom type-safe parse functions (these will override the ones from zod/v4)
export { parse, safeParse, safeParseAsync } from './lib/parse'

# dagger-env

## 0.4.2

### Patch Changes

- ccae119: fix: use range for Zod peer dependency

## 0.4.1

### Patch Changes

- 70ac37f: chore: change order of opSection params

## 0.4.0

### Minor Changes

- 6db8e9b: feat: add 1password integration and command runner to dagger-env

  Extends dagger-env package with new features:

  - Adds command runner with 1Password secret integration
  - Provides new `/run` export for executing Dagger commands
  - Updates README with comprehensive documentation for new functionality
  - Introduces type-safe command execution with environment validation

  Enables more robust secret management and simplified Dagger command execution across different environments

### Patch Changes

- 4943347: chore: update deps (zod)
- 434c569: fix: improve secret type checking in dagger environment

  Enhances type validation for secret objects by adding function type checks for 'id' and 'plaintext' methods

  Ensures more robust type checking by verifying that secret objects not only have the required properties but also that those properties are functions

## 0.3.0

### Minor Changes

- 352d201: feat: add 1Password schema to dagger-env

## 0.2.1

### Patch Changes

- f3c6014: fix: actually export dagger-env

## 0.2.0

### Minor Changes

- cd492ec: feat: add dagger-env package

### Patch Changes

- cd492ec: chore: update deps (zod)

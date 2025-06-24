---
'dagger-env': patch
---

fix: improve secret type checking in dagger environment

Enhances type validation for secret objects by adding function type checks for 'id' and 'plaintext' methods

Ensures more robust type checking by verifying that secret objects not only have the required properties but also that those properties are functions

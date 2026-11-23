---
name: spec
description: Change the API contract in openapi/openapi.yaml, regenerate code and open a contract-only pull request. Use before implementing any endpoint change.
---

# Change the contract

1. Read the issue and the existing endpoints in `openapi/openapi.yaml`. Follow their conventions: `/api/v1` paths, `operationId` in camelCase, shared schemas in `components`, RFC 9457 `Problem` for errors, `page`/`size` pagination, cookie session security.
2. Edit the spec. Describe every field, enum value and error response.
3. `make gen`, then `make check` (spec lint, generated code in sync, Go and TypeScript still compile; stub new handlers to return 501 if needed).
4. Check the breaking-change report. Breaking changes need a reason in the pull request.
5. Open a pull request that changes only the contract and generated code, linked to the issue, so it can be reviewed before implementation.

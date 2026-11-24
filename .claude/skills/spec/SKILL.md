---
name: spec
description: Change the API contract in openapi/openapi.yaml and regenerate code, as the first step of a backend slice. The contract ships in the same pull request as its implementation.
---

# Change the contract

1. Read the issue and the existing endpoints in `openapi/openapi.yaml`. Follow their conventions: `/api/v1` paths, `operationId` in camelCase, shared schemas in `components`, RFC 9457 `Problem` for errors, `page`/`size` pagination, cookie session security.
2. Edit the spec. Describe every field, enum value and error response.
3. `make gen`, then `make check-spec` (lint, generated code in sync). New handlers must exist for the code to compile; they are implemented next in the same branch.
4. Check the breaking-change report. Breaking changes need a reason in the pull request.
5. Continue with the backend implementation on the same branch (ADR 0005): the pull request contains the contract, the generated code, the handlers and the tests that prove them. If implementing shows the contract is wrong, fix the contract here rather than working around it in code.

# 0005. Spec-first OpenAPI contract with generated server and client

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
The web app, end-to-end tests and future clients all consume the API. Backend and frontend work is split between coding agents running in parallel, so both sides need a fixed contract before implementation starts.

## Decision
`openapi/openapi.yaml` is the source of truth.
- **Go:** `oapi-codegen` generates a strict server interface and types; the controller implements it. Code that does not match the contract does not compile.
- **TypeScript:** `openapi-typescript` generates types and `openapi-fetch` makes typed requests.
- **Version:** OpenAPI 3.1 if the M0 spike shows `oapi-codegen` handles our patterns (nullable fields, problem errors, enums, cookie auth). Otherwise 3.0.3.
- **Errors:** RFC 9457 problem details with an `errors` map per field.
- **CI:** the spec is linted (Redocly), generated code must be in sync, breaking changes are reported on every pull request, and integration tests validate real responses against the spec.

A feature slice starts with a small pull request that changes only the contract.

### Spike result (2026-11-23, issue #5)
**OpenAPI 3.1 is used.** With `oapi-codegen` v2.8.2 and `kin-openapi` v0.153.0:
- `type: [string, "null"]` generates `nullable.Nullable[string]`; the validator accepts a string or `null` and rejects other types.
- Enums, path patterns, query bounds, cookie security, `default` problem responses and the embedded spec all work with the strict `net/http` server.
- Requests are validated against the spec before handlers run (`nethttp-middleware`); failures become 400 problems naming the field.
- `oapi-codegen` ignores `-o` when the config sets `output`, so the output path is passed as a flag.
- `openapi-typescript` 7.13 + `openapi-fetch` 0.17 type paths, parameters, responses and problem errors in the web app.

## Alternatives considered
- **Code-first with Huma:** one source of truth in Go and less duplication, but the frontend waits for the backend and contract changes are reviewed through code.
- **Code-first with Swaggo comments:** the spec drifts from the code because comments are not checked.

## Consequences
- Contract changes are reviewed as readable YAML diffs before any code exists.
- Generators sometimes lag behind the OpenAPI specification; the spike checks this early.
- Some duplication between the spec and validation rules in use cases.

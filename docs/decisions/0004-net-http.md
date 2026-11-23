# 0004. Standard library net/http instead of a web framework

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
Go web frameworks (Gin, Echo, Fiber) exist mainly because the old standard router could not match methods or path parameters. Since Go 1.22 `http.ServeMux` does both. Routing and request binding are generated from the OpenAPI contract ([0005](0005-spec-first-openapi.md)).

## Decision
Use `net/http` with `http.ServeMux` and the `oapi-codegen` strict server. Middleware is plain `func(http.Handler) http.Handler`.

## Alternatives considered
- **Fiber:** built on fasthttp, not `net/http`. Standard middleware (`otelhttp`, `http.CrossOriginProtection`, `httptest`) needs adapters, and request values are only valid during the handler because fasthttp reuses buffers, which risks data from one request appearing in another. Its speed advantage is invisible next to database time.
- **Gin or Echo:** compatible with `net/http` and mature, but with generated routing and binding they add a dependency and a custom context type for little gain.
- **Chi:** a small `net/http` router; kept as an option if route groups become hard to express.

## Consequences
- The whole `net/http` ecosystem works directly, including `http.CrossOriginProtection` ([0015](0015-csrf-origin-checks.md)).
- Covered by the Go 1 compatibility promise; no framework upgrades.
- Some helpers (JSON responses, problem errors) are written in-house in `pkg/` and the controller layer.

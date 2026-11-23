# 0001. Rewrite employee-portal as a Go API and a Next.js web app

- **Status:** Accepted
- **Date:** 2026-11-23

## Context
[employee-portal](https://github.com/agusheryanto182/employee-portal) was an internship assignment: an employee dashboard and a backoffice, built with Spring Boot, a Vite + React SPA and PostgreSQL. It is finished and frozen as that deliverable.

The project continues as the foundation of a SaaS product that is not decided yet. It may serve businesses, consumers or both, so it needs room for public pages and many organizations. It is also a portfolio piece and a testbed for an agentic coding workflow.

## Decision
Start a new repository, `go-next-saas`, and rebuild:
- **API in Go**: small memory footprint, fast start-up, a single static binary.
- **Web app in Next.js** (App Router): server rendering is available when public, indexable pages are needed, while the signed-in app keeps working as an interactive client.
- **PostgreSQL** stays.

The old repository is the reference for behaviour, business rules, UI texts and end-to-end tests. It is not a parity target: sessions and multi-tenancy change the API.

## Alternatives considered
- **Keep Spring Boot and the Vite SPA:** least work, but no server rendering and the JVM costs more memory on a small server.
- **Rewrite only the API, keep the SPA:** keeps a proven frontend, but public pages would need a second app later.
- **Evolve the old repository in place:** would change a frozen deliverable.

## Consequences
- Two rewrites. The API is built slice by slice together with the web pages, with end-to-end tests as the check for each slice.
- The old Spring version remains available for a performance comparison (memory, image size, start-up, latency).

# One entry point for people, agents and CI. Targets for areas that do not exist yet say so and succeed,
# so the same commands work from the first commit; each skeleton issue fills its part in.
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# Areas checked by check-fast; the stop hook passes only the areas that changed.
AREAS ?= api web spec

GO_TOOL := go tool -modfile=tools/go.mod

# Code generated from openapi/openapi.yaml (ADR 0005). Never edit these by hand.
OAPI_DIR := api/internal/controller/restapi/v1/gen
OAPI_GEN := cd $(OAPI_DIR) && go tool -modfile=../../../../../tools/go.mod oapi-codegen -config oapi-codegen.yaml
SPEC_FROM_OAPI := ../../../../../../openapi/openapi.yaml
TS_SCHEMA := web/src/lib/api/schema.d.ts
# Scratch output for the sync check; overwritten on every run, gitignored.
GEN_CHECK := $(CURDIR)/.cache/gen-check
# Migration linter (ADR 0009), run through npx at a pinned version.
SQUAWK := npx --yes squawk-cli@2.72.0
# Git ref whose contract check-breaking compares against.
BASE ?= origin/main

.PHONY: help dev down reset-db migrate gen fmt check check-fast check-api check-web check-spec check-breaking vuln test-integration e2e seed

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-18s %s\n", $$1, $$2}'

dev: ## Start PostgreSQL and Mailpit
	docker compose up -d --wait postgres mailpit
	@echo "PostgreSQL on localhost:5432, Mailpit inbox on http://localhost:8025"

down: ## Stop the local stack (data is kept)
	docker compose --profile app down

reset-db: ## Delete local database data and start again (re-runs role setup)
	docker compose down -v
	$(MAKE) dev

migrate: ## Apply pending migrations as the owner role (needs MIGRATION_DATABASE_URL, e.g. from .env)
	cd api && go run ./cmd/app -migrate

gen: ## Regenerate code from the OpenAPI contract and the SQL queries
	$(OAPI_GEN) -o api.gen.go $(SPEC_FROM_OAPI)
	cd web && npm run --silent gen:api
	cd api && $(GO_TOOL) sqlc generate

fmt: ## Format Go code
	cd api && $(GO_TOOL) golangci-lint fmt ./...

check: ## Everything CI runs except end-to-end tests
	$(MAKE) check-fast AREAS="api web spec"
	$(MAKE) vuln
	cd web && npm run --silent build

check-fast: ## Format, lint and unit tests for AREAS (default: all)
	@for area in $(AREAS); do $(MAKE) --no-print-directory check-$$area; done

check-api: ## Lint (incl. format and layer rules), SQL code in sync, migration lint, unit tests
	cd api && $(GO_TOOL) golangci-lint run ./...
	cd api && $(GO_TOOL) sqlc diff || { echo "Generated SQL code is out of date with api/queries or api/migrations: run 'make gen'."; exit 1; }
	cd api && $(SQUAWK) -c .squawk.toml migrations/*.sql
	cd api && go test -race ./...

check-web: ## Lint, typecheck, unit tests and format check for the web app
	cd web && npm run --silent lint && npm run --silent typecheck && npm test --silent && npm run --silent format:check

check-spec: ## Lint the OpenAPI contract and verify generated code matches it
	cd openapi && npm run --silent lint
	@mkdir -p $(GEN_CHECK)
	@$(OAPI_GEN) -o $(GEN_CHECK)/api.gen.go $(SPEC_FROM_OAPI)
	@cd web && npx --no-install openapi-typescript ../openapi/openapi.yaml -o $(GEN_CHECK)/schema.d.ts >/dev/null 2>&1
	@if ! cmp -s $(GEN_CHECK)/api.gen.go $(OAPI_DIR)/api.gen.go || ! cmp -s $(GEN_CHECK)/schema.d.ts $(TS_SCHEMA); then \
	  echo "Generated code is out of date with openapi/openapi.yaml: run 'make gen'."; exit 1; \
	fi
	@echo "Generated code matches the contract."

check-breaking: ## Fail on contract changes that break existing clients, compared to BASE (default origin/main)
	@mkdir -p .cache
	@git show $(BASE):openapi/openapi.yaml > .cache/openapi-base.yaml
	cd api && $(GO_TOOL) oasdiff breaking ../.cache/openapi-base.yaml ../openapi/openapi.yaml --fail-on ERR

vuln: ## Known vulnerabilities in the API code and its dependencies
	cd api && $(GO_TOOL) govulncheck ./...

test-integration: ## API tests against a real PostgreSQL (Docker; testcontainers)
	cd api && go test -tags integration -race ./...

e2e: ## Build and start the full stack, then run Playwright (once: cd e2e && npm ci)
	docker compose --profile app up -d --build --wait
	web=$$(docker compose port web 3000); api=$$(docker compose port api 8080); \
	  cd e2e && E2E_BASE_URL=http://$$web E2E_API_URL=http://$$api npx --no-install playwright test

seed: ## Sample organizations and users for development
	@$(call todo,seed,M1)

# $(call todo,what,issue): explain a not-yet-implemented target without failing.
define todo
echo "make: $(1) not set up yet (see $(2))"
endef

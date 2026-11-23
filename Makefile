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

.PHONY: help dev down reset-db gen fmt check check-fast check-api check-web check-spec test-integration e2e seed

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

gen: ## Regenerate code from the OpenAPI contract (SQL queries follow in #6)
	$(OAPI_GEN) -o api.gen.go $(SPEC_FROM_OAPI)
	cd web && npm run --silent gen:api

fmt: ## Format Go code
	cd api && $(GO_TOOL) golangci-lint fmt ./...

check: ## Everything CI runs except end-to-end tests
	$(MAKE) check-fast AREAS="api web spec"
	cd api && $(GO_TOOL) govulncheck ./...
	cd web && npm run --silent build
	@$(call todo,migration lint,#6)

check-fast: ## Format, lint and unit tests for AREAS (default: all)
	@for area in $(AREAS); do $(MAKE) --no-print-directory check-$$area; done

check-api: ## Lint (incl. format and layer rules) and unit tests for the API
	cd api && $(GO_TOOL) golangci-lint run ./...
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

test-integration: ## API tests against a real PostgreSQL
	@$(call todo,integration tests,#6)

e2e: ## Full stack + Playwright
	@$(call todo,end-to-end tests,#8)

seed: ## Sample organizations and users for development
	@$(call todo,seed,M1)

# $(call todo,what,issue): explain a not-yet-implemented target without failing.
define todo
echo "make: $(1) not set up yet (see $(2))"
endef

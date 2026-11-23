# One entry point for people, agents and CI. Targets for areas that do not exist yet say so and succeed,
# so the same commands work from the first commit; each skeleton issue fills its part in.
SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

# Areas checked by check-fast; the stop hook passes only the areas that changed.
AREAS ?= api web spec

GO_TOOL := go tool -modfile=tools/go.mod

.PHONY: help dev down reset-db gen fmt check check-fast test-integration e2e seed

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

gen: ## Regenerate code from the OpenAPI contract and SQL queries
	@$(call todo,gen,#5 and #6)

fmt: ## Format Go code
	cd api && $(GO_TOOL) golangci-lint fmt ./...

check: ## Everything CI runs except end-to-end tests
	$(MAKE) check-fast AREAS="api web spec"
	cd api && $(GO_TOOL) govulncheck ./...
	@$(call todo,generated-code sync and migration lint,#5 #6)

check-fast: ## Format, lint and unit tests for AREAS (default: all)
	@for area in $(AREAS); do $(MAKE) --no-print-directory check-$$area; done

check-api: ## Lint (incl. format and layer rules) and unit tests for the API
	cd api && $(GO_TOOL) golangci-lint run ./...
	cd api && go test -race ./...

check-web:
	@$(call todo,web checks,#7)

check-spec:
	@$(call todo,spec checks,#5)

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

.PHONY: build build-server run-server restart kill proto test test-arango test-all vet lint clean reset-db serve-templates

export PATH := /usr/local/go/bin:$(PATH)

# ── Build ─────────────────────────────────────────────────────────────────────

## Verify the module compiles cleanly.
build:
	go build ./...

## Build the service binary to bin/codevaldagency.
build-server:
	go build -o bin/codevaldagency ./cmd

## Build and run the service.
## ArangoDB and Cross vars can be placed in a .env file (loaded automatically).
run-server: build-server
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a; \
	fi; \
	./bin/codevaldagency

## Stop any running instance, rebuild, and run.
restart: kill build-server
	@echo "Running codevaldagency..."
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a; \
	fi; \
	./bin/codevaldagency

## Stop any running instances of codevaldagency.
kill:
	@echo "Stopping any running instances..."
	-@pkill -9 -f "bin/codevaldagency" 2>/dev/null || true
	-@fuser -k $${CODEVALDAGENCY_GRPC_PORT:-50053}/tcp 2>/dev/null || true
	@sleep 1

# ── Proto Codegen ─────────────────────────────────────────────────────────────

## Regenerate Go stubs from proto/codevaldagency/v1/*.proto.
## Requires: buf, protoc-gen-go, protoc-gen-go-grpc on PATH.
## Install: go install github.com/bufbuild/buf/cmd/buf@latest
##          go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
##          go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
proto:
	buf generate

# ── Tests ─────────────────────────────────────────────────────────────────────

## Run all unit tests (integration tests skip if ArangoDB is unreachable).
test:
	go test -v -race -count=1 ./...

## Run ArangoDB integration tests.
## Loads .env if it exists, otherwise falls back to environment variables.
## Usage: make test-arango
##        AGENCY_ARANGO_ENDPOINT=http://host:8529 AGENCY_ARANGO_USER=root AGENCY_ARANGO_PASSWORD=pw make test-arango
test-arango:
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a; \
	fi; \
	go test -v -race -count=1 ./internal/server/ ./storage/arangodb/

## Run everything: unit tests + ArangoDB integration tests (loads .env).
test-all:
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a; \
	fi; \
	go test -v -race -count=1 ./...

# ── Quality ───────────────────────────────────────────────────────────────────

vet:
	go vet ./...

lint:
	golangci-lint run ./...

# ── Clean ─────────────────────────────────────────────────────────────────────

clean:
	go clean ./...
	rm -rf bin/
	rm -f coverage.out coverage.html

# ── Database ──────────────────────────────────────────────────────────────────

## Truncate all agency ArangoDB collections (dev reset).
## Loads .env if present; honours AGENCY_ARANGO_* env vars.
## Usage: make reset-db
##        AGENCY_ARANGO_DATABASE=mydb make reset-db
reset-db:
	@if [ -f .env ]; then \
		set -a && . ./.env && set +a; \
	fi; \
	ENDPOINT=$${AGENCY_ARANGO_ENDPOINT:-http://localhost:8529}; \
	USER=$${AGENCY_ARANGO_USER:-root}; \
	PASS=$${AGENCY_ARANGO_PASSWORD:-}; \
	DB=$${AGENCY_ARANGO_DATABASE:-codevaldagency}; \
	echo "Resetting agency collections in '$$DB' at $$ENDPOINT ..."; \
	for col in \
		agency_details \
		agency_goals \
		agency_workflows \
		agency_work_items \
		agency_instructions \
		agency_deliverables \
		agency_deliverable_results \
		agency_content_refs \
		agency_configured_roles \
		agency_drafts \
		agency_draft_entities \
		agency_snapshots \
		agency_publications \
		agency_publication_statuses \
		agency_work_plans \
		agency_git_context_sources \
		agency_comm_context_sources \
		agency_work_context_sources; do \
		STATUS=$$(curl -s -o /dev/null -w "%{http_code}" \
			-u "$$USER:$$PASS" \
			-X PUT "$$ENDPOINT/_db/$$DB/_api/collection/$$col/truncate"); \
		case "$$STATUS" in \
			200) echo "  [ok]   $$col" ;; \
			404) echo "  [skip] $$col (collection not found)" ;; \
			*)   echo "  [FAIL] $$col (HTTP $$STATUS)" ;; \
		esac; \
	done; \
	echo "Done."

# ── Templates ─────────────────────────────────────────────────────────────────

TEMPLATES_DIR := /workspaces/CodeVald-AIProject/dehallu/documentation/2-SoftwareDesignAndArchitecture/templates

## Serve the dehallu UI templates folder over HTTP for local preview.
## Usage: make serve-templates            # serves on http://localhost:8080
##        make serve-templates PORT=9000  # override the port
serve-templates:
	@PORT=$${PORT:-8080}; \
	echo "Serving $(TEMPLATES_DIR) at http://localhost:$$PORT ..."; \
	python3 -m http.server $$PORT --directory "$(TEMPLATES_DIR)"

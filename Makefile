SHELL := /bin/bash
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || echo $$(go env GOPATH)/bin/golangci-lint)
GOMARKDOC ?= $(shell command -v gomarkdoc 2>/dev/null || echo $$(go env GOPATH)/bin/gomarkdoc)
MODULE := github.com/agentruntimecontrolprotocol/go-sdk

.PHONY: all fmt fmt-check vet lint test cover build doc-check docs-api tidy clean install gates conformance diagrams examples-smoke

all: gates

fmt:
	gofmt -w .

fmt-check:
	@out=$$(gofmt -l .); \
	if [ -n "$$out" ]; then \
		echo "gofmt needs to be run on:"; \
		echo "$$out"; \
		exit 1; \
	fi

vet:
	go vet ./...

lint:
	$(GOLANGCI_LINT) run

test:
	go test -race -count=1 ./...

cover:
	go test -race -coverpkg=./... -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

build:
	go build ./...
	go build -o /dev/null ./cmd/arcp

doc-check:
	@missing=$$(go doc -all ./... 2>&1 | grep -E "^(func|type|var|const) [A-Z]" | grep -v " //" || true); \
	if [ -n "$$missing" ]; then \
		echo "Public symbols missing godoc comments may exist; run 'go doc -all' to inspect."; \
	fi

# Generate Markdown API docs (one file per package) under docs/api/.
# Skips internal/, examples/, recipes/, and tests/ packages.
# Requires gomarkdoc: go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest
docs-api:
	@command -v "$(GOMARKDOC)" >/dev/null 2>&1 || { \
		echo "gomarkdoc not found at $(GOMARKDOC)"; \
		echo "Install: go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@latest"; \
		exit 1; \
	}
	@rm -rf docs/api && mkdir -p docs/api
	@set -e; \
	while read -r pkg; do \
		rel="$${pkg#$(MODULE)}"; rel="$${rel#/}"; \
		if [ -z "$$rel" ]; then name="arcp"; else name="$$(echo "$$rel" | tr '/' '-')"; fi; \
		echo "==> $$pkg -> docs/api/$$name.md"; \
		"$(GOMARKDOC)" --output "docs/api/$$name.md" "$$pkg"; \
	done < <(go list ./... | grep -vE '/(internal|examples|recipes|tests)(/|$$)')
	@echo "Generated $$(ls docs/api/*.md | wc -l | tr -d ' ') API doc files in docs/api/."

tidy:
	go mod tidy

install:
	go install ./cmd/arcp

clean:
	rm -f coverage.out

conformance:
	ARCP_CONFORMANCE_OUT=$$PWD/conformance.json go test ./tests/conformance/...

diagrams:
	@bash docs/diagrams/render.sh 2>/dev/null || echo "(no diagrams to render)"

examples-smoke:
	@go build ./examples/... && echo "examples build OK"

gates: fmt-check vet test cover build conformance
	@echo "All gates passed."

SHELL := /bin/bash
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || echo $$(go env GOPATH)/bin/golangci-lint)

.PHONY: all fmt fmt-check vet lint test cover build doc-check tidy clean install gates

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

tidy:
	go mod tidy

install:
	go install ./cmd/arcp

clean:
	rm -f coverage.out

gates: fmt-check vet lint test cover build doc-check
	@echo "All gates passed."

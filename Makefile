# ScottyLabs Agent Platform — monorepo task runner.
#
# This is a MULTI-MODULE Go monorepo: each MCP server and service is its own Go module so it can
# deploy independently (design Section 7.6). Targets discover every go.mod automatically, so adding
# a module needs no changes here. `make ci` mirrors the GitHub Actions pipeline exactly.

SHELL := /bin/bash
GO ?= go
GOLANGCI_LINT ?= golangci-lint

# Every directory containing a go.mod (i.e. every Go module).
MODULES := $(shell find . -name go.mod -not -path './.git/*' -exec dirname {} \; | sort)

.PHONY: help ci fmt fmt-check vet lint test test-race build validate tidy deploy

help:
	@echo "ScottyLabs Agent Platform — make targets:"
	@echo "  make ci         fmt-check + vet + lint + test + validate (mirrors CI)"
	@echo "  make fmt        gofmt -w every module"
	@echo "  make fmt-check  fail if any file needs gofmt"
	@echo "  make vet        go vet every module"
	@echo "  make lint       golangci-lint every module (skips with a notice if not installed)"
	@echo "  make test       go test every module"
	@echo "  make test-race  go test -race every module"
	@echo "  make build      go build every module"
	@echo "  make validate   validate manifests + recipes against their JSON Schemas"
	@echo "  make tidy       go mod tidy every module"
	@echo "  make deploy     deploy to Railway (human-gated — see infra/README.md)"
	@echo ""
	@echo "Modules:"
	@for m in $(MODULES); do echo "  $$m"; done

ci: fmt-check vet lint test validate

fmt:
	@for m in $(MODULES); do echo "==> gofmt -w $$m"; (cd $$m && gofmt -w .); done

fmt-check:
	@bad=""; \
	for m in $(MODULES); do \
		out=$$(cd $$m && gofmt -l .); \
		if [ -n "$$out" ]; then echo "needs gofmt in $$m:"; echo "$$out"; bad=1; fi; \
	done; \
	if [ -n "$$bad" ]; then exit 1; fi; \
	echo "gofmt: clean"

vet:
	@for m in $(MODULES); do echo "==> go vet $$m"; (cd $$m && $(GO) vet ./...) || exit 1; done

lint:
	@if ! command -v $(GOLANGCI_LINT) >/dev/null 2>&1; then \
		echo "golangci-lint not installed; skipping (CI runs it). See https://golangci-lint.run/"; \
		exit 0; \
	fi; \
	for m in $(MODULES); do echo "==> golangci-lint $$m"; (cd $$m && $(GOLANGCI_LINT) run ./...) || exit 1; done

test:
	@for m in $(MODULES); do echo "==> go test $$m"; (cd $$m && $(GO) test ./...) || exit 1; done

test-race:
	@for m in $(MODULES); do echo "==> go test -race $$m"; (cd $$m && $(GO) test -race ./...) || exit 1; done

build:
	@for m in $(MODULES); do echo "==> go build $$m"; (cd $$m && $(GO) build ./...) || exit 1; done

validate:
	@echo "==> validate manifests + recipes"
	@cd infra/validate && $(GO) run . all "$(CURDIR)"

tidy:
	@for m in $(MODULES); do echo "==> go mod tidy $$m"; (cd $$m && $(GO) mod tidy) || exit 1; done

deploy:
	@bash infra/deploy.sh

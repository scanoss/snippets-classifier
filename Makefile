.PHONY: help test vet

help: ## List targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-14s %s\n", $$1, $$2}'

test: ## Run the Go tests
	GOWORK=off go test ./...

vet: ## go vet + gofmt check
	GOWORK=off go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l .; exit 1)

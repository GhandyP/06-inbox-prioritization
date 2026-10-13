GO ?= go
BIN := bin/06-inbox-prioritization

.PHONY: all build test vet fmt fmt-check check demo clean

all: check

build: ## Build the binary
	$(GO) build -o $(BIN) .

test: ## Run the tests
	$(GO) test ./... -count=1

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format the tree
	$(GO) fmt ./...

fmt-check: ## Fail when the tree is not gofmt-clean
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt-clean:" >&2; \
		echo "$$unformatted" >&2; \
		exit 1; \
	fi

check: fmt-check vet test ## Everything CI runs

demo: ## Run the CLI against the committed fixture (needs GOOGLE_API_KEY)
	$(GO) run . -input testdata/messages_ok.json

clean: ## Remove build output
	rm -rf bin

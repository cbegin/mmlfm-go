# mmlfm-go Makefile
# Full-featured build, test, and development targets

.PHONY: all build build-cli build-ui build-web build-release test test-verbose test-coverage run run-ui run-example \
	serve-web fmt vet lint mod-download mod-tidy clean install help

# Project config
BINARY_NAME    := play_mml
UI_BINARY_NAME := play_mml_ui
MODULE         := github.com/cbegin/mmlfm-go
MAIN_PKG       := ./cmd/play_mml
UI_PKG         := ./cmd/play_mml_ui
WASM_DIR       := bin/web
GO             := go
GOFLAGS        :=
LDFLAGS        :=
ifdef VERSION
	LDFLAGS += -ldflags "-X main.version=$(VERSION)"
endif

# Default target
all: build

# --- Build ---

build: build-cli build-ui build-web ## Build all targets (CLI, GUI, web)

build-cli: ## Build play_mml CLI
	@mkdir -p bin
	$(GO) build $(GOFLAGS) -o bin/$(BINARY_NAME) $(MAIN_PKG)

build-ui: ## Build play_mml_ui (Ebitengine GUI)
	@mkdir -p bin
	$(GO) build $(GOFLAGS) -o bin/$(UI_BINARY_NAME) $(UI_PKG)

build-release:
	$(GO) build $(GOFLAGS) $(LDFLAGS) -trimpath -ldflags "-s -w" -o bin/$(BINARY_NAME) $(MAIN_PKG)
	$(GO) build $(GOFLAGS) $(LDFLAGS) -trimpath -ldflags "-s -w" -o bin/$(UI_BINARY_NAME) $(UI_PKG)

build-all: ## Cross-compile CLI for common platforms
	GOOS=darwin  GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w" -o bin/$(BINARY_NAME)-darwin-amd64       $(MAIN_PKG)
	GOOS=darwin  GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w" -o bin/$(BINARY_NAME)-darwin-arm64       $(MAIN_PKG)
	GOOS=linux   GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w" -o bin/$(BINARY_NAME)-linux-amd64       $(MAIN_PKG)
	GOOS=linux   GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w" -o bin/$(BINARY_NAME)-linux-arm64       $(MAIN_PKG)
	GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags "-s -w" -o bin/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PKG)
	GOOS=windows GOARCH=arm64 $(GO) build -trimpath -ldflags "-s -w" -o bin/$(BINARY_NAME)-windows-arm64.exe $(MAIN_PKG)

build-web: ## Build play_mml_ui as WASM for web browsers
	@mkdir -p $(WASM_DIR)/examples
	GOOS=js GOARCH=wasm $(GO) build $(GOFLAGS) -o $(WASM_DIR)/play_mml_ui.wasm $(UI_PKG)
	cp "$$($(GO) env GOROOT)/lib/wasm/wasm_exec.js" $(WASM_DIR)/
	cp web/index.html $(WASM_DIR)/
	cp examples/*.mml $(WASM_DIR)/examples/
	@printf '[\n' > $(WASM_DIR)/examples/files.json; \
	sep=""; \
	for f in $(WASM_DIR)/examples/*.mml; do \
		[ -n "$$sep" ] && printf ',\n' >> $(WASM_DIR)/examples/files.json; \
		printf '  "%s"' "$$(basename $$f)" >> $(WASM_DIR)/examples/files.json; \
		sep=y; \
	done; \
	printf '\n]\n' >> $(WASM_DIR)/examples/files.json
	@echo "Web build ready in $(WASM_DIR)/"
	@echo "Serve with: make serve-web"

serve-web: build-web ## Serve WASM build on localhost:8080
	@echo "Serving $(WASM_DIR) at http://localhost:8080"
	cd $(WASM_DIR) && python3 -m http.server 8080

# --- Test ---

test:
	$(GO) test $(GOFLAGS) ./...

test-verbose:
	$(GO) test $(GOFLAGS) -v ./...

test-race:
	$(GO) test $(GOFLAGS) -race ./...

test-short:
	$(GO) test $(GOFLAGS) -short ./...

test-coverage:
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# --- Run ---

run: ## Run play_mml CLI with default MML
	$(GO) run $(MAIN_PKG)

run-ui: ## Run play_mml_ui (GUI; optional FILE=path to load MML)
	$(GO) run $(UI_PKG) $(FILE)

run-example: ## Run play_mml with examples/tr.mml (use FILE=path for other files)
	$(GO) run $(MAIN_PKG) -file $(or $(FILE),examples/tr.mml)

run-mml: ## Run play_mml with inline MML (use MML="..." to specify)
	$(GO) run $(MAIN_PKG) -mml $(or $(MML),"t140 o5 l8 cdefgab>c")

# --- Code quality ---

fmt: ## Format code
	$(GO) fmt ./...
	@test -z "$$(gofmt -l . | grep -v '^bin/' || true)" || (gofmt -l . | grep -v '^bin/' ; exit 1)

vet: ## Run go vet
	$(GO) vet ./...

lint: ## Run golangci-lint (install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	@command -v golangci-lint >/dev/null 2>&1 || (echo "golangci-lint not found. Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" ; exit 1)
	golangci-lint run ./...

check: fmt vet test ## Run fmt, vet, and test

# --- Dependencies ---

mod-download:
	$(GO) mod download

mod-tidy:
	$(GO) mod tidy

mod-verify:
	$(GO) mod verify

# --- Install ---

install: ## Install play_mml and play_mml_ui to $GOPATH/bin
	$(GO) install $(MAIN_PKG)
	$(GO) install $(UI_PKG)

# --- Clean ---

clean: ## Remove build artifacts
	rm -rf bin/
	rm -f coverage.out coverage.html

# --- Examples ---

examples-run: ## Run play_mml for each example file
	@for f in examples/*.mml; do \
		echo "=== $$f ===" ; \
		$(GO) run $(MAIN_PKG) -file "$$f" 2>&1 || true ; \
	done

# --- Golden tests ---

# Golden WAV hashes live in testdata/*.sha256. If you change engine behavior,
# regenerate by rendering, hashing, and updating those files.

# --- Help ---

help: ## Show this help
	@echo "mmlfm-go Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make build              Build all (CLI, GUI, web)"
	@echo "  make build-cli          Build play_mml CLI"
	@echo "  make build-ui           Build play_mml_ui GUI"
	@echo "  make build-web          Build play_mml_ui as WASM for web"
	@echo "  make serve-web          Build and serve WASM on localhost:8080"
	@echo "  make test               Run tests"
	@echo "  make run                Run CLI with default MML"
	@echo "  make run-ui             Run GUI"
	@echo "  make run-ui FILE=examples/tr.mml"
	@echo "  make run-example        Run with examples/tr.mml"
	@echo "  make run-example FILE=examples/gr.mml"
	@echo "  make run-mml MML='t120 o4 l4 cec>c'"

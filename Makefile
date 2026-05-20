# clapip - Makefile
# The primary interface for build, test, run, and release tasks.

BINARY  := clapip
BIN_DIR := bin
PKG     := github.com/nealhardesty/clapip
VERSION := $(shell awk -F'"' '/const Version/ {print $$2}' version.go)
BRANCH  := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)

.DEFAULT_GOAL := help

.PHONY: help build run test smoke clean fmt vet version install \
        bump-patch bump-minor bump-major release push

help: ## Show this help
	@echo "clapip $(VERSION) - available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "} {printf "  \033[36m%-13s\033[0m %s\n", $$1, $$2}'

build: ## Compile the binary into ./bin
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY) .

run: build ## Build and run with default flags
	./$(BIN_DIR)/$(BINARY)

test: ## Run all tests with the race detector
	go test -race ./...

smoke: ## Run curl-based smoke tests against a running server
	./scripts/test.sh

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)

fmt: ## Format all Go source
	gofmt -w .

vet: ## Run go vet
	go vet ./...

version: ## Print the current version
	@echo $(VERSION)

install: ## Install clapip into the Go bin directory
	go install $(PKG)

bump-patch: ## Increment the patch version in version.go
	@$(call bump,3)

bump-minor: ## Increment the minor version in version.go
	@$(call bump,2)

bump-major: ## Increment the major version in version.go
	@$(call bump,1)

# bump increments the Nth dotted component (1=major, 2=minor, 3=patch) of the
# version string in version.go and rewrites the file in place.
define bump
	cur=$$(awk -F'"' '/const Version/ {print $$2}' version.go); \
	v=$${cur#v}; \
	maj=$$(echo $$v | cut -d. -f1); \
	min=$$(echo $$v | cut -d. -f2); \
	pat=$$(echo $$v | cut -d. -f3); \
	case $(1) in \
	  1) maj=$$((maj+1)); min=0; pat=0 ;; \
	  2) min=$$((min+1)); pat=0 ;; \
	  3) pat=$$((pat+1)) ;; \
	esac; \
	new="v$$maj.$$min.$$pat"; \
	sed -i.bak "s/\"$$cur\"/\"$$new\"/" version.go && rm -f version.go.bak; \
	echo "version: $$cur -> $$new"
endef

release: ## Commit, tag, push, and publish a GitHub release for the current version
	@v=$$(awk -F'"' '/const Version/ {print $$2}' version.go); \
	echo "releasing $$v on branch $(BRANCH)" && \
	git commit -am "chore: bump version to $$v" && \
	git tag $$v && \
	git push origin $(BRANCH) && \
	git push origin --tags && \
	gh release create $$v --generate-notes

push: bump-patch build ## Bump patch version, build, then commit/tag/push/release
	@$(MAKE) release

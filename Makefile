# Project-wide metadata
BUILD := $(shell git rev-parse --short HEAD)
DOCKER_REGISTRY := inseefrlab

BINARIES := onyxia-onboarding onyxia-services
GOBIN := $(shell pwd)/bin

# Docker arch handling
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_M), x86_64)
	LOCAL_PLATFORM := linux/amd64
else ifeq ($(UNAME_M), aarch64)
	LOCAL_PLATFORM := linux/arm64
else ifeq ($(UNAME_M), arm64)
	LOCAL_PLATFORM := linux/arm64
else
	LOCAL_PLATFORM := linux/amd64
endif

MULTIARCH ?= 0
DOCKER_PLATFORMS := $(LOCAL_PLATFORM)
ifeq ($(MULTIARCH), 1)
	DOCKER_PLATFORMS := linux/amd64,linux/arm64
endif


# Shell helper: prints latest version (X.Y.Z) for a given component name ($1)
define sh_get_version
tag=$$(git tag -l "$1-v*" --sort=-version:refname | head -n1); \
if [ -n "$$tag" ]; then printf '%s' "$${tag#"$1-v"}"; fi
endef

# --- HELP ---------------------------------------------------------------------

.PHONY: help
help:
	@echo
	@echo "🛠️  Available make commands:"
	@echo
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/^## //g' | column -t -s ':' | sed 's/^/ /'
	@echo

# --- DEP MANAGEMENT ----------------------------------------------------------

.PHONY: install
## install: Install dependencies using Go modules
install:
	@echo "📦 Installing dependencies..."
	go mod tidy

.PHONY: verify
## verify: Verify module dependencies
verify:
	@echo "🔍 Verifying dependencies..."
	go mod verify

.PHONY: generate
## generate: Run go generate on all packages
generate:
	@echo "⚡ Running go generate..."
	go generate ./...

.PHONY: fmt
## fmt: Format all Go code
fmt:
	@echo "🖌️  Formatting code..."
	go fmt ./...

# --- LINT / TEST -------------------------------------------------------------

.PHONY: lint
## lint: Run static analysis (auto-install golangci-lint if missing or outdated)
lint:
	@echo "🔍 Running golangci-lint..."
	@mkdir -p $(GOBIN)
	@LATEST=$$(curl -s https://api.github.com/repos/golangci/golangci-lint/releases/latest | grep tag_name | cut -d '"' -f4 | sed 's/^v//'); \
	if [ ! -x "$(GOBIN)/golangci-lint" ]; then \
		echo "📥 Installing golangci-lint $$LATEST..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(GOBIN) v$$LATEST; \
	else \
		CURRENT=$$($(GOBIN)/golangci-lint --version | head -n1 | awk '{print $$4}'); \
		if [ "$$CURRENT" != "$$LATEST" ]; then \
			echo "📥 Updating golangci-lint from $$CURRENT to $$LATEST..."; \
			curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(GOBIN) v$$LATEST; \
		else \
			echo "✅ golangci-lint is up to date ($$CURRENT)"; \
		fi; \
	fi
	@$(GOBIN)/golangci-lint run --timeout=1m ./...

.PHONY: test
## test: Run all unit tests
test:
	@echo "✅ Running tests..."
	go test ./...

# --- BUILD -------------------------------------------------------------------

.PHONY: build
## build: Build all binaries
build:
	@echo "🔨 Building binaries..."
	@mkdir -p $(GOBIN)
	@for bin in $(BINARIES); do \
		comp=$${bin#onyxia-}; \
		version=$$( { $(call sh_get_version,$$comp); } ); \
		echo "📦 Building $$bin (version: $$version)..."; \
		go build -ldflags "-X=main.Version=$$version -X=main.Build=$(BUILD)" -o $(GOBIN)/$$bin ./cmd/$$bin; \
	done

.PHONY: run-%
## run-<api>: Run specific API (example: make run-onboarding)
run-%:
	@echo "🚀 Running onyxia-$*..."
	go run ./cmd/onyxia-$*/main.go

.PHONY: clean
## clean: Clean all build artifacts
clean:
	@echo "🧹 Cleaning..."
	@rm -rf $(GOBIN)
	go clean

# --- DOCKER ------------------------------------------------------------------

.PHONY: docker-setup-builder
## docker-setup-builder: Setup Docker Buildx for multiarch
docker-setup-builder:
ifeq ($(MULTIARCH), 1)
	@echo "🔧 Setting up Buildx..."
	docker buildx create --use --name multiarch-builder || true
endif

.PHONY: docker-build-%
## docker-build-<api>: Build Docker image for API (example: make docker-build-onboarding)
docker-build-%: docker-setup-builder
	@echo "🐳 Building Docker image for onyxia-$*..."
	@VERSION=$$( { $(call sh_get_version,$*); } ); \
	if [ -z "$$VERSION" ]; then \
	  echo "❌ No version tag found for '$*' (expected tags like '$*-vX.Y.Z')"; exit 1; \
	fi; \
	echo "→ version=$$VERSION"; \
	docker buildx build \
		--platform $(DOCKER_PLATFORMS) \
		--tag $(DOCKER_REGISTRY)/onyxia-$*:$$VERSION \
		--tag $(DOCKER_REGISTRY)/onyxia-$*:latest \
		--build-arg VERSION="$$VERSION" \
		--build-arg BUILD_SHA="$(BUILD)" \
		-f $*/Dockerfile \
		$(if $(filter 1,$(MULTIARCH)),,--load) \
		$(if $(PUSH),--push,) .

.PHONY: docker-push-%
## docker-push-<api>: Push Docker image to registry
docker-push-%:
	@$(MAKE) docker-build-$* PUSH=1

.PHONY: docker-run-%
## docker-run-<api>: Run Docker container
docker-run-%:
	@docker run -p 8080:8080 $(DOCKER_REGISTRY)/onyxia-$*:latest

.PHONY: docker-clean
docker-clean:
	@echo "🗑️  Removing all local docker images for project binaries..."
	@for bin in $(BINARIES); do \
		docker images "$(DOCKER_REGISTRY)/$$bin" --format '{{.Repository}}:{{.Tag}}' | xargs -r docker rmi -f || true; \
	done
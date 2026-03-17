# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is Onyxia

Onyxia is an open-source data science workspace platform. Users launch on-demand data science services (JupyterLab, VSCode, MLflow, etc.) as Helm releases on Kubernetes. This backend is a progressive Go rewrite of the original Java API, split into focused APIs (two so far, more to come):

- **onyxia-services** — provides the frontend with information about Onyxia services running on the Kubernetes cluster (catalog, installed releases, regions, user info)
- **onyxia-onboarding** — provisioning API: creates Kubernetes namespaces and applies resource quotas when a new user or group is onboarded

Each API is intentionally decoupled so they can be deployed and scaled independently.

## Tech Stack

- **Go** (latest, see `go.mod`)
- **HTTP**: `chi/v5` (router), `ogen` (type-safe OpenAPI code generation)
- **Kubernetes / Helm**: `k8s.io/client-go`, `helm.sh/helm/v3`
- **Auth**: `go-oidc/v3`, `go-jose/v4` (OIDC + DPoP)
- **Config**: `spf13/viper`
- **Logging**: `log/slog` + `go.uber.org/zap` (as backend)
- **Testing**: `testify`

## Commands

```bash
make install        # go mod tidy + install git hooks
make build          # Build all binaries to bin/
make test           # Run all unit tests (go test ./...)
make lint           # Run golangci-lint (auto-installs if missing)
make fmt            # Format all Go code
make generate       # Run go generate (regenerates OpenAPI code from specs)
make run-services   # Run services API locally
make run-onboarding # Run onboarding API locally
```

To run a single test:
```bash
go test ./services/usecase/... -run TestFunctionName
```

To verify a change is clean before pushing:
```bash
make fmt && make test && make lint
```

## Architecture

This is a monorepo. Entry points are in `cmd/`, the two APIs in `services/` and `onboarding/`, shared infrastructure in `internal/`.

```
cmd/
  onyxia-services/      # entry point
  onyxia-onboarding/    # entry point
services/               # services API
onboarding/             # onboarding API
internal/
  auth/                 # OIDC + no-auth implementations
  kube/                 # Kubernetes client wrapper
  usercontext/          # user info storage/retrieval in context
  logging/              # slog contextAttrHandler
  httputil/             # CORS, proxy headers middleware
  tools/                # misc utilities
```

Each API (`services/`, `onboarding/`) follows the same internal structure:

### Hexagonal Architecture

```
api/controller → usecase → domain ← ports ← adapters
```

- **`api/`** — HTTP layer: chi router, ogen-generated OpenAPI handlers (`api/oas/`), controllers, middleware
- **`usecase/`** — Business logic, depends only on port interfaces
- **`domain/`** — Core models and domain error types
- **`ports/`** — Interfaces for external dependencies (e.g., `HelmReleasesGateway`, `PackageRepository`)
- **`adapters/`** — Concrete implementations of ports (Helm, Kubernetes)
- **`bootstrap/`** — Dependency injection: wires adapters → usecases → controllers, loads config via Viper

### OpenAPI Code Generation

The `api/oas/` directories contain **generated code** — do not edit manually. They are regenerated from OpenAPI specs (`services/openapi.yaml`, `onboarding/openapi.yaml`) via `make generate` using ogen. Controllers implement the generated handler interface.

### Dependency Injection

Manual DI in `api/route/setup.go` — adapters are instantiated, injected into usecases, usecases injected into controllers. The `bootstrap/app.go` holds the top-level `Application` struct with shared deps (K8s client, config, user context).

### Authentication

Two modes selectable via config: OIDC (with DPoP support) or no-auth (for development). Auth middleware extracts user info into `context.Context`; controllers read it via `internal/usercontext`.

### Logging

Uses `log/slog` with a custom `contextAttrHandler` (`internal/logging/context_handler.go`) that enriches log records via an `AttrFunc`. In both APIs (`bootstrap/logger.go`), this function reads `username`, `groups`, and `roles` from the usercontext and appends them as fields. This is why the lint rule enforces `slog.*Context` variants — calling `slog.Info` instead of `slog.InfoContext` silently drops those fields from the log record.

### Configuration

Viper-based config with embedded defaults (`env.default.yaml`). Config structs use `mapstructure` tags. Config is validated at startup in `bootstrap/env/load.go`.

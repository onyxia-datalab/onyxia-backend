# Onyxia Backend

<p align="center">
    <a href="https://github.com/onyxia-datalab/onyxia-backend/actions/workflows/release.yaml">
      <img src="https://github.com/onyxia-datalab/onyxia-backend/actions/workflows/release.yml/badge.svg?branch=main">
    </a>
    <a href="https://join.slack.com/t/3innovation/shared_invite/zt-2skhjkavr-xO~uTRLgoNOCm6ubLpKG7Q">
      <img src="https://img.shields.io/badge/slack-550_Members-brightgreen.svg?logo=slack">
    </a>
</p>

Go-based backend for the [Onyxia](https://onyxia.sh) platform. This monorepo is a progressive refactor of the original [Java Onyxia API](https://github.com/inseeFrlab/onyxia-api), split into focused REST API modules.

## Project Website

Visit [onyxia.sh](https://onyxia.sh) to learn more about the Onyxia ecosystem.

## APIs

| Module              | Description                                                   | Docs                                         |
| ------------------- | ------------------------------------------------------------- | -------------------------------------------- |
| `onyxia-services`   | Core services API (Helm releases, regions, user info)         | [services/README.md](services/README.md)     |
| `onyxia-onboarding` | Kubernetes namespace provisioning with quotas and annotations | [onboarding/README.md](onboarding/README.md) |

Both APIs follow a [hexagonal architecture](https://alistair.cockburn.us/hexagonal-architecture/) pattern: `api → usecase → domain ← adapters`.

## Getting started

**Prerequisites:** Go, Docker (optional), a Kubernetes cluster with appropriate RBAC.

```sh
git clone https://github.com/onyxia-datalab/onyxia-backend.git
cd onyxia-backend
make install
```

Copy and edit the configuration for the API you want to run:

```sh
cp onboarding/bootstrap/env.default.yaml env.onboarding.yaml
cp services/bootstrap/env/env.default.yaml env.services.yaml
```

## Common commands

| Command                        | Description                       |
| ------------------------------ | --------------------------------- |
| `make build`                   | Build all binaries to `bin/`      |
| `make test`                    | Run all tests                     |
| `make lint`                    | Run `golangci-lint`               |
| `make fmt`                     | Format all Go code                |
| `make run-onboarding`          | Run the onboarding API locally    |
| `make run-services`            | Run the services API locally      |
| `make docker-build-onboarding` | Build Docker image for onboarding |
| `make docker-build-services`   | Build Docker image for services   |
| `make docker-push`             | Build and push all images         |

Run `make help` for the full list.

## Docker

Docker images are built per API. By default the image targets your local architecture.

```sh
make docker-build-onboarding
make docker-run-onboarding
```

To build for both `amd64` and `arm64`:

```sh
MULTIARCH=1 make docker-build-onboarding
```

To push to a registry:

```sh
DOCKER_REGISTRY=<your-registry> make docker-push
```

## Repository structure

```
cmd/                    # Entry points for each binary
  onyxia-services/
  onyxia-onboarding/
services/               # Services API module
onboarding/             # Onboarding API module
internal/               # Shared packages (auth, logging, k8s, ...)
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature-name`)
3. Commit using [conventional commits](git-conventional-commits.yaml)
4. Open a pull request

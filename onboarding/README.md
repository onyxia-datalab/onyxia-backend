# Onyxia Onboarding


REST API responsible for managing user onboarding in Onyxia by provisioning **Kubernetes namespaces** with associated **quotas and annotations**.

## Features

- **Automated namespace creation**: Ensures users have their own dedicated Kubernetes namespace.
- **Resource quotas**: Enforces limits on CPU, GPU, memory, and storage usage.
- **Namespace annotations**: Allows additional metadata if enabled via environment variables.
- **REST API**: Simple and efficient API for managing onboarding operations.

## Local development

From the repository root:

```sh
cp onboarding/bootstrap/env.default.yaml env.onboarding.yaml
```

Modify `env.yaml` as needed (`env.yaml` is git-ignored).

```sh
make run-onboarding  # start the API
make test            # run tests
```

## Environment variables

The configuration is loaded via [Viper](https://github.com/spf13/viper) from:

1. Embedded `bootstrap/env.default.yaml`
2. An `env.yaml` file at the repository root (overrides defaults)
3. Direct environment variables

### General

| Variable             | Description                      | Default |
| -------------------- | -------------------------------- | ------- |
| `authenticationMode` | Authentication mode (none, oidc) | `none`  |

### Server

| Variable | Description | Default |
| -------- | ----------- | ------- |
| `port`   | Server port | `8080`  |

### Security

| Variable             | Description                  | Default |
| -------------------- | ---------------------------- | ------- |
| `corsAllowedOrigins` | List of allowed CORS origins | `[]`    |

### OIDC Authentication

| Variable        | Description           | Default              |
| --------------- | --------------------- | -------------------- |
| `issuerURI`     | OIDC Issuer URI       | `""`                 |
| `skipTLSVerify` | Skip TLS verification | `false`              |
| `clientID`      | OIDC Client ID        | `""`                 |
| `audience`      | OIDC Audience         | `""`                 |
| `usernameClaim` | Claim for username    | `preferred_username` |
| `groupsClaim`   | Claim for groups      | `groups`             |
| `rolesClaim`    | Claim for roles       | `roles`              |

### Onboarding

| Variable               | Description                                                                    | Default                      |
| ---------------------- | ------------------------------------------------------------------------------ | ---------------------------- |
| `namespacePrefix`      | Prefix for user namespaces                                                     | `user-`                      |
| `groupNamespacePrefix` | Prefix for group namespaces                                                    | `projet-`                    |
| `namespaceLabels`      | Static labels to add to the namespace (at creation and subsequent user logins) | `{ "created-by": "onyxia" }` |
| `annotations`          | See [Annotations](#annotations)                                                |                              |
| `quotas`               | See [Quotas](#quotas)                                                          |                              |

#### Annotations

| Variable                     | Description                                                                                     | Default |
| ---------------------------- | ----------------------------------------------------------------------------------------------- | ------- |
| `enabled`                    | Enable annotations                                                                              | `false` |
| `static`                     | Static annotations key-value pairs                                                              | `{}`    |
| `dynamic.lastLoginTimestamp` | Track last login timestamp by adding `onyxia_last_login_timestamp: <unix time in milliseconds>` | `false` |
| `dynamic.userAttributes`     | List of user attributes                                                                         | `[]`    |

#### Quotas

| Variable       | Description                                                                                                                                                                                      | Default |
| -------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------- |
| `enabled`      | Enable quotas                                                                                                                                                                                    | `false` |
| `default`      | Default quotas values — see [Quota values](#quota-values)                                                                                                                                        |         |
| `userEnabled`  | Enable user-specific quotas                                                                                                                                                                      | `false` |
| `user`         | User quotas values — see [Quota values](#quota-values)                                                                                                                                           |         |
| `groupEnabled` | Enable group-specific quotas                                                                                                                                                                     | `false` |
| `group`        | Group quotas values — see [Quota values](#quota-values)                                                                                                                                          |         |
| `roles`        | Map of quotas corresponding to user roles. In case the user has multiple of those roles, only the first one will be applied. If user has no role from this list then user quota will be applied. | `{}`    |

#### Quota values

| Variable                     | Description                       | Default |
| ---------------------------- | --------------------------------- | ------- |
| `requests.memory`            | Default requested memory limit    | `10Gi`  |
| `requests.cpu`               | Default requested CPU limit       | `10`    |
| `limits.memory`              | Default memory limit              | `10Gi`  |
| `limits.cpu`                 | Default CPU limit                 | `10`    |
| `requests.storage`           | Default storage request           | `100Gi` |
| `count.pods`                 | Default max pods count            | `50`    |
| `requests.ephemeral-storage` | Default ephemeral storage request | `10Gi`  |
| `limits.ephemeral-storage`   | Default ephemeral storage limit   | `20Gi`  |
| `requests.nvidia.com/gpu`    | Default GPU requests              | `0`     |
| `limits.nvidia.com/gpu`      | Default GPU limits                | `0`     |

The full configuration structure can be found in [`bootstrap/env.default.yaml`](bootstrap/env.default.yaml).

# Onyxia Onboarding


REST API responsible for managing user onboarding in Onyxia by provisioning **Kubernetes namespaces** with associated **quotas, annotations and admin-defined resources**.

## Features

- **Automated namespace creation**: Ensures users have their own dedicated Kubernetes namespace.
- **Resource quotas**: Enforces limits on CPU, GPU, memory, and storage usage.
- **Namespace annotations**: Allows additional metadata if enabled via environment variables.
- **Extra resources**: Applies admin-defined namespaced objects (e.g. a service account, role and role binding) in every namespace.
- **REST API**: Simple and efficient API for managing onboarding operations.

## Local development

From the repository root:

```sh
cp onboarding/bootstrap/env.default.yaml env.onboarding.yaml
```

Modify `env.onboarding.yaml` as needed (git-ignored).

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
| `skipTLSVerify` | Disable TLS certificate verification when retrieving the IdP's discovery document and keys. Token signatures are still verified. Not intended for production: mount the IdP's CA in `/usr/local/share/ca-certificates` instead. | `false` |
| `publicKey`     | Optional: base64-encoded X.509 (DER) RSA public key used to validate tokens instead of retrieving the keys from `issuerURI`. In this mode the issuer is never contacted and the token issuer is not checked (same as onyxia-api's `oidc.public-key`). | `""` |
| `audience`      | OIDC Audience         | `""`                 |
| `usernameClaim` | Claim for username    | `preferred_username` |
| `groupsClaim`   | Claim for groups      | `groups`             |
| `rolesClaim`    | Claim for roles       | `roles`              |

Extra CA certificates (PEM) mounted in `/usr/local/share/ca-certificates` are trusted in addition to the system ones (`SSL_CERT_DIR`, same directory as onyxia-api's `CACERTS_DIR`).

### Onboarding

| Variable               | Description                                                                    | Default                      |
| ---------------------- | ------------------------------------------------------------------------------ | ---------------------------- |
| `namespacePrefix`      | Prefix for user namespaces                                                     | `user-`                      |
| `groupNamespacePrefix` | Prefix for group namespaces                                                    | `projet-`                    |
| `namespaceLabels`      | Static labels to add to the namespace (at creation and subsequent user logins) | `{ "created-by": "onyxia" }` |
| `annotations`          | See [Annotations](#annotations)                                                |                              |
| `quotas`               | See [Quotas](#quotas)                                                          |                              |
| `namespaceResources`   | See [Namespace resources](#namespace-resources)                                | `[]`                         |

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

#### Namespace resources

`namespaceResources` lists Kubernetes objects, chosen by the platform admin, applied in every onboarded namespace at each onboarding, in the configured order. A typical use is a namespace-scoped identity to install services with (`ServiceAccount`, `Role`, `RoleBinding`), instead of the backend's own cluster-wide one.

- Each entry is **one YAML manifest, as a string** (`- |`): the configuration loader lowercases every key it reads, which would break fields such as `apiVersion` if manifests were written as plain YAML.
- Objects are applied with a **forced server-side apply** (field manager `onyxia-onboarding`): the fields a manifest sets are brought back to their configured values at each onboarding, even if they were changed in the namespace; fields it does not set are left alone. Removing a manifest from the configuration does not delete the objects already created.
- `metadata.namespace` must not be set: objects are always created in the onboarded namespace. **Cluster-scoped kinds** (`ClusterRole`, `Namespace`…) **are rejected**, as are unknown kinds. Everything is checked at startup.
- Objects get the label `created-by: onyxia` unless the manifest sets it.
- String values may contain [placeholders](#placeholders), replaced at each onboarding.

##### Placeholders

Any string value of a manifest (names, labels, annotations, `subjects`, `ConfigMap` data…) may contain the following placeholders, anywhere in the string (`${onyxia.username}-sa` is valid):

| Placeholder           | Replaced by                                                                                                                         | User namespace `user-jdoe` (onboarded by `jdoe`) | Group namespace `projet-sspcloud` (onboarded by member `jdoe`) |
| --------------------- | ----------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------ | -------------------------------------------------------------- |
| `${onyxia.namespace}` | The onboarded namespace. Typically the `namespace` of a `ServiceAccount` subject in a `RoleBinding`.                                | `user-jdoe`                                      | `projet-sspcloud`                                              |
| `${onyxia.username}`  | The user who triggered the onboarding (`usernameClaim` of their token). For a group namespace, this is whichever member onboarded it, so it should not be used to identify the namespace owner. | `jdoe`                                           | `jdoe`                                                         |
| `${onyxia.group}`     | The single group named in the onboarding request (`group` of the request body), without `groupNamespacePrefix`. It is not the array of the user's OIDC groups (`groupsClaim`): the request is only accepted if the user belongs to that group. Empty for a user namespace. | *(empty)*                                        | `sspcloud`                                                     |

- Placeholders are replaced on the parsed object, never in the YAML text: a value (the username comes from the user's token) always stays inside the string it replaces and can never add fields to the object.
- Only `${onyxia.*}` placeholders are replaced. An unknown one (`${onyxia.user}`, a typo…) is rejected at startup. Any other `${...}`, such as a shell variable in a script stored in a `ConfigMap`, is left untouched.
- Deploying the configuration through Terraform's `templatefile`, which expands every `${...}`, requires escaping them as `$${onyxia.namespace}`.

The onboarding API's own service account must be allowed to apply (`patch`) every kind listed, and, because of Kubernetes RBAC escalation prevention, hold every permission granted by a `Role` it applies (or the `escalate` verb on `roles`), and `bind` on a role referenced by a `RoleBinding`. A `RoleBinding`'s `roleRef` cannot be changed once created: delete the existing bindings if you change it.

Example (namespace-scoped identity to install services with):

```yaml
onboarding:
  namespaceResources:
    - |
      apiVersion: v1
      kind: ServiceAccount
      metadata:
        name: onyxia-installer-sa
      automountServiceAccountToken: false
    - |
      apiVersion: rbac.authorization.k8s.io/v1
      kind: Role
      metadata:
        name: onyxia-installer-role
      rules:
        - apiGroups: [""]
          resources: ["pods", "services", "configmaps", "secrets", "persistentvolumeclaims"]
          verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
        - apiGroups: ["apps"]
          resources: ["deployments", "statefulsets"]
          verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
    - |
      apiVersion: rbac.authorization.k8s.io/v1
      kind: RoleBinding
      metadata:
        name: onyxia-installer-rb
      roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: Role
        name: onyxia-installer-role
      subjects:
        - kind: ServiceAccount
          name: onyxia-installer-sa
          namespace: ${onyxia.namespace}
```

The full configuration structure can be found in [`bootstrap/env.default.yaml`](bootstrap/env.default.yaml).

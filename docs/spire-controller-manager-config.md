# SPIRE Controller Manager Configuration

The SPIRE Controller Manager configuration is defined [here](../api/v1alpha1/controllermanagerconfig_types.go).

Beyond the
standard [controller manager configuration](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/config/v1alpha1#ControllerConfigurationSpec),
the following fields are defined: 

| Field                                | Required | Default                                          | Description                                                                                                                                                                                                   |
|--------------------------------------|----------|--------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `clusterName`                        | REQUIRED |                                                  | The name of the cluster                                                                                                                                                                                       |
| `trustDomain`                        | REQUIRED |                                                  | The trust domain name for the cluster                                                                                                                                                                         |
| `clusterDomain`                      | OPTIONAL |                                                  | The domain of the cluster, ie `cluster.local`. If not specified will attempt to auto detect.                                                                                                                  |
| `ignoreNamespaces`                   | OPTIONAL | `["kube-system", "kube-public", "spire-system"]` | Namespaces that the controllers should ignore                                                                                                                                                                 |
| `validatingWebhookConfigurationName` | OPTIONAL | `spire-controller-manager-webhook`               | The name of the validating admission controller webhook to manage                                                                                                                                             |
| `gcInterval`                         | OPTIONAL | `10s`                                            | How often the SPIRE state is reconciled when the controller is otherwise idle. This impacts how quickly SPIRE state will converge after CRDs are removed or SPIRE state is mutated underneath the controller. |
| `spireServerSocketPath`              | OPTIONAL | `/spire-server/api.sock`                         | The path the the SPIRE Server API socket                                                                                                                                                                      |
| `logLevel`                           | OPTIONAL | `info`                                           | The log level for the controller manager. Supported values are `info`, `error`, `warn` and `debug`.                                                                                                           |
| `logEncoding`                        | OPTIONAL | `console`                                        | The log encoder for the controller manager. Supported values are `console` and `json`.                                                                                                                        |
| `className`                          | OPTIONAL |                                                  | Only sync resources that have the specified className set on them.                                                                                                                                            |
| `watchClassless`                     | OPTIONAL |                                                  | If className is set, also watch for resources that do not have any className set.                                                                                                                             |
| `staticManifestPath`                 | OPTIONAL |                                                  | If specified, manifests will be read from disk instead of from Kubernetes                                                                                                                                     |
| `grpc`                               | OPTIONAL |                                                  | Allows configuring the GRPC config used when connecting to the SPIRE server API.                                                                                                                              |
| `tlsProfile`                         | OPTIONAL |                                                  | TLS security profile for terminating endpoints such as the admission webhook server. When unset, Go TLS defaults are used.                                                                                  |

### TLS Profile

The `tlsProfile` block applies only to TLS-terminating endpoints (currently the admission webhook server).

| Field                          | Required | Default | Description                                                                                                                                                                                                 |
|--------------------------------|----------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `tlsProfile.minTLSVersion`     | OPTIONAL |         | Minimum TLS version in Kubernetes-style naming (for example `VersionTLS12`). When unset, the minimum TLS version is not changed.                                                                          |
| `tlsProfile.cipherSuites`      | OPTIONAL |         | Allowed cipher suites in IANA naming (for example `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`). When unset, Go defaults are used. TLS 1.3 cipher suites cannot be configured individually in Go.                |
| `tlsProfile.curvePreferences`  | OPTIONAL |         | Ordered list of allowed key exchange curves or groups (for example `X25519MLKEM768`, `X25519`, `secp256r1`). When unset, Go defaults are used.                                                            |

Example:

```yaml
tlsProfile:
  minTLSVersion: VersionTLS12
  cipherSuites:
    - TLS_AES_128_GCM_SHA256
    - TLS_AES_256_GCM_SHA384
    - TLS_CHACHA20_POLY1305_SHA256
    - TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256
    - TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
    - TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384
    - TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256
    - TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256
  curvePreferences:
    - X25519MLKEM768
    - X25519
    - secp256r1
    - secp384r1
```

GRPC Config Options

| Field                                | Required | Default | Description                                               |
|--------------------------------------|----------|---------|-----------------------------------------------------------|
| `maxCallRecvMsgSize`                 | OPTIONAL | 4MB     | The maximum message size in bytes the client can receive. |

## Kubernetes Mode

By default, all objects are synced from the Kubernetes cluster the spire-controller-manager is running in.

## Static Mode

If `staticManifestPath` is specified, Kubernetes will not be used and instead, manifests are loaded from yaml files located in the specified path and synchronized to the SPIRE server.

In this mode, validating webhooks will be ignored as its not useful without Kubernetes.

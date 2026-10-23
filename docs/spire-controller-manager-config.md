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
| `clusterSPIFFEIDLabelSelector`       | OPTIONAL |                                                  | If specified, restricts the ClusterSPIFFEID informer cache to only objects matching the given labels. When empty, all ClusterSPIFFEID objects are cached. Useful in SPIRE setups to limit the controller manager cache to only the targeted ClusterSPIFFEIDs it needs to reconcile. |
| `filterByClassName`                  | OPTIONAL |                                                  | If set, restricts the ClusterSPIFFEID informer cache to objects labeled with this controller's `className`, using the well-known `spire.spiffe.io/class-name` label. Shorthand for adding `{"spire.spiffe.io/class-name": className}` to `clusterSPIFFEIDLabelSelector`; if that selector already sets the label to a different value, this option's value takes precedence. Requires `className` to be set. |
| `staticManifestPath`                 | OPTIONAL |                                                  | If specified, manifests will be read from disk instead of from Kubernetes                                                                                                                                     |
| `grpc`                               | OPTIONAL |                                                  | Allows configuring the GRPC config used when connecting to the SPIRE server API.                                                                                                                              |
| `tlsConfig`                         | OPTIONAL |                                                  | TLS security config for terminating endpoints such as the admission webhook server. When unset, the minimum TLS version defaults to TLS 1.2.                                                                                  |

### TLS Config

The `tlsConfig` block applies only to TLS-terminating endpoints (currently the admission webhook server).

| Field                          | Required | Default | Description                                                                                                                                                                                                 |
|--------------------------------|----------|---------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `tlsConfig.minTLSVersion`     | OPTIONAL | TLS 1.2 | Minimum TLS version in Kubernetes-style naming (for example `VersionTLS12`, `VersionTLS13`). Values below TLS 1.2 cause startup to fail. TLS 1.3 connections may still be negotiated when the minimum is TLS 1.2.                                                                          |
| `tlsConfig.cipherSuites`      | OPTIONAL |         | Allowed cipher suites in IANA naming for TLS 1.2 connections (for example `TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256`). Insecure cipher suites are logged and skipped. When unset or all entries are unsupported, Go defaults are used. Ignored with a log message when `minTLSVersion` is `VersionTLS13` or higher because Go negotiates TLS 1.3 cipher suites automatically.                |
| `tlsConfig.curvePreferences`  | OPTIONAL |         | Ordered list of allowed key exchange curves or groups (for example `X25519MLKEM768`, `X25519`, `secp256r1`). Decimal curve IDs are supported and validated via component-base. Classical and hybrid curves may be configured together. With `minTLSVersion` below `VersionTLS13`, hybrid curves are kept for TLS 1.3 connections but are ignored for TLS 1.2; at least one classical curve is required. When unset or all entries are unsupported, Go defaults are used.                                                            |

Unsupported or incompatible values are logged at startup. Invalid values that cannot be parsed cause the controller manager to fail at startup.

Example:

```yaml
tlsConfig:
  minTLSVersion: VersionTLS13
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

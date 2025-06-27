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

## Kubernetes Mode

By default, all objects are synced from the Kubernetes cluster the spire-controller-manager is running in.

## Static Mode

If `staticManifestPath` is specified, Kubernetes will not be used and instead, manifests are loaded from yaml files located in the specified path and synchronized to the SPIRE server.

In this mode, validating webhooks will be ignored as its not useful without Kubernetes.

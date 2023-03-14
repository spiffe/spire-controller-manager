# ClusterSPIFFEID Custom Resource Definition

The ClusterSPIFFEID Custom Resource Definition (CRD) is a cluster-wide resource
used to register workloads with SPIRE.

The ClusterSPIFFEID can target all workloads in the cluster, or can be
optionally scoped to specific pods or namespaces via label selectors.

The controller registers the workloads with SPIRE, using templates to provide
per-workload customization to various properties of the registration (e.g. the
SPIFFE ID).

The definition can be found [here](../api/v1alpha1/clusterspiffeid_types.go).

## ClusterSPIFFEIDSpec

| Field | Required | Description |
| ----- | -------- | ----------- |
| `spiffeIDTemplate`          | REQUIRED | The template used to render the SPIFFE ID of the workload. See [Templates](#templates). |
| `podSelector`               | OPTIONAL | A label selector used to scope which workload pods this ClusterSPIFFEID targets |
| `namespaceSelector`         | OPTIONAL | A label selector used to scope which workload namespaces this ClusterSPIFFEID targets |
| `dnsNameTemplates`          | OPTIONAL | One or more templates used to render DNS names for the target workload. See [Templates](#templates). |
| `workloadSelectorTemplates` | OPTIONAL | One or more templates used to render additional selectors for the target workload. See [Templates](#templates). |
| `ttl`                       | OPTIONAL | Duration value indicating an upper bound on the time-to-live for SVIDs issued to target workload |
| `federatesWith`             | OPTIONAL | One or more trust domain names that target workloads federate with |
| `admin`                     | OPTIONAL | Indicates whether the target workload is an admin workload (i.e. can access SPIRE administrative APIs) |
| `downstream`                | OPTIONAL | Indicates that the entry describes a downstream SPIRE server. |

## ClusterSPIFFEIDStatus

| Field | Description |
| ----- | ----------- |
| `stats` | Statistics on what the ClusterSPIFFEID was applied to and any failures. See [ClusterSPIFFEIDStats](#cluster-spiffeid-stats). |

### ClusterSPIFFEIDStats

| Field | Description |
| ----- | ----------- |
| `namespaceSelected`      | How many namespaces were selected |
| `namespacesIgnored`      | How many namespaces were ignored |
| `podsSelected`           | How many pods were selected |
| `podEntryRenderFailures` | How many failures were encountered rendering a registration entry for the pod |
| `entriesMasked`          | How many entries were masked because they were similar to other registration entries |
| `entriesToSet`           | How many entries are supposed to exist based on the targeted workloads |
| `entryFailures`          | How many entries were unable to be created/updated on SPIRE server |

## Templates

Many of the fields in the specification define templates. These templates are
rendered using the Go standard library [text template](https://pkg.go.dev/text/template) package.

The following data is available to the template:

| Field | Type | Description |
| ----- | ---- | ----------- |
| `{{ .TrustDomain }}`   | string                                                                           | The name of the trust domain the controller is operating for |
| `{{ .ClusterName }}`   | string                                                                           | The name of the cluster, as defined in the controller [configuration](./spire-controller-manager-config.md) |
| `{{ .ClusterDomain }}` | string                                                                           | The domain of the cluster, as defined in the controller [configuration](./spire-controller-manager-config.md) |
| `{{ .PodMeta }}`       | [ObjectMeta](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta) | The pod metadata |
| `{{ .PodSpec }}`       | [PodSpec](https://pkg.go.dev/k8s.io/api/core/v1#PodSpec)                         | The pod specification |
| `{{ .NodeMeta }}`      | [ObjectMeta](https://pkg.go.dev/k8s.io/apimachinery/pkg/apis/meta/v1#ObjectMeta) | The node metadata for the node the pod is scheduled on |
| `{{ .NodeSpec }}`      | [NodeSpec](https://pkg.go.dev/k8s.io/api/core/v1#NodeSpec)                       | The node specification for the node the pod is scheduled on |

## Examples

1. Apply an Istio-style SPIFFE ID to workloads running in namespaces with the "backend" label:

    ```yaml
    apiVersion: spire.spiffe.io/v1alpha1
    kind: ClusterSPIFFEID
    metadata:
      name: backend-workloads
    spec:
      spiffeIDTemplate: "spiffe://domain.test/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
      namespaceSelector:
        matchLabels:
          backend: "true"
    ```

1. Federate workloads running the pods with the "banking" label with the "auditing" trust domain.

    ```yaml
    apiVersion: spire.spiffe.io/v1alpha1
    kind: ClusterSPIFFEID
    metadata:
      name: backend-workloads
    spec:
      spiffeIDTemplate: "spiffe://domain.test/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
      podSelector:
        matchLabels:
          banking: "true"
      federatesWith: ["auditing"]
    ```

1. Add a DNS name:

    ```yaml
    apiVersion: spire.spiffe.io/v1alpha1
    kind: ClusterSPIFFEID
    metadata:
      name: backend-workloads-with-dns-names
    spec:
      spiffeIDTemplate: "spiffe://domain.test/ns/{{ .PodMeta.Namespace }}/sa/{{ .PodSpec.ServiceAccountName }}"
      dnsNameTemplates: ["{{ .PodMeta.Name }}.{{ .PodMeta.Namespace }}.{{ .ClusterDomain }}"]
    ```

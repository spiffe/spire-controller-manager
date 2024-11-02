# ClusterStaticEntry Custom Resource Definition

The ClusterStaticEntry Custom Resource Definition (CRD) is a cluster-wide
resource used to automate the registration of workloads that aren't running
within the Kubernetes cluster.

The definition can be found [here](../api/v1alpha1/clusterstaticentry_types.go).

## ClusterStaticEntrySpec

| Field | Required | Description |
| ----- | -------- | ----------- |
| `spiffeID`                  | REQUIRED | The SPIFFE ID of the workload or node alias |
| `parentID`                  | REQUIRED | The parent ID of the node or nodes authorized for the entry or the SPIRE server ID for a node alias |
| `selectors`                 | REQUIRED | One or more workload selectors (when registering a workload) or node selectors (when registering a node alias) |
| `federatesWith`             | OPTIONAL | One or more trust domain names that target workloads federate with |
| `x509SVIDTTL`               | OPTIONAL | Duration value indicating an upper bound on the time-to-live for X509-SVIDs issued to target workload |
| `jwtSVIDTTL`                | OPTIONAL | Duration value indicating an upper bound on the time-to-live for JWT-SVIDs issued to target workload |
| `dnsNames`                  | OPTIONAL | One or more DNS names for the target workload |
| `hint`                      | OPTIONAL | An opaque string that is provided to the workload as a hint on how the SVID should be used |
| `admin`                     | OPTIONAL | Indicates whether the target workload is an admin workload (i.e. can access SPIRE administrative APIs) |
| `downstream`                | OPTIONAL | Indicates that the entry describes a downstream SPIRE server. |
| `storeSVID`                 | OPTIONAL | Indicates whether the issued SVID must be stored through an SVIDStore plugin. |
| `className`                 | OPTIONAL | The class name of the SPIRE controller manager. |

## ClusterStaticEntryStatus

| Field | Description |
| ----- | ----------- |
| `rendered` | True if the cluster static entry was successfully rendered into a registration entry |
| `masked` | True if the entry produced by the cluster static entry was masked by another entry |
| `set` | True if the entry produced by the cluster static entry was successfully set on the SPIRE server |

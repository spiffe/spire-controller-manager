# SPIRE Controller Manager


[![Build Status](https://github.com/spiffe/spire-controller-manager/actions/workflows/nightly_build.yaml/badge.svg)](https://github.com/spiffe/spire-controller-manager/actions/workflows/nightly_build.yaml)
[![Pre-Production Phase](https://github.com/spiffe/spiffe/blob/main/.img/maturity/pre-prod.svg)](https://github.com/spiffe/spiffe/blob/main/MATURITY.md#pre-production)


A [Kubernetes Controller](https://kubernetes.io/docs/concepts/architecture/controller/)
manager which facilitates the registration of workloads and establishment
of federation relationships.

## How it Works

### Custom Resources

#### ClusterSPIFFEID

The [ClusterSPIFFEID](docs/clusterspiffeid-crd.md) resource is a cluster scoped
CRD that describes the shape of the identity that is applied to workloads, as
well as selectors that describe which workloads the identity applies to.

#### ClusterFederatedTrustDomain

The [ClusterFederatedTrustDomain](docs/clusterfederatedtrustdomain-crd.md)
resource is a cluster scoped CRD that describes a federation relationship for
the cluster.

### ClusterStaticEntry

The [ClusterStaticEntry](docs/clusterstaticentry-crd.md) resource is a cluster
scoped CRD that describes a static SPIRE registration entry. It is typically
used for registering workloads that do not run in the Kubernetes cluster but
otherwise need to be part of the trust domain (e.g. downstream nested SPIRE
servers).

### Reconciliation

#### Workload Registration

To facilitate workload registration, the SPIRE Controller manager registers
controllers against the following resources:

- [Pods](https://kubernetes.io/docs/concepts/workloads/pods/)
- [ClusterSPIFFEID](docs/clusterspiffeid-crd.md)
- [ClusterStaticEntry](docs/clusterstaticentry-crd.md)

When changes are detected on these resources, a workload reconciliation process
is triggered. This process determines which SPIRE entries should exist based on
the existing Pods and ClusterSPIFFEID resources which apply to those pods, as
well as static entries declared via ClusterStaticEntry resources. The
reconciliation process creates, updates, and deletes entries on SPIRE server as
appropriate to match the declared state.

#### Federation

To facilitate federation, the SPIRE Controller manager registers controllers
against the following resources:

- [ClusterFederatedTrustDomain](docs/clusterfederatedtrustdomain-crd.md)

When changes are detected on these resources, a federation relationship
reconciliation process is triggered. This process determines which SPIRE
federation relationships should exist based on the existing
ClusterFederatedTrustDomain resources. It creates, updates, and deletes
federation relationships as appropriate to match the declared state.

## Deployment

The SPIRE Controller Manager is designed to be deployed in the same pod as the
SPIRE Server. It communicates with the SPIRE Server API using a private Unix
Domain Socket within a shared volume. It requires [configuration](docs/spire-controller-manager-config.md)
for the environment where it is being deployed.

The [demo](demo) includes [sample configuration](demo/config/cluster1) for
deploying the SPIRE Controller Manager, SPIRE, and the SPIFFE CSI driver,
including requisite RBAC and Webhook configuration.

### Upgrading

The SPIRE Controller Manager must have the correct set of [Custom Resources](#custom-resources) 
and the `manager-role` that corresponds to the version to be installed.

Before upgrading, please install custom resources from [config/crd](/config/crd) and 
verify that [manager-role](/config/rbac/role.yaml) is up-to-date.

## Compatibility

The SPIRE APIs used by the SPIRE Controller Manager are generally stable and
supported since at least SPIRE v1.0. However, the API has gained support for
additional entry fields beyond what was supported in SPIRE v1.0. Notably, these
include the `jwt_svid_ttl`, `hint` and the `store_svid` fields. The
ClusterStaticEntry CRD allows these fields to be set, however, a SPIRE server
that does not support these fields will not retain them. This means if these
fields are set on a ClusterStaticEntry with an older version of SPIRE, the
SPIRE Controller Manager will continously try to reconcile SPIRE server. In
order to use these fields, you must be on a version of SPIRE Server which
supports them.

At the moment, SPIRE Controller Manager will silently try and reconcile these
fields over and over. Future updates may cause the SPIRE Controller Manager
to fail when an unsupporting SPIRE Server is encounted while these fields
are set.

The `hint` field is supported as of SPIRE 1.6.3.

The `jwt_svid_ttl` field is supported as of SPIRE 1.5.0.

The `store_svid` field is supported as of SPIRE 1.1.0.

## Demo

[Link](demo)

## Troubleshooting

### Workloads

#### Workload Not Registered

##### ClusterSPIFFEID Not Defined

Define a ClusterSPIFFEID that applies to the workload pod.

##### Workload Pod Excluded by ClusterSPIFFEID PodSelector or NamespaceSelector

Adjust the ClusterSPIFFEID selectors.

##### Failed to Render Templates Against Workload Pod or Node

Check the ClusterSPIFFEID status for entry render failures. Check logs to
determine why the rendering failed.

##### Failed to Register with SPIRE Server

Check logs for API failures talking to SPIRE Server.

### Federation

#### Federation Relationship Missing

##### ClusterFederatedTrustDomain Not Defined

Define a ClusterFederatedTrustDomain for the target trust domain.

##### ClusterFederatedTrustDomain TrustDomain Conflict

Ensure each ClusterFederatedTrustDomain resource has a unique trust domain. The
controller will only ignore all but the oldest ClusterFederatedTrustDomain
resource with a conflicting trust domain. 

#### Workload Not Federated With Trust Domain

Check the ClusterSPIFFEID for the workload. The federatesWith field must
include the federated trust domain.

## Security

### Reporting a Vulnerability

Vulnerabilities can be reported by sending an email to security@spiffe.io. A
confirmation email will be sent to acknowledge the report within 72 hours. A
second acknowledgement will be sent within 7 days when the vulnerability has
been positively or negatively confirmed.

# Changelog

## [0.3.0] - 2023-09-14

<pre>&#1F6A8;</pre><font size='7'>***PLEASE READ BEFORE UPGRADING***</font><pre>&#1F6A8;</pre>

 This version contains changes in the `ClusterSPIFFEID` CRD. It also adds a new `ClusterStaticEntry` CRD. Before upgrading you __MUST__ do the following, in order:

- Update/install those CRDs into your cluster (see [here](./config/crd/bases/spire.spiffe.io_clusterstaticentries.yaml) and [here](./config/crd/bases/spire.spiffe.io_clusterspiffeids.yaml)).
- Update the the `manager-role` ClusterRole, which includes additional permissions for the new `ClusterStaticEntry` CRD (see [here](./config/rbac/role.yaml))

### Added

- ClusterStaticEntry CRD for registering workloads that live outside the cluster (#149)
- ClusterSPIFFEID CRD can configure JWT-SVID TTL (#189)
- The namespaces to ignore can now be defined using a regex (#170)

### Updated

- Minor documentation changes (#213)

### Changed

- Use distroless static image as base (#198)

## [0.2.3] - 2023-06-20

### Added

- Auto-detection for the cluster domain name (#90)

### Updated

- Examples to use the downward API to locate the kubelet for Kubernetes workload attestation (#160)
- Migrated to the latest controller runtime (#151)

### Security

- Enforce TLS1.2 as a minimum version on the webhook server (#128)

## [0.2.2] - 2023-02-28

### Added

- Multiarch docker images supporting both amd64 and arm64 (#51)
- Support for registration for downstream workloads (#44)
- Migration guide for migrating from the k8s-workload-registrer (#40)

### Fixed

- Status subresource yaml in demo preventing status from being updated (#38)

### Changed

- Waits for 5 seconds for the SPIRE Server socket to become available (#80)
- Generated DNS Names are deduplicated before registration (#85)

## [0.2.1] - 2022-07-11

### Fixed

- Bug causing entries to be recreated on every reconciliation (#32)

## [0.2.0] - 2022-06-01

### Added

- Ability to configure the SPIRE Server API socket path via the `spireServerSocketPath` value in the configuration file (#29)

### Updated

- Various documentation fixes (#18, #23, #26)

### Deprecated

- The `spire-api-socket` CLI flag is deprecated in favor of the `spireServerSocketPath` value in the configuration file (#29)

## [0.1.0] - 2022-05-16

First official release! The SPIRE controller manager supports:
- Registering workloads using the ClusterSPIFFEID custom resource
- Establishing federation relationships with foreign trust domains using the ClusterFederatedTrustDomain resource
- Full management of the Validating Admission Controller webhook credentials

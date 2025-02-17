# Changelog

## [0.6.1] - 2025-02-14

### Added

- Support for configuring the log level (#388, #464)
- New metrics to track `ClusterStaticEntry` failures (#387)

### Fixed

- Failed controller upgrade when webhook certificate is expired (#450)

### Updated

- Minor documentation changes (#435, #443)
- Version used in migration guide (#465)

## [0.6.0] - 2024-10-03

<font size='7'>:rotating_light: ***PLEASE READ BEFORE UPGRADING*** :rotating_light:</font>

This version contains changes in the `ClusterSPIFFEID` CRD. Before upgrading you __MUST__ do the following:

- Update the CRD in your cluster (see [here](./config/crd/bases/spire.spiffe.io_clusterspiffeids.yaml)).

### Added

- Hint field to the ClusterSPIFFEID CRD that controls the hint on resulting entries (#416)
- Fallback field to the ClusterSPIFFEID CRD which causes the CR to only apply if no other non-fallback CRs have been applied to a given pod (#415)
- Missing documentation for the className on the ClusterFederatedTrustDomain CRD (#413)

## [0.5.0] - 2024-04-10

<font size='7'>:rotating_light: ***PLEASE READ BEFORE UPGRADING*** :rotating_light:</font>

This version contains changes in the `ClusterStaticEntry` CRD. Before upgrading you __MUST__ do the following:

- Update the CRD in your cluster (see [here](.config/crd/bases/spire.spiffe.io_clusterstaticentries.yaml)).

### Added

- Support for `storeSVID` on ClusterStaticEntry (#304)
- Support for more than one spire-controller-manager managing entries against a single SPIRE server cluster via entry prefixes (#325)

## [0.4.4] - 2024-04-05

### Security

- Updated Golang to 1.21.9 to address CVE-2023-45288 (#338)

## [0.4.3] - 2024-02-22

### Added

- Ability to selectively choose which CRDs to reconcile (#297)

### Changed

- Join token novelty entries are ignored during entry reconciliation (#306)

## [0.4.2] - 2024-01-24

### Added

- Process-wide support for customizing the parent ID template for workload registration (#289)

### Fixed

- Failed controller startup when webhook was disabled via ENABLE_WEBHOOKS=false (#294)

## [0.4.1] - 2024-01-17

### Added

- Support for caching multiple namespaces instead of one or all (#271,#286)
- Support for expanding environment variables in the controller configuration (#256)
- Support for disabling webhooks by setting the environment variable ENABLE_WEBHOOKS=false (#234)

## [0.4.0] - 2023-11-02

<font size='7'>:rotating_light: ***PLEASE READ BEFORE UPGRADING*** :rotating_light:</font>

 This version contains changes in the `ClusterSPIFFEID` CRD, `ClusterFederatedTrustDomain` CRD and `ClusterStaticEntry` CRD. Before upgrading you __MUST__ do the following, in order:

- Update those CRDs into your cluster (see [here](./config/crd/bases/spire.spiffe.io_clusterspiffeids.yaml), [here](./config/crd/bases/spire.spiffe.io_clusterfederatedtrustdomains.yaml) and [here](.config/crd/bases/spire.spiffe.io_clusterstaticentries.yaml)).
- Update the `manager-role` ClusterRole, which includes additional permissions for `endpoints` CRD (see [here](./config/rbac/role.yaml))

### Security

- Updated to google.golang.org/grpc v1.59.0 to address CVE-2023-44487 (#231)

### Added

- ClusterSPIFFEID CRD support for DNS name auto-population (#122)
- Support for multiple SPIRE clusters running in the same K8S cluster using ClassName's (#230)

### Fixed

- Missing status subresource definitions (#223)

## [0.3.0] - 2023-09-14

<font size='7'>:rotating_light: ***PLEASE READ BEFORE UPGRADING*** :rotating_light:</font>

 This version contains changes in the `ClusterSPIFFEID` CRD. It also adds a new `ClusterStaticEntry` CRD. Before upgrading you __MUST__ do the following, in order:

- Update/install those CRDs into your cluster (see [here](./config/crd/bases/spire.spiffe.io_clusterstaticentries.yaml) and [here](./config/crd/bases/spire.spiffe.io_clusterspiffeids.yaml)).
- Update the `manager-role` ClusterRole, which includes additional permissions for the new `ClusterStaticEntry` CRD (see [here](./config/rbac/role.yaml))

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

# Changelog

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

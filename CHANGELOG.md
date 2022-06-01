# Changelog

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

# ClusterFederatedTrustDomain Custom Resource Definition

The ClusterFederatedTrustDomain Custom Resource Definition (CRD) is a
cluster-wide resource used to program SPIRE with a federation relationship with
a foreign trust domain.

The definition can be found [here](/api/v1alpha1/clusterfederatedtrustdomain_types.go).

See the [SPIFFE Federation](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Federation.md) specification for more information.

## Specification

| Field                   | Required | Example                                                 | Description                                                                                                             |
| ----------------------- | -------- | ------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `trustDomain`           | REQUIRED | `somedomain`                                            | The name of the foreign trust domain to federate with. Must be unique across all ClusterFederatedTrustDomain resources. |
| `bundleEndpointURL`     | REQUIRED | `https://somedomain.test/bundle`                        | An HTTPS URL to the bundle endpoint for the foreign trust domain.                                                       |
| `bundleEndpointProfile` | REQUIRED | See [Bundle Endpoint Profile](#bundle-endpoint-profile) | The profile for the bundle endpoint for the foreign trust domain.                                                       |
| `trustDomainBundle`     | OPTIONAL |                                                         | The bundle contents for the foreign trust domain.                                                                       |

### Bundle Endpoint Profile

| Field                   | Required    | Example                                                 | Description                                                                                                                                                                             |
| ----------------------- | ----------- | ------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `type`                  | REQUIRED    | `https_web`                                             | One of `https_web` or `https_spiffe` indicating the [endpoint profile](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Federation.md#52-endpoint-profiles) of the endpoint. |
| `endpointSPIFFEID`      | OPTIONAL[1] | `https://somedomain.test/bundle`                        | The SPIFFE ID of the bundle endpoint. Used to authenticate the endpoint in the `https_spiffe` profile                                                                                   |

[1] Required for the `https_spiffe` bundle endpoint profile

## Status

The ClusterFederatedTrustDomain does not have any status fields.

# ClusterFederatedTrustDomain Custom Resource Definition

The ClusterFederatedTrustDomain Custom Resource Definition (CRD) is a
cluster-wide resource used to program SPIRE with a federation relationship with
a foreign trust domain.

The definition can be found [here](../api/v1alpha1/clusterfederatedtrustdomain_types.go).

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

## Examples

1. Create a federation relationship with the "backend" trust domain using the [https_web](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Federation.md#521-web-pki-https_web) profile.

    ```
    apiVersion: spire.spiffe.io/v1alpha1
    kind: ClusterFederatedTrustDomain
    metadata:
      name: backend
    spec:
      trustDomain: backend
      bundleEndpointURL: https://backend.test/bundle
      bundleEndpointProfile:
        type: https_web
    ```

1. Create a federation relationship with the "backend" trust domain using the [https_spiffe](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Federation.md#522-spiffe-authentication-https_spiffe) profile, including the initial bundle contents to authenticate the endpoint:

    ```
    apiVersion: spire.spiffe.io/v1alpha1
    kind: ClusterFederatedTrustDomain
    metadata:
      name: backend
    spec:
      trustDomain: backend
      bundleEndpointURL: https://backend.teste/bundle
      bundleEndpointProfile:
        type: https_spiffe
        endpointSPIFFEID: spiffe://backend/bundle-endpoint-server
      trustDomainBundle: |-
        {
            "keys": [
                {
                    "use": "x509-svid",
                    "kty": "EC",
                    "crv": "P-256",
                    "x": "HW4nwENKhVNP8PZIPRE82qQk_MN_6tHHtofvuqhJQeY",
                    "y": "4_DOmvWD1vnvn4ZSQ9xVEcfiP2hNYCugPpczki9irT0",
                    "x5c": [
                        "MIIBnjCCAUWgAwIBAgIQaSWSbPs2wDfYCSRvKPTQOTAKBggqhkjOPQQDAjAeMQswCQYDVQQGEwJVUzEPMA0GA1UEChMGU1BJRkZFMB4XDTIyMDUxNjE3MTc0N1oXDTIyMDUxNzE3MTc1N1owHjELMAkGA1UEBhMCVVMxDzANBgNVBAoTBlNQSUZGRTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABB1uJ8BDSoVTT/D2SD0RPNqkJPzDf+rRx7aH77qoSUHm4/DOmvWD1vnvn4ZSQ9xVEcfiP2hNYCugPpczki9irT2jZTBjMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBQGf0uNK8ZEEBkU9S5HLzhAVaI35jAhBgNVHREEGjAYhhZzcGlmZmU6Ly9jbHVzdGVyMS5kZW1vMAoGCCqGSM49BAMCA0cAMEQCIF+Z71be5RzIoO/Ys3mKiJBaEXEyPY2ZerDrv3aukMxdAiBTB/TS9CDXz+J40e8/3AdVynFIvUbnzr77XgFLlySR/g=="
                    ]
                },
                {
                    "use": "jwt-svid",
                    "kty": "EC",
                    "kid": "BSZV4bb6MLNky0Xhk402GChJVBvSYzZq",
                    "crv": "P-256",
                    "x": "yQ0A7fv5hevYrI82tLp6j7GM5llU2-okfUWHUenrFI8",
                    "y": "76LcJhFJn6ACO8mNE42fuGuYd-WmujVM93AhoJf16ME"
                }
            ]
        }
    ```

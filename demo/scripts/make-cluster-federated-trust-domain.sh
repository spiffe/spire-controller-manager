#!/bin/bash

set -eo pipefail

DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

endpointAddr=$($DIR/get_service_ip_port.sh spire-system spire-server-bundle-endpoint)
if [ -z "${endpointAddr}" ]; then
    echo "Endpoint service not ready" 2>&1
    exit 1
fi

bundleContents=$(kubectl exec \
    -n spire-system \
    -c spire-server \
    deployment/spire-server -- \
    /opt/spire/bin/spire-server bundle show --format=spiffe) \
trustDomain="${KIND_CLUSTER_NAME}.demo" \
resourceName="${KIND_CLUSTER_NAME}" \
bundleEndpointURL="https://${endpointAddr}" \
endpointSPIFFEID="spiffe://${KIND_CLUSTER_NAME}.demo/spire/server" \
    yq eval -n '{
    "apiVersion": "spire.spiffe.io/v1alpha1",
    "kind": "ClusterFederatedTrustDomain",
    "metadata": {
        "name": strenv(resourceName)
    },
    "spec": {
        "trustDomain": strenv(trustDomain),
        "bundleEndpointURL": strenv(bundleEndpointURL),
        "bundleEndpointProfile": {
            "type": "https_spiffe",
            "endpointSPIFFEID": strenv(endpointSPIFFEID)
        },
        "trustDomainBundle": strenv(bundleContents)
    }
}'

#!/bin/bash

set -eo pipefail

serverIP=$(kubectl -nspire-system get services/spire-server-bundle-endpoint -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
if [ -z "${serverIP}" ]; then
    echo "Server load balancer is not ready" 2>&1
    exit 1
fi

bundleContents=$(kubectl exec \
    -n spire-system \
    -c spire-server \
    deployment/spire-server -- \
    /opt/spire/bin/spire-server bundle show --format=spiffe) \
trustDomain="${KIND_CLUSTER_NAME}.demo" \
resourceName="${KIND_CLUSTER_NAME}" \
bundleEndpointURL="https://${serverIP}:8443" \
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

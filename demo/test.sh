#!/bin/bash

set -e -o pipefail

norm=$(tput sgr0) || true
red=$(tput setaf 1) || true
green=$(tput setaf 2) || true
yellow=$(tput setaf 3) || true
bold=$(tput bold) || true

timestamp() {
    date -u "+[%Y-%m-%dT%H:%M:%SZ]"
}

log-info() {
    echo "${bold}$(timestamp) $*${norm}"
}

log-good() {
    echo "${green}$(timestamp) $*${norm}"
}

fail-now() {
    echo "${red}$(timestamp) $*${norm}" 2>&1
    exit 1
}


DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

cd "$DIR"

cleanup() {
    echo "Cleaning up..."
    ./cluster1 kind delete cluster || true
    ./cluster2 kind delete cluster || true
    echo "Done."
}

trap cleanup EXIT

log-info "Tagging devel image as nightly..."
docker tag ghcr.io/spiffe/spire-controller-manager:{devel,nightly}

log-info "Building greeter server/client..."
(cd greeter; make docker-build)

log-info "Pulling docker images..."
echo ghcr.io/spiffe/spire-server:1.2.3 \
    ghcr.io/spiffe/spire-agent:1.2.3 \
    ghcr.io/spiffe/spiffe-csi-driver:0.2.0 \
    | xargs -n1 docker pull

log-info "Creating cluster1..."
./cluster1 kind create cluster --config=config/cluster1/config.yaml

log-info "Creating cluster2..."
./cluster2 kind create cluster --config=config/cluster2/config.yaml

log-info "Loading images into cluster1..."
echo \
    ghcr.io/spiffe/spire-server:1.2.3 \
    ghcr.io/spiffe/spire-agent:1.2.3 \
    ghcr.io/spiffe/spiffe-csi-driver:0.2.0 \
    ghcr.io/spiffe/spire-controller-manager:nightly \
    greeter-server:demo \
    | xargs -n1 ./cluster1 kind load docker-image

log-info "Loading images into cluster2..."
echo \
    ghcr.io/spiffe/spire-server:1.2.3 \
    ghcr.io/spiffe/spire-agent:1.2.3 \
    ghcr.io/spiffe/spiffe-csi-driver:0.2.0 \
    ghcr.io/spiffe/spire-controller-manager:nightly \
    greeter-client:demo \
    | xargs -n1 ./cluster2 kind load docker-image

############################################################################
# Deploy SPIRE and pals
############################################################################
log-info "Applying cluster1 SPIRE config..."
./cluster1 kubectl apply -k config/cluster1

log-info "Applying cluster2 SPIRE config..."
./cluster2 kubectl apply -k config/cluster2

log-info "Waiting for SPIRE server and spire-controller-manager to deploy in cluster1..."
./cluster1 kubectl rollout status -w --timeout=30s -nspire-system deployment/spire-server

log-info "Waiting for SPIRE server and spire-controller-manager to deploy in cluster2..."
./cluster2 kubectl rollout status -w --timeout=30s -nspire-system deployment/spire-server

############################################################################
# Deploy the greeter server and client
############################################################################

log-info "Applying greeter-server config in cluster1..."
./cluster1 kubectl apply -k config/cluster1/greeter-server

log-info "Waiting for the greeter server to deploy in cluster1..."
./cluster1 kubectl rollout status -w --timeout=30s deployment/greeter-server

GREETER_SERVER_ADDR=$(./cluster1 ./scripts/get_service_ip_port.sh default greeter-server)

./cluster2 kubectl apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: greeter-client-config
data:
  greeter-server-addr: "$GREETER_SERVER_ADDR"
EOF

log-info "Applying greeter-client config in cluster2..."
./cluster2 kubectl apply -k config/cluster2/greeter-client

log-info "Waiting for the greeter server to deploy in cluster1..."
./cluster2 kubectl rollout status -w --timeout=30s deployment/greeter-client

############################################################################
# Federate the clusters
############################################################################

log-info "Federating cluster1 with cluster2..."
./cluster1 scripts/make-cluster-federated-trust-domain.sh | \
    ./cluster2 kubectl apply -f -

log-info "Federating cluster2 with cluster1..."
./cluster2 scripts/make-cluster-federated-trust-domain.sh | \
    ./cluster1 kubectl apply -f -

############################################################################
# Configure the greeter server/client IDs
############################################################################

log-info "Configuring the greeter server ID in cluster1..."
./cluster1 kubectl apply -f config/greeter-server-id.yaml

log-info "Configuring the greeter client ID in cluster2..."
./cluster2 kubectl apply -f config/greeter-client-id.yaml

############################################################################
# Check status
############################################################################


log-info "Checking greeter server logs for success..."
SUCCESS=
for ((i = 0; i < 30; i++)); do
    if ./cluster1 kubectl logs deployment/greeter-server | grep -q spiffe://cluster2.demo/greeter-client; then
        log-info "Server received request from client!"
        SUCCESS=true
        break
    fi
    sleep 1
done
if [ -z "$SUCCESS" ]; then
    fail-now "Server never received request from client :("
fi

log-info "Checking greeter client logs for success..."
SUCCESS=
for ((i = 0; i < 30; i++)); do
    if ./cluster2 kubectl logs deployment/greeter-client | grep -q spiffe://cluster1.demo/greeter-server; then
        log-info "Client received response from server!"
        SUCCESS=true
        break
    fi
    sleep 1
done
if [ -z "$SUCCESS" ]; then
    fail-now "Client never received response from server :("
fi

log-good "Success."

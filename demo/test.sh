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
    if [[ "$1" -ne 0 ]]; then
      cat <<EOF >>"$GITHUB_STEP_SUMMARY"
### Describe Pods Cluster 1
\`\`\`
$(./cluster1 kubectl describe pods -n "spire-system")
\`\`\`

### Logs Cluster 1

\`\`\`
$(./cluster1 kubectl get pods -o name -n "spire-system" | while read -r line; do echo; echo "logs for ${line}:"; ./cluster1 kubectl logs -n "spire-system" "${line}" --prefix --all-containers=true --ignore-errors=true; done)
\`\`\`

### Describe Pods Cluster 2

\`\`\`
$(./cluster2 kubectl describe pods -n "spire-system")
\`\`\`

### Logs Cluster 2

\`\`\`
$(./cluster2 kubectl get pods -o name -n "spire-system" | while read -r line; do echo; echo logs for "${line}:"; ./cluster2 kubectl logs -n "spire-system" "${line}" --prefix --all-containers=true --ignore-errors=true; done)
\`\`\`
EOF
    fi

    echo "Cleaning up..."
    ./cluster1 kind delete cluster || true
    ./cluster2 kind delete cluster || true
    echo "Done."
}

trap 'EC=$? && trap - SIGTERM && cleanup $EC' SIGINT SIGTERM EXIT

log-info "Tagging devel image as nightly..."
docker tag ghcr.io/spiffe/spire-controller-manager:{devel,nightly}

log-info "Building greeter server/client..."
(cd greeter; make docker-build)

log-info "Pulling docker images..."
echo ghcr.io/spiffe/spire-server:1.10.4 \
    ghcr.io/spiffe/spire-agent:1.10.4 \
    ghcr.io/spiffe/spiffe-csi-driver:0.2.6 \
    registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.12.0 \
    | xargs -n1 docker pull

log-info "Creating cluster1..."
./cluster1 kind create cluster --config=config/cluster1/config.yaml

log-info "Creating cluster2..."
./cluster2 kind create cluster --config=config/cluster2/config.yaml

log-info "Loading images into cluster1..."
echo \
    ghcr.io/spiffe/spire-server:1.10.4 \
    ghcr.io/spiffe/spire-agent:1.10.4 \
    ghcr.io/spiffe/spiffe-csi-driver:0.2.6 \
    registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.12.0 \
    ghcr.io/spiffe/spire-controller-manager:nightly \
    greeter-server:demo \
    | xargs -n1 ./cluster1 kind load docker-image

log-info "Loading images into cluster2..."
echo \
    ghcr.io/spiffe/spire-server:1.10.4 \
    ghcr.io/spiffe/spire-agent:1.10.4 \
    ghcr.io/spiffe/spiffe-csi-driver:0.2.6 \
    registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.12.0 \
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
# Add a static entry
############################################################################

log-info "Configuring the static entry in cluster1..."
./cluster1 kubectl apply -f config/static-entry.yaml

############################################################################
# Check status
############################################################################


# End-to-end success here is the sum of several coarse-grained delays: the
# controller's 10s reconcile interval, the federated bundle exchange the
# greeter entry depends on, agent SVID propagation, and the greeter client's
# 10s request interval. Observed successes range from ~12s to ~21s, so a 30s
# budget leaves too little headroom and fails intermittently on busy runners.
log-info "Checking greeter server logs for success..."
SUCCESS=
for ((i = 0; i < 90; i++)); do
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
for ((i = 0; i < 90; i++)); do
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

log-info "Checking for the static entry..."
SUCCESS=
for ((i = 0; i < 90; i++)); do
    if ./cluster1 scripts/show-spire-entries.sh | grep -q static-spiffe-id; then
        log-info "Static entry created in cluster1"
        SUCCESS=true
        break
    fi
    sleep 1
done
if [ -z "$SUCCESS" ]; then
    fail-now "Static entry never created :("
fi

############################################################################
# ClusterSPIFFEID informer cache label filtering
#
# Verifies filterByClassName: the controller manager restricts its
# ClusterSPIFFEID informer cache to objects labeled with its own className.
#
# Two ClusterSPIFFEIDs are applied that are identical except for that label --
# same className, same podSelector, both targeting the real greeter-server pod.
# Only the labeled one may produce a registration entry. Because both set a
# className matching the controller, the pre-existing className reconcile
# filter would admit both; the cache label selector is the only thing that can
# exclude the unlabeled one.
############################################################################

log-info "Enabling ClusterSPIFFEID cache filtering in cluster1..."

# The existing greeter-server ClusterSPIFFEID predates the filter and carries no
# class-name label, so it would fall out of the cache and have its entry deleted.
# Label it so the entry survives once filtering is on.
./cluster1 kubectl label clusterspiffeid greeter-server spire.spiffe.io/class-name=demo-class --overwrite

./cluster1 kubectl create configmap spire-controller-manager-config \
    -n spire-system \
    --from-file=spire-controller-manager-config.yaml=config/cluster1/spire/spire-controller-manager-config-cache-filter.yaml \
    --dry-run=client -o yaml | ./cluster1 kubectl apply -f -

log-info "Restarting spire-server in cluster1 to pick up the new config..."
./cluster1 kubectl rollout restart -nspire-system deployment/spire-server
./cluster1 kubectl rollout status -w --timeout=90s -nspire-system deployment/spire-server

# The spire-server pod also hosts the ClusterSPIFFEID validating webhook, which
# has failurePolicy: Fail and mints its serving certificate at startup. A
# finished rollout does not mean the webhook is already answering, so applies
# issued immediately after the restart can be rejected with
# "failed calling webhook ...: context deadline exceeded". Retry until the
# webhook is actually serving.
log-info "Waiting for the ClusterSPIFFEID validating webhook to serve again..."
SUCCESS=
for ((i = 0; i < 60; i++)); do
    if ./cluster1 kubectl apply -f config/cache-filter-cached-id.yaml >/dev/null 2>&1; then
        log-info "Webhook is serving; labeled ClusterSPIFFEID applied"
        SUCCESS=true
        break
    fi
    sleep 2
done
if [ -z "$SUCCESS" ]; then
    fail-now "Validating webhook never became available after the restart :("
fi

# The webhook answered above, so this normally succeeds first try; retry anyway
# so a transient webhook blip fails with a clear message instead of aborting the
# script mid-apply.
log-info "Applying the unlabeled (uncached) ClusterSPIFFEID..."
SUCCESS=
for ((i = 0; i < 30; i++)); do
    if ./cluster1 kubectl apply -f config/cache-filter-uncached-id.yaml >/dev/null 2>&1; then
        SUCCESS=true
        break
    fi
    sleep 2
done
if [ -z "$SUCCESS" ]; then
    fail-now "Could not apply the unlabeled ClusterSPIFFEID :("
fi

log-info "Checking that the labeled ClusterSPIFFEID produced an entry..."
SUCCESS=
for ((i = 0; i < 60; i++)); do
    if ./cluster1 scripts/show-spire-entries.sh | grep -q cache-filter-cached; then
        log-info "Entry for the labeled ClusterSPIFFEID was created"
        SUCCESS=true
        break
    fi
    sleep 1
done
if [ -z "$SUCCESS" ]; then
    fail-now "Labeled ClusterSPIFFEID never produced an entry :("
fi

# The labeled entry existing proves a full reconcile pass has completed, so the
# unlabeled object has had at least as long to be (incorrectly) reconciled.
# Allow a further settle window to reduce the chance of a passing race: with the
# filter wiring removed the unlabeled entry shows up within ~10s, so 30s leaves
# ample margin on slower CI runners.
log-info "Checking that the unlabeled ClusterSPIFFEID produced NO entry..."
for ((i = 0; i < 30; i++)); do
    if ./cluster1 scripts/show-spire-entries.sh | grep -q cache-filter-uncached; then
        fail-now "Unlabeled ClusterSPIFFEID was reconciled despite the cache filter :("
    fi
    sleep 1
done
log-info "Unlabeled ClusterSPIFFEID was correctly excluded from the cache"

# Confirm the filter did not disturb the pre-existing demo resources.
log-info "Checking that the greeter-server and static entries still exist..."
./cluster1 scripts/show-spire-entries.sh | grep -q "spiffe://cluster1.demo/greeter-server" \
    || fail-now "greeter-server entry disappeared after enabling the cache filter :("
./cluster1 scripts/show-spire-entries.sh | grep -q static-spiffe-id \
    || fail-now "static entry disappeared after enabling the cache filter :("

log-good "Success."

# Demo

## Introduction

This demo will guide you through creating two clusters. Each cluster will
contain a SPIRE deployment along with the SPIRE Controller Manager. The first
cluster (cluster1) will host a greeter server workload. The second cluster
(cluster2) will host a greeter client workload. Each cluster is a distinct
SPIFFE trust domain.

The greeter server and greeter client communicate with each other over TLS and
perform mutual authentication.

To facilitate this cross-cluster authentication, each cluster will be federated
with the other by deploying a ClusterFederatedTrustDomain CRD in each cluster
that directs that cluster to the SPIFFE Bundle Endpoint for the other cluster.

Additionally, each workload will obtain an X509-SVID from SPIRE. The workload
registration will be accomplished by deploying a ClusterSPIFFEID CRD in each
cluster that targets the greeter workloads and assigns them the proper ID.

## Prerequisites

- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [Kubectl](https://kubernetes.io/docs/tasks/tools/)
- [yq](https://github.com/mikefarah/yq)

## Steps

Build the greeter server and client:

    $ (cd greeter; make docker-build)

Pull the requisite images:

    $ echo ghcr.io/spiffe/spire-server:1.2.3 \
        ghcr.io/spiffe/spire-agent:1.2.3 \
        ghcr.io/spiffe/spiffe-csi-driver:0.2.0 \
        ghcr.io/spiffe/spire-controller-manager:nightly \
        | xargs -n1 docker pull

Start up cluster1 and load the requisite images:

    $ ./cluster1 kind create cluster
    $ echo \
        ghcr.io/spiffe/spire-server:1.2.3 \
        ghcr.io/spiffe/spire-agent:1.2.3 \
        ghcr.io/spiffe/spiffe-csi-driver:0.2.0 \
        ghcr.io/spiffe/spire-controller-manager:nightly \
        greeter-server:demo \
        | xargs -n1 ./cluster1 kind load docker-image

Start up cluster 2 and load the requisite images:

    $ ./cluster2 kind create cluster
    $ echo \
        ghcr.io/spiffe/spire-server:1.2.3 \
        ghcr.io/spiffe/spire-agent:1.2.3 \
        ghcr.io/spiffe/spiffe-csi-driver:nightly \
        ghcr.io/spiffe/spire-controller-manager:nightly \
        greeter-client:demo \
        | xargs -n1 ./cluster2 kind load docker-image


Deploy SPIRE components in cluster1:

    $ ./cluster1 kubectl apply -k config/cluster1

Deploy SPIRE components in cluster2:

    $ ./cluster2 kubectl apply -k config/cluster2

Federate cluster1 with cluster2:

    $ ./cluster1 scripts/make-cluster-federated-trust-domain.sh | \
        ./cluster2 kubectl apply -f -

Federate cluster2 with cluster1:

    $ ./cluster2 scripts/make-cluster-federated-trust-domain.sh | \
        ./cluster1 kubectl apply -f -

Deploy the greeter server in cluster1:

    $ ./cluster1 kubectl apply -k config/cluster1/greeter-server

Configure the greeter client with the address of the server:

    $ ./cluster2 kubectl apply -f - <<EOF
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: greeter-client-config
    data:
      greeter-server-addr: "$(./cluster1 ./scripts/get_service_ip_port.sh default greeter-server)"
    EOF

Deploy the greeter client in cluster2:

    $ ./cluster2 kubectl apply -k config/cluster2/greeter-client

Create the ClusterSPIFFEID for the greeter server in cluster1:

    $ ./cluster1 kubectl apply -f config/greeter-server-id.yaml

Create the ClusterSPIFFEID for the greeter client in cluster2:

    $ ./cluster2 kubectl apply -f config/greeter-client-id.yaml

Check the greeter server logs to see that it has received authenticated
requests from the greeter client:

    $ ./cluster1 kubectl logs deployment/greeter-server

Check the greeter client logs to see that it as able to authenticate
the greeter server and issue the request and receive the response:

    $ ./cluster2 kubectl logs deployment/greeter-client

List the SPIRE registration entries and federated trust domain relationships that were created by the controller:

    $ ./cluster1 scripts/show-spire-entries.sh
    $ ./cluster1 scripts/show-spire-federated-bundles.sh
    $ ./cluster2 scripts/show-spire-entries.sh
    $ ./cluster2 scripts/show-spire-federated-bundles.sh

When you are finished, delete the clusters:

    $ ./cluster1 kind delete cluster
    $ ./cluster2 kind delete cluster

# Demo

Build the greeter server and client:

    $ (cd greeter; make docker-build)

Start up cluster1 and load the requisite images:

    $ ./cluster1 kind create cluster
    $ ./cluster1 kind load docker-image \
        ghcr.io/spiffe/spire-server:1.1.0 \
        ghcr.io/spiffe/spire-agent:1.1.0 \
        ghcr.io/spiffe/spiffe-csi-driver:nightly \
        ghcr.io/spiffe/spire-controller-manager:nightly \
        greeter-server:demo

Start up cluster 2 and load the requisite images:

    $ ./cluster2 kind create cluster
    $ ./cluster2 kind load docker-image \
        ghcr.io/spiffe/spire-server:1.1.0 \
        ghcr.io/spiffe/spire-agent:1.1.0 \
        ghcr.io/spiffe/spiffe-csi-driver:nightly \
        ghcr.io/spiffe/spire-controller-manager:nightly \
        greeter-client:demo

Deploy SPIRE components and greeter server in cluster1:

    $ ./cluster1 kubectl apply -k config/cluster1

Deploy SPIRE components and greeter client in cluster2:

    $ ./cluster2 kubectl apply -k config/cluster2

Federate cluster1 with cluster2:

    $ ./cluster1 scripts/make-cluster-federated-trust-domain.sh | \
        ./cluster2 kubectl apply -f -

Federate cluster2 with cluster1:

    $ ./cluster2 scripts/make-cluster-federated-trust-domain.sh | \
        ./cluster1 kubectl apply -f -

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

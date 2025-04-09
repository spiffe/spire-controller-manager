#!/bin/bash

set -eo pipefail

kubectl exec -t \
    -n spire-system \
    -c spire-server deployment/spire-server -- \
        /opt/spire/bin/spire-server bundle list -format spiffe

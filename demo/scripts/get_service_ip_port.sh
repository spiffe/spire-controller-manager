#!/bin/bash

set -e -o pipefail

NAMESPACE="$1"
NAME="$2"

NODEPORT=$(kubectl get "service/$NAME" "-n$NAMESPACE" -o jsonpath='{.spec.ports[0].nodePort}')
NODEIP=$(kubectl get nodes -o jsonpath='{.items[*].status.addresses[?(@.type=="InternalIP")].address}')

echo "$NODEIP:$NODEPORT"

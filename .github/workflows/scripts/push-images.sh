#!/bin/bash

set -e

IMAGETAG="$1"
if [ -z "$IMAGETAG" ]; then
    echo "IMAGETAG not provided!" 1>&2
    echo "Usage: push-image.sh IMAGETAG" 1>&2
    exit 1
fi

echo "Pushing image tagged as $IMAGETAG..."

LOCALIMG=ghcr.io/spiffe/spire-controller-manager:devel
REMOTEIMG=ghcr.io/spiffe/spire-controller-manager:"${IMAGETAG}"

echo "Executing: docker tag $LOCALIMG $REMOTEIMG"
docker tag "$LOCALIMG" "$REMOTEIMG"
echo "Executing: docker push $REMOTEIMG"
docker push "$REMOTEIMG"

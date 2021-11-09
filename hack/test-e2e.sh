#!/bin/sh -eu

NAMESPACE=${NAMESPACE:-e2e-test}

REGISTRY_ENDPOINT="$(oc get quayintegrations -o jsonpath='{.items[*].spec.quayHostname}')"
REGISTRY="${REGISTRY_ENDPOINT#*://}"

oc new-project "$NAMESPACE"

# Wait until QBO creates a new project and secrets
sleep 10
oc -n "$NAMESPACE" get secrets builder-quay-openshift deployer-quay-openshift default-quay-openshift

# Create a new build and check its results
oc -n "$NAMESPACE" new-app --template=httpd-example
i=0
while [ $i -lt 120 ]; do
    phase=$(oc -n "$NAMESPACE" get build httpd-example-1 -o jsonpath='{.status.phase}' || true)
    if [ "$phase" != "" ] && [ "$phase" != "New" ] && [ "$phase" != "Pending" ] && [ "$phase" != "Running" ]; then
        break
    fi
    sleep 1
    i=$((i+1))
done
if [ "$phase" != "Complete" ]; then
    echo "ERROR: The build httpd-example-1 is $phase, but expected to be Complete" >&2
    set -x
    oc -n "$NAMESPACE" get builds
    oc -n "$NAMESPACE" get pods
    oc -n "$NAMESPACE" logs httpd-example-1-build
    exit 1
fi

IMAGE_REF=$(oc -n "$NAMESPACE" get is httpd-example -o jsonpath='{.status.tags[0].items[0].dockerImageReference}')
case "$IMAGE_REF" in
"$REGISTRY/"*)
    ;;
*)
    echo "ERROR: The image $IMAGE_REF is expected to be in the registry $REGISTRY" >&2
    set -x
    oc -n "$NAMESPACE" get builds
    oc -n "$NAMESPACE" get is httpd-example -o yaml
    exit 1
    ;;
esac

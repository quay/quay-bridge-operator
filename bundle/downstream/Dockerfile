FROM scratch

LABEL com.redhat.delivery.operator.bundle=true
LABEL com.redhat.delivery.openshift.ocp.versions="v4.7-v4.9"
LABEL com.redhat.openshift.versions="v4.7-v4.9"
LABEL com.redhat.delivery.backport=true

LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=quay-bridge-operator
LABEL operators.operatorframework.io.bundle.channels.v1=stable-3.6
LABEL operators.operatorframework.io.bundle.channel.default.v1=stable-3.6

LABEL com.redhat.component="quay-bridge-operator-bundle-container" \
      io.k8s.display-name="Red Hat Quay OpenShift Bridge Operator" \
      name="quay/quay-bridge-operator-bundle" \
      summary="Quay OpenShift Bridge Operator bundle container image" \
      description="Operator bundle for Quay OpenShift Bridge Operator" \
      maintainer="Red Hat <support@redhat.com>" \
      version="v3.6.0"

COPY manifests/*.yaml /manifests/
COPY metadata/annotations.yaml /metadata/annotations.yaml

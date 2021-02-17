FROM golang:1.12.5 as builder

WORKDIR /workspace
COPY Makefile Makefile
COPY scripts/ scripts/
COPY go.mod go.mod
COPY go.sum go.sum
COPY version/ version/
COPY tools.go tools.go
COPY vendor/ vendor/
COPY cmd/ cmd/
COPY pkg/ pkg/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build

FROM registry.access.redhat.com/ubi7/ubi-minimal:latest

ENV OPERATOR=/bin/quay-bridge-operator \
    USER_UID=1001 \
    USER_NAME=quay-bridge-operator

WORKDIR /
COPY --from=builder /workspace/build/_output/bin/quay-bridge-operator /bin/quay-bridge-operator
COPY build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

USER ${USER_UID}

ENTRYPOINT ["/usr/local/bin/entrypoint"]

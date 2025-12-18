# Development

## Building

```bash
# Build manager binary
make build

# Run controller locally (requires kubeconfig)
make run

# Build Docker image
make docker-build IMG=quay.io/quay/quay-bridge-operator:latest

# Push Docker image
make docker-push IMG=quay.io/quay/quay-bridge-operator:latest
```

## Code Generation

```bash
# Generate CRDs, RBAC, and webhook manifests
make manifests

# Generate DeepCopy methods and mocks
make generate

# Install mockgen tool
make mockgen
```

## Testing

```bash
# Run all tests with coverage
make test

# Run specific test file
go test ./pkg/client/quay/... -v

# Run specific test function
go test ./pkg/client/quay/... -v -run TestClientGetOrganization

# Run e2e tests (requires operator running on OpenShift)
make test-e2e
```

## Code Quality

```bash
# Format code
make fmt

# Run go vet
make vet
```

## Deployment

```bash
# Install CRDs to cluster
make install

# Uninstall CRDs
make uninstall

# Deploy operator
make deploy IMG=quay.io/quay/quay-bridge-operator:latest

# Undeploy operator
make undeploy

# Generate OLM bundle
make bundle
```

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `DISABLE_WEBHOOK` | Set to disable mutating webhook |
| `WEBHOOK_CERT_DIR` | Override webhook certificate directory |

## Webhook Setup

The webhook requires TLS certificates. In development:
1. Deploy cert-manager via OLM
2. Create a `Certificate` resource
3. Certificates default to `/apiserver.local.config/certificates/`

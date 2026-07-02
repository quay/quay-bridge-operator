# Contributing to quay-bridge-operator

## Setup

```bash
# Requires OpenShift cluster
make install
make deploy
```

## Development

Integrates Quay with OpenShift:
- Syncs Quay repos → ImageStreams
- Triggers builds from ImageStream changes
- Secrets synchronization

## Testing

```bash
# Unit tests
make test

# E2E (requires OpenShift + Quay)
make test-e2e
```

## Pull Requests

- Test on OpenShift 4.x
- Verify ImageStream sync doesn't create loops
- Test secret rotation scenarios
- Update bundle/ for OLM changes

## Code Structure

- `api/v1alpha1/` - CRD types
- `controllers/` - reconciliation logic
- `config/` - deployment manifests
- `bundle/` - OLM metadata

# Architecture

## Custom Resource

The operator manages a single cluster-scoped CRD:

**QuayIntegration** (`api/v1/quayintegration_types.go`): Configures connection to Quay registry.

Key fields:
- `clusterID`: Unique identifier for this cluster (used in organization naming)
- `quayHostname`: Full URL to Quay registry
- `credentialsSecret`: Reference to secret containing OAuth token
- `insecureRegistry`: Skip TLS verification
- `scheduledImageStreamImport`: Enable scheduled imports
- `allowlistNamespaces` / `denylistNamespaces`: Namespace filtering

## Controllers

Three reconcilers in `controllers/`:

### QuayIntegrationReconciler
- File: `quayintegration_controller.go`
- Watches: `QuayIntegration` CR
- Purpose: Validates configuration changes

### NamespaceIntegrationReconciler
- File: `namespace_controller.go`
- Watches: `Namespace`, `ImageStream`
- Purpose: Main integration logic
  - Creates Quay organizations for allowed namespaces
  - Creates robot accounts with role-based permissions
  - Generates Docker config secrets
  - Attaches secrets to service accounts
  - Uses finalizer to clean up Quay organizations on namespace deletion

### BuildIntegrationReconciler
- File: `build_controller.go`
- Watches: `Build` (completed builds with operator annotations)
- Purpose: Imports pushed images back into OpenShift ImageStreams

## Mutating Webhook

File: `pkg/webhook/webhook.go`

Intercepts Build creation/updates:
1. Rewrites output from `ImageStreamTag` to `DockerImage` pointing at Quay
2. Adds tracking annotations for BuildIntegrationReconciler
3. Validates builder service account has required secrets

## Service Account Permission Matrix

OpenShift SA -> Quay Robot Role:
- `builder` -> write (push images)
- `default` -> read (pull images)
- `deployer` -> read (pull images)

## Key Packages

| Package | Purpose |
|---------|---------|
| `pkg/client/quay/` | HTTP client for Quay REST API |
| `pkg/core/` | Shared controller utilities, error handling |
| `pkg/credentials/` | Docker config JSON secret generation |
| `pkg/constants/` | Annotation keys, env vars, defaults |
| `pkg/utils/` | Helpers for secret names, namespace validation |

## Namespace Filtering

Default denied namespaces (hardcoded):
- `default`, `openshift`, `management-infra`
- Any namespace starting with `openshift-` or `kube-`

Override via CR:
- `allowlistNamespaces`: Explicitly include (overrides default deny)
- `denylistNamespaces`: Explicitly exclude

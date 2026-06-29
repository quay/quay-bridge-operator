# Quay Bridge Operator - Agent Context

## Project Overview
Kubernetes operator integrating Red Hat Quay container registry with OpenShift.
Syncs namespaces as Quay organizations, manages robot accounts, rewrites builds to push to Quay.

## Tech Stack
- **Language**: Go 1.21+
- **Framework**: Kubebuilder v3, controller-runtime
- **APIs**: OpenShift Build/Image APIs, Quay REST API
- **Infra**: Kubernetes, cert-manager (for webhook TLS)

## Core Workflow
- **Build**: `make build`
- **Test**: `make test`
- **Lint**: `make fmt && make vet`
- **Run locally**: `make run`
- **Deploy**: `make deploy IMG=<image>`

## Documentation Map

Read the specific documentation below if your task involves these keywords:

- **Controllers, CRD, Webhook, Reconciler, Finalizer** -> `read file agent_docs/architecture.md`
  - **CRITICAL**: Understand the three-controller design before modifying reconciliation logic.

- **Build, Test, Deploy, Run, Make, Docker** -> `read file agent_docs/development.md`
  - **CRITICAL**: Use `make test` for full test suite with envtest setup.

- **Quay, API, Organization, Robot, Repository, Prototype** -> `read file agent_docs/quay-api.md`
  - **CRITICAL**: All Quay client methods return (result, *http.Response, error) tuple.

- **Commit, PR, Jira, Contribution** -> `read file agent_docs/contributing.md`
  - **CRITICAL**: All changes require a PROJQUAY Jira reference.

## Key Files

| File | Purpose |
|------|---------|
| `api/v1/quayintegration_types.go` | QuayIntegration CRD definition |
| `controllers/namespace_controller.go` | Main integration logic |
| `controllers/build_controller.go` | Post-build image import |
| `pkg/webhook/webhook.go` | Build mutation webhook |
| `pkg/client/quay/client.go` | Quay API client |
| `config/samples/quay_v1_quayintegration.yaml` | Example CR |

## Universal Conventions
- **Style**: Follow existing Go idioms, use controller-runtime patterns
- **Testing**: Mocks in `pkg/client/quay/mocks/`, use envtest for controller tests
- **Safety**: Never commit secrets; webhook requires TLS certificates

## Contextification Addendum

Low-token routing:

- API type: `api/v1/`
- Controllers: `controllers/`
- Quay client: `pkg/client/quay/`
- Webhook: `pkg/webhook/`
- Samples/manifests: `config/`
- E2E helper: `hack/test-e2e.sh`

Commands: `make build`, `make test`, `make fmt && make vet`, `make run`, `make deploy IMG=<image>`, `make test-e2e`.

Guardrails: keep `QuayIntegration` compatible, never commit OAuth tokens or kubeconfigs, and preserve webhook TLS behavior.

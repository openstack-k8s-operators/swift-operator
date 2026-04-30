# AGENTS.md - swift-operator

## Project overview

swift-operator is a Kubernetes operator that manages
[OpenStack Swift](https://docs.openstack.org/swift/latest/) (the object
storage service: distributed, eventually-consistent storage for unstructured
data accessed via HTTP) on OpenShift/Kubernetes. It is part of the
[openstack-k8s-operators](https://github.com/openstack-k8s-operators) project.

Key Swift domain concepts: **rings** (consistent-hash mappings for account,
container, and object data), **proxy** (HTTP front-end handling auth and
routing), **storage nodes** (account, container, object servers), **replication**,
**object expirer**, **dispersion** (cluster health checks).

Go module: `github.com/openstack-k8s-operators/swift-operator`
API group: `swift.openstack.org`
API version: `v1beta1`

## Tech stack

| Layer | Technology |
|-------|------------|
| Language | Go (modules, multi-module workspace via `go.work`) |
| Scaffolding | [Kubebuilder v4](https://book.kubebuilder.io/) + [Operator SDK](https://sdk.operatorframework.io/) |
| CRD generation | controller-gen (DeepCopy, CRDs, RBAC, webhooks) |
| Config management | Kustomize |
| Packaging | OLM bundle |
| Testing | Ginkgo/Gomega + envtest (functional), KUTTL (integration) |
| Linting | golangci-lint (`.golangci.yaml`) |
| CI | Zuul (`zuul.d/`), Prow (`.ci-operator.yaml`), GitHub Actions |

## Custom Resources

| Kind | Purpose |
|------|---------|
| `Swift` | Top-level CR. Spawns `SwiftRing`, `SwiftStorage`, and `SwiftProxy` sub-CRs. |
| `SwiftProxy` | Manages the proxy-server deployment (httpd front-end, auth pipeline). Has defaulting and validation webhooks. |
| `SwiftRing` | Manages ring building via Jobs (account, container, object rings). |
| `SwiftStorage` | Manages storage nodes as a StatefulSet (account, container, object servers, replicators, expirer). |

The `Swift` CR has defaulting and validating admission webhooks.
Sub-CRs are created and owned by the `Swift` controller -- not intended to
be created directly by users.

## Directory structure

| Directory | Contents |
|-----------|----------|
| `api/v1beta1/` | CRD types (`swift_types.go`, `swiftproxy_types.go`, `swiftring_types.go`, `swiftstorage_types.go`), conditions, webhook markers |
| `cmd/` | `main.go` entry point |
| `internal/controller/` | Reconcilers: `swift_controller.go`, `swiftproxy_controller.go`, `swiftring_controller.go`, `swiftstorage_controller.go`, plus `swift_common.go` |
| `internal/swift/` | Swift-level helpers (constants, errors, template utilities) |
| `internal/swiftproxy/` | SwiftProxy resource builders (Deployment, Barbican integration, volumes) |
| `internal/swiftring/` | SwiftRing resource builders (ring-building Job, volumes) |
| `internal/swiftstorage/` | SwiftStorage resource builders (StatefulSet, Services, DNS data, NetworkPolicy) |
| `internal/webhook/v1beta1/` | Webhook implementation |
| `templates/` | Config files: `swift/` (swift.conf), `swiftproxy/` (proxy-server, httpd, SSL, dispersion, keymaster), `swiftstorage/` (account/container/object server confs, rsync, expirer), `swiftring/bin/` (ring-tool script) |
| `config/crd,rbac,manager,webhook/` | Generated Kubernetes manifests (CRDs, RBAC, deployment, webhooks) |
| `config/samples/` | Example CRs: `swift_v1beta1_swift.yaml` (default), `*_tls.yaml`, `*_topology.yaml`, plus individual sub-CR samples |
| `test/functional/` | envtest-based Ginkgo/Gomega tests |
| `test/kuttl/` | KUTTL integration tests (basic-deploy, TLS, topology, customization, replication) |
| `hack/` | Helper scripts (CRD schema checker, local webhook runner) |
| `docs/` | Configuration guide, design decisions |

## Build commands

After modifying Go code, always run: `make generate manifests fmt vet`.

## Code style guidelines

- Follow standard openstack-k8s-operators conventions and lib-common patterns.
- Use `lib-common` modules for conditions, endpoints, TLS, storage, and other
  cross-cutting concerns rather than re-implementing them.
- CRD types go in `api/v1beta1/`. Controller logic goes in
  `internal/controller/`. Resource-building helpers go in `internal/swift*`
  packages matching the CR they support.
- Config templates are plain files in `templates/` -- they are mounted at
  runtime via the `OPERATOR_TEMPLATES` environment variable.
- Webhook logic is split between the kubebuilder markers in `api/v1beta1/` and
  the implementation in `internal/webhook/v1beta1/`.

## Testing

- Functional tests use the envtest framework with Ginkgo/Gomega and live in
  `test/functional/`.
- KUTTL integration tests live in `test/kuttl/`.
- Run all functional tests: `make test`.
- When adding a new field or feature, add corresponding test cases in
  `test/functional/`.

## Key dependencies

- [lib-common](https://github.com/openstack-k8s-operators/lib-common): shared modules for conditions, endpoints, database, TLS, secrets, etc.
- [infra-operator](https://github.com/openstack-k8s-operators/infra-operator): RabbitMQ and topology APIs.
- [keystone-operator](https://github.com/openstack-k8s-operators/keystone-operator): identity service registration.
- [barbican-operator](https://github.com/openstack-k8s-operators/barbican-operator): key management (keymaster integration).
- [gophercloud](https://github.com/gophercloud/gophercloud): Go OpenStack SDK.

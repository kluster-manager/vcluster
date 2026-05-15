# AGENTS.md

This file provides guidance to coding agents (e.g. Claude Code, claude.ai/code) when working with code in this repository.

> **Note for coding agents**: `CLAUDE.md` and the `.claude/` directory are the canonical, project-curated agent instructions for this repository — including the **E2E TDD workflow** at `.claude/e2e-tdd-workflow.md` (require `AGENT_SESSION` env var to isolate sessions when working in parallel) and the CI/GitHub Actions convention pointing at `loft-sh/github-actions`. **Read those first**; this file is a higher-level architecture summary, not a replacement.

## Repository purpose

Go module `github.com/loft-sh/vcluster` — **vCluster**, virtual Kubernetes clusters running inside namespaces of a host cluster. The vCluster control plane runs as workloads on the host; pod/PV/etc. resources sync between virtual and host clusters via configurable mappings. Headline use case: multi-tenant production Kubernetes, including AI-supercluster scale (per the README marketing).

CNCF "Certified Kubernetes — Distribution" and "Kubernetes AI Conformant" (v1.35).

Two binaries:
- `vcluster` — the virtual cluster control plane (runs *inside* a vCluster pod on the host cluster).
- `vclusterctl` — the user-facing CLI (`vcluster create`, `vcluster connect`, `vcluster delete`, etc.).

This repo is the OCM-org mirror of `loft-sh/vcluster`; **upstream is `github.com/loft-sh/vcluster`** and the module path stays that way.

## Architecture

- `cmd/`:
  - `cmd/vcluster/` — control-plane binary entry point (`main.go` + `cmd/` subcommand tree).
  - `cmd/vclusterctl/` — CLI binary (same layout).
- `pkg/` — the heart of the project. Selected directories:
  - `apis/` — vCluster's own CRDs.
  - `apiservice/` — Kubernetes APIService aggregation hooks.
  - `controllers/` — the controller-runtime reconcilers that keep virtual ↔ host state in sync.
  - `mappings/` — per-resource mapping rules (which resources sync, how names/namespaces translate).
  - `patcher/` — patch-based reconciler helpers.
  - `setup/` — virtual cluster bootstrap.
  - `k8s/`, `kubeadm/`, `kube/` — vendored chunks of upstream kube startup logic used to bring up the virtual API server.
  - `etcd/` — embedded etcd lifecycle (vCluster can use embedded etcd, external etcd, or sqlite-backed k3s).
  - `coredns/` — CoreDNS lifecycle inside the virtual cluster.
  - `helm/`, `chart/` — Helm-based deploy paths.
  - `cli/` — CLI command implementations (consumed by `cmd/vclusterctl`).
  - `platform/`, `plugin/`, `pro/` — paid/platform integration boundaries.
  - `integrations/` — first-party integration hooks (Istio, Cilium, etc.).
  - `embed/` — `go:embed`-backed assets shipped in the binary.
  - `authentication/`, `authorization/`, `certs/`, `leaderelection/`, `lifecycle/`, `log/`, `scheme/`, `server/`, `config/`, `constants/`, `projectutil/`, `docker/` — supporting packages.
- `chart/` — Helm chart published as `vcluster`. `values.yaml`, `values.schema.json`, `templates/`.
- `config/` — config schema and helpers.
- `Dockerfile`, `Dockerfile.release` — control-plane images. `Dockerfile.cli`, `Dockerfile.cli.release` — CLI images.
- `Justfile` — primary task runner (replaces a Makefile). `Justfile.agent` — agent-isolated E2E TDD workflow (uses `AGENT_SESSION` for parallelism). `import 'Justfile'` chains the two.
- `assets/` — release artifacts.
- `e2e-next/`, `test/`, `conformance/`, `load-test/` — test suites.
- `docs/` — public-facing docs (also driving `vcluster.com/docs`).
- `devspace.yaml`, `devspace_start.sh` — DevSpace-based dev loops.
- `hack/` — helper scripts (asset generation, codegen).
- `.claude/` — **the agent contract**: `e2e-tdd-workflow.md`, `rules/`, `references/`, `settings.json`, `skills/`. Defer to these.
- `vendor/` — checked-in deps.

## Common commands

This repo uses **[`just`](https://just.systems/)**, not `make`. Targets are defined in `Justfile` (general) and `Justfile.agent` (the parallel-agent E2E flow). Both use [`goreleaser`](https://goreleaser.com/) for builds.

Selected `just` targets (run `just --list` for the full set):

- `just build-snapshot` — build the `vcluster` (control-plane) binary via `goreleaser` and produce `ghcr.io/loft-sh/vcluster:dev-next`. Forces `GOOS=linux` so it works on macOS/Windows hosts.
- `just build-cli-snapshot` — build the `vcluster` CLI binary and copy it to `$GOBIN/vcluster`.
- `just -f Justfile.agent bootstrap <label>` — one-time per agent: create a Kind cluster, build the image, set up vclusters. Uses `AGENT_SESSION` to isolate image tags so parallel agents don't collide.
- `just -f Justfile.agent preflight` — sanity check the kind cluster and vcluster pods are healthy.
- `just -f Justfile.agent push` / etc. — see `Justfile.agent` for the full TDD loop.

`PRIVATE_GO_ENV` is `GOPRIVATE=github.com/loft-sh/* GONOSUMDB=github.com/loft-sh/*` — set this when fetching the private loft-sh modules.

Run a single Go test:

```
go test ./pkg/controllers/... -run TestName -v
```

For end-to-end work, **follow `.claude/e2e-tdd-workflow.md`** rather than improvising.

## Conventions

- Module path is `github.com/loft-sh/vcluster` (**upstream**); imports must use that. The OCM-org filesystem location is just where this checkout lives.
- **Upstream-tracking** fork. Prefer rebasing onto upstream over diverging.
- License: Apache-2.0 (`LICENSE`).
- Sign off commits (`git commit -s`); contributions follow `CONTRIBUTING.md`.
- **Read `CLAUDE.md` and `.claude/` first** for the project-curated agent contract — `e2e-tdd-workflow.md`, the rules under `.claude/rules/`, and the skills directory take precedence over generic conventions. The `github-actions-developer` skill auto-loads when editing `.github/`.
- CI lives in `loft-sh/github-actions` (a separate repo) — do not inline shell into workflows here; reuse those actions.
- Vendor directory is checked in.
- Use `just`, not `make` — there is no Makefile. New build/test targets go into `Justfile` (general) or `Justfile.agent` (E2E TDD).
- For parallel agent work, set a unique `AGENT_SESSION` so image tags (`ghcr.io/loft-sh/vcluster:dev-next-<session>`) and report files (`/tmp/e2e-report-<session>.json`) don't collide.
- `go.mod` is on Go 1.26 — keep that in sync with `goreleaser` and the build images when bumping.

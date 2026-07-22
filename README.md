# coroot-graft

[![release](https://img.shields.io/badge/release-v1.0.0-blue)](https://github.com/MB3R-Lab/coroot-graft/releases/tag/v1.0.0)
[![checks](https://img.shields.io/github/actions/workflow/status/MB3R-Lab/coroot-graft/ci.yml?branch=main&label=checks)](https://github.com/MB3R-Lab/coroot-graft/actions/workflows/ci.yml)
[![mb3r toolchain](https://img.shields.io/badge/mb3r%20toolchain-v1-blue)](docs/compatibility.md)

## Related MB3R repositories

`coroot-graft` wraps Bering and Sheaft as an existing toolchain.

[Bering](https://github.com/MB3R-Lab/Bering) owns discovery and artifact publishing. [Sheaft](https://github.com/MB3R-Lab/Sheaft) owns downstream resilience analysis and CI/CD gating.

It does not reimplement discovery and it does not reimplement the resilience evaluator:

- Bering stays responsible for model discovery / normalization.
- Sheaft stays responsible for downstream resilience analysis / gate.
- `coroot-graft` owns Coroot-specific extraction, topology shaping, orchestration, artifact lifecycle, and publishing results back into Coroot.

## Architecture

Pipeline:

1. `coroot-graft` logs into Coroot and reads a stable topology window plus a
   short runtime activity window from Coroot HTTP APIs.
2. The stable Coroot snapshot is normalized into explicit Bering `topology_api` input.
3. `coroot-graft` runs upstream `bering discover`.
4. `coroot-graft` runs upstream `sheaft run` on the produced Bering artifact.
5. It overlays current activity on the stable Sheaft report: an endpoint is
   forced to `0` when its blocking dependency path contains a service that was
   not observed in the short window.
6. `coroot-graft` exposes the effective gate/report state as Prometheus metrics.
7. Coroot scrapes these metrics via `coroot-cluster-agent` and renders them in a custom dashboard.
8. Optional Coroot webhook integration triggers re-runs on alerts, incidents, or deployments.

## Simple Mental Model

- `Coroot` provides the observed operational graph of the system.
- `coroot-graft` translates that observed graph into MB3R toolchain input.
- `Bering` turns that input into canonical MB3R discovery artifacts.
- `Sheaft` computes resilience posture and gate results from those artifacts.
- `coroot-graft` combines that stable posture with Coroot's latest service
  observation, without deleting known topology.
- `Coroot` renders the effective result back to engineers through custom metrics and a managed dashboard.

In this MVP, `Coroot` is the primary source of topology membership and dependency context.

`Bering` is still required because `Sheaft` consumes MB3R artifacts, not raw Coroot API payloads.

## Terms

### Coroot snapshot

When this repo says "Coroot snapshot", it means the in-memory snapshot collected by `coroot-graft` during a sync:

- applications
- dependencies
- replica counts
- optional traced HTTP entrypoints
- a separate set of services observed in the short runtime activity window

This snapshot is reconstructed from Coroot APIs on every sync. It is not a separate long-lived Coroot export format.

### `topology_api`

`topology_api` is Bering's explicit batch input format for already-known topology.

It is just a YAML/JSON document with:

- `services`
- `edges`
- `endpoints`
- a `source` reference

`coroot-graft` generates this document from the Coroot snapshot and passes it to `bering discover`.

### Bering snapshot

After `bering discover`, upstream `Bering` emits canonical MB3R artifacts such as:

- `bering-model.json`
- `bering-snapshot.json`

Those are the artifacts consumed by `Sheaft`.

## What The Dashboard Means

The managed Coroot dashboard combines two signals:

- stable resilience posture calculated by Bering and Sheaft
- current service observation from Coroot's short activity window

It answers questions like:

- which entrypoints are most fragile under service failures?
- which downstream dependency hurts an upstream user path the most?
- does the current topology pass the configured resilience gate?
- which known services were not observed recently?
- which endpoint paths are currently broken by an unobserved blocking dependency?

When `cart` disappears from the activity window, the stable topology still
contains `cart`, but every endpoint whose blocking path requires it is exported
with availability `0`. The effective verdict becomes `warn` or `fail` according
to the configured gate mode.

This runtime overlay is intentionally narrower than full health monitoring. It
does not explain why a service disappeared, replace alerts/incidents, or expose
CPU and memory health. Built-in Coroot views remain the source for that detail.

Detection is windowed rather than instantaneous. With the shipped defaults
(`activity_window: 2m`, `interval: 1m`), a stopped service is reflected after it
has left the activity window and the next sync completes.

It does not answer on its own:

- why did the service stop?
- which alert or incident caused the outage?
- what are the current CPU, memory, latency, and error details?

For current health and incidents, use the built-in Coroot views.

For resilience posture and gate decisions, use the `coroot-graft` dashboard.

## Coroot Versus coroot-graft

| Question | Coroot built-in views | `coroot-graft` |
| --- | --- | --- |
| Was the service observed in the latest activity window? | Yes | Yes |
| Does an unobserved dependency break a modeled endpoint path? | No | Yes |
| Why is a service or instance unhealthy? | Yes | No |
| Is a container down at this exact second? | Yes | Windowed observation only |
| Are there active alerts, incidents, or regressions right now? | Yes | No |
| What applications and dependencies does Coroot currently observe? | Yes | Indirectly, as input |
| What is the resilience posture of the currently observed topology? | No | Yes |
| Which entrypoints are fragile under downstream failures? | No | Yes |
| Does the current topology pass the configured resilience gate? | No | Yes |
| Is this a release-gating / resilience-review signal? | Partial | Yes |

## Why This Is Useful

The dashboard gives engineers:

- a gate verdict: `pass`, `warn`, or `fail`
- a risk score for the currently observed topology
- cross-profile resilience estimates
- per-profile failure-mode estimates such as steady-state, service-fault, and fixed blast radius
- per-endpoint availability estimates against configured thresholds
- per-service recent observation (`coroot_graft_service_observed`)
- per-endpoint runtime path availability (`coroot_graft_endpoint_runtime_available`)

This is useful for release gating, topology reviews, dependency risk assessment, and resilience prioritization.

## Commands

`coroot-graft` provides:

- `serve`: run the HTTP server, expose `/metrics`, accept Coroot webhooks, and schedule syncs
- `sync`: run one-shot Coroot -> Bering -> Sheaft sync for one or all configured projects
- `install-dashboard`: create or update a managed Coroot dashboard backed by exported metrics

## Installation

### Release packages

Download the packaged release artifacts from [GitHub Releases](https://github.com/MB3R-Lab/coroot-graft/releases):

- `coroot-graft_<version>_linux_amd64.tar.gz`
- `coroot-graft_<version>_linux_arm64.tar.gz`
- `coroot-graft_<version>_darwin_amd64.tar.gz`
- `coroot-graft_<version>_darwin_arm64.tar.gz`
- `coroot-graft_<version>_windows_amd64.zip`
- `coroot-graft_<version>_source.tar.gz`

Each release also publishes:

- `coroot-graft_<version>_checksums.txt`
- `coroot-graft_<version>_<os>_<arch>.tar.gz.sbom.json` for Linux and macOS archives
- `coroot-graft_<version>_windows_amd64.zip.sbom.json`
- `coroot-graft-<version>.tgz` Helm chart package
- `compatibility-manifest.json`
- `toolchain.env`

### OCI image

The release workflow publishes a runtime image that already contains:

- `coroot-graft`
- `bering`
- `sheaft`

Image repository:

- `ghcr.io/mb3r-lab/coroot-graft`

### OCI Helm chart

The Helm chart is also published to:

- `oci://ghcr.io/mb3r-lab/charts/coroot-graft`

## Config

See `configs/graft.example.yaml` for the expected shape.

Main config areas:

- `coroot`: Coroot URL, credentials, stable `time_window`, and short `activity_window`
- `toolchain`: command lines for upstream `bering` and `sheaft`
- `projects[]`: Coroot project mapping, analysis/policy config, include/exclude filters, edge overrides, scrape/webhook settings

Environment variables are expanded before YAML parsing, so production deployments can keep secrets out of Git:

```yaml
coroot:
  password: "${COROOT_GRAFT_COROOT_PASSWORD}"
```

`activity_window` defaults to `2m` and must not exceed `time_window`. The stable
window preserves known services and dependencies; the activity window decides
which of them are currently observed.

## Runtime Artifacts

For each project, the HTTP API exposes:

- `/api/v1/projects/{project}/report`: effective runtime-adjusted report
- `/api/v1/projects/{project}/sheaft-report`: raw Sheaft report over stable topology
- `/api/v1/projects/{project}/summary`: effective summary
- `/api/v1/projects/{project}/sheaft-summary`: raw Sheaft summary
- `/api/v1/projects/{project}/activity`: active/inactive services and impacted endpoints

## Coroot Wiring

### Custom metrics

Expose `coroot-graft` in Kubernetes and annotate the pod so `coroot-cluster-agent` scrapes `/metrics`:

```yaml
metadata:
  annotations:
    coroot.com/scrape-metrics: "true"
    coroot.com/metrics-port: "8095"
    coroot.com/metrics-path: "/metrics"
```

### Webhook trigger

`coroot-graft` is not hosted by MB3R. The Coroot Webhook integration must send
`POST` requests to the `coroot-graft` service deployed in your own cluster.

For the default Helm release name, namespace, and example project config, the
in-cluster URL is:

```text
http://coroot-graft.coroot-graft.svc.cluster.local:8095/webhooks/coroot/production?secret=replace-me
```

URL parts:

| Part | Default | Source |
| --- | --- | --- |
| Service DNS name | `coroot-graft.coroot-graft.svc.cluster.local` | Helm release `coroot-graft` in namespace `coroot-graft` |
| Path project name | `production` | `projects[].name` in `graft.yaml` |
| Secret query value | `replace-me` | value of `projects[].webhook_secret` after environment expansion |

The path project name can differ from `projects[].coroot_project`.

Coroot documents `{{ json . }}` as its built-in JSON template function. Use it in
the Coroot Webhook integration JSON template field:

```gotemplate
{{ json . }}
```

`coroot-graft` treats the webhook as a trigger. It does not currently parse the
request body; the project and secret are taken from the URL.

## Documentation

- Production install guide: `docs/install.md`
- Pinned compatibility baseline: `docs/compatibility.md`
- Architecture and MVP boundaries: `docs/architecture.md`
- Release assets and package matrix: `docs/release-assets.md`
- Integration policy: `docs/integration-policy.md`
- Machine-readable version pins: `compatibility-manifest.json`
- Public v1.0.0 release notes: `docs/releases/v1.0.0.md`

## License

MIT, see [LICENSE](LICENSE).

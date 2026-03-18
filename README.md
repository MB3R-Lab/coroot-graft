# coroot-graft

`coroot-graft` is an external Coroot extension that wraps upstream `Bering` and `Sheaft` as an existing toolchain.

It does not reimplement discovery and it does not reimplement the resilience evaluator:

- `Bering` stays responsible for model discovery / normalization.
- `Sheaft` stays responsible for downstream resilience analysis / gate.
- `coroot-graft` owns Coroot-specific extraction, topology shaping, orchestration, artifact lifecycle, and publishing results back into Coroot.

## Architecture

Pipeline:

1. `coroot-graft` logs into Coroot and reads a project snapshot from Coroot HTTP APIs.
2. The Coroot snapshot is normalized into explicit Bering `topology_api` input.
3. `coroot-graft` runs upstream `bering discover`.
4. `coroot-graft` runs upstream `sheaft run` on the produced Bering artifact.
5. `coroot-graft` exposes the latest gate/report state as Prometheus metrics.
6. Coroot scrapes these metrics via `coroot-cluster-agent` and renders them in a custom dashboard.
7. Optional Coroot webhook integration triggers re-runs on alerts, incidents, or deployments.

## Simple Mental Model

- `Coroot` provides the observed operational graph of the system.
- `coroot-graft` translates that observed graph into MB3R toolchain input.
- `Bering` turns that input into canonical MB3R discovery artifacts.
- `Sheaft` computes resilience posture and gate results from those artifacts.
- `Coroot` renders the resulting posture back to engineers through custom metrics and a managed dashboard.

In this MVP, `Coroot` is the primary source of topology membership and dependency context.

`Bering` is still required because `Sheaft` consumes MB3R artifacts, not raw Coroot API payloads.

## Terms

### Coroot snapshot

When this repo says "Coroot snapshot", it means the in-memory snapshot collected by `coroot-graft` during a sync:

- applications
- dependencies
- replica counts
- optional traced HTTP entrypoints

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

The managed Coroot dashboard shows `resilience posture`, not raw runtime health.

It answers questions like:

- which entrypoints are most fragile under service failures?
- which downstream dependency hurts an upstream user path the most?
- does the current topology pass the configured resilience gate?

It does not answer:

- is the service healthy right now?
- is a container currently down?
- is there traffic in this exact second?

For current health and incidents, use the built-in Coroot views.

For resilience posture and gate decisions, use the `coroot-graft` dashboard.

## Coroot Versus coroot-graft

| Question | Coroot built-in views | `coroot-graft` |
| --- | --- | --- |
| Is the service unhealthy right now? | Yes | No |
| Is a container or instance down right now? | Yes | No |
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

This is useful for release gating, topology reviews, dependency risk assessment, and resilience prioritization.

## Commands

`coroot-graft` provides:

- `serve`: run the HTTP server, expose `/metrics`, accept Coroot webhooks, and schedule syncs
- `sync`: run one-shot Coroot -> Bering -> Sheaft sync for one or all configured projects
- `install-dashboard`: create or update a managed Coroot dashboard backed by exported metrics

## Installation

### Release packages

Download the packaged release artifacts from GitHub Releases:

- `coroot-graft_<version>_linux_amd64.tar.gz`
- `coroot-graft_<version>_linux_arm64.tar.gz`
- `coroot-graft_<version>_darwin_amd64.tar.gz`
- `coroot-graft_<version>_darwin_arm64.tar.gz`
- `coroot-graft_<version>_windows_amd64.zip`

Each release also publishes:

- `coroot-graft_<version>_checksums.txt`
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

- `coroot`: Coroot URL and credentials
- `toolchain`: command lines for upstream `bering` and `sheaft`
- `projects[]`: Coroot project mapping, analysis/policy config, include/exclude filters, edge overrides, scrape/webhook settings

Environment variables are expanded before YAML parsing, so production deployments can keep secrets out of Git:

```yaml
coroot:
  password: "${COROOT_GRAFT_COROOT_PASSWORD}"
```

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

Configure a Coroot Webhook integration to call:

```text
http://coroot-graft:8095/webhooks/coroot/<project>?secret=<secret>
```

The body can be the documented Coroot JSON payload template:

```gotemplate
{{ json . }}
```

## Upstream Notes

Contract notes used for this implementation are summarized in `docs/upstream-summary.md`.

Implementation notes and MVP boundaries are documented in `docs/architecture.md`.

## Install And Compatibility

- Production install guide: `docs/install.md`
- Pinned compatibility baseline: `docs/compatibility.md`
- Release assets and package matrix: `docs/release-assets.md`
- Roadmap and issue index: `docs/roadmap.md`
- Integration policy: `docs/integration-policy.md`
- Machine-readable version pins: `compatibility-manifest.json`

## License

MIT, see [LICENSE](LICENSE).

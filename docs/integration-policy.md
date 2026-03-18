# Integration Policy

`coroot-graft` is intentionally split into stable contracts and version-pinned adapters.

## Stable Contracts

These are the surfaces the project is designed around:

- upstream `Bering` CLI and emitted artifacts
- upstream `Sheaft` CLI and emitted report contract
- Prometheus exposition from `coroot-graft` on `/metrics`
- Coroot custom dashboards built on top of PromQL and custom metrics

These are product-level integration contracts, not implementation accidents.

## Observed Versus Simulated Signals

The product intentionally keeps two different signal classes separate:

- observed operational context from Coroot
- simulated resilience posture from `Sheaft`

`coroot-graft` consumes the first and publishes the second.

It is not meant to replace Coroot's built-in health, incident, or alert views.

## Version-Pinned Adapter Surfaces

These are Coroot-specific surfaces that must be pinned and revalidated for every `coroot-graft` release:

- Coroot HTTP routes used for snapshot extraction
- Coroot HTTP routes used for dashboard CRUD
- pinned Coroot runtime images used in the release baseline

These surfaces are valid to use, but they are adapter surfaces rather than portable contracts. They are the reason every `coroot-graft` release publishes a compatibility baseline.

## Local Docker Harness

The local Docker demo is not the production integration model.

In Kubernetes, Coroot custom metrics are expected to be discovered by `coroot-cluster-agent` using pod annotations.

In local Docker mode, that discovery path does not exist, so `deploy/docker/coroot-compose.graft-local.yaml` injects an explicit Prometheus scrape target for `coroot-graft`.

That explicit scrape is a development harness for local validation. It is not a product hack and it is not a substitute for the Kubernetes integration path.

## Non-Goals

`coroot-graft` does not rely on:

- Coroot built-in discovery internals as a replacement for `Bering`
- Coroot built-in evaluators as a replacement for `Sheaft`
- patching or forking Coroot UI to render graft results inside built-in screens

The Coroot-facing result surface is:

- operational context in
- custom metrics out
- managed custom dashboards out
- optional webhook-triggered resync

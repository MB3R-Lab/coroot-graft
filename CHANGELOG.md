# Changelog

## Unreleased

## v0.1.0 - 2026-03-18

First public `coroot-graft` MVP release.

Included in this release:

- Coroot adapter for project resolution, snapshot extraction, topology normalization, and dashboard installation
- upstream `Bering` and `Sheaft` orchestration without reimplementing discovery or evaluation
- persisted run artifacts, Prometheus metrics, HTTP artifact access, and webhook-triggered resync
- local Docker validation harness against a pinned Coroot stack
- production packaging via container image, Helm chart, install guide, and compatibility manifest
- GitHub Actions CI, upstream smoke coverage, and release-time live end-to-end validation workflow

Stable within `v0.1.0`:

- Coroot-derived `topology_api` generation for upstream `Bering`
- MB3R artifact handoff from `Bering` to `Sheaft`
- managed Coroot dashboard publication backed by exported `coroot_graft_*` metrics
- pinned compatibility baseline for Coroot, Bering, and Sheaft

Known limitations:

- published dashboard values represent recomputed resilience posture, not a direct live health score
- topology membership is primarily driven by Coroot application inventory in the current MVP
- runtime degradation and posture are not yet separated into distinct published signal families

- Added live Coroot validation path using a pinned local Docker stack.
- Added production packaging: multi-binary container image, Helm chart, install guide, and compatibility manifest.
- Added pinned compatibility policy for Coroot, Bering, and Sheaft.

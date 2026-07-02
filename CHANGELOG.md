# Changelog

## Unreleased

## v0.2.0 - 2026-07-02

Compatibility release for the MB3R v1 major line.

Included in this release:

- pinned upstream Bering to `v1.0.0`
- pinned upstream Sheaft to `v1.0.0`
- updated the Bering model/snapshot contract baseline to `1.3.0`
- updated local Docker, Helm, CI, and release build defaults to embed the v1 toolchain

## v0.1.2 - 2026-03-18

Patch release that turns the first public MVP into a fully packaged public release.

Included in this release:

- cross-platform CLI archives for Linux, macOS, and Windows
- release checksum publication
- multi-arch OCI image publication to `ghcr.io/mb3r-lab/coroot-graft`
- OCI Helm chart publication to `oci://ghcr.io/mb3r-lab/charts/coroot-graft`
- release automation with GoReleaser and tag-triggered GitHub Actions
- README and install docs updated with release package locations
- repository About summary populated

Stable within `v0.1.2`:

- the same Coroot adapter, topology normalization, and Bering -> Sheaft orchestration shipped in `v0.1.0`
- pinned compatibility baseline for Coroot, Bering, and Sheaft
- managed Coroot dashboard publication backed by `coroot_graft_*` metrics

Known limitations remain unchanged from `v0.1.0`:

- published dashboard values represent recomputed resilience posture, not a direct live health score
- topology membership is primarily driven by Coroot application inventory in the current MVP
- runtime degradation and posture are not yet separated into distinct published signal families

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

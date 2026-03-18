# Releasing

Release checklist:

1. Update [compatibility-manifest.json](/compatibility-manifest.json) and [versions/toolchain.env](/versions/toolchain.env) with the exact tested Coroot, Bering, and Sheaft refs. Coroot runtime images must be pinned as immutable digests.
2. Bump the release version everywhere it is part of the shipped surface:
   - [CHANGELOG.md](/CHANGELOG.md)
   - [compatibility-manifest.json](/compatibility-manifest.json)
   - [charts/coroot-graft/Chart.yaml](/charts/coroot-graft/Chart.yaml)
   - [charts/coroot-graft/values.yaml](/charts/coroot-graft/values.yaml)
   - [build/Dockerfile](/build/Dockerfile)
   - [Makefile](/Makefile)
   - [deploy/docker/coroot-compose.graft-local.yaml](/deploy/docker/coroot-compose.graft-local.yaml)
3. Run the full local test suite:
   - `go test -count=1 ./...`
   - `go build ./cmd/coroot-graft`
4. Build the production image:
   - `make docker-build IMAGE=<registry>/coroot-graft:<tag> APP_VERSION=<tag>`
5. Validate the Helm chart:
   - `make chart-lint`
   - `make chart-template`
6. Validate release packages locally if GoReleaser is available:
   - `goreleaser release --clean --snapshot --skip=publish`
7. Push the release commit and `vX.Y.Z` tag. The tag workflow is expected to publish:
   - GitHub Release archives
   - release checksums
   - Helm chart package
   - multi-arch OCI image
   - OCI Helm chart
8. Publish the release only after the `Release E2E` GitHub workflow passes against the pinned local Coroot stack.

The release workflow is expected to validate:

- pinned upstream toolchain refs
- pinned Coroot stack images
- release archive build and checksum generation
- OCI image push
- Helm chart package and OCI push
- live `sync`
- dashboard installation
- runtime `/metrics` and `/api/v1/projects` export

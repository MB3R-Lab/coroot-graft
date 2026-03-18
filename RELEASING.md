# Releasing

Release checklist:

1. Update [compatibility-manifest.json](/compatibility-manifest.json) and [versions/toolchain.env](/versions/toolchain.env) with the exact tested Coroot, Bering, and Sheaft refs. Coroot runtime images must be pinned as immutable digests.
2. Run the full local test suite:
   - `go test -count=1 ./...`
   - `go build ./cmd/coroot-graft`
3. Build the production image:
   - `make docker-build IMAGE=<registry>/coroot-graft:<tag> APP_VERSION=<tag>`
4. Validate the Helm chart:
   - `make chart-lint`
   - `make chart-template`
5. Publish the release only after the `Release E2E` GitHub workflow passes against the pinned local Coroot stack.

The release workflow is expected to validate:

- pinned upstream toolchain refs
- pinned Coroot stack images
- live `sync`
- dashboard installation
- runtime `/metrics` and `/api/v1/projects` export

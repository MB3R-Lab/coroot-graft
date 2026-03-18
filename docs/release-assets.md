# Release Assets

`coroot-graft` release packaging follows the same basic shape as the upstream MB3R tools:

- cross-platform CLI archives
- a checksum file
- a packaged Helm chart
- pinned compatibility metadata
- pinned toolchain metadata
- a multi-arch OCI runtime image

## GitHub Release assets

Every `vX.Y.Z` release is expected to publish:

- `coroot-graft_X.Y.Z_linux_amd64.tar.gz`
- `coroot-graft_X.Y.Z_linux_arm64.tar.gz`
- `coroot-graft_X.Y.Z_darwin_amd64.tar.gz`
- `coroot-graft_X.Y.Z_darwin_arm64.tar.gz`
- `coroot-graft_X.Y.Z_windows_amd64.zip`
- `coroot-graft_X.Y.Z_checksums.txt`
- `coroot-graft-<chart-version>.tgz`
- `compatibility-manifest.json`
- `toolchain.env`

Archive contents:

- `coroot-graft`
- `README.md`
- `LICENSE`

The release archives package only the `coroot-graft` CLI. The production OCI image remains the distribution format that embeds pinned upstream `bering` and `sheaft` binaries together with `coroot-graft`.

## OCI image

The release workflow publishes:

- `ghcr.io/mb3r-lab/coroot-graft:vX.Y.Z`
- `ghcr.io/mb3r-lab/coroot-graft:X.Y.Z`

The image is multi-arch for:

- `linux/amd64`
- `linux/arm64`

## OCI Helm chart

The release workflow also publishes:

- `oci://ghcr.io/mb3r-lab/charts/coroot-graft`

Use `helm pull` or `helm upgrade --install` against that OCI chart reference.
